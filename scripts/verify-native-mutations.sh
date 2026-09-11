#!/usr/bin/env bash
# Native acceptance mutations. Every edit is restored, even on interruption;
# a compiler error, timeout or unrelated failure is not a killed mutation.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
platform="$(go env GOOS)"
files=(internal/source/source.go internal/server/handler.go internal/server/lifecycle.go internal/stream/payload.go internal/stream/archive.go selection_source.go single_instance_darwin.go .github/workflows/verify.yml)
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

baseline() {
  local test_name="$1" package="$2" status=0
  go test -count=1 -v -timeout 60s -run "^$test_name$" "$package" > "$scratch/baseline-$test_name.log" 2>&1 || status=$?
  cat "$scratch/baseline-$test_name.log"
  go run ./scripts/mutationverdict -mode baseline -test "$test_name" -status "$status" -log "$scratch/baseline-$test_name.log"
}

# A failing or skipped fixture cannot establish the mutation's baseline.
baseline TestNaturalCompletionWaitsForHTTPFinalization ./internal/server
baseline TestPreparationFailureFinalizes410BeforeTerminalTeardown ./internal/server
baseline TestHTTPRejectionsPreserveTCPHalfClose ./internal/server
baseline TestWriteToConcurrentCallersStreamExactlyOnce ./internal/stream
baseline TestStageTransferRefusesSelectedSymlinkAndPreservesTraversalRefusals .
baseline TestSelectionResolutionHonoursAdmissionAndCancellation .
baseline TestVerifyWorkflowLinuxJobIsAdapterVerificationOnly .
baseline TestVerifyWorkflowPinsNativeProofGatesToTheirJobsAndPlatforms .
if [[ "$platform" == linux || "$platform" == darwin ]]; then
  baseline TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking ./internal/source
  baseline TestNativeChildNameRefusesReceiverVolumePrefixes ./internal/source
fi
if [[ "$platform" == darwin ]]; then
  baseline TestDarwinMetadataNoFollowAndSpecialFileClassification ./internal/source
  baseline TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement ./internal/source
  baseline TestDarwinMetadataRefusesRecycledIdentityAcrossStatRepresentations ./internal/source
  baseline TestDarwinMetadataSnapshotNeedsNoContentPermission ./internal/source
  baseline TestDarwinLockPreflightRefusesFIFOAndSymlinkWithoutBlocking .
  baseline TestDarwinLockKindIsCheckedBeforeFlock .
fi

expect_named_failure() {
  local label="$1" test_name="$2" package="$3" evidence="$4" status=0
  go test -count=1 -v -timeout 60s -run "^${test_name}$" "$package" > "$scratch/result.log" 2>&1 || status=$?
  cat "$scratch/result.log"
  go run ./scripts/mutationverdict -mode mutation -test "$test_name" -assert "$evidence" -status "$status" -log "$scratch/result.log"
  echo "KILLED: $label by $test_name"
  restore
}

if [[ "$platform" == linux || "$platform" == darwin ]]; then
  perl -0pi -e 's/ \| unix\.O_NONBLOCK// or die "O_NONBLOCK mutation did not match\n"' "internal/source/handle_${platform}.go"
  expect_named_failure 'drop O_NONBLOCK' TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking ./internal/source 'content open blocked on substituted FIFO: O_NONBLOCK guard missing'

  perl -0pi -e 's/if status\.Mode&unix\.S_IFMT != unix\.S_IFREG/if false \&\& status.Mode\&unix.S_IFMT != unix.S_IFREG/ or die "S_IFREG mutation did not match\n"' internal/source/handle_posix.go
  expect_named_failure 'drop regular-file refusal' TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking ./internal/source 'FIFO received a content handle'

  # This regression is only observable on a POSIX sender.
  perl -0pi -e 's/volumeQualified\(name\)/filepath.IsAbs(name) || filepath.VolumeName(name) != ""/ or die "volume mutation did not match\n"' internal/source/source.go
  expect_named_failure 'host-dependent source volume predicate' TestNativeChildNameRefusesReceiverVolumePrefixes ./internal/source 'unsafe fixture name accepted'
fi

if [[ "$platform" == darwin ]]; then
  perl -0pi -e 's/unix\.AT_SYMLINK_NOFOLLOW/0/ or die "fstatat no-follow mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'follow metadata symlinks' TestDarwinMetadataNoFollowAndSpecialFileClassification ./internal/source 'want mode'

  perl -0pi -e 's/return nativeSameFile\(first, second\)/return os.SameFile(first, second)/ or die "native identity mutation did not match\n"' internal/source/source.go
  expect_named_failure 'use os.SameFile on stat-derived metadata' TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement ./internal/source 'snapshot-to-descriptor identity rejected the same object'

  perl -0pi -e 's/firstID == secondID/firstID == firstID \&\& secondID == secondID/ or die "identity refusal mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'accept replaced metadata identity' TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement ./internal/source 'replacement identity gate'

  perl -0pi -e 's/var status unix.Stat_t/fd, openErr := openPosixDescriptor(locator, unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK); if openErr != nil { return nil, openErr }; _ = unix.Close(fd); var status unix.Stat_t/ or die "metadata read-access mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'require content read permission for metadata' TestDarwinMetadataSnapshotNeedsNoContentPermission ./internal/source 'native metadata open failed'

  perl -0pi -e 's/firstID == secondID/firstID.device == secondID.device \&\& firstID.inode == secondID.inode/ or die "recycled identity mutation did not match\n"' internal/source/handle_darwin.go
  expect_named_failure 'ignore generation and birth identity' TestDarwinMetadataRefusesRecycledIdentityAcrossStatRepresentations ./internal/source 'recycled identity accepted after generation changed'

  perl -0pi -e 's/\|syscall\.O_NONBLOCK// or die "lock nonblocking mutation did not match\n"' single_instance_darwin.go
  expect_named_failure 'block on lock FIFO' TestDarwinLockPreflightRefusesFIFOAndSymlinkWithoutBlocking . 'lock preflight blocked on an unconnected FIFO'

  perl -0pi -e 's/\|syscall\.O_NOFOLLOW// or die "lock no-follow mutation did not match\n"' single_instance_darwin.go
  expect_named_failure 'follow lock symlink' TestDarwinLockPreflightRefusesFIFOAndSymlinkWithoutBlocking . 'special lock path was accepted'

  perl -0pi -e 's/!info\.Mode\(\)\.IsRegular\(\)/false \&\& !info.Mode().IsRegular()/ or die "lock kind mutation did not match\n"' single_instance_darwin.go
  expect_named_failure 'accept nonregular lock descriptor' TestDarwinLockKindIsCheckedBeforeFlock . 'nonregular lock descriptor reached flock'
fi

perl -0pi -e 's/connection\.terminal = \&event/r.finish(\&event); connection.terminal = \&event/ or die "early terminal mutation did not match\n"' internal/server/handler.go
expect_named_failure 'publish terminal before response finalization' TestNaturalCompletionWaitsForHTTPFinalization ./internal/server 'unexpected complete event'

perl -0pi -e 's/if !p\.streamed\.CompareAndSwap/if false \&\& !p.streamed.CompareAndSwap/ or die "file ownership mutation did not match\n"' internal/stream/payload.go
expect_named_failure 'allow a concurrent file reader' TestWriteToConcurrentCallersStreamExactlyOnce ./internal/stream 'concurrent second WriteTo read the file instead of refusing ownership'

perl -0pi -e 's/if !a\.streamed\.CompareAndSwap/if false \&\& !a.streamed.CompareAndSwap/ or die "archive ownership mutation did not match\n"' internal/stream/archive.go
expect_named_failure 'allow a concurrent archive writer' TestWriteToConcurrentCallersStreamExactlyOnce ./internal/stream 'concurrent second WriteTo was not refused before writing'

perl -0pi -e 's/if err == nil \&\& n != len\(p\)/if false \&\& err == nil \&\& n != len(p)/ or die "short-write mutation did not match\n"' internal/server/lifecycle.go
expect_named_failure 'ignore final short write' TestNaturalCompletionWaitsForHTTPFinalization ./internal/server 'failed final write was not reported as transfer_failed'

perl -0pi -e 's/r\.finalizeAfterResponse\(request, event\)/r.finish(\&event)/ or die "early 410 terminal mutation did not match\n"' internal/server/handler.go
expect_named_failure 'publish preparation failure before 410 finalization' TestPreparationFailureFinalizes410BeforeTerminalTeardown ./internal/server 'unexpected failed event'

perl -0pi -e 's/return filepath\.Join\(parent, leaf\) \+ selection\[len\(leafPath\):\]/resolved, _ := filepath.EvalSymlinks(filepath.Join(parent, leaf)); return resolved + selection[len(leafPath):]/ or die "leaf mutation did not match\n"' selection_source.go
expect_named_failure 'resolve the selected leaf' TestStageTransferRefusesSelectedSymlinkAndPreservesTraversalRefusals . 'entry resolution bypassed an existing traversal refusal'

perl -0pi -e 's/\n  linux-adapters:.*\z/\n/s or die "Linux job mutation did not match\n"' .github/workflows/verify.yml
expect_named_failure 'remove Linux adapter job' TestVerifyWorkflowLinuxJobIsAdapterVerificationOnly . 'Linux adapter verification job is missing'

perl -0pi -e 's/run: bash scripts\/smoke-darwin-unusable-lock\.sh/run: true/ or die "smoke gate mutation did not match\n"' .github/workflows/verify.yml
expect_named_failure 'remove native lock smoke' TestVerifyWorkflowPinsNativeProofGatesToTheirJobsAndPlatforms . 'native proof gates missing or misplaced: [Native macOS unusable-lock launch smoke]'

perl -0pi -e 's/run: bash scripts\/verify-native-mutations\.sh/run: true/g or die "mutation gate mutation did not match\n"' .github/workflows/verify.yml
expect_named_failure 'remove mutation gates' TestVerifyWorkflowPinsNativeProofGatesToTheirJobsAndPlatforms . 'native proof gates missing or misplaced: [Prove native acceptance tests detect broken guards'

# Half-close and admission regressions must fail their own assertions.
perl -0pi -e 's/return half\.CloseWrite\(\)/_ = half; return nil/ or die "half-close mutation did not match\n"' internal/server/lifecycle.go
expect_named_failure 'hide TCP half-close' TestHTTPRejectionsPreserveTCPHalfClose ./internal/server 'finalizing connection hid TCP CloseWrite'

perl -0pi -e 's/if !s\.resolving\.CompareAndSwap\(false, true\)/if false \&\& !s.resolving.CompareAndSwap(false, true)/ or die "resolution ownership mutation did not match\n"' selection_source.go
expect_named_failure 'admit multiple unresolved calls' TestSelectionResolutionHonoursAdmissionAndCancellation . 'retry admitted additional unresolved filesystem work'
