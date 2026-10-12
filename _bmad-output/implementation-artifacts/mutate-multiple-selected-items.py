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
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-collection-cap-10000-logs/go-mutations"

# (id, file, exact original, exact mutant, focused package, focused test, assertion fragment)
#
# Inventory for the 10,000-root lazy-preparation design (2026-10-11). The 2026-10-07
# inventory encoded a 16-root cap and accumulated directory pins; those cases are
# replaced, not deleted: each guarantee that still applies keeps a case below, and each
# obsolete accumulation guarantee has its one-at-a-time equivalent.
CASES = [
    # --- the cap: 10,000 accepted, 10,001 refused, in every layer that enforces it
    ("count_cap", "internal/transfer/types.go", "const MaxCollectionItems = 10000", "const MaxCollectionItems = 10001", "./internal/transfer", "TestCollectionCountRefusalPrecedesFilesystemAndNetwork", "count 10001"),
    ("count_cap_lower", "internal/transfer/types.go", "const MaxCollectionItems = 10000", "const MaxCollectionItems = 9999", "./internal/transfer", "TestCollectionAdmitsTenThousandRootsAndInspectsEachOnce", "10,000 roots refused"),
    ("count_coordinator", "internal/transfer/coordinator.go", "len(paths) > MaxCollectionItems", "len(paths) > MaxCollectionItems+1", "./internal/transfer", "TestCollectionCountRefusalPrecedesFilesystemAndNetwork", "count 10001"),
    ("count_server", "internal/server/lifecycle.go", "len(request.Item.Collection.Members) > transfer.MaxCollectionItems)", "len(request.Item.Collection.Members) > transfer.MaxCollectionItems+1)", "./internal/server", "TestStartAdmitsCollectionsOfTwoThroughTenThousandMembersOnly", "Start with 10001 members"),
    ("count_stream", "internal/stream/payload.go", "len(item.Collection.Members) > transfer.MaxCollectionItems {", "len(item.Collection.Members) > transfer.MaxCollectionItems+1 {", "./internal/stream", "TestCollectionPrepareRefusesCountsOutsideTheContractBeforeTouchingTheSource", 'want "setup_failed"'),
    # --- overlap, now a component-aware index instead of a pairwise scan
    ("overlap", "internal/transfer/coordinator.go", "if !overlaps.add(member.Path) {", "if !overlaps.add(member.Path) && false {", "./internal/transfer", "TestCollectionRejectsDuplicateAndAncestorSelections", "paths [/a/report /a/report]"),
    ("overlap_duplicate", "internal/transfer/coordinator.go", "if _, duplicate := o.selected[key]; duplicate {", "if _, duplicate := o.selected[key]; duplicate && false {", "./internal/transfer", "TestCollectionRejectsDuplicateAndAncestorSelections", "paths [/a/report /a/report]"),
    ("root_overlap", "internal/transfer/coordinator.go", "if _, inside := o.selected[parentKey]; inside {", "if _, inside := o.selected[parentKey]; inside && false {", "./internal/transfer", "TestSelectionOverlapRespectsRootAndComponentBoundaries", "filesystem root did not contain child"),
    ("overlap_contains", "internal/transfer/coordinator.go", "if _, contains := o.ancestors[key]; contains {", "if _, contains := o.ancestors[key]; contains && false {", "./internal/transfer", "TestSelectionOverlapRespectsRootAndComponentBoundaries", "filesystem root did not contain child"),
    ("component_boundary", "internal/transfer/coordinator.go", "ancestors = append(ancestors, parentKey)", "ancestors = append(ancestors, parentKey, key[:len(key)-1])", "./internal/transfer", "TestSelectionOverlapRespectsRootAndComponentBoundaries", "component prefix mistaken for ancestor"),
    ("overflow", "internal/transfer/coordinator.go", "member.LogicalSize > 9007199254740991-total", "member.LogicalSize > 9007199254740991", "./internal/transfer", "TestCollectionCheckedSumAndCallerSliceOwnership", "overflow = setup_failed"),
    ("slice_copy", "internal/transfer/coordinator.go", "paths = slices.Clone(paths)", "paths = paths", "./internal/transfer", "TestCollectionCheckedSumAndCallerSliceOwnership", "caller mutated admitted paths"),
    ("member_copy", "internal/stream/payload.go", "members := slices.Clone(item.Collection.Members)", "members := item.Collection.Members; _ = slices.Clone(members)", "./internal/stream", "TestCollectionPreparationSnapshotsCallerMembersBeforeExternalWork", "caller mutation replaced admitted member"),
    ("handler_copy", "internal/server/handler.go", "r.payloads.Prepare(r.ctx, cloneServerItem(r.item))", "r.payloads.Prepare(r.ctx, r.item)", "./internal/server", "TestCollectionPayloadAdapterCannotMutateServerMetadata", "payload adapter mutated server metadata"),
    # --- Stage reserves one future pin per directory (like a singleton) and inspects each root once
    ("stage_pin_reservation", "internal/source/source.go", "i.withSelectionRetained(ctx, absolutePath, 1, func(selected selection) error {", "i.withSelectionRetained(ctx, absolutePath, 0, func(selected selection) error {", ".", "TestNativeCollectionManyFoldersAtTheSingleFolderBoundaryStreamTogether", "collection download failed"),
    ("stage_single_inspect", "internal/transfer/coordinator.go", "member, inspectErr := c.source.Inspect(setupCtx, path)", "_, _ = c.source.Inspect(setupCtx, path); member, inspectErr := c.source.Inspect(setupCtx, path)", "./internal/transfer", "TestCollectionStageInspectsEachFolderOnceWithoutAccumulatedReservations", "were inspected"),
    ("decorator_single_inspect", "selection_source.go", "return s.SourcePort.Inspect(ctx, canonical)", "_, _ = s.SourcePort.Inspect(ctx, canonical); return s.SourcePort.Inspect(ctx, canonical)", ".", "TestSelectionResolvesAndTraversesEachCollectionRootOnce", "three roots resolved"),
    # --- ordinals: one-based, zero-padded to the width of the count
    ("numbered_names", "internal/stream/payload.go", "width, index+1, name)", "width, index, name)", "./internal/stream", "TestCollectionStreamsAllMembersWithNumberedRoots", "unexpected ZIP entry"),
    ("ordinal_width", "internal/stream/payload.go", "return max(2, len(strconv.Itoa(count)))", "_ = strconv.Itoa(count); return 2", "./internal/stream", "TestCollectionOrdinalsArePaddedToTheWidthOfTheCount", "of 100 is"),
    ("name_fallback", "internal/stream/payload.go", "name := downloadName(member)", "name := sanitizeDownloadName(member.Name)", "./internal/stream", "TestCollectionRootNameFallbackAndPostPrefixLimit", "empty sanitized name did not use basename fallback"),
    ("name_trim", "internal/stream/payload.go", "name = strings.TrimRight(name, nameTrailingCutset)", "name = name", "./internal/stream", "TestCollectionRootNameFallbackAndPostPrefixLimit", "numbered root exceeded safe limit or ended in dot"),
    ("name_bytes", "internal/stream/payload.go", "bounded.Len()+utf8.RuneLen(char) > 255-reserve", "bounded.Len()+utf8.RuneLen(char) > 255", "./internal/stream", "TestCollectionNumberedRootFitsMultibyteFilesystemLimit", "numbered multibyte root exceeded bound"),
    ("name_runes", "internal/stream/payload.go", "count >= maxDownloadNameRunes-reserve ||", "count >= maxDownloadNameRunes ||", "./internal/stream", "TestCollectionWideOrdinalStaysWithinTheRootNameBound", "want exactly 200 starting with the full ordinal"),
    # --- lazy preparation: one member at a time, first before headers, later ones when reached
    ("lazy_release", "internal/stream/archive.go", "a.lazy.release(member)", "_ = member", "./internal/stream", "TestCollectionHoldsAtMostOneMemberAtAnyMoment", "one-at-a-time collection streamed with error"),
    ("lazy_install_guard", "internal/stream/collection.go", "closed, held := l.closed, l.current != nil", "closed, held := l.closed, false", "./internal/stream", "TestLazyCollectionRefusesToHoldASecondMember", "a second held member was installed"),
    ("lazy_install_closed", "internal/stream/collection.go", "closed, held := l.closed, l.current != nil", "closed, held := false, l.current != nil", "./internal/stream", "TestLazyCollectionClosesAMemberThatFinishesPreparingAfterShutdown", "error = nil, want code"),
    ("first_member_before_headers", "internal/stream/payload.go", "if err := lazy.openFirst(ctx); err != nil {", "if err := error(nil); err != nil {", "./internal/stream", "TestCollectionPreparesFirstMemberBeforeHeadersAndLaterMembersOnlyWhenReached", "Prepare touched"),
    ("lazy_later_abort", "internal/stream/archive.go", "member, err := a.lazy.next(ctx, index)\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}", "member, err := a.lazy.next(ctx, index)\n\t\tif err != nil {\n\t\t\tcontinue\n\t\t}", "./internal/stream", "TestCollectionLaterMemberChangedReplacedOrRemovedAbortsUnfinalized", "want code"),
    ("halt_finalization", "internal/stream/archive.go", "halting.halt()", "_ = halting", "./internal/stream", "TestCollectionLaterMemberChangedReplacedOrRemovedAbortsUnfinalized", "failed collection has a valid ZIP central directory"),
    ("short_file", "internal/stream/archive.go", "if bounded.N != 0 {", "if false {", "./internal/stream", "TestCollectionPreparedFileLengthIsEnforced", "short source yielded successful collection"),
    ("shutdown_releases_current", "internal/stream/archive.go", "err = a.lazy.shutdown()", "err = nil", "./internal/stream", "TestCollectionCloseBeforeWriteReleasesEveryMember", "after Close held="),
    ("directory_member_close", "internal/stream/collection.go", "return m.dir.Close()", "return nil", "./internal/stream", "TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit", "directory 0 Close called"),
    ("cleanup", "internal/stream/payload.go", "if closeErr := collection.Close(); closeErr != nil {", "if closeErr := error(nil); closeErr != nil {", "./internal/stream", "TestCollectionPrepareCancelledAfterTheFirstMemberOpensReleasesIt", "a cancelled Prepare left the first member"),
    ("cancel_cleanup", "internal/stream/collection.go", "l.discard(member)\n\t\treturn nil, err", "_ = member\n\t\treturn nil, err", "./internal/stream", "TestCollectionCancelledJustAfterALaterMemberOpensClosesIt", "cancelled just after a later member opened"),
    ("close_mid_stream", "internal/stream/collection.go", "\tcurrent := l.current\n\tl.current = nil\n\tprior := l.closeErr", "\tvar current *openMember\n\tl.current = nil\n\tprior := l.closeErr", "./internal/stream", "TestCollectionCloseMidStreamReleasesTheCurrentMemberOnly", "Close mid-stream"),
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
    LOGS.mkdir(parents=True, exist_ok=True)
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
            source = path.read_text(encoding="utf-8").replace("\r\n", "\n")
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
        killed = sum(1 for _, baseline, mutated in results if baseline == 0 and mutated == 0)
        print(f"{killed}/{len(results)} collection Go mutations killed")
        if any(baseline != 0 or mutated != 0 for _, baseline, mutated in results):
            raise SystemExit(1)


if __name__ == "__main__":
    main()
