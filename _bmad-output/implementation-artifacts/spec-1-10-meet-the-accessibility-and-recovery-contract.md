---
title: 'Story 1.10 — Meet the Accessibility and Recovery Contract'
type: 'feature'
created: '2026-08-31'
status: 'in-progress'
review_loop_iteration: 0
baseline_commit: 'f7338af172c1225cf3935d6bbd8eca72ace77e87'
context:
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1.9 built the views but wired none of the accessibility contract. The status announcer is pre-mounted and permanently empty, every state heading carries `tabIndex={-1}` and nothing ever calls `.focus()`, and seven registered copy strings — both firewall recoveries, both receiver-help texts, the cancel-winning summary, the full-name disclosure, the external promise — render nowhere. A keyboard or screen-reader user is told nothing when the transfer changes state, and a user whose network or firewall blocks the transfer is offered no guidance.

**Approach:** Implement the `EXPERIENCE.md` routing table as the single owner of every transition: each one either moves focus to a state heading or writes the atomic polite announcer, never both. Add the recovery and help surfaces the registry already holds copy for, the assistive progress throttle, and the visual floor — forced colors, reduced motion, and the published contrast proof.

## Boundaries & Constraints

**Always:** Give every transition exactly one announcement owner from the routing table. Prove a focus target exists before focusing it. Replace the announcer's text, never append. Throttle assistive progress speech to at most one update per five seconds and only after 10 percentage points or 10 MiB of new wire bytes, independent of visual refresh, cancelled by any terminal outcome. Keep `role="progressbar"` with a finite `aria-valuenow` for known-positive totals only. Under `prefers-reduced-motion: reduce`, remove spatial motion and continuous animation while keeping text, pattern, bytes, and state legible. Under `forced-colors: active`, let system colors supersede the palette everywhere except the tested QR substrate. Keep every activation target at or above 44px and every layout reflowing at 320 CSS pixels under 200% text.

**Ask First:** Any Go, binding, or `main.go` change; any new runtime dependency; any string not already in the copy registry, or any wording change to one; any change to Story 1.8's reducer transitions, correlation, or reset rules; adding a lifecycle timer or persistence; changing a `DESIGN.md` token value.

**Never:** Never announce focused content through a live or alert region as well, never use `role="alert"` on a path that also moves focus, never speak throughput, never let the announcer become an event log, never move focus a second time on reset after a terminal outcome, never trap focus outside an OS dialog, never predict or restyle the OS firewall prompt, never render `cancelled` as an Error, never disable forced-color adjustment outside the QR bitmap, and never introduce a branded receiver page, protocol change, or cloud fallback.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Focus-owned transition | Stage pending, Stage success, `transfer-started`, terminal outcome, validation failure | Focus moves once to that state's heading or panel; the announcer stays silent | A missing target is proven absent before the call, never focused blindly |
| Announcer-owned transition | `beacon_warning`, copy success, throttled progress, cancel requested | The announcer's text is replaced once; focus does not move | Announcing replaces, never appends |
| Reset after terminal | `transfer-reset` following Done or Error | No focus move and no announcement; the retained node stays mounted | Focus already inside the outcome stays there |
| Cancel wins | `transfer-reset` with no terminal event | Focus moves to the Idle summary carrying `copy.cancel.won` | Never rendered as an Error |
| Dismiss retained | Dismiss activated | Focus moves to the Idle instruction, as the sole owner | The instruction is proven present first |
| Progress speech | Accepted snapshots over time | One update at start, then at most every 5s and only after 10pp or 10 MiB | A terminal outcome cancels anything queued; throughput never spoken |
| Cancel pending | Cancel activated from Pending, Staged, or Transferring | The control keeps focus, gains `aria-disabled="true"`, and swaps to the pending label | Repeat activation issues no second command |
| Recovery guidance | Idle and Staged | Firewall preflight plus platform recovery, and receiver help covering wrong/expired link, competing opener, changed source, guest isolation | Copy comes from the registry verbatim |
| Long or bidi name | Mixed LTR/RTL, emoji, bidi controls, long unbroken names | Bidi-isolated, with full value reachable through `copy.name.show_full` | Accessible name matches the visible name |
| Reduced motion / forced colors | `prefers-reduced-motion: reduce`, `forced-colors: active` | Motion removed, system colors applied, QR substrate exempt | No state distinguished by color alone |

</frozen-after-approval>

## Code Map

- `.../EXPERIENCE.md:206` — the announcement-ownership routing table; `:197` interaction primitives and the speech throttle; `:228` firewall preflight and recovery; `:255` the accessibility floor. This is the contract; read it before writing.
- `.../DESIGN.md:135` — the published contrast table and the instruction to re-run unrounded checks when adjacent surfaces change. Muted now sits on elevated (4.504478:1 light, 6.569759:1 dark) and that pair is unpublished — add it.
- `frontend/src/App.tsx:36` — the announcer, pre-mounted, atomic, and never written. `phaseView` at `:47` is where a transition becomes a rendered view.
- `frontend/src/ui/IdleView.tsx:62`, `StagePendingCard.tsx:35`, `StagedView.tsx:56`, `TransferringView.tsx:31`, `OutcomePanel.tsx:35` — every focus target already carries `tabIndex={-1}`; none is ever focused.
- `frontend/src/ui/StagedView.tsx:92` — the direct URL is a `div` with `role="textbox"`; settle the element, not just its name.
- `frontend/src/ui/copy.ts` — `help.differentLan`, `help.receiverHttp`, `firewall.windowsRecovery`, `firewall.macosRecovery`, `cancel.won`, `name.showFull`, `external.promise` are registered and rendered nowhere. Render them; add no new strings.
- `frontend/src/transfer/selectors.ts` — `selectVisibleError` and `selectRetainedOutcome` are dead and contradict their replacements; this story may remove them.
- `frontend/src/transfer/state.ts:159` — `reduceLifecycle`; a cancel-winning reset reaches Idle with no retained outcome, which is what `copy.cancel.won` must key off.
- `frontend/src/ui/TransferringView.tsx:61` — the three progress presentations and their existing ARIA; verify rather than rebuild.
- `frontend/src/ui/styles.test.ts` — stylesheet guarantees are proved against the stylesheet because jsdom performs no layout; forced colors and reduced motion belong there too.
- `frontend/package.json` — `framer-motion` is locked and unused; `frontend/src/assets/fonts/` holds a face nothing references.

## Tasks & Acceptance

**Execution:**
- [ ] `frontend/src/ui/announce.ts` — one owner map from transition to focus target or announcer text, so the routing table exists once rather than per view.
- [ ] `frontend/src/App.tsx` — drive the announcer and the single focus move from state transitions, proving each target exists first.
- [ ] `frontend/src/ui/progressSpeech.ts` — the assistive throttle: start once, then 5s plus 10pp or 10 MiB, cancelled by a terminal outcome, never speaking throughput.
- [ ] `frontend/src/ui/IdleView.tsx` — platform firewall recovery and receiver help; `copy.cancel.won` as the focused cancel-winning summary; heading order that does not open the document on an `h2`.
- [ ] `frontend/src/ui/StagedView.tsx` — recovery help beside the handoff; the URL as a readonly form control rather than a `div` with a textbox role; `copy.name.show_full` full-value access.
- [ ] `frontend/src/ui/{StagePendingCard,TransferringView,OutcomePanel}.tsx` — `aria-disabled` and focus retention on a pending cancellation.
- [ ] `frontend/src/style.css` — a `forced-colors: active` block, `prefers-reduced-motion: reduce` behaviour, and the focus indicator token.
- [ ] `frontend/src/transfer/selectors.ts` — remove `selectVisibleError` and `selectRetainedOutcome` with their tests; they contradict their replacements.
- [ ] `frontend/package.json`, `frontend/src/assets/fonts/` — drop `framer-motion` and the unreferenced face, or use them deliberately.
- [ ] `frontend/src/ui/*.test.tsx`, `styles.test.ts` — one test per routing-table row, the throttle's boundaries, target existence, forced colors, reduced motion, and the unrounded contrast proof including muted on elevated.

**Acceptance Criteria:**
- Given each row of the routing table, when its transition occurs, then exactly one mechanism announces it and the other is provably silent.
- Given any `.focus()` the app performs, when the target is absent, then the call never happens and the transition still completes.
- Given a run of accepted progress snapshots, when assistive output is produced, then it obeys both the time and the change thresholds while visual refresh stays unthrottled.
- Given every authored token pair the views actually place together, when contrast is computed unrounded, then each meets its WCAG 2.2 AA ratio and the pair is published in `DESIGN.md`.
- Given forced colors and reduced motion, when the UI renders, then no state or available action is distinguished by color or motion alone.

## Spec Change Log

## Design Notes

The announcer and the focus move are alternatives, never a pair. The failure this story exists to prevent is a screen reader hearing a transition twice — once from the focused heading and once from a live region — which is why the owner map is one table rather than a decision spread across five views.

`copy.cancel.won` is the one retained-outcome shape Story 1.8 deliberately refused and Story 1.9 left absent. A cancel-winning reset arrives as `transfer-reset` with no preceding terminal event, so the reducer lands on plain Idle; this story owns making that state visible and focused without adding a third retained outcome kind to the reducer.

Progress speech is throttled separately from painting. A snapshot may update the meter every time it arrives and still be silent, and the two thresholds are both required: five seconds elapsed **and** a meaningful change.

The known-empty and unknown-total progress modes already omit `aria-valuenow` correctly. Verify and pin them rather than rebuilding; the work here is the throttle and the announcement owner, not the roles.

## Verification

**Commands:**
- `cd frontend && npm.cmd test` — expected: every existing test still passes alongside the new routing, throttle, and stylesheet tests.
- `cd frontend && npm.cmd run build` — expected: `tsc` and Vite production build clean.
- `go test -count=1 ./... && go vet ./...` — expected: unchanged; this story touches no Go.
- `wails build` — expected: bindings regenerate unchanged and the binary builds.
- `git diff --check` — expected: clean.

**Manual checks (if no CLI):**
- Drive one full transfer with the keyboard only, with a screen reader running, and confirm each transition is announced exactly once and focus never jumps twice.
- Render Staged at 320 CSS pixels with 200% text and forced colors on, and confirm the QR stays scannable while every action stays reachable.
