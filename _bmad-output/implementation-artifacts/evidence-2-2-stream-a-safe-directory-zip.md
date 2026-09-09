# Evidence: Story 2.2: Stream a Safe Directory ZIP

Moved out of the spec on 2026-09-08 so the spec stays at its intended size. Nothing here was
edited in the move; section order is preserved.

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

