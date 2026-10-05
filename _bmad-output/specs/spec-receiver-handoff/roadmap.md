# FairDrop additions: implementation order

Owner direction, 2026-10-04: implement all proposed additions one by one, highest impact first. Specs precede Sol implementation; the main session orchestrates review, verification and integration. These are sequencing decisions, not unreviewed implementation contracts.

| Order | Slice | Intended benefit | Contract work before implementation |
| --- | --- | --- | --- |
| 1 | Receiver landing page | Inspect, then intentionally download; ordinary GET previews do not consume the transfer | receiver-protocol.md; current story |
| 2 | Send Again | Offer the same successfully sent item using a fresh code | Retain only the in-memory selection through success; reuse normal Stage |
| 3 | Multiple selected items | Send several files/folders as one streamed ZIP | Explicit limits, duplicate/archive-name collisions, safe traversal and cancellation |
| 4 | Phone-to-desktop receiving | Choose files in a phone browser and save on the desktop | Destination approval, bounds, filenames, no overwrite surprises, partial-write cleanup and honest persistence wording |
| 5 | Connection recovery | Recover when the selected network is wrong or changes | Eligible interface choice, restaging/retiring stale codes, truthful diagnostics |
| 6 | Text/link sharing | Move short text or a URL without creating a file | Payload limits, escaping, copy fallback, no auto-navigation or automatic clipboard reads |

Each slice is independently reviewable. Current baseline is 1.3.1. Version bumps and publication belong to a separately verified release step; this roadmap does not claim later slices are implemented.
