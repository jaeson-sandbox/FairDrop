# Spine Pair Review — FairDrop

## Overall verdict

The pair is mechanically coherent and adequate as a draft: flows, components, named tokens, source paths, and required spine shape are unusually complete. It is not yet a safe final downstream contract because three acknowledged presentation/copy decisions remain open and several memlog assumptions are written as committed requirements without visible `[ASSUMPTION]` status.

## 1. Flow coverage — strong

Checked all source requirement families (`CAP-1`–`CAP-7`, `FR1`–`FR22`, `NFR1`–`NFR14`, `UX-DR1`–`UX-DR8`), all three epics, and Stories 1.1–1.9, 2.1–2.2, and 3.1–3.3 against the four Key Flows and the requirement-to-flow table. Every product/release journey has a named protagonist, numbered steps, a marked climax, and an applicable failure path; cross-cutting quality constraints are explicitly routed to component, lifecycle, privacy, accessibility, and platform contracts rather than forced into artificial journeys (`EXPERIENCE.md` § Key Flows, lines 175–231).

### Findings

- None.

## 2. Token completeness — adequate

Extracted every `colors`, `typography`, `rounded`, `spacing`, and `components` token and every `{path.to.token}` occurrence in both spines. All named paths exist; every color is a six-digit hex value, applicable light/dark pairs are present, typography/rounded/spacing values fit the DESIGN.md type rules, and load-bearing contrast targets and verified pairs are stated (`DESIGN.md` frontmatter, lines 15–94; § Colors, lines 107–120).

### Findings

- **medium** `staged-view.shadow` and `shadow-dark` embed token references inside longer CSS strings (`'3px 3px 0 {colors.border}'`), while the component-token contract permits a concrete value or a standalone `{path.to.token}` reference. A literal resolver can therefore emit invalid CSS even though the inner paths exist (`DESIGN.md` line 81). *Fix:* use complete concrete shadow values in frontmatter, or split shadow geometry/color into independently resolvable component tokens and compose them in implementation guidance.

## 3. Component coverage — strong

Extracted the 18 named components from DESIGN.md frontmatter and compared them with the visual table and EXPERIENCE.md behavioral table. App Shell, DropZone, Selection Controls, Stage Pending Card, StagedView, Item Summary, QR Panel, Direct URL Row, Copy Feedback, Trusted-LAN Note, Warning Banner, TransferView, Progress Meter, Transfer Metrics, Cancel Action, Done Panel, Error Panel, and Status Announcer all have substantive visual and behavioral rules under matching names (`DESIGN.md` lines 76–94 and 142–165; `EXPERIENCE.md` lines 60–83).

### Findings

- None.

## 4. State coverage — thin

Walked every IA surface: desktop Idle, native dialogs, staging pending, Staged, Transferring, Done/Error, receiver browser/download, firewall prompt, and second-instance restoration. Rest/active/focus/error/offline/cancel/reset/permission and HTTP claim variants are broadly covered, but three explicitly recorded gaps leave downstream presentation or copy unresolved (`EXPERIENCE.md` lines 27–41 and 85–112; `.working/finalize-gaps.md`).

### Findings

- **high** Known-empty file progress has a binding data shape but no committed visual or copy treatment, so Story 1.9 cannot be implemented or accepted consistently (`EXPERIENCE.md` lines 78 and 100). *Fix:* choose and specify the visible meter state, label, metrics, and accessibility semantics for `totalKnown=true`, `totalBytes=0`, `percent=0`.
- **high** The firewall surface is listed and Flow 4 assumes first-use guidance is visible, but placement, timing, and approved wording remain open; downstream cannot determine what FairDrop owns around the OS prompt (`EXPERIENCE.md` lines 38, 109–110, 127, and 216–222). *Fix:* commit the product-owned pre-prompt/denial/release-note surfaces and exact neutral guidance, including platform deltas where required.
- **high** Error states depend on fixed safe `PublicError.message` values, but no code-to-copy table defines those messages. This leaves privacy-sensitive UI wording to backend implementers and prevents consistent state acceptance (`EXPERIENCE.md` lines 55–56, 82, 94, 103, and 125). *Fix:* approve one safe user-facing message per stable error code, with the non-terminal `beacon_warning` kept separate.

## 5. Visual reference coverage — strong

Enumerated `mockups/`, `wireframes/`, and `imports/`: `imports/` is empty and the other two contain no files, so there are no required promoted references to orphan. The working direction, theme, source extract, and flow artifacts that do inform the draft resolve at their linked paths, are described inline, and the spines-win-on-conflict rule is stated (`DESIGN.md` line 99; `EXPERIENCE.md` lines 19 and 41).

### Findings

- None.

## 6. Bloat & overspecification — strong

The pair stays centered on visual and behavioral decisions. The detailed lifecycle, HTTP outcome, trusted-LAN, and disclosure rules repeat some upstream facts, but each repetition directly constrains a visible state, transition, message, or accessibility behavior; the invented privacy and backend-authority sections therefore earn their place (`EXPERIENCE.md` lines 122–141). DESIGN.md uses compact tables where token-backed rules benefit from them and reserves prose for editorial posture.

### Findings

- None.

## 7. Inheritance discipline — thin

All seven frontmatter source paths resolve in both spines, requirement/epic names used in Key Flows match their sources, component names align across both files, and all EXPERIENCE.md token references resolve to DESIGN.md. Canonical authority is recoverable by reading the first source and the linked source extract, but assumption status and source precedence are not fully carried into the spines themselves.

### Findings

- **high** Several memlog entries explicitly recorded as assumptions are presented as unconditional contract language: automatic OS theme with no setting, immediate staging after picker return, `Copied` feedback, the voice rules, detailed live-region/focus behavior, and the no-tray/no-notification posture (`.memlog.md` lines 24 and 26–29; `DESIGN.md` line 105; `EXPERIENCE.md` lines 25, 45–58, 68, 74, 117–120, 145–151, and 162). Only the invented protagonists in Flows 2 and 4 retain `[ASSUMPTION]` tags. *Fix:* confirm and log these as decisions before finalization, or mark each governing rule visibly `[ASSUMPTION]` so downstream consumers do not mistake it for approved scope.
- **medium** The frontmatter lists the canonical SPEC/companions beside traceability-era sources without stating precedence, although the legacy spec contains superseded names, APIs, and dependencies. A consumer that does not follow the full source graph must infer authority from ordering or from `.working/source-extract.md` (`DESIGN.md` and `EXPERIENCE.md` lines 7–14; `EXPERIENCE.md` line 19). *Fix:* add one short authority rule to the spines: `SPEC.md` and its binding companions win; original and Phase 1 documents are traceability only where non-conflicting.

## 8. Shape fit — strong

DESIGN.md contains all eight canonical sections in the locked order: Brand & Style, Colors, Typography, Layout & Spacing, Elevation & Depth, Shapes, Components, and Do's and Don'ts (`DESIGN.md` lines 101–176). EXPERIENCE.md contains every required default section; Responsive & Platform and Inspiration & Anti-patterns are correctly present for the multi-surface/reference-driven product, while Trusted-LAN & Privacy and Backend-Authoritative Lifecycle are justified product-specific additions (`EXPERIENCE.md` lines 21–175).

### Findings

- None.

## Mechanical notes

- Frontmatter is parse-shaped and complete for the draft stage; both spines remain `status: draft`.
- All seven source paths resolve after substituting `{project-root}`, `{planning_artifacts}`, and `{implementation_artifacts}`.
- Every standalone `{colors.*}`, `{typography.*}`, `{rounded.*}`, `{spacing.*}`, and `{components.*}` reference resolves; the only syntax concern is the two embedded shadow references identified in §2.
- The 18 component names match across DESIGN.md frontmatter, DESIGN.md.Components, and EXPERIENCE.md.Component Patterns.
- Working links resolve. No files exist in `mockups/`, `wireframes/`, or `imports/`; therefore there are no orphans in the Reviewer Gate's promoted-reference scope.
- Neither spine contains Mermaid, so there is no Mermaid syntax to validate.
