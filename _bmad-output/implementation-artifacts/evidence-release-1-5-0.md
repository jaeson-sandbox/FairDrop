# FairDrop 1.5.0 release evidence

## Implementation diff audit

Baseline: `f03537b` (`docs(release): specify FairDrop 1.5.0`, on top of `8712760`).

- `wails.json` product version, `frontend/package.json` root version, and the two root version fields in `frontend/package-lock.json` changed from 1.4.0 to 1.5.0. A structured JSON comparison against the baseline found no other metadata or dependency-graph differences (below). The unrelated `expect-type` `1.4.0` entry in the lockfile was not touched.
- `README.md`: current release is now v1.5.0 (`https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.5.0`). The "In development" section and its "not part of the published v1.4.0 release" caveat are removed; **Send multiple items** is described as a released capability (2-16 items by native drop or list with **Add Files** / **Add Folder**, one streamed `FairDrop.zip` with numbered `FairDrop/NN-<name>` members, whole selection validated first, one failing member fails the transfer, single item unchanged). Using-FairDrop steps 2, 4 and 5 and the Send Again memory sentence mention selections. Button labels were verified against `frontend/src/ui/copy.ts` (`selection.open` = "Send multiple items", `addFiles` = "Add Files", `addFolder` = "Add Folder", `send` = "Send"). Unrelated sections are unchanged.
- `docs/release-notes-1.5.0.md` (new): collection feature per `_bmad-output/specs/spec-multiple-selected-items/SPEC.md` and `selection-contract.md`; single-item behavior stated as unchanged from 1.4.0; trusted-LAN plain-HTTP scope; unsigned Windows / ad-hoc macOS, no notarization, auto-update or Linux packaging; checksums prove integrity, not authenticity. No claim of snapshots, more than 16 items, multiple receivers, phone-to-desktop receiving, recovery, text sharing, history or resume. Historical release notes are unchanged.
- No transfer code, native template, workflow, dependency, generated binding, icon or historical release-note file was edited. Untracked `build/.DS_Store` in the root checkout was not touched.

## Focused verification

- `export PATH="/opt/homebrew/bin:$HOME/go/bin:$PATH"; go test -count=1 -run 'TestReleaseIdentity|TestTheVersionIsStatedOnceAndFollowedEverywhere|TestReleaseWorkflow|TestTheDraftReleaseNotesStateWhatTheBuildIsNot|TestTheReleaseWorkflowSurvivesARetryAndCannotRaceItself' -v .` (go1.27.0 darwin/arm64) — 8 tests, 8 passed (`ok fairdrop 0.459s`): `TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo`, `TestReleaseWorkflowAssertsTheTagMatchesProductVersionBeforeBuilding`, `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles`, `TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction`, `TestTheVersionIsStatedOnceAndFollowedEverywhere`, `TestReleaseWorkflowHoldsTheSmallestTokenAndChecksWhatItPublishes`, `TestTheDraftReleaseNotesStateWhatTheBuildIsNot`, `TestTheReleaseWorkflowSurvivesARetryAndCannotRaceItself`.
- `go test -count=1 ./...` — all 9 packages `ok` (fairdrop, internal/network, internal/qr, internal/server, internal/source, internal/stream, internal/transfer, scripts, scripts/mutationverdict); no failures.
- Structured Python JSON comparison of the four product-version fields against `git show HEAD:<file>` (HEAD = `f03537b`), asserting 1.4.0 -> 1.5.0 at exactly those fields and equality of all remaining JSON:

  ```text
  wails.json: 1 product version field(s) changed; all other JSON identical
  frontend/package.json: 1 product version field(s) changed; all other JSON identical
  frontend/package-lock.json: 2 product version field(s) changed; all other JSON identical
  ```

- `git diff --check` — passed.
- Manual review of `docs/release-notes-1.5.0.md` and README against the multiple-selected-items SPEC, `selection-contract.md`, `docs/fairdrop-architecture.md` and `copy.ts` for count/size preview, numbered member names, atomic validation, and no per-item path disclosure. The automated draft-notes test checks the generic notes built by `release.yml`, not this file.

Not run by the implementer: frontend Vitest/browser suites, `wails build`, race suite, staticcheck, platform preflight (no frontend or Go source changed; these are the orchestrator's sequential gate).

## Release proof PENDING with orchestrator

The following have **not** been verified here and no release is claimed: canonical sequential local gate, platform preflight, exact-head native Windows/macOS/Linux CI (conclusions read job by job for the tip headSha), merge, tag `v1.5.0`, tag-triggered release workflow, four downloaded assets with matching SHA-256 sidecars, native bundle metadata (`CFBundleShortVersionString` 1.5.0), replacement of the generic draft body with the exact contents of `docs/release-notes-1.5.0.md`, publication state (non-draft, non-prerelease, latest) and release URL.
