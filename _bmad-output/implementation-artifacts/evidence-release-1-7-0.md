# FairDrop 1.7.0 release evidence

Spec: [spec-release-1-7-0.md](spec-release-1-7-0.md). Baseline: `954d91813b0d6b190be8bc19267eb46739c92d5f`.

## Implementation and matrix coverage

| Matrix row | Evidence or pending check |
| --- | --- |
| Product version | `wails.json` and `frontend/package.json` declare 1.7.0. Structured JSON inspection confirms both `frontend/package-lock.json` product fields are 1.7.0 and every other lock record equals the baseline. `TestTheVersionIsStatedOnceAndFollowedEverywhere` and release template tests passed. Native bundle inspection remains pending. |
| Release contents | `docs/release-notes-1.7.0.md` describes connection recovery, bounded outbound text, the receive-notice claim fix, receive safety and trust/platform limits. Claims were checked against the active connection and text contracts and implementation evidence. Publication of the notes remains pending. |
| Tag/artifacts | Pending: the main orchestrator will verify the tag, native assets, checksum sidecars and macOS bundle version after the release run. |
| CI | Pending: exact-head branch, main and tag job conclusions. |
| Publication | Pending: draft review, public release body/assets and latest-release identity. |

## Focused implementation verification

- `go test -count=1 -run 'TestRelease|TestBoth|TestTheDraft|TestTheRelease' ./` passed (`ok fairdrop 0.365s`). This exercises the matching release identity, platform template identities, workflow safeguards, and draft-note trust claims. The pattern excludes the version-coherence test.
- `go test -count=1 -run '^TestTheVersionIsStatedOnceAndFollowedEverywhere$' ./` passed (`ok fairdrop 0.295s`), explicitly verifying Wails and frontend package version coherence.
- Structured JSON inspection passed: Wails, frontend package and both root lock versions equal 1.7.0; removing those two lock fields leaves the same parsed lock object as the baseline, including the unrelated dependency constraint containing `^1.6.0`.
- `git diff --check` passed.

The three independent review layers are complete; the main orchestrator owns their triage, the full ordered local gate, canonical mutations, exact-head CI, native artifact checks and publication. No manual phone, browser, native UI or Photos observation is claimed.

## Independent review

Three context-free layers reviewed the complete release diff. Edge-case and verification-gap layers returned no findings. Blind review returned 12 findings, triaged individually as documentation/procedure patches (medium for verification accuracy; low for copy completeness). No runtime changes, new features or deferred work were introduced.

- Corrected the unsupported focused-test claim and ran the omitted version-coherence test explicitly.
- Added explicit draft-body replacement before byte equality/publication, and absent-tag/release preflight; never rerun the existing clobber-capable workflow against a published release.
- Added downloaded Windows FileVersion/ProductVersion verification, macOS deep/strict signature integrity plus ad-hoc identity, and both artifact architectures to publication proof. Architecture recording is accepted; no unverified claim of universal platform compatibility will be made.
- Clarified text newline normalization and invalid-input refusals; file-path revalidation and invalid old codes after failed replacement; bounded notice waiting; and receiving storage/filesystem limits.
- Main inspection corrected the release body's README link to a version-pinned absolute URL with no invented heading anchor.

The existing v1.7.0 tag and release were both absent at preflight. Local built macOS bundle has CFBundleVersion and CFBundleShortVersionString 1.7.0. No manual UI check is claimed. Photo changes are explicitly on hold pending owner discussion.

## Local release gate passed

The isolated snapshot `/private/tmp/fairdrop-release17-d1praye1` passed the complete ordered gate; full logs are in `/private/tmp/fairdrop-release17-logs-d1praye1`: Wails build, binding/build-asset drift, formatting, vet, pinned staticcheck, full Go suite, native matrix, cgo proof, full race suite, 1,319 frontend tests, 128 browser tests plus live receiver/upload fixtures, line endings, Darwin/Linux build preflight, Darwin staticcheck, canonical native mutations, 10 text Go mutations and 13 text frontend mutations. Later review edits were documentation only; all product metadata matches the verified snapshot. The CI-only macOS unusable-lock smoke remains for CI.

Exact-head remote checks and publication are still pending.

## Remote integration and tag

- PR #20 head `f43131686f460cb54c46bc9e6130159e9cea7ba1`: [run 38841855773](https://github.com/jaeson-sandbox/FairDrop/actions/runs/38841855773), Windows/macOS/Linux jobs all success.
- Main merge `b05e5d7c66a9c138c8f75340e8587508dcd31793`: [run 38843340172](https://github.com/jaeson-sandbox/FairDrop/actions/runs/38843340172), Windows/macOS/Linux jobs all success. The merged tree exactly matches the PR head. Conclusions were read job by job, not inferred from watch exit status.
- Rechecked remote tag/release absence, then created annotated `v1.7.0` pointing to that verified main commit. No existing tag or published release was replaced.
- [Release run 38844946025](https://github.com/jaeson-sandbox/FairDrop/actions/runs/38844946025) is building that exact tag/head. Asset verification and publication remain pending.

## Tagged race-gate test synchronization correction

The tagged run's Linux and macOS `go test -race ./...` jobs both failed `TestClaimWaitingForNoticeHonorsCallerCancellation` at the same assertion: `state = "STAGED", want ERROR after spent caller claim` (`coordinator_receive_test.go:944`). The full original transcripts are preserved at `/private/tmp/fairdrop-release17-linux-failure.log` and `/private/tmp/fairdrop-release17-macos-failure.log` (macOS job 116793242420). The preceding exact event assertion passed. `acceptTerminal` publishes `TransferError` before committing `stateError`, so observing the event alone did not prove terminal processing had finished. The local test-only correction now blocks the fake observer after it records `TransferError`, confirms that `STAGED` is the expected intermediate state, retains the notice/error ordering and absence-of-started checks, releases the observer, and joins `drainerDone` before asserting `ERROR`. Runtime code is unchanged.

The corrected focused test passed `go test -race -count=1000 -run '^TestClaimWaitingForNoticeHonorsCallerCancellation$' ./internal/transfer`; full output is `/private/tmp/fairdrop-release17-cancel-fixed-race.log`. In isolated copy `/private/tmp/fairdrop-release17-notice-proof.2XO1N4`, removing the release-and-drainer-completion barrier made the test fail deterministically at the retained state assertion with `state = "STAGED", want ERROR` (`/private/tmp/fairdrop-release17-no-completion-barrier.log`). The canonical `scripts/verify-receive-notice-claim-mutations.sh` passed all eight baselines and killed all seven defined mutations in that isolated copy. Its full console output is `/private/tmp/fairdrop-release17-notice-canonical-mutations.log`; individual JSON test logs are under `/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/tmp.6f0fLV42wy/`. Exact-head native release gates and publication remain pending.

Independent review found no actionable issues in the test correction. The tag remains immutable at `b05e5d7`; the correction is absent from that tag. The only differences from it are this test and release spec/evidence documents, so the shipped runtime/build/version tree is unchanged. After deterministic diagnosis and mutation proof, main deliberately reran only failed jobs of release run 38844946025. Corrected-branch CI must validate the harness repair and exact-tag CI must independently pass before publication; neither substitutes for the other. The original attempt ended with Linux/macOS race assertion failures, Windows success, and build/release jobs skipped.

The corrected test passed the complete ordered local gate in `/private/tmp/fairdrop-release17-testfix-tu2n0arn`, with full logs in `/private/tmp/fairdrop-release17-testfix-logs`: Wails, drift/formatting/static checks, full Go/native/race suites, frontend/browser/live fixtures, platform build preflight, canonical native mutations, and all text Go/frontend mutations. The test file matches that verified snapshot; later edits are release records only. No manual UI observation is claimed.
