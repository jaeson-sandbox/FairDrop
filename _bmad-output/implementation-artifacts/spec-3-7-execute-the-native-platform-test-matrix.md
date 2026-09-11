---
title: 'Story 3.7: Execute the Native Platform Test Matrix'
type: 'chore'
created: '2026-09-11'
status: 'ready-for-dev'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop's most security-relevant code is the code least often executed. `handle_posix.go`'s no-follow opens, search-only ancestors, `O_PATH`/`O_EVTONLY` metadata handles and the `O_NONBLOCK`-plus-`fstat` content guard are compiled for Linux and Darwin and run on neither — the Windows host cross-compiles them and Story 3.2's runners are Windows and macOS only. That is exactly the state darwin was in until its first native run failed forty tests. Two defects found while chasing that build are still open for the same reason: on macOS every path under `/tmp`, `/var` or `/etc` is refused as unsupported because each is a symlink, and a lock file that cannot be opened for any reason at all makes FairDrop exit silently instead of launching. Alongside them sit the claims no host has ever checked: the path classes the product advertises, archive names built with host-dependent predicates, ZIP64 thresholds, and a once-only guard never driven by two callers.

**Approach:** Run the tests where they mean something. Add a Linux job that executes the Go suite for adapter verification and says in the job itself that it is not release proof; extend the native matrix with the path classes and archive claims; and fix the two defects the native runs exist to catch.

## Boundaries & Constraints

**Always:** A guarantee that depends on an operating system is asserted on that operating system. The Linux job names itself adapter verification, never release proof. The traversal's refusal of link-like components is unchanged and stays pinned: what changes is where a path is resolved, not what the walker accepts. AD-9 holds — no test may put a real selected path or token into diagnostics.

**Ask First:** Any change to what the product refuses, beyond the resolution point this story moves. A second CI dependency or a new runner image. Any new public error code or user-visible string — Story 3.11 owns the remaining copy.

**Never:** Weaken a guard to make a native run pass; a failure there is the finding. Let a Linux result stand in for Windows or macOS release proof. Run `wails build` on Linux. Decide D-065 or D-068 — those are Story 3.12's.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| File under a symlinked system directory | `/tmp/x/report.pdf` on macOS | Staged and transferred | Resolved at entry (D-094) |
| Selection that *is* a symlink | a link the user picked | Still refused | `path_unsupported`, unchanged |
| FIFO swapped in after metadata | POSIX content open | Refused without blocking | `path_unsupported` (D-076) |
| Metadata open | POSIX, no read rights | `O_PATH` / `O_EVTONLY` succeeds natively | Coded (D-074) |
| Path over 260 chars, UNC, non-ASCII, spaces | Windows and macOS | Staged and streamed end to end | Coded per class (D-007) |
| Windows-shaped entry name on a POSIX sender | `C:evil.txt` | Refused by both name gates | `path_unsupported` (D-084) |
| Archive past the ZIP64 thresholds | >65,535 entries; a 4 GiB entry | Read back and validated | Coded (D-078) |
| Two concurrent `WriteTo` calls | one prepared payload | Exactly one streams | Second refused (D-014) |
| Lock file unopenable for any reason but contention | macOS, unwritable temp dir | FairDrop launches and says why | Never a silent exit (D-089) |

</frozen-after-approval>

## Code Map

- `.github/workflows/verify.yml:44` `verify` job, `matrix.os: [windows-latest, macos-latest]`, `runs-on: ${{ matrix.os }}`. A Linux job is a **second job**, not a matrix entry: the existing one runs `wails build`, `npm ci` and the frontend suite, none of which belong on Linux.
- `verify_workflow_test.go:118` currently fails the build if the workflow mentions `ubuntu` at all — "a Linux job is Ask First and none was approved". That rule is now approved and narrowed; rewrite it rather than delete it.
- `internal/source/handle_posix_test.go` (`//go:build linux || darwin`) — two tests, both with runner-capability skips at `:52` and `:81`. `handle_linux_test.go` — `O_PATH` and FIFO. Neither file has ever executed. Check what the skips do on the real runners; a skip that always fires is a vacuous pass.
- `internal/source/handle_posix.go:100` `OpenChildContent` — the only read-granting open: `nativeContentFlags()`, `unix.Fstat`, the `S_IFMT != S_IFREG` refusal, then `clearPosixNonBlocking`. Nothing asserts any of it (D-076); every fixture entry is an ordinary file.
- `internal/source/source.go:454` `childRelativeName` — still uses `filepath.IsAbs` and `filepath.VolumeName`, which are no-ops on a POSIX sender (D-084). `internal/stream/archive.go:393` `volumeQualified` is the host-independent shape to copy.
- `app.go:174` `StageTransfer` — the single choke point every selection passes, chooser and native drop alike, before `coordinator.Stage`. `app.go:214` `SelectFile` / `:220` `SelectDirectory` are the chooser half.
- `internal/stream/archive_memory_test.go:46` streams 50,000 entries to `io.Discard` and never reads one back (D-078). `internal/stream/archive.go:295` writes file entries with `zip.Deflate`, so a 4 GiB entry of repeating bytes costs CPU, not disk.
- `internal/stream/payload.go:354` `p.streamed.CompareAndSwap` — driven only sequentially by `TestWriteToRefusesASecondCall` (D-014); `TestCloseIsSafeWhenCalledConcurrently:1415` is the concurrency shape to copy.
- Wails v2.15.0 `internal/frontend/desktop/darwin/single_instance.go` `SetupSingleInstance` is upstream and treats every `createLockFile` error as contention, then `os.Exit(0)`. FairDrop cannot change it; it can decide whether to hand Wails the option (D-089).

## Tasks & Acceptance

**Execution:**
- [ ] `.github/workflows/verify.yml` — a Linux job running vet and the Go suite, labelled adapter verification and not release proof; no `wails build`, no frontend steps.
- [ ] `verify_workflow_test.go` — replace the blanket ubuntu ban with the narrowed rule, and pin that the Linux job runs no `wails build`.
- [ ] `internal/source/handle_posix_test.go`, `handle_linux_test.go` — assert the content-open guard (a FIFO substituted after metadata is refused without blocking), and make every capability skip report why, so a skip that always fires is visible.
- [ ] `internal/source/source.go` — `childRelativeName` refuses a volume-qualified or absolute name by the same host-independent test `volumeQualified` uses.
- [ ] `app.go` — resolve a selection's *ancestors* once, where it enters, before the coordinator sees it; the final component is never resolved.
- [ ] `main.go` — on darwin, confirm the single-instance lock path is usable before handing Wails the option, and say so on stderr when it is not.
- [ ] `internal/stream` — read a past-threshold archive back and validate it; cover the 4 GiB entry and total; drive `WriteTo` from concurrent callers under `-race`.
- [ ] Path-class coverage for spaces, non-ASCII, >260 characters, UNC and zero-path drops, run on both native hosts.
- [ ] `evidence-3-7-execute-the-native-platform-test-matrix.md`, and the nine ids closed with `epics.md` kept in step.

**Acceptance Criteria:**
- Given the POSIX and Linux platform tests, when CI runs, then they execute natively and a skip that fired is reported rather than silently counted as a pass.
- Given a file under `/tmp` or `/var` on macOS, when a user selects it, then it transfers, while a selection that is itself a symlink is still refused.
- Given an archive past the ZIP64 entry-count and size thresholds, when it is produced, then it is read back and every entry validates.
- Given a macOS temp directory that cannot hold a lock file, when FairDrop launches, then it opens and reports the degraded state instead of exiting silently.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-7-execute-the-native-platform-test-matrix.md](evidence-3-7-execute-the-native-platform-test-matrix.md), created with the implementation.

## Spec Change Log

## Design Notes

**Resolve ancestors, never the leaf.** `filepath.EvalSymlinks` on the whole path would resolve a symlink the user deliberately picked, which is precisely what the traversal refuses and must keep refusing. The shape is `filepath.Join(evaluated(filepath.Dir(p)), filepath.Base(p))`: the ancestors stop being link-like, the leaf reaches `Inspect` exactly as the user named it. A test must prove the resolution happens *before* `Inspect`, not inside it — resolving inside would silently retire the guard.

**The Linux job blocks, and still is not release proof.** It runs on pull requests like every other job, so an `O_PATH` regression fails the build. What "not release proof" means is that no Linux result substitutes for a Windows or macOS one: the release workflow's artifacts are still built and checked on their own native runners, and the job says so in its own name so a later reader cannot mistake it.

**A skip is not a pass.** Both existing POSIX tests skip when the runner cannot make a symlink or cannot be denied search rights (root bypasses it). On a real runner those skips may always fire, which would make this story's central claim vacuous. Each must report the skip with its reason, and the evidence file records which fired on which host.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 240s ./...`; `go test -count=1 -race -timeout 420s ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent, per AGENTS.md's pre-flight
- `cd frontend && npx vitest run`; then, alone, `wails build`
- The CI run itself, read with `gh run view --json conclusion,jobs` — never `gh run watch`, which has exited 0 on a failed run
- Mutations: drop `O_NONBLOCK` from `nativeContentFlags`; drop the `S_IFREG` refusal; revert `childRelativeName` to `filepath.VolumeName`; resolve the leaf as well as the ancestors; remove the Linux job — each must fail a named test
