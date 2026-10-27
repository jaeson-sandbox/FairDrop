# Receive Again evidence

Baseline: `cebc6678fbe08743890e8a23addfc0b2a09be403`. Implementation by GPT-6.1 Sol under the approved `spec-receive-again.md`. Main owns independent review, full sequential gate, integration and remote operations. Published version remains 1.8.0.

## Implementation

`useTransfer` keeps only the original approved parent string in a private ref. Complete file-upload outcomes expose eligibility; other modes and incomplete/cancelled outcomes do not. Receive Again synchronously reserves admission against another repeat, an active Show in Folder or terminal cancellation. It releases through `cancelImpl(true)`, waits at most 15 seconds for reducer-confirmed Idle and checks mount, selection epoch and parent ownership before ordinary `StartReceive(parent)`. Missing reset removes its waiter, unlocks the action and reports “FairDrop couldn’t confirm the previous session closed. Try again or choose Done.” without authorizing a new start. Parent clearing occurs in explicit handlers and committed lifecycle effects, never during render. The obsolete finalizer checks its captured epoch and parent before clearing, so it cannot discard a replacement parent admitted before Pending renders. A pending new admission uses the existing StageOperation cancellation and late-result cleanup. No backend command or storage guarantee changes.

User cancellation/dismissal, replacement, unmount, failed upload and failed new admission clear the parent. Successful waiting refresh preserves it. The receive outcome carries an optional command-error purpose, containing no path: failed retirement receives fixed retirement copy while failed Show in Folder retains its own copy. Done is ignored while Show in Folder is unresolved, but still abandons a pending repeat. Show explicitly captures/focuses its initiating control before disabling and restores it on success only for the same outcome with no error or superseding focus; an explicit focus-return marker supplies the visible outline. App serializes all outcome actions and disables every control while one is pending. Success has primary Receive Again, secondary Show in Folder and quiet Done, using the existing wrapping action group and focus routing.

## Matrix coverage

| Spec row | Executable coverage |
| --- | --- |
| Success / actual reset / original parent | `useTransfer.receive.test.tsx`: “waits for actual reset and admits one repeat with the original parent and clean metadata”; real App click reaches a fresh QR and link without duplicate speech. |
| Admission and competing actions | Same controller test checks synchronous ignored completion, one Cancel and one new start; “does not overlap Receive Again with an unresolved Show in Folder”; App test checks all three disabled actions. |
| Retirement failure | “keeps failed retirement retryable and reports a retirement error”; real App error receives focus, preserves usable actions and hides adapter text; view copy test distinguishes retirement from folder-open failure. |
| Revalidation | “clears a refused repeat destination rather than retaining a retry target”; native repeated composition moves the parent and verifies missing-path, file replacement and symlink replacement refusal before any network discovery; the symlink subtest uses the existing native privilege policy (local skip when unavailable, mandatory on CI). Existing `TestNativeReceiveRefusesUnusableFoldersBeforeAnyNetworkResource`, sink destination validation and space boundary/default-volume tests retain unsafe-path and no-network proof. |
| New upload | `TestNativeReceiveAgainUsesFreshSessionsFoldersValidationAndSpaceChecks`: two actual HTTP uploads to the same parent use different session/URL/QR, reject old token against the new listener, recheck oversized declared size on each cycle (fresh-check proof, not a second reserve-boundary proof), create two exclusive subfolders and preserve both exact contents under the same filename. |
| Abandonment | Controller tests cover Done during wait, failed user Done, dismissal, unmount during release, pending StartReceive cancellation, unmount with unresolved repeated StartReceive and late metadata, a new composition replacing the repeat during release, and a replacement receive admitted before Pending renders. |
| Waiting refresh / repeated cycles / stale events | Controller refresh followed by two completed repeat cycles preserves the original parent, resets counters and metadata; old identity error events cannot affect the next receiver. |
| Failure / other modes | Separate named controller tests assert waiting, incomplete, cancelled, received-text and successful-send exclusions and refuse direct repeat calls; private parent is separate from send/text refs and only a complete receive outcome enables the action. Existing receiving App suite preserves send-action exclusions. |

`frontend/browser/receive.test.tsx` exercises the complete outcome with all three actions throughout the existing 1024px/640px/320px and 200% text/spacing matrix. Three new rendered cases verify Tab/Enter, 44px targets, wrapping and real forced-colors outlines at 320px. Another rendered case verifies actual disabled-button blur and conditional Show focus restoration with a forced-colors outline. Chromium evidence does not establish WKWebView or physical-phone behavior.

## Canonical mutation inventory

Entry point: `python3 scripts/verify-receive-again-mutations.py --log-dir <unique-directory>`. The 19 cases in `CASES` are the complete final inventory. The runner delegates to the existing tested receive-text harness: resolved repository-local workspace/cwd, Windows symlink fallback, explicit UTF-8 decoding, ANSI-tolerant named assertion verdicts, timeout/error logs and rejection of compiler/import failures as kills. The admission mutation removes both independent synchronous admission fences together; it must fail the immediate ignored-call assertion, rather than a timeout. Waiting-state guarding and incomplete/cancelled/text/send eligibility are separate proofs; the earlier combined guard established only waiting exclusion.

| Mutation | Named assertion kill |
| --- | --- |
| `admission` | waits for actual reset and admits one repeat with the original parent and clean metadata |
| `actual-reset` | waits for actual reset and admits one repeat with the original parent and clean metadata |
| `reset-bound` | bounds a missing repeat reset, releases its waiter and keeps the held success retryable |
| `original-parent` | waits for actual reset and admits one repeat with the original parent and clean metadata |
| `waiting-state-guard` | refuses repeat outside successful file outcomes |
| `incomplete-eligibility` | never enables Receive Again on a held incomplete outcome |
| `cancelled-eligibility` | never enables Receive Again on a held cancelled outcome |
| `text-eligibility` | never enables Receive Again on a received text outcome |
| `send-eligibility` | never enables Receive Again on a successful send outcome |
| `user-abandonment-clears` | forgets Receive Again after failed user Done even while the successful outcome stays held |
| `stale-work-fence` | forgets the parent on Done and fences an already waiting repeat |
| `replacement-owner-finalizer` | keeps a newly admitted parent when an obsolete repeat finalizes before its pending render |
| `show-done-lock` | ignores Done while Show owns the outcome without forgetting its repeat parent |
| `app-show-lock` | locks all outcome controls during Show and restores its initiating focus after success |
| `app-repeat-entry` | clicks Receive Again through the real controller, blocks overlapping actions and focuses the fresh code without duplicate speech |
| `app-outcome-busy` | clicks Receive Again through the real controller, blocks overlapping actions and focuses the fresh code without duplicate speech |
| `show-focus-capture` | restores the Show initiating control after disabling it without relying on pointer focus |
| `show-focus-restore` | restores the Show initiating control after disabling it without relying on pointer focus |
| `retirement-error-purpose` | renders truthful fixed retirement error separately from Show in Folder failure |

All 19 passed their named baselines and failed their named assertions on the final corrected production/test source. Full final local logs: `/tmp/fairdrop-receive-again-review-mutations-02/<case>-baseline.log` and `<case>-mutation.log`; summary `/tmp/fairdrop-receive-again-review-mutations-02.log`. The first corrected 19-case pass is retained separately at `/tmp/fairdrop-receive-again-review-mutations-01/` and its sibling summary. The initial implementation's seven-case pass remains at `/tmp/fairdrop-receive-again-mutations/`; it is historical and does not claim the later review fixes. CI runs the exact final inventory on native desktop runners and retains per-OS logs even on failure. `verify_workflow_test.go` pins command, order and artifact retention. `verify_receive_again_mutations_test.py` pins complete inventory/delegation; the existing receive-text parser/platform suite covers the reused harness.

## Review corrections and triage

The main session completed the independent review layers and consolidated the following corrections. All accepted findings were **patch** within frozen intent: no `intent_gap`, `bad_spec` or deferral. The reset lock was high severity; ownership/render side effects, Show/Done overlap, focus loss and proof gaps were medium; evidence precision was low.

| Item | Disposition and proof |
| --- | --- |
| 1. Missing reset locks UI indefinitely | Patched with explicit 15-second Receive Again-only wait. Fake-timer controller and real-App tests prove unlock/error/retryability and no late-reset rearm; `reset-bound` mutation kills. The unrelated Send Again wait is unchanged. |
| 2. Show disables its focused button and loses focus | Patched explicit pointer focus capture and fenced success restoration. Component tests avoid relying on click focus, explicitly model lost focus, and reject restoration over another session/error/focus move. Chromium test measures native disabling blur and restored forced-colors outline. Both focus mechanism mutations kill. |
| 3. Direct Done overlaps Show | Patched controller refusal while Show owns the outcome, preserving deliberate abandonment during pending repeat. Real-App deferred Show proves all three controls disabled, no Cancel, and settled usability. Show also succeeds after a failed retirement and clears that error. Controller/App wiring mutations kill. |
| 4. Parent clearing during render | Patched to committed lifecycle effect and explicit admission/abandonment handlers. Derived eligibility and command state guards reject invalid outcomes immediately. |
| 5. Unmount while repeated StartReceive is unresolved | Added deterministic cancellation-count and late-metadata non-commit proof. |
| 6. Native unsafe replacement and evidence precision | Repeated composition now refuses missing/file/symlink replacements before network discovery, using the existing directory-link helper and native privilege policy. The giant-length POST proves a fresh space check only. `TestTheSpaceReserveBoundary`, `TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve`, and `TestCheckSpaceAsksTheRealVolume` separately prove reserve arithmetic and the real volume path. |
| 7. Combined state mutation overclaims exclusions | Split waiting guard from named incomplete/cancelled/text/send eligibility tests/mutations. All five final cases kill their own named assertion. |
| 8. Old finalizer may clear a replacement parent | Confirmed by a deterministic same-batch test: replacement StartReceive enters while public controller state still reads Idle, then resolves old release. Old finalizer now checks epoch and parent ownership. `replacement-owner-finalizer` kills when that fence is removed. |
| 9. Complete enqueue may race terminal state commit | **Rejected after source audit.** `acceptTerminal` owns the operation lease across terminal publication and settled-state commit; `Cancel` calls `awaitLeaseBounded` before `retire` and reset. Enqueue therefore does not let Cancel bypass the commit. No arbitrary sleep or redundant synchronization was added. |

The independent verification reviewer rechecked the corrected App lock, bounded reset wait, owner-scoped finalizer, conditional focus restoration and final 19-case inventory. No actionable regression or verification gap remained. The orchestrator independently inspected the corrected controller, App/view wiring and native composition.

## Focused verification after corrections

- Six controller/view/App files: **235 passed**. Final output `/tmp/fairdrop-receive-again-review-focused-03.log`; earlier corrected passes are retained at `review-focused-01.log` and `review-focused-02.log` under the same `/tmp/fairdrop-receive-again-` prefix.
- Rendered receive browser suite: **41 passed**, including actual disabled-button blur and restored focus. Final output `/tmp/fairdrop-receive-again-review-browser-02.log`; earlier pass retained at `review-browser-01.log`.
- Native repeated composition and workflow pins: **passed**. Final output `/tmp/fairdrop-receive-again-review-native-02.log`; earlier pass retained at `review-native-01.log`.
- TypeScript `tsc --noEmit -p frontend/tsconfig.json`: passed. Final output `/tmp/fairdrop-receive-again-review-types-02.log`.
- Shared mutation runner/inventory Python suites: **12 passed**. Output `/tmp/fairdrop-receive-again-review-python-02.log`.
- Final canonical inventory: **19/19 named baseline passes and assertion kills**, all 38 complete logs retained in the final directory above.
- `git diff --check`: passed. Existing `build/.DS_Store` preserved.

## Failed attempts and log retention

The first review UI attempt failed two new focus tests because jsdom refuses `blur()` on a disabled button; complete output is retained at `/tmp/fairdrop-receive-again-review-ui-01.log`. The corrected harness explicitly focuses body, and all 40 component/App cases passed in `/tmp/fairdrop-receive-again-review-ui-02.log`. Actual disabling blur is proved separately in Chromium, not inferred from jsdom.

Earlier initial-implementation failures are recorded honestly: the first refusal assertion used nonexistent `failedItemKind` instead of `commandContext`; an App integration test initially observed state before waiting committed and used the wrong focus-target name; the admission mutation initially survived its redundant cancellation fence; the error-copy mutant initially threw a Testing Library lookup error rather than an assertion kill. Those were corrected before the initial seven-case pass. Their original tool output remains in the implementation conversation, but complete separate failure files were not retained then, and this evidence does not claim otherwise. All review-correction attempts use unique paths; no earlier review failure log was overwritten.

## Full-gate copy registration correction

The main session's first ordered full gate passed Wails/build assets, gofmt, vet, staticcheck, Go/native matrix, cgo and race, then found **two frontend failures**: the explicit approved-copy key inventory omitted `receive.again`/`receive.retireFailed`, and the canonical UX copy table omitted both rows. Complete original failure output is retained at `/private/tmp/fairdrop-receive-again-gate-01/frontend.log` (**1371 passed, 2 failed**). This was a missing registration, not a reason to loosen either exact inventory or table-value matching.

The correction adds both keys to the existing literal inventory and both exact final strings to the UX Voice and Tone table (`copy.receive.again` and `copy.receive.retire_failed`). The table also now describes Show in Folder's secondary weight beside Receive Again, and its failure-card explanation accurately distinguishes a forgotten failed parent from a successful repeat target. No production source, canonical mutation case or named mutation target changed.

- Focused copy suite: **25 passed**, `/tmp/fairdrop-receive-again-copy-remediation-focused-01.log`.
- Full frontend suite: **1373 passed in 43 files**, `/tmp/fairdrop-receive-again-copy-remediation-full-01.log`.
- Canonical inventory/parser validation: **12 passed**, `/tmp/fairdrop-receive-again-copy-remediation-inventory-01.log`; all 19 case anchors and named tests remain registered. The final 19-case execution above remains applicable because its production sources and target tests are unchanged.
- `git diff --check`: passed. The failed full-gate log and all earlier review logs were preserved.

## Full-gate outbound text mutation-anchor correction

The main session's next full-gate continuation passed the preceding stages, including the 86-case native inventory and 10-case outbound text Go inventory, then stopped in the existing 13-case outbound text frontend runner: `explicit-cancel-forget` found zero source anchors after Receive Again inserted its own parent clearing before the old text-clearing anchor. The original failure is preserved at `/private/tmp/fairdrop-receive-again-gate-02/text-frontend-mutations.log`. This was an inventory maintenance failure, not a failed text-forgetting guarantee.

Only that case's before/after anchor strings in `scripts/verify-text-frontend-mutations.py` changed. Both strings now include the new `receiveParentRef.current = null` line unchanged; the mutant still removes **only** `rememberedTextRef.current = null` and preserves receive-parent and send-path clearing. Production source is unchanged. The complete 13-case inventory was audited, including every sequential multi-file patch and named test: every anchor matches exactly once. Audit output is retained at `/tmp/fairdrop-receive-again-text-anchor-audit-01.log`.

The exact full canonical runner then passed all **13 named baselines and 13 assertion kills**. Summary: `/tmp/fairdrop-receive-again-text-mutations-remediation-01.log`; all 26 complete baseline/mutant logs are retained separately under `/tmp/fairdrop-receive-again-text-mutations-remediation-01/`. No subset was substituted and the failed gate output was not overwritten. `git diff --check` passed. Sources, inventory and evidence are frozen for the main session's remaining gate continuation.

## Full gate and limitations

Main owns corrected review acceptance and the full ordered local/native CI gate, integration and remote operations. No manual phone, WKWebView, screen-reader or firewall pass is claimed. No commit, push, version change or release was performed by the implementer.

## Orchestrator local gate acceptance

The ordered gate completed against the frozen implementation in `/private/tmp/fairdrop-receive-again-91gn47lg`. Full logs are retained in `/private/tmp/fairdrop-receive-again-gate-01/` and `...-gate-02/`; the final outbound text mutation correction uses the separately retained remediation logs above. The first pass stopped on the two copy-registration assertions. After only copy-test/documentation corrections, verification resumed at frontend; the final mutation anchor correction changed only its before/after strings. Production source was byte-compared against the tested snapshot and remained unchanged. These are recorded corrections, not discarded failures.

- Wails macOS build, binding/build-asset drift and `.gitkeep`, gofmt, vet and pinned staticcheck: passed.
- Full Go suite and explicit native integration matrix: passed. Actual cgo probe succeeded with `CGO_ENABLED=1`; every package passed the race suite (including the streaming package).
- Frontend: 1,373 tests across 43 files passed. Browser: 136 tests across five files passed, plus live receiver/download and three-file multipart upload checks.
- Runner Python tests: 11 shared receive-text tests plus one Receive Again inventory test passed.
- Complete canonical inventories: native 86, outbound text Go 10, outbound text frontend 13, receive-text frontend 9, Receive Again 19 — all named baselines and assertion kills passed.
- Line endings, Darwin arm64 and Linux amd64 builds, and bare-binary Darwin staticcheck: passed.

The unusable-lock native application smoke intentionally runs only on CI under its script policy. Exact-head Windows/macOS/Linux CI is required before merge and will be linked from the pull request. No manual phone/WKWebView result is claimed. Version remains 1.8.0.
