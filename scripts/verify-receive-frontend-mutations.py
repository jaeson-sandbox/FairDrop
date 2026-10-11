#!/usr/bin/env python3
"""Run the canonical receive frontend mutations in isolated copies.

Story B of phone-to-desktop receiving. This one script is the whole inventory:
the evidence file's mutation table is derived from CASES below, never from a
hand-copied list, so a case that exists here cannot be omitted from the proof
and a case that is not here cannot be claimed.

Each case, in a throwaway copy of frontend/ (node_modules linked, dist and
captures left out):

  1. proves a passing baseline for its test file and -t filter, and that the
     filter really selected tests (a filter that matches nothing "passes");
  2. proves every source anchor matches exactly once, so a mutation that
     silently did nothing cannot be reported as a kill;
  3. applies the mutation and requires the run to FAIL, with a FAIL line in the
     named test file whose test path contains the named assertion;
  4. refuses a kill that is only a compile or import failure.

Complete stdout/stderr of every baseline and mutation run is kept in
_bmad-output/implementation-artifacts/evidence-phone-to-desktop-receiving-frontend-logs/.

Usage: python3 scripts/verify-receive-frontend-mutations.py [case-name ...]
With no names every case runs. A scoped run proves only the cases it executed.
"""
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-phone-to-desktop-receiving-frontend-logs"
NODE = shutil.which("node")

VALIDATION_TEST = "src/transfer/validation.receive.test.ts"
STATE_TEST = "src/transfer/state.receive.test.ts"
CONTROLLER_TEST = "src/transfer/useTransfer.receive.test.tsx"
OUTCOME_TEST = "src/ui/ReceiveOutcomeView.test.tsx"
WAITING_TEST = "src/ui/ReceiveWaitingView.test.tsx"
RECEIVING_TEST = "src/ui/ReceivingView.test.tsx"
IDLE_TEST = "src/ui/IdleView.receive.test.tsx"
ANNOUNCE_TEST = "src/ui/announce.receive.test.ts"
COPY_TEST = "src/ui/copy.receive.test.ts"
APP_TEST = "src/App.receive.test.tsx"
BROWSER_TEST = "browser/receive.test.tsx"

# (name, requirement, edits, test file, -t filter, text the killing FAIL line must contain)
# An edit is (file relative to frontend/, old text, new text); old must match once.
# A test file under browser/ runs in the rendered-Chromium project (vitest.browser.config.ts);
# every other case runs under jsdom.
CASES = (
    # ---- chooser dismissal is a no-op ------------------------------------
    ("dismissal-noop", "a whitespace-only chooser result is a dismissal too",
     (("src/transfer/useTransfer.ts",
       "if (typeof selected !== 'string' || selected.trim() === '') return 'dismissed'",
       "if (typeof selected !== 'string' || selected === '') return 'dismissed'"),),
     CONTROLLER_TEST, "treats a whitespace-only result like a dismissal",
     "treats a whitespace-only result like a dismissal"),
    ("dismissal-startreceive", "a dismissed chooser never reaches StartReceive",
     (("src/transfer/useTransfer.ts",
       "if (typeof selected !== 'string' || selected.trim() === '') return 'dismissed'",
       "if (typeof selected !== 'string') return 'dismissed'"),),
     CONTROLLER_TEST, "does nothing when the chooser is dismissed",
     "does nothing when the chooser is dismissed"),
    ("dismissal-focus-return", "a dismissed chooser returns focus to Receive",
     (("src/ui/IdleView.tsx",
       "if (result !== 'dismissed' || !button.isConnected) return",
       "if (!button.isConnected) return"),),
     IDLE_TEST, "focus after the chooser",
     "does not touch focus or the marker when the activation ends"),
    ("focus-ring-marker-pointer", "a pointer activation never marks the scripted focus return",
     (("src/ui/IdleView.tsx",
       "setReceiveFocusReturned(fromKeyboard)",
       "setReceiveFocusReturned(true)"),),
     IDLE_TEST, "focus after the chooser",
     "returns focus after a pointer activation without marking it"),

    # ---- single flight ----------------------------------------------------
    ("double-activation", "a second Receive while the chooser is open is refused",
     (("src/transfer/useTransfer.ts",
       "if (stageOperationRef.current !== null || browseOperationRef.current !== null) return 'ignored'",
       "if (stageOperationRef.current !== null) return 'ignored'"),),
     CONTROLLER_TEST, "single flight",
     "refuses a second activation while the chooser is open"),
    ("receive-outside-idle", "Receive is admitted from Idle only",
     (("src/transfer/useTransfer.ts",
       "if (!mountedRef.current || stateRef.current.phase !== 'idle') return 'ignored'\n        if (stageOperationRef.current !== null || browseOperationRef.current !== null) return 'ignored'",
       "if (!mountedRef.current) return 'ignored'\n        if (stageOperationRef.current !== null || browseOperationRef.current !== null) return 'ignored'"),),
     CONTROLLER_TEST, "refuses Receive outside Idle",
     "refuses Receive outside Idle"),
    ("cancel-invalidates-chooser", "Cancel invalidates a chooser result that arrives late",
     (("src/transfer/useTransfer.ts",
       "if (browseOperationRef.current?.receive === true) browseOperationRef.current = null",
       "void browseOperationRef.current"),),
     CONTROLLER_TEST, "invalidated chooser completions",
     "ignores a folder chosen after Cancel was activated"),
    ("unmount-invalidates-chooser", "unmount invalidates a chooser result that arrives late",
     (("src/transfer/useTransfer.ts",
       "return mountedRef.current && browseOperationRef.current === operation",
       "return browseOperationRef.current === operation"),
      ("src/transfer/useTransfer.ts",
       "            activeCancelRef.current = null\n            browseOperationRef.current = null\n            rememberedPathsRef.current = null",
       "            activeCancelRef.current = null\n            rememberedPathsRef.current = null"),
      ("src/transfer/useTransfer.ts",
       "const startReceiveAt = useCallback(async (folder: string): Promise<void> => {\n        if (!mountedRef.current || stateRef.current.phase !== 'idle') return",
       "const startReceiveAt = useCallback(async (folder: string): Promise<void> => {\n        if (stateRef.current.phase !== 'idle') return")),
     CONTROLLER_TEST, "invalidated chooser completions",
     "ignores a folder chosen after unmount"),

    # ---- malformed receive metadata and events are rejected ---------------
    ("metadata-send-url", "a send (download) url is refused as receive metadata",
     (("src/transfer/validation.ts",
       "(\\/upload\\/[0-9a-f]{32})$/",
       "(\\/(?:upload|download)\\/[0-9a-f]{32})$/"),),
     VALIDATION_TEST, "refuses a send",
     "refuses a send (download) url"),
    ("metadata-extra-field", "extra receive metadata fields are refused",
     (("src/transfer/validation.ts",
       "if (record === null || !ownsExactly(record, receiveMetadataFields)) return null",
       "if (record === null || !ownsEvery(record, receiveMetadataFields)) return null"),),
     VALIDATION_TEST, "refuses an extra field",
     "refuses an extra field"),
    ("metadata-control-chars", "a destination with control characters is refused",
     (("src/transfer/validation.ts",
       "        !/\\p{Cc}/u.test(value)",
       "        true"),),
     VALIDATION_TEST, "refuses a destination with a control",
     "refuses a destination with a control character"),
    ("metadata-session-id", "a malformed receive session id is refused",
     (("src/transfer/validation.ts",
       "if (!isSessionId(sessionId) || !isDestinationName(destination)) return null",
       "if (!isDestinationName(destination)) return null"),),
     VALIDATION_TEST, "refuses a non-hex session id",
     "refuses a non-hex session id"),
    ("status-extra-field", "a receive payload naming anything beyond the pinned keys is refused",
     (("src/transfer/validation.ts",
       "if (Object.keys(record).some((key) => !allowed.includes(key))) return null",
       "void allowed"),),
     VALIDATION_TEST, "refuses an extra field naming a path",
     "refuses an extra field naming a path"),
    ("status-complete-result", "an error event can never say complete",
     (("src/transfer/validation.ts",
       "if (kind === 'complete' ? result !== 'complete' : result === 'complete') return null",
       "if (kind === 'complete' ? result !== 'complete' : false) return null"),),
     VALIDATION_TEST, "refuses error saying complete",
     "refuses error saying complete"),
    ("cancel-code-contradiction", "a cancelled result beside a non-cancelled code is refused",
     (("src/transfer/validation.ts",
       "if ((receive.result === 'cancelled') !== cancelledCode) return null",
       "if (false) return null"),),
     VALIDATION_TEST, "a contradiction is never guessed at",
     "a contradiction is never guessed at"),
    ("controller-malformed-ack", "a malformed StartReceive acknowledgement quiesces and says nothing started",
     (("src/transfer/useTransfer.ts",
       "const metadata = parseReceiveMetadata(rawMetadata)",
       "const metadata = rawMetadata as never"),),
     CONTROLLER_TEST, "refuses a malformed acknowledgement",
     "refuses a malformed acknowledgement"),

    ("receive-draft-open", "Receive is refused while the collection draft is open",
     (("src/transfer/useTransfer.ts",
       "if (draftPathsRef.current !== null || draftChooserRef.current !== null || draftAdmissionRef.current) return 'ignored'\n        return browse(SelectReceiveFolder",
       "return browse(SelectReceiveFolder"),),
     CONTROLLER_TEST, "send flows stay out of a receive session",
     "refuses Receive while the collection draft is open, without opening a chooser"),

    # ---- Show in Folder ---------------------------------------------------
    ("show-hidden-without-subfolder", "Show in Folder is not rendered without a subfolder",
     (("src/ui/ReceiveOutcomeView.tsx",
       "{outcome.subfolderExists ? (\n                    <button",
       "{true ? (\n                    <button"),),
     OUTCOME_TEST, "is not rendered for a",
     "is not rendered for a complete outcome with no subfolder"),
    ("show-flag-not-count", "Show in Folder follows the backend flag, not the saved count",
     (("src/ui/ReceiveOutcomeView.tsx",
       "{outcome.subfolderExists ? (\n                    <button",
       "{saved ? (\n                    <button"),),
     OUTCOME_TEST, "follows the backend flag",
     "follows the backend flag, not the count"),
    ("show-controller-guard", "the controller never calls ShowReceivedFolder without a subfolder",
     (("src/transfer/useTransfer.ts",
       "|| !current.outcome.subfolderExists) return",
       ") return"),),
     CONTROLLER_TEST, "never calls the command when the subfolder does not exist",
     "never calls the command when the subfolder does not exist"),
    ("show-single-launch", "a double activation of Show in Folder launches once",
     (("src/transfer/useTransfer.ts",
       "if (showFolderPendingRef.current) return\n        showFolderPendingRef.current = true",
       "showFolderPendingRef.current = true"),),
     CONTROLLER_TEST, "launches once for a double activation",
     "launches once for a double activation"),
    ("show-failure-note", "a failed Show in Folder is reported as a fixed note",
     (("src/transfer/state.ts",
       "return {...state, commandError: publicError('path_not_found')}",
       "return state"),),
     STATE_TEST, "records a failed Show in Folder",
     "records a failed Show in Folder beside the outcome"),

    # ---- the saved count and its wording ---------------------------------
    ("saved-count-plural", "1 file, not 1 files",
     (("src/ui/copy.ts",
       "`${count} ${count === 1 ? 'file' : 'files'} saved`",
       "`${count} files saved`"),),
     COPY_TEST, "the composed count copy",
     "the composed count copy > says 1 as"),
    ("saved-count-zero", "no files saved is said as no files saved",
     (("src/ui/copy.ts",
       "return count === 0 ? 'no files saved' : `${count}",
       "return count === 0 ? '0 files saved' : `${count}"),),
     COPY_TEST, "the composed count copy",
     "the composed count copy > says 0 as"),
    ("saved-count-receiving-view", "the receiving view shows the backend's saved count",
     (("src/ui/ReceivingView.tsx",
       "copy.receive.filesSaved(state.filesSaved)",
       "copy.receive.filesSaved(0)"),),
     RECEIVING_TEST, "says",
     "the receiving view > says 2 saved"),
    ("saved-count-outcome-heading", "the outcome heading carries the exact count",
     (("src/ui/ReceiveOutcomeView.tsx",
       "? copy.receive.incompleteHeading(outcome.filesSaved)",
       "? copy.receive.incompleteHeading(0)"),),
     OUTCOME_TEST, "an incomplete receive is never shown as a success",
     "heads it \"Upload incomplete\" with the exact count"),
    ("saved-count-monotonic", "the saved count never goes down while receiving",
     (("src/transfer/state.ts",
       "|| event.receive.filesSaved < state.filesSaved) return state",
       ") return state"),),
     STATE_TEST, "refuses progress that goes backwards",
     "refuses progress that goes backwards, changes the declared total, or lowers the saved count"),

    # ---- an incomplete or cancelled upload is never shown as complete ----
    ("incomplete-as-complete-reducer", "an incomplete error event becomes an incomplete outcome",
     (("src/transfer/state.ts",
       "return receiveOutcomeState(state, session, event.receive, event.receive.result)",
       "return receiveOutcomeState(state, session, event.receive, 'complete')"),),
     STATE_TEST, "shows an incomplete upload as incomplete",
     "shows an incomplete upload as incomplete with the exact count"),
    ("incomplete-as-complete-demotion", "a success that cannot be reconciled is demoted to incomplete",
     (("src/transfer/state.ts",
       "coherent ? 'complete' : 'incomplete'",
       "'complete'"),),
     STATE_TEST, "demotes a success",
     "demotes a success whose snapshot does not follow the accepted progress to incomplete"),
    ("incomplete-as-complete-view", "the incomplete heading is never the success heading",
     (("src/ui/ReceiveOutcomeView.tsx",
       "? copy.receive.incompleteHeading(outcome.filesSaved)",
       "? copy.receive.completeHeading(outcome.filesSaved)"),),
     OUTCOME_TEST, "an incomplete receive is never shown as a success",
     "heads it \"Upload incomplete\" with the exact count"),
    ("incomplete-styling", "an incomplete outcome does not take the success styling",
     (("src/ui/ReceiveOutcomeView.tsx",
       "const complete = outcome.kind === 'complete'",
       "const complete = outcome.kind !== 'cancelled'"),),
     OUTCOME_TEST, "an incomplete receive is never shown as a success",
     "heads it \"Upload incomplete\" with the exact count"),
    ("cancelled-heading", "a cancelled upload says cancelled, not incomplete",
     (("src/ui/ReceiveOutcomeView.tsx",
       ": copy.receive.cancelledHeading(outcome.filesSaved)",
       ": copy.receive.incompleteHeading(outcome.filesSaved)"),),
     OUTCOME_TEST, "a cancelled receive",
     "heads it \"Upload cancelled\" with the exact count"),

    # ---- Done leaves through Cancel --------------------------------------
    ("done-calls-cancel", "Done leaves through the existing CancelTransfer",
     (("src/App.tsx",
       "onDone={() => void transfer.cancel()}",
       "onDone={() => void transfer.dismissRetained()}"),),
     APP_TEST, "Done leaves through the existing Cancel command",
     "Done leaves through the existing Cancel command, never a new one"),
    ("done-button-wiring", "the Done button calls onDone",
     (("src/ui/ReceiveOutcomeView.tsx",
       "data-receive-done\n                    onClick={onDone}",
       "data-receive-done\n                    onClick={onShowInFolder}"),),
     OUTCOME_TEST, "leaving",
     "Done calls onDone once and nothing else"),
    ("controller-cancel-outcome", "the controller cancels a held receive outcome",
     (("src/transfer/useTransfer.ts",
       "if (current.phase === 'done' || current.phase === 'error' || current.phase === 'receive-outcome') {",
       "if (current.phase === 'done' || current.phase === 'error') {"),),
     CONTROLLER_TEST, "leaves the outcome through the existing Cancel command",
     "leaves the outcome through the existing Cancel command, and only the reset returns to Idle"),
    ("outcome-held", "a receive outcome is held, not reset by anything but the user's Done",
     (("src/transfer/state.ts",
       "return event.kind === 'transfer-reset' ? createInitialTransferState() : state",
       "return event.kind === 'transfer-reset' || event.kind === 'transfer-progress' ? createInitialTransferState() : state"),),
     STATE_TEST, "is held: nothing but a reset moves it",
     "is held: nothing but a reset moves it"),

    # ---- the too-large notice keeps the waiting view ---------------------
    ("notice-keeps-waiting", "a space refusal keeps the session waiting",
     (("src/transfer/state.ts",
       "return {...state, session, refusals: state.refusals + 1}",
       "return createInitialTransferState()"),),
     STATE_TEST, "counts each refused upload",
     "counts each refused upload as a new state and keeps waiting"),
    ("notice-count", "each refusal is a new state, so it is announced again",
     (("src/transfer/state.ts",
       "return {...state, session, refusals: state.refusals + 1}",
       "return {...state, session}"),),
     STATE_TEST, "counts each refused upload",
     "counts each refused upload as a new state and keeps waiting"),
    ("notice-announced", "a refusal is spoken by the announcer",
     (("src/ui/announce.ts",
       "if (next.refusals > previous.refusals && !next.cancelPending)",
       "if (false)"),),
     ANNOUNCE_TEST, "a refused upload",
     "a refused upload > is spoken by the announcer, not focused"),
    ("notice-not-over-cancel", "a refusal is not spoken over Canceling",
     (("src/ui/announce.ts",
       "if (next.refusals > previous.refusals && !next.cancelPending)",
       "if (next.refusals > previous.refusals)"),),
     ANNOUNCE_TEST, "is not spoken over a pending cancellation",
     "is not spoken over a pending cancellation"),
    ("notice-visible", "a refusal leaves a visible note beside the code",
     (("src/ui/ReceiveWaitingView.tsx",
       "{state.refusals > 0 ? (\n                    <aside id=\"fd-receive-refused\"",
       "{false ? (\n                    <aside id=\"fd-receive-refused\""),),
     WAITING_TEST, "a refused upload",
     "keeps waiting and says what happened"),

    # ---- send flows are never offered on a receive outcome ---------------
    ("receive-not-retained", "a receive outcome is never retained as a send outcome card",
     (("src/transfer/state.ts",
       "return event.kind === 'transfer-reset' ? createInitialTransferState() : state",
       "return event.kind === 'transfer-reset' ? {phase: 'idle', retainedOutcome: {kind: 'done', receipt: {name: state.destination, isDir: true, bytesSent: 0}}, commandError: null} : state"),),
     STATE_TEST, "leaves to plain Idle on reset",
     "leaves to plain Idle on reset, retaining nothing a send outcome card could offer"),
    ("receive-forgets-send-path", "starting a receive forgets any remembered send path",
     (("src/transfer/useTransfer.ts",
       "        selectionEpochRef.current += 1\n        rememberedPathsRef.current = null\n        const promise = Promise.resolve().then(() => StartReceive(folder)",
       "        selectionEpochRef.current += 1\n        const promise = Promise.resolve().then(() => StartReceive(folder)"),),
     CONTROLLER_TEST, "forgets a remembered send path when a receive session starts",
     "forgets a remembered send path when a receive session starts"),
    ("send-rejects-receive-events", "a send session never applies a receive payload",
     (("src/transfer/state.ts",
       "event.receive !== undefined) return state\n    }",
       "false) return state\n    }"),),
     STATE_TEST, "ignores receive progress, complete and error on a transferring send session",
     "ignores receive progress, complete and error on a transferring send session"),
    ("receive-rejects-send-events", "a receive session never applies a send event",
     (("src/transfer/state.ts",
       "if (event.receive === undefined || !event.progress.totalKnown) return state",
       "if (!event.progress.totalKnown) return state"),),
     STATE_TEST, "refuses progress with no receive payload",
     "refuses progress with no receive payload and progress with an unknown total"),
    ("send-ack-over-receive", "a send acknowledgement cannot commit over a pending receive",
     (("src/transfer/state.ts",
       "state.cancelPending ||\n                state.itemKind === 'receive') return state",
       "state.cancelPending) return state"),),
     STATE_TEST, "refuses to commit a send acknowledgement over a pending receive",
     "refuses to commit a send acknowledgement over a pending receive"),
    ("receive-error-context", "a failed receive start uses receive wording and receive actions",
     (("src/transfer/state.ts",
       "idleWithError(action.error.code === 'cancelled' ? null : action.error, state.itemKind === 'receive')",
       "idleWithError(action.error.code === 'cancelled' ? null : action.error)"),),
     STATE_TEST, "marks a failed receive start",
     "marks a failed receive start so the failure card uses receive wording"),
    ("receive-card-wording", "a receive failure card reads the receive copy",
     (("src/ui/OutcomePanel.tsx",
       "errorHeadingFor(outcome.error.code, receive)",
       "errorHeadingFor(outcome.error.code, false)"),),
     IDLE_TEST, "a failed Receive card",
     "uses receive wording for the folder it could not use"),
    ("receive-retry-action", "a receive failure card offers a retry that re-opens the folder chooser",
     (("src/App.tsx",
       "onRetry: again ? receiveAgain : undefined,",
       "onRetry: undefined,"),),
     APP_TEST, "a failed Receive start",
     "says Try Again for a failure that is worth another attempt, and reopens the Receive chooser"),
    ("receive-choose-label", "a receive failure card for an unusable folder says Choose Another Folder",
     (("src/App.tsx",
       "retryLabel: action === 'choose' ? copy.receive.chooseAnotherFolder : undefined,",
       "retryLabel: undefined,"),),
     APP_TEST, "a failed Receive start",
     "shows receive wording and a folder-chooser action, never the send ones"),

    # ---- a receive nobody claimed can still end (review patch 2) ---------
    ("waiting-error-accepted", "a terminal error arriving while waiting ends the session instead of leaving a dead QR",
     (("src/transfer/state.ts",
       "if (receive === undefined || receive.result !== 'incomplete' ||",
       "if (true || receive === undefined || receive.result !== 'incomplete' ||"),),
     STATE_TEST, "accepts a terminal incomplete outcome with nothing saved while still waiting",
     "accepts a terminal incomplete outcome with nothing saved while still waiting"),
    ("waiting-error-accepted-controller", "the same ending reaches the screen through the real parser and controller",
     (("src/transfer/state.ts",
       "if (receive === undefined || receive.result !== 'incomplete' ||",
       "if (true || receive === undefined || receive.result !== 'incomplete' ||"),),
     CONTROLLER_TEST, "closes it before any phone claimed it",
     "ends a waiting session with an incomplete outcome when the backend closes it before any phone claimed it"),
    ("waiting-error-coherence", "a waiting error claiming saved files or a folder is refused, not guessed at",
     (("src/transfer/state.ts",
       "receive.filesSaved !== 0 || receive.subfolderExists) return state",
       "false) return state"),),
     STATE_TEST, "still drops a waiting error",
     "still drops a waiting error whose receive payload is missing, unresolved or claims saved files"),
    ("waiting-error-result-kind", "only an incomplete result can end a session nobody claimed",
     (("src/transfer/state.ts",
       "receive.result !== 'incomplete' ||\n                receive.filesSaved",
       "receive.result === null ||\n                receive.filesSaved"),),
     STATE_TEST, "still drops a waiting error",
     "still drops a waiting error whose receive payload is missing, unresolved or claims saved files"),
    ("route-unclaimed-outcome-focus", "an outcome reached straight from waiting still moves focus to the outcome",
     (("src/ui/announce.ts",
       "if (previous.phase !== 'receive-outcome') return focusRow('receive-outcome', 'receive-outcome')",
       "if (previous.phase !== 'receive-outcome' && previous.phase !== 'receive-waiting') return focusRow('receive-outcome', 'receive-outcome')"),),
     ANNOUNCE_TEST, "receive nobody claimed ends while still waiting",
     "focuses the outcome when a receive nobody claimed ends while still waiting"),
    ("app-unclaimed-outcome", "App replaces the waiting QR with the outcome and focuses it when the session ends unclaimed",
     (("src/ui/announce.ts",
       "if (previous.phase !== 'receive-outcome') return focusRow('receive-outcome', 'receive-outcome')",
       "if (previous.phase !== 'receive-outcome' && previous.phase !== 'receive-waiting') return focusRow('receive-outcome', 'receive-outcome')"),),
     APP_TEST, "before any phone claimed it",
     "replaces a waiting QR with an incomplete outcome when the session ends before any phone claimed it"),

    # ---- routing, focus and the drag guard -------------------------------
    ("route-leave-outcome", "leaving the outcome focuses the Idle instruction",
     (("src/ui/announce.ts",
       "if (previous.phase === 'receive-outcome') return focusRow('receive-leave', 'idle-instruction')",
       "if (previous.phase === 'receive-outcome') return null"),),
     ANNOUNCE_TEST, "focuses the Idle instruction when the outcome is left",
     "focuses the Idle instruction when the outcome is left"),
    ("route-outcome-focus", "a receive outcome moves focus once to the outcome",
     (("src/ui/announce.ts",
       "if (previous.phase !== 'receive-outcome') return focusRow('receive-outcome', 'receive-outcome')",
       "if (previous.phase !== 'receive-outcome') return null"),),
     ANNOUNCE_TEST, "focuses the outcome when a receive ends",
     "focuses the outcome when a receive ends, whichever way it ended"),
    ("qr-draggable", "the receive QR image cannot start a drag",
     (("src/ui/ReceiveWaitingView.tsx",
       "                            draggable={false}\n",
       "                            draggable\n"),),
     WAITING_TEST, "marks the QR image non-draggable",
     "marks the QR image non-draggable"),
    ("marking-warning", "the marking warning appears when the backend reports it",
     (("src/ui/ReceiveOutcomeView.tsx",
       "const markingStep = outcome.markingWarning ? step++ : null",
       "const markingStep = null"),),
     OUTCOME_TEST, "the marking warning",
     "appears when the backend says marking failed"),

    # ---- rendered Chromium: layout, targets, forced colors ----------------
    ("browser-forced-colors-outline", "the Receive control keeps a real outline after a scripted focus return under forced colors",
     (("src/style.css",
       "    .fd-url:focus-visible,\n    .fd-button[data-focus-return],\n    .fd-disclosure__summary:focus-visible,",
       "    .fd-url:focus-visible,\n    .fd-disclosure__summary:focus-visible,"),),
     BROWSER_TEST, "forced colors on the receive views",
     "draws a real outline, not a box-shadow, on the Receive control after a scripted focus return"),
    ("browser-reflow-320", "the receive views reflow to one column at 320 CSS pixels",
     (("src/style.css",
       "@media (max-width: 759px) {\n    .fd-hero {\n        grid-template-columns: minmax(0, 1fr);",
       "@media (max-width: 759px) {\n    .fd-hero {\n        grid-template-columns: 216px minmax(0, 1fr);"),),
     BROWSER_TEST, "the receive views at 320 CSS pixels",
     "scrolls only vertically with nothing clipped"),
    ("browser-clipped-text", "the receive views grow under 200% text and text spacing without clipping",
     (("src/style.css",
       ".fd-hero__details {\n    display: flex;",
       ".fd-hero__details {\n    block-size: 40px;\n    overflow: hidden;\n    display: flex;"),),
     BROWSER_TEST, "the receive views under 200% text",
     "grows without horizontal scroll or a clipped container"),
    ("browser-qr-draggable", "the rendered receive QR image is not draggable",
     (("src/ui/ReceiveWaitingView.tsx",
       "                            draggable={false}\n",
       "                            draggable\n"),),
     BROWSER_TEST, "the receive QR image cannot start a drag",
     "marks the receive QR non-draggable both ways"),
    ("browser-outcome-centred", "the held outcome card is centred, not pinned to the top",
     (("src/style.css",
       ".fd-app > .fd-outcome,\n.fd-region > .fd-outcome:only-child {\n    margin-block: auto;",
       ".fd-app > .fd-outcome,\n.fd-region > .fd-outcome:only-child {\n    margin-block: 0;"),),
     BROWSER_TEST, "the held outcome does not jump when it appears",
     "centres the outcome card in the window rather than pinning it to the top"),
    ("browser-entry-wrap", "the Receive entry row wraps instead of widening the page at 320 CSS pixels",
     (("src/style.css",
       ".fd-receive-entry {\n    display: flex;\n    flex-wrap: wrap;",
       ".fd-receive-entry {\n    display: flex;\n    flex-wrap: nowrap;\n    white-space: nowrap;"),),
     BROWSER_TEST, "the receive views at 320 CSS pixels",
     "scrolls only vertically with nothing clipped"),
)


def edit_target(workspace: Path, relative: str) -> Path:
    return workspace / "frontend" / relative


def run(workspace: Path, test_file: str, test_filter: str, log: Path) -> tuple[int, str]:
    if NODE is None:
        raise SystemExit("node executable not found on PATH")
    executable = workspace / "frontend/node_modules/vitest/vitest.mjs"
    command = [str(Path(NODE).resolve()), str(executable), "run"]
    if test_file.startswith("browser/"):
        command += ["--config", "vitest.browser.config.ts"]
    completed = subprocess.run(
        command + [test_file, "-t", test_filter],
        cwd=workspace / "frontend", text=True, stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, timeout=300, check=False,
    )
    log.write_text(completed.stdout, encoding="utf-8")
    return completed.returncode, completed.stdout


def fail_lines(output: str, test_file: str) -> list[str]:
    # The browser project prefixes its file with the engine, e.g. "FAIL  |chromium| browser/x.test.tsx >".
    pattern = rf"\s*FAIL\s+(?:\|[^|]+\|\s+)?{re.escape(test_file)}\s+>"
    return [line for line in output.splitlines() if re.match(pattern, line)]


def passed_test_count(output: str) -> int:
    match = re.search(r"Tests\s+(?:\d+ failed \| )?(\d+) passed", output)
    return int(match.group(1)) if match else 0


def main() -> None:
    wanted = set(sys.argv[1:])
    names = [case[0] for case in CASES]
    if len(set(names)) != len(names):
        raise SystemExit("duplicate case names in CASES")
    unknown = wanted - set(names)
    if unknown:
        raise SystemExit(f"unknown case(s): {', '.join(sorted(unknown))}")
    selected = [case for case in CASES if not wanted or case[0] in wanted]

    LOGS.mkdir(parents=True, exist_ok=True)
    for name, requirement, edits, test_file, test_filter, expected in selected:
        with tempfile.TemporaryDirectory(prefix=f"fairdrop-receive-{name}-") as directory:
            workspace = Path(directory)
            shutil.copytree(ROOT / "frontend", workspace / "frontend",
                            ignore=shutil.ignore_patterns("node_modules", "dist", "captures"))
            dependencies = workspace / "frontend/node_modules"
            try:
                dependencies.symlink_to(ROOT / "frontend/node_modules", target_is_directory=True)
            except OSError:
                shutil.copytree(ROOT / "frontend/node_modules", dependencies)

            baseline_code, baseline = run(workspace, test_file, test_filter, LOGS / f"{name}-baseline.log")
            if baseline_code != 0 or "Test Files  1 passed" not in baseline or passed_test_count(baseline) < 1:
                raise SystemExit(f"{name}: baseline failed or selected no tests; see {name}-baseline.log")

            for relative, old, new in edits:
                target = edit_target(workspace, relative)
                source = target.read_text(encoding="utf-8").replace("\r\n", "\n")
                if source.count(old) != 1:
                    raise SystemExit(f"{name}: anchor in {relative} matched {source.count(old)} times: {old!r}")
                if old == new:
                    raise SystemExit(f"{name}: mutation for {relative} changes nothing")
                target.write_text(source.replace(old, new), encoding="utf-8")

            code, output = run(workspace, test_file, test_filter, LOGS / f"{name}-mutation.log")
            if "Failed to resolve import" in output or "SyntaxError:" in output or "Transform failed" in output:
                raise SystemExit(f"{name}: compile/import failure is not an assertion kill; see {name}-mutation.log")
            failures = fail_lines(output, test_file)
            if code == 0 or not failures:
                raise SystemExit(f"{name}: mutation did not fail any test in {test_file}; see {name}-mutation.log")
            if not any(expected in line for line in failures):
                raise SystemExit(
                    f"{name}: mutation failed, but not the named assertion {expected!r}; "
                    f"failed instead: {failures}; see {name}-mutation.log")
            print(f"KILLED {name}: {expected}")
    print(f"{len(selected)}/{len(selected)} receive frontend mutations killed"
          + ("" if len(selected) == len(CASES) else f" (scoped run; {len(CASES)} are canonical)"))


if __name__ == "__main__":
    main()
