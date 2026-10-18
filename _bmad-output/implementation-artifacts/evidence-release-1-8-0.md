# FairDrop 1.8.0 release evidence

Companion to [spec-release-1-8-0.md](spec-release-1-8-0.md). Baseline: `af4199273318602da22fbdcf7ec1c72a8c2ed2c9`.

## Implementation and matrix coverage

| Scenario | Evidence at implementation handoff | Remaining release proof |
| --- | --- | --- |
| Identity | `wails.json`, `frontend/package.json`, and both root version fields in `frontend/package-lock.json` say `1.8.0`. A parsed JSON comparison with the baseline, after removing only those version fields, found all other Wails, package and lock data identical. Existing native templates remain unchanged. | Native Windows/macOS artifact metadata inspection. |
| Contents | `docs/release-notes-1.8.0.md` states the explicit Receive text flow, normalization and byte limits, one-shot RAM receipt, explicit native copy, clearing limits, trusted-LAN transport and signing/checksum limitations. README current-release link and receiver roadmap baseline point to 1.8.0. | Review draft body against the checked-in notes before publication. |
| Gate | Focused identity/workflow tests passed locally and again after documentation review patches: `go test -count=1 -run 'TestRelease|TestBoth|TestTheDraft|TestTheRelease|TestTheVersionIsStatedOnceAndFollowedEverywhere' .` → `ok fairdrop 0.298s` on the rerun. `git diff --check` passed on the rerun. | Main agent runs the full ordered local gate and checks every exact-head PR and tag CI job conclusion. |
| Assets | No artifact built or published by implementation subagent. | Download four draft assets; check SHA-256, native versions and architecture, macOS codesign, Windows VERSIONINFO. |
| Publication | No remote operations by implementation subagent. | Main agent confirms absent tag/release, integrates, tags, verifies draft, publishes, and records public latest-release identity. |

Manual phone/native interaction was not performed at this stage and remains unverified. The production receive-text behavior and automated tests were already present at the baseline; this implementation changes only release metadata and documentation.

## Reproducible product metadata comparison

Run from the repository root. This compares parsed JSON against the spec's full baseline commit, removing only the product version fields before comparing the remaining structures:

```sh
python3 - <<'PY'
import json, pathlib, subprocess
root = pathlib.Path('.')
base = 'af4199273318602da22fbdcf7ec1c72a8c2ed2c9'
def old(path):
    return json.loads(subprocess.check_output(['git', 'show', f'{base}:{path}'], text=True))
def new(path):
    return json.loads((root / path).read_text())
lock_old, lock_new = old('frontend/package-lock.json'), new('frontend/package-lock.json')
assert lock_new['version'] == lock_new['packages']['']['version'] == '1.8.0'
assert lock_old['version'] == lock_old['packages']['']['version'] == '1.7.0'
for lock in (lock_old, lock_new):
    del lock['version']
    del lock['packages']['']['version']
assert lock_old == lock_new, 'lock data changed beyond two product version fields'
package_old, package_new = old('frontend/package.json'), new('frontend/package.json')
assert package_new.pop('version') == '1.8.0'
assert package_old.pop('version') == '1.7.0'
assert package_old == package_new, 'frontend package changed beyond product version'
wails_old, wails_new = old('wails.json'), new('wails.json')
assert wails_old['info'].pop('productVersion') == '1.7.0'
assert wails_new['info'].pop('productVersion') == '1.8.0'
assert wails_old == wails_new, 'wails config changed beyond product version'
print('PASS: metadata fields agree at 1.8.0; all other package, lock, and Wails data matches baseline')
PY
```

## Review triage

Three independent review layers completed before this documentation patch. The edge-case and verification-gap layers reported no findings. The blind layer raised 12 low/medium documentation or release-procedure findings: clarify retry privacy, link retirement, fresh receive sessions, supported-control reference, README scope, target architectures, exact asset names, canonical mutation runners and their unit suite, draft-versus-clobber responsibility, central release evidence, completed task status, and reproducible dependency comparison. These were addressed in `README.md`, `docs/release-notes-1.8.0.md`, this evidence file, and the release spec. No production code, dependency, or release workflow changed in response to these findings.

## Complete local gate

The frozen snapshot `/private/tmp/fairdrop-release18-5a146b2j` passed the full ordered gate. Complete logs are retained in `/private/tmp/fairdrop-release18-logs-5a146b2j`: Wails build, generated binding/build asset drift and .gitkeep, formatting, vet, pinned staticcheck, full Go suite, native matrix, compiled cgo probe, full race suite, 1,346 frontend tests, 11 runner tests, 9 receive-text frontend baselines/mutations, 132 browser tests plus real download/text/upload fixtures, line endings and Darwin/Linux build preflights with Darwin staticcheck. Native mutations completed 86 named kills (including the receive-text Go inventory); additional canonical inventories completed 10 outbound-text Go and 13 outbound-text frontend kills. No failed local gate was hidden or retried. The macOS unusable-lock launch smoke is reserved for CI by its script.

All runtime, tests, workflow and product metadata match the gate snapshot after mutations restored their edits. Subsequent changes are release documentation only; the browser capture changed only inside the isolated copy. The workspace Wails build also succeeded (`/private/tmp/fairdrop-release18-workspace-build.log`), with generated file permissions restored to their tracked mode. The local bundle reports 1.8.0 in both plist version keys, arm64 architecture, and passes deep/strict codesign verification. These local checks do not substitute for downloaded release artifact verification.

The owner explicitly authorized this release and standing repository instructions require pushing verified milestones and integration. Those instructions supersede the build skill's generic no-auto-push/checkpoint prompts. No repeat approval is needed. Tag/release absence was confirmed during initial preflight and will be rechecked immediately before tagging. Remote integration and publication are still pending.
