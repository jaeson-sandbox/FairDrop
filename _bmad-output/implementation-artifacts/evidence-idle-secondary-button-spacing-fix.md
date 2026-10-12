# Defect fix: "Send multiple items" touches the browse pill

**Observed (owner, built macOS app, 2026-10-11):** In the Idle drop zone, the secondary **Send multiple items** button rendered flush against the bottom of the **Choose File or Folder** pill.

**Cause:** `.fd-drop-zone__inner` is a grid with no row gap. Its children are spaced only by their own margins, and the collection entry button (`[data-draft-entry]`) had none.

**Failing test first:** `frontend/browser/accessibility.test.tsx`, "Send multiple items sits clear of the browse pill (defect fix)". In the real `.fd-app` shell at 1024x900 and at the 640x480 minimum, it measures the button's top edge against the pill's bottom edge and requires a gap of 12–20px.
- **Red, before the fix:** both cases failed with "the secondary button's top edge sits 0.0px below the pill's bottom edge: expected 0 to be greater than or equal to 12".
- **Green, after the fix:** both cases pass. The existing "drop zone packs its contents together" test also still passes.

**Fix:** `.fd-drop-zone__inner [data-draft-entry] { margin-top: var(--spacing-3); }`, which is 12px.

**Mutation:** removing the rule is the red run above, and the test names the 0px gap. The upper bound of 20px kills an oversized margin.

**Unverified:** the gap has been measured in Chromium only. Seeing it in WKWebView needs the built app.
