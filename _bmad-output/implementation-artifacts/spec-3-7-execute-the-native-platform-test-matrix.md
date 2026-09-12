---
title: 'Story 3.7: Execute the Native Platform Test Matrix'
type: 'chore'
created: '2026-09-11'
status: 'in-review'
baseline_commit: 'd43aa69db42c19324bae9f837b909649ca608099'
review_loop_iteration: 1
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

- `app.go`, `main.go`, new selection-source boundary adapter and `native_matrix_test.go`: Stage delegates unchanged; a coordinator-facing SourcePort decorator resolves lexical ancestors only after lifecycle admission, before the raw inspector. The stream adapter keeps the raw inspector and receives the canonical staged path. Preserve leaf/trailing/dot/namespace refusals. Resolution observes cancellation before/after and while waiting; at most one unresolved filesystem call per decorator may remain outstanding, retries refuse busy until it returns. Never hold a mutex across filesystem I/O or claim the OS call itself was cancelled. Test busy/shutdown admission, cancellation with a blocked resolver, and no late Inspect/network work.
- `internal/server/{handler,lifecycle}.go`, `finalization_test.go`: observe actual final connection writes before natural Complete or pre-header 410 failure; retain force-close cancellation, write-error/short-write failure and producer quiescence. Preserve TCP CloseWrite through the wrapper, with a real unread-body/oversized-header path test. Handler return or Flush alone is insufficient.
- `internal/source/{handle,handle_posix,handle_darwin,handle_linux,source}.go`: Darwin parent-relative no-follow stat snapshots, separate descriptors and identity checks across both stat representations; include generation/birth metadata to distinguish recycled device/inode pairs, native unlink/recreate coverage plus deterministic recycled-identity tests. Document residual fingerprint limitations honestly. Linux O_PATH and portable archive-name refusal remain. Snapshot Close never owns the borrowed parent.
- `main.go`, `single_instance_darwin*.go`: probe Wails' exact Foundation lock path nonblocking/no-follow, require a regular descriptor before flock, preserve contention handoff and fixed private warning. Native FIFO/symlink fixtures must not hang. Options wiring tests control usability and separately test degraded behavior; keep production default wiring pinned. The CI smoke locks the runner-owned inode and restores its mode.
- `internal/stream/{archive_zip64,payload_concurrent}_test.go`: retain full entry-count and real 4 GiB+1 CRC/readback and concurrent loser no-read/no-write assertions. Add a sparse/virtual-prefix ZIP64 offset-threshold test using the production entry writer and archive/zip.SetOffset; validate central-directory and subsequent-entry offsets above 32 bits and CRC. Distinguish synthetic offset coverage from the real compressed stream; no multi-GiB compressed-stream claim.
- `.github/workflows/verify.yml`, `verify_workflow_test.go`, `scripts/`: native desktop full gates, Linux Go-only gate, explicit skips, native mutations and guarded process smoke. Pin the new gates within the correct jobs/platform conditions. Mutation verdicts require passing named baselines, mutation-specific assertion evidence and explicit rejection of timeout/build/setup failures; test the verdict helper. Keep 1200s race allowance and accurate comments.
- `internal/transfer/coordinator_outcomes_test.go`: force the observer-return window and use an unbuffered refused event as a drainer barrier before asserting lease return. CI exposed a test scheduling assumption; production coordinator behavior is unchanged.
- Canonical SPEC, architecture spine/memlog, contracts and epic context move together for metadata/finalization guarantees. `docs/release-policy.md` records the owner's optional-manual-check decision. Evidence, failures, mutation results and the ten deferred-id closures stay in the sibling evidence file.

## Tasks & Acceptance

**Execution:**
- [x] Review loop 1: implement the corrected admission, lock, identity, half-close, ZIP64-offset and CI-proof requirements above; full-stack matrix observes matching natural Complete before cleanup and tests Unicode/space leaf names, returned metadata, HTTP filename and ZIP entry names. Record independent review triage and new mutation/native results in evidence.
- [x] `internal/server` — close D-110 with deterministic framing/final-write tests and full-stack file/folder regression coverage.
- [x] `internal/source` — supported Darwin no-read metadata inspection with native permissions, identity, no-follow and content-read separation tests; retain Linux O_PATH.
- [x] `.github/workflows/verify.yml` — a Linux job running vet and the Go suite, labelled adapter verification and not release proof; no `wails build`, no frontend steps.
- [x] `verify_workflow_test.go` — replace the blanket ubuntu ban with the narrowed rule, and pin that the Linux job runs no `wails build`.
- [x] `internal/source/handle_posix_test.go`, `handle_linux_test.go` — assert the content-open guard (a FIFO substituted after metadata is refused without blocking), and make every capability skip report why, so a skip that always fires is visible.
- [x] `internal/source/source.go` — `childRelativeName` refuses a volume-qualified or absolute name by the same host-independent test `volumeQualified` uses.
- [x] `selection_source.go` — resolve a selection's *ancestors* once after coordinator admission and before raw Inspect, with bounded outstanding work and cancellation-aware waiting; the final component is never resolved and `app.go` delegates unchanged.
- [x] `main.go` — on darwin, confirm the single-instance lock path is usable before handing Wails the option, and say so on stderr when it is not.
- [x] `internal/stream` — read a past-threshold archive back and validate it; cover the 4 GiB entry and total; drive `WriteTo` from concurrent callers under `-race`.
- [x] Path-class coverage for spaces, non-ASCII, >260 characters, UNC and zero-path drops, run on both native hosts.
- [x] `evidence-3-7-execute-the-native-platform-test-matrix.md`, and the ten ids (including D-110) closed with `epics.md` kept in step.

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

- 2026-09-11 (review loop 1, bad_spec): ancestor resolution before Stage bypassed lifecycle refusal and cancellation; non-frozen Code Map now places a bounded, cancellable selection decorator behind admission and before the raw inspector. Clarified snapshot reuse identity, nonblocking lock probing, preserved TCP half-close, ZIP64 offset proof and gate-verdict requirements. Frozen user intent is unchanged. KEEP: all verified implementation at adc492cfb656f888c9a284c89df445086a326ab7 except the identified flawed details; reconstruct from that Git checkpoint, not from historical interfaces. Preserve real HTTP finalization ordering/force-close cancellation, Darwin no-read/no-follow metadata, Linux O_PATH, native path/ZIP64/concurrency fixtures, fixed private diagnostics, optional manual policy, full failed logs and ten stable deferred IDs. Re-derive the code against this corrected map, with no dependency/public API/refusal-policy change. Other accepted review patches are carried into the same derivation.

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
