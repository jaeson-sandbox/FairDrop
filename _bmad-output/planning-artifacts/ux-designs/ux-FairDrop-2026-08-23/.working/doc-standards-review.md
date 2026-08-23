# Document Standards Review — FairDrop UX

This document pair exists to help human developers and downstream implementation agents build and verify FairDrop's approved visual and behavioral UX consistently.

Structure model: **Reference/Database** — random access, mutually exclusive topics, stable schemas, and one source of truth per decision.

## Structure pass

| Pass | Original Text | Revised Text | Changes |
|---|---|---|---|
| structure | `EXPERIENCE.md` §Backend-Authoritative Lifecycle (158 words), after component/state/interaction/firewall/privacy details | **MOVE** immediately after §Information Architecture | Establishes the authoritative event model before dependent tables. Saves 0 words. |
| structure | `EXPERIENCE.md` §Voice and Tone plus §Stable public error and warning copy (826 words), with strings repeated later | **MERGE** literal product strings into the copy registry with stable keys; reference keys elsewhere | One source of truth. Estimated saving: ~70 words. |
| structure | `EXPERIENCE.md` §Key Flows and coverage (652 words) | **CONDENSE** each flow to unique cross-surface sequence, failure branch, and evidence relationship | Preserve personas, ordering, climax, failure, and coverage. Estimated saving: ~150 words. |
| structure | `EXPERIENCE.md` §Foundation (257 words) | **CONDENSE** to definition, roles, external promise, and pointers | Preserve orientation; detailed boundaries remain authoritative later. Estimated saving: ~70 words. |
| structure | `DESIGN.md` repeated mockup catalogs in §Layout & Spacing and §Components | **MERGE** into one compact index; use a short back-reference elsewhere | Preserve all production references. Estimated saving: ~45 words. |
| structure | `DESIGN.md` §Do's and Don'ts plus `EXPERIENCE.md` §Inspiration & Anti-patterns | **CONDENSE** into nonoverlapping visual versus behavioral/scope guardrails | Retain both required sections. Estimated saving: ~35 words. |
| structure | `EXPERIENCE.md` §Component Patterns and §State Patterns | **PRESERVE** separately | They are valid component-implementation and lifecycle-verification indexes. |
| structure | `DESIGN.md` machine frontmatter and matching body tables | **PRESERVE** both | Machine-readable tokens and human usage rules have different consumers. |

Accepting the six edits is estimated to reduce the pair by about 370 words, from 7,574 to 7,204 words (4.9%), without dropping requirements.

## Prose pass

| Pass | Original Text | Revised Text | Changes |
|---|---|---|---|
| prose | WCAG 2.2 targets are normal text ≥4.5:1, large text ≥3:1, and load-bearing non-text boundaries/focus/value distinctions >3:1 without rounding. | WCAG 2.2 targets are ≥4.5:1 for normal text, ≥3:1 for large text, and >3:1 without rounding for load-bearing non-text boundaries, focus indicators, and value distinctions. | Parallel construction and expanded shorthand. |
| prose | At an effective 320 CSS-pixel content width, reflow into one vertical column with no page-level horizontal scroll, lost information, overlap, or clipped action. | At an effective content width of 320 CSS pixels, reflow into one vertical column with no page-level horizontal scrolling, information loss, overlap, or clipped actions. | Clearer measurement and parallel outcomes. |
| prose | Renders one authoritative lifecycle presentation plus an optional retained terminal outcome in Idle; subscribes once to `transfer-*`; second launch preserves session and retained status. | Renders one authoritative lifecycle presentation plus an optional retained terminal outcome in Idle; subscribes once to `transfer-*`; a second launch preserves the session and retained status. | Added missing articles. |
| prose | Pending state retains focus, suppresses repeats with `aria-disabled="true"`, and changes visible/accessibility label to Canceling…; lifecycle/command authority determines outcome. | While cancellation is pending, the control retains focus, suppresses repeat activation with `aria-disabled="true"`, and changes its visible and accessible label to Canceling…; lifecycle and command authority determine the outcome. | Named subject and expanded shorthand. |
| prose | Pre-ack cancellation has no lifecycle event, so the matching local command pair is authoritative: remain pending until `CancelTransfer` resolves and the guarded `StageTransfer` promise rejects with `cancelled`; either may arrive first. | Pre-ack cancellation emits no lifecycle event, so the matching local command pair is authoritative. Remain pending until `CancelTransfer` resolves and the guarded `StageTransfer` promise rejects with `cancelled`; either result may arrive first. | Split overloaded sentence. |
| prose | When it closes, focus returns to Stage Pending/the next state heading if allowed, or the applicable focused Error Panel if the command reports denial/setup failure. | When the prompt closes, focus moves to Stage Pending or the next state heading if access is allowed. If the command reports denial or setup failure, focus moves to the applicable Error Panel. | Replaced unclear antecedent and shorthand. |
| prose | If the app cannot observe denial directly, **Not downloading?** guidance covers the same local Wi-Fi, guest/client isolation, firewall permission, Cancel, and a fresh link. | If the app cannot observe denial directly, the **Not downloading?** guidance tells users to check the same local Wi-Fi, guest or client isolation, and firewall permission, then Cancel and create a fresh link. | Made the recovery sequence explicit. |
| prose | FairDrop sends directly over the local network and uploads/stores no extra payload copy. | FairDrop sends directly over the local network and does not upload or store an extra copy of the payload. | Replaced compressed slash wording. |
| prose | At an effective 320 CSS px, use one-dimensional vertical reflow only. | At an effective content width of 320 CSS pixels, use only one-dimensional vertical reflow. | Aligned unit wording. |
| prose | Every release record names sender OS/version, receiver OS/browser/version, artifact version or checksum, date, reviewer, and pass/fail. | Every release record names the sender OS and version, receiver OS and version, browser and version, artifact version or checksum, date, reviewer, and pass/fail result. | Removed ambiguous groupings. |
| prose | **Reject security theater:** no lock/shield or secure/private/encrypted claim for plain HTTP. | **Reject security theater:** do not use lock or shield icons or describe plain HTTP as secure, private, or encrypted. | Direct prohibition. |
| prose | The QR remains primary; link, first-opener/preview warning, unencrypted-network disclosure, no-extra-copy copy, troubleshooting, and Cancel are available. | The QR remains primary; the link, first-opener and preview warning, unencrypted-network disclosure, no-extra-copy disclosure, troubleshooting, and Cancel are available. | Removed repetition and slash shorthand. |
| prose | After the browser completes, Jaeson chooses where to keep/open the ZIP in Safari/Files; FairDrop does not claim to observe that action. | After the browser completes the download, Jaeson chooses where to keep or open the ZIP in Safari or Files; FairDrop does not claim to observe that action. | Supplied missing object and clarified alternatives. |
| prose | **Climax:** the existing window restores with session and focus context intact; no competing listener, process, or duplicated speech appears. | **Climax:** the existing window is restored with the session and focus context intact; no competing listener or process starts, and no speech is duplicated. | Restored parallel predicates. |

All product decisions, approved strings, technical meaning, canonical sections, and machine-readable frontmatter remain unchanged.
