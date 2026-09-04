---
title: 'Story 2.2: Stream a Safe Directory ZIP'
type: 'feature'
created: '2026-09-03'
status: 'in-review'
review_loop_iteration: 0
baseline_commit: '6ba41368789c6bb2980c755928ab1222b4b58a01'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 2.1 made folders stageable, but `Payloads.Prepare` still refuses `ItemDirectory`, so a folder reaches a scanned QR then fails at download — under a staged note promising it downloads as a ZIP.

**Approach:** Extend `SourcePort` with a callback tree walk that owns every descriptor, and add a directory payload streaming `archive/zip` through `io.Pipe` with a worker joined on every exit.

## Boundaries & Constraints

**Always:** Reuse 2.1's no-follow handles. Content opens are parent-relative, no-follow; POSIX opens `O_NONBLOCK` and rejects by `fstat` before clearing it; Windows adds `FILE_READ_DATA` with `FILE_NON_DIRECTORY_FILE`. `internal/source` owns every handle and closes in reverse order; the visitor borrows a reader valid only for its call. Re-check every entry at stream time. Entry names are root-relative `filepath.ToSlash`, under one top-level directory, never absolute, volume-qualified, empty, dot-dot, or traversal-bearing. `zip.Writer.Close()` precedes pipe-writer close; `WriteTo` leaves no goroutine of its own and returns non-nil on truncation. `Size()` is `(0, false)`. Download name is the sanitized root plus `.zip`, capped after appending. Memory is O(buffer + depth), never O(entries). Contracts and architecture move with the port. File behavior stays byte-identical.

**Ask First:** Any change to the frozen HTTP header matrix, including the recorded CORS, `Accept-Ranges`, and `Access-Control-Expose-Headers` gaps. Any `SourcePort` change beyond adding the walk. Weakening no-follow, identity, cancellation, memory, or path-preservation rules.

**Never:** Create a temporary archive, retain a per-entry index, read a payload into memory, or size a buffer from the tree. Use `filepath.WalkDir`, `ReadDir(-1)`, a shell, or destructive path cleaning. Declare a shadow source interface. Append an error body after headers. Change the frontend — its unknown-total path is already correct.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Safe tree | Nested dirs, files, empty dirs, empty files | One top-level root; empties representable; valid central directory | N/A |
| Empty root | No entries | Valid ZIP holding only the root entry | N/A |
| Unsafe entry mid-stream | Becomes link, junction, reparse, special | Abort before emitting it; no later entries | `path_unsupported`; abort, no error body |
| Entry lost | Removed or unreadable while streaming | Abort; every owned handle closed | `path_not_found`/`path_unsupported`; no path disclosed |
| Root changed at claim | Now missing, a file, or link-like | Refuse before headers | `source_changed`/`path_unsupported`/`path_not_found`; generic `410` |
| Cancel or disconnect | Context cancelled, or destination fails | Prompt return; worker joined; pipe ends and ZIP writer closed | `cancelled`; nothing appended after |
| Name | Sanitizes to empty, or sits at the rune cap | `download.zip`; capped after `.zip` | N/A |
| Regular file | Staged `ItemFile` | Unchanged, including known `Content-Length` | Unchanged |

</frozen-after-approval>

## Code Map

- `internal/transfer/ports.go:17` — `SourcePort`; add walk + visitor/entry types. `contracts.md:14` names consumers "coordinator plus adapters"; `:19` forbids a shadow port.
- `internal/source/source.go:34,188` — reuse anchor walk, `verifyOpened:298`, `rejectUnsupportedInfo:318`, `statChecked:335`, frame stack, `ReadDir(1)`, cycle check. Only the size arm `:257` is kind-specific. Missing: relative-name accumulation, per-entry emitter, content open.
- `internal/source/handle.go:17` — no `Read` on any handle. Rights: `handle_windows.go:226`, `handle_linux.go:7`, `handle_darwin.go:9`.
- `internal/stream/archiver.go` — `:104` directory guard, first of all; `:240` lifecycle to mirror (`streamed` CAS, `closeOnce` + owner, repeat `Close` nil); `:218` bare `Lstat`; `:421` cap applied *before* `TrimRight`, so `.zip` overflows. Rename per retro item 5.
- `internal/server/handler.go:136,164,185,247` — read-only: unknown size omits `Content-Length`, skips the length recheck, aborts via `http.ErrAbortHandler`.
- `internal/stream/archiver_test.go:1379,1437`, `memory_test.go:19` — `syntheticAdapter`, `fakeFile`, `TotalAlloc`-delta proof.
- `internal/source/source_test.go:711` — `fakeFactory`; `onOperation:717` mutates mid-run, closing the `source.go:174` item.
- `docs/fairdrop-contracts.md:131,209`, `docs/fairdrop-architecture.md:129` — binding; update with the port.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/ports.go` — add walk, visitor, entry types — contract names adapters as consumers, forbids a shadow port.
- [x] `internal/source/handle*.go` — parent-relative no-follow content open per platform — handles provably cannot read bytes; POSIX must dodge the FIFO block.
- [x] `internal/source/source.go` — one traversal for inspection and streaming, carrying a relative name per frame.
- [x] `internal/stream/` — rename `archiver.go`; add the directory payload: lazy `Prepare`, `io.Pipe`, joined worker, `zip.Writer.Close()` before pipe close.
- [x] `internal/stream` claim path — re-apply the reparse check to the claim-time `Lstat` — closes the entry promising `path_unsupported`, not `source_changed`.
- [x] tests — memory bound constant in tree size; goroutine exit; close ordering; central-directory validity; the `source.go:174` pin via `onOperation`.
- [x] `docs/fairdrop-{contracts,architecture}.md`, `deferred-work.md`, `sprint-status.yaml` — record port, guarantees, ownership.

**Acceptance Criteria:**
- Given a staged directory, when the receiver downloads, then a browser-openable ZIP arrives with one top-level root and a valid central directory, no `Content-Length`, and progress reporting `totalKnown=false`.
- Given any exit — success, cancel, disconnect, unsafe entry, read failure — when `WriteTo` returns, then no goroutine of its own runs, both pipe ends and the ZIP writer are closed, and a later `Close` neither races nor double-releases.
- Given a wide or deep tree, when it streams, then retained memory is bounded by buffer plus depth, does not grow with entry count, and no temporary archive exists on disk.
- Given each load-bearing guarantee is deliberately broken, when its focused mutation runs, then a named behavioral test fails rather than compilation or an unrelated assertion.

## Implementation Evidence

Gates on this worktree: `gofmt -l .` clean; `go vet ./...` 0; `go test -count=1 ./...` 0
across three consecutive runs; `go test -count=1 -race ./...` 0; frontend Vitest 17 files /
490 tests unchanged; frontend production build; `wails build` exit 0.

Matrix coverage, every test executed and passing: safe tree
(`TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory`, plus
`TestStreamedArchiveOpensWithASecondImplementation`); empty root
(`TestWriteToArchivesAnEmptyRootAsAFolder`); unsafe entry mid-stream
(`TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream`); entry lost
(`TestWriteToPropagatesAWalkFailureWithoutAppendingToTheBody/missing`,
`TestWalkPropagatesAContentOpenFailureWithItsCode`); root changed at claim
(`TestPrepareRejectsARootThatIsNoLongerADirectory`, `...ThatDisappeared`,
`...ALinkLikeRootWithPathUnsupported`); cancel or disconnect
(`TestWriteToStopsPromptlyWhenTheReceiverDisconnects`,
`TestPrepareHonorsCancellationForADirectory`); name
(`TestArchiveDownloadNameIsCappedAfterTheExtensionIsAppended`); regular file
(the unchanged `payload_test.go` suite).

Mutations run against the new guarantees; each failed through the named test and was
restored:

| Deliberate break | Named failing test |
| --- | --- |
| Return from `WriteTo` without joining the worker | `TestWriteToPropagatesAWalkFailureWithoutAppendingToTheBody` |
| Close the pipe writer before `zip.Writer.Close()` | `TestStreamedArchiveOpensWithASecondImplementation` |
| Skip `halt()` so a failed stream still yields a valid archive | `TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream` |
| Report a known total from `Size()` | `TestPrepareDirectoryIsLazyAndReportsAnUnknownLength` |
| Leave the pipe read end open after drain | `TestWriteToStopsPromptlyWhenTheReceiverDisconnects` |
| Admit a dot-dot segment in an entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/dot-dot` |
| Admit a backslash or NUL in an entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/backslash` |
| Admit a volume-qualified entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/volume_qualified` |
| Stop placing entries under the single root | `TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory` |
| Retain one record per entry while streaming | `TestArchiveRetainedMemoryDoesNotGrowWithEntryCount` |
| Drop the withheld-stop diagnostic | `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer` |

Two survivors were examined and are not defects. Removing the absolute-prefix check in
`archiveEntryName` is an equivalent mutant: a leading `/` splits to an empty first segment,
which the segment check already refuses, and removing both together is caught. The bounded
memory assertion was itself mutation-checked after a first version proved dead -- a 6 MiB
ceiling admitted the ~4 MiB a retained index costs, so it was tightened to 2 MiB against a
measured ~0.8 MiB baseline and now fails the retained-index mutation.

Out of spec, recorded rather than hidden: `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer`
was flaky from Epic 1 and is now diagnosed and fixed. It spawned a `Cancel` racing the reset
arming and then asserted a diagnostic that is only recorded when the race resolves one way;
at `-count=200` it failed twice with "0 diagnostics recorded". The race assertion is now its
own test asserting only what holds under both orderings, and the diagnostic assertion is
deterministic. 1000 runs clean, and the diagnostic assertion still fails when the branch it
covers is removed.

## Spec Change Log

## Design Notes

The visitor borrows its reader: `source` opens, hands an `io.Reader` valid only for the callback, and closes in reverse order on every exit. That keeps 2.1's unwind discipline and stops `stream` owning a descriptor it cannot release, at the cost of holding the depth-bounded stack open for the whole response.

POSIX content open is the sharp edge: `O_RDONLY|O_NOFOLLOW` on a FIFO blocks forever and the kind check runs after the open. Open `O_NONBLOCK`, `fstat`, reject non-regular, then clear.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go test -count=1 ./...`; `go test -count=1 -race ./...` — clean
- `cd frontend && npm test && npm run build` — 490 unchanged; `wails build` — exit 0
- Greps: one production `SourcePort`; no `archive/zip` outside `internal/stream`; no `WalkDir`, `ReadDir(-1)`, or temp-file creation in the stream path
- Unzip with a second tool and with `archive/zip` to prove central-directory validity
- Record assertion-level mutations for entry-name relativity, one-top-level-root, close ordering, worker join, per-entry revalidation, the FIFO guard, and the `.zip` cap; restore after each
