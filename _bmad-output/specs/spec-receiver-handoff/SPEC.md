---
id: SPEC-receiver-handoff
companions:
  - receiver-protocol.md
  - ../spec-fairdrop/SPEC.md
sources: []
---

# Receiver handoff and repeat sending

## Why

FairDrop 1.3.1 consumes a transfer when a capability URL is opened, including an automated link preview. Receivers need to inspect an item before intentionally downloading it, and senders need to send the same item again without reopening a chooser.

## Capabilities

- **CAP-1**
  - **intent:** A receiver can inspect an offered item and explicitly start its download.
  - **success:** Repeated valid page visits neither claim nor read the payload; the first explicit Download receives the file or streamed ZIP, and competing claims cannot receive a second copy.
- **CAP-2**
  - **intent:** A sender can offer the successfully sent item again without selecting it again.
  - **success:** Send Again revalidates the in-memory selection through normal Stage and creates a fresh session/token/QR; dismissal, cancellation, replacement and unmount forget the selection. Send Another and error retry still work.

## Constraints

- This extension supersedes only the original contract's first-GET claim rule and forgetting the selection on success; all other existing invariants remain until a later spec explicitly changes them.
- Preserve one live session, one selected root, one receiver, trusted-LAN HTTP, source identity checks, streaming and bounded teardown; no persistent product state or staged payload copies.
- The receiver page and request matrix are defined in receiver-protocol.md. Metadata is Stage-time information, not a content snapshot or compressed ZIP size guarantee.
- Each capability is independently implemented and reviewed through bmad-build, with an implementation spec created before Sol receives the story.

## Non-goals

- Multi-item sending, incoming uploads, network selection, text/link payloads, multiple concurrent receivers, resume, TLS and retained history are not part of this slice.
- No receiver installation, external assets, scripts, analytics, expiry-page promise after listener shutdown, or claim that a browser saved received bytes.

## Success signal

A browser can open the same staged URL repeatedly while the desktop stays ready; pressing Download transfers exactly one file or valid ZIP. Send Again produces a working fresh code for the revalidated item, and the old session cannot serve it again.

## Assumptions

- One additional tap after scanning is acceptable; use a native HTML POST form for explicit intent.
- Direct-GET download integrations change to POST. This is an intentional protocol change to be stated in release notes.
