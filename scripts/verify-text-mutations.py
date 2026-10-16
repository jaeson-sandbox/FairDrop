#!/usr/bin/env python3
"""Named, scoped text mutations. Run only in an isolated source copy."""

import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
CASES = (
    ("raw-before-allocation", "internal/transfer/text.go", "./internal/transfer",
     "TestRawTextLimitPrecedesValidationAndAllocation", "raw text guard missing",
     "len(raw) > MaxRawTextBytes || !utf8.ValidString(raw)", "!utf8.ValidString(raw)"),
    ("normalized-cap", "internal/transfer/text.go", "./internal/transfer",
     "TestNormalizeTextExactAndBoundary", "NormalizeText",
     "MaxTextBytes    = 65536", "MaxTextBytes    = 65537"),
    ("no-source-access", "internal/transfer/coordinator_text.go", "./internal/transfer",
     "TestStageTextUsesNoSourceAndOffersLiteralRoute", "text touched source",
     "setupCtx, stopSetup := context.WithCancel(live.ctx)",
     "setupCtx, stopSetup := context.WithCancel(live.ctx)\n\t_, _ = c.source.Inspect(setupCtx, value)"),
    ("content-before-claim", "internal/server/text.go", "./internal/server",
     "TestTextPreviewAndOneShotRevealExactDOM", "preview disclosed or claimed",
     "if request.Method == http.MethodGet {", "if false && request.Method == http.MethodGet {"),
    ("escaped-get-disclosure", "internal/server/text.go", "./internal/server",
     "TestTextPreviewAndOneShotRevealExactDOM", "preview disclosed or claimed",
     "writeStatus(writer, http.StatusInternalServerError)\n\t\t\treturn\n\t\t}",
     "writeStatus(writer, http.StatusInternalServerError)\n\t\t\treturn\n\t\t}\n\t\tbody.WriteString(template.HTMLEscapeString(*r.text))"),
    ("method-guard", "internal/server/text.go", "./internal/server",
     "TestTextWrongMethodsAndRoutesDoNotReserveClaim", "refusal status=",
     "if request.Method != http.MethodGet && request.Method != http.MethodPost || !tokenMatches",
     "if false && request.Method != http.MethodGet && request.Method != http.MethodPost || !tokenMatches"),
    ("escaping", "internal/server/text.go", "./internal/server",
     "TestTextPreviewAndOneShotRevealExactDOM", "text became active markup",
     "text response could not be rendered\")))\n\t\treturn\n\t}",
     "text response could not be rendered\")))\n\t\treturn\n\t}\n\tunsafe := bytes.ReplaceAll(body.Bytes(), []byte(\"&lt;\"), []byte(\"<\"))\n\tunsafe = bytes.ReplaceAll(unsafe, []byte(\"&gt;\"), []byte(\">\"))\n\tbody.Reset()\n\tbody.Write(unsafe)"),
    ("fresh-recovery-identity", "internal/transfer/recovery.go", "./internal/transfer",
     "TestTextWaitingRefreshKeepsValueWithoutSourceAndRotatesIdentity", "identity was not rotated",
     "newID, token, err := c.newIdentity()", "newID, token, err := c.newIdentity()\n\tnewID = old.id"),
    ("retired-token", "internal/transfer/recovery.go", "./internal/server",
     "TestTextRefreshRetiresActualHTTPRoute", "retired token authorized at new endpoint",
     "newID, token, err := c.newIdentity()", "newID, token, err := c.newIdentity()\n\ttoken = old.token"),
    ("disconnect-terminal", "internal/server/text.go", "./internal/server",
     "TestTextRequestDisconnectAfterAuthorizationSettlesFailure", "no event",
     "text receiver disconnected before reveal\"))\n\tr.finish(&event)",
     "text receiver disconnected before reveal\"))\n\t_ = event // injected: disconnect terminal suppressed"),
)


def test(package, name, log, mode, assertion=""):
    command = ["go", "test", "-count=1", "-json", "-timeout", "60s", "-run", f"^{name}$", package]
    with log.open("w", encoding="utf-8") as stream:
        result = subprocess.run(command, cwd=ROOT, stdout=stream, stderr=subprocess.STDOUT, check=False)
    verdict = ["go", "run", "./scripts/mutationverdict", "-mode", mode,
               "-test", name, "-status", str(result.returncode), "-log", str(log)]
    if assertion:
        verdict.extend(["-assert", assertion])
    subprocess.run(verdict, cwd=ROOT, check=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("FAIRDROP_MUTATION_ISOLATED") != "1":
        parser.error("set FAIRDROP_MUTATION_ISOLATED=1 in an isolated source copy")
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, filename, package, name, assertion, before, after in CASES:
        source = ROOT / filename
        original = source.read_text(encoding="utf-8")
        if original.count(before) != 1:
            raise RuntimeError(f"{label}: source anchor count {original.count(before)}")
        test(package, name, args.log_dir / f"{label}-baseline.json", "baseline")
        try:
            source.write_text(original.replace(before, after, 1), encoding="utf-8")
            test(package, name, args.log_dir / f"{label}-mutation.json", "mutation", assertion)
        finally:
            source.write_text(original, encoding="utf-8")
        print(f"KILLED: {label} by {name} ({assertion})", flush=True)


if __name__ == "__main__":
    main()
