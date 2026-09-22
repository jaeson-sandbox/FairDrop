# Evidence — macOS Escape investigation

**Question:** with the browse menu open, does Escape close it on the built macOS app?

**Answer: yes. Escape works.** An automated probe said otherwise and was wrong; the
correction and the reason are below, because the reason is the useful part.

## What actually happened

An owner report of "Escape does nothing" was investigated twice.

1. An investigation pass drove the **keyboard-open** path and reported Escape reaching
   the DOM and closing the menu. It could not drive the pointer-open path and said so,
   and held the finding back from `AGENTS.md` pending confirmation.
2. A second pass used an on-screen `window` keydown probe printing a monotonic counter.
   Escape left the counter unchanged, twice — with focus on the menu container
   (`target=DIV`) and on a menu item (`target=BUTTON`) — while the ArrowDown and ArrowUp
   pressed either side of it advanced it immediately. That was read as proof that macOS
   never dispatches Escape to web content, and committed as a platform fact.
3. **The owner then pressed Escape on real hardware and the menu closed**, with the only
   visible problem being a stray focus ring afterwards (a separate defect, fixed
   separately).

## The correction

Both measurements were accurate about what they touched. A hardware Escape reaches web
content. An Escape synthesized by the computer-use tooling evidently does not, while
synthesized arrow keys do.

The arrow keys were serving as the positive control, and they were **the wrong control**.
They established that *some* synthetic keys arrive — not that synthetic Escape is
faithful to a real Escape. The probe was measuring the harness, not the application, and
the conclusion inverted a correct earlier finding.

## The rule

**A synthetic keystroke is evidence about the harness until a human has pressed the key.**

A *negative* result from a synthetic key — "nothing happened" — cannot tell a dead
feature apart from an undelivered event. A positive control only counts if it exercises
the same delivery path as the thing under test, and a different key is not the same path.
Before recording "this key does nothing on this platform" as fact, have a person press it.

This is the inverse of the lesson the rest of this epic taught. Everywhere else, driving
the built binary caught what green suites missed. Here, driving it produced a false
negative that a green suite and a human both contradicted. Automation is not a stronger
instrument than a suite; it is a *different* one, with its own failure mode.

## Status

No product defect. `handleMenuKeyDown`'s Escape branch is correct and reachable on macOS.
No Cocoa-level work is needed and none should be undertaken on the strength of the
retracted finding.

## Process note

The probe was removed from `main.tsx` but the **built binary still contained it** — the
gate's `wails build` had run before the removal, so "full gate green" described a build
of instrumented code, and the app launched afterwards showing a green `keyprobe:`
overlay. Remove instrumentation first, then build, then gate. A clean rebuild followed.
