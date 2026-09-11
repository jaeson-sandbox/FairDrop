# Story 3.7 evidence

## Current verdict: independent review requires implementation loop 1

All three BMAD review layers completed after their first attempts failed with a usage-limit error (no findings existed from those failed attempts). Verification-gap returned no gaps. Blind Hunter returned ten findings; Edge Case Hunter returned the lock FIFO and TCP half-close findings independently. Deduplicated only those two exact claims/actions. The full 293,214-character cumulative diff from d43aa69db42c19324bae9f837b909649ca608099 was delivered as a temporary diff file, read completely by each reviewer; no reviewer edited the shared worktree.

| Finding | Severity | Route / action |
|---|---|---|
| Resolution happens before busy/shutdown admission and cannot observe cancellation | high | bad_spec: corrected non-frozen Code Map; stage-facing source decorator behind admission, bounded outstanding resolver, explicit cancellation tests |
| Lock FIFO can block startup; symlink/nonregular target not refused | high | patch carried through loop: nonblocking/no-follow regular-file probe, native fixtures |
| Connection wrapper hides TCP CloseWrite used by net/http | high | patch carried through loop: forward half-close and exercise real closing paths |
| Darwin snapshot can match recycled device/inode | medium | patch carried through loop: generation/birth fingerprint and unlink/recreate plus deterministic identity tests; honest limits |
| Logical ZIP size does not prove >32-bit archive offsets | medium | patch carried through loop: synthetic offset-threshold readback through production entry writer, distinguished from actual large-entry stream |
| New mutation/smoke gates not pinned in workflow tests | medium | patch carried through loop: job/platform-scoped pins and removal mutations |
| Mutation script accepts unrelated named failure and may accept timeout | medium | patch carried through loop: passing baseline, exact expected assertion, reject timeout/build/setup failure; verdict tests |
| Full-stack download helper never waits for natural terminal event | medium | patch carried through loop: matching Complete before cleanup; no cancellation counted as success |
| Ordinary options tests assume usable host lock and can panic | medium | patch carried through loop: deterministic option usability plus default-path wiring proof |
| Native names cover ancestors but not Unicode/space leaf metadata and archive names | medium | patch carried through loop: leaf fixtures and literal metadata/header/archive-name assertions |

The loop counter is now 1. Frozen intent remains approved and unchanged. Verified code is preserved in adc492cfb656f888c9a284c89df445086a326ab7; code changes are reverted for BMAD re-derivation with positive KEEP instructions in the spec. Documentation and evidence remain intact. The earlier green native gate below proves that checkpoint, not the upcoming review fixes. Story 3.7 is not done.

## Review loop 1 re-derivation (2026-09-11, verification in progress)

Reconstructed the KEEP implementation from adc492cfb656f888c9a284c89df445086a326ab7,
then corrected each of the ten routed findings. App delegates Stage unchanged;
the coordinator's SourcePort now resolves ancestors after admission, checks
cancellation, retains at most one unresolved OS call, and refuses busy retries
until that call returns. The stream retains the same raw inspector.
Darwin metadata identity compares device/inode/generation/birth timestamp across
both stat representations, with native unlink/recreate and deterministic recycled
identity coverage. The residual identical-fingerprint risk is documented; snapshots
do not claim inode ownership.

Darwin locking uses nonblocking/no-follow opens and a regular-descriptor gate
before flock, with independent production wiring and FIFO/symlink/kind tests.
The HTTP connection wrapper preserves TCP half-close, exercised by real unread
body and oversized-header requests. ZIP64 offset coverage uses a virtual prefix
and production entry writer; it checks central-directory and subsequent-entry
offsets plus CRC, and does not claim a multi-GiB compressed stream.
The full-stack matrix now checks returned metadata, Unicode/space leaf names,
HTTP attachment names, ZIP entry names and a matching natural Complete before
cleanup. Workflow tests pin proof gates to jobs/platforms and mutation verdicts
require an executed passing named baseline plus the intended assertion failure;
timeouts, panics, build/setup failures and skips are rejected.

Local Windows verification passes: Wails build, bindings/gitkeep, gofmt, vet,
staticcheck, ordinary Go suite, cgo=1 race suite (stream 263.358s), frontend
(17 files / 498 tests), and LF checks. Darwin arm64 build/vet/bare staticcheck
and Linux amd64 build/vet preflights pass; these are not native proof.
Local selected-symlink tests explicitly skip because this account lacks creation
privilege; local UNC fixtures are absent. CI requires those native capabilities.
Manual UI/focus/browser observations remain optional and unverified.

Ten local mutations each passed its exact named baseline and then failed its
specific assertion: early Complete, early 410 failure, ignored final short write,
concurrent file ownership, concurrent ZIP ownership, hidden TCP half-close and
multiple outstanding resolvers, removed Linux job, removed native lock smoke,
and removed mutation gates. Complete baseline/mutation transcripts are retained
under C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-7-review-loop-1.
Native-only mutations and corrected CI conclusions remain pending.

### Complete new ordinary-gate failure, corrected before retry

The existing composition test expected the raw inspector directly in the
coordinator. It now checks the decorator, its production resolver, and pointer
identity of the shared raw inspector held by staging and streaming.

```text
--- FAIL: TestComposeWiresTheSixRealAdapters (0.00s)
    app_test.go:350: coordinator field "source" holds *main.selectionSource, want *source.Inspector
2026/09/11 19:46:39 fairdrop: shutdown begin
FAIL
FAIL    fairdrop    0.393s
ok      fairdrop/internal/network    0.580s
ok      fairdrop/internal/qr    0.393s
ok      fairdrop/internal/server    4.621s
ok      fairdrop/internal/source    0.642s
ok      fairdrop/internal/stream    10.468s
ok      fairdrop/internal/transfer    0.746s
ok      fairdrop/scripts/mutationverdict    0.420s
FAIL
```

### Complete new foreign-preflight failure, corrected before retry

x/sys v0.46.0 calls Darwin birth metadata Btim; syscall calls it Birthtimespec.
The initial field spelling failed compilation and was corrected in production
and synthetic tests before the successful build/vet preflight.

```text
# fairdrop/internal/source
internal\source\handle_darwin.go:151:73: status.Birthtim undefined (type *"golang.org/x/sys/unix".Stat_t has no field or method Birthtim)
# fairdrop/internal/source
internal\source\handle_darwin.go:151:73: status.Birthtim undefined (type *"golang.org/x/sys/unix".Stat_t has no field or method Birthtim)
# fairdrop/internal/source
# [fairdrop/internal/source]
vet.exe: internal\source\handle_darwin.go:151:73: status.Birthtim undefined (type *unix.Stat_t has no field or method Birthtim)
```

Story acceptance and independent review remain pending. No manual observation or
foreign-platform preflight is counted as native execution.

## Prior implementation verdict: step 03 complete before independent review

Native Verify run [34656650584](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34656650584)
at `adc492cfb656f888c9a284c89df445086a326ab7` concluded **success**, with all three
job conclusions explicitly read: Windows `103450426742`, macOS `103450426600`,
Linux adapters `103450426700`. Both desktop jobs passed native Wails build,
bindings/gitkeep, formatting, vet/staticcheck, ordinary and cgo-backed race Go
suites, 17 frontend files / 498 tests, LF checks and native mutations. Linux passed
native Go vet, ordinary/race suites and mutations; it is not release proof.
Stream race durations: Windows 424.291s, macOS 524.325s, Linux 477.951s.

### Matrix test audit and deferred closure

All ten rows pass. Names below identify executable covering tests; the native
coverage logs were read, as were the unfiltered suite and mutation results.

| Row | Covering test(s) | Native evidence / closed id |
| --- | --- | --- |
| System aliases | `TestDarwinSystemTemporaryAncestorStageAndDownload`; `TestStageTransferResolvesAncestorsBeforeInspect` | macOS /tmp and /var/tmp file+folder transfers; boundary tests on all hosts. D-094 |
| Selected symlink refusal | `TestStageTransferRefusesSelectedSymlinkAndPreservesTraversalRefusals` | Windows/macOS/Linux pass; leaf-follow mutation killed on each. Part of D-094 |
| FIFO after metadata | `TestPOSIXContentOpenRefusesFIFOAfterMetadataWithoutBlocking`; `TestPOSIXContentOpenClearsNonblockingForRegularFile` | Native macOS/Linux, both guard mutations killed. D-076 |
| Non-reading metadata and identity | `TestLinuxMetadataHandleUsesOPathWithoutReadAccess`; `TestDarwinMetadataSnapshotNeedsNoContentPermission`; Darwin identity/no-follow/parent-relative tests; POSIX search-only tests | All relevant native tests pass, no permission capability skips. Four Darwin mutations killed. D-074 |
| Path classes | `TestNativePathClassesStageAndDownload`; `TestNativeUNCStageAndDownload`; `TestNativeEmptySelectionRemainsInvalid` | Spaces/Unicode/>260 file+folder on both desktops; real Windows SMB file+folder; POSIX coded UNC refusal, not a UNC success claim. D-007 |
| Portable name gate | `TestNativeChildNameRefusesReceiverVolumePrefixes`; existing archive-name suite | All hosts; reverting to host-dependent VolumeName killed on POSIX. D-084 |
| ZIP64 | `TestArchiveZIP64EntryCountReadsBackEveryEntry`; `TestArchiveZIP64FourGiBEntryAndTotalReadBack` | Full suites pass all hosts; Linux verbose log names both. 65,537 entries and 4 GiB+1 entry plus tail, contents/CRC verified. Logical total, not a >4 GiB compressed-offset claim. D-078 |
| Concurrent streaming | `TestWriteToConcurrentCallersStreamExactlyOnce` | Full native race suites; file and archive ownership mutations killed on all hosts. D-014 |
| Unusable native lock | `TestDarwinBuiltAppSurvivesUnusableLock`; Darwin preflight tests; `TestNativeSingleInstanceDegradationReportsNoPathOrCause` | Actual built macOS process survives with fixed diagnostic (2.32s); mode restored through held descriptor. No window/focus claim. D-089 |
| HTTP finalization | `TestNaturalCompletionWaitsForHTTPFinalization`; `TestPreparationFailureFinalizes410BeforeTerminalTeardown`; full App path tests | File/chunked delayed-write, failure, short-write, cancel, disconnect and 410 assertions; native suites and three native HTTP mutations. D-110 |

D-095 also closes: the independently pinned Linux-only adapter job now actually
executes O_PATH/POSIX behavior; deleting the job fails its named workflow test on
all hosts. These are ten distinct discharged ids: D-007, D-014, D-074, D-076,
D-078, D-084, D-089, D-094, D-095, D-110. The owning story already cites all ten.

Native mutation outcomes: 14 killed on macOS, 10 on Linux, 7 on Windows (shared
mutations repeat across platforms). No compiler error or unrelated timeout counts
as a kill. Local extra mutations pin final-error conversion and progress lease
release. Formal review has not yet run.

Skip audit: Windows skips macOS-only aliases; POSIX skips Windows-only namespace
forms. macOS's opt-in process smoke skips in ordinary suites but separately ran
and passed its dedicated CI step, whose survival marker is mandatory. Linux's
real-user-folder diagnostic is intentionally opt-in and unrelated to acceptance.
Local UNC/symlink privilege skips were replaced by successful native Windows CI
fixtures. No required row is closed by a skipped test. Manual UI/device checks
remain optional/unverified under the owner's policy.

Everything below is chronological checkpoint evidence, including failures that
precede this successful verdict; those failures are intentionally preserved.

## Owner-approved fix continuation — 2026-09-11

### Native harness correction and stronger once-only assertion

Third native run: `34656650584`, code checkpoint
`adc492cfb656f888c9a284c89df445086a326ab7`. The corrected macOS process smoke
passed and ordinary native Go gates passed; full native race/mutation conclusions
are still pending at this checkpoint. The story Code Map has been refreshed to
describe the implemented design and actual CI discoveries, not stale baseline
defects. No approved intent or matrix expectation changed.

Additional finalization mutation: suppressing conversion of an observed final
write error into ServerFailed causes all six file/chunked error, short-write and
disconnect cases to fail `failed final write was not reported as transfer_failed`.
The production file was restored byte-for-byte; five focused race repetitions
passed afterward. This pins error reporting as well as completion ordering.

Corrected-harness checkpoint local gate: Wails 4.946s; stable bindings/gitkeep;
gofmt/vet/staticcheck; seven Go packages (stream 10.204s); cgo=1; seven race
packages (stream 221.791s); 17 frontend files / 498 tests; LF/diff and foreign
Darwin/Linux build/vet plus Darwin bare staticcheck and root-test compilation:
all pass. The prior intermediate race run also passed (stream 224.766s).
The smoke script requires its actual survival marker, so a skipped/renamed test
cannot pass the CI step. Native execution of this corrected harness is pending.

Second-run conclusions were read explicitly: Windows success, macOS failure,
Linux failure. Windows includes the complete native/race/frontend/mutation gate;
the two failed jobs above are retained, not retried away as successful evidence.

Linux second-run race output below proves the ZIP64 suite finished (471.331s)
but revealed a pre-existing scheduling assumption in the coordinator test. The
observer records an event before its Publish returns, and `forwardProgress`
correctly returns the lease only after publication. `awaitEvents` is not a join.
The test now holds the second progress callback explicitly, asserts lease ownership
while held, releases it, then sends a foreign-session event over the unbuffered
lane as a drainer barrier before asserting release. This retains and strengthens
the postcondition; production coordinator code is unchanged. The test passed
1,000 race repetitions, then the transfer package passed ten race repetitions.
A mutation suppressing the second progress lease release fails the named test
with `the operation lease was not returned after a progress publication`.

```text
2026-09-11T22:47:55.6840656Z ##[group]Run test "$(go env CGO_ENABLED)" = 1
2026-09-11T22:47:55.6840809Z test "$(go env CGO_ENABLED)" = 1
2026-09-11T22:47:55.6840954Z go test -count=1 -race -timeout 1200s ./...
2026-09-11T22:47:55.6876804Z shell: /usr/bin/bash --noprofile --norc -e -o pipefail {0}
2026-09-11T22:47:55.6876888Z env:
2026-09-11T22:47:55.6876983Z   WAILS_VERSION: v2.15.0
2026-09-11T22:47:55.6877086Z   GOTOOLCHAIN: local
2026-09-11T22:47:55.6877171Z ##[endgroup]
2026-09-11T22:48:21.4296121Z ok  	fairdrop	2.458s
2026-09-11T22:48:21.4297046Z ok  	fairdrop/internal/network	1.020s
2026-09-11T22:48:21.4298061Z ok  	fairdrop/internal/qr	1.846s
2026-09-11T22:48:23.1290171Z ok  	fairdrop/internal/server	4.156s
2026-09-11T22:48:23.1291198Z ok  	fairdrop/internal/source	1.125s
2026-09-11T22:56:12.1487914Z ok  	fairdrop/internal/stream	471.331s
2026-09-11T22:56:12.1488915Z --- FAIL: TestProgressPublishesContiguousSequences (0.00s)
2026-09-11T22:56:12.1492026Z     coordinator_outcomes_test.go:46: the operation lease was not returned after a progress publication
2026-09-11T22:56:12.1492930Z FAIL
2026-09-11T22:56:12.1493353Z FAIL	fairdrop/internal/transfer	0.444s
2026-09-11T22:56:12.1493847Z FAIL
2026-09-11T22:56:12.2161709Z ##[error]Process completed with exit code 1.
```

Second CI checkpoint `1679a81c7548ce2e0c08eed6388a4531928e2156`, run
`34655474672`, macOS job `103446806340`, verified the fixture assumption was wrong:

```text
##[group]Run bash scripts/smoke-darwin-unusable-lock.sh
bash scripts/smoke-darwin-unusable-lock.sh
shell: /bin/bash --noprofile --norc -e -o pipefail {0}
env:
  WAILS_VERSION: v2.15.0
  GOTOOLCHAIN: local
##[endgroup]
Foundation ignored the isolated TMPDIR; no native lock fixture was modified
##[error]Process completed with exit code 1.
```

Foundation ignores the isolated TMPDIR on this runner, so the earlier smoke never
blocked the actual lock file. Replace that harness, not the working product: an
explicit GitHub-Actions-only Go test uses the production native path, opens the
regular runner-owned lock no-follow, takes a nonblocking exclusive lock, refuses
contention, saves its mode and temporarily denies permissions. It verifies the
permission refusal before launching the real built app. Cleanup stops only its
child with bounded TERM/KILL waits, restores the original mode through the same
descriptor, and closes it. No lock is removed or renamed; no developer machine
runs this opt-in mutation. Startup output is retained on failure. The temporary
Foundation probe source is removed; its diagnostic evidence above is preserved.

D-014 mutation audit found a false-green file assertion: removing the file CAS
still returned transfer_failed and zero output because the first caller had read
the small fixture to EOF. The folder CAS mutation already failed the named test.
The file test now counts reads on the actual prepared descriptor and proves the
loser never reads at all. Removing CAS now fails `concurrent second WriteTo read
the file instead of refusing ownership`. Restored test passed five race runs,
then a final race run. Both mutations are added to the native script. Production
code is restored; this corrects evidence, not a newly discovered product defect.

The owner approved fixing both blockers, choosing the working macOS permissions
approach, and making manual release observations optional for this personal
project. `docs/release-policy.md` records that decision; active planning/context
and top-level canonical documents point to it. Historical unperformed checks are
not rewritten as passes. Automated native gates and known-defect resolution remain
mandatory. D-110 moves from 3.8 to 3.7 with both deferred ownership and the epic
Closes line changed. The original baseline and the failed-checkpoint transcript
below remain intact.

### Implementation and reasoning

- HTTP: defer natural Complete until `net/http` finishes its actual connection
  writes, including the final zero chunk. Keep-alives are disabled, so StateClosed
  is the observation point; a wrapper records write errors and short writes.
  Request context is unsuitable because Go cancels it before finishRequest;
  cancellation uses the run context. Track the connection through terminal
  publication so Stop cannot close the event lane first. Payloads close before
  finalization; cancellation still force-closes a blocked final write. Prepare's
  empty 410 also finalizes before publishing its original failure. This is
  sender-observed transport success, not receiver saving/opening proof.
- Darwin: use parent-relative `fstatat(AT_SYMLINK_NOFOLLOW)` snapshots, separate
  search/enumeration/content opens and device/inode identity comparisons. A
  snapshot does not pin the leaf. Go's `os.SameFile` cannot compare custom FileInfo,
  so the Darwin identity helper accepts both unix and syscall stat representations.
  No private entitlement, new dependency or content permission is required for
  metadata. Linux O_PATH and all replacement/link/special-file guards remain.
- Native lock smoke observes a real process and fixed private warning, not window
  visibility. Its exact child receives TERM, a five-second grace, then KILL if
  necessary; cleanup no longer waits indefinitely for graceful termination.

### Focused verification, restored implementation

- `go test -count=40 -race -timeout 120s -run '^TestNativePathClassesStageAndDownload$' .`:
  PASS, `ok fairdrop 20.493s`, 240 real downloads. The earlier 54/240 failures below
  are preserved, not excluded from the record.
- `go test -count=10 -race ./internal/server`: PASS, 31.687s.
- Final focused server/source checks: PASS, 3.443s / 0.335s; focused vet/staticcheck,
  shell syntax and diff checks pass. Darwin/Linux source-test compilation and vet
  pass; these are preflight, not native execution.
- Early-terminal mutation: named finalization test fails `unexpected complete event`.
- Ignored-short-write mutation: same test fails `failed final write was not reported
  as transfer_failed`.
- Early-410 mutation: preparation-finalization test fails `unexpected failed event`.
  All three were restored. The native mutation script also covers metadata
  no-follow, representation compatibility, replacement identity and permission
  separation; native results remain required.

Current checkpoint: full sequential verification and native CI are underway;
formal BMAD review and the ten-id closure have not yet been claimed.

### Local full gate after fixes — 2026-09-11

Sequentially executed: Wails v2.15.0 production Windows build (5.303s), unchanged
generated bindings and restored `.gitkeep`, clean gofmt, vet and pinned staticcheck,
all seven Go packages without cache (stream 10.181s), cgo explicitly `1` with gcc
16.1.0, all seven Go packages under race (stream 219.317s), frontend 17 files / 498
tests, line-ending and diff checks. All passed. Foreign CGO-disabled Darwin arm64
and Linux amd64 builds/vet plus native-host bare staticcheck targeting Darwin passed.

Local verbose path coverage passed spaces/Unicode/>260 file and folder downloads,
empty selection, Windows namespace refusal and private lock diagnostic assertions.
UNC fixtures and directory-symlink privileges are unavailable locally, so those
three tests explicitly skipped; CI provisions UNC and requires symlink capability.
These skips do not close their matrix rows. No manual observation is claimed.

This is a verified local milestone, committed/pushed under AGENTS.md's handoff
rule so native CI can run. Story status stays in-progress pending actual native
results, full matrix acceptance and formal review.

## Resumption audit — 2026-09-11

### Native CI first fix checkpoint: not accepted

Before pushing the diagnostic/allowance correction, repeated the complete local
gate in order: Wails build 4.884s; stable bindings/gitkeep; gofmt/vet/staticcheck;
all seven ordinary Go packages (stream 10.405s); explicit cgo=1; all seven race
packages with the 1200s limit (stream 218.936s); 17 frontend files / 498 tests;
LF/diff checks and Darwin/Linux foreign preflights. All passed. The preceding
local race run also passed at 223.013s; no test expectation or product deadline
changed to accommodate the hosted-CPU cost.

Windows job `103443628170` likewise exhausted 420 seconds, this time actively
inflating the large entry during readback (after production writing completed).
Its full failed race-step output is retained below with runner prefix/timestamps
removed only. Final run conclusion and all three job conclusions were read as
failure, not inferred from a watch exit. No functional assertion failed before the
timeouts, but neither unfinished stream suite is counted as a pass.

```text
##[group]Run go test -count=1 -race -timeout 420s ./...
^[[36;1mgo test -count=1 -race -timeout 420s ./...^[[0m
shell: C:\Program Files\Git\bin\bash.EXE --noprofile --norc -e -o pipefail {0}
env:
  WAILS_VERSION: v2.15.0
  GOTOOLCHAIN: local
  FAIRDROP_TEST_UNC_FILE: \\localhost\FairDropNativeMatrix\report.txt
  FAIRDROP_TEST_UNC_DIRECTORY: \\localhost\FairDropNativeMatrix
##[endgroup]
ok  	fairdrop	3.520s
ok  	fairdrop/internal/network	1.095s
ok  	fairdrop/internal/qr	2.006s
ok  	fairdrop/internal/server	4.266s
ok  	fairdrop/internal/source	1.311s
panic: test timed out after 7m0s
	running tests:
		TestArchiveZIP64FourGiBEntryAndTotalReadBack (6m37s)

goroutine 61 [running]:
testing.(*M).startAlarm.func1()
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2802 +0x605
created by time.goFunc
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/time/sleep.go:215 +0x45

goroutine 1 [chan receive, 6 minutes]:
testing.(*T).Run(0xc000102000, {0x1404dccc4, 0x2c}, 0x1404e84e0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2109 +0xb56
testing.runTests.func1(0xc000102000)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2585 +0x85
testing.tRunner(0xc000102000, 0xc0000fbad0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
testing.runTests({0x1404cb33f, 0x8}, {0x1404d34e6, 0x18}, 0xc000096180, {0x14043f600, 0x4e, 0x4e}, {0xc2a13eca1f2dcf5c, 0x61ca4c03fd, ...})
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2583 +0x9f8
testing.(*M).Run(0xc000092460)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2443 +0xf4c
main.main()
	_testmain.go:202 +0x165

goroutine 50 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000102800)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestArchiveFailsASourceThatNeverProgresses(0xc000102800)
	D:/a/FairDrop/FairDrop/internal/stream/archive_stall_test.go:35 +0x3f
testing.tRunner(0xc000102800, 0x1404e84b8)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 51 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000102a00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestStreamPackageNeverWritesToDisk(0xc000102a00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_stall_test.go:91 +0x3f
testing.tRunner(0xc000102a00, 0x1404e8628)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 52 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000102c00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareDirectoryIsLazyAndReportsAnUnknownLength(0xc000102c00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:27 +0x3f
testing.tRunner(0xc000102c00, 0x1404e8540)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 53 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000102e00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory(0xc000102e00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:60 +0x3f
testing.tRunner(0xc000102e00, 0x1404e86a8)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 54 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000103000)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestStreamedArchiveOpensWithASecondImplementation(0xc000103000)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:99 +0x3f
testing.tRunner(0xc000103000, 0x1404e8630)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 55 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000103200)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToArchivesAnEmptyRootAsAFolder(0xc000103200)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:126 +0x3c
testing.tRunner(0xc000103200, 0x1404e8660)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 56 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000103400)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream(0xc000103400)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:140 +0x3c
testing.tRunner(0xc000103400, 0x1404e8648)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 57 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000103600)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToPropagatesAWalkFailureWithoutAppendingToTheBody(0xc000103600)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:178 +0x3f
testing.tRunner(0xc000103600, 0x1404e86b0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 7 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444000)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToClosesEveryBorrowedEntryBeforeReturning(0xc001444000)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:263 +0x3f
testing.tRunner(0xc001444000, 0x1404e8670)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 8 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444200)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToRefusesASecondCallAndACallAfterClose(0xc001444200)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:289 +0x3f
testing.tRunner(0xc001444200, 0x1404e86c0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 9 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444400)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestCloseIsSafeConcurrentlyForADirectoryPayload(0xc001444400)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:324 +0x3f
testing.tRunner(0xc001444400, 0x1404e84e8)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 10 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444600)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToStopsPromptlyWhenTheReceiverDisconnects(0xc001444600)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:352 +0x3f
testing.tRunner(0xc001444600, 0x1404e8700)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 11 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444800)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestWriteToRejectsMissingContextOrDestinationForADirectory(0xc001444800)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:383 +0x2f
testing.tRunner(0xc001444800, 0x1404e86d0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 12 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444a00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot(0xc001444a00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:392 +0x3f
testing.tRunner(0xc001444a00, 0x1404e84b0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 13 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444c00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestArchiveRefusesAnEntryNameTheSourceShouldNeverEmit(0xc001444c00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:427 +0x2f
testing.tRunner(0xc001444c00, 0x1404e84c0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 14 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001444e00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestArchiveDownloadNameIsCappedAfterTheExtensionIsAppended(0xc001444e00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:440 +0x3f
testing.tRunner(0xc001444e00, 0x1404e84a8)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 15 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445000)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareRejectsARootThatIsNoLongerADirectory(0xc001445000)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:490 +0x3f
testing.tRunner(0xc001445000, 0x1404e85a8)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 16 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445200)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareRejectsARootThatDisappeared(0xc001445200)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:507 +0x3f
testing.tRunner(0xc001445200, 0x1404e85a0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 66 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445400)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareRejectsALinkLikeRootWithPathUnsupported(0xc001445400)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:525 +0x3f
testing.tRunner(0xc001445400, 0x1404e8598)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 67 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445800)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareRejectsALinkLikeFileRootWithPathUnsupported(0xc001445800)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:557 +0x3f
testing.tRunner(0xc001445800, 0x1404e8590)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 68 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445a00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestPrepareHonorsCancellationForADirectory(0xc001445a00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:584 +0x3f
testing.tRunner(0xc001445a00, 0x1404e8570)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 69 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc001445c00)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:1803 +0x4ef
fairdrop/internal/stream.TestArchiveStreamingErrorsDoNotDiscloseTheSourcePath(0xc001445c00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_test.go:593 +0x3f
testing.tRunner(0xc001445c00, 0x1404e84d0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b

goroutine 72 [runnable]:
compress/flate.(*decompressor).huffSym(0xc00323c008, 0xc00323c038)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/compress/flate/inflate.go:708 +0x52a
compress/flate.(*decompressor).huffmanBlock(0xc00323c008)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/compress/flate/inflate.go:495 +0x99
compress/flate.(*decompressor).Read(0xc00323c008, {0xc00324e000, 0x8000, 0xc00324e000?})
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/compress/flate/inflate.go:348 +0xba
archive/zip.(*pooledFlateReader).Read(0xc000be0018, {0xc00324e000, 0x8000, 0x8000})
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/archive/zip/register.go:89 +0x1cb
archive/zip.(*checksumReader).Read(0xc0030d6000, {0xc00324e000, 0x8000, 0x8000})
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/archive/zip/reader.go:299 +0xad
io.copyBuffer({0x1404ed1a0, 0x14075d120}, {0x47bd4b68, 0xc0030d6000}, {0x0, 0x0, 0x0})
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/io/io.go:429 +0x271
io.Copy(...)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/io/io.go:388
fairdrop/internal/stream.TestArchiveZIP64FourGiBEntryAndTotalReadBack(0xc000103c00)
	D:/a/FairDrop/FairDrop/internal/stream/archive_zip64_test.go:119 +0x1130
testing.tRunner(0xc000103c00, 0x1404e84e0)
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2036 +0x1cb
created by testing.(*T).Run in goroutine 1
	C:/hostedtoolcache/windows/go/1.26.7/x64/src/testing/testing.go:2101 +0xb2b
FAIL	fairdrop/internal/stream	420.086s
ok  	fairdrop/internal/transfer	1.509s
FAIL
##[error]Process completed with exit code 1.
```

Linux's race suite timed out at 420 seconds with its producer runnable inside
`compress/flate.findMatch`, processing the real 4 GiB fixture. The first five
packages and transfer passed; the stream suite did not finish and is not accepted.
Increase only the test-process allowance to a bounded 1200 seconds under the
existing 30-minute job limit; retain the real >4 GiB stream, content/CRC readback,
race instrumentation and all assertions. Workflow literal pins and spec commands
move together. Full failed-step output (timestamps stripped only):

```text
##[group]Run test "$(go env CGO_ENABLED)" = 1
test "$(go env CGO_ENABLED)" = 1
go test -count=1 -race -timeout 420s ./...
shell: /usr/bin/bash --noprofile --norc -e -o pipefail {0}
env:
  WAILS_VERSION: v2.15.0
  GOTOOLCHAIN: local
##[endgroup]
ok  	fairdrop	2.465s
ok  	fairdrop/internal/network	1.018s
ok  	fairdrop/internal/qr	1.808s
ok  	fairdrop/internal/server	4.156s
ok  	fairdrop/internal/source	1.120s
panic: test timed out after 7m0s
	running tests:
		TestArchiveZIP64FourGiBEntryAndTotalReadBack (5m43s)

goroutine 11 [running]:
testing.(*M).startAlarm.func1()
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2802 +0x605
created by time.goFunc
	/opt/hostedtoolcache/go/1.26.7/x64/src/time/sleep.go:215 +0x45

goroutine 1 [chan receive, 5 minutes]:
testing.(*T).Run(0xc000146248, {0x82c28e, 0x2c}, 0x8375f8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2109 +0xb3e
testing.runTests.func1(0xc000146248)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2585 +0x85
testing.tRunner(0xc000146248, 0xc000153ad0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
testing.runTests({0x81d25d, 0x8}, {0x823a70, 0x18}, 0xc000016108, {0xab3720, 0x4e, 0x4e}, {0xc2a13eb7d06ea157, 0x61ca00adb5, ...})
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2583 +0x9ea
testing.(*M).Run(0xc000108460)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2443 +0xf4c
main.main()
	_testmain.go:202 +0x165

goroutine 34 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0001466c8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestArchiveFailsASourceThatNeverProgresses(0xc0001466c8)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_stall_test.go:35 +0x3f
testing.tRunner(0xc0001466c8, 0x8375d0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 35 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000146908)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestStreamPackageNeverWritesToDisk(0xc000146908)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_stall_test.go:91 +0x3f
testing.tRunner(0xc000146908, 0x837740)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 36 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000146b48)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareDirectoryIsLazyAndReportsAnUnknownLength(0xc000146b48)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:27 +0x3f
testing.tRunner(0xc000146b48, 0x837658)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 37 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000146d88)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory(0xc000146d88)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:60 +0x3f
testing.tRunner(0xc000146d88, 0x8377c0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 38 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000146fc8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestStreamedArchiveOpensWithASecondImplementation(0xc000146fc8)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:99 +0x3f
testing.tRunner(0xc000146fc8, 0x837748)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 39 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000147208)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToArchivesAnEmptyRootAsAFolder(0xc000147208)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:126 +0x3c
testing.tRunner(0xc000147208, 0x837778)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 40 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000147448)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream(0xc000147448)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:140 +0x3c
testing.tRunner(0xc000147448, 0x837760)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 41 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000147688)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToPropagatesAWalkFailureWithoutAppendingToTheBody(0xc000147688)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:178 +0x3f
testing.tRunner(0xc000147688, 0x8377c8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 46 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000147b08)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToClosesEveryBorrowedEntryBeforeReturning(0xc000147b08)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:263 +0x3f
testing.tRunner(0xc000147b08, 0x837788)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 47 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc000147d48)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToRefusesASecondCallAndACallAfterClose(0xc000147d48)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:289 +0x3f
testing.tRunner(0xc000147d48, 0x8377d8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 48 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2008)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestCloseIsSafeConcurrentlyForADirectoryPayload(0xc0011a2008)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:324 +0x3f
testing.tRunner(0xc0011a2008, 0x837600)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 49 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2248)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToStopsPromptlyWhenTheReceiverDisconnects(0xc0011a2248)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:352 +0x3f
testing.tRunner(0xc0011a2248, 0x837818)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 50 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2488)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestWriteToRejectsMissingContextOrDestinationForADirectory(0xc0011a2488)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:383 +0x2f
testing.tRunner(0xc0011a2488, 0x8377e8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 51 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a26c8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot(0xc0011a26c8)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:392 +0x3f
testing.tRunner(0xc0011a26c8, 0x8375c8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 52 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2908)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestArchiveRefusesAnEntryNameTheSourceShouldNeverEmit(0xc0011a2908)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:427 +0x2f
testing.tRunner(0xc0011a2908, 0x8375d8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 53 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2b48)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestArchiveDownloadNameIsCappedAfterTheExtensionIsAppended(0xc0011a2b48)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:440 +0x3f
testing.tRunner(0xc0011a2b48, 0x8375c0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 54 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2d88)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareRejectsARootThatIsNoLongerADirectory(0xc0011a2d88)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:490 +0x3f
testing.tRunner(0xc0011a2d88, 0x8376c0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 55 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a2fc8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareRejectsARootThatDisappeared(0xc0011a2fc8)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:507 +0x3f
testing.tRunner(0xc0011a2fc8, 0x8376b8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 56 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a3208)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareRejectsALinkLikeRootWithPathUnsupported(0xc0011a3208)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:525 +0x3f
testing.tRunner(0xc0011a3208, 0x8376b0)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 57 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a3448)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareRejectsALinkLikeFileRootWithPathUnsupported(0xc0011a3448)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:557 +0x3f
testing.tRunner(0xc0011a3448, 0x8376a8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 58 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a3688)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestPrepareHonorsCancellationForADirectory(0xc0011a3688)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:584 +0x3f
testing.tRunner(0xc0011a3688, 0x837688)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 59 [chan receive, 6 minutes]:
testing.(*T).Parallel(0xc0011a38c8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:1803 +0x50c
fairdrop/internal/stream.TestArchiveStreamingErrorsDoNotDiscloseTheSourcePath(0xc0011a38c8)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:593 +0x3f
testing.tRunner(0xc0011a38c8, 0x8375e8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 21 [select]:
io.(*pipe).read(0xc002b7d080, {0xc0001cc000, 0x20000, 0x4c3fc9?})
	/opt/hostedtoolcache/go/1.26.7/x64/src/io/pipe.go:57 +0x145
io.(*PipeReader).Read(0xc002b7d080, {0xc0001cc000, 0x20000, 0x20000})
	/opt/hostedtoolcache/go/1.26.7/x64/src/io/pipe.go:134 +0x47
fairdrop/internal/stream.(*archive).drain(0xc000148080, {0x83dbc0, 0xada440}, {0x83bd00, 0xc002967bc0}, {0x83be60, 0xc002b7d080})
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:166 +0x151
fairdrop/internal/stream.(*archive).WriteTo(0xc000148080, {0x83dbc0, 0xada440}, {0x83bd00, 0xc002967bc0})
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:98 +0x487
fairdrop/internal/stream.TestArchiveZIP64FourGiBEntryAndTotalReadBack(0xc0011a3d48)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_zip64_test.go:85 +0x15d
testing.tRunner(0xc0011a3d48, 0x8375f8)
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2036 +0x21d
created by testing.(*T).Run in goroutine 1
	/opt/hostedtoolcache/go/1.26.7/x64/src/testing/testing.go:2101 +0xb13

goroutine 22 [runnable]:
compress/flate.(*compressor).findMatch(0xc00020c000, 0xea21, 0xea20, 0x3, 0x15df)
	/opt/hostedtoolcache/go/1.26.7/x64/src/compress/flate/deflate.go:233 +0x63d
compress/flate.(*compressor).deflate(0xc00020c000)
	/opt/hostedtoolcache/go/1.26.7/x64/src/compress/flate/deflate.go:439 +0x7e5
compress/flate.(*compressor).write(0xc00020c000, {0xc0001ec000, 0x20000, 0x20000})
	/opt/hostedtoolcache/go/1.26.7/x64/src/compress/flate/deflate.go:547 +0xdd
compress/flate.(*Writer).Write(...)
	/opt/hostedtoolcache/go/1.26.7/x64/src/compress/flate/deflate.go:709
archive/zip.(*pooledFlateWriter).Write(0xc002a00020, {0xc0001ec000, 0x20000, 0x20000})
	/opt/hostedtoolcache/go/1.26.7/x64/src/archive/zip/register.go:51 +0x1c5
archive/zip.(*countWriter).Write(...)
	/opt/hostedtoolcache/go/1.26.7/x64/src/archive/zip/writer.go:647
archive/zip.(*fileWriter).Write(0xc00076e050, {0xc0001ec000, 0x20000, 0x20000})
	/opt/hostedtoolcache/go/1.26.7/x64/src/archive/zip/writer.go:579 +0x202
fairdrop/internal/stream.writeArchiveFile({0x83dbc0, 0xada440}, 0xc00076e000, {0xc002ad6032, 0xe}, {0xc00007b808?, 0xc002ac4048?, 0x0?}, {0x83baa0, 0xc002ac4048}, ...)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:332 +0x4a6
fairdrop/internal/stream.(*archive).writeEntries.func1({{0x81d859, 0x9}, {0x81c4b9, 0x4}, 0x100000001, {0x0, 0x0, 0x0}}, {0x83baa0, 0xc002ac4048})
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:221 +0x1cf
fairdrop/internal/stream.TestArchiveZIP64FourGiBEntryAndTotalReadBack.func1({0x40?, 0xc00001ab80?}, {0xc00001ab80?, 0xc0001c7e50?}, 0xc00001ab80)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_zip64_test.go:78 +0x138
fairdrop/internal/stream.(*scriptedSource).Walk(0xc002b5e9e0, {0x83dbc0, 0xada440}, {0xc00002a140, 0x44}, 0xc00001ab80)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive_test.go:632 +0x90
fairdrop/internal/stream.(*archive).writeEntries(0xc000148080, {0x83dbc0, 0xada440}, 0xc00076e000)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:209 +0x327
fairdrop/internal/stream.(*archive).produce(0xc000148080, {0x83dbc0, 0xada440}, 0xc002b7d080)
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:138 +0x2f5
fairdrop/internal/stream.(*archive).WriteTo.func1()
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:96 +0x5e
created by fairdrop/internal/stream.(*archive).WriteTo in goroutine 21
	/home/runner/work/FairDrop/FairDrop/internal/stream/archive.go:96 +0x445
FAIL	fairdrop/internal/stream	420.045s
ok  	fairdrop/internal/transfer	1.423s
FAIL
##[error]Process completed with exit code 1.
```

Commit `092900b764322f3ae7dde5420fba9144a701ab41`, run
`34654431420`, macOS job `103443628144` (macOS 26.6.2 / arm64) built successfully.
The launch-smoke failed without its expected warning while the child stayed alive.
Full failed-step output follows; this is not yet attributed to a product or fixture
cause. The harness now verifies its TMPDIR with the same Foundation API before
creating the lock blocker, and preserves blank-app startup output plus a bounded
process sample on failure. No new product behavior or weakened assertion.

```text
##[group]Run bash scripts/smoke-darwin-unusable-lock.sh
2026-09-11T22:33:32.5814480Z bash scripts/smoke-darwin-unusable-lock.sh
2026-09-11T22:33:32.7168710Z shell: /bin/bash --noprofile --norc -e -o pipefail {0}
2026-09-11T22:33:32.7169210Z env:
2026-09-11T22:33:32.7169680Z   WAILS_VERSION: v2.15.0
2026-09-11T22:33:32.7170060Z   GOTOOLCHAIN: local
2026-09-11T22:33:32.7170290Z ##[endgroup]
2026-09-11T22:33:52.7627910Z Native launch did not report the degraded-lock diagnostic
2026-09-11T22:33:53.0314710Z ##[error]Process completed with exit code 1.
2026-09-11T22:33:53.1302320Z
```

The always-run native coverage and mutation steps still executed: Darwin metadata
permission/identity/no-follow/parent-relative/mode tests, POSIX guards and system
alias transfers passed without capability skips. All twelve native mutations were
killed by named test failures, including all four new Darwin metadata mutations.
This is evidence for those guards, not acceptance of the failed macOS full gate.


Baseline: `d43aa69db42c19324bae9f837b909649ca608099` on
`epic-3-run-reliably-on-supported-desktops`, clean and matching origin before
this audit. Main is `627191466ebd0a14b17aeddf5e576cf19a2e703d`.
This is a resumption of an approved story, not a new architecture or scope decision.

### Current project state

- Sprint tracking records Epics 1 and 2 and their retrospectives done. Epic 3
  Stories 3.1–3.6 are done; 3.7 is in progress with an approved ready-for-dev
  spec, and 3.8–3.12 are backlog. The spec now records its implementation baseline.
- Stories 3.1–3.3 supplied single-instance wiring, native verification and release
  artifacts. Stories 3.4–3.6 bounded lifecycle waits with honest failure diagnostics,
  revised public error copy, and made lost/malformed events observable. Existing
  consumer-owned ports, session/sequence validation, and file/folder streaming
  remain the baseline; do not resurrect Phase 1 interfaces.
- GitHub Verify run `34648696619` at this exact HEAD is successful. Its conclusion
  and both Windows/macOS job conclusions were read explicitly. The native jobs
  already execute build, bindings checks, vet/staticcheck, ordinary and race Go
  tests, the frontend suite, and line-ending checks. No fresh local baseline run
  is claimed. Linux native adapter execution is still absent.
- Local tools: Go 1.26.7 windows/amd64; cgo enabled (`1`); Node v24.15.0 satisfies
  `.nvmrc` major `24`; Wails, staticcheck, gcc and gh are available. The module
  floor is Go 1.26.0. `.gitattributes` now normalizes text to LF, superseding old
  assumptions that a normal checkout necessarily contains CRLF.
- `gh release list` reports published latest `v0.1.0` on 2026-09-10. The successful
  release workflow run `34540749917` used `cc9ce580aa63281b5bd8cc4c6de685b0ac79068f`,
  not this branch's current HEAD. Publication is not acceptance evidence for this
  checkpoint. No release or tag was modified during this audit.
- `release-evidence.md` still lacks the receiver OS/browser and confirmation that
  the folder ZIP opened. Native second-launch restoration/accessibility evidence
  is pending. The September 4 phone failure remains unexplained, not fixed.
  These are Story 3.9's human evidence obligations, not automated passes.

### Reconciled drift and reasoning

- Epic context still said eleven stories and omitted the approved 3.12 split.
  The current epics and sprint file agree on twelve. Preserve numbering: 3.10,
  3.11 and 3.12 must finish before 3.9's release evidence, despite their numbers.
- The approved 3.7 scope includes two product fixes (ancestor-only path resolution
  and Darwin lock preflight), so the old context's "no product behaviour" summary
  was inaccurate. Approval remains authoritative; the frozen spec is unchanged.
- The frozen problem statement and old Code Map said POSIX tests never execute.
  Current macOS CI disproves that blanket claim. The implementation gap is Linux
  execution and specific missing platform assertions. Corrected outside the
  frozen section so a new agent does not reason from obsolete evidence.
- Context's twelve-code count was stale: the domain registry now has thirteen,
  including `shutting_down`. Refer to the synchronized registry, not a historic
  count. The residual public-copy work stays with 3.11.
- Context also claimed Stop was quiescent on every return, contradicting 3.4:
  successful teardown proves quiescence; an elapsed bound reports coded failure
  and never claims the resource stopped. Preserve that distinction.

### Scope and acceptance guardrails

Story 3.7 owns D-007, D-014, D-074, D-076, D-078, D-084, D-089, D-094 and D-095.
All nine remain open until implemented and evidenced. D-065/D-068 belong to 3.12;
D-099/D-102 to 3.8. No dependencies, public strings, release promises or refusal
semantics beyond the approved ancestor-resolution decision are silently added.
Native-only rows require native execution; cross-builds and skipped tests do not
close them. Windows UNC syntax and macOS mounted network paths must be described
honestly, without treating a host-inapplicable case as a successful transfer.

BMAD Build step 03 requires a context-free implementation handoff, followed by
matrix audit and adversarial review. Gate execution is sequential per AGENTS.md;
in particular Wails build precedes, and never overlaps, frontend tests. Persist
implementation, mutation and review findings here for the next agent.

### Recovered id-less findings

Two legacy entries still named the completed Story 3.2 without an id. Its evidence
section 4 explicitly left them alone because they were not among its named ids.
The owner/citation test silently skipped `id == ""`, so green tracking tests did
not mean all findings were routed. Entry parsing now validates each record's
unique D-NNN id and single owner before checking citations, independently of field
order. Before repairing the records, the focused test failed with:

```text
--- FAIL: TestEveryOpenDeferredEntryIsCitedByItsOwningStory (0.00s)
    main_test.go:691: deferred entry 69 must have exactly one stable D-NNN id (got "", count 0)
FAIL
FAIL fairdrop 0.075s
```

- D-108: the lost-failure-output finding is discharged by Story 3.2's direct Go
  test commands and retained CI job logs (no tail/truncation pipeline). This closes
  the evidence-retention requirement, **not** the cause of the unreproduced 1.10
  failure; no root cause is claimed. Original observation is preserved unchanged.
- D-109: Story 1.10's missing Blind Hunter and verification-gap layers remain
  outstanding. Story 3.2's section 6 reviews **1.6**, so it does not discharge this
  entry. Re-owned to 3.12, cited in its Closes line and explicit acceptance row.
  This is recovery of existing review debt, not a new product feature.

## Implementation checkpoint — not accepted

BMAD step 03 remains incomplete. The implementation agent added ancestor-only
resolution, portable entry-name refusal, Darwin lock preflight, Linux adapter CI,
native path/rights tests, concurrent WriteTo tests, ZIP64 readback, native mutation
scripts and a macOS process-survival smoke. Parent independently inspected the
boundary changes and ran the full local gate sequentially. No story-owned id is
discharged yet; no formal step-04 review, commit, push, or new native CI run occurred.

### Local gate (2026-09-11)

| Check | Result |
| --- | --- |
| Wails v2.15.0 production Windows build | pass, 5.333 s; bindings regenerated |
| Bindings drift / restored `.gitkeep` | pass |
| gofmt, vet, pinned staticcheck | pass |
| `go test -count=1 -timeout 240s ./...` | all 7 packages pass; stream 9.799 s |
| cgo / compiler | `1`; gcc 16.1.0 UCRT POSIX |
| `go test -count=1 -race -timeout 420s ./...` | **FAIL**: real HTTP folder body incomplete; other packages passed, stream 205.197 s |
| Frontend | 17 files / 498 tests pass |
| LF / `git diff --check` | pass |
| Darwin arm64 + Linux amd64 build/vet, Darwin bare staticcheck | pass with CGO_ENABLED=0; type checking only, **not** native or Foundation bridge proof |

The initial full race failure is preserved here in full, with its final package
output (no race detector warning was emitted; the test failed under race scheduling):

```text
2026/09/11 17:52:25 fairdrop: shutdown begin
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:148: native download incomplete: status=200
FAIL
FAIL fairdrop 1.323s
ok fairdrop/internal/network 1.168s
ok fairdrop/internal/qr 1.460s
ok fairdrop/internal/server 4.300s
ok fairdrop/internal/source 1.356s
ok fairdrop/internal/stream 205.197s
ok fairdrop/internal/transfer 1.522s
FAIL
```

### Response-finalization defect (D-110)

Repeated-run tally: 30 of 40 iterations failed; 54 of 240 downloads ended in
unexpected EOF (48 folder ZIPs, 6 files). Every recorded failure received HTTP 200
but zero body bytes before EOF. This tally is observational, not a probabilistic
test assertion or a claim that every premature close has the same result.

The new full-stack test drives App → coordinator → real source/payload/server over
HTTP. The initial assertion obscured the body-read error, so the parent improved
it to record status, byte count, error type and `errors.Is(io.ErrUnexpectedEOF)`
without a selected path or capability URL. Repeated execution reproduces premature
EOF on **files and folders**, across path classes; it is not a long-path-specific
filesystem defect. The complete repeated-run output is retained below.

Read-only diagnosis by the implementer, independently checked by the parent:
`internal/server/handler.go` closes the payload and calls `r.finish(ServerComplete)`
before returning. Coordinator terminal handling calls Stop; `run.teardown` calls
`http.Server.Close` before awaiting quiescence. Go 1.26.7's `net/http/server.go`
calls `response.finishRequest` only **after** ServeHTTP returns; that flushes the
body, closes chunk framing, and flushes the connection. Thus a completion can
cause forced socket closure while response bytes are still buffered. Existing
server success tests read the entire response before calling Stop, excluding this
ordering from their checks. A handler-level Flush alone does not close chunk
framing and is not a sufficient fix.

Next reproduction should be deterministic: wrap an accepted connection, pass the
initial headers, block its next body/framing write, and observe that Complete has
already been published and Stop closes it. Preserve forced-close cancellation and
failure semantics when fixing natural-completion ordering. This is routed to 3.8
because the defect lives in server/coordinator teardown, outside 3.7's approved
Code Map. It blocks 3.7's matrix acceptance too: re-owning it does not make the
failing HTTP assertion pass. No link to the earlier phone failure is proven.

### macOS metadata rights — native decision still pending (D-074)

The new Darwin test retains the approved expectation that metadata inspection
does not require or grant read access. Apple's kernel source contradicts the
existing O_EVTONLY assumption unless a private process policy is enabled. Parent
independently checked these primary sources at XNU commit
`f6217f891ac0bb64f3d375211650a4c1ff8ca1ea`:

- [open flags / conditional read-right removal](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/vfs/vfs_syscalls.c#L4609).
- [private entitlement required to enable that policy](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_resource.c#L2913).

This is a source-backed concern, **not** a claimed native failure: no new macOS
runner has executed this checkpoint. Do not weaken the test, enable a private
entitlement, or redesign metadata silently. A parent-relative no-follow metadata
query is a candidate design to investigate, not an approved implementation. The
story's Ask First gate covers changes to refusal/rights semantics beyond ancestor
resolution. Obtain the human decision and native evidence before closing D-074.

### Remaining acceptance evidence

- Linux/Darwin native filesystem assertions, capability-skip report, SMB setup,
  mutation scripts and Foundation compilation are pending CI. Local symlink/UNC
  tests skip for missing privileges/fixtures; this is not a pass for those rows.
- ZIP64 entry count and >4 GiB logical-entry/total readback pass locally, including
  the full race package run. A >4 GiB **compressed archive offset** was not tested.
- Concurrent file and archive WriteTo tests passed under race.
- macOS smoke asserts a real built process survives with the fixed degraded-lock
  diagnostic; even when it runs, it does not observe a visible window or focus.
- Selected leaf / traversal refusal mutations and missing-Linux-job mutation have
  not executed. Only tracking's original missing-id failure has been demonstrated.
- The preflight releases its probe before Wails acquires the lock; the remaining
  TOCTOU window is documented, not eliminated.

## Full targeted reproduction output

Tracking hardening was also mutation-checked: temporarily changing D-109 to
D-108 made `TestEveryOpenDeferredEntryIsCitedByItsOwningStory` fail with
`duplicate deferred id D-108`. The mutation was restored and the focused deferred
tests passed. The original id-less records supplied the missing-id red test.

### Resume instructions

1. Read this evidence and the in-progress spec; all changes are local and
   uncommitted on the original epic branch. Nothing has been pushed or released.
2. Obtain approval to bring the D-110 fix forward and investigate/revise Darwin
   metadata acquisition without weakening no-follow/no-read guarantees. Existing
   D-074 and newly routed D-110 remain open; no new pass may paper over them.
3. Add deterministic response-finalization coverage, then fix ordering; a retry,
   delay, handler-only Flush or moving publication into a handler defer is not
   proof of final HTTP framing. Re-run the new full-stack matrix and full gate.
4. Resolve Darwin's design decision in the spec/architecture/contract together,
   then obtain actual Windows/macOS/Linux native results, including all capability
   skips and native mutations. Cross-compilation cannot answer the rights question.
5. Complete step-03 matrix audit before step-04 adversarial review. Do not mark
   3.7 done or merge to main before acceptance. The human release evidence and
   Story 1.10 review debt are still open under their recorded owners.

Command: `go test -count=40 -race -timeout 120s -run '^TestNativePathClassesStageAndDownload$' .`

```text
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.42s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.42s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.43s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.47s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.46s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.49s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.17s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.47s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.46s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.49s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
FAIL
FAIL	fairdrop	18.599s
FAIL
```
