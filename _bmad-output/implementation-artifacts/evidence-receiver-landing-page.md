# Receiver landing page evidence

## Matrix audit

| Scenario | Executed proof |
| --- | --- |
| Inspect and revisit | `TestLandingVisitsDoNotClaimOrRead`: twelve concurrent real GETs return metadata/form, leave reservation false, and make zero authorization/preparation calls or events; subsequent POST succeeds. |
| Download and finalization | `TestNativePathClassesStageAndDownload` through `assertNativeDownloadWithApp` visits the real page then POSTs a real file/folder and checks exact bytes or ZIP; existing `internal/server/handler_test.go`, `finalization_test.go`, `longevity_test.go` byte and HTTP-finalization assertions now use POST. |
| Race | `TestCompetingClaimsAuthorizeExactlyOnce` retains its POST race; `TestGetDuringReservedPostIsLockedWithoutAnotherClaim` blocks the first authorizer and verifies GET receives 423 with no second authorization or prepare. |
| Invalid | `TestRejectedRequestsNeverReachClaimLogic` drives wrong routes, tokens, noncanonical and oversized paths through GET and POST, plus HEAD/PUT/etc., and verifies generic 404 with no reservation, authorization, preparation or events; a valid POST still succeeds afterward. |
| Metadata | `TestLandingEscapesMetadataAndCarriesPagePolicies` checks escaped metacharacters, Unicode, absent source path, no external elements, and exact no-store/referrer/nosniff/CSP. `TestFileLandingRendersStagedSize` checks literal rendered sizes at zero, singular, byte, KB, MB, GB, TB and unavailable boundaries. `TestFolderLandingExplainsZIPWithoutPromisingArchiveSize` checks folder wording. `frontend/browser/receiver-live.mjs` drives the real top-level server page at 320px, with a long Unicode/bidi name, 200% zoom, dark mode, forced colors, keyboard focus, POST submission and completed attachment. |
| Cancel/change | `TestNativeInspectedPageCancelRetiresOldCapability` visits a real escaped-name page, observes no lifecycle start, cancels, refuses old POST, then stages a fresh capability. The real native GET-to-POST path checks no start event and a busy second Stage, establishing that the staged session remains owned until POST. Existing coordinator/server cancel and pre-stream failure tests remain active; `TestPrepareFailureIsGenericGone` and `TestPrepareFailureClosesTheListener` pin 410 and teardown. |

## Mutation proof

- Changed the GET branch to enter on POST. `TestLandingVisitsDoNotClaimOrRead` failed naming an invalid page/form response. Full output: [get-claim.log](evidence-receiver-landing-page-logs/get-claim.log).
- Replaced Go `html/template` with `text/template`. `TestLandingEscapesMetadataAndCarriesPagePolicies` failed on the unescaped hostile name. Full output: [escape.log](evidence-receiver-landing-page-logs/escape.log).
- The reproducible driver [verify-receiver-mutations.py](../../scripts/verify-receiver-mutations.py) runs six receiver mutations in temporary module copies without editing this checkout. It baselines the named test, applies one change, requires that same named test to fail, and retains each complete baseline and mutant log in [evidence-receiver-landing-page-logs](evidence-receiver-landing-page-logs). The six cases are GET claim, HTML escaping, wrong-token POST, staged-size wiring, form method, and CSP header. Result: **6/6 killed**, `/tmp/fairdrop-fixes-mutations.log`. This is the scoped receiver inventory; it is not the repository-wide native mutation denominator.

## Verification and limits

- `go test -count=1 ./internal/server`: passed, `/tmp/fairdrop-server-test.log`.
- `go test -count=1 ./...`: passed, `/tmp/fairdrop-go-test.log`.
- Targeted frontend copy/staged suite: passed, `/tmp/fairdrop-copy-test.log`.
- Live Chromium receiver check: passed, `/tmp/fairdrop-receiver-browser.log`.
- Wails build: passed, `/tmp/fairdrop-wails-build.log`; generated binding contents did not drift. This macOS machine regenerated executable file modes, which were restored to their committed mode.
- Independent orchestrator visual check: actual receiver page captured at 390px light/dark and 320px dark with keyboard focus in desktop Chromium (`/tmp/fairdrop-receiver-light.png`, `/tmp/fairdrop-receiver-dark.png`, `/tmp/fairdrop-receiver-narrow.png`). Layout and focus appeared correct. This is viewport emulation, not a real phone observation.
- The first full rendered browser run found one stale sender-copy expectation; its exact test passed after updating the expectation (`/tmp/fairdrop-browser-targeted.log`). The subsequent full rendered run passed in the gate below.
- Native Windows CI and manual nearby-device transfer are not claimed by local macOS checks.

## Gate transcript

The sequential local gate completed; each command retained its complete output under `/tmp/fairdrop-gate-*.log`, summarized in `/tmp/fairdrop-gate-summary.log`. See post-review verification below for the corrected frontend assertions and additional coverage. Build-asset drift passed after Wails build in `/tmp/fairdrop-receiver-integration`, without modifying the pre-existing untracked `build/.DS_Store` in the shared checkout. The adopted architecture spine also now describes GET inspection and POST claim.

## Independent review triage (2026-10-04)

Three context-free review layers ran in isolated worktrees. Findings below are patches under the existing spec; no intent or trust-model change is needed.

- High verification gap: wrong-token POST bypass mutation survives current suite; expand negative request matrix to POST, preserve valid capability afterward, and kill the demonstrated mutation.
- Medium verification gap: file size can be changed to zero without a failure; assert real rendered metadata for literal sizes and cover formatter boundaries. Cosmetic unit rounding is not a new product requirement.
- Medium: active architecture and UX still describe any request claiming / forbid branded receiver pages; reconcile current prose and register receiver copy.
- Medium: native GET inspection must assert coordinator STAGED/no started event, and a visited page cancelled before POST must not reopen its session.
- Medium: browser harness needs bounded post-readiness failure behavior, explicit fixture lifetime and robust spawn/signal cleanup.
- Medium: live browser should receive a known nonempty payload matching metadata, assert completed bytes, and native HTML assertion must handle escaped filenames.
- Medium reproducibility: commit a receiver mutation inventory/runner and retain its complete logs, including the two demonstrated missing guards.

Duplicate negative-POST and displayed-size findings from blind and verification reviewers are merged. The edge reviewer independently identified the spawn/signal cleanup hang. No production capability bypass was found: the surviving mutation demonstrated missing regression coverage.

## Post-review focused verification

- Isolated fix checkout `/tmp/fairdrop-receiver-fixes`: `go test -count=1 ./internal/server ./...` passed (`/tmp/fairdrop-fixes-go.log`).
- `frontend/npm test`: 20 files, 829 tests passed (`/tmp/fairdrop-fixes-frontend.log`). The three stale “another opener” assertions from the original gate now match “another downloader.”
- Focused `go test -count=1 -race` over the changed receiver cases passed (`/tmp/fairdrop-fixes-server-race.log`).
- `node frontend/browser/receiver-live.mjs`: passed against the real top-level HTTP server and saved exact `hello world!` bytes from the 12-byte staged fixture (`/tmp/fairdrop-fixes-browser.log`). The harness has a browser deadline, post-readiness exit detection, bounded cleanup, and a two-minute fixture fail-safe.
- The original candidate gate (before these review fixes) passed Wails build, gofmt, vet, staticcheck, full Go, cgo, full race including 4 GiB fixture, rendered browser suite plus live receiver check, line endings, Darwin/Linux builds, and 68/68 canonical native mutations. Its frontend suite had only three stale wording assertions, corrected and passed above. The shared checkout’s build-asset drift check refused the pre-existing untracked `build/.DS_Store`; the orchestrator proved build assets and bindings clean in a separate checkout without touching that file. Native CI on these post-review changes remains the orchestrator’s release proof.

- After integration, Wails build, gofmt, vet, staticcheck and diff whitespace checks passed again on the final local candidate (`/tmp/fairdrop-receiver-final-build.log`).
