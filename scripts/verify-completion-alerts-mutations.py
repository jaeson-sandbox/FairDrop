#!/usr/bin/env python3
"""Canonical completion-alert mutation inventory, isolated from the checkout."""

import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
CASES = (
    ("startup-wiring", "app.go", "TestAppNativeCompletionStartupAndEnableCallsites",
     "\tnativeCompletionStart()\n\ta.mu.Lock()", "\ta.mu.Lock()",
     "App.startup no longer calls nativeCompletionStart"),
    ("enable-wiring", "app.go", "TestAppNativeCompletionStartupAndEnableCallsites",
     "\tif enabled {\n\t\tnativeCompletionEnable()\n\t}", "\tif enabled {}",
     "App.SetCompletionAlerts no longer calls nativeCompletionEnable"),
    ("adapter-composition", "app.go", "TestNewAppUsesNativeCompletionAdapters",
     "newCompletionAlerts(nativeCompletionCue, nativeCompletionInvalidate, nativeCompletionSnapshot)",
     "newCompletionAlerts(func(alertRequest) {}, nativeCompletionInvalidate, nativeCompletionSnapshot)",
     "NewApp did not wire the native cue adapter"),
    ("success-gate", "app.go", "TestRealFileHTTPCompletionRequestsOneBackgroundCue",
     "event.Kind == transfer.TransferComplete && a.alerts != nil",
     "event.Kind == transfer.TransferError && a.alerts != nil",
     "real HTTP completion did not reach the native cue adapter"),
    ("opt-in", "completion_alerts.go", "TestCompletionAlertsRequireOptInAndOneBackgroundSuccess",
     "a.closed || !a.enabled || session ==", "a.closed || false || session ==",
     "disabled completion entered alert dispatcher"),
    ("background-at-event", "completion_alerts.go", "TestCompletionAlertsRequireOptInAndOneBackgroundSuccess",
     "if !focus.background {", "if false {", "foreground completion requested a cue"),
    ("stale-work", "completion_alerts.go", "TestCompletionAlertsDiscardQueuedWorkAndDoNotBlockPublisher",
     "case <-a.pending:\n\t\tdefault:", "case <-make(chan alertRequest):\n\t\tdefault:",
     "disabled alert left stale work queued"),
    ("latest-slot", "completion_alerts.go", "TestRapidCompletionsReplacePendingWithoutBlocking",
     "\tselect {\n\tcase <-a.pending:\n\tdefault:\n\t}\n\ta.pending <- request",
     "\tselect {\n\tcase <-a.pending:\n\t\treturn false\n\tdefault:\n\t}\n\ta.pending <- request",
     "newest pending success was swallowed"),
    ("mac-guard", "completion_alerts_darwin.go", "TestCompletionAlertPlatformMechanisms",
     "dispatch_async(dispatch_get_main_queue(), ^{\n        @autoreleasepool",
     "dispatch_async(dispatch_get_global_queue(0, 0), ^{\n        @autoreleasepool",
     "no longer contains \"dispatch_async(dispatch_get_main_queue()"),
    ("windows-guard", "completion_alerts_windows.go", "TestCompletionAlertPlatformMechanisms",
     "flashTray = 0x00000002", "flashTray = 0x00000003", "no longer contains \"flashTray = 0x00000002\""),
    ("mac-invocation", "completion_alerts_darwin.go", "TestDarwinCompletionCueChecksFocusEpochAndCoalescesLatestMainQueueRequest",
     "return available && !active && atomic_load(&fairdropAlertReady)",
     "return available && active && atomic_load(&fairdropAlertReady)",
     "informational attention seam was not invoked"),
    ("mac-attention-kind", "completion_alerts_darwin.go", "TestDarwinCompletionCueChecksFocusEpochAndCoalescesLatestMainQueueRequest",
     "fairdropInvokeAttention(NSInformationalRequest);", "fairdropInvokeAttention(NSCriticalRequest);",
     "informational attention seam was not invoked"),
    ("mac-stale-epoch", "completion_alerts_darwin.go", "TestDarwinCompletionCueChecksFocusEpochAndCoalescesLatestMainQueueRequest",
     "atomic_load(&fairdropAlertEpoch) == epoch", "true", "opt-out failed to fence delayed native work"),
    ("windows-invocation", "completion_alerts_windows.go", "TestWindowsCompletionCueUsesOwnVisibleBackgroundWindowAndFiniteTrayFlash",
     'alertWindowClassFunc(ownWindow) != "wailsWindow"',
     'alertWindowClassFunc(ownWindow) != "otherWindow"', "want one finite taskbar-only flash"),
    ("windows-stale-epoch", "completion_alerts_windows.go", "TestWindowsCompletionCueUsesOwnVisibleBackgroundWindowAndFiniteTrayFlash",
     "func nativeCompletionInvalidate(epoch uint64) { alertEpoch.Store(epoch) }",
     "func nativeCompletionInvalidate(epoch uint64) { _ = epoch }", "disabled epoch flashed the taskbar"),
    ("windows-focus-transition", "completion_alerts_windows.go", "TestWindowsObserverPreservesBackgroundAcrossForeignAppSwitch",
     "previous.known != known || previous.background != background",
     "previous.hwnd != hwnd", "foreign-to-foreign switch invalidated eligibility"),
    ("windows-observer-exit", "completion_alerts_windows.go", "TestWindowsRegisteredObserverClearsBackgroundOnEveryExit",
     "defer alertFocus.Store(nil)", "// injected: stale focus survives observer exit",
     "observer exit retained obsolete background eligibility"),
)


def isolated_source(destination: Path) -> None:
    files = subprocess.check_output(
        ["git", "ls-files", "-co", "--exclude-standard", "-z"], cwd=ROOT,
    ).split(b"\0")
    for raw in files:
        if not raw:
            continue
        relative = Path(os.fsdecode(raw))
        if any(part in {".git", ".claude", ".cache", "ms-playwright", "node_modules", "build", "render"}
               for part in relative.parts):
            continue
        if relative.parts[:2] == ("_bmad-output", "implementation-artifacts") and \
                relative.name.endswith(".log"):
            continue
        source = ROOT / relative
        if not source.is_file() or source.is_symlink():
            continue
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, target)


def verify(directory: Path, name: str, log: Path, mode: str, assertion: str = "") -> None:
    command = ["go", "test", "-count=1", "-json", "-timeout", "60s", "-run", f"^{name}$", "."]
    with log.open("w", encoding="utf-8") as stream:
        result = subprocess.run(command, cwd=directory, stdout=stream, stderr=subprocess.STDOUT,
                                check=False, timeout=180)
    verdict = ["go", "run", "./scripts/mutationverdict", "-mode", mode, "-test", name,
               "-status", str(result.returncode), "-log", str(log)]
    if assertion:
        verdict.extend(["-assert", assertion])
    subprocess.run(verdict, cwd=directory, check=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    log_dir = args.log_dir.resolve()
    if log_dir.is_relative_to(ROOT):
        parser.error("--log-dir must be outside the checkout")
    log_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="fairdrop-alert-mutations-") as temporary:
        clone = Path(temporary) / "project"
        clone.mkdir()
        isolated_source(clone)
        for label, filename, name, before, after, assertion in CASES:
            if label.startswith("mac-") and label != "mac-guard" and sys.platform != "darwin":
                continue
            if label.startswith("windows-") and label != "windows-guard" and os.name != "nt":
                continue
            source = clone / filename
            original_bytes = source.read_bytes()
            original = original_bytes.decode("utf-8").replace("\r\n", "\n")
            if original.count(before) != 1:
                raise RuntimeError(f"{label}: source anchor count {original.count(before)}")
            verify(clone, name, log_dir / f"{label}-baseline.json", "baseline")
            try:
                source.write_bytes(original.replace(before, after, 1).encode("utf-8"))
                verify(clone, name, log_dir / f"{label}-mutation.json", "mutation", assertion)
            finally:
                source.write_bytes(original_bytes)
            print(f"KILLED: {label} by {name}", flush=True)


if __name__ == "__main__":
    main()
