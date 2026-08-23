---
title: 'Story 1.1 — Validate and Describe One File Selection'
type: 'feature'
created: '2026-08-23'
status: 'done' # draft | ready-for-dev | in-progress | in-review | done
review_loop_iteration: 1
baseline_commit: '92d473d24bde002804c4d12548b6329c6e86caea'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop cannot tell a usable file from a symlink, a junction, a device node, or a path that no longer exists. There is no `internal/transfer` package, no shared error vocabulary, and no way to turn a path into staged metadata — so every later Epic 1 story has nothing to build on and no agreed way to fail.

**Approach:** Create the consumer-owned `internal/transfer` package holding the binding domain values and coded-error contract, then implement a file-only `SourcePort` that inspects one absolute path and returns an immutable `StagedItem` or a typed error — before any network resource exists.

## Boundaries & Constraints

**Always:**
- `docs/fairdrop-contracts.md` is binding for exported names, shapes, and meanings.
- Filesystem APIs only, never a shell. Pass the path as a value — no cleaning, case-folding, or rewriting.
- `os.Lstat`, never `os.Stat`: never follow or open a link target.
- Errors carry a stable `ErrorCode`, wrap causes with `%w`, and survive `errors.As`.
- Public messages are **not** invented here: `PublicErrorOf` emits the exact string the UX copy registry fixes for that code, verbatim, for all twelve codes. They hold no path, basename, or token.
- `context.Context` first and honored; a cancelled context returns `cancelled`.
- Preserve spaces, Unicode, and long/UNC Windows paths wherever Go permits.

**Ask First:**
- Any third-party dependency. This story is stdlib-only.
- Any deviation from `docs/fairdrop-contracts.md` — that needs a contract update, not a local workaround.

**Never:**
- No directory inspection; Epic 2 extends this adapter, so leave the seam open without building it.
- Do not edit `internal/network`, `internal/stream`, or `internal/server` — Stories 1.2–1.4 replace those.
- No coordinator, session ID, token, HTTP, QR, mDNS, or frontend work.
- No second source interface anywhere — no provider-owned or shadow type.
- No persistence, network call, whole-file read, or directory walk.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Regular file | Absolute path, existing regular file | `StagedItem{Path, Name: basename, Kind: ItemFile, LogicalSize, ModTime}` | N/A |
| Spaces / Unicode | Spaces, non-ASCII, emoji in path | Success; path preserved byte-for-byte | N/A |
| Zero-byte file | Existing empty file | Success, `LogicalSize: 0` | N/A |
| Empty path | `""` | Rejected before any syscall | `invalid_selection` |
| Relative path | `./notes.txt` | Rejected before any syscall | `invalid_selection` |
| Missing path | Absolute, does not exist | Rejected | `path_not_found`, message omits path |
| Directory | Absolute path to a directory | Rejected, not walked | `path_unsupported` |
| Symlink | Symlink, valid or dangling | Rejected, target never followed | `path_unsupported` |
| Junction / reparse | Windows junction or reparse point | Rejected, target never followed | `path_unsupported` |
| Special file | Device, pipe, socket (e.g. `NUL`) | Rejected | `path_unsupported` |
| Unreadable metadata | `Lstat` fails, not not-exist | Rejected, cause wrapped not surfaced | `path_unsupported` |
| Cancelled context | Context already cancelled | Returns before filesystem work | `cancelled` |
| Wrapped domain error | `DomainError` wrapped via `%w` n times | `ErrorCodeOf` returns the original code | N/A |
| Unknown error | Non-nil, not a `DomainError` | Fixed safe fallback | `transfer_failed` |

</frozen-after-approval>

## Code Map

- `internal/transfer/` — **new package**, owns the contracts. Split across files freely.
- `docs/fairdrop-contracts.md` — **read-only, binding.** Copy exact shapes from three sections: `## Canonical domain values`, `## Coordinator-facing ports` (`SourcePort` only), `## Source mutation and link policy`.
- `.../ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md` — **read-only, binding.** Its `### Stable public error and warning copy` table fixes the exact `PublicError.Message` for every code. Copy the strings; do not paraphrase.
- `internal/source/source.go` plus narrowly scoped platform files — **new**, the file-only `SourcePort` adapter. Validate every syntactically traversed ancestor and the selected root without cleaning or rewriting the caller's path. Reject link-like ancestors. On Windows, reject `FILE_ATTRIBUTE_REPARSE_POINT` even when Go deliberately reports the item as regular (for example a dedup reparse point). Keep platform-specific attribute access behind build-tagged stdlib-only helpers.
- `internal/{network,stream,server}/*.go` — Phase 1 provider-owned placeholders. **Read only, do not edit.**
- `go.mod` — module `fairdrop`, `go 1.25.0`. Must not gain a dependency.
- `main.go`, `app.go`, `main_test.go` — untouched; `main_test.go` must keep passing.

Only the types the Tasks below name are in scope. `FileMetadata`, `ProgressSnapshot`, and the other ports belong to later stories.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/types.go` — `SessionID`, `CapabilityToken`, `ItemKind` (`ItemFile`, `ItemDirectory`), `StagedItem`. Vocabulary every later story imports.
- [x] `internal/transfer/errors.go` — `ErrorCode` + twelve constants, `PublicError`, `CodedError`, `DomainError` with `Unwrap`, `NewError`/`WrapError`/`ErrorCodeOf`/`PublicErrorOf`. One contract so no adapter invents its own. `Error()` must not render the wrapped cause; causes remain available only through `Unwrap`, preventing routine logs from disclosing a path or token.
- [x] `internal/transfer/ports.go` — `SourcePort`, consumer-owned.
- [x] `internal/source/source.go` plus build-tagged platform helpers — file-only implementation using `Lstat`, `Mode().IsRegular()`, ancestor validation, and Windows reparse-attribute validation. Recheck cancellation after filesystem inspection. On Windows, classify an unreachable network path/share as `path_unsupported`, not `path_not_found`.
- [x] `internal/source/source_test.go` — every matrix row plus always-running seam tests for an ancestor link, an unchanged filesystem-call path, a regular-mode Windows reparse item, cancellation during inspection, and unreachable-network-path classification. Skip uncreatable integration fixtures rather than failing; synthetic coverage for the required behavior must still run.
- [x] `internal/transfer/errors_test.go` — `%w` wrapping, code preservation, unknown fallback, exact safe-message content, typed-nil safety, and proof that `Error()` does not render wrapped path/token text.

**Acceptance Criteria:**
- Given a wrapped `DomainError`, when `PublicErrorOf` reads it, then `Code` matches the original and `Message` is the exact registry string for that code, with no path, basename, or token.
- Given each of the twelve `ErrorCode` values, when `PublicErrorOf` maps it, then the message matches the UX copy registry character-for-character.
- Given the finished package, when its exported surface is compared to `docs/fairdrop-contracts.md`, then names and meanings match and no second source interface exists in the module.
- Given the module, when `go build ./...`, `go vet ./...`, `go test -race ./...`, and `gofmt -l .` run, then all pass and `main_test.go` still holds.

## Spec Change Log

- **Review loop 1 (2026-08-23):** Adversarial review showed that the prescribed final-component `Lstat` + `Mode().IsRegular()` check was not sufficient for the binding source-mutation policy. The Code Map, tasks, design notes, and verification guidance now require link-like ancestor rejection, an explicit Windows reparse-attribute check, safe local error rendering, post-inspection cancellation precedence, unchanged-path observation, and Windows unreachable-share classification. This avoids the known-bad state where a file beneath a symlink/junction or a regular-mode reparse file is accepted, wrapped causes leak a source path through ordinary error formatting, and `ERROR_BAD_NETPATH` becomes `path_not_found`. **KEEP:** the exact consumer-owned exported domain surface; the twelve-code and character-for-character UX-copy registry; `%w`/`errors.As` preservation through `Unwrap`; the stateless per-instance filesystem seam; `Mode().IsRegular()` as the portable special-file gate; the successful matrix tests, including synthetic mode coverage and honest integration skips.
- **Post-loop verification hardening (2026-08-23):** The second review of loop 1 added nil-context safety, fail-closed Windows `FileInfo.Sys` handling, a selected-component cancellation seam, directory-mode reparse-ancestor rejection, independent coded-error and JSON wire tests, and native Windows junction/device/long-path fixtures. These patches prevent panics and false trust at unusual boundaries while preserving the same contract surface and public copy; they do not increment `review_loop_iteration` because no second spec loopback occurred.

## Design Notes

**Measured on go1.26.7/windows/amd64 — a symlink-bit check is wrong here.** `os.Lstat` on a Windows junction reports `ModeIrregular`, *not* `ModeSymlink`, so `mode&os.ModeSymlink != 0` silently accepts junctions:

```
junction.lnkdir   symlink=false  irregular=true   IsRegular=false
realdir           symlink=false  IsDir=true       IsRegular=false
NUL               symlink=false  device=true      IsRegular=false
```

`Mode().IsRegular()` is `mode&ModeType == 0`, so one check rejects all three plus symlinks. Keep it as the portable special-file gate, but do not treat it as a complete Windows reparse-point test: Go intentionally reports some reparse tags, including dedup, as regular. Inspect `Win32FileAttributeData.FileAttributes` in a Windows-only helper and reject `FILE_ATTRIBUTE_REPARSE_POINT` as well. If Windows `FileInfo.Sys` does not provide the expected native attribute data, fail closed as `path_unsupported` rather than assuming the item is safe.

**Ancestor traversal:** `os.Lstat` protects only the final component. Validate every syntactically traversed ancestor before accepting the selected root, including components that appear before `..`; a symlink or reparse ancestor is unsupported. Do not `Clean` the caller's path or substitute a normalized string for the final metadata lookup or returned `StagedItem.Path`. The test seam must record every inspected path so unchanged-path and ancestor behavior are observable.

**Diagnostic disclosure:** `DomainError` retains its cause for `errors.Is`/`errors.As` through `Unwrap`, but `Error()` renders only stable code plus the adapter's safe local message. It must not concatenate `cause.Error()`, because `*os.PathError` includes the source path and ordinary logging commonly formats `Error()`.

**Fixtures:** `os.Symlink` needs elevation or Developer Mode on Windows and fails for an ordinary user; junctions (`mklink /J`) do not. Guard symlink fixtures with a skip so the suite stays green unprivileged.

**Classification:** a true missing selected component maps to `path_not_found`; any other `Lstat` failure maps to `path_unsupported`, which the contract defines to include host-unsupported paths. On Windows, check bad/unreachable network-path errors before the broad `errors.Is(err, fs.ErrNotExist)` branch because Go deliberately makes `ERROR_BAD_NETPATH` match `fs.ErrNotExist`.

**Cancellation mechanics:** `os.Lstat` is synchronous and cannot be interrupted through `context.Context`. Check cancellation immediately before and after every metadata call; do not move `Lstat` into an abandoned goroutine, which would leak blocked work on a slow or unreachable filesystem.

**Snapshot and later defenses:** inspection is a metadata snapshot and therefore has an inherent path TOCTOU window. The binding claim-time re-`Lstat` comparison and descriptor-open checks in later stories are independent defense layers; this story does not pretend the initial snapshot provides a lasting filesystem lock.

**Vocabulary scope:** `SessionID` and `CapabilityToken` are shared domain types only. Entropy, generation, lifecycle, authorization, and disclosure behavior remain owned by later stories.

**Filesystem/network boundary:** UNC paths are valid source inputs and are inspected through filesystem APIs wherever native Go can reach them. Application-owned LAN selection, listeners, discovery, and other network operations remain out of scope.

## Verification

**Toolchain:** Go is not on this shell's default PATH. Prefix with:
`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`

**Commands:**
- `go build ./...` — expected: exit 0
- `go vet ./...` — expected: exit 0
- `go test -race ./...` — expected: exit 0, new tests pass, `main_test.go` still passes
- `gofmt -l .` — expected: no output (ignore `node_modules`)
- `rg -n 'type SourcePort interface|func \([^)]*\) Inspect\(' internal --glob '*.go' --glob '!**/*_test.go'` — expected: one interface declaration in `internal/transfer`, one implementation in `internal/source`

**Verified 2026-08-23 on go1.26.7/windows/amd64:** Winget package `BrechtSanders.WinLibs.POSIX.UCRT` 16.1.0-14.0.0-r4 supplied GCC 16.1.0. With its `mingw64/bin` and Go on `PATH` plus `CGO_ENABLED=1`, `go test -race -count=1 ./...` passed for the full module, including `main_test.go`. `go build ./...`, `go vet ./...`, uncached ordinary tests, `gofmt -l .`, and `git diff --check` also passed. Native symlink fixtures skipped on this unprivileged Windows account; always-running synthetic final-component and ancestor-link cases passed, so those matrix outcomes do not depend on the skipped fixtures.

**Cross-compile evidence (2026-08-23):** the `internal/source` test package compiled successfully for both `linux/amd64` and `darwin/amd64` with `go test -c`, exercising the non-Windows platform helper build.

## Suggested Review Order

**Source validation boundary**

- Start here: layered guards classify one selection without opening its contents.
  [`source.go:33`](../../internal/source/source.go#L33)

- Syntactic prefixes preserve caller bytes while exposing every traversed ancestor.
  [`source.go:183`](../../internal/source/source.go#L183)

- Windows metadata fails closed and catches regular-mode reparse points.
  [`platform_windows.go:12`](../../internal/source/platform_windows.go#L12)

**Safe error boundary**

- Wrapped causes remain inspectable without entering routine error strings.
  [`errors.go:42`](../../internal/transfer/errors.go#L42)

- Classification accepts registered coded errors and safely collapses unknowns.
  [`errors.go:89`](../../internal/transfer/errors.go#L89)

- Public copy stays fixed, complete, and independent of adapter detail.
  [`errors.go:117`](../../internal/transfer/errors.go#L117)

**Shared contract surface**

- The coordinator owns the module's single source interface.
  [`ports.go:7`](../../internal/transfer/ports.go#L7)

- Staged metadata preserves sender-private path identity for later revalidation.
  [`types.go:22`](../../internal/transfer/types.go#L22)

**Defense evidence and handoff**

- Mutation-resistant coverage proves directory-mode reparse ancestors stop traversal.
  [`source_test.go:276`](../../internal/source/source_test.go#L276)

- A native junction verifies production Windows behavior without symlink privilege.
  [`platform_windows_test.go:82`](../../internal/source/platform_windows_test.go#L82)

- Wrapped public errors retain exact codes and disclose no cause detail.
  [`errors_test.go:66`](../../internal/transfer/errors_test.go#L66)

- Deferred handle-pinning work preserves the residual TOCTOU reasoning for later stories.
  [`deferred-work.md:43`](deferred-work.md#L43)
