---
title: 'Story 3.10: Settle the Release-Blocking Platform Decisions'
type: 'feature'
created: '2026-09-12'
status: 'done'
baseline_commit: '572fc9aea4b96785ea27034bce1426290f4618d7'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Five questions sit between FairDrop and a release, and each has been open long enough that a story was created to hold them. A receiver page that fetches the capability URL cross-origin sees an opaque failure instead of the 404, 410 or 423 the server actually sent, cannot read the filename the response was built to carry, and may have a download manager range-retry a capability that is already consumed (D-018). A dark-mode machine gets one light frame at every launch, because Wails takes a single background colour and nothing reads the OS theme before the options are built (D-055). On macOS the webview's custom scheme is not a secure context, so a whole class of browser API is silently absent there — recorded in the agent instructions, but nowhere a designer would look (D-064). Wails' Windows lock falls through to a full second instance in two reachable cases, so two coordinators, listeners and beacons can run at once (D-088). And a wiring defect in `compose` would panic before any window exists, which in a release build means the process vanishes with nothing on screen and nothing written down (D-107).

**Approach:** Decide each, implement what the decision requires, and move the contract, architecture, agent instructions and tests together rather than leaving a private adapter rule behind.

## Boundaries & Constraints

**Always:** A header change moves `docs/fairdrop-architecture.md`, `docs/fairdrop-spec.md`, the affected tests and this spec together — never a private compatibility rule inside the adapter. The single-instance pins in `main_test.go` stay green. AD-9 holds: no diagnostic this story adds may carry a path, a token or a selected name. Platform code is build-tagged with a seam a test can drive on any host.

**Ask First:** Which headers change, since the set is frozen. What a Windows first instance that Wails cannot detect should do. What a wiring panic should leave behind. Any new public error code or user-visible string — Story 3.11's.

**Never:** Weaken the identical-body rule that keeps a rejected request from saying which kind of wrong it was. Add a dependency; `golang.org/x/sys` is already a direct requirement. Claim a platform behaviour was observed when it was reasoned about. Decide Story 3.11's copy.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cross-origin fetch of a consumed capability | receiver page, `410` | The page reads the status and shows a coded failure | CORS on the error too (D-018) |
| Cross-origin fetch of the live capability | receiver page, `200` | The page can read `Content-Disposition` | `Access-Control-Expose-Headers` (D-018) |
| Download manager retries a range | `Range:` against a consumed token | No range retry is invited | `Accept-Ranges: none` (D-018) |
| Launch on a dark-theme OS | Windows registry / macOS defaults | The native window paints the dark canvas before the webview | Unreadable preference falls back to light (D-055) |
| Launch on a light-theme OS | the same | The light canvas, as today | Same fallback |
| A browser API gated on a secure context | macOS webview | Routed through Go, or recorded as a platform limit with its cost | Documented, not discovered (D-064) |
| Wails' Windows lock falls through | elevated owner, tight double launch | No competing coordinator, listener or beacon starts | Backstop refuses (D-088) |
| Two logged-in Windows users | one instance each | Allowed and recorded as deliberate | Per-user scope stated |
| `compose` panics before any window | a wiring regression in a release build | Something a user can find says so | Never a silent vanish (D-107) |

</frozen-after-approval>

## Code Map

- `internal/server/handler.go:247` `writeStatus` — sets `Content-Type`, `Cache-Control`, `X-Content-Type-Options` and no CORS header, which is the whole of D-018's error half. `:256` `writeDownloadHeaders` sets `Access-Control-Allow-Origin: *` but exposes no header and advertises no range policy. The identical-body rule at `:243` is the thing not to weaken.
- `internal/server/handler_test.go:1016` `renderResponse` — the header list every response assertion renders through; a header the list does not name is a header no test can see change.
- `docs/fairdrop-architecture.md:123` — the response-rules bullet that fixes the header set, and `:125` "No range/resume behavior in v1", which is what `Accept-Ranges: none` would state on the wire. `docs/fairdrop-spec.md:107,126` carry the same set.
- `main.go:156` `BackgroundColour: &options.RGBA{R: 0xF7, G: 0xF0, B: 0xE7, A: 1}` with the comment that names the coupling, and `main_test.go:67` which pins it to the literal `--color-canvas: #F7F0E7;` in `frontend/src/style.css:30`. The dark token is `style.css:140` `#1C1916`.
- `main.go:125` `appOptionsWithLockProbe(app, nativeSingleInstanceLockUsable)` and `:178` `singleInstanceOption` — the existing build-tagged probe seam Story 3.7 added for macOS (`single_instance_darwin.go`, `single_instance_other.go`). A theme read and a Windows backstop both belong on seams shaped like this one, so a test can drive them on any host.
- Wails v2.15.0 `internal/frontend/desktop/windows/single_instance.go` `SetupSingleInstance` — upstream and unchangeable: any `CreateMutex` error other than `ERROR_ALREADY_EXISTS` skips both the exit and `createEventTargetWindow`, and `FindWindowW` returning 0 falls through the same way. The mutex name carries no `Global\` prefix, so it is session-scoped.
- `main.go`'s `compose` and `newBoundApp` — what runs before `wails.Run`, and therefore before any window could show a message (D-107).
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md` — where a platform limit belongs for D-064; `AGENTS.md:105-107` already warns implementers, which is the half that exists.

## Tasks & Acceptance

**Execution:**
- [x] `internal/server/handler.go` — the decided header changes, with `renderResponse` widened so each is visible to every existing response assertion.
- [x] `docs/fairdrop-architecture.md`, `docs/fairdrop-spec.md` — the same set, moved together, with the reasoning recorded.
- [x] `main.go` plus build-tagged siblings — read the OS theme before the options are built and paint the matching canvas; a seam a test drives on any host, and the existing pin extended to both tokens.
- [x] `main.go` plus a Windows sibling — the decided answer to Wails' fallthrough, keeping the existing single-instance pins green.
- [x] `main.go` — the decided answer for a panic before any window exists.
- [x] `EXPERIENCE.md` — the macOS secure-context limit and what it costs a user.
- [x] `evidence-3-10-settle-the-release-blocking-platform-decisions.md`, the five ids discharged, `epics.md` kept in step.

**Acceptance Criteria:**
- Given a cross-origin receiver page, when it fetches a consumed capability, then it can read the coded status rather than an opaque failure, and a test renders that header on the error path.
- Given a dark-theme OS, when the options are built, then the background is the dark canvas token, and a test drives both themes through the seam without needing that OS.
- Given Wails' lock falling through on Windows, when a second instance starts, then no second coordinator, listener or beacon runs, and the existing pins still pass.
- Given a panic in `compose`, when it happens in a release build, then the failure leaves something a user can find, proved by a test rather than by argument.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-10-settle-the-release-blocking-platform-decisions.md](evidence-3-10-settle-the-release-blocking-platform-decisions.md), created with the implementation.

## Spec Change Log

**2026-09-12 (approval).** All three Ask First items answered before implementation.

*D-018 — all three headers.* `Access-Control-Allow-Origin: *` joins `writeStatus` so a receiver
page reads the coded status instead of an opaque failure;
`Access-Control-Expose-Headers: Content-Disposition` joins the 200 so that page can read the
filename the response was built to carry; `Accept-Ranges: none` states on the wire what the
architecture already says in prose, so no download manager range-retries a consumed capability.
The identical-body rule is untouched.

*D-088 — a per-user backstop lock.* FairDrop holds its own lock under `os.UserConfigDir()` before
`wails.Run`. A process that cannot acquire it exits without composing a coordinator, listener or
beacon, so Wails' own restore still works where its lock worked and nothing competes where it fell
through. Per-user on purpose: two logged-in Windows users keep one instance each.

*D-107 — a native message box.* The owner chose visibility over a written record: a wiring panic
before any window exists shows a platform dialog rather than writing a line to a file. That is new
platform code on a path no test can reach through production, so the dialog sits behind a seam a
test drives, the message is a fixed literal (AD-9 — no path, no panic value), and the recover is
scoped to composition rather than wrapped around `wails.Run`, which would swallow runtime panics
the process should not survive.

## Design Notes

**The CORS change is smaller than it looks.** A cross-origin page can already send the request; what it cannot do is read the answer. Adding `Access-Control-Allow-Origin: *` to `writeStatus` lets it read a status it could otherwise only infer from timing, and every one of those statuses is about a token the caller already holds. The identical-body rule is untouched: the body stays empty and the same for every rejection, so nothing new distinguishes a guessed token from a consumed one beyond the status the contract already defines.

**The theme read is two small platform functions behind one seam.** Windows is a single `registry` read of `AppsUseLightTheme` under `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`; macOS is `AppleInterfaceStyle` being `Dark` or absent. `golang.org/x/sys` is already a direct requirement, so neither adds a dependency. Both answer "light" when they cannot tell, which is the honest default: light is what ships today, so an unreadable preference changes nothing rather than guessing.

**A backstop lock is per-user by design.** Wails' mutex is session-scoped, so two logged-in Windows users already get one instance each, and that is the right behaviour — each user has their own listener, their own port and their own transfer. The backstop should hold the same scope rather than reaching for a machine-wide lock that would need privileges and let one user deny another the app.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 300s ./...`; `go test -count=1 -race -timeout 1200s ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent, per AGENTS.md's pre-flight
- `cd frontend && npx vitest run`; then, alone, `wails build`
- The native run read with `gh run view --json conclusion,jobs`
- Mutations: drop the CORS header from `writeStatus`; return light from the dark-theme seam; let the backstop admit a second instance; remove what a wiring panic leaves behind — each must fail a named test
