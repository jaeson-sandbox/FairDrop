---
title: 'Story 2.1: Validate and Stage One Directory'
type: 'feature'
created: '2026-09-01'
status: 'in-review'
review_loop_iteration: 0
baseline_commit: 'a7285e015826a87d1a962d53a96dcadfe1be4771'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop exposes folder selection but rejects directories, and temporarily describes a dropped folder as a file. This blocks Epic 2 and makes the UI promise false.

**Approach:** Extend `SourcePort` with bounded-batch directory preflight and no retained index. Reuse the coordinator transaction and show neutral pending copy until a dropped path's kind is known.

## Boundaries & Constraints

**Always:** Preserve the caller's path byte-for-byte; inspect root, ancestors, and nested entries with cancellation checks, `Lstat`, and native reparse detection. Before enumerating a directory, prove its opened-handle identity matches the inspected object. Source traversal sums non-negative regular-file sizes with `int64` overflow checks; the coordinator rejects every file or directory total outside `0..9007199254740991` before acquiring resources. Return root name/modtime and `ItemDirectory`; close every reader; bound memory by depth plus one fixed batch. Errors preserve coded causes without paths. Ordinary file staging and reverse unwind remain unchanged.

**Ask First:** Any exported contract or dependency change; weakening identity, batching, link/reparse refusal, cancellation precedence, representability, or path-preservation rules.

**Never:** Implement ZIP preparation/streaming, claim-time revalidation, server/progress changes, or temporary archives in this story. Do not use `filepath.WalkDir`, read-all `ReadDir(-1)`, shell commands, destructive path cleaning, a retained entry list, or a shadow source interface.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Safe tree | Nested or empty directory | Original path, basename, root modtime, `ItemDirectory`, exact sum (`0` when empty); no retained handles/index | N/A |
| Unsafe entry | Selected/nested symlink, junction/reparse point, or special file | Stop before descending/following or visiting later entries | `path_unsupported`; no path disclosure |
| Metadata race | Entry disappears, becomes unreadable, or checked directory is replaced before open | Abort, close readers, and never enumerate a mismatched handle | `path_not_found`, `path_unsupported`, or `source_changed`; no path disclosure |
| Cancellation | Cancelled before or during open/read/stat/reparse | Cancellation wins immediately after the active syscall and traversal stops | `cancelled`, preserving cause |
| Invalid size | Negative entry, `int64` overflow, or any item total above `9007199254740991` | Source rejects arithmetic faults; coordinator rejects unrepresentable metadata before setup | `transfer_failed` |
| Native paths | Spaces, Unicode, `..`, supported long path or reachable UNC root | Native APIs; source spelling preserved | Skip only when capability is genuinely absent |
| Trailing separator | Directory or Windows junction selected with trailing separators | Real directory stages; junction cannot bypass reparse refusal | `path_unsupported` for link-like root |

</frozen-after-approval>

## Code Map

- `internal/source/source.go:52-497` -- validate files/directories, traverse with bounded readers, revalidate opened identities and link status, detect active-stack cycles, and preserve metadata-only trailing-separator handling.
- `internal/source/{source_test.go,platform_windows_test.go}` -- preserve file regressions and pin the matrix through production defaults plus deterministic seams.
- `internal/transfer/{coordinator.go,coordinator_stage_test.go}` -- enforce one pre-resource safe-integer invariant for both kinds; pin boundaries, directory forwarding, and unwind.
- `frontend/src/ui/{copy.ts,StagePendingCard.tsx}` and tests -- register/use “Preparing your item…” only for unknown native-drop kind; file/folder browse copy remains specific.
- `EXPERIENCE.md` and `docs/fairdrop-{architecture,contracts}.md` -- register pending copy and record traversal, identity, representability, and errors.
- `_bmad-output/implementation-artifacts/{deferred-work.md,sprint-status.yaml}` -- reroute claim-time findings to Story 2.2 and keep execution status accurate.

## Tasks & Acceptance

**Execution:**
- [x] `internal/source/source.go` -- implement bounded, cancellation-aware directory preflight while preserving the file path and existing port.
- [x] `internal/source/{source_test.go,platform_windows_test.go}` -- cover the matrix with defaults and seams; record the named failure for each mutation.
- [x] `internal/transfer/{coordinator.go,coordinator_stage_test.go}` -- enforce representable metadata for both kinds before setup; pin directory forwarding and reverse unwind.
- [x] `frontend/src/ui/{copy.ts,copy.test.ts,StagePendingCard.tsx,StagePendingCard.test.tsx}` -- make unknown-kind pending copy truthful and accessible.
- [x] `docs/fairdrop-contracts.md`, `EXPERIENCE.md`, `deferred-work.md`, `sprint-status.yaml` -- preserve decisions, ownership, and execution state for the next agent.

**Acceptance Criteria:**
- Given a safe directory, when Stage completes, then exact metadata, URL, QR, session, and warnings return with `isDir=true`, while ordinary file behavior is unchanged.
- Given wide/deep traversal or any failure exit, when inspection ends, then fixed-size batch assertions and ownership counters prove no read-all call, retained index, or leaked handle.
- Given a directory substitution or unrepresentable item total, when inspection/Stage reaches its boundary, then it fails before enumeration/resource acquisition and closes owned handles.
- Given each safety invariant is deliberately broken, when its focused mutation runs, then a named test fails for that invariant rather than for compilation or an unrelated assertion.

## Spec Change Log

- 2026-09-02: Implemented directory preflight, safe-integer staging, neutral native-drop copy, tests, and durable contract/ownership documentation.
- 2026-09-02: Recorded assertion-level mutation evidence and restored review status; this prevents a green compile failure from masquerading as behavioral proof.

## Design Notes

Use `Lstat → reparse check → open → handle Stat → SameFile → ReadDir(n)` with fixed positive batches and a depth-first reader stack. This is O(depth + batch), independent of payload bytes and total entries. Trim trailing separators only for metadata lookup beyond the volume/share root; never clean `.`/`..` or alter the returned path. Preflight is not a snapshot: Story 2.2 repeats claim/stream checks.

## Verification

**Commands:**
- `gofmt -w internal/source/source.go internal/source/source_test.go internal/source/platform_windows_test.go internal/transfer/coordinator.go internal/transfer/coordinator_stage_test.go && gofmt -l .` -- no unformatted Go.
- `go test ./... && go vet ./... && go test -race ./...` -- all Go gates pass with complete output.
- `cd frontend && npm test && npm run build` -- frontend tests and build pass.
- `wails build` -- the native application builds; Story 2.2 remains responsible for a successful directory download smoke test.
- `rg -n 'type SourcePort interface' --glob '*.go' --glob '!**/*_test.go' .` plus a no-match search for `type (NetworkManager|Streamer|TransferServer|TransferStats)` with the same globs -- one source contract and no retired production declarations.

## Mutation Evidence

Every mutation below failed through the named behavioral assertion, not compilation or an unrelated test, and was then restored:

| Mutation | Named failing test |
| --- | --- |
| `directoryReadBatchSize: 1 → 0` | `TestInspectDirectoryUsesOneFixedPositiveBatchAndClosesEveryReader` |
| Remove opened-identity mismatch refusal | `TestInspectDirectoryRefusesChangedOpenedIdentityBeforeEnumeration` |
| Disable selected-path reparse refusal | `TestInspectRejectsRegularModeReparsePoint` |
| Remove post-reparse cancellation check | `TestInspectDirectoryCancellationWinsAfterEachActiveOperation/reparse_check` |
| Disable negative/overflow checks | `TestInspectDirectoryRejectsNegativeAndOverflowingLogicalSizes/{negative,overflow}` |
| Permit `maxSafeInteger + 1` | `TestStageRejectsUnrepresentableMetadataBeforeResourceAcquisition/{oversize_file,oversize_directory}` |
| Bypass dot/dot-dot display-name cases | `TestInspectDirectoryUsesInspectedNameForDotAndDotDotSelections` |
| Disable post-open reparse refusal | `TestInspectDirectoryRevalidatesLinkStatusAfterOpenBeforeEnumeration` |
| Disable active-ancestor cycle comparison | `TestInspectDirectoryRejectsOpenedAncestorCycleAndClosesBoundedStack` |
| Replace production `os.SameFile` with `true` | `TestInspectDirectoryDefaultSameFileRejectsDifferentOpenedDirectory` |

## Suggested Review Order

**Directory safety and bounded traversal**

- Start with the source boundary that preserves spelling and resolves directory metadata.
  [`source.go:52`](../../internal/source/source.go#L52)

- Follow the depth-first reader stack, fixed batch, size checks, and cycle refusal.
  [`source.go:204`](../../internal/source/source.go#L204)

- Review post-open identity and link/reparse revalidation before enumeration.
  [`source.go:330`](../../internal/source/source.go#L330)

**Coordinator and UI boundaries**

- Confirm unsafe JavaScript integer sizes fail before any resource acquisition.
  [`coordinator.go:328`](../../internal/transfer/coordinator.go#L328)

- Confirm unknown native drops use neutral pending copy without a kind tab.
  [`StagePendingCard.tsx:25`](../../frontend/src/ui/StagePendingCard.tsx#L25)

- Verify the neutral phrase is registered in the approved copy catalog.
  [`copy.ts:42`](../../frontend/src/ui/copy.ts#L42)

**Contracts and regression evidence**

- Read the durable traversal, identity, error, and representability contract.
  [`fairdrop-contracts.md:209`](../../docs/fairdrop-contracts.md#L209)

- Inspect exact depth, default identity, substitution, cycle, and failure tests.
  [`source_test.go:220`](../../internal/source/source_test.go#L220)

- Finish with directory forwarding and pre-resource boundary coverage.
  [`coordinator_stage_test.go:796`](../../internal/transfer/coordinator_stage_test.go#L796)
