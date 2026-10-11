"""Canonical, scoped mutation inventory for phone-to-desktop receiving (Story A).

Run from the repository root with
`python3 _bmad-output/implementation-artifacts/mutate-phone-to-desktop-receiving.py`.

Every case derives from CASES below -- there is no second, hand-copied list. Each
case runs one focused top-level test twice in an isolated copy of the tree: once
unmodified (it must pass) and once with exactly one anchored source edit applied
(it must fail, and the failing test's own transcript must contain the named
assertion fragment). `scripts/mutationverdict` rejects build failures, panics,
skips, timeouts and an assertion that appears only in some other test's output.
Full transcripts are kept beside the evidence file.

Platform-specific guarantees (no-replace rename, exclusive creation, download
marking) have one anchor per platform, chosen by the host this runs on. A scoped
run proves only the cases for its own platform; the macOS and Windows CI jobs are
where the other platforms' anchors are exercised.
"""

from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-phone-to-desktop-receiving-logs"

PLATFORM = {"darwin": "darwin", "linux": "linux", "win32": "windows"}.get(sys.platform, sys.platform)

# (id, file, exact original, exact mutant, focused package, focused test, assertion fragment)
COMMON = [
    # --- storage safety (internal/sink) ---
    ("reserve_boundary", "internal/sink/sink.go", "return available-uint64(declared) >= uint64(ReserveBytes)", "return available-uint64(declared) > uint64(ReserveBytes)", "./internal/sink", "TestTheSpaceReserveBoundary", "exactly the reserve remains"),
    ("reserve_constant", "internal/sink/sink.go", "ReserveBytes int64 = 3 << 30", "ReserveBytes int64 = 2 << 30", "./internal/sink", "TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve", "want exactly 3 GiB"),
    ("partial_removal", "internal/sink/sink.go", "written, markFailed, err := d.writeAndClose(ctx, sub, file, tempName, content)\n\tif err != nil {\n\t\td.discardTemp(sub, tempName)\n", "written, markFailed, err := d.writeAndClose(ctx, sub, file, tempName, content)\n\tif err != nil {\n", "./internal/sink", "TestAReadErrorMidFileRemovesOnlyThePartialFile", "want only the completed file"),
    ("empty_subfolder", "internal/sink/sink.go", "\t\tif saved == 0 {\n", "\t\tif saved < 0 {\n", "./internal/sink", "TestAFirstFileFailureLeavesNoSubfolderOnceTheDestinationCloses", "a session that saved nothing left"),
    ("bounded_buffer", "internal/sink/sink.go", "copyBufferBytes = 64 << 10", "copyBufferBytes = 1 << 20", "./internal/sink", "TestAFileIsCopiedThroughOneBoundedBufferAndNothingGoesToOSTempStorage", "want exactly 64 KiB"),
    ("sanitize_separators", "internal/sink/naming.go", "strings.LastIndexAny(raw, `/\\`)", "strings.LastIndexAny(raw, \"/\")", "./internal/sink", "TestSanitizeNameStripsBothSeparatorsOnEveryHost", 'want "evil.dll"'),
    ("dedupe_case", "internal/sink/naming.go", "key := strings.ToLower(candidate)", "key := candidate", "./internal/sink", "TestDuplicateNamesAreNumberedBeforeTheExtensionCaseInsensitively", 'want "X (2).JPG"'),
    # --- HTTP receive (internal/server) ---
    ("overflow_abort", "internal/server/upload.go", "if b.read > b.declared {", "if false {", "./internal/server", "TestTheServerItselfAbortsWhenTheBodyExceedsItsDeclaredLength", "want incomplete with exactly 1 file saved"),
    ("inactivity_refresh", "internal/server/upload.go", "\t\t\tb.meter.record(n)\n\t\t\tb.refreshDeadline()\n", "\t\t\tb.meter.record(n)\n", "./internal/server", "TestAnUploadLongerThanTheWholeRequestDeadlineStillCompletes", "inactivity bound must never have fired"),
    ("inactivity_deadline", "internal/server/upload.go", "_ = b.controller.SetReadDeadline(time.Now().Add(b.idle))", "_ = b.idle", "./internal/server", "TestAnUploadThatStallsIsEndedByTheInactivityBound", "no terminal event within 10s"),
    ("multipart_streaming", "internal/server/upload.go", "multi, err := request.MultipartReader()\n\tif err != nil {", "multi, err := request.MultipartReader()\n\tif err == nil {\n\t\t_, _ = multi.ReadForm(1 << 20)\n\t}\n\tif err != nil {", "./internal/server", "TestALargeUploadStreamsWithBoundedMemoryAndNothingInOSTempStorage", "status = 400, want 200"),
    ("multipart_streaming_source", "internal/server/upload.go", "multi, err := request.MultipartReader()\n\tif err != nil {", "multi, err := request.MultipartReader()\n\tif err == nil {\n\t\t_, _ = multi.ReadForm(1 << 20)\n\t}\n\tif err != nil {", "./internal/server", "TestTheServerNeverBuffersAMultipartBody", "buffers a multipart body"),
    ("space_refusal", "internal/server/upload.go", "if err := r.destination.CheckSpace(declared); err != nil {", "if false {", "./internal/server", "TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting", "want 413"),
    ("space_notice", "internal/server/upload.go", "r.lane.publishProgress(noticeEvent(r.sessionID, transfer.NoticeReceiveTooLarge))", "_ = transfer.NoticeReceiveTooLarge", "./internal/server", "TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting", "no notice within 10s"),
    ("length_required", "internal/server/upload.go", 'if request.ContentLength < 0 || (request.ContentLength == 0 && request.Header.Get("Content-Length") == "") {', "if false {", "./internal/server", "TestRejectedRequestsWriteNothingAndLeaveTheSessionWaiting", "want 411"),
    ("claimed_locked", "internal/server/upload.go", "\tif r.claimed.Load() {\n\t\twriteStatus(writer, http.StatusLocked)\n\t\treturn\n\t}\n\tif request.Method == http.MethodGet {\n\t\tr.uploadPage", "\tif false {\n\t\twriteStatus(writer, http.StatusLocked)\n\t\treturn\n\t}\n\tif request.Method == http.MethodGet {\n\t\tr.uploadPage", "./internal/server", "TestASecondRequestWhileAnUploadIsInFlightGets423AndWritesNothing", "during an upload"),
    ("complete_not_demoted", "internal/server/lifecycle.go", "event.Kind == transfer.ServerComplete && r.destination == nil {", "event.Kind == transfer.ServerComplete {", "./internal/server", "TestAFailedResultPageWriteDoesNotDemoteACompleteUpload", "want complete"),
    # --- coordinator lifecycle (internal/transfer) ---
    ("receive_admission_idle", "internal/transfer/coordinator.go", '\tif c.state != stateIdle {\n\t\tc.mu.Unlock()\n\t\treturn nil, NewError(ErrBusy, "a transfer is already in progress")', '\tif false {\n\t\tc.mu.Unlock()\n\t\treturn nil, NewError(ErrBusy, "a transfer is already in progress")', "./internal/transfer", "TestStartReceiveIsAdmittedOnlyFromIdle", "want busy"),
    ("cancel_outcome", "internal/transfer/lifecycle.go", "holdOutcome := announce && live.receive != nil", "holdOutcome := false && announce && live.receive != nil", "./internal/transfer", "TestCancellingAnUploadInFlightPublishesACancelledOutcomeAndHoldsIt", "want started then one error and no reset"),
    ("destination_close", "internal/transfer/coordinator.go", "\t\tcase resourceDestination:\n\t\t\terr = c.closeDestinationBounded(live)\n", "\t\tcase resourceDestination:\n\t\t\terr = nil\n", "./internal/transfer", "TestCancellingAWaitingReceiveWritesNothingAndResets", "the destination was closed"),
    ("outcome_held", "internal/transfer/outcomes.go", "\tif live.receive == nil {\n\t\t// A receive outcome is held", "\tif true {\n\t\t// A receive outcome is held", "./internal/transfer", "TestACompleteUploadPublishesProgressThenAnOutcomeAndHoldsIt", "want it held until the user leaves it"),
    ("show_refuses_without_subfolder", "internal/transfer/receive.go", "if !exists || folder == \"\" {", "if exists && folder == \"\" {", "./internal/transfer", "TestReceivedFolderRefusesWhenThereIsNoSubfolder", "ReceivedFolder before any file was saved"),
    # --- bound commands (app.go) ---
    ("show_in_folder_no_shell", "app.go", 'return "open", []string{dir}, nil', 'return "sh", []string{"-c", "open " + dir}, nil', ".", "TestTheFileManagerCommandIsAnArgumentVectorNeverAShellLine", 'want "open"'),
]

BY_PLATFORM = {
    "darwin": [
        ("no_replace_rename", "internal/sink/rename_darwin.go", "unix.RenameatxNp(dirfd, from, dirfd, to, unix.RENAME_EXCL)", "unix.RenameatxNp(dirfd, from, dirfd, to, 0)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_posix.go", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil {\n\t\treturn nil, err\n\t}", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_posix.go", "unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "unix.O_WRONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
        ("quarantine_marking", "internal/sink/mark_darwin.go", "return unix.Fsetxattr(int(file.Fd()), quarantineAttribute, quarantineValue(now), 0)", "_ = unix.Fsetxattr\n\t_ = quarantineValue\n\treturn nil", "./internal/sink", "TestSavedFilesCarryTheQuarantineAttributeOnMacOS", "want the attribute present on a saved file"),
    ],
    "linux": [
        ("no_replace_rename", "internal/sink/rename_linux.go", "unix.Renameat2(dirfd, from, dirfd, to, unix.RENAME_NOREPLACE)", "unix.Renameat2(dirfd, from, dirfd, to, 0)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_posix.go", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil {\n\t\treturn nil, err\n\t}", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_posix.go", "unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "unix.O_WRONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
    ],
    "windows": [
        ("no_replace_rename", "internal/sink/fs_windows.go", "windows.MoveFileEx(source, target, 0)", "windows.MoveFileEx(source, target, windows.MOVEFILE_REPLACE_EXISTING)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_windows.go", "if err := os.Mkdir(target, 0o755); err != nil {\n\t\treturn nil, err\n\t}", "if err := os.Mkdir(target, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_windows.go", "os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)", "os.O_WRONLY|os.O_CREATE, 0o644)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
        ("mark_of_the_web", "internal/sink/fs_windows.go", "func (d *windowsDir) mark(_ *os.File, name string, _ time.Time) error {\n", "func (d *windowsDir) mark(_ *os.File, name string, _ time.Time) error {\n\tif name != \"\" {\n\t\treturn nil\n\t}\n", "./internal/sink", "TestSavedFilesCarryTheMarkOfTheWebOnWindows", "want it present on a saved file"),
    ],
}

CASES = COMMON + BY_PLATFORM.get(PLATFORM, [])


def run(directory: Path, case_id: str, phase: str, package: str, test: str, assertion: str) -> int:
    command = ["go", "test", "-json", "-count=1", "-timeout=300s", package, "-run", f"^{test}$"]
    result = subprocess.run(command, cwd=directory, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    log = LOGS / f"{case_id}-{phase}.log"
    log.write_text(result.stdout, encoding="utf-8")
    verdict = subprocess.run([
        "go", "run", "./scripts/mutationverdict", f"-mode={'baseline' if phase == 'baseline' else 'mutation'}",
        f"-test={test}", f"-assert={assertion if phase == 'mutant' else ''}",
        f"-status={result.returncode}", f"-log={log}",
    ], cwd=directory, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if verdict.returncode != 0:
        print(f"{case_id}-{phase}: verdict rejected: {verdict.stdout.strip()}")
    return verdict.returncode


def main() -> None:
    LOGS.mkdir(exist_ok=True)
    ids = [case[0] for case in CASES]
    if len(ids) != len(set(ids)):
        raise RuntimeError("duplicate case id in the inventory")
    with tempfile.TemporaryDirectory(prefix="fairdrop-receive-mutations-") as location:
        snapshot = Path(location) / "repo"

        def ignore(directory: str, names):
            relative = Path(directory).relative_to(ROOT)
            excluded = {".git", ".claude", "build", "_bmad-output", "node_modules", "__pycache__", ".DS_Store"}
            if relative == Path("frontend"):
                excluded.update({"src", "browser"})
            return set(names) & excluded

        shutil.copytree(ROOT, snapshot, ignore=ignore)
        results = []
        for case_id, relative, original, mutant, package, test, assertion in CASES:
            path = snapshot / relative
            source = path.read_text(encoding="utf-8")
            if source.count(original) != 1:
                raise RuntimeError(f"{case_id}: expected one exact anchor, found {source.count(original)}")
            baseline = run(snapshot, case_id, "baseline", package, test, assertion)
            path.write_text(source.replace(original, mutant, 1), encoding="utf-8")
            mutated = run(snapshot, case_id, "mutant", package, test, assertion)
            path.write_text(source, encoding="utf-8")
            results.append((case_id, baseline, mutated))
        summary = f"platform={PLATFORM} cases={len(results)}\n" + "\n".join(
            f"{case_id}: baseline={'pass' if baseline == 0 else 'INVALID'} mutant={'named assertion' if mutated == 0 else 'INVALID'}"
            for case_id, baseline, mutated in results
        ) + "\n"
        (LOGS / "summary.txt").write_text(summary, encoding="utf-8")
        print(summary, end="")
        if any(baseline != 0 or mutated != 0 for _, baseline, mutated in results):
            raise SystemExit(1)


if __name__ == "__main__":
    main()
