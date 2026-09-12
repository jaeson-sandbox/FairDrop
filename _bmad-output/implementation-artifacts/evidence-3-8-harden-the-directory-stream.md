# Story 3.8 evidence

## Current status

Implementation started, not accepted. Baseline:
`6a366121fa9fbcd218935aa2119970e3b4e6913a`.

The owner approved retaining all assigned findings with smaller, separately
verified checkpoints following party mode ("yeah we can"). First checkpoint:
source/root/reader safety and archive names/modes/drain, with matching contracts.
Full local verification and mutations precede milestone commit/push; native CI
conclusions are read explicitly. Second checkpoint: network/coordinator cleanup,
retry fencing, and server timeout branches. Formal adversarial review and final
matrix audit follow implementation, not this planning discussion.

## Continuity and authorization

- Pre-existing working-tree edits were this session's Story 3.7 acceptance and
  Story 3.8 planning/party documents, not unexplained user changes. Preserve them.
- Story 3.7 is done after owner acceptance and all three jobs in Verify
  34679085296 succeeding. No production code differs from the recorded baseline
  at the start of Story 3.8 implementation.
- Frozen intent/matrix remains unchanged by party mode. Non-frozen execution and
  design notes carry the approved sequencing and narrower claims: identity from
  Prepare, not original Stage; unsnapshotted contents; no receiver-wide collision
  guarantee. No public UI/copy/dependency expansion is authorized.
- Party discussion used configured session-mode personas, not independent code
  reviewers; its notes are `_bmad-output/party-mode/story-3-8-2026-09-12.md`.
- Keep every scoped D-ID open until its corresponding verified closure exists.
  D-096/099/101/102 remain unfinished during the first checkpoint. Never mark the
  story complete merely because the folder-only checkpoint is green.

## Verification ledger

Checkpoint 1 implementation and full ordered local verification are complete;
native CI is pending. Tasks and deferred IDs remain open until native proof is
recorded. No cleanup implementation or independent whole-story review has run.

### Implementation and reasoning

- Added consumer-owned PreparedDirectory (Walk/Close) to SourcePort. The raw
  source implements the search-only pin; the existing selection decorator embeds
  SourcePort and promotes the added method without another path resolver.
  Payload preparation retains that capability and ZIP writing consumes it.
- The pin is acquired at Prepare, not Inspect/Stage. Fresh no-follow validation
  compares identity before walking the same validated handle. Contents remain
  unsnapshotted; no promise about original Stage identity or collision-free
  extraction. ZIP root timestamp remains staged metadata.
- Inspection reserves the future pin. Without that reservation, an unchanged
  tree could pass preflight and exceed the handle budget only after headers.
- Borrowed Read and release share one synchronization helper spanning real
  native reads, not merely a synchronized boolean. The reader test blocks actual
  Read and checks lock ownership; a separate production emitFile test observes
  revocation at the owned-close boundary. Prepared Close similarly joins Walk.
  These do not make blocked OS I/O interruptible; scheduler-dependent overlap
  observation alone is not offered as deterministic proof.
- Portable names share one host-independent predicate. Parent audit caught
  `NUL .txt`: Windows device stems need trailing-space trimming before extension
  classification. Ordinary Unicode/spaces and apostrophes/semicolons remain;
  nested names are refused rather than sanitized. Download header sanitization
  remains a separate boundary. ZIP modes are defaults, not source preservation.
- Source arithmetic/batch errors are phase-specific without remapping other
  coded errors. The archive drain now shares the 101-empty-read behavior of file
  and entry copies; a finite test reader independently ends at 102, so deleting
  the guard fails an assertion instead of hanging.
- Binding SPEC, contracts, architecture/spine/memlog and AGENTS were synchronized
  without regenerating managed context or dropping Story 3.7 protections.

### Matrix coverage (checkpoint 1 only)

| Frozen row | Executed targeted tests / remaining full-gate obligation |
| --- | --- |
| Healthy folder / modes | Existing empty/nested/Unicode/ZIP64 suites; TestArchiveEmitsExplicitPortableModes |
| Depth | TestDirectoryHandleBudgetIncludesAncestorsAndPreparedPin; TestLexicalHandleBudgetRefusesBeforeSearchOpen |
| Root replacement | TestPreparedDirectoryOwnsOnlyLazyPinAndRejectsReplacement; TestPreparedDirectoryNativeReplacementNeverReadsNewBytes; TestPreparedArchiveNativeRootReplacementIsRefused |
| Borrow lifetime | TestBorrowedReaderRevocationJoinsAnInFlightRead; TestWalkRevokesBorrowBeforeOwnedClose; TestPreparedCloseJoinsActiveWalk; TestPreparedDirectoryClosesWithoutWalkingAndOnPreparationFailure |
| Unsafe names | TestPortableArchiveSegmentsRejectAmbiguousReceiverNames; TestSourceRejectsPortableNamesDuringInspectionAndWalk; TestArchivePortableNamesAreRejectedAtBothBoundaries; TestPrepareValidatesSanitizedArchiveRootAndReleasesPin |
| Empty reads | TestEmptyReadGuardsFailOnRead101AndResetOnProgress (drain/entry/file, stalled/progress) |
| Arithmetic / batch | TestSourceArithmeticAndBatchFaultsArePhaseCorrect |
| Wedged cleanup / nested budgets / isolated server waits | Pending checkpoint 2; not covered by this milestone |

### Mutation proof

`scripts/verify-native-mutations.sh` now adds 16 named baselines and 18 injected
breaks for this checkpoint. Its default remains the complete native suite;
restoration includes prepared.go and the shared name predicate. Each verdict
requires a passing unskipped baseline plus the matching assertion in a failing
test/subtest. Existing POSIX volume-prefix mutation now targets the shared source
predicate; it no longer silently misses the deleted volumeQualified helper.

Local scoped run: **18/18 killed**, 16/16 baselines passed. Exact mutation labels,
target tests and assertion substrings are executable in the script. They cover
cap/reservation/lexical admission, prepared identity and production ZIP wiring,
pin closure/Walk ownership, borrowed lock/visitor-return wiring, source codes,
device stem/source/ZIP/root boundaries, both modes, and drain stall/reset.

Full logs (including failed attempts) are retained at
`C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-1/`:

- `mutations.log`: full local script correctly refused a skipped existing
  symlink baseline because this Windows account lacks symlink privilege. This
  is not a passing full mutation run. Native CI still requires that capability.
- `directory-mutations.log`: first scoped run stopped on an exact assertion
  mismatch (`want` versus the stream helper's `want code`). The underlying
  regression failed the intended test, but the verifier correctly refused the
  mismatched evidence. No killed claim was recorded for that attempt.
- `directory-mutations-2.log`: corrected scoped runner, 16 passing baselines and
  18 valid kills. The temporary runner extracts the same baseline/mutation blocks
  from the canonical script; no native default was weakened to accommodate this
  workstation.
- `full-gate.log` and per-gate logs: complete ordered gate passed, including Wails
  build, binding drift/.gitkeep, gofmt, vet, pinned staticcheck, uncached normal
  Go tests, CGO_ENABLED=1 and a compiled cgo probe, uncached race tests (every
  package `ok`), 498 frontend tests in 17 files, LF and diff checks. Darwin arm64
  build/vet/bare-staticcheck and Linux amd64 build/vet passed as preflight only.

Native Windows/macOS and Linux adapter conclusions: pending push/CI. Cross-build
checks, even when green, are not native proof.
