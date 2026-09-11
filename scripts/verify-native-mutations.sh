#!/usr/bin/env bash
# Native acceptance mutations. Every edit is restored, even on interruption;
# a compiler error, timeout or unrelated failure is not a killed mutation.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
platform="$(go env GOOS)"
files=(internal/source/source.go internal/server/handler.go internal/server/lifecycle.go app.go .github/workflows/verify.yml)
if [[ "$platform" == linux || "$platform" == darwin ]]; then
  files+=("internal/source/handle_${platform}.go" internal/source/handle_posix.go)
fi
for file in "${files[@]}"; do
  mkdir -p "$scratch/$(dirname "$file")"
  cp "$file" "$scratch/$file"
done
restore() {
  for file in "${files[@]}"; do cp "$scratch/$file" "$file"; done
}
trap restore EXIT

expect_named_failure() {
  local label="$1" test_name="$2" package="$3"
  if go test -count=1 -timeout 30s -run "^${test_name}$" "$package" > "$scratch/result.log" 2>&1; then
    cat "$scratch/result.log"
    echo "SURVIVED: $label" >&2
    exit 1
  fi
  cat "$scratch/result.log"
  if ! grep -F -- "--- FAIL: $test_name" "$scratch/result.log" >/dev/null; then
    echo "Mutation failed without naming its expected test: $label" >&2
    exit 1
  fi
  echo "KILLED: $label by $test_name"
  restore
}

if [[ "$platform" == linux || "$platform" == darwin ]]; then
  perl -0pi -e 's/ \| unix\.O_NONBLOCK// or die "O_NONBLOCK mutation did not match\n"' "internal/source/handle_${platform}.go"
  expect_named_failure 'drop O_NONBLOCK' TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking ./internal/source

  perl -0pi -e 's/if status\.Mode&unix\.S_IFMT != unix\.S_IFREG/if false \&\& status.Mode\&unix.S_IFMT != unix.S_IFREG/ or die "S_IFREG mutation did not match\n"' internal/source/handle_posix.go
  expect_named_failure 'drop regular-file refusal' TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking ./internal/source

  # This regression is only observable on a POSIX sender.
  perl -0pi -e 's/volumeQualified\(name\)/filepath.IsAbs(name) || filepath.VolumeName(name) != ""/ or die "volume mutation did not match\n"' internal/source/source.go
  expect_named_failure 'host-dependent source volume predicate' TestNativeChildNameRefusesReceiverVolumePrefixes ./internal/source
fi

if [[ "$platform" == darwin ]]; then
  perl -0pi -e 's/unix\.AT_SYMLINK_NOFOLLOW/0/ or die "fstatat no-follow mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'follow metadata symlinks' TestDarwinMetadataNoFollowAndSpecialFileClassification ./internal/source

  perl -0pi -e 's/return nativeSameFile\(first, second\)/return os.SameFile(first, second)/ or die "native identity mutation did not match\n"' internal/source/source.go
  expect_named_failure 'use os.SameFile on stat-derived metadata' TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement ./internal/source

  perl -0pi -e 's/firstDev == secondDev \&\& firstIno == secondIno/firstDev == firstDev \&\& secondDev == secondDev \&\& firstIno == firstIno \&\& secondIno == secondIno/ or die "identity refusal mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'accept replaced metadata identity' TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement ./internal/source

  perl -0pi -e 's/var status unix.Stat_t/fd, openErr := openPosixDescriptor(locator, unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK); if openErr != nil { return nil, openErr }; _ = unix.Close(fd); var status unix.Stat_t/ or die "metadata read-access mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'require content read permission for metadata' TestDarwinMetadataSnapshotNeedsNoContentPermission ./internal/source
fi

perl -0pi -e 's/connection\.terminal = \&event/r.finish(\&event); connection.terminal = \&event/ or die "early terminal mutation did not match\n"' internal/server/handler.go
expect_named_failure 'publish terminal before response finalization' TestNaturalCompletionWaitsForHTTPFinalization ./internal/server

perl -0pi -e 's/if err == nil \&\& n != len\(p\)/if false \&\& err == nil \&\& n != len(p)/ or die "short-write mutation did not match\n"' internal/server/lifecycle.go
expect_named_failure 'ignore final short write' TestNaturalCompletionWaitsForHTTPFinalization ./internal/server

perl -0pi -e 's/r\.finalizeAfterResponse\(request, event\)/r.finish(\&event)/ or die "early 410 terminal mutation did not match\n"' internal/server/handler.go
expect_named_failure 'publish preparation failure before 410 finalization' TestPreparationFailureFinalizes410BeforeTerminalTeardown ./internal/server

perl -0pi -e 's/return filepath\.Join\(parent, leaf\) \+ selection\[len\(leafPath\):\]/resolved, _ := filepath.EvalSymlinks(filepath.Join(parent, leaf)); return resolved + selection[len(leafPath):]/ or die "leaf mutation did not match\n"' app.go
expect_named_failure 'resolve the selected leaf' TestStageTransferRefusesSelectedSymlinkAndPreservesTraversalRefusals .

perl -0pi -e 's/\n  linux-adapters:.*\z/\n/s or die "Linux job mutation did not match\n"' .github/workflows/verify.yml
expect_named_failure 'remove Linux adapter job' TestVerifyWorkflowLinuxJobIsAdapterVerificationOnly .
