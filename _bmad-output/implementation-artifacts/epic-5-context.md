# Epic 5 Context: Make the Transfer Coordinator Legible

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 5 makes the transfer coordinator readable: a maintainer should be able to find the code that owns a concern without scrolling through a thousand-line file first. `internal/transfer/coordinator.go` is the largest production file in the repository and the one every lifecycle change touches, and it holds several concerns that have nothing to do with coordination. Two consecutive retrospectives named the split and deferred it; a later unspecced branch then did roughly a third of it anyway, which is exactly the pattern the project's 2026-09-17 governance rule now forbids. This epic changes no behaviour, no contract, and no public surface — it moves code between files inside one package, and it is the first work to run under that governance rule.

## Stories

- Story 5.1: Split internal/transfer/coordinator.go

## Requirements & Constraints

- **Behaviour is frozen.** No renames, no signature changes, no altered behaviour, no restructuring of the coordination logic itself. This is a file-level move within the same package and nothing else.
- **Zero test edits.** The full suite must pass with no change to any `_test.go` file. The moved code stays in package `transfer`, so a move that forced a test change was not a move — it was a redesign.
- **Public surface is byte-identical.** Package documentation output must be unchanged, and no exported identifier may be added, removed, or renamed.
- **A measurable target.** `coordinator.go` ends under 750 lines and retains only the coordination itself; each extracted file is named for the single concern it owns and reads as a unit in one sitting.
- **Comments travel with their code.** The commentary that explained each region inside the large file moves alongside it rather than being left behind or rewritten.
- **Full story ceremony applies.** Because this changes production code, it produces a spec, three review layers, and an evidence file — this is the rule's first exercise, so cutting the ceremony would undercut the epic's own premise.

## Technical Decisions

- **What is separable, and what is not.** The self-contained regions are the bounded diagnostics ring buffer with its overflow sentinel, the session state machine and its resource ledger, identity and capability-URL construction, and the two warning constructors. The coordination logic — staging, claim authorization, and the unwind/failure family — is explicitly out of scope; splitting it is a different and riskier story.
- **Precedent already set.** The bounded-call subsystem was previously lifted into its own file in the same package, so the shape of a correct extraction already exists in the tree; follow it rather than inventing a new layout.
- **The package boundary is load-bearing.** `internal/transfer` owns the coordinator, state, session, lifecycle ports, and domain errors and events. Interfaces live with their consumer, the coordinator imports no Wails API and no concrete adapter, and no new public interface may appear. Extraction must not push a concern into another package or create a new import edge.
- **Concurrency discipline must survive intact.** The coordinator is the single serializer of transfer state: a mutex guards state and session identity, no external port is called while locked, and every callback and timer is validated against the current session ID. Session-scoped resource ownership runs through one per-session operation lease with reverse-order unwind and bounded waits. Moving these declarations between files must not reorder initialization, change lock scope, or separate a guard from what it guards.
- **Conventions to preserve verbatim while moving:** lowercase package nouns for files and identifiers, `context.Context` as first parameter, `%w` wrapping with typed stable codes and safe user messages, and the rule that no absolute path or capability token reaches a diagnostic or any surface outside the process.
- **Timing constants are not yours to touch.** Several bounds in this file are pinned by tests to their own literals and are coupled by documentation rather than by code to bounds in the server package. Carry values across unchanged.

## Cross-Story Dependencies

- Inherited from the Epic 3 and Epic 4 retrospectives: this epic exists to close a twice-deferred action item, and the Epic 3 exclusion that originally deferred it is itself flagged for amendment now that part of the split has already shipped.
- **No open deferred-work entry depends on this file**, and the constraint runs the opposite way from what it looks like. Of 119 ledger entries, two mention the coordinator (D-099 by name, D-103 with line numbers) and both are `discharged`. Their evidence describes what was true when it was written, so re-pointing those line numbers at moved code would falsify a closed record rather than maintain it. The same holds for the line-numbered citations in the Epic 1 and 3 retrospectives and the Story 3.5 and 1.5 artifacts: historical documents, left alone. If the split ever does touch a *live* pointer, prefer naming the function over re-numbering the line.
- No dependency on any in-flight product work: this epic covers no functional requirement and shares no mechanism with the selection-experience work in Epic 4.
