"""Portable, scoped collection mutation inventory. Runs in an isolated copy.

Run from the repository root with `python3 _bmad-output/implementation-artifacts/mutate-multiple-selected-items.py`.
Each case derives from CASES below; the same focused assertion runs on baseline
and mutant, with separate full logs beside the implementation evidence.
"""

from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-multiple-selected-items-mutation-logs"

# (id, file, exact original, exact mutant, focused package, focused test, assertion fragment)
CASES = [
    ("count", "internal/transfer/coordinator.go", "len(paths) > 16", "len(paths) > 17", "./internal/transfer", "TestCollectionCountRefusalPrecedesFilesystemAndNetwork", "count 17 reached external work"),
    ("overlap", "internal/transfer/coordinator.go", "if selectionsOverlap(previous.Path, member.Path) {", "if false && selectionsOverlap(previous.Path, member.Path) {", "./internal/transfer", "TestCollectionRejectsDuplicateAndAncestorSelections", "paths [/a/report /a/report]"),
    ("root_overlap", "internal/transfer/coordinator.go", "if !strings.HasSuffix(parent, separator) {", "if true {", "./internal/transfer", "TestSelectionOverlapRespectsRootAndComponentBoundaries", "filesystem root did not contain child"),
    ("overflow", "internal/transfer/coordinator.go", "member.LogicalSize > 9007199254740991-total", "member.LogicalSize > 9007199254740991", "./internal/transfer", "TestCollectionCheckedSumAndCallerSliceOwnership", "overflow = setup_failed"),
    ("slice_copy", "internal/transfer/coordinator.go", "paths = slices.Clone(paths)", "paths = paths", "./internal/transfer", "TestCollectionCheckedSumAndCallerSliceOwnership", "caller mutated admitted paths"),
    ("member_copy", "internal/stream/payload.go", "members := slices.Clone(item.Collection.Members)", "members := item.Collection.Members; _ = slices.Clone(members)", "./internal/stream", "TestCollectionPreparationSnapshotsCallerMembersBeforeExternalWork", "caller mutation replaced admitted member"),
    ("handler_copy", "internal/server/handler.go", "r.payloads.Prepare(r.ctx, cloneServerItem(r.item))", "r.payloads.Prepare(r.ctx, r.item)", "./internal/server", "TestCollectionPayloadAdapterCannotMutateServerMetadata", "payload adapter mutated server metadata"),
    ("numbered_names", "internal/stream/payload.go", "index+1, name", "index, name", "./internal/stream", "TestCollectionStreamsAllMembersWithNumberedRoots", "unexpected ZIP entry"),
    ("name_fallback", "internal/stream/payload.go", "name := downloadName(member)", "name := sanitizeDownloadName(member.Name)", "./internal/stream", "TestCollectionRootNameFallbackAndPostPrefixLimit", "empty sanitized name did not use basename fallback"),
    ("name_trim", "internal/stream/payload.go", "name = strings.TrimRight(name, nameTrailingCutset)", "name = name", "./internal/stream", "TestCollectionRootNameFallbackAndPostPrefixLimit", "numbered root exceeded safe limit or ended in dot"),
    ("name_bytes", "internal/stream/payload.go", "bounded.Len()+utf8.RuneLen(char) > 255-3", "bounded.Len()+utf8.RuneLen(char) > 255", "./internal/stream", "TestCollectionNumberedRootFitsMultibyteFilesystemLimit", "numbered multibyte root exceeded bound"),
    ("shared_budget", "internal/source/source.go", "absolutePath, 1+otherPins,", "absolutePath, 1,", "./internal/source", "TestCollectionPinsShareRetainedDirectoryBudget", "single root refused before shared budget boundary"),
    ("decorator_single_inspect", "selection_source.go", "return budgeted.InspectWithRetained(ctx, canonical, otherPins)", "_, _ = s.SourcePort.Inspect(ctx, canonical); return budgeted.InspectWithRetained(ctx, canonical, otherPins)", ".", "TestBudgetedSelectionResolvesOnceAndTraversesOnce", "budgeted selection traversed plain="),
    ("coordinator_pin_reservation", "internal/transfer/coordinator.go", "reserved--", "reserved = 0", ".", "TestNativeCollectionSharedDirectoryBoundaryAndHTTP", "unreservable third directory depth admitted"),
    ("stream_pin_reservation", "internal/stream/payload.go", "member.Path, dirs-1)", "member.Path, 0)", "./internal/stream", "TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit", "two simultaneous directory pins reserved"),
    ("directory_member_close", "internal/stream/archive.go", "if closeErr := a.members[index].payload.Close(); closeErr != nil {", "if closeErr := error(nil); closeErr != nil {", "./internal/stream", "TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit", "directory 0 Close called"),
    ("short_file", "internal/stream/archive.go", "if bounded.N != 0 {", "if false {", "./internal/stream", "TestCollectionPreparedFileLengthIsEnforced", "short source yielded successful collection"),
    ("halt_finalization", "internal/stream/archive.go", "halting.halt()", "_ = halting", "./internal/stream", "TestCollectionLaterDirectoryReplacementAbortsWithoutCentralDirectory", "failed collection has a valid ZIP central directory"),
    ("cleanup", "internal/stream/payload.go", "if closeErr := collection.Close(); closeErr != nil {", "if closeErr := error(nil); closeErr != nil {", "./internal/stream", "TestCollectionLaterFilePrepareFailureClosesEarlierDescriptor", "earlier descriptor close count"),
    ("cancel_cleanup", "internal/stream/payload.go", "return cleanup(wrapUncodedSourceError(err))", "return nil, wrapUncodedSourceError(err)", "./internal/stream", "TestCollectionCancellationBetweenMembersClosesEarlierDescriptor", "descriptor closes; want 2 and 1"),
]


def run(directory: Path, case_id: str, phase: str, package: str, test: str, assertion: str) -> int:
    command = ["go", "test", "-json", "-count=1", package, "-run", f"^{test}$"]
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
    with tempfile.TemporaryDirectory(prefix="fairdrop-collection-mutations-") as location:
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
        summary = "\n".join(f"{case_id}: baseline={'pass' if baseline == 0 else 'INVALID'} mutant={'named assertion' if mutated == 0 else 'INVALID'}" for case_id, baseline, mutated in results) + "\n"
        (LOGS / "summary.txt").write_text(summary, encoding="utf-8")
        print(summary, end="")
        if any(baseline != 0 or mutated != 0 for _, baseline, mutated in results):
            raise SystemExit(1)


if __name__ == "__main__":
    main()
