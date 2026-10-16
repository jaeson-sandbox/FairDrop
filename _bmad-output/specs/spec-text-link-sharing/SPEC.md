---
id: SPEC-text-link-sharing
companions:
  - text-contract.md
  - ../spec-connection-recovery/recovery-contract.md
  - ../../../docs/fairdrop-contracts.md
  - ../../../docs/fairdrop-architecture.md
  - ../../planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md
  - ../../planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md
sources: []
---

# Text and link sharing

## Why

Moving a short note or URL to a nearby device should not require making a file. FairDrop can offer the same intentional, ephemeral handoff for text while retaining its single-session LAN model.

## Capabilities

- **CAP-1**
  - **intent:** A desktop user can compose or paste short text and offer it to one nearby browser.
  - **success:** Accepted text produces a fresh code without clipboard reads or filesystem access; invalid or oversized input produces a clear refusal without starting a session.
- **CAP-2**
  - **intent:** A receiver can intentionally reveal and copy the offered text using ordinary browser controls.
  - **success:** Preview GET reveals no payload and consumes nothing; one authorized POST reveals the exact normalized text in a selectable readonly field, including with scripts and clipboard APIs unavailable. URLs remain inert text.
- **CAP-3**
  - **intent:** A sender can offer the same text again or refresh a waiting connection without retyping it.
  - **success:** Send Again, retry and waiting recovery preserve only the bounded in-memory text and issue a fresh identity; explicit cancellation, dismissal, replacement or unmount forget it, and stale work cannot resurrect it.

## Constraints

- Accept at most 65,536 normalized UTF-8 bytes, with a 131,072-byte raw bound checked before normalization. Normalize CRLF/CR to LF and preserve all other accepted text exactly. Use the common explicit blank predicate and malformed-Unicode/control rules in text-contract.md.
- No text payload in files, cache, logs, discovery, metadata, lifecycle events or retained receipts. No secure-erasure promise for browser or operating-system copies.
- Preserve exact-token routing, intentional one-shot claim, no-store receiver pages, strict script-free content policy, and escaped plain-text rendering. Completion proves full local write and HTTP finalization, never remote delivery or copying.
- Composing, staging and active text exclude other file/drop/receive selections. Preserve cancellation, bounded teardown, stale-response fencing, file/collection/receive contracts and waiting recovery ownership.
- Keep ordinary selection and copy available over plain HTTP without JavaScript, permissions or secure-context APIs. Follow Quartz focus, keyboard, target-size, forced-colors and announcement contracts.

## Non-goals

- Phone-to-desktop text, clipboard synchronization, automatic clipboard reads, URL previews or navigation, custom browser clipboard buttons, persistence/history, encryption, version changes or release publication.

## Success signal

A desktop user pastes a short note or URL, the nearby browser previews without disclosure, then reveals and copies its exact normalized content. Deterministic tests prove one claim, escaping, input bounds, no filesystem access, truthful completion, and safe fresh-code reuse/recovery.

## Assumptions

- The first text slice is outbound only, capped at 64 KiB, with newline normalization and native browser selection/copy. These are lean choices under the owner's authorization to implement the roadmap in order.
