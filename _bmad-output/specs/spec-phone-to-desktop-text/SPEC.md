---
id: SPEC-phone-to-desktop-text
companions:
  - receive-text-contract.md
  - ../spec-text-link-sharing/text-contract.md
  - ../spec-connection-recovery/recovery-contract.md
  - ../../../docs/fairdrop-contracts.md
  - ../../../docs/fairdrop-architecture.md
  - ../../planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md
  - ../../planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md
sources: []
---

# Phone-to-desktop text and link sharing

## Why

FairDrop can send a short note or URL from desktop to phone, but the reverse still requires another app or a file. Extend its intentional, ephemeral LAN handoff so a phone browser can send text to the desktop clipboard without accounts or synchronization.

## Capabilities

- **CAP-1**
  - **intent:** A desktop user can invite one nearby browser to send text.
  - **success:** Receive text starts a fresh text-only QR/link without folder selection or clipboard/filesystem access; waiting cancellation and network recovery work with fresh identities.
- **CAP-2**
  - **intent:** A phone user can deliberately submit a short note or URL.
  - **success:** A script-free form accepts one complete valid input into bounded desktop memory; invalid or interrupted input cannot consume a successful receipt or expose partial text.
- **CAP-3**
  - **intent:** A desktop user can inspect and explicitly copy received text, then dismiss it.
  - **success:** A held outcome shows exact normalized selectable text, native copy reports only actual success, and Done clears application state; stale commands or responses cannot affect another session.

## Constraints

- Reuse the 65,536-byte normalized UTF-8 cap, 131,072-byte raw cap, newline normalization and Unicode/control rules. Bound form encoding and simultaneous request processing separately as specified in receive-text-contract.md.
- One single-use capability and coordinator session; exact-token routing, script-free/no-store pages, explicit commit, bounded teardown and generation fencing. Receipt means committed RAM, independent of phone acknowledgement delivery, never clipboard success.
- No payload in metadata, lifecycle events, logs, discovery, files or browser storage. Fetch/copy only by the current committed session ID. Native clipboard writes are explicit; Done does not erase the OS clipboard.
- Keep existing file receive, outbound text, collection, recovery and Quartz accessibility contracts. This extension supersedes only the earlier outbound slice's exclusion of reverse text sharing.

## Non-goals

- Clipboard synchronization or reads, history, persistent settings, photo changes, URL navigation/previews, multi-sender sessions, encryption, new dependencies, version bump or release publication.

## Success signal

Scan the desktop's text-receive QR, submit a note in the phone browser, inspect and copy its exact normalized value on the desktop, then dismiss it. Deterministic tests prove bounded input, a single authorized commit, native-copy honesty and clearing across cancellation/recovery/stale-response races.

## Assumptions

- Use a distinct Receive text action, readonly desktop preview and the outbound 64 KiB limit. These lean interaction choices preserve existing file receiving and avoid adding a chooser or clipboard permissions to the phone flow.
