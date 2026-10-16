# Text and link sharing implementation evidence

Baseline: `9c6813ab0c6401f2f592b62d218e708d8c5f85cd`. Implementation remains uncommitted in the isolated `codex/text-link-sharing` worktree. Main owns the ordered full gate, review and integration.

## Matrix coverage

| Matrix row | Executed check |
|---|---|
| Exact normalization, Unicode, spaces, controls and both byte bounds | `TestNormalizeTextExactAndBoundary`; `TestStageTextInvalidInputNeverStartsResources`; `TestAppStageTextPreservesRawBoundaryAndRefusesMalformedInput`; frontend `text.test.tsx` validation cases |
| GET metadata only, exact route/method, no claim | `TestTextPreviewAndOneShotRevealExactDOM` checks decoded disclosure, `TestTextWrongMethodsAndRoutesDoNotReserveClaim` checks HEAD/PUT/OPTIONS/DELETE and path refusals before a valid GET/POST; live browser receiver fixture |
| Authorized POST, escaping, exact readonly textarea including leading LF | `TestTextPreviewAndOneShotRevealExactDOM` parses real HTML DOM; `frontend/browser/receiver-live.mjs` drives real server with scripting disabled |
| Concurrent or refused POST | `TestTextConcurrentClaimHasOneAuthorizer`; `TestTextRefusedClaimNeverWritesBody` |
| Short write, HTTP finalization, request-only disconnect | `TestTextCompletionWaitsForFullHTTPFinalization`; `TestTextRequestDisconnectAfterAuthorizationSettlesFailure` |
| No source/sink access, fresh recovery identity, retired waiting code | `TestStageTextUsesNoSourceAndOffersLiteralRoute`; `TestTextWaitingRefreshKeepsValueWithoutSourceAndRotatesIdentity`; `TestTextRefreshRetiresActualHTTPRoute` confirms the old endpoint stops serving and the old token is rejected at the replacement listener |
| Send Again, explicit Cancel/Done, stale Stage/recovery acknowledgments, unmount, mutually exclusive input | `frontend/src/transfer/text.test.tsx` controller tests and `frontend/src/ui/TextViews.test.tsx` |
| Browser without scripting or Clipboard API, 320px/forced-colors focus | Live Playwright receiver fixture checks readonly field, exact selected range, no horizontal overflow and outline with JavaScript disabled |

## Focused verification

- `go test ./internal/server ./internal/transfer ./` passed after the final concurrent-claim and App-boundary tests. After review fixes, a focused `-count=1` run of the new route, method, disclosure and raw-guard regressions passed.
- `node frontend/browser/receiver-live.mjs` passed with the real Go HTTP fixture; file download proof remained green while text reveal was added.
- Wails regenerated StageText bindings. Its first build reached TypeScript while frontend edits were in progress; a final Wails build remains in the main ordered gate.
- `npm run build` and `npm test -- --run` passed in the frontend directory: 39 files, 1,307 tests. The review patch then passed its five-file focused suite (134 tests), a further three-file composer-alert suite (33 tests), and `npm run build` after each patch. The main session owns the final full frontend gate on the stable snapshot.
- `git diff --check -- frontend/src scripts/verify-text-frontend-mutations.py` passed. Whole-tree `git diff --check` reports four trailing-space lines in Wails-generated `frontend/wailsjs/go/models.ts` (59, 63, 72, 227); generated bindings were not hand-edited.

## Named mutation inventory

The canonical Go case list is `scripts/verify-text-mutations.py`. It ran in a temporary isolated source copy. Every baseline passed and every mutation failed its named assertion; complete JSON transcripts are in `evidence-text-link-sharing-logs/`. The raw cap is twice the normalized cap, so output-only rejection cannot prove that the raw guard ran before normalization. A source-order test pins the exact `len(raw) > MaxRawTextBytes || !utf8.ValidString(raw)` guard before `b.Grow`; inspection of the current function found no earlier scan or allocation. This is a source assertion, not a runtime resource-bound measurement, and it does not preclude arbitrary earlier work in a future edit. The output-bound case and its two logs were renamed from `raw-and-normalized-cap` to `normalized-cap` after this distinction was identified; the mutation source and executed test are unchanged.

| Case | Named defense |
|---|---|
| `raw-before-allocation` | `TestRawTextLimitPrecedesValidationAndAllocation` |
| `normalized-cap` | `TestNormalizeTextExactAndBoundary` |
| `no-source-access` | `TestStageTextUsesNoSourceAndOffersLiteralRoute` |
| `content-before-claim` | `TestTextPreviewAndOneShotRevealExactDOM` |
| `escaped-get-disclosure` | `TestTextPreviewAndOneShotRevealExactDOM` checks decoded text, killing a GET mutation that leaks HTML-escaped payload |
| `method-guard` | `TestTextWrongMethodsAndRoutesDoNotReserveClaim` |
| `escaping` | `TestTextPreviewAndOneShotRevealExactDOM` |
| `fresh-recovery-identity` | `TestTextWaitingRefreshKeepsValueWithoutSourceAndRotatesIdentity` |
| `retired-token` | `TestTextRefreshRetiresActualHTTPRoute`, against the replacement server's live HTTP endpoint |
| `disconnect-terminal` | `TestTextRequestDisconnectAfterAuthorizationSettlesFailure` |

The canonical frontend case list is `scripts/verify-text-frontend-mutations.py`. It ran in isolated copies after the final local-alert fix; all 13 baselines passed and all 13 mutations failed their named behavioral assertions. All 26 baseline/mutant logs are in `evidence-text-link-sharing-logs/frontend/`.

| Frontend case | Named defense |
|---|---|
| `send-again-retention` | keeps text across the internal live-outcome cancel used by Send Again |
| `explicit-cancel-forget` | forgets remembered text on explicit Cancel |
| `unmount-cancel` | cancels a pending text stage on unmount and ignores its later acknowledgement |
| `stale-stage-ack` | cancels an in-flight text stage and ignores its late acknowledgement |
| `stale-recovery-ack` | ignores a late text recovery acknowledgement after Cancel |
| `oversize-before-unsupported` | retains the bounded draft when an oversized edit also contains unsupported characters |
| `cleanup-status` | reports uncertain cleanup after malformed StageText metadata and failed cancellation |
| `live-unmount-cancel` | cancels and forgets a live waiting text session on unmount |
| `clear-stale-error-on-edit` | clears an earlier setup failure so later local validation owns its alert |
| `oversize-local-alert` | announces an oversized paste immediately after setup failure |
| `stage-conflict-local-alert` | announces a file-stage conflict while the text composer is open |
| `browse-conflict-local-alert` | announces a browse conflict while the text composer is open |
| `drop-conflict-local-alert` | announces a drop/reject conflict while the text composer is open |

## Review triage

- A source review found that a text receiver could disconnect after authorization but before headers, leaving the coordinator active. The handler now reports a terminal failure for request-only cancellation; session-owned Cancel stays silent. A deterministic test cancels inside the authorizer before the handler's next check.
- The server copies and validates text before binding, and rejects a mixed text/file/receive request. The receiver DOM test verifies the parser's special leading-newline behavior and inert markup-looking payload.
- Review findings on the initial proof were addressed: GET disclosure now checks HTML-decoded content and an escaped-payload mutation; wrong methods have an explicit no-authorization regression; recovery retirement is checked against a real old and replacement HTTP listener, with an old-token reuse mutation. The raw pre-normalization guard has its own source-order pin because the two byte limits make its removal observationally equivalent for accepted/rejected output. This proof does not measure runtime resource use.
- Frontend review corrections added the associated 65,536-byte composer limit and App-level heading/return-focus tests; text transfer Cancel and recovery failures render and focus their safe errors. Composer-local validation and selection refusals clear stale reducer error ownership without duplicate announcements, including an oversized paste immediately after setup failure. Malformed metadata preserves the `cleanup_unconfirmed` public message when cleanup fails. Oversized draft refusal precedes character classification, and live text unmount cancels and forgets only the owned session. Non-text error outcomes retain Send multiple items. Text copy now uses the existing copy contract and EXPERIENCE table. Focused view, controller, App and copy-contract tests passed; the 13 frontend mutations above pin the lifecycle and error ownership paths.

## Independent review triage (2026-10-14)

All three context-free layers reviewed the complete baseline-to-working diff before triage. Blind: 12 findings; edge: 5; verification: 1 verification gap plus 1 other finding. Duplicate claims with the same required correction were consolidated below. These are implementation/test/copy corrections within the existing contract; no product intent change or deferred work.

| Finding | Severity | Route | Required correction |
|---|---|---|---|
| Missing composer byte-limit guidance (blind/verification) | medium | patch | Show and associate the 65,536 UTF-8-byte limit. |
| Composer opening focus lacks App-level proof (verification) | medium | patch | Exercise actual App focus movement. |
| Composer cancellation strands focus (blind/edge) | medium | patch | Restore Idle focus and prove through App. |
| Text transfer cancellation error hidden/unannounced (blind/edge) | medium | patch | Render and focus safe command failure. |
| Failed waiting refresh focuses heading instead of error (blind) | medium | patch | Prioritize recovery error focus. |
| Setup error leaves stale announcement ownership (blind/edge) | medium | patch | Ensure later local validation is announced without duplicate ownership. |
| Malformed acknowledgment hides failed cleanup (edge) | high | patch | Show cleanup_unconfirmed when cancellation fails. |
| Invalid characters bypass oversize-edit refusal (blind) | medium | patch | Enforce draft byte bounds independently of character validity. |
| Live text survives frontend unmount (blind) | high | patch | Cancel/forget live text with stale-operation protection. |
| File error loses multiple-items action (blind/edge) | medium | patch | Preserve existing non-text outcome actions. |
| Raw-limit and retired-endpoint proof overstated (blind) | medium | patch | Add distinct raw guard and actual retired HTTP capability proof; describe limits honestly. |
| GET privacy assertion misses escaped leakage (blind) | high | patch | Check decoded content and mutation-kill escaped preview leakage. |
| Unsupported text methods omitted (blind) | medium | patch | Exercise HEAD/PUT/OPTIONS/DELETE without claim. |
| Text copy scattered outside canonical contract (blind) | low | patch | Centralize copy and align EXPERIENCE/test use. |

Independent backend proof recheck accepted the corrected HTTP/normalized-cap assertions; raw-guard evidence was narrowed to the actual source-order check. Initial isolated full gate passed at `/private/tmp/fairdrop-text-gate-logs-40xkg87j` before review patches. Final gate follows below.

Targeted independent frontend recheck inspected all five edge findings plus invalid oversized drafts and live unmount cleanup. Four files / 111 focused tests passed; no remaining concrete regressions were found within that scope.

## Final local gate

Final reviewed source was copied to `/private/tmp/fairdrop-text-final-7g6u1j9m`; complete local logs are at `/private/tmp/fairdrop-text-final-logs-7g6u1j9m`. The ordered gate passed Wails build, generated-binding/build-asset drift, gofmt, vet, pinned staticcheck, full Go suite, native matrix, explicit cgo probe, full race suite, frontend (39 files / 1,319 tests), browser (128 tests plus real receiver/upload fixtures), line endings, Darwin/Linux build preflight, Darwin staticcheck, native mutation inventory, and all 10 Go + 13 frontend text mutations. Every mutation had a passing baseline and named assertion failure. Reviewed source matched the snapshot; subsequent edits only record completion, review order and roadmap status.

The CI-only macOS unusable-lock smoke remains for GitHub Actions, whose script forbids developer-machine execution. Manual nearby-device, native UI and assistive-technology observations were not performed and are not claimed. Version remains 1.6.0. Exact-head Windows/macOS/Linux CI is required before merging; its durable result will be linked in the pull request.
