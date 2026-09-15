---
title: 'Story 3.12: Capture the Accessibility Evidence a Runner Can Produce'
type: 'chore'
created: '2026-09-14'
status: 'done'
baseline_commit: 'cb4c9e9'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop's accessibility floor is asserted against text, not against anything rendered. jsdom performs no layout and evaluates no media query, so 320-pixel reflow, the 44-pixel target floor, 200% text and forced colors are all proved by reading the stylesheet and trusting that a browser would agree (D-068). The QR substrate's `forced-color-adjust: none` exemption is in the same position but worse: `DESIGN.md` permits it *"only after native scan evidence confirms it remains readable"*, and no such evidence exists — the rule is applied on the strength of an argument (D-065). Separately, Story 1.10 — the story whose acceptance criteria *are* the epic's accessibility gate — was reviewed by one adversarial layer instead of three, because a rate limit killed the other two (D-109).

**Approach:** Render the thing and measure it. A real browser engine in CI, driving the built frontend at the sizes and settings the contract names, failing on measurements rather than on substrings. Capture the forced-colors QR rendering as the artefact `DESIGN.md` asks for, and say plainly which half of that evidence a runner still cannot produce. Then re-run the two review layers Story 1.10 never got.

## Boundaries & Constraints

**Always:** A rendered check fails on a measured value and names the element and the measurement. What a runner cannot observe is recorded as unverified in `release-evidence.md`, never implied. The existing stylesheet-text assertions stay: they run everywhere and catch a token edit in milliseconds, and the rendered pass is a second opinion, not a replacement. Any browser download is pinned and cached like every other toolchain here.

**Ask First:** The browser dependency itself — this is the first non-Go, non-vite tool the gate has needed. Any change to what `DESIGN.md` permits for the QR exemption.

**Never:** Claim a QR code was scanned. Loosen a contrast, reflow or target rule because a rendered measurement disagrees — a disagreement is a finding. Run the browser suite concurrently with `wails build`. Let the rendered suite become the only proof of a rule the text suite already pins.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Staged at 320 CSS pixels | rendered, one column | No horizontal page scroll, nothing clipped | Fails naming the overflowing element (D-068) |
| Staged at 200% text | rendered | Same, with no clipped control | Fails naming the element |
| Every interactive control | rendered box measured | At least 44×44 CSS pixels | Fails naming the control and its size |
| Forced colors active | rendered | System palette honoured everywhere but the QR substrate | Fails if a second selector takes the exemption |
| Forced colors, QR panel | rendered and captured | The bitmap and quiet zone stay distinguishable; the capture is kept | Capture is evidence, not a scan (D-065) |
| A camera reading that bitmap | — | Not attempted | Recorded unverified in the release record |
| Story 1.10's diff | `f7338af..HEAD` for that story | Two independent review layers run, findings revalidated against current code | Fixed here or routed with a reason (D-109) |

</frozen-after-approval>

## Code Map

- `frontend/src/ui/styles.test.ts` — the text-based floor: it reads `style.css`, recomputes every contrast ratio from declared tokens, and asserts the `forced-color-adjust` declaration appears exactly once. All of it stays; the rendered suite is a second opinion on the rules jsdom cannot evaluate.
- `frontend/src/style.css` — the media queries jsdom never evaluates: `prefers-color-scheme`, `forced-colors`, `prefers-reduced-motion`, and the 320-pixel column rules.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md` — the rule that gates the QR exemption on native scan evidence, and the contrast tables Story 3.9 moved. `EXPERIENCE.md`'s Accessibility Floor and Compatibility and Evidence Gates are where an observation is recorded.
- `_bmad-output/implementation-artifacts/release-evidence.md` — rows 10 to 13 are the manual accessibility checks. Whatever this story automates stops being a manual row; whatever it cannot reach stays one, and the file's own test refuses a pass without a run id or a named reviewer.
- `.github/workflows/verify.yml` — the three jobs and their caching. A browser install belongs on the desktop runners only; the Linux adapter job is Go-only by design and must stay that way.
- `frontend/vite.config.ts`, `frontend/package.json` — where a second test project and its dependency would live.
- Story 1.10's spec and evidence files name what its one completed layer found; `review-layer-prompts-1-6.md` holds the layer prompts (D-109).

## Tasks & Acceptance

**Execution:**
- [x] `frontend/` — a browser-run test project alongside the jsdom one, pinned and cached, running only the checks that need layout.
- [x] Rendered checks for 320-pixel reflow, 200% text, the 44-pixel target floor, and forced colors, each failing on a measurement that names the element.
- [x] The forced-colors QR capture, retained as an artefact, with what it does and does not prove stated beside it.
- [x] `.github/workflows/verify.yml` — the browser step on the desktop runners, never on the Linux adapter job, and never beside `wails build`.
- [x] `release-evidence.md` — the rows this story automates move to machine-verified with their run identity; the camera scan and the screen-reader pass stay optional and unverified.
- [x] D-109: run the two missing review layers against Story 1.10's change, revalidate every finding against current code, and fix or route each with a reason.
- [x] `evidence-3-12-capture-the-accessibility-evidence-a-runner-can-produce.md`, the three ids discharged, `epics.md` kept in step.

**Acceptance Criteria:**
- Given the built frontend at 320 CSS pixels and at 200% text, when the rendered suite runs, then a clipped control or a horizontal page scroll fails by name rather than by substring.
- Given forced colors, when the page renders, then only the QR substrate keeps its exemption, and the capture proving it is retained.
- Given the claims a runner cannot make, when the release record is read, then the camera scan and the screen-reader pass are optional and unverified, and nothing implies otherwise.
- Given Story 1.10's diff, when both layers have run, then every finding is fixed here or carries an id and an owner.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-12-capture-the-accessibility-evidence-a-runner-can-produce.md](evidence-3-12-capture-the-accessibility-evidence-a-runner-can-produce.md), created with the implementation.

## Spec Change Log

**2026-09-14 (approval).** The owner approved vitest browser mode with a Playwright provider: a
second vitest project running only the layout-dependent checks in real Chromium, beside the
existing jsdom suite rather than replacing it. Chosen over a standalone Playwright suite because
the component states this needs are mounted directly rather than navigated to, and over keeping
these as human rows because the accessibility floor being proved against stylesheet text is the
gap the Epic 1 retrospective named.

The browser download is pinned and cached like the Wails CLI, on the two desktop runners only. The
Linux adapter job stays Go-only: it exists to execute the `O_PATH` branch and is explicitly not
release proof, and a browser there would blur that.

## Design Notes

**The rendered suite is a second opinion, not a replacement.** The text assertions run in milliseconds on every platform and catch a token edit immediately; the rendered ones need a browser download and only run where one exists. Deleting the first because the second is "more real" would trade a fast, universal check for a slow, conditional one.

**A capture is not a scan.** A screenshot proves the substrate still renders as light-on-dark under forced colors. It does not prove a phone camera decodes it, which is what `DESIGN.md` actually gates the exemption on. Both halves get said: the capture is recorded as what it is, and the scan stays an optional unverified row rather than quietly becoming "covered".

**D-109 is a review, not a feature.** Its output is findings, and findings are either fixed here or become ids with owners. A layer that reports nothing is itself a result worth recording — Story 1.10 is two epics old, and much of what those layers would have found has since been fixed by other stories.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...` — clean
- `cd frontend && npx vitest run` (jsdom) and the rendered project separately; then, alone, `wails build`
- The native run read with `gh run view --json conclusion,jobs`
- Mutations: widen a control below 44 pixels; let a second selector take the forced-colors exemption; force a horizontal scroll at 320 pixels — each must fail a named rendered check
