# Evidence: Story 6.2: Pin What Ships, Not the Template

Baseline: `611cad441173b94dcba2f11ade94c2d121f870db`.

Story 6.1 ended with eight assertions about the icon, and every one of them read an input asset
under `build/`. None opened the product. This story closes that, and with it the same gap for the
version strings that `release_identity_test.go` pins only in template form.

## The mechanism was verified before the spec was written

Not assumed, because the last time this repo reasoned about the built exe's resources without
inspecting them it produced [[D-128]], a shipped defect that did not exist. Walking a real
`build/bin/fairdrop.exe`:

| resource | found |
|---|---|
| `RT_ICON` (3) | six PNG payloads decoding to 256, 128, 64, 48, 32, 16 RGBA |
| `RT_GROUP_ICON` (14) | one |
| `RT_VERSION` (16) | one |

`debug/pe` supplies the section table; the three-level directory walk (type -> name -> language)
is by hand because no standard-library package exposes it. A leaf's data entry holds an **RVA**,
so the section's virtual address is subtracted to index its raw bytes. No new dependency.

## What the embedded icons actually measure

The comparison is exact, not approximate: `winres` copies the `.ico`'s payloads verbatim, so every
size measures **0.000** mean per-channel distance against the committed entry.

```
256x256: exe vs committed .ico distance 0.000
128x128: exe vs committed .ico distance 0.000
64x64:   exe vs committed .ico distance 0.000
48x48:   exe vs committed .ico distance 0.000
32x32:   exe vs committed .ico distance 0.000
16x16:   exe vs committed .ico distance 0.000
```

The tolerance is still Story 6.1's per-size measured value rather than an exact-match assertion, so
a future resource compiler that re-encodes is diagnosed as drift rather than reported as a wholesale
mismatch.

## D-128's residue, and what was actually wrong

D-128 was withdrawn as a defect because the version values are correct and Win32 reads them. Two
real things remained, and this story fixed both:

- `build/windows/info.json` declared **no `FileVersion` string at all** -- only `fixed.file_version`,
  which lands in FIXEDFILEINFO. The new test caught this on its first run, before any change.
- The string table sat under the language-neutral key `0000`. Win32 reads that correctly; .NET's
  `FileVersionInfo` does not, which is the tool that produced D-128's wrong diagnosis.

After adding `FileVersion` and moving to `0409`, the reader that could not see the table before:

```
ProductName=[FairDrop]  FileDescription=[FairDrop]  CompanyName=[FairDrop]
FileVersion=[1.0.0]     ProductVersion=[1.0.0]
```

Every one of those was blank before this story. Recorded because the change is a *shipped-behaviour*
change made to accommodate one reader's known limitation: the neutral table was not wrong, and the
trade-off is stated here rather than presented as a cleanup.

## Derived mutation table

From `scripts/verify-asset-mutations.py --exe`, which mutates the committed `.ico` and `wails.json`
rather than patching a PE by hand -- an exe that disagrees with its committed inputs *is* the
regression, and it is reachable without hand-editing a binary.

```
| # | mutation | named test | killed |
|---|---|---|---|
| 1 | committed .ico no longer matches the exe | `EmbedTheCommittedIcon` | yes |
| 2 | wails.json productVersion bumped, no rebuild | `CarryTheCommittedIdentity` | yes |
| 3 | exe absent | both tests | skipped, not passed |

3 of 3 exe mutations behaved as required.
```

Case 3 matters as much as the other two: the criterion is "absent build skips, **never passes**",
and a test that silently passed when there was nothing to inspect would be the most comfortable
kind of vacuous.

## The drift clause, and why `git diff` would not have worked

D-130 is that `wails build` GENERATES `build/windows/icon.ico` whenever it is absent, so an `.ico`
missing from git is manufactured by the runner and then satisfies every icon assertion against a
file the repository does not contain. The fix is a clause in the post-build drift check -- but it
has to use `git status --porcelain`, because **`git diff` never reports untracked paths**, and
untracked is exactly the case:

```
clean tree            -> ''
modified .ico         -> ' M build/windows/icon.ico'
.ico missing from git -> 'D  build/windows/icon.ico' + '?? build/windows/icon.ico'
```

## A deviation from the spec's Boundaries, recorded rather than taken quietly

The frozen Boundaries said to env-gate this exactly as `TestDarwinBuiltAppSurvivesUnusableLock`
does. That test *launches* a binary and is gated for its side effects; this one only reads bytes,
and env-gating it would mean it never ran locally after a build. It skips on the exe's absence
instead, which serves the boundary's stated purpose ("a local `go test ./...` with no build present
skips rather than fails") while still running for anyone who has built, and always on CI, which
builds first. Flagged for the owner rather than buried.

## Deliberately out of scope

The macOS product icon. `iconfile.icns` is derived on every build and nothing is committed under
`build/darwin/` but the two plists, so the `.app`'s icon is unverified at the product layer exactly
as the exe's was. Recorded as [[D-131]] rather than left as an unstated omission.

## Verification

`go test -count=1 -run '^TestExeResources' -v .` (both pass, six distances logged) ·
`go test -count=1 -run TestVerifyWorkflow .` (the workflow pins accept the new step) ·
`gofmt -l .` / `go vet ./...` / `go tool staticcheck ./...` clean · `go test -count=1 ./...` and
`-race ./...` green with `CGO_ENABLED=1` confirmed · `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go vet
./...` and the linux equivalent clean -- **vet, not build**, because `go build` never compiles
`_test.go` files and this story is entirely a test file.
