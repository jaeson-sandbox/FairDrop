#!/usr/bin/env python3
"""Run receiver protocol mutations in isolated temporary Go modules.

Only the server and transfer packages are copied. The working checkout is
never edited, so this proof can run while other agents inspect its files.
"""
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-receiver-landing-page-logs"
LOGS.mkdir(parents=True, exist_ok=True)
CASES = (
    ("get-claim", "internal/server/handler.go", "if request.Method == http.MethodGet {", "if request.Method == http.MethodPost {", "TestLandingVisitsDoNotClaimOrRead"),
    ("escape", "internal/server/landing.go", '"html/template"', '"text/template"', "TestLandingEscapesMetadataAndCarriesPagePolicies"),
    ("wrong-token-post", "internal/server/handler.go", 'if !tokenMatches(request.PathValue("token"), r.token) {', 'if request.Method == http.MethodGet && !tokenMatches(request.PathValue("token"), r.token) {', "TestRejectedRequestsNeverReachClaimLogic"),
    ("size-wiring", "internal/server/landing.go", "formatLogicalSize(r.item.LogicalSize)", "formatLogicalSize(0)", "TestFileLandingRendersStagedSize"),
    ("form-method", "internal/server/landing.go", '<form method="post" action="">', '<form method="get" action="">', "TestLandingVisitsDoNotClaimOrRead"),
    ("page-csp", "internal/server/landing.go", 'header.Set("Content-Security-Policy",', 'header.Set("X-Removed-Content-Security-Policy",', "TestLandingEscapesMetadataAndCarriesPagePolicies"),
)


def run(workspace: Path, test: str, path: Path):
    completed = subprocess.run(
        ["go", "test", "-count=1", "-v", "-timeout", "60s", "-run", f"^{test}$", "./internal/server"],
        cwd=workspace, universal_newlines=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        timeout=90, check=False,
    )
    path.write_text(completed.stdout, encoding="utf-8")
    return completed.returncode, completed.stdout


def main() -> None:
    killed = 0
    for name, relative, old, new, test in CASES:
        with tempfile.TemporaryDirectory(prefix=f"fairdrop-receiver-{name}-") as directory:
            workspace = Path(directory)
            for file in ("go.mod", "go.sum"):
                shutil.copy2(ROOT / file, workspace / file)
            for package in ("server", "transfer"):
                shutil.copytree(ROOT / "internal" / package, workspace / "internal" / package)
            baseline_code, baseline = run(workspace, test, LOGS / f"{name}-baseline.log")
            if baseline_code or f"--- PASS: {test}" not in baseline:
                raise SystemExit(f"{name}: baseline did not run and pass {test}; see {name}-baseline.log")
            target = workspace / relative
            source = target.read_text(encoding="utf-8")
            if source.count(old) != 1:
                raise SystemExit(f"{name}: mutation anchor matched {source.count(old)} times")
            target.write_text(source.replace(old, new), encoding="utf-8")
            code, output = run(workspace, test, LOGS / f"{name}.log")
            if code == 0 or f"--- FAIL: {test}" not in output:
                raise SystemExit(f"{name}: mutation did not fail the named test; see {name}.log")
            killed += 1
            print(f"KILLED {name}: {test}")
    print(f"{killed}/{len(CASES)} receiver mutations killed in isolated copies")


if __name__ == "__main__":
    main()
