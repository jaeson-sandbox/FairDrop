# FairDrop additions: implementation order

Owner direction, 2026-10-04: implement all proposed additions one by one, highest impact first. Specs precede Sol implementation; the main session orchestrates review, verification and integration. These are sequencing decisions, not unreviewed implementation contracts.

| Order | Slice | Intended benefit | Contract work before implementation |
| --- | --- | --- | --- |
| 1 | Receiver landing page | Inspect, then intentionally download; ordinary GET previews do not consume the transfer | Implemented: receiver-protocol.md and spec-receiver-landing-page.md |
| 2 | Send Again | Offer the same successfully sent item using a fresh code | Implemented: spec-send-again.md; retain only in-memory selection and reuse normal Stage |
| 3 | Multiple selected items | Send several files/folders as one streamed ZIP | Implemented: spec-multiple-selected-items/SPEC.md, selection-contract.md and spec-multiple-selected-items.md; 10,000-root sanity cap, numbered archive roots, atomic validation and lazy one-member preparation |
| 4 | Phone-to-desktop receiving | Choose files in a phone browser and save on the desktop | Implemented: spec-phone-to-desktop-receiving/SPEC.md, receive-contract.md and spec-phone-to-desktop-receiving.md; chosen folder per session, time-named subfolder, 3 GiB free-space reserve, keep completed files on failure, OS download marking |
| 4 extension | Receive Again | Receive another complete file upload into the same approved parent with a fresh code | Implemented by `spec-receive-again.md`; private in-memory parent only, ordinary StartReceive revalidation, fresh exclusive subfolder, explicit user activation |
| 4 extension | Phone-to-desktop text | Enter one note or link in a phone browser and inspect/copy it on the desktop | Implemented by `spec-phone-to-desktop-text/SPEC.md`, `receive-text-contract.md`, and `spec-phone-to-desktop-text.md`; bounded RAM-only receipt, held selectable outcome, explicit native copy, no folder or history |
| 5 | Connection recovery | Recover when the selected network is wrong or changes | Implemented: spec-connection-recovery/SPEC.md and recovery-contract.md; eligible interface choice, explicit refresh of waiting send/receive sessions, retirement of stale codes and truthful diagnostics |
| 6 | Text/link sharing | Move short text or a URL without creating a file | Implemented: spec-text-link-sharing/SPEC.md, text-contract.md and spec-text-link-sharing.md; bounded outbound text, intentional browser reveal, native selection/copy, fresh-code reuse and waiting recovery |

Each slice is independently reviewable. Current published baseline is 1.8.0. Version bumps and publication belong to separately verified release steps; this roadmap does not claim later slices are implemented.
