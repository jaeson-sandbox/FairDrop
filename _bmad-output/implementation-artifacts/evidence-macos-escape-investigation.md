# Evidence — macOS Escape investigation

**Question:** with the browse menu open, does a DOM `keydown` for Escape fire at all
inside the built macOS app?

**Answer: no. Escape is never dispatched to web content. Arrow keys are.**

## Method

An on-screen probe was added temporarily to `frontend/src/main.tsx`: a `window`
`keydown` listener rendering a monotonic counter, the `key`, and the event target into
a fixed overlay. The app was rebuilt with `wails build` and driven at the tip
(`dfcb64f`).

The counter is the point. A negative result on its own proves nothing — "Escape did
nothing" is indistinguishable from "the keystroke never left the harness". A counter
that demonstrably advances for a *different* key pressed moments later, in the same
state, through the same path, is what turns the silence into a measurement.

## Raw result

| Step | Key sent | Probe after | Menu |
|---|---|---|---|
| Pointer-open the menu, then press Escape | Escape | `keyprobe: (none yet)` | still open |
| Press ArrowDown from that state | ArrowDown | `keyprobe #1: key="ArrowDown" target=DIV` | open, item marked |
| Press Escape again | Escape | `keyprobe #1: key="ArrowDown" target=DIV` — **unchanged** | still open |
| Press ArrowUp | ArrowUp | `keyprobe #2: key="ArrowUp" target=BUTTON` | open |

Escape was pressed twice and produced **no event either time**, with focus on the menu
container (`target=DIV`) and on a menu item (`target=BUTTON`) respectively. Both arrow
presses advanced the counter immediately.

## Conclusion

macOS routes Escape up the responder chain as `-[NSResponder cancelOperation:]` rather
than dispatching it to web content. Wails overrides that method in `WailsContext.m`,
but the override returns early only when `disableEscapeExitsFullscreen` is set *and*
the window is fullscreen — neither applies — so Wails is not the culprit. The key
simply never reaches the page.

**Therefore the ARIA menu pattern's Escape has never worked in this app on macOS, and
no JavaScript change can make it work.** A listener cannot hear an undispatched event.
`handleMenuKeyDown`'s Escape branch is correct code that is unreachable on this
platform, and remains correct and reachable on Windows/WebView2. It should not be
rewritten.

## A prior pass concluded the opposite

An earlier investigation reported that Escape *does* reach the DOM and closes the menu.
It exercised only the **keyboard-open** path, could not drive the pointer-open path
(the computer-use tooling refused to click a control carrying `aria-haspopup="menu"`,
and display-scope control was unavailable in that session), and said so explicitly
rather than papering over it. Holding the finding back from `AGENTS.md` pending
confirmation was the correct call and is why the wrong fact never landed.

Two lessons, recorded because they generalise:

1. **A negative result needs a positive control in the same breath.** The ArrowUp that
   advanced the counter is what makes "Escape produced nothing" a measurement instead
   of an absence of evidence.
2. **An investigation that cannot reach the reported reproduction has not reproduced
   it**, whatever else it establishes. The reported case was a pointer-opened menu; the
   pass that could not open the menu with a pointer had not tested the report.

## Process note

The probe was removed from `main.tsx` (the source diff is clean) but the **built
binary still contained it** — `build/bin/fairdrop.app` rendered the green
`keyprobe:` overlay on launch afterwards. The gate's `wails build` had run before the
instrumentation was removed, so "full gate green" described a build of instrumented
code. Remove instrumentation *first*, then build, then run the gate. A clean rebuild
was performed after this investigation.

## What was NOT done

No fix. There is nothing to fix in the frontend: the handler is correct and the event
does not arrive. Whether to pursue a Cocoa-level hook, bind Escape through the
application menu, or accept that macOS dismissal happens by click-outside, Tab-away or
a second press of the trigger is an open product decision, not a defect with an obvious
repair. It is recorded in `AGENTS.md` item 5 so the next reader starts from the
measurement rather than from the handler.
