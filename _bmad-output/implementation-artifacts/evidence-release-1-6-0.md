# FairDrop 1.6.0 — release evidence

Spec: [spec-release-1-6-0.md](spec-release-1-6-0.md). Feature evidence: [evidence-phone-to-desktop-receiving.md](evidence-phone-to-desktop-receiving.md).

## Scope

- **Shipped feature:** phone-to-desktop receiving (PR #13, forward-merged with 1.5.1 in PR #14).
- **Defect fixes before release, each with a failing test first:**
  - The Idle **Send multiple items** button sat flush against the browse pill. Reported by the owner from the built app; [evidence](evidence-idle-secondary-button-spacing-fix.md).
  - The upload inactivity deadline is now re-armed before every read (PR #16). Windows CI on the first 1.6.0 merge commit (run 38199503856) failed a healthy upload; the fix is in the receive evidence.
- **Version:** product fields bumped to 1.6.0; README and `docs/release-notes-1.6.0.md` written.

## Verification

- **Owner hands-on (2026-10-11):** a real phone upload on a build of `main` worked. The owner reported only the spacing defect above.
- **Local gate on the release tree:** the full ordered gate passed (1270 unit tests, 127 browser tests, receiver-live and upload-live), and the receive Go mutation inventory killed 49/49.
- **Exact-head CI:**
  - PR #15 head `1aa6d40`: run 38198273836 passed.
  - Its `main` merge commit `2a6ceee`: run 38199503856 failed on Windows (the deadline defect), so that commit was **not tagged**.
  - PR #16 head `48e2d8c`: run 38201274336 passed.
  - `main` merge commit `26f083c`: run 38202527255, all three jobs passed. The tree is identical to the verified PR head.
- **Tag:** v1.6.0 points at `26f083c3ae80da594a973154eb0278eb438adf40`.
- **Release run:** https://github.com/jaeson-sandbox/FairDrop/actions/runs/38203692590, all six jobs passed. Every conclusion was read from GitHub JSON.
- **Downloaded assets:** both `shasum -a 256 -c` checks passed.
  - macOS ZIP: `82997772256defc107c743c7913765c952bc944de415964acbe927eaf5a2e28a`
  - Windows exe: `7eda5d42bd9d21c9f33ecfc997c8c346e16114273b9fb1a3bc868d48e8137b47`
- **macOS bundle:** both version fields read 1.6.0, and `codesign --verify --deep --strict` passed (ad-hoc signature). The Windows asset is a PE32+ x86-64 GUI executable.
- **Release body:** set to `docs/release-notes-1.6.0.md` and confirmed equal before publication.

## Published — 2026-10-12

https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.6.0, published 2026-10-12T04:41:07Z. It is non-draft and non-prerelease, and `releases/latest` returns v1.6.0. The owner authorized it on 2026-10-11.

**Still unobserved:**
- Receiving on Windows by hand, including the writability refusal for a standard-user account, which CI cannot exercise.
- The 1.6.0 downloaded app itself; the owner's hands-on test used a local build of `main` before the two fixes.
