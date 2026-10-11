#!/usr/bin/env python3
"""Run the canonical collection frontend mutations in isolated copies.

Each case proves a passing baseline, validates a unique source anchor, then
requires a named assertion failure. Logs keep complete stdout/stderr.
"""
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-collection-cap-10000-logs/frontend-mutations"
NODE = shutil.which("node")
CASES = (
    ("metadata-count", "src/transfer/validation.ts", "(itemCount as number) > MAX_COLLECTION_ITEMS", "(itemCount as number) > MAX_COLLECTION_ITEMS + 1", "src/transfer/validation.test.ts", "accepts a coherent collection"),
    ("metadata-count-lower", "src/transfer/validation.ts", "(itemCount as number) > MAX_COLLECTION_ITEMS", "(itemCount as number) >= MAX_COLLECTION_ITEMS", "src/transfer/validation.test.ts", "accepts a coherent collection"),
    ("append-limit", "src/transfer/useTransfer.ts", "current.length + added.length > MAX_COLLECTION_ITEMS", "current.length + added.length > MAX_COLLECTION_ITEMS + 1", "src/transfer/useTransfer.test.tsx", "rejects a 10,001st append"),
    ("append-limit-lower", "src/transfer/useTransfer.ts", "current.length + added.length > MAX_COLLECTION_ITEMS", "current.length + added.length >= MAX_COLLECTION_ITEMS", "src/transfer/useTransfer.test.tsx", "accepts appends up to exactly 10,000 rows"),
    ("open-limit", "src/transfer/useTransfer.ts", "const invalid = proposed.length > MAX_COLLECTION_ITEMS ||", "const invalid = proposed.length > MAX_COLLECTION_ITEMS + 1 ||", "src/transfer/useTransfer.test.tsx", "refuses an initial 10,001-path drop"),
    ("stage-limit", "src/transfer/useTransfer.ts", "selectedPaths.length > MAX_COLLECTION_ITEMS ||", "selectedPaths.length >= MAX_COLLECTION_ITEMS ||", "src/transfer/useTransfer.test.tsx", "stages a draft of exactly 10,000 paths as one batch"),
    ("count-copy", "src/ui/copy.ts", "} of 10,000 items`", "} of 16 items`", "src/ui/CollectionDraftView.test.tsx", "shows the count out of 10,000"),
    ("count-grouping", "src/ui/copy.ts", "${count.toLocaleString('en-US')} of 10,000 items", "${count} of 10,000 items", "src/ui/CollectionDraftView.test.tsx", "shows the count out of 10,000"),
    ("list-scroll", "src/style.css", "overflow-y: auto; overscroll-behavior: contain;", "overflow-y: visible; overscroll-behavior: contain;", "src/ui/draftListStyles.test.ts", "scrolls within the window instead of stretching the page"),
    ("focus-room", "src/style.css", "padding-inline-end: .75rem; padding-block: .25rem; margin-block: 1rem;", "margin-block: 1rem;", "src/ui/draftListStyles.test.ts", "leaves room inside the scrollport"),
    ("batch-command", "src/transfer/useTransfer.ts", "paths.length === 1\n            ? StageTransfer", "false\n            ? StageTransfer", "src/transfer/useTransfer.test.tsx", "restores the editable batch"),
    ("draft-recovery", "src/transfer/useTransfer.ts", "const error = parseCommandError(rejection)\n            const recovery = draftRecoveryRef.current\n            draftRecoveryRef.current = null\n            if (recovery !== null) showDraft(recovery, error)", "const error = parseCommandError(rejection)\n            const recovery = draftRecoveryRef.current\n            draftRecoveryRef.current = null\n            if (false) showDraft(recovery, error)", "src/transfer/useTransfer.test.tsx", "restores the editable batch"),
    ("cancel-forgets", "src/transfer/useTransfer.ts", "draftRecoveryRef.current = null\n        rememberedPathsRef.current = null\n        draftChooserRef.current = null", "draftRecoveryRef.current = null\n        void rememberedPathsRef.current\n        draftChooserRef.current = null", "src/transfer/useTransfer.test.tsx", "forgets retryable remembered paths"),
    ("display-name", "src/transfer/useTransfer.ts", "name: sanitizeDraftBasename(path)}", "name: path}", "src/transfer/useTransfer.test.tsx", "sanitizes draft display names"),
    ("draft-refusal", "src/transfer/useTransfer.ts", "if (current !== null) showDraftEntries(current, publicError('invalid_selection'))", "if (false) showDraftEntries(current, publicError('invalid_selection'))", "src/transfer/useTransfer.test.tsx", "keeps draft rows and shows a safe error"),
    ("visible-order", "src/ui/CollectionDraftView.tsx", 'className="fd-draft__ordinal" aria-hidden="true">{index + 1}.</span>', 'className="fd-draft__ordinal" aria-hidden="true">{null}</span>', "src/ui/CollectionDraftView.test.tsx", "shows explicit one-based order"),
    ("remove-order", "src/ui/CollectionDraftView.tsx", 'aria-label={`${copy.selection.remove} ${index + 1}. ${name}`}', 'aria-label={`${copy.selection.remove} ${name}`}', "src/ui/CollectionDraftView.test.tsx", "shows explicit one-based order"),
    # --- 1.5.1 review patches: stable row identity, no re-sanitizing, ordinal column width
    ("row-key", "src/ui/CollectionDraftView.tsx", "<DraftRow key={keys[index]} rowKey", "<DraftRow key={`${index}-${name}`} rowKey", "src/ui/CollectionDraftView.test.tsx", "keeps the remaining rows mounted when the first row is removed"),
    ("row-memo", "src/ui/CollectionDraftView.tsx", "const DraftRow = memo(function DraftRow({rowKey, index, name, onRemove, register}: RowProps) {", "const DraftRow = (function DraftRow({rowKey, index, name, onRemove, register}: RowProps) {", "src/ui/CollectionDraftView.test.tsx", "re-renders only the new row when a row is appended"),
    ("draft-id-issue", "src/transfer/useTransfer.ts", "({id: draftNextIdRef.current++, path,", "({id: 0, path,", "src/transfer/useTransfer.test.tsx", "gives every draft row a stable id"),
    ("draft-append-resanitize", "src/transfer/useTransfer.ts", "showDraftEntries([...current, ...newDraftEntries(added)])", "showDraft([...current.map(entry => entry.path), ...added])", "src/transfer/useTransfer.test.tsx", "sanitizes each path once"),
    ("draft-remove-resanitize", "src/transfer/useTransfer.ts", "showDraftEntries(current.filter((_, position) => position !== index))", "showDraft(current.filter((_, position) => position !== index).map(entry => entry.path))", "src/transfer/useTransfer.test.tsx", "sanitizes each path once"),
    ("draft-refusal-resanitize", "src/transfer/useTransfer.ts", "if (current !== null) showDraftEntries(current, publicError('invalid_selection'))", "if (current !== null) showDraft(current.map(entry => entry.path), publicError('invalid_selection'))", "src/transfer/useTransfer.test.tsx", "sanitizes each path once"),
    ("ordinal-digits", "src/ui/CollectionDraftView.tsx", "Math.max(2, String(names.length).length)", "Math.max(2, 3)", "src/ui/CollectionDraftView.test.tsx", "sizes the ordinal column to the digit width"),
    ("ordinal-column", "src/style.css", "flex: 0 0 calc((var(--fd-ordinal-digits, 2) + 1) * 1ch);", "flex: 0 0 3ch;", "src/ui/draftListStyles.test.ts", "sizes the ordinal column to the widest ordinal"),
    ("ordinal-column-rendered", "src/style.css", "flex: 0 0 calc((var(--fd-ordinal-digits, 2) + 1) * 1ch);", "flex: 0 0 3ch;", "browser/collection-draft.test.tsx", "never lets a wide ordinal touch the name at 10,000 rows"),
    ("error-focus", "src/ui/CollectionDraftView.tsx", "aria-describedby={error ? 'fd-draft-error' : undefined}", "aria-describedby={undefined}", "src/ui/CollectionDraftView.test.tsx", "describes a newly focused draft heading"),
)


def run(workspace: Path, test_file: str, test_name: str, log: Path) -> tuple[int, str]:
    if NODE is None:
        raise SystemExit("node executable not found on PATH")
    executable = workspace / "frontend/node_modules/vitest/vitest.mjs"
    # Rendered-Chromium cases live under browser/ and need the browser config.
    config = ["--config", "vitest.browser.config.ts"] if test_file.startswith("browser/") else []
    completed = subprocess.run(
        [str(Path(NODE).resolve()), str(executable), "run", *config, test_file, "-t", test_name],
        cwd=workspace / "frontend", text=True, stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT, timeout=120, check=False,
    )
    log.write_text(completed.stdout, encoding="utf-8")
    return completed.returncode, completed.stdout


def main() -> None:
    LOGS.mkdir(parents=True, exist_ok=True)
    for name, relative, old, new, test_file, test_name in CASES:
        with tempfile.TemporaryDirectory(prefix=f"fairdrop-frontend-{name}-") as directory:
            workspace = Path(directory)
            shutil.copytree(ROOT / "frontend", workspace / "frontend",
                            ignore=shutil.ignore_patterns("node_modules", "dist", "captures"))
            dependencies = workspace / "frontend/node_modules"
            try:
                dependencies.symlink_to(ROOT / "frontend/node_modules", target_is_directory=True)
            except OSError:
                # Windows may refuse directory symlinks without Developer Mode.
                # A copy keeps the isolated test runnable without privileges.
                shutil.copytree(ROOT / "frontend/node_modules", dependencies)
            baseline_code, baseline = run(workspace, test_file, test_name, LOGS / f"{name}-baseline.log")
            if baseline_code != 0 or "Test Files  1 passed" not in baseline:
                raise SystemExit(f"{name}: baseline failed; see {name}-baseline.log")
            target = workspace / "frontend" / relative
            source = target.read_text(encoding="utf-8").replace("\r\n", "\n")
            if source.count(old) != 1:
                raise SystemExit(f"{name}: anchor matched {source.count(old)} times")
            target.write_text(source.replace(old, new), encoding="utf-8")
            code, output = run(workspace, test_file, test_name, LOGS / f"{name}-mutation.log")
            if code == 0 or f"{test_file} >" not in output or "FAIL" not in output or test_name not in output:
                raise SystemExit(f"{name}: mutation did not fail the named assertion; see {name}-mutation.log")
            if "Failed to resolve import" in output or "SyntaxError:" in output or "Transform failed" in output:
                raise SystemExit(f"{name}: compile/import failure is not an assertion kill")
            print(f"KILLED {name}: {test_name}")
    print(f"{len(CASES)}/{len(CASES)} collection frontend mutations killed")


if __name__ == "__main__":
    main()
