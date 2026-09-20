# Epic 6 retrospective: native macOS and verification evidence

Date: 2026-09-20. Reviewed commit: `4f7a86e6b3902c842d7f55d15529d10f9bb4a9cb`.
No production changes were made during this retrospective.

## Native build and logo

Built using Wails v2.15.0 / Go 1.27.0 / Node 26.7.0 on macOS 26.6.2 arm64.
The app launched through computer-use and showed its normal idle screen in both
accessibility output and a screenshot. Finder visibly showed the FairDrop logo.
Dock observation timed out; this is a Finder visual check, not a Dock claim.
No manual nearby-device transfer was performed.

`plutil -p build/bin/fairdrop.app/Contents/Info.plist` reports
`CFBundleIconFile = iconfile`, `CFBundleIdentifier = com.fairdrop.fairdrop`.
`iconutil -c iconset build/bin/fairdrop.app/Contents/Resources/iconfile.icns`
successfully decoded 32, 64, 256, 512 and 1024 pixel images. Pillow RGBA inspection
showed each has alpha extrema (0,255) and a transparent top-left corner.
The 1024 image is byte-for-byte equal to the master's decoded RGBA pixels.
The 512 image was visually inspected. Screenshots are present in the task's tool
record; no screenshot file is claimed to be committed.

Build transcript:

```text
Wails CLI v2.15.0


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
ld: warning: object file (/private/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/go-link-4060081382/go.o) was built for newer 'macOS' version (13.0) than being linked (11.0)
Done.
  • Packaging application: Done.
  • Self-signing application: Done.
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 30.996s.

 ♥   If Wails is useful to you or your company, please consider sponsoring the project:
https://github.com/sponsors/leaanthony

```

## Local automated checks

Ran sequentially after the build: bindings content drift / .gitkeep / build-asset
drift, gofmt, go vet, go tool staticcheck, uncached Go tests, explicit
`go env CGO_ENABLED == 1`, uncached race tests, then frontend tests. All passed.
The race build itself also demonstrates a working cgo toolchain; the standalone
CI cgo probe was not separately repeated locally.

```text
ok  	fairdrop	2.200s
ok  	fairdrop/internal/network	0.979s
ok  	fairdrop/internal/qr	0.602s
ok  	fairdrop/internal/server	4.367s
ok  	fairdrop/internal/source	1.161s
ok  	fairdrop/internal/stream	3.434s
ok  	fairdrop/internal/transfer	1.592s
ok  	fairdrop/scripts/mutationverdict	0.739s
ok  	fairdrop	8.239s
ok  	fairdrop/internal/network	2.233s
ok  	fairdrop/internal/qr	3.107s
ok  	fairdrop/internal/server	6.577s
ok  	fairdrop/internal/source	1.567s
ok  	fairdrop/internal/stream	107.652s
ok  	fairdrop/internal/transfer	2.386s
ok  	fairdrop/scripts/mutationverdict	1.600s

> fairdrop-frontend@1.0.0 test
> vitest run


 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend


 Test Files  17 passed (17)
      Tests  547 passed (547)
   Start at  03:33:21
   Duration  3.17s (transform 2.04s, setup 0ms, import 3.57s, tests 3.50s, environment 15.05s)


```

The browser suite initially failed because this Mac lacked Playwright's Chromium
binary. Installed it with `npx playwright install chromium`, then reran successfully.
The complete initial error is retained below rather than reported as a product failure.

<details><summary>Initial missing-browser output</summary>

```text

> fairdrop-frontend@1.0.0 test:browser
> vitest run --config vitest.browser.config.ts


 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

⎯⎯⎯⎯⎯⎯ Unhandled Errors ⎯⎯⎯⎯⎯⎯

Vitest caught 1 unhandled error during the test run.
This might cause false positive tests. Resolve unhandled errors to make sure your tests are not affected.

⎯⎯⎯⎯⎯⎯ Unhandled Error ⎯⎯⎯⎯⎯⎯⎯
Error: browserType.launch: Executable doesn't exist at /Users/jaesonmartin/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell
╔════════════════════════════════════════════════════════════╗
║ Looks like Playwright was just installed or updated.       ║
║ Please run the following command to download new browsers: ║
║                                                            ║
║     npx playwright install                                 ║
║                                                            ║
║ <3 Playwright Team                                         ║
╚════════════════════════════════════════════════════════════╝
 ❯ node_modules/@vitest/browser-playwright/dist/index.js:941:55
 ❯ PlaywrightBrowserProvider.createContext node_modules/@vitest/browser-playwright/dist/index.js:1077:19
 ❯ PlaywrightBrowserProvider.openBrowserPage node_modules/@vitest/browser-playwright/dist/index.js:1147:19
 ❯ PlaywrightBrowserProvider.openPage node_modules/@vitest/browser-playwright/dist/index.js:1161:23
 ❯ TestProject._openBrowserPage node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:11040:3
 ❯ BrowserPool.openPage node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2578:3
 ❯ BrowserPool.runTests node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2573:3
 ❯ runWorkspaceTests node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2473:3
 ❯ executeTests node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:3779:25
 ❯ node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13657:7
 ❯ node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13684:11
 ❯ node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13546:19
 ❯ startVitest node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:14621:8
 ❯ start node_modules/vitest/dist/chunks/cac.uFydS1Z4.js:2340:15
 ❯ CAC.run node_modules/vitest/dist/chunks/cac.uFydS1Z4.js:2318:2

⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯
Serialized Error: { log: [] }
⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯


 Test Files   (1)
      Tests  no tests
     Errors  1 error
   Start at  03:33:42
   Duration  222ms (transform 0ms, setup 0ms, import 0ms, tests 0ms, environment 0ms)

error during close browserType.launch: Executable doesn't exist at /Users/jaesonmartin/Library/Caches/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell
╔════════════════════════════════════════════════════════════╗
║ Looks like Playwright was just installed or updated.       ║
║ Please run the following command to download new browsers: ║
║                                                            ║
║     npx playwright install                                 ║
║                                                            ║
║ <3 Playwright Team                                         ║
╚════════════════════════════════════════════════════════════╝
    at /Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js:941:55
    at PlaywrightBrowserProvider.createContext (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js:1077:19)
    at PlaywrightBrowserProvider.openBrowserPage (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js:1147:19)
    at PlaywrightBrowserProvider.openPage (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js:1161:23)
    at TestProject._openBrowserPage (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:11040:3)
    at BrowserPool.openPage (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2578:3)
    at BrowserPool.runTests (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2573:3)
    at runWorkspaceTests (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:2473:3)
    at executeTests (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:3779:25)
    at /Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13657:7
    at /Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13684:11
    at /Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:13546:19
    at startVitest (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js:14621:8)
    at start (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cac.uFydS1Z4.js:2340:15)
    at CAC.run (/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cac.uFydS1Z4.js:2318:2) {
  log: [],
  name: 'Error',
  type: 'Unhandled Error',
  stacks: [
    {
      method: '',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js',
      line: 941,
      column: 55
    },
    {
      method: 'PlaywrightBrowserProvider.createContext',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js',
      line: 1077,
      column: 19
    },
    {
      method: 'PlaywrightBrowserProvider.openBrowserPage',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js',
      line: 1147,
      column: 19
    },
    {
      method: 'PlaywrightBrowserProvider.openPage',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/@vitest/browser-playwright/dist/index.js',
      line: 1161,
      column: 23
    },
    {
      method: 'TestProject._openBrowserPage',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 11040,
      column: 3
    },
    {
      method: 'BrowserPool.openPage',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 2578,
      column: 3
    },
    {
      method: 'BrowserPool.runTests',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 2573,
      column: 3
    },
    {
      method: 'runWorkspaceTests',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 2473,
      column: 3
    },
    {
      method: 'executeTests',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 3779,
      column: 25
    },
    {
      method: '',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 13657,
      column: 7
    },
    {
      method: '',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 13684,
      column: 11
    },
    {
      method: '',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 13546,
      column: 19
    },
    {
      method: 'startVitest',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cli-api.CnMVyzaz.js',
      line: 14621,
      column: 8
    },
    {
      method: 'start',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cac.uFydS1Z4.js',
      line: 2340,
      column: 15
    },
    {
      method: 'CAC.run',
      file: '/Users/jaesonmartin/Projects/FairDrop/frontend/node_modules/vitest/dist/chunks/cac.uFydS1Z4.js',
      line: 2318,
      column: 2
    }
  ]
}

```
</details>

Final browser result:

```text

> fairdrop-frontend@1.0.0 test:browser
> vitest run --config vitest.browser.config.ts


 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend


 Test Files  1 passed (1)
      Tests  17 passed (17)
   Start at  03:34:21
   Duration  3.72s (transform 0ms, setup 0ms, import 216ms, tests 697ms, environment 0ms)


```

Preflight also passed: `GOOS=darwin GOARCH=arm64 go build ./...`,
`GOOS=linux GOARCH=amd64 go build ./...`, native-built standalone staticcheck with
`GOOS=darwin GOARCH=arm64`, and `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...`.
Line-ending and `git diff --check` checks passed.
Hosted-only unusable-lock smoke and the full native mutation battery were not run
locally; the matching hosted run below includes them. These checks plus newer
local toolchains are not represented as an exact local replay of CI.

## Canonical asset mutation rerun

Executed the exact final `scripts/verify-asset-mutations.py` from reviewed HEAD in
an isolated detached worktree, using a temporary Python venv with Pillow. The
script restores and SHA-verifies assets between cases. All 27 declared cases
failed their named test. Worktree status was clean afterward. No omitted D-133 or
D-134 case is claimed covered by this result. Windows `--exe` mutations were not
rerun on macOS.

```text
baseline: all TestAppIcon* pass

| # | mutation | named test | killed |
|---|---|---|---|
| 1 | master brightened +1/255, .ico stale | `MasterMatchesIcoFreshness` | yes |
| 2 | master brightened +2/255, .ico stale | `MasterMatchesIcoFreshness` | yes |
| 3 | master hue-swapped, .ico stale | `MasterMatchesIcoFreshness` | yes |
| 4 | 256x256 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 5 | 128x128 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 6 | 64x64 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 7 | 48x48 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 8 | 32x32 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 9 | 16x16 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 10 | 16x16 entry opaque white | `EveryIcoEntryCarriesTheArtwork` | yes |
| 11 | re-derived with ERODE_PX = 0 | `NoOpaqueBackdrop` | yes |
| 12 | master alpha 0, colour intact | `CarriesRealColour` | yes |
| 13 | premultiply defect, full | `MasterAppliesAlphaOnce` | yes |
| 14 | premultiply defect, 20% (ratio bound only) | `MasterAppliesAlphaOnce` | yes |
| 15 | premultiply defect, trailing edge only | `MasterAppliesAlphaOnce` | yes |
| 16 | plaque stretched to fill canvas | `MasterGeometry` | yes |
| 17 | plaque narrowed 240px (columns) | `MasterGeometry` | yes |
| 18 | master fully blank | `MasterGeometry` | yes |
| 19 | master is the Wails scaffold placeholder | `MasterGeometry` | yes |
| 20 | .ico reverted to the Wails scaffold | `IcoCarriesTheFullWailsSizeSet` | yes |
| 21 | .ico 48x48 entry dropped | `IcoCarriesTheFullWailsSizeSet` | yes |
| 22 | .ico gains an extra 24x24 entry | `IcoCarriesTheFullWailsSizeSet` | yes |
| 23 | source render deleted | `SourceRenderIsThePinnedGeometry` | yes |
| 24 | source render replaced, same size | `SourceRenderIsThePinnedGeometry` | yes |
| 25 | backdrop opaque except the four corners | `NoOpaqueBackdrop` | yes |
| 26 | master saved without an alpha channel | `MasterGeometry` | yes |
| 27 | 16x16 entry, one opaque pixel in its centre | `EveryIcoEntryCarriesTheArtwork` | yes |

27 of 27 mutations failed their named test.

```

## Matching hosted verification

Queried GitHub's run and job APIs, checking actual conclusions rather than a watch
exit code. [Run 35489791923](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35489791923)
for `4f7a86e` completed with `success`.

- [Linux adapter verification (not release proof)](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35489791923/job/106022649906): `success`.
- [verify (windows-latest)](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35489791923/job/106022649982): `success`.
- [verify (macos-latest)](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35489791923/job/106022650032): `success`.

Every non-skipped step in those jobs concluded success. CI supplies the pinned
Go 1.26.7 / Node 24 native platform evidence. Passing ordinary CI does not prove
the omitted asset/executable mutation cases, which are outside its case inventory.

## Git inventory

Range `de16807..4f7a86e`: ten commits, no merges, 21 changed paths. `git diff --stat`
reports 3,563 insertions and seven deletions, excluding binary sizes from line counts.
The original JPEG is 2,096,157 bytes. No production Go/frontend source changed.
