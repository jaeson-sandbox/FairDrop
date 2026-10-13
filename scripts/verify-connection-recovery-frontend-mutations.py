#!/usr/bin/env python3
"""Canonical frontend recovery mutation inventory, run in isolated copies.

Each case proves a passing named baseline and a failing named assertion, not
an import/compile error. Complete transcripts remain under --log-dir.
"""

import argparse
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
CASES = (
    ("old_code_hidden", "src/transfer/state.ts",
     "state.cancelPending || state.session.sessionId !== action.sessionId) return state",
     "true || state.cancelPending || state.session.sessionId !== action.sessionId) return state",
     "src/transfer/recovery.test.tsx", "keeps the old code hidden through reset"),
    ("stale_list", "src/ui/ConnectionRecoveryPanel.tsx",
     "if (request !== requestRef.current) return", "if (false) return",
     "src/ui/ConnectionRecoveryPanel.test.tsx", "ignores a late list result after the session changes"),
    ("unmount_cancel", "src/transfer/useTransfer.ts",
     "if (recoveryRef.current !== null && recoveryCancelRef.current === null) {",
     "if (false) {",
     "src/transfer/recovery.test.tsx", "cancels an undisplayed replacement when the controller unmounts"),
    ("retired_rejection", "src/transfer/useTransfer.ts",
     "retired = JSON.parse(message)?.retired === true", "retired = false",
     "src/transfer/recovery.test.tsx", "removes the retired code when rejection is serialized"),
)


def run(frontend: Path, test_file: str, test_name: str, log: Path, expect_failure: bool) -> None:
    node = shutil.which("node")
    if node is None:
        raise RuntimeError("node is not on PATH")
    completed = subprocess.run([node, str(frontend / "node_modules/vitest/vitest.mjs"),
                                "run", test_file, "-t", test_name], cwd=frontend,
                               stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               text=True, timeout=120, check=False)
    log.write_text(completed.stdout, encoding="utf-8")
    if expect_failure:
        named_fail = re.search(r"FAIL\s+.*>\s+" + re.escape(test_name), completed.stdout)
        if completed.returncode != 1 or not named_fail or not re.search(r"AssertionError|TestingLibraryElementError", completed.stdout):
            raise RuntimeError(f"{log.name}: mutation lacks its named assertion failure")
        if re.search(r"Failed to import|SyntaxError|TS\d{4}|Error: Transform failed", completed.stdout):
            raise RuntimeError(f"{log.name}: compile/import failure is not a kill")
    elif completed.returncode != 0 or "Test Files  1 passed" not in completed.stdout:
        raise RuntimeError(f"{log.name}: named baseline did not pass")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, relative, before, after, test_file, test_name in CASES:
        with tempfile.TemporaryDirectory(prefix=f"fairdrop-recovery-{label}-") as directory:
            frontend = Path(directory) / "frontend"
            shutil.copytree(ROOT / "frontend", frontend,
                            ignore=shutil.ignore_patterns("node_modules", "dist", "captures"))
            try:
                frontend.joinpath("node_modules").symlink_to(ROOT / "frontend/node_modules", target_is_directory=True)
            except OSError:
                shutil.copytree(ROOT / "frontend/node_modules", frontend / "node_modules")
            run(frontend, test_file, test_name, args.log_dir / f"{label}-baseline.log", False)
            target = frontend / relative
            original = target.read_text(encoding="utf-8").replace("\r\n", "\n")
            if original.count(before) != 1:
                raise RuntimeError(f"{label}: source anchor count = {original.count(before)}")
            target.write_text(original.replace(before, after, 1), encoding="utf-8")
            run(frontend, test_file, test_name, args.log_dir / f"{label}-mutation.log", True)
            print(f"KILLED: {label} by {test_name}", flush=True)


if __name__ == "__main__":
    main()
