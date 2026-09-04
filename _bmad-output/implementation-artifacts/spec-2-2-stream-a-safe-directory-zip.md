---
title: 'Story 2.2: Stream a Safe Directory ZIP'
type: 'feature'
created: '2026-09-03'
status: 'done'
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

**Always:** Reuse 2.1's no-follow handles. Content opens are parent-relative, no-follow; POSIX opens `O_NONBLOCK` and rejects by `fstat` before clearing it; Windows adds `FILE_READ_DATA` with `FILE_NON_DIRECTORY_FILE`. `internal/source` owns every handle and closes in reverse order; the visitor borrows a reader valid only for its call. Re-check every entry at stream time. Entry names are root-relative `filepath.ToSlash`, under one top-level directory, never absolute, volume-qualified, empty, dot-dot, or traversal-bearing. `zip.Writer.Close()` precedes pipe-writer close; `WriteTo` leaves no goroutine of its own and returns non-nil on truncation. `Size()` is `(0, false)`. Download name is the sanitized root plus `.zip`, capped after appending. Memory is O(buffer + depth + one central-directory record per entry), never O(payload bytes) and never a second per-entry index of this story's own. Contracts and architecture move with the port. File behavior stays byte-identical.

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
- Given a wide or deep tree, when it streams, then live heap during the walk stays within the format's own per-entry central-directory cost, does not grow with payload bytes, and no temporary archive exists on disk.
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
| Close the pipe writer before `zip.Writer.Close()` | `TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory` (cited over `TestStreamedArchiveOpensWithASecondImplementation`, which skips when no second ZIP tool is on PATH) |
| Skip `halt()` so a failed stream still yields a valid archive | `TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream` |
| Report a known total from `Size()` | `TestPrepareDirectoryIsLazyAndReportsAnUnknownLength` |
| Leave the pipe read end open after drain | `TestWriteToStopsPromptlyWhenTheReceiverDisconnects` |
| Admit a dot-dot segment in an entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/dot-dot` |
| Admit a backslash or NUL in an entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/backslash` |
| Admit a volume-qualified entry name | `TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot/volume_qualified` |
| Stop placing entries under the single root | `TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory` |
| Retain one record per entry while streaming, rooted in the payload | `TestArchiveRetainedMemoryDoesNotGrowWithEntryCount` |
| Retain one record per entry in a worker local | `TestArchiveRetainedMemoryDoesNotGrowWithEntryCount` |
| Drop the withheld-stop diagnostic | `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer` |

Two survivors were examined and are not defects. Removing the absolute-prefix check in
`archiveEntryName` is an equivalent mutant: a leading `/` splits to an empty first segment,
which the segment check already refuses, and removing both together is caught. The bounded
memory assertion went through three versions, each killed by mutation rather than by argument. A
6 MiB ceiling admitted the ~4 MiB a retained index costs. Tightening it to 2 MiB caught an index
rooted in the payload but still missed one held in a worker local, because it read the heap after
`WriteTo` returned, by which point that local is unreachable. It now samples inside the walk at the
last entry against a per-entry budget, and catches both variants.

Out of spec, recorded rather than hidden: `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer`
was flaky from Epic 1 and is now diagnosed and fixed. It spawned a `Cancel` racing the reset
arming and then asserted a diagnostic that is only recorded when the race resolves one way;
at `-count=200` it failed twice with "0 diagnostics recorded". The race assertion is now its
own test asserting only what holds under both orderings, and the diagnostic assertion is
deterministic. 1000 runs clean, and the diagnostic assertion still fails when the branch it
covers is removed.

## Formal Review Triage

Three context-free layers reviewed the diff. Findings were deduplicated by claim and action, then
routed. Two required a human decision and were taken to the user rather than inferred.

- **Intent gap, resolved by amendment:** the frozen memory bound was unachievable. See the change log.
- **Human decision, kept deliberately:** `Inspect` refusing a POSIX-legal backslash entry name. Kept so a
  folder fails at selection rather than at download, and recorded at the `Inspect` boundary in the
  contract.
- **Patched:** the reintroduced shell fixture, which interpolated a caller path into a PowerShell
  `-Command` string -- Story 2.1 removed the last such fixture and recorded that the no-shell rule covers
  verification code, its own triage drawing the line at "no command shell or interpolated path"; the path
  now reaches PowerShell through the environment. Also: the contract paragraphs that were false for a
  directory (`Size` as a bound, and the `Lstat`-plus-`SameFile` identity claim); the undocumented
  halt-on-failure invariant; two Windows mask assertions that implied a separation the API does not make,
  since `FILE_LIST_DIRECTORY` and `FILE_READ_DATA` are the same bit; `SourceEntry.Size` and directory
  `ModTime`, both of which could be zeroed with the suite green; the archive's empty-read guard, which had
  no test and whose removal now hangs a named test; an inert fixture line and an `err != nil` assertion
  that any failure satisfied; a dead test helper; and a comment naming the wrong rune.
- **Deferred with owners:** native POSIX execution of the content-open guards, a traversal depth bound,
  large-archive and ZIP64 validation, the entry-name versus download-name hardening asymmetry, borrowed
  reader goroutine safety, per-entry file modes, root identity across the Prepare-to-WriteTo window, a
  handler-level `Content-Length` proof, and the host-dependent name predicates.
- **Accepted:** the post-open regular-file recheck in `emitFile`, which `verifyOpened` already makes
  unreachable. Kept as defence in depth and recorded so it is not re-derived as a gap.

Gates were rerun after the patches: `gofmt` clean, `go vet` 0, `go test` 0, `-race` 0, frontend 490
unchanged, `wails build` exit 0 producing `build/bin/fairdrop.exe`.

## Spec Change Log

- 2026-09-04: The frozen memory bound was unachievable and is amended with human approval. "Memory is
  O(buffer + depth), never O(entries)" cannot hold for any streamed ZIP: the format writes its central
  directory at the end, so `archive/zip` retains one record per entry -- measured at a steady ~248 bytes
  from 10,000 entries upward, about 12 MiB at fifty thousand. The bound now names that cost explicitly and
  forbids what this story can actually control: a second per-entry index of its own, and any growth with
  payload bytes. The known-bad state avoided is a criterion no correct implementation could ever satisfy,
  which a green test can only be hiding. **KEEP:** the pipe-and-worker design, the close ordering, the halt
  on failure, and the per-entry budget assertion, which catches both a payload-rooted and a worker-local
  index.
- 2026-09-04: Recorded out-of-spec, human-approved: `Inspect` now refuses an entry name containing a
  backslash, which is legal on POSIX, so a folder that staged under Story 2.1 no longer stages. Kept
  deliberately rather than relaxed, because failing at selection is the behaviour the Epic 1 retrospective
  demanded over offering a folder and refusing it at download. The contract records it at the `Inspect`
  boundary rather than only under `Walk`.
- 2026-09-04: Also out of spec and recorded rather than hidden:
  `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer` was flaky from Epic 1, diagnosed here,
  and split so the race assertion and the diagnostic assertion no longer share a test.

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

## Suggested Review Order

**The port and who owns the descriptors**

- Start here: the visitor borrows a reader the source closes, so stream owns no handle.
  [`ports.go:40`](../../internal/transfer/ports.go#L40)

- The walk contract: re-validated per entry, no index, one handle per active depth.
  [`ports.go:54`](../../internal/transfer/ports.go#L54)

**Reading bytes safely, which Story 2.1 never did**

- One traversal serves both inspection and streaming; only the size arm differs.
  [`source.go:69`](../../internal/source/source.go#L69)

- POSIX opens content non-blocking and rejects by fstat, so a FIFO cannot park the response.
  [`handle_posix.go:74`](../../internal/source/handle_posix.go#L74)

- Windows asks the kernel to refuse a directory rather than checking afterwards.
  [`handle_windows.go:301`](../../internal/source/handle_windows.go#L301)

- Entry names are built from the walk, never reconstructed from a path.
  [`source.go:454`](../../internal/source/source.go#L454)

**The archive, and what happens when it fails**

- The worker is joined on every exit path, including the early cancellation returns.
  [`archive.go:63`](../../internal/stream/archive.go#L63)

- zip.Writer.Close precedes the pipe close so the central directory reaches the receiver.
  [`archive.go:135`](../../internal/stream/archive.go#L135)

- On failure the writer is halted first: a broken stream must not read as a complete archive.
  [`archive.go:245`](../../internal/stream/archive.go#L245)

- The last gate before a name goes into the archive.
  [`archive.go:370`](../../internal/stream/archive.go#L370)

**Claim time, the last moment a failure can still pick a status**

- The re-Lstat now carries the reparse refusal, closing a deferred entry from Story 1.3.
  [`payload.go:247`](../../internal/stream/payload.go#L247)

**Peripherals**

- Bounded memory, sampled inside the walk because measuring afterwards proved nothing.
  [`archive_memory_test.go:37`](../../internal/stream/archive_memory_test.go#L37)

- The empty-read guard, whose removal now hangs a named test instead of nothing.
  [`archive_stall_test.go:34`](../../internal/stream/archive_stall_test.go#L34)

- Size and ModTime, both of which were zeroable with the suite green.
  [`walk_metadata_test.go:21`](../../internal/source/walk_metadata_test.go#L21)

- The contract's directory carve-outs for identity pinning and the Size bound.
  [`fairdrop-contracts.md:256`](../../docs/fairdrop-contracts.md#L256)
