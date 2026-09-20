# Story 6.3 evidence — Verify and Release 1.1.0

## Implementation handoff

The canonical asset driver now derives the ICO size cases from `wantIcoSizes` and
adds the four D-133 mutations: off-centre transparent bands, a non-square ICO entry,
an inflated measured-distance calibration, and disagreement between the Go and Python
source digest pins. It snapshots the clean implemented inputs and verifies exact
restoration after every case.

The executable table now includes a standard-library PE mutation that removes one
RT_ICON directory entry and a real rebuild with `info.json`'s language key changed
from `0409` to `0000`. The Windows executable and tracked inputs are snapshotted from
the implemented baseline and restored between cases. The `--exe` path does not import
or require Pillow. A failed mutation command or failed rebuild aborts the driver and
cannot be scored as a kill.

The build-asset drift assertion lives in `scripts/verify-build-asset-drift.sh`; CI calls
that exact script after `wails build`. This permits the missing-ICO/rebuild mutation to
exercise the production assertion directly while retaining untracked-file detection.

## Local verification

The reviewed asset inventory ran in a detached worktree: **31 of 31** mutations failed their
named test. The four D-133 cases additionally required the dedicated diagnostic,
so a different assertion failing in the same test could not score the case. All 32
baseline/per-case transcripts are retained in
`evidence-6-3-asset-mutation-logs/`. After
the run, every mutated input matched its baseline digest; only the intentionally
copied driver/helper differed from detached HEAD.

The ordered local gate passed on macOS arm64 with Go 1.27.0, Node 26.7.0 and Wails
2.15.0: `wails build`; `gofmt -l`; `go vet ./...`; pinned staticcheck; all Go tests;
all Go tests under the race detector with cgo enabled; 547 frontend jsdom tests;
17 rendered Chromium tests; and the frontend production build. Darwin/arm64 and
Linux/amd64 `go build ./...` preflights passed, as did Darwin/arm64 staticcheck.
`git diff --check` and the canonical line-ending check were clean.

The packaged Mac bundle reports CFBundleShortVersionString and CFBundleVersion
`1.1.0`; `codesign --verify --deep --strict` exited 0. After the prior FairDrop
process was quit, the rebuilt bundle launched to the normal idle window with its
heading and choose control. This is a native launch observation, not a transfer,
Dock-icon, signing-identity or notarization claim.

Final candidate `54a5f87223f34647aed40344296d79824a33d4bd` passed the complete native gate twice.
PR run [35499365200](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35499365200)
concluded **success**: macOS job `106048506575`, Linux adapter job `106048506728`, and
Windows job `106048506738` each concluded **success**. Push run
[35499362706](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35499362706) independently
concluded **success** at the same SHA: Linux adapter job `106048342426`, Windows job
`106048342511`, and macOS job `106048342522` each concluded **success**. The candidate was
then merged by merge commit `cb40879764a31c512ba835f9c638c959ffa7adfd`, and annotated tag
`v1.1.0` was pushed at that exact merge commit.

Release run [35500351435](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35500351435)
concluded **success** at tag commit `cb40879764a31c512ba835f9c638c959ffa7adfd`: Linux gate job
`106050918775`, macOS gate job `106050918841`, Windows gate job `106050918926`, Windows build
job `106052726162`, macOS build job `106052726659`, and release job `106053084898` all
concluded **success**. The non-draft, non-prerelease
[v1.1.0 release](https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.1.0) was published
at 2026-09-20 09:03:07 UTC and is the latest release.

Both published binaries and their checksum files were downloaded from the release. The checksum
files passed `shasum -c`, and the downloads were byte-identical to the inspected CI artifacts:
`fairdrop.exe` is 13,819,904 bytes with SHA-256
`79b12f642728e09d59600d88c8d36a615b1e1106c26e07e9f5c7d373538f0a98`; `fairdrop-macos.zip`
is 6,256,840 bytes with SHA-256
`128de947600e6c7485c7dfb1fee57f609e585e987fb097dc33041464d77abecc`.

The downloaded macOS bundle reports both plist versions as 1.1.0, passes
`codesign --verify --deep --strict`, and decodes through `iconutil` to the exact 1024px RGBA
master. The Mac was locked, so an additional downloaded-artifact GUI launch was not attempted;
the earlier local 1.1.0 native launch observation remains recorded above. A portable copy of the
existing Windows PE byte parser, copied under a platform-neutral test filename with only its Windows build constraint removed, passed both
resource tests against the downloaded executable: all six icon distances were zero and product
identity/version was 1.1.0. This inspects the actual Windows artifact bytes; it is not a claim that
the executable was launched on Windows.

Native Windows PR run **35499106925**, job **106047534078**, executed the built-resource
baseline and all five executable probes successfully: stale committed ICO, stale product
version, structurally missing RT_ICON, rebuilt neutral `0000` language key, and exact
two-test absence skips. It also rebuilt after removing the ICO and observed the canonical
drift check reject the regenerated untracked path. The six complete executable transcripts
downloaded from the job are retained in `evidence-6-3-windows-mutation-logs/`; inspection
confirmed the missing 16px assertion, embedded `000004b0` neutral-table assertion, and
both missing-executable diagnostics. This proves the scoped Windows mutations, not the
full final-head gate; the run's Linux and macOS jobs later failed on the portable test defect below.

## Review layers

All three independent review layers completed. Consolidated triage found one blocking
real failure: the committed ICO's hue-mutated 64px PNG grew beyond its original slot.
A regression test now reproduces that input and the helper repacks every payload and
rewrites all offsets. Review also required process-exit/result agreement, diagnostics
scoped to the intended test block, stable inventory pins, fresh log directories,
explicit regenerated-ICO proof, catchable-interrupt wording, and candidate-aware release
documentation. The actionable fixes are implemented, and the final reviewed head's two
successful native gates are recorded above.
The driver now refuses mutation mode outside GitHub Actions unless the caller explicitly
marks the checkout disposable. CI runs in a fresh checkout that is discarded with the job;
local evidence runs in a detached disposable worktree. Catchable SIGINT/SIGTERM paths restore
their snapshots, while SIGKILL cannot execute cleanup and is handled by discarding the worktree.
The review suggestion to claim restoration of every path a full Wails rebuild might touch was
rejected as broader than the implementation can prove: explicit mutation inputs and the exe are
hash-restored, final CI drift checks other build outputs, and the whole checkout is disposable.

The first candidate Linux CI run exposed a Go-version-sensitive regression test: Go 1.27's
PNG encoder made the hue-mutated 64px payload larger, while pinned CI Go 1.26.7 made it five
bytes smaller. The mutation succeeded under both; only the test's size assumption was wrong.
The replacement verifies the committed icon's actual hue-rotated pixels and every entry
boundary, then separately feeds `repackICO` a deterministic grown payload and asserts that
the following offset is rewritten. Both tests pass under Go 1.26.7 and Go 1.27.0.
