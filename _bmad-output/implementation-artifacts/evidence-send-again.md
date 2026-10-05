# Send Again implementation evidence

Spec: [spec-send-again.md](spec-send-again.md). The six-case mutation inventory and reproducible runner are [mutation-send-again.py](mutation-send-again.py). The runner requires each named baseline to pass, then requires a structured Vitest assertion failure in the same named test. It restores the controller source even on failure. Its complete baseline and mutant JSON reports are `/tmp/fairdrop-send-again-mutation-<case>-{baseline,mutant}.json` on the verification machine. The runner's complete output is `/tmp/fairdrop-send-again-mutation-summary.log`.

## Matrix coverage audit

| Spec row | Executed test and assertion |
| --- | --- |
| Live success | `useTransfer.test.tsx` “waits for actual reset after releasing a live Done lease” holds Cancel and reset separately, proves no early Stage, and checks the fresh returned session. |
| Retained success | “stages directly from retained Done and refuses overlapping activations” proves one Stage, no Cancel, and synchronous duplicate refusal. |
| Missing target | `App.test.tsx` “wires Send Again on a Done card only when the controller has a target” omits the action when `canSendAgain` is false; hook “does nothing outside Done or without a target” refuses the command. |
| Changed item | Hook “uses normal Stage validation when the remembered item has changed” exercises `path_not_found`; `TestNativeSendAgainRevalidatesDeletedSource` deletes the real selected file and gets `path_not_found` from App/Source. |
| Double activation | Hook live and retained cases prove one operation; `App.test.tsx` “admits one Send Again operation” proves the busy action group and guards Done while in flight. |
| Dismiss/cancel | Hook “forgets on user Done, cancellation, replacement, and unmount” and “cannot stage from a delayed reset after unmount”; `App.sendAgain.test.tsx` proves user Done removes Send Again even when Cancel rejects without reset. |
| Replacement | Hook “keeps the target when an outcome chooser is cancelled, then replaces it on a new Stage” verifies the admitted new path; “preserves the live Done target through Send Another lease release and a cancelled chooser” covers the live lease route. |
| Other states | Hook “refuses Send Again while a selection is pending, staged, transferring, or errored”, including retained Error; existing retry tests remain green. |

The real-controller `App.sendAgain.test.tsx` drives a native-drop callback, lifecycle Complete and reset, then clicks Send Again. It checks one new Stage of the original path, a distinct rendered QR image, the new revealed link, stable focus on the Done panel at reset, focus at the new Staged heading, and an empty status announcer. The rendered Chromium test in `browser/accessibility.test.tsx` proves the three actions fit at 320px with normal, 200% text, and forced colors; provider keyboard Tab/Enter activates Send Again, Tab/ArrowDown opens Send Another with File focused, and forced colors retains a focus outline and secondary border. Chromium is not native WebKit proof.

The real App/Source/Server test `TestNativeCompletedItemCanBeStagedAgainWithFreshCapability` repeats both a file and a folder. Each second download has exact bytes (the folder is parsed as ZIP), a new session ID and a different extracted URL token path, and the old URL cannot deliver again. `TestStageDrawsTwoIndependentIdentifiers` retains the production entropy proof. The frontend makes no claim that it generated a token.

## Mutation audit

The exact six-case inventory is in `mutation-send-again.py`. All six baselines passed alone and all six mutants produced one named assertion failure, with zero unrelated passing or failing tests in the filtered run:

| Case | Broken guarantee | Named failing assertion |
| --- | --- | --- |
| retention | Clear remembered path on Done | “retains the remembered path on a completed Done” |
| state-guard | Admit Send Again outside Done | “refuses Send Again while a selection is pending, staged, transferring, or errored” |
| admission | Remove synchronous Send Again operation guard | “waits for actual reset after releasing a live Done lease” |
| abandonment | Remove epoch/path check after asynchronous release | “forgets on user Done, cancellation, replacement, and unmount” |
| release-recovery | Ignore rejected internal Cancel result | “settles a failed terminal lease release without staging or hiding the outcome” |
| live-chooser | Treat internal Send Another lease release as user cancel | “preserves the live Done target through Send Another lease release and a cancelled chooser” |

## Verification

The first pre-review candidate passed the canonical sequential gate in the integration worktree: Wails, binding/assets drift, format, vet, staticcheck, full Go, cgo, full Go race, frontend, browser plus live receiver, and Darwin/Linux builds. The exact gate summary is `/tmp/fairdrop-send-again-gate-summary.log`; the twelve complete logs use `/tmp/fairdrop-send-again-gate-<step>.log`. It was an earlier candidate, so the final review fixes were checked separately below and require exact-tip native CI for release proof.

Final review-fix local checks, sequentially:

- `/Users/jaesonmartin/go/bin/wails build`: passed, packaged macOS app (`/tmp/fairdrop-send-again-final-wails.log`).
- `frontend/npm test`: 21 files, 846 tests passed (`/tmp/fairdrop-send-again-final-frontend.log`).
- `frontend/npm run test:browser`: 84 Chromium tests passed, followed by live receiver GET/page/keyboard POST/attachment bytes pass (`/tmp/fairdrop-send-again-final-browser.log`).
- `go test -count=1 -run 'TestNativeCompletedItemCanBeStagedAgainWithFreshCapability|TestNativeSendAgainRevalidatesDeletedSource|TestStageDrawsTwoIndependentIdentifiers' -v ./ ./internal/transfer`: file, folder, deleted source, independent identifier cases passed (`/tmp/fairdrop-send-again-final-go.log`).
- `python3 _bmad-output/implementation-artifacts/mutation-send-again.py`: six passing baselines and six named assertion failures (`/tmp/fairdrop-send-again-mutation-summary.log`).

Main orchestrator observed a same-machine macOS Wails smoke on the pre-review candidate: chooser selected a 48-byte file; first browser Download reached Sent; synthetic Tab/Return activated Send Again and produced a new observed URL token without a chooser; second browser Download reached Sent; synthetic Tab/Down opened Send Another with File focused. This is a native WebKit positive observation with synthetic keys on the earlier candidate, not a physical-keyboard, phone, or final patched-build pass. The final Chromium suite and Wails build are separate evidence.

## Verification transcripts

The pre-review gate ran in the integration worktree. These are its complete command outputs and conclusions; final review-fix outputs follow. Empty logs mean the command succeeded silently.

### Pre-review summary

```text
build exit=0
drift exit=0
format exit=0
vet exit=0
staticcheck exit=0
go exit=0
cgo exit=0
race exit=0
frontend exit=0
browser exit=0
darwin exit=0
linux exit=0

```

### Pre-review build

```text
[0;92mWails CLI[0m [0;31mv2.15.0[0m


# Build Options

Platform(s)        | darwin/arm64
Compiler           | /opt/homebrew/bin/go
Skip Bindings      | false
Build Mode         | production
Devtools           | false
Frontend Directory | /private/tmp/fairdrop-send-again-integration/frontend
Obfuscated         | false
Install Scope      | machine
Skip Frontend      | false
Compress           | false
Package            | true
Clean Bin Dir      | false
LDFlags            |
Tags               | []
Race Detector      | false


# Building target: darwin/arm64

  • Generating bindings: Done.
  • Installing frontend dependencies: Done.
  • Compiling frontend: Done.
  • Compiling application: # fairdrop
ld: warning: object file (/private/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/go-link-870463117/go.o) was built for newer 'macOS' version (13.0) than being linked (11.0)
Done.
  • Packaging application: Done.
  • Self-signing application: Done.
Built '/private/tmp/fairdrop-send-again-integration/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 13.512s.

 ♥   If Wails is useful to you or your company, please consider sponsoring the project:
https://github.com/sponsors/leaanthony

```

### Pre-review drift

```text

```

### Pre-review format

```text

```

### Pre-review vet

```text

```

### Pre-review staticcheck

```text

```

### Pre-review go

```text
ok  	fairdrop	1.252s
ok  	fairdrop/internal/network	0.214s
ok  	fairdrop/internal/qr	0.862s
ok  	fairdrop/internal/server	4.811s
ok  	fairdrop/internal/source	1.033s
ok  	fairdrop/internal/stream	4.047s
ok  	fairdrop/internal/transfer	1.653s
ok  	fairdrop/scripts	1.518s
ok  	fairdrop/scripts/mutationverdict	1.682s

```

### Pre-review cgo

```text

```

### Pre-review race

```text
ok  	fairdrop	8.983s
ok  	fairdrop/internal/network	1.505s
ok  	fairdrop/internal/qr	2.318s
ok  	fairdrop/internal/server	5.473s
ok  	fairdrop/internal/source	1.817s
ok  	fairdrop/internal/stream	111.655s
ok  	fairdrop/internal/transfer	2.631s
ok  	fairdrop/scripts	2.843s
ok  	fairdrop/scripts/mutationverdict	2.460s

```

### Pre-review frontend

```text

> fairdrop-frontend@1.3.1 test
> vitest run


 RUN  v4.1.11 /private/tmp/fairdrop-send-again-integration/frontend


 Test Files  20 passed (20)
      Tests  842 passed (842)
   Start at  20:44:26
   Duration  4.41s (transform 2.71s, setup 0ms, import 4.65s, tests 8.67s, environment 18.94s)


```

### Pre-review browser

```text

> fairdrop-frontend@1.3.1 test:browser
> vitest run --config vitest.browser.config.ts && node browser/receiver-live.mjs


 RUN  v4.1.11 /private/tmp/fairdrop-send-again-integration/frontend


 Test Files  2 passed (2)
      Tests  84 passed (84)
   Start at  20:44:30
   Duration  70.29s (transform 0ms, setup 0ms, import 468ms, tests 71.60s, environment 0ms)

receiver live browser: real GET page, headers, 320px layout, keyboard POST and exact attachment bytes passed

```

### Pre-review darwin

```text

```

### Pre-review linux

```text

```

### Final review-fix wails

```text
[0;92mWails CLI[0m [0;31mv2.15.0[0m


# Build Options

Platform(s)        | darwin/arm64
Compiler           | /opt/homebrew/bin/go
Skip Bindings      | false
Build Mode         | production
Devtools           | false
Frontend Directory | /Users/jaesonmartin/Projects/FairDrop/frontend
Obfuscated         | false
Install Scope      | machine
Skip Frontend      | false
Compress           | false
Package            | true
Clean Bin Dir      | false
LDFlags            |
Tags               | []
Race Detector      | false


# Building target: darwin/arm64

  • Generating bindings: Done.
  • Installing frontend dependencies: Done.
  • Compiling frontend: Done.
  • Compiling application: # fairdrop
ld: warning: object file (/private/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/go-link-2654989978/go.o) was built for newer 'macOS' version (13.0) than being linked (11.0)
Done.
  • Packaging application: Done.
  • Self-signing application: Done.
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 11.551s.

 ♥   If Wails is useful to you or your company, please consider sponsoring the project:
https://github.com/sponsors/leaanthony

```

### Final review-fix frontend

```text

> fairdrop-frontend@1.3.1 test
> vitest run


 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend


 Test Files  21 passed (21)
      Tests  846 passed (846)
   Start at  21:00:47
   Duration  3.84s (transform 2.51s, setup 0ms, import 4.49s, tests 7.71s, environment 16.57s)


```

### Final review-fix browser

```text

> fairdrop-frontend@1.3.1 test:browser
> vitest run --config vitest.browser.config.ts && node browser/receiver-live.mjs


 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend


 Test Files  2 passed (2)
      Tests  84 passed (84)
   Start at  20:57:28
   Duration  70.28s (transform 0ms, setup 0ms, import 431ms, tests 71.74s, environment 0ms)

receiver live browser: real GET page, headers, 320px layout, keyboard POST and exact attachment bytes passed

```

### Final review-fix go

```text
=== RUN   TestNativeCompletedItemCanBeStagedAgainWithFreshCapability
=== RUN   TestNativeCompletedItemCanBeStagedAgainWithFreshCapability/file
=== RUN   TestNativeCompletedItemCanBeStagedAgainWithFreshCapability/folder
--- PASS: TestNativeCompletedItemCanBeStagedAgainWithFreshCapability (0.04s)
    --- PASS: TestNativeCompletedItemCanBeStagedAgainWithFreshCapability/file (0.02s)
    --- PASS: TestNativeCompletedItemCanBeStagedAgainWithFreshCapability/folder (0.02s)
=== RUN   TestNativeSendAgainRevalidatesDeletedSource
--- PASS: TestNativeSendAgainRevalidatesDeletedSource (0.01s)
PASS
ok  	fairdrop	0.347s
=== RUN   TestStageDrawsTwoIndependentIdentifiers
--- PASS: TestStageDrawsTwoIndependentIdentifiers (0.00s)
PASS
ok  	fairdrop/internal/transfer	0.471s

```

### Final mutation inventory

```text
retention: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-retention-{baseline,mutant}.json
state-guard: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-state-guard-{baseline,mutant}.json
admission: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-admission-{baseline,mutant}.json
abandonment: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-abandonment-{baseline,mutant}.json
release-recovery: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-release-recovery-{baseline,mutant}.json
live-chooser: baseline=pass; named_assertion_failure=True; logs=/tmp/fairdrop-send-again-mutation-live-chooser-{baseline,mutant}.json
```

Each structured mutant report has exactly one failed selected assertion. The failed assertion messages are:

- `retention`: AssertionError: expected false to be true // Object.is equality
- `state-guard`: AssertionError: expected "vi.fn()" to be called 1 times, but got 2 times
- `admission`: AssertionError: duplicate activation is refused before lease release: expected false to be true // Object.is equality
- `abandonment`: AssertionError: expected "vi.fn()" to be called 1 times, but got 2 times
- `release-recovery`: AssertionError: rejected release must settle the action: expected false to be true // Object.is equality
- `live-chooser`: AssertionError: expected false to be true // Object.is equality

## Retained logs and review disposition

Complete baseline/mutant reports, final Wails/frontend/browser/native logs, and the reviewer’s surviving live-chooser mutation are retained in [evidence-send-again-logs](evidence-send-again-logs). The temporary paths above identify their original execution locations.

Three isolated review layers examined the candidate. The edge review returned no findings. The blind and verification reviews identified release-failure recovery, known folder kind, explicit token comparison, folder repetition and source revalidation, real-controller UI/focus coverage, browser keyboard/forced-colors coverage, and live-chooser cancellation preservation. All were patched and verified under the existing intent; no deferred production finding remains. Main additionally identified stale action availability after public Cancel rejection; the Sol implementer patched it and the real-controller test verified it.

Final orchestration checks: line endings and diff whitespace passed. Focused native repeat/deleted-source tests also passed with the race detector (`fairdrop-send-again-final-native-race.log` in the retained logs). The final patched build was launched on macOS and displayed its ready Idle view; it remains open for use. No final patched-build transfer or physical-keyboard pass is inferred from that startup observation.

The final portable mutation runner was re-executed using the OS temporary directory: all six named baselines passed and all six structured assertion failures were confirmed. The retained JSON reports above are from that final run. The controller SHA-256 was identical before and after (`0b639c66cb5bfa7b985406fd335415e62dd3795ce3b34805342d7cc895acb736`).

Committed text transcripts normalize terminal line endings and trailing padding only; no diagnostic lines are omitted. Structured JSON reports retain their complete contents.
