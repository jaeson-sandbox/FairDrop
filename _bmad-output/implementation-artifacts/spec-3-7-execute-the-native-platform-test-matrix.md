---
title: 'Story 3.7: Execute the Native Platform Test Matrix'
type: 'chore'
created: '2026-09-11'
status: 'in-progress'
baseline_commit: 'd43aa69db42c19324bae9f837b909649ca608099'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Native coverage gaps hide filesystem and transfer failures. Windows/macOS CI exists, but Linux adapter execution and specific filesystem/archive assertions are missing. macOS selections under symlinked system directories and unusable lock files need boundary fixes. The new real HTTP matrix also reproduced premature response shutdown (D-110), and Darwin's O_EVTONLY is not the assumed non-reading metadata capability (D-074).

**Approach:** Execute native adapter/path/archive tests, fix selection and lock boundaries, and hold natural completion until HTTP finalization. Use supported parent-relative no-follow metadata queries on Darwin, retaining Linux O_PATH and existing traversal/identity/content guards. On 2026-09-11 the owner approved fixing both newly surfaced blockers and choosing the working macOS approach. Manual release observations are optional for this personal project; automated gates and truthful limits remain mandatory.

## Boundaries & Constraints

**Always:** A guarantee that depends on an operating system is asserted on that operating system. The Linux job names itself adapter verification, never release proof. The traversal's refusal of link-like components is unchanged and stays pinned: what changes is where a path is resolved, not what the walker accepts. AD-9 holds — no test may put a real selected path or token into diagnostics.

**Ask First:** Refusal-policy changes beyond approved ancestor resolution and replacing Darwin metadata acquisition without read access. New CI dependencies or runner images beyond the approved Linux job. New public error codes or UI copy remain Story 3.11's; fixed private diagnostics are allowed.

**Never:** Weaken a guard to make a native run pass; a failure there is the finding. Let a Linux result stand in for Windows or macOS release proof. Run `wails build` on Linux. Decide D-065 or D-068 — those are Story 3.12's.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| File under a symlinked system directory | `/tmp/x/report.pdf` on macOS | Staged and transferred | Resolved at entry (D-094) |
| Selection that *is* a symlink | a link the user picked | Still refused | `path_unsupported`, unchanged |
| FIFO swapped in after metadata | POSIX content open | Refused without blocking | `path_unsupported` (D-076) |
| Metadata inspection | POSIX, no file read rights | Linux O_PATH or Darwin parent-relative no-follow stat succeeds without opening content; identity checks stay effective | Coded (D-074) |
| Path over 260 chars, non-ASCII, spaces; Windows UNC | Native supported path forms on Windows/macOS; UNC syntax is Windows-only | Staged and streamed end to end; POSIX rejects Windows UNC syntax with a coded error | Coded per class (D-007) |
| Windows-shaped entry name on a POSIX sender | `C:evil.txt` | Refused by both name gates | `path_unsupported` (D-084) |
| Archive past the ZIP64 thresholds | >65,535 entries; a 4 GiB entry | Read back and validated | Coded (D-078) |
| Two concurrent `WriteTo` calls | one prepared payload | Exactly one streams | Second refused (D-014) |
| Lock file unopenable for any reason but contention | macOS, unusable lock path | Built process remains running and emits a fixed degraded-protection diagnostic | Never a silent exit; visual/focus observation optional (D-089) |
| Natural response finalization delayed or fails | File or chunked ZIP, coordinator consumes terminal event immediately | No Complete before final framing/write succeeds; exact file/valid ZIP received; write failure reports failure | Cancel/Stop remain bounded and force-closing (D-110) |

</frozen-after-approval>

## Code Map

- `internal/server/{handler,lifecycle}.go` — success currently publishes Complete before ServeHTTP returns; coordinator then force-closes before net/http's finishRequest flushes body/framing. Fix actual finalization ordering, not a timing delay or handler-only Flush. Keep cancellation/failure force-close behavior. Test delayed final writes, final-write errors, early disconnect and cancellation deterministically through the listener/connection seam; real App/coordinator HTTP tests must pass repeatedly under race.
- `internal/source/{handle,handle_posix,handle_darwin,source}.go` — replace Darwin O_EVTONLY with parent-relative `fstatat(AT_SYMLINK_NOFOLLOW)` metadata views, acquiring separate O_SEARCH/enumeration/content descriptors only when needed. Do not claim a stat snapshot pins a file descriptor. Go `os.SameFile` only accepts its own FileInfo implementation: provide and test a Darwin identity comparison for stat-derived metadata versus opened-descriptor metadata. Preserve cancellation, close ownership, mode classification, link refusal and replacement detection. No private entitlement or extra dependency.
- Update SPEC, architecture spine/memlog, contracts, epic context, source comments and affected tests together for the approved metadata and HTTP guarantees. Parent handles personal-release policy/owner synchronization and the final full gates; implementer reports focused checks and all pending native evidence. Preserve the existing evidence file.

- `.github/workflows/verify.yml:44` `verify` job, `matrix.os: [windows-latest, macos-latest]`, `runs-on: ${{ matrix.os }}`. A Linux job is a **second job**, not a matrix entry: the existing one runs `wails build`, `npm ci` and the frontend suite, none of which belong on Linux.
- `verify_workflow_test.go:118` currently fails the build if the workflow mentions `ubuntu` at all — "a Linux job is Ask First and none was approved". That rule is now approved and narrowed; rewrite it rather than delete it.
- `internal/source/handle_posix_test.go` (`//go:build linux || darwin`) — two tests, both with runner-capability skips at `:52` and `:81`. `handle_linux_test.go` — `O_PATH` and FIFO. Check what the skips do on the real runners; a skip that always fires is a vacuous pass. Baseline correction (2026-09-11): macOS already executes the shared POSIX tests in native CI; Linux execution and the additional guard coverage are missing. The frozen problem statement describes an older checkpoint, not current execution evidence.
- `internal/source/handle_posix.go:100` `OpenChildContent` — the only read-granting open: `nativeContentFlags()`, `unix.Fstat`, the `S_IFMT != S_IFREG` refusal, then `clearPosixNonBlocking`. Nothing asserts any of it (D-076); every fixture entry is an ordinary file.
- `internal/source/source.go:454` `childRelativeName` — still uses `filepath.IsAbs` and `filepath.VolumeName`, which are no-ops on a POSIX sender (D-084). `internal/stream/archive.go:393` `volumeQualified` is the host-independent shape to copy.
- `app.go:174` `StageTransfer` — the single choke point every selection passes, chooser and native drop alike, before `coordinator.Stage`. `app.go:214` `SelectFile` / `:220` `SelectDirectory` are the chooser half.
- `internal/stream/archive_memory_test.go:46` streams 50,000 entries to `io.Discard` and never reads one back (D-078). `internal/stream/archive.go:295` writes file entries with `zip.Deflate`, so a 4 GiB entry of repeating bytes costs CPU, not disk.
- `internal/stream/payload.go:354` `p.streamed.CompareAndSwap` — driven only sequentially by `TestWriteToRefusesASecondCall` (D-014); `TestCloseIsSafeWhenCalledConcurrently:1415` is the concurrency shape to copy.
- Wails v2.15.0 `internal/frontend/desktop/darwin/single_instance.go` `SetupSingleInstance` is upstream and treats every `createLockFile` error as contention, then `os.Exit(0)`. FairDrop cannot change it; it can decide whether to hand Wails the option (D-089).

## Tasks & Acceptance

**Execution:**
- [ ] `internal/server` — close D-110 with deterministic framing/final-write tests and full-stack file/folder regression coverage.
- [ ] `internal/source` — supported Darwin no-read metadata inspection with native permissions, identity, no-follow and content-read separation tests; retain Linux O_PATH.
- [ ] `.github/workflows/verify.yml` — a Linux job running vet and the Go suite, labelled adapter verification and not release proof; no `wails build`, no frontend steps.
- [ ] `verify_workflow_test.go` — replace the blanket ubuntu ban with the narrowed rule, and pin that the Linux job runs no `wails build`.
- [ ] `internal/source/handle_posix_test.go`, `handle_linux_test.go` — assert the content-open guard (a FIFO substituted after metadata is refused without blocking), and make every capability skip report why, so a skip that always fires is visible.
- [ ] `internal/source/source.go` — `childRelativeName` refuses a volume-qualified or absolute name by the same host-independent test `volumeQualified` uses.
- [ ] `app.go` — resolve a selection's *ancestors* once, where it enters, before the coordinator sees it; the final component is never resolved.
- [ ] `main.go` — on darwin, confirm the single-instance lock path is usable before handing Wails the option, and say so on stderr when it is not.
- [ ] `internal/stream` — read a past-threshold archive back and validate it; cover the 4 GiB entry and total; drive `WriteTo` from concurrent callers under `-race`.
- [ ] Path-class coverage for spaces, non-ASCII, >260 characters, UNC and zero-path drops, run on both native hosts.
- [ ] `evidence-3-7-execute-the-native-platform-test-matrix.md`, and the ten ids (including D-110) closed with `epics.md` kept in step.

**Acceptance Criteria:**
- Given the POSIX and Linux platform tests, when CI runs, then they execute natively and a skip that fired is reported rather than silently counted as a pass.
- Given a file under `/tmp` or `/var` on macOS, when a user selects it, then it transfers, while a selection that is itself a symlink is still refused.
- Given an archive past the ZIP64 entry-count and size thresholds, when it is produced, then it is read back and every entry validates.
- Given an unusable macOS lock path, when the built app launches, then its process remains alive and reports the degraded state instead of exiting silently. Native visual/focus checks are optional, not claimed automated proof.
- Given a delayed or failed final HTTP write, when natural transfer completion is considered, then success follows response finalization only, final-write failure is not Complete, and cancellation can still force-close without waiting for a blocked final write.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-7-execute-the-native-platform-test-matrix.md](evidence-3-7-execute-the-native-platform-test-matrix.md), created with the implementation.

## Spec Change Log

- 2026-09-11 (owner approval): "Lets fix them all" and "MacOS perms ... best option ... get the application to work" authorize bringing D-110 into 3.7 and replacing Darwin metadata acquisition. Owner also makes human release observations optional for personal development. Frozen intent/matrix amended explicitly on that approval, not weakened to match a test. Baseline is preserved. Prior paused-checkpoint notes below are historical.

- 2026-09-11: implementation checkpoint is not accepted. The full race gate exposed HTTP response finalization ordering (D-110, owned by 3.8), and the Darwin metadata-only assumption requires native evidence and an Ask First design decision. Matrix acceptance, mutation audit, formal review and native CI remain incomplete; no commit/push. See the sibling evidence file before resuming.

- 2026-09-11: resumed from the approved spec at the recorded baseline; reconciled current native CI evidence and epic context without changing approved intent or matrix. Implementation and review evidence remain in the sibling evidence file.

## Design Notes

**Resolve ancestors, never the leaf.** Whole-path `filepath.EvalSymlinks` would follow a selected symlink the traversal must refuse. Split the original path without lexical cleaning, resolve only its ancestor substring, preserve trailing separators and leave selected `.`/`..` and Windows device namespaces to the adapter. `filepath.Dir`/`Base` alone would clean away meaningful syntax. Tests prove resolution happens before `Inspect`, not inside it, and pin selected-link/refusal behavior.

**The Linux job blocks, and still is not release proof.** It runs on pull requests like every other job, so an `O_PATH` regression fails the build. What "not release proof" means is that no Linux result substitutes for a Windows or macOS one: the release workflow's artifacts are still built and checked on their own native runners, and the job says so in its own name so a later reader cannot mistake it.

**A skip is not a pass.** Both existing POSIX tests skip when the runner cannot make a symlink or cannot be denied search rights (root bypasses it). On a real runner those skips may always fire, which would make this story's central claim vacuous. Each must report the skip with its reason, and the evidence file records which fired on which host.

## Verification

**Commands:**
- Follow AGENTS.md sequentially: `wails build`; bindings-drift and `.gitkeep` checks; `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 240s ./...`; confirm `go env CGO_ENABLED` is `1`; `go test -count=1 -race -timeout 1200s ./...`; frontend `npm test`; line-ending check. Do not overlap these gates. The race allowance covers the real 4 GiB ZIP64 fixture on hosted CPUs; no size or assertion is reduced.
- Foreign-platform preflight: Darwin arm64 and Linux amd64 `go build ./...` and `go vet ./...`, plus Darwin arm64 bare `staticcheck ./...` (not `go tool staticcheck` under a foreign GOOS). These are type checks, not native proof.
- The CI run itself, read with `gh run view --json conclusion,jobs` — never `gh run watch`, which has exited 0 on a failed run
- Mutations: drop `O_NONBLOCK` from `nativeContentFlags`; drop the `S_IFREG` refusal; revert `childRelativeName` to `filepath.VolumeName`; resolve the leaf as well as the ancestors; remove the Linux job — each must fail a named test
