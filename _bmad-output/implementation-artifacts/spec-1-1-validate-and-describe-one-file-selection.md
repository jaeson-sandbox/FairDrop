---
title: 'Story 1.1 — Validate and Describe One File Selection'
type: 'feature'
created: '2026-08-23'
status: 'in-review' # draft | ready-for-dev | in-progress | in-review | done
review_loop_iteration: 0
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
- `internal/source/source.go` — **new**, the file-only `SourcePort` adapter.
- `internal/{network,stream,server}/*.go` — Phase 1 provider-owned placeholders. **Read only, do not edit.**
- `go.mod` — module `fairdrop`, `go 1.25.0`. Must not gain a dependency.
- `main.go`, `app.go`, `main_test.go` — untouched; `main_test.go` must keep passing.

Only the types the Tasks below name are in scope. `FileMetadata`, `ProgressSnapshot`, and the other ports belong to later stories.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/types.go` — `SessionID`, `CapabilityToken`, `ItemKind` (`ItemFile`, `ItemDirectory`), `StagedItem`. Vocabulary every later story imports.
- [x] `internal/transfer/errors.go` — `ErrorCode` + twelve constants, `PublicError`, `CodedError`, `DomainError` with `Unwrap`, `NewError`/`WrapError`/`ErrorCodeOf`/`PublicErrorOf`. One contract so no adapter invents its own.
- [x] `internal/transfer/ports.go` — `SourcePort`, consumer-owned.
- [x] `internal/source/source.go` — file-only impl via `Lstat` + `Mode().IsRegular()`.
- [x] `internal/source/source_test.go` — every matrix row; skip uncreatable fixtures rather than failing.
- [x] `internal/transfer/errors_test.go` — `%w` wrapping, code preservation, unknown fallback, safe-message content.

**Acceptance Criteria:**
- Given a wrapped `DomainError`, when `PublicErrorOf` reads it, then `Code` matches the original and `Message` is the exact registry string for that code, with no path, basename, or token.
- Given each of the twelve `ErrorCode` values, when `PublicErrorOf` maps it, then the message matches the UX copy registry character-for-character.
- Given the finished package, when its exported surface is compared to `docs/fairdrop-contracts.md`, then names and meanings match and no second source interface exists in the module.
- Given the module, when `go build ./...`, `go vet ./...`, `go test -race ./...`, and `gofmt -l .` run, then all pass and `main_test.go` still holds.

## Spec Change Log

## Design Notes

**Measured on go1.26.7/windows/amd64 — a symlink-bit check is wrong here.** `os.Lstat` on a Windows junction reports `ModeIrregular`, *not* `ModeSymlink`, so `mode&os.ModeSymlink != 0` silently accepts junctions:

```
junction.lnkdir   symlink=false  irregular=true   IsRegular=false
realdir           symlink=false  IsDir=true       IsRegular=false
NUL               symlink=false  device=true      IsRegular=false
```

`Mode().IsRegular()` is `mode&ModeType == 0`, so one check rejects all three plus symlinks. Prefer it over enumerating bits.

**Fixtures:** `os.Symlink` needs elevation or Developer Mode on Windows and fails for an ordinary user; junctions (`mklink /J`) do not. Guard symlink fixtures with a skip so the suite stays green unprivileged.

**Classification:** `errors.Is(err, fs.ErrNotExist)` → `path_not_found`; any other `Lstat` failure → `path_unsupported`, which the contract defines to include host-unsupported paths.

## Verification

**Toolchain:** Go is not on this shell's default PATH. Prefix with:
`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`

**Commands:**
- `go build ./...` — expected: exit 0
- `go vet ./...` — expected: exit 0
- `go test -race ./...` — expected: exit 0, new tests pass, `main_test.go` still passes
- `gofmt -l .` — expected: no output (ignore `node_modules`)
- `grep -rn "Inspect(" internal/ --include=*.go` — expected: one interface declaration in `internal/transfer`, one implementation in `internal/source`
