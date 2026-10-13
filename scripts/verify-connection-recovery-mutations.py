#!/usr/bin/env python3
"""Canonical scoped recovery mutations. Run only in an isolated source copy.

Every case has one baseline, one exact source edit, and a named test assertion.
The full go test JSON transcript stays in --log-dir; compilation failures never
count as kills because scripts/mutationverdict checks the structured result.
"""

import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
CASES = (
    ("stale_choice", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshConnectionPreflightRefusesStaleChoiceWithoutRetirement",
     "preflight failure retired the waiting session",
     'if !found {\n\t\t\treturn ConnectionRefresh{}, NewError(ErrNetworkUnavailable, "the selected local network is no longer available")',
     'if false && !found {\n\t\t\treturn ConnectionRefresh{}, NewError(ErrNetworkUnavailable, "the selected local network is no longer available")'),
    ("fresh_identity", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshConnectionRetiresOldSendBeforePublishingFreshIdentity",
     "identity did not rotate",
     'newID, token, err := c.newIdentity()\n\tif err != nil {',
     'newID, token, err := c.newIdentity()\n\tnewID = old.id\n\tif err != nil {'),
    ("reinspect_collection", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshCollectionInspectionFailureRetiresOldCode",
     "failed reinspection",
     'item, paths, err := c.reinspectForRecovery(setupCtx, old.item)',
     'item, paths, err := old.item, 1, error(nil)'),
    ("destination_owner", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshReceiveRetainsOneDestinationAndClosesOnExit",
     "destination closes =",
     '\t\tlive.hold(resourceDestination)\n',
     '\t\t// injected mutation: destination omitted from replacement ledger\n'),
    ("explicit_endpoint", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshConnectionForwardsDistinctExplicitEndpointIntoURLAndBeacon",
     "selection=",
     'cloneStagedItem(item)}, downloadPathPrefix, networkID)',
     'cloneStagedItem(item)}, downloadPathPrefix, "")'),
    ("cleanup_quiescence", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRefreshConnectionCleanupFailureBlocksReplacement", "cleanup =",
     'cleanupErr := c.unwind(old)\n\tif cleanupErr != nil {',
     'cleanupErr := c.unwind(old)\n\tif false && cleanupErr != nil {'),
    ("claim_fence", "internal/transfer/recovery.go", "./internal/transfer",
     "TestRecoveryReservationIsClaimFenceBeforeAdapterStop", "claim fence was not reserved",
     'c.state = stateStaging // Claiming is excluded before any external work.',
     'c.state = stateStaged // injected: claiming is not excluded'),
    ("identity_cancel", "internal/transfer/recovery.go", "./internal/transfer",
     "TestCancelDuringRecoveryIdentityGenerationPreventsReplacement", "refresh =",
     'if c.closing || old.cancelled || ctx.Err() != nil {\n\t\tc.session = nil\n\t\tc.state = stateIdle\n\t\tc.mu.Unlock()\n\t\tstop()',
     'if false {\n\t\tc.session = nil\n\t\tc.state = stateIdle\n\t\tc.mu.Unlock()\n\t\tstop()'),
    ("retired_marker", "main.go", ".",
     "TestRecoveryFailureWailsFormatterMarksRetirementWithoutLeakingCause", "Wails error =",
     '}{public.Code, public.Message, true})',
     '}{public.Code, public.Message, false})'),
)


def run_test(package: str, name: str, log: Path, mode: str, evidence: str = "") -> None:
    command = ["go", "test", "-count=1", "-json", "-timeout", "60s", "-run", f"^{name}$", package]
    with log.open("w", encoding="utf-8") as output:
        result = subprocess.run(command, cwd=ROOT, stdout=output, stderr=subprocess.STDOUT, check=False)
    verdict = ["go", "run", "./scripts/mutationverdict", "-mode", mode,
               "-test", name, "-status", str(result.returncode), "-log", str(log)]
    if evidence:
        verdict.extend(["-assert", evidence])
    subprocess.run(verdict, cwd=ROOT, check=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get("FAIRDROP_MUTATION_ISOLATED") != "1":
        parser.error("set FAIRDROP_MUTATION_ISOLATED=1 in an isolated source copy")
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, file_name, package, test, assertion, before, after in CASES:
        path = ROOT / file_name
        original = path.read_text(encoding="utf-8")
        if original.count(before) != 1:
            raise RuntimeError(f"{label}: expected exactly one source anchor, found {original.count(before)}")
        run_test(package, test, args.log_dir / f"{label}-baseline.json", "baseline")
        try:
            path.write_text(original.replace(before, after, 1), encoding="utf-8")
            run_test(package, test, args.log_dir / f"{label}-mutation.json", "mutation", assertion)
        finally:
            path.write_text(original, encoding="utf-8")
        print(f"KILLED: {label} by {test} ({assertion})", flush=True)


if __name__ == "__main__":
    main()
