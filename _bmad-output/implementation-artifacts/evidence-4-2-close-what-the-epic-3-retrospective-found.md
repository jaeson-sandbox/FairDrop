# Evidence: Story 4.2: Close What the Epic 3 Retrospective Found

Baseline: `fa6ee8a`.

## What the owner decided, and when

**2026-09-14, after the Epic 3 retrospective.** Asked where to go next — Story 4.1, the
retrospective's action items, or a release — the owner chose the action items. Ten of the twelve
carry a verified source and a named fix and are done here; items 22 and 23 are excluded and stay
open, the first because it needs the owner's decision and the second because splitting
`coordinator.go` deserves a story that is not also fixing ten unrelated things.

## The story with no investigation phase

Every item arrived with its failure mode already established, most of them by mutation during the
retrospective: break the guarantee, run the suite, watch nothing fail. So the work was not finding
out what was wrong — it was making each one fail a named test. The mutation table below is
therefore mostly the retrospective's own mutations, re-applied and now dying.

Eight of the ten add proof to code that was already correct, or correct a sentence. Two change
behaviour: the `defaults read` deadline and the instance-lock diagnostic. That ratio is the
retrospective's finding restated — the epic's code was in better shape than its prose.

## Three vacuity guards that earned their place

Written in as a matter of routine, and all three fired on my own work before any mutation did:

- The drain pin asserted **one** `acceptTerminal` call site in `drain` and found **two**. They are
  deliberately different: the in-loop arm forwards a real server report and stays gated to
  TRANSFERRING, because widening it would let a stray event end a session nobody claimed; the
  post-loop synthesis covers a server that left without reporting and must reach STAGED and
  CLAIMING too. The test now pins both, told apart by which one builds its own event rather than by
  source order.
- The ancestor test built `filepath.Join("C:", "Users", …)`, which is `C:Users` — drive-**relative**.
  `resolveAncestorsWith` correctly declines to treat that as an ancestor, so the test reached the
  branch zero times. Its call counter said so instead of passing.
- The AD-9 assertion on the lock diagnostic first read the format string rather than the formatted
  line. A path reaches a log through the *arguments*, so it would have missed exactly the
  interpolation it exists to refuse. It formats first now, and the mutation proving it shows the
  leaked temp path in the failure message.

## Item by item

### 12 — WCAG 1.4.12 text spacing: the half of a declared requirement nothing checked

`epic-3-context.md` requires "200% text with text-spacing overrides". `DESIGN.md:178` says content
containers grow under both. `EXPERIENCE.md:275` names all four values. Nothing checked any of them:
Story 3.12 delivered the 200% half because its own I/O matrix named 320 pixels, 200% text, targets
and forced colors and never named text spacing — so no story was wrong and the requirement stayed
half met until the retrospective counted it.

Applied as a **user stylesheet with `!important`**, not as tokens, because that is what 1.4.12 is
about — and because it has to be. `style.css` declares `letter-spacing` on three selectors, and an
author-level rule of equal specificity would lose to it, leaving the suite measuring the shipped
spacing and calling it a pass.

Four cases: Staged, Transferring, Staged at the 320-pixel floor, and the overrides together with
200% text. Each begins by reading the spacing back off a rendered element and checking it against
that element's own font size, which is what `em` resolves against.

### 13 — D-018's three response headers, pinned to their values

One of the four release-blocking decisions Story 3.10 settled, propagated correctly to
`fairdrop-architecture.md`, `fairdrop-spec.md`, `handler.go` and `handler_test.go` — and implemented
by three header lines that each survived deletion against a green repository.

The rejection header is the instructive one, because a test looked like it covered it.
`TestRejectionsAreIndistinguishable` renders each rejection and compares them **to each other**,
taking the first as its reference. All-missing is exactly as indistinguishable as all-present.
Indistinguishable from each other and correct are two different claims, and only one was being made.
`TestEveryRejectionCarriesTheCrossOriginHeader` makes the other.

### 14, 19 — the function that decides composition, and the protection that disabled itself silently

`acquireInstanceLock` had one call site in `main()` and no test. `main_test.go` proved
`lockFileExclusive` is exclusive and handed `buildApp` a `held` boolean, so nothing ever ran the
code that decides it: the retrospective replaced the whole body with `return func() {}, true`
against a green repo.

Its two process-wide dependencies are injectable now — `os.UserConfigDir` and the logger — the way
`darwinLockPathUsable` already passes `syscall.Flock` to `probeDarwinLock`. Four tests drive the
real function: two acquisitions against one config directory, the lock file actually existing, both
reachable setup failures, and AD-9 on the line they leave.

And it leaves one. All three setup failures returned `held=true` with no diagnostic at all, while
every other protection in this build that switches itself off says so — `singleInstanceOption` logs
when Wails' own lock is unusable, `buildApp` logs when this one is not held. A locked-down profile
directory silently removed D-088's backstop with no trail.

### 15 — CLAIMING, pinned structurally because it cannot be driven

Three tests appeared to cover `drain`'s synthesis and none pinned CLAIMING. The third,
`TestTheDrainerMayEndASessionFromEveryStateOneCanBeIn`, declares its own three-state list and hands
it to the guard — proving the guard honours what it is given, a different claim, as its own comment
admits.

CLAIMING cannot be reached deterministically and that is not an oversight: it exists only inside
`AuthorizeClaim`, which holds the operation lease the synthesis itself needs, so a behavioural test
would race for it and usually prove nothing. The claim is structural — *this call site names this
state* — so it is checked structurally, the way
`TestEveryBoundedCallArmsItsBoundBeforeLaunching` checks an ordering a comment could not hold.

### 16 — a leaked semaphore is not a degraded path

`StartBeacon`'s unselected branch returns the warning and hands back the selection gate; only the
first was tested. The gate is a one-token semaphore, so a leak blocks every later `GetLocalIP` and
`StartBeacon` on that Manager until its caller's context is cancelled — for the life of the process.
The test makes the call that needs the gate, under a two-second context, so a regression fails by
name in seconds instead of hanging the package until Go's timeout reports the whole binary.

### 17 — the branch that refuses to invent a path

`resolveAncestorsWith` returns the selection unchanged when `EvalSymlinks` fails, so the source
layer below refuses it with its own coded error rather than this decorator refusing something the
user never chose. Nothing reached it. Its companion test exists because a function that ignored
`eval` entirely and returned its argument would satisfy the first test alone.

### 18 — a subprocess in front of the window, with nothing behind it

`nativeOSPrefersDarkTheme` shells out to `defaults` on the main goroutine **before** `wails.Run`,
outside everything D-107 built to keep a failed start from being silent. With no context, a wedged
`cfprefsd` meant no window, no fatal dialog and no log line. The Windows sibling reads a registry
key and cannot fail that way, which is why nobody asked when the theme read was added.

Two seconds, then light — the same answer every other failure in that function already gives. The
command is injectable so the test bounds a subprocess that really hangs (`sleep 30`, killed at
100 ms) rather than a stub claiming to.

**Pinned twice on purpose.** The behavioural test compiles on darwin alone, so a Windows or Linux
runner that reintroduced a bare `exec.Command` would report green. `TestEveryPreWindowSubprocessIsBounded`
reads all three theme files as text on every platform and refuses the shape.

### 20, 21 — five comments, one of them mechanical

| Where | Claimed | Actually |
|---|---|---|
| `internal/transfer/types.go` | "the one WarningCode this build produces" | a second sat fourteen lines below |
| `internal/transfer/coordinator.go` | (the same, and orphaned) | `go doc` printed it under `unportableNamesWarning` and printed nothing under `beaconWarning` |
| `app.go` | `a.ctx` is "the only one the App stores" | the next line stores `a.commands` |
| `main.go` | an inert window reaches a user on "the two Windows fallthroughs" | macOS is a third, added by Story 3.10's probe |
| `internal/server/handler.go` | "a status is only ever about a token the caller already holds" | false for the 404 branch, reached when nobody holds it |
| `internal/server/lifecycle.go` | a repeated `Stop()` is "a no-op" | it replays the cached diagnostic |

Each states what is now true rather than being trimmed to avoid being wrong: these comments carry
the reasoning that stops the next session removing a guard.

Two are worth reading twice. The `writeStatus` correction **keeps the behaviour** — the conclusion
survives, because this product's threat model already concedes a plain-HTTP transfer on a trusted
LAN, so anyone positioned to read a status is positioned to read the token off the wire. And the
WarningCode count is not re-stated but **moved**: `transfer.WarningCodes()` is the one list, and
`main_test.go`'s cross-language pin reads it instead of declaring its own copy — which it used to,
and admitted: *"a WarningCode added to types.go without also being added here is not reported as
missing."*

## Mutation table

| # | Mutation | Result |
|---|---|---|
| M1 | the text-spacing stylesheet never reaches the document | KILLED — all four cases, on the in-effect guard, naming the element and the measured value |
| M2 | a container that cannot fit wider text | KILLED — at three distinct measured widths across the 320px, 200% and text-spacing cases |
| M3 | delete `Access-Control-Allow-Origin` from `writeStatus` | KILLED — `TestEveryRejectionCarriesTheCrossOriginHeader`, all three rejection shapes |
| M4 | delete `Access-Control-Expose-Headers` | KILLED — `TestSuccessfulDownloadServesHeadersBodyAndOneCompleteEvent` |
| M5 | delete `Accept-Ranges: none` | KILLED — same |
| M6 | `acquireInstanceLockIn` returns `func() {}, true` (the retrospective's own) | KILLED — three named tests |
| M7 | the lock's setup failures stop logging | KILLED — `logged 0 lines, want exactly one` |
| M8 | the lock diagnostic interpolates the error | KILLED — both the exact-line check and the AD-9 path check, which prints the leaked path |
| M9 | narrow `drain`'s synthesis to `stateStaged, stateTransferring` | KILLED — `does not name stateClaiming` |
| M10 | widen the forwarded report to `stateStaged` | KILLED — `want exactly [stateTransferring]` |
| M11 | the synthesis stops building its own event | KILLED — the discriminator breaks loudly rather than mis-attributing |
| M12 | the unselected `StartBeacon` branch stops releasing the gate | KILLED in 2.2s — `the selection gate was never released` |
| M13 | `resolveAncestorsWith` builds a path from a zero-value parent | KILLED — `a path built from a parent that never resolved` |
| M14 | `theme_darwin.go` back to a bare `exec.Command` | KILLED — `TestEveryPreWindowSubprocessIsBounded`, on every platform |
| M15 | a context with no deadline | KILLED — `which bounds nothing` |
| M16 | a third WarningCode that never reaches `validation.ts` | KILLED — the pin reads the package's list now |
| M17 | the WarningCode list empties | KILLED — vacuity guard |

## Verification

Read stage by stage:

- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...` — clean
- `go test -count=1 ./...` — 8 packages ok
- `go doc -all -u ./internal/transfer` — `beaconWarning` and `unportableNamesWarning` each document themselves
- `cd frontend && npx tsc --noEmit -p tsconfig.json` — clean
- `npx vitest run` — 521 passing; `npm run test:browser` — 12 passing (8 before this story)

## Deferrals

None new. Retro items 22 and 23 were out of scope from the spec and stay open, and the six ids the
retrospective deferred are untouched: the Complete event dropped when a stop races finalization, the
wedged ancestor resolution that makes `busy` permanent, the per-snapshot publish goroutine, the
inert second window on macOS, the beacon stop in front of the receiver's first byte, and
`StopBeacon` answering with "did not start".

One of them is now *documented* rather than fixed: `main.go`'s corrected comment names macOS as the
third path to an inert window and says plainly that whether that is the right answer there is an
open question, so the next reader meets the question rather than a claim that it cannot happen.
