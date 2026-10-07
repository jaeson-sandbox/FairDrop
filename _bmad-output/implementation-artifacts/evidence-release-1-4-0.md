# FairDrop 1.4.0 release evidence

## Implementation diff audit

Baseline: `7fdb98ede732d15a1f1129168e9ae9d805e37900`.

- `wails.json` product version, `frontend/package.json` root version, and the two root version fields in `frontend/package-lock.json` changed from 1.3.1 to 1.4.0. A structured JSON comparison against the baseline found no other metadata or dependency-graph differences.
- `README.md` links to the v1.4.0 release page and describes the receiver landing page, explicit Download, and Send Again as current behavior. The development-only caveat and old immediate-GET description are removed.
- `docs/release-notes-1.4.0.md` describes GET preview versus POST claim, direct-client migration, Send Again revalidation and fresh capability, in-memory path retention, and release limitations. Historical notes remain unchanged.
- No transfer code, native templates, release workflow, dependency, generated binding, icon, or historical release-note file was edited. The pre-existing untracked `build/.DS_Store` was left untouched.

## Focused verification

- `go test -count=1 -run 'TestReleaseIdentity|TestTheVersionIsStatedOnceAndFollowedEverywhere|TestReleaseWorkflow|TestTheDraftReleaseNotesStateWhatTheBuildIsNot|TestTheReleaseWorkflowSurvivesARetryAndCannotRaceItself' ./...` — passed on the implementation checkout. The existing tests check version agreement, native template wiring, release gate ordering, checksum checks, and workflow constraints.
- Structured Python JSON comparison of all four product-version fields with the baseline, then equality of the remaining JSON — passed. This includes every package-lock entry and confirms the dependency graph is identical. Reproduction command and output:

  ```sh
  python3 - <<'PY'
  import copy
  import json
  import subprocess
  from pathlib import Path

  baseline = '7fdb98ede732d15a1f1129168e9ae9d805e37900'
  fields = {
      'wails.json': [('info', 'productVersion')],
      'frontend/package.json': [('version',)],
      'frontend/package-lock.json': [('version',), ('packages', '', 'version')],
  }
  for filename, paths in fields.items():
      old = json.loads(subprocess.check_output(['git', 'show', f'{baseline}:{filename}']))
      new = json.loads(Path(filename).read_text())
      normalized = copy.deepcopy(new)
      for path in paths:
          previous, current = old, normalized
          for part in path[:-1]:
              previous, current = previous[part], current[part]
          assert current[path[-1]] == '1.4.0', (filename, path)
          assert previous[path[-1]] == '1.3.1', (filename, path)
          current[path[-1]] = previous[path[-1]]
      assert normalized == old, f'unrelated JSON change in {filename}'
      print(f'{filename}: {len(paths)} product version field(s) changed; all other JSON identical')
  PY
  ```

  ```text
  wails.json: 1 product version field(s) changed; all other JSON identical
  frontend/package.json: 1 product version field(s) changed; all other JSON identical
  frontend/package-lock.json: 2 product version field(s) changed; all other JSON identical
  ```

- Manual review of the exact `docs/release-notes-1.4.0.md` file against `_bmad-output/specs/spec-receiver-handoff/SPEC.md` and `receiver-protocol.md` — the notes say GET preview reserves nothing, first valid POST claims without promising delivery, and Send Again revalidates current contents rather than replaying a snapshot. They state that sender completion does not prove browser save, give a direct POST example, and avoid promises of persistent history, resume, multi-item selection, or desktop receiving. The automated draft-notes test above checks the generic notes built by `release.yml`; it does not check this file.
- `git diff --check` — passed.

## Release proof pending with orchestrator

The canonical sequential local gate, platform preflight, exact-head native Windows/macOS/Linux CI, tag workflow, four downloaded release assets, matching SHA-256 sidecars, native bundle version metadata, and published release URL have **not** been verified here. No release has been claimed or published by this implementation task.

The existing release workflow creates a draft with generic notes generated inside `.github/workflows/release.yml`. The orchestrator must set the final release body to the exact contents of `docs/release-notes-1.4.0.md` before publication; this implementation did not edit the workflow, per the spec boundary.

## Independent review triage

Three context-free review layers completed: edge-case and verification-gap reviewers returned no findings. The blind review's actionable wording and evidence improvements were applied by the Sol implementer: claim versus delivery, non-reserving preview, current-content revalidation, sender completion versus browser save, actionable POST example, release link, reproducible metadata comparison, and explicit review of the final notes file. Publication-body equality, central evidence linkage and task reconciliation are orchestrator-owned completion checks. More detailed metadata/stale-preview documentation was not required for this bounded release; the canonical protocol remains the source for those details. The README's current-release claim is prepared for the authorized publication, not evidence that publication has already happened. No change to transfer behavior or workflow was introduced.

## Local release gate — 2026-10-07

A disposable checkout at `/private/tmp/fairdrop-release-1.4.0-check` applied the exact three metadata file changes to baseline `7fdb98ede732d15a1f1129168e9ae9d805e37900`. Sequential Wails build, generated-binding and build-asset drift, restored .gitkeep, gofmt, vet, pinned staticcheck, full Go suite, CGO_ENABLED=1 and full race suite, frontend suite (846 tests), rendered browser suite (84 tests plus real GET/POST receiver fixture), line-ending check, Darwin arm64/Linux amd64 builds and Darwin staticcheck all passed. Full local logs: `/private/tmp/fairdrop-1.4.0-build.log` and `/private/tmp/fairdrop-1.4.0-gate.log`. The built Info.plist reports 1.4.0 and `codesign --verify --deep --strict` passed. Later edits were release prose and evidence only. Root working tree build/.DS_Store remains untouched.

Publication remains pending exact-head CI and tagged native artifact verification.

## Published release — 2026-10-07

- Release: https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.4.0 — published at 2026-10-07T18:48:18Z, non-draft, non-prerelease, latest stable. Owner explicitly authorized publication.
- PR #10: https://github.com/jaeson-sandbox/FairDrop/pull/10; verified head `f41edc12a6d9742bbee3c8d6536f7621a2ac6ec7`. Run https://github.com/jaeson-sandbox/FairDrop/actions/runs/37664467222 passed all three jobs.
- Tag v1.4.0 points at merge `551c46db886b3ecef17bb36c7cce08347f60e2ca`; its tree exactly matches the verified PR head.
- Tagged release run https://github.com/jaeson-sandbox/FairDrop/actions/runs/37666889342 completed successfully: Windows/macOS native gates, Linux adapter gate, both native builds, and release job. Exact headSha and every job conclusion checked via GitHub JSON, not watch exit status.
- Downloaded all four draft assets into `/private/tmp/fairdrop-1.4.0-assets`. Both `shasum -a 256 -c` checks passed. macOS ZIP SHA-256: `769cad832893649d389421091cabcf2f6efd6ad3cc68ba215d2d102d1ea0fe61`; Windows exe SHA-256: `94098c566b8151de7f6b9a6b78dcb864a1c75ea7596b8be8c644e52943992fdd`.
- Extracted macOS bundle is arm64, identifier com.fairdrop.fairdrop, both bundle version fields 1.4.0. Strict/deep signature verification passed; signature is ad-hoc with no TeamIdentifier. Downloaded Windows executable is x86-64 PE; native resource and execution proof comes from Windows CI, not a local Windows run.
- Downloaded macOS bundle launched to its idle screen through native app automation. This is a launch observation, not phone, firewall, screen-reader or hardware-keyboard certification.
- Replaced generic draft body with docs/release-notes-1.4.0.md and compared equality before publishing. Four expected assets and release state were checked before publication; afterward GitHub reported v1.4.0 as Latest, not draft or prerelease.

All earlier pending statements above describe their original checkpoints and are superseded by this completed proof. The I/O matrix is covered by existing identity tests, reproduced structured metadata comparison, explicit notes review, exact-head CI, downloaded checksum/native metadata verification, and final release-state checks. No known failures were waived.
