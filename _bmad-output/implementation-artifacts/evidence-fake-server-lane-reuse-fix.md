# Evidence: the fake server reused a closed event lane

## Defect

CI run 36253748499 (commit `fc94b36`, a docs-only commit on `epic-9-motion-and-clarity`) failed
the Linux job's race step:

```
--- FAIL: TestTimedOutServerStopFencesNewSessionsUntilTheProductionCallCompletes (0.00s)
    coordinator_lifecycle_test.go:1696: late cleanup touched the newer run: ServerPort.Stop calls = 2, want one coalesced call
```

The previous run on the same code (36252776598, `e0ce076`) passed every job, and 300 local
`-race` runs of the test passed, so it was intermittent. No Go code changed in Epic 9.

## Cause: the test harness, not the coordinator

`fakeServer.Start` (`internal/transfer/helpers_test.go`) returned the same `f.events` channel on
every call, and `fakeServer.Stop` closes it. The test stages, times out a Stop, releases it (the
lane closes), then stages a **second** session. That second `Start` handed back the closed lane,
so the new session's drainer read it as the server ending and, as designed (D-042), synthesised a
terminal outcome whose teardown called `ServerPort.Stop` again. Whether that happened before the
test's final `stopCalls` check was down to scheduling.

The real `ServerPort.Start` builds a new lane per session, so production code cannot hit this.
The test's intent -- a stale cleanup must not touch a newer run -- is unaffected; it was the fake
that manufactured a second Stop.

## Proof

| Step | Result |
|---|---|
| Probe: `time.Sleep(50ms)` before the final check, old harness | **5/5 FAIL**, same message as CI |
| New `TestFakeServerHandsEachStartAnOpenLane`, old harness | **FAIL**: "the second Start handed over the lane the first Stop closed…" |
| Fix: `Start` opens a new lane (same depth) when the previous one was closed | — |
| Probe still in place, fixed harness | 5/5 pass |
| Probe removed; `go test -race -count=20 ./internal/transfer/` | ok |
| Mutation: revert the fix | the new harness test fails, naming the reused lane |

## Gate

`gofmt -l .` clean · `go vet ./...` clean · `go tool staticcheck ./...` clean ·
`go test -count=1 ./...` ok (9 packages) · `CGO_ENABLED=1 go test -count=1 -race ./...` ok (9
packages) · `GOOS=linux GOARCH=amd64 go vet ./internal/...` clean. Test-only change: nothing that
ships is touched, so no story is required (AGENTS.md, Git workflow).
