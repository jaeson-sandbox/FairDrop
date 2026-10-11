---
id: SPEC-multiple-selected-items
companions:
  - selection-contract.md
  - ../spec-fairdrop/SPEC.md
sources: []
---

# Multiple selected items

## Why

Sending several items currently requires separate sessions or manually assembling a folder. A sender should be able to choose the items once and let one receiver download a streamed ZIP.

## Capabilities

- **CAP-1**
  - **intent:** A sender can select several local files and folders for one handoff.
  - **success:** Native multi-drop and accessible selection controls produce an editable, memory-only list of at most 10,000 items. Sending validates every item atomically before opening a listener. Single-item transfers retain their existing behavior.
- **CAP-2**
  - **intent:** A receiver can inspect and download the collection, and a sender can offer it again.
  - **success:** GET previews collection count and logical size; one explicit POST receives a valid streamed ZIP with distinct numbered top-level member names. Send Again revalidates all remembered paths and creates fresh identifiers.

## Constraints

- This extension supersedes the canonical single-selected-root restriction only for the bounded collection defined in selection-contract.md. All other lifecycle, trust, disclosure and resource guarantees remain.
- The whole selection is accepted or refused before networking; no silently omitted member. Duplicate canonical paths and parent/descendant overlaps are refused. Aggregate logical size must be JavaScript-safe.
- Prepare members one at a time in archive order, so at most one top-level file descriptor and one prepared directory pin are retained at any moment, within the existing 64 retained plus three transient directory-handle budget; the item count never consumes that budget. The first member is prepared before response headers, and every later member as the stream reaches it.
- Unwind every acquisition on failure, and never finalize a failed archive: a later member that changed or vanished after Stage aborts the stream. Memory is bounded by copy buffers, at most 10,000 selected-root records, and ZIP's required central directory; no extra descendant index or staged archive.
- Preserve security refusals and existing warnings for names that some receiving filesystems cannot represent. Never expose absolute paths in public metadata, names, diagnostics or errors.

## Non-goals

- Incoming transfers, connection recovery, text sharing, persistent queues/history, resume, multiple receivers, dependency upgrades and release publication.
- Snapshot/backup semantics, preserving filesystem permissions, or resolving collisions already inside a selected folder.

## Success signal

A sender combines files and folders with matching basenames using keyboard controls or a native drop. One browser previews the collection and downloads a complete ZIP containing every selected member under distinct names. Cancellation and member failure leak no owned resources, and Send Again revalidates the whole selection.

## Assumptions

- 10,000 roots is a sanity cap, not a resource budget (owner, 2026-10-11, superseding an agent-chosen 16). Ordinals are zero-padded to the count's width so collisions stay understandable without another naming UI.
