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
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-multiple-selected-items-frontend-logs"
NODE = shutil.which("node")
CASES = (
    ("metadata-count", "src/transfer/validation.ts", "(itemCount as number) > 16", "(itemCount as number) > 17", "src/transfer/validation.test.ts", "accepts a coherent collection"),
    ("append-limit", "src/transfer/useTransfer.ts", "current.length + added.length > 16", "current.length + added.length > 17", "src/transfer/useTransfer.test.tsx", "rejects a seventeenth append"),
    ("batch-command", "src/transfer/useTransfer.ts", "paths.length === 1\n            ? StageTransfer", "false\n            ? StageTransfer", "src/transfer/useTransfer.test.tsx", "restores the editable batch"),
    ("draft-recovery", "src/transfer/useTransfer.ts", "const error = parseCommandError(rejection)\n            const recovery = draftRecoveryRef.current\n            draftRecoveryRef.current = null\n            if (recovery !== null) showDraft(recovery, error)", "const error = parseCommandError(rejection)\n            const recovery = draftRecoveryRef.current\n            draftRecoveryRef.current = null\n            if (false) showDraft(recovery, error)", "src/transfer/useTransfer.test.tsx", "restores the editable batch"),
    ("cancel-forgets", "src/transfer/useTransfer.ts", "draftRecoveryRef.current = null\n        rememberedPathsRef.current = null\n        draftChooserRef.current = null", "draftRecoveryRef.current = null\n        void rememberedPathsRef.current\n        draftChooserRef.current = null", "src/transfer/useTransfer.test.tsx", "forgets retryable remembered paths"),
    ("display-name", "src/transfer/useTransfer.ts", "paths.map(sanitizeDraftBasename)", "paths.map(path => path)", "src/transfer/useTransfer.test.tsx", "sanitizes draft display names"),
    ("draft-refusal", "src/transfer/useTransfer.ts", "if (current !== null) showDraft(current, publicError('invalid_selection'))", "if (false) showDraft(current, publicError('invalid_selection'))", "src/transfer/useTransfer.test.tsx", "keeps draft rows and shows a safe error"),
    ("visible-order", "src/ui/CollectionDraftView.tsx", 'className="fd-draft__ordinal" aria-hidden="true">{index + 1}.</span>', 'className="fd-draft__ordinal" aria-hidden="true">{null}</span>', "src/ui/CollectionDraftView.test.tsx", "shows explicit one-based order"),
    ("remove-order", "src/ui/CollectionDraftView.tsx", 'aria-label={`${copy.selection.remove} ${index + 1}. ${name}`}', 'aria-label={`${copy.selection.remove} ${name}`}', "src/ui/CollectionDraftView.test.tsx", "shows explicit one-based order"),
    ("error-focus", "src/ui/CollectionDraftView.tsx", "aria-describedby={error ? 'fd-draft-error' : undefined}", "aria-describedby={undefined}", "src/ui/CollectionDraftView.test.tsx", "describes a newly focused draft heading"),
)


def run(workspace: Path, test_file: str, test_name: str, log: Path) -> tuple[int, str]:
    if NODE is None:
        raise SystemExit("node executable not found on PATH")
    executable = workspace / "frontend/node_modules/vitest/vitest.mjs"
    completed = subprocess.run(
        [str(Path(NODE).resolve()), str(executable), "run", test_file, "-t", test_name],
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
            if code == 0 or f"FAIL  {test_file} >" not in output or test_name not in output:
                raise SystemExit(f"{name}: mutation did not fail the named assertion; see {name}-mutation.log")
            if "Failed to resolve import" in output or "SyntaxError:" in output or "Transform failed" in output:
                raise SystemExit(f"{name}: compile/import failure is not an assertion kill")
            print(f"KILLED {name}: {test_name}")
    print(f"{len(CASES)}/{len(CASES)} collection frontend mutations killed")


if __name__ == "__main__":
    main()
