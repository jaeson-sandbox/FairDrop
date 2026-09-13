# Evidence: Story 3.9: Record Release Evidence and Optional Manual Checks

Baseline: `c3a80064640db97c94188dce25e39a9211dbccf2`.

## The two decisions this story needed

Both were Ask First items in the spec and both were answered by the owner on 2026-09-12, before
any file changed.

**Ordering.** `epics.md` said Stories 3.10, 3.11 and 3.12 were each "required before Story 3.9",
in four separate places. That stopped being true on 2026-09-11, when the owner's policy rewrote
3.9 to stop waiting on a person, and nobody updated the statements that still said otherwise —
a contradiction inside the planning artifacts rather than a question about the work. Ruled: 3.9
may close while they are open, because those lines were protecting a *release*, not a story. All
four now say "required before a release", and the record 3.9 writes names the four open Story
3.10 decisions, Story 3.11's nine copy gaps and Story 3.12's three capture gaps as limitations
rather than omitting them.

**D-086.** Restoration means the window is unminimised with its session, transfer and keyboard
focus intact, and that the second process starts no coordinator, listener or beacon and exits.
Coming to the front is a platform courtesy Windows may withhold: it refuses `SetForegroundWindow`
from a process that is not in the foreground, and the second instance has already exited by the
time the first would ask. A flashing taskbar button is therefore the expected Windows outcome,
not a defect, and the `AlwaysOnTop` toggle workaround is deliberately not pursued — it fights a
deliberate OS protection to win a cosmetic race. No native second launch has been observed on
either platform, and the record says so rather than implying the decision was tested.

## D-073: the contrast figures

Nine text pairs lived as one run-on sentence at `DESIGN.md:144`, beside the formatted table at
`:132-142` that holds the load-bearing pairs. They are now a table of their own, the status-text
floor is its own sentence, and the load-bearing table is unchanged.

The stale half of the entry mattered as much as the layout: the document instructed an editor to
"Re-run unrounded automated checks if opacity, blending, color-mix, or adjacent surfaces change",
which predates `styles.test.ts` deriving these ratios from the declared tokens. Following that
instruction meant hand-editing values a test computes, and the test would then fail against the
document it serves. The replacement says the figures are derived, not maintained, and that the
way to update one is to run the frontend suite and copy what it reports.

Moving published figures is only safe because a test already owns them, and that is exactly what
the mutation below confirms.

## Mutation table

| # | Mutation | Result |
|---|---|---|
| M1 | round one published contrast figure (`13.064952890` → `13.06`) | KILLED — `the unrounded contrast proof > publishes every figure it proves, unrounded, in DESIGN.md` |
| M2 | flip an unobserved manual row to "pass" | KILLED — `a row claims "pass" without naming who observed it` |
| M3 | restore "pending" as a row's result | KILLED — `the policy replaced pending with optional / unverified` |
| M4 | remove the workflow run citation from the record | KILLED — `the release record cites no workflow run at all` |
| M5 | take the run reference off one machine-verified row | KILLED — `a row claims "**pass**" without citing the run that produced it` |
| M6 | take the reviewer off the one genuinely observed row | KILLED — `a row claims "**pass, sender-side; receiver-side unverified**" without naming who observed it` |

**The new test found a real defect in the file it was written for.** The first draft of the
machine-verified table said "every runner, in that gate" for five rows — a back-reference a reader
would follow and a checker cannot. The test rejected all five on its first run against the
finished file, and the rows were rewritten to cite the run. That is the whole reason it exists:
the gap between a claim a person would accept and one that carries its own evidence.

The rule deliberately differs by table shape. A table with a Reviewer column records what a
person observed, so the person is the evidence; the machine table has no such column and owes the
run instead. An earlier version accepted either everywhere, which let the word "run" appearing
anywhere in a prose cell discharge a manual row — the shape of the mistake this test exists to
stop, not a spelling of it.

## What the record now claims, and what it refuses to

Claimed, each with a run id and commit: three green jobs of Verify 34726508196 at `c3a8006`; 502
Go test functions across 8 packages; the same under `-race` behind a compiled cgo probe; 498
frontend tests; `wails build` on both desktop runners with a bindings-drift check; and 68
assertion-named mutations, 57 on every runner, 3 on Linux and macOS, 8 macOS-only.

Refused: any claim that the product works. The record says so in as many words — the gate shows
the code compiles, builds and behaves as its tests describe, and everything a machine cannot
observe is listed as optional and unverified with what would have to be seen.

**The published artifact is 36 commits behind `main`.** `v0.1.0` was built from `cc9ce58` on
2026-09-10 and predates Stories 3.4 through 3.8 entirely: no bounded lifecycle waits, no
visible-failure work, no reconciled error copy, no native platform matrix, no directory-stream
hardening. Recording its checksums without that sentence would have been the most misleading true
statement in the file.

## Verification

Read stage by stage, never chained behind a shell exit status:

- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go tool staticcheck ./...` — clean
- `go test -count=1 -timeout 300s ./...` — 8 packages ok
- `cd frontend && npx vitest run` — 17 files, 498 tests passing, including the contrast proof
  after the table move

## Deferrals

None. Both ids this story names are discharged, and the two decisions it needed were taken rather
than deferred.
