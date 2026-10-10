# FairDrop additions: implementation order

Owner direction, 2026-10-04: implement all proposed additions one by one, highest impact first. Specs precede Sol implementation; the main session orchestrates review, verification and integration. These are sequencing decisions, not unreviewed implementation contracts.

| Order | Slice | Intended benefit | Contract work before implementation |
| --- | --- | --- | --- |
| 1 | Receiver landing page | Inspect, then intentionally download; ordinary GET previews do not consume the transfer | Implemented: receiver-protocol.md and spec-receiver-landing-page.md |
| 2 | Send Again | Offer the same successfully sent item using a fresh code | Implemented: spec-send-again.md; retain only in-memory selection and reuse normal Stage |
| 3 | Multiple selected items | Send several files/folders as one streamed ZIP | Implemented: spec-multiple-selected-items/SPEC.md, selection-contract.md and spec-multiple-selected-items.md; 16-root limit, numbered archive roots, atomic validation and shared resource accounting |
| 4 | Phone-to-desktop receiving | Choose files in a phone browser and save on the desktop | Destination approval, bounds, filenames, no overwrite surprises, partial-write cleanup and honest persistence wording |
| 5 | Connection recovery | Recover when the selected network is wrong or changes | Eligible interface choice, restaging/retiring stale codes, truthful diagnostics |
| 6 | Text/link sharing | Move short text or a URL without creating a file | Payload limits, escaping, copy fallback, no auto-navigation or automatic clipboard reads |

Each slice is independently reviewable. Current published baseline is 1.4.0. Version bumps and publication belong to a separately verified release step; this roadmap does not claim later slices are implemented.
