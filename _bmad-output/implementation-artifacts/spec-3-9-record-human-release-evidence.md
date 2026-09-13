---
title: 'Story 3.9: Record Release Evidence and Optional Manual Checks'
type: 'chore'
created: '2026-09-12'
status: 'done'
baseline_commit: 'c3a80064640db97c94188dce25e39a9211dbccf2'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The release record does not describe this project. `release-evidence.md` still carries the pre-policy rule that "a missing, ambiguous, stale, or failed row blocks a release", one row that is half-verified and one that has been **pending** since Story 3.1 — while the evidence a personal release actually rests on lives nowhere near it: three native CI jobs per commit, sixty-eight assertion-named mutation proofs, a Wails build on each supported OS, and artifact identity with checksums, all of them reachable only through GitHub run logs and eight story evidence files. Two smaller debts sit in the same blind spot: `DESIGN.md` publishes nine contrast pairs as a run-on sentence beside the table built to hold them and instructs an editor to re-run by hand the figures `styles.test.ts` now derives and pins (D-073); and the one Windows behaviour nobody has observed — a restored window that may only flash its taskbar button — has no recorded decision about whether it counts as restoration (D-086).

**Approach:** Make the record state exactly what machines verified, with the run and artifact identity that makes each claim checkable, and label everything else optional and unverified rather than pending. Close the two documentation debts with the decisions they need, and add the one mechanical check that keeps a future editor from turning an unobserved row into a pass.

## Boundaries & Constraints

**Always:** A pass a machine produced carries its run identity — workflow run id, commit SHA, and the per-job conclusions read from `gh run view --json conclusion,jobs`, never from `gh run watch`, which has exited 0 over a failed run. An unobserved check reads optional and unverified, with what would have to be seen, and never "pending". A failed automated check or a known functional defect still blocks acceptance under `docs/release-policy.md`. Attempts that did not pass stay recorded as they were written.

**Ask First:** Whether this story may close while Stories 3.10, 3.11 and 3.12 are open — `epics.md` says each is required before 3.9, and the owner's 2026-09-11 policy says 3.9 no longer waits on a person. Any new public error code or user-visible string, which is Story 3.11's.

**Never:** Invent a manual pass, or convert a pending row into one. Claim browser or assistive-technology certification from unit tests. Hand-edit a contrast figure `styles.test.ts` derives. Decide D-018, D-055, D-064 or D-088 — those are Story 3.10's; this story records them as open.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Native gate on a commit | Verify run, three jobs | One row: run id, SHA, each job's own conclusion | A failed job is recorded as failed |
| Release artifact | Windows and macOS builds | Identity and checksum recorded per platform | Absent artifact reads unverified, not pending |
| Mutation proof | the native proof script | Recorded as a count with where the transcript lives | A flaked proof is a failure, not a retry |
| Manual check nobody ran | browser matrix, screen reader, firewall | Optional / unverified, naming what would be observed | Never a pass |
| Manual check somebody ran | a real device observation | Platform, OS version, artifact, date, observer, result | Recorded whichever way it went |
| Windows restoration only flashes the taskbar | second launch, first window behind | The record states whether that counts (D-086) and that it is unobserved | Decision recorded, not assumed |
| Contrast figures | nine pairs published as prose | Moved into the table; the test still finds every figure | A lost figure fails `styles.test.ts` |
| A row claiming a pass | any future edit | Carries run identity or an observer | A pass without either fails a test |

</frozen-after-approval>

## Code Map

- `_bmad-output/implementation-artifacts/release-evidence.md` — the file this story owns. Its rules paragraph is pre-policy and contradicts `docs/release-policy.md`; row 1 is a real sender-side observation worth keeping verbatim; row 2 has read **pending** since Story 3.1 and is exactly what the policy renames. The "Attempts that did not pass" section is history and stays.
- `docs/release-policy.md` — the owner's 2026-09-11 decision, and the only authority for what blocks. It already states that Linux and cross-compilation never substitute for native verification.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md:130-144` — the contrast table ends at `:142`; `:144` is the nine-pair run-on sentence plus the stale instruction "Re-run unrounded automated checks if opacity, blending, color-mix, or adjacent surfaces change" (D-073).
- `frontend/src/ui/styles.test.ts:481-615` — already the owner of those figures: it recomputes each ratio from the declared tokens and asserts `designSpine` contains the exact nine-decimal string, plus the literal floor sentence `exceed 5.14:1 light and 7.05:1 dark` and the weakest-of-three claim. This is what makes the table move mechanically safe and a hand-edit self-defeating.
- `.github/workflows/verify.yml` — three jobs: `verify (windows-latest)`, `verify (macos-latest)`, and the Linux adapter job that names itself not release proof. `.github/workflows/release.yml` — the per-platform build and checksum that supply artifact identity.
- `scripts/verify-native-mutations.sh` and `scripts/mutationverdict` — 68 mutations, each requiring a named assertion; the count and the rule are what the record should cite rather than a claim of coverage.
- `app.go`'s `restoreWindow` and `main_test.go`'s options pins — what Story 3.1 proved with fakes, which is the boundary D-086 sits just outside of.

## Tasks & Acceptance

**Execution:**
- [x] `release-evidence.md` — rewrite the rules to match `docs/release-policy.md`; add the machine-verified rows with run identity; convert row 2 and every unrun manual check to optional/unverified with what would have to be observed; keep row 1 and the failed-attempts section as written.
- [x] `release-evidence.md` — record the D-086 decision: what counts as restoration on Windows when the foreground rules leave the window behind, and that no native second launch has been observed.
- [x] `DESIGN.md` — move the nine pairs from prose into the contrast table; replace the stale re-run instruction with a note that `styles.test.ts` derives and pins these figures, so a hand-edit fails the suite.
- [x] `main_test.go` — one test over `release-evidence.md`: no row says "pending", and any row whose result claims a pass carries either a Verify run id or a named observer.
- [x] `evidence-3-9-record-human-release-evidence.md`, with D-073 and D-086 closed and `epics.md` kept in step.

**Acceptance Criteria:**
- Given the contrast figures moved into the table, when the frontend suite runs, then `styles.test.ts` still finds every published figure, the floor sentence and the weakest-of-three claim.
- Given a row that claims a machine-verified pass, when the new test runs, then a row missing its run identity fails it by name.
- Given every check no one has run, when the record is read, then each says optional and unverified and names what would have to be observed, and the word "pending" appears nowhere as a result.

## Evidence

Audit and gate transcripts live in
[evidence-3-9-record-human-release-evidence.md](evidence-3-9-record-human-release-evidence.md), created with the implementation.

## Spec Change Log

**2026-09-12 (approval).** Both Ask First items were settled before implementation began.

*Ordering.* `epics.md` said Stories 3.10, 3.11 and 3.12 were each "required before Story 3.9",
which stopped being true when the owner's 2026-09-11 policy rewrote 3.9 to stop waiting on a
person. The owner ruled that 3.9 may close while they are open: those lines were protecting the
*release*, not the story, and now read "required before a release". This story's record therefore
names the four open Story 3.10 decisions and Story 3.11's copy gaps as limitations rather than
omitting them, and claims no pass against behaviour those stories will change.

*D-086.* Restoration means the window is unminimised with its session, transfer and keyboard focus
intact, and no second coordinator, listener or beacon started. Coming to the front is a platform
courtesy Windows may withhold -- it refuses `SetForegroundWindow` from a process that is not in the
foreground, and the second instance has already exited by then -- so a flashing taskbar button is
the expected Windows outcome and not a defect. The `AlwaysOnTop` toggle workaround is therefore not
pursued; it fights a deliberate OS protection and flickers the window. No native second launch has
been observed either way, and the record says so.

## Design Notes

**The record cites, it does not summarise.** "Native CI passed" is the claim every one of this project's own lessons says to distrust: `gh run watch --exit-status` once exited 0 over a failed run, and a green suite has hidden a defect here more than once. A row therefore carries the run id and SHA so the claim can be re-read at its source, and the per-job conclusions so a two-of-three does not read as three.

**Moving the figures is safe because a test already owns them.** `styles.test.ts` asserts `designSpine` contains each nine-decimal figure, so a figure dropped in the move fails the suite rather than the reviewer's eye — which is also why the replacement instruction must say the test owns them, and why the exact sentence `exceed 5.14:1 light and 7.05:1 dark` has to survive the edit verbatim.

**D-086 needs a stated rule, not an observation.** Windows refuses `SetForegroundWindow` from a process that is not in the foreground, and the second instance has already exited by the time the first would ask. A flashing taskbar button is therefore the *expected* outcome on Windows, not a defect — so the record should say that restoration means the window is unminimised with its session and focus intact, that coming to the front is a platform courtesy Windows may withhold, and that no native second launch has been observed either way.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 300s ./...` — clean
- `cd frontend && npx vitest run` — 498 passing, including the contrast proof after the table move
- Mutations: drop one contrast figure from the table; change a pass row's run id to nothing; reintroduce the word "pending" as a result — each must fail a named test
