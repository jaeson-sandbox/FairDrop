# FairDrop UX Source Evidence

## Scope and authority

Read in full:

- `_bmad-output/specs/spec-fairdrop/SPEC.md`
- `_bmad-output/planning-artifacts/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md`
- `docs/fairdrop-contracts.md`
- `_bmad-output/planning-artifacts/epics.md`
- `docs/fairdrop-spec.md`, with **Phase 1 Corrections** applied before the original body

Review notes and party artifacts were excluded. `SPEC-fairdrop` and its binding companions, especially `docs/fairdrop-contracts.md`, take precedence. `epics.md` supplies the exact `FR*`, `NFR*`, `UX-DR*`, epic, and story decomposition. The original `docs/fairdrop-spec.md` contributes narrative only where it does not conflict.

## Users, actors, and goals

- **Sender** — a Windows or macOS desktop user who wants to move exactly one local regular file or directory to a nearby device without accounts, cloud storage, receiver installation, history, or retained product data (`CAP-1`, `CAP-2`, `CAP-5`, `CAP-6`; Epic 1, Epic 2).
- **Receiver** — a person using a modern browser on the same trusted LAN. They claim one capability URL and receive exact file bytes or a browser-compatible ZIP; there is no native receiver app and no specified receiver web application (`CAP-3`, `CAP-4`; Stories 1.3, 1.4, 2.2).
- **Maintainer/release operator** — verifies and ships a single-instance native application on Windows amd64 and macOS (`CAP-7`; Epic 3). This actor affects first-launch, firewall, restoration, and release-copy requirements rather than the core transfer screen flow.
- No named persona or contextual protagonist is defined; the sources use only the role names **sender** and **receiver**.

## Evidence-backed journeys

### 1. Share one file (`FR1`-`FR11`, `FR13`-`FR21`; Stories 1.1-1.9)

1. In **Idle**, the sender either drops exactly one native path on the marked target or invokes **Select File**. The native dialog command returns a path and does not itself stage; cancelling the dialog is quiet (`FR1`-`FR3`, `UX-DR1`, `UX-DR2`, Story 1.7).
2. Invalid selection fails safely: zero/multiple dropped paths are rejected without silently taking the first; missing, empty, link-like, reparse, or special-file inputs produce stable safe errors with no absolute path disclosure (`FR3`, `FR21`, Story 1.1).
3. While `StageTransfer` is pending, the frontend may show local pending status, but it cannot enter **Staged** until successful backend metadata arrives. Setup failure/cancellation before acknowledgement produces a command error and no lifecycle event (Story 1.8; `AD-8`).
4. **Staged** shows sanitized item name, human-readable logical size, QR code, copyable direct URL, non-fatal warnings, trusted-LAN guidance, and **Cancel** (`UX-DR3`, Story 1.9).
5. The receiver scans the QR code or uses the direct URL. Only the first exact-token `GET` claims the session. Wrong method/route/token appears as `404`; a competing valid request gets `423` only while the listener is live (`FR10`, Story 1.4).
6. The sender moves to **Transferring** only after the backend accepts the claim and emits `transfer-started`. The view shows wire bytes, human-readable throughput, honest progress, and **Cancel** (`FR14`-`FR18`, `UX-DR4`).
7. Completion or failure is backend-authoritative. **Done** or **Error** shows safe outcome text, announces the change, and returns to **Idle** only on `transfer-reset`, normally three seconds later (`FR19`, `UX-DR5`, Story 1.6).

### 2. Share one folder (`FR12`; Stories 2.1-2.2)

The sender uses the same journey through **Select Directory** or native drop. **Staged** may show the directory's logical display size, but transfer wire size is unknown. **Transferring** must show actual ZIP wire bytes and throughput with an indeterminate treatment; it must not turn logical directory size into a ZIP percentage. The receiver gets a sanitized `.zip` download with one top-level root and no temporary archive.

### 3. Cancel, fail, and recover (`FR18`, `FR19`; Story 1.6)

- Cancel before successful Stage acknowledgement: Stage returns `cancelled`; no lifecycle event or terminal screen is emitted.
- Cancel from **Staged** or pre-commit `CLAIMING`: resources quiesce and one reset returns the UI to **Idle**; no started/complete/error event appears.
- Cancel after transfer commit: any visible `started, progress*` prefix ends with reset, not completion or cancellation-as-error.
- Natural transfer failure: optional final progress, then a safe `transfer-error`, then reset.
- A second application launch restores and shows the existing window without replacing its active session (`FR22`, Story 3.1).

## Screens and surfaces

| Surface | Required evidence |
| --- | --- |
| FairDrop desktop window | Wails v2 desktop app, normal start state, standard OS chrome (`Frameless: false`), responsive minimum dimensions preserved (`AD-11`, Story 1.9). |
| **Idle** / `DropZone.tsx` | One clearly marked native drop target; separate semantic **Select File** and **Select Directory** controls; safe selection/command errors (`UX-DR1`, `UX-DR2`). |
| Native file and directory dialogs | Keyboard-reachable OS dialogs; cancelled dialog returns an empty selection quietly and does not stage automatically (Story 1.7). |
| Staging-pending presentation | A local pending state is permitted, but is not backend **Staged** and must not initialize a session before metadata succeeds (Story 1.8). Exact presentation is unspecified. |
| **Staged** / `StagedView.tsx` | Name, human-readable logical size, QR image, copyable URL, warnings, trusted-LAN guidance, Cancel (`UX-DR3`, Story 1.9). |
| **Transferring** / `TransferView.tsx` | Progress treatment, wire bytes, throughput, Cancel (`UX-DR4`). Original narrative asks for a large progress bar and MB/s metric; canonical stories broaden this to human-readable throughput and honest known/unknown totals. |
| **Done** | Safe success outcome plus announced transition; visible only until backend reset, normally three seconds (`UX-DR5`). |
| **Error** | Stable safe message/outcome plus announced transition; no raw adapter text, path, or token; visible until backend reset (`FR21`, `UX-DR5`). |
| Warning/validation presentation | Non-fatal `beacon_warning` leaves the direct transfer usable; drop validation and command errors fail safely. Exact placement is unspecified. |
| Receiver browser/download surface | A modern same-LAN browser receives an attachment response. Files have known length; directory ZIPs omit `Content-Length`. No custom receiver landing page is specified. |
| OS firewall surface | Product/release guidance must prepare the sender for the first-run Windows/macOS firewall prompt (Story 3.3; original spec §7). |
| Second-instance restoration | A later launch unminimizes/shows the existing FairDrop window and retains its current session (`FR22`, Story 3.1). |

## State and interaction contract

- Public presentations are **Idle**, **Staged**, **Transferring**, **Done**, and **Error** (`CAP-6`, `FR20`). `STAGING` and `CLAIMING` are internal and must not be presented as authoritative public states (`FR14`, `AD-2`).
- Only `IDLE` accepts Stage. Stage elsewhere returns `busy` with no state/resource change (`FR14`; command/state table).
- Backend events are authoritative: `transfer-started`, `transfer-progress`, `transfer-complete`, `transfer-error`, and `transfer-reset` (`FR17`). The reducer initializes from successful metadata, then accepts only matching `sessionId` and strictly increasing `seq`; stale sessions, duplicates, obsolete Stage promises, and post-terminal progress are ignored (`UX-DR7`, Story 1.8).
- Known positive file totals use a determinate, clamped 0-100 bar. Directory/unknown totals use `totalKnown=false`, `totalBytes=0`, `percent=0` and an indeterminate treatment. Known-empty files use `totalKnown=true`, zero total, and zero percent; their exact visual treatment is not fully specified (`NFR8`, `UX-DR4`, Story 1.9).
- Progress measures bytes successfully written on the wire, includes rolling bytes/second, and is emitted at most 4 Hz plus required terminal progress (`FR16`, `AD-7`).
- Native drop is implemented only through `OnFileDrop(callback, true)` with `OnFileDropOff()` cleanup. The marked target inherits `--wails-drop-target: drop`; no DOM drop handler or class-only gate is allowed (`UX-DR8`, `AD-11`).
- Exactly one dropped path is forwarded to `StageTransfer`. Native browse uses `SelectFile()` / `SelectDirectory()` and then the frontend decides whether to invoke Stage; the binding commands do not stage by themselves.
- The QR value is padded PNG base64 without a data-URI prefix; React prepends `data:image/png;base64,` only when rendering (`docs/fairdrop-contracts.md`, Story 1.9).
- Cancel must remain available in **Staged** and **Transferring**. UI animation callbacks cannot drive lifecycle state (`UX-DR3`, `UX-DR4`, `UX-DR6`).
- The direct URL is single-use for one receiver. V1 has no resume/range, transfer history, persistent preferences, multiple receivers, or multi-item staging (`SPEC-fairdrop` Constraints/Non-goals).

## Voice, copy, privacy, and disclosure constraints

- Use **FairDrop** everywhere. **DeadDrop** is a stale working title (Phase 1 Correction 3).
- State the **trusted-LAN, plain-HTTP** boundary honestly. Required release-copy meaning: the capability URL reduces blind discovery, but plain HTTP does not protect against a LAN observer (`NFR4`, Story 3.3). Do not imply end-to-end encryption, hostile-network security, authenticated discovery, cloud relay, signing, notarization, auto-update, resume, or multi-receiver support.
- Explain first-launch Windows/macOS firewall access because inbound LAN HTTP requires it.
- Public errors and warnings use stable safe `{code,message}` values. Never display arbitrary adapter text, absolute/relative source paths, or capability tokens (`FR21`; `docs/fairdrop-contracts.md`).
- A cancelled native dialog is quiet. User cancellation is not presented as a transfer failure.
- `beacon_warning` is non-terminal: copy must make clear that direct URL/QR transfer still works even though mDNS publication failed.
- The capability token may appear only in the local Stage URL/QR and receiver request path. The selected basename may appear in local metadata and the authorized download filename. mDNS, diagnostics, unrelated HTTP errors, and persistent storage must not expose them.
- Runtime product copy must not imply retained history, telemetry, cloud storage, or payload archives; `NFR2` requires none of these exist.
- Exact user-facing strings, tone, terminology for file versus folder, size/speed units, and copy-confirmation language are not specified.

## Accessibility requirements

- Every pointer action has a semantic keyboard equivalent; native selection controls are keyboard reachable (`NFR10`, `UX-DR1`).
- Visible focus is mandatory (`CAP-6`, `NFR10`, `AD-11`).
- Lifecycle and outcome changes are announced through an `aria-live` region (`CAP-6`, `UX-DR5`).
- Focus moves predictably across state changes without trapping keyboard or screen-reader users (Story 1.9).
- Reduced-motion preferences remain usable; transitions cannot be required to understand or complete the flow (`UX-DR6`).
- Drop and lifecycle event listeners register once and clean up on unmount/remount, preventing duplicate announcements or state changes (`NFR10`, `UX-DR8`, Story 1.8).
- No contrast targets, color-use rules, text scaling behavior, high-contrast mode behavior, screen-reader wording, or exact focus destinations are specified by the sources.

## Form factor, platform, and technical UX constraints

- Sender surface: native Wails v2 desktop application on Windows amd64 and macOS; Linux is out of scope (`NFR12`). Receiver surface: modern browser on the same LAN, including mobile-browser download compatibility implied by QR and CORS requirements.
- Frontend stack is React 19, TypeScript, Tailwind CSS v4 via the Vite plugin/CSS import, and optional Framer Motion 13. Tailwind v3/PostCSS configuration is forbidden by the project conventions (`AD-10`, Story 1.8).
- Standard OS chrome and normal start state are fixed. Frameless presentation is not an available design direction.
- One FairDrop process, one live session, one selected root, and one receiver are hard V1 constraints. A second launch restores the existing window (`AD-3`, `FR22`).
- There is no interface-selection UI, IPv6 UI, receiver application, settings, history, notification system, offline queue, or persistent state in scope.
- Directory **logical size** can be shown while staged, but directory **wire total** is unknown during streaming. This distinction must remain legible.
- Backend reset, not a frontend timer or animation, returns terminal screens to Idle. Backend event order and session correlation must prevent stale UI.
- Source names may contain spaces and Unicode; supported long Windows and UNC paths must work without exposing the absolute path (`NFR11`). Layout must therefore tolerate long item names, but truncation/wrapping behavior is not specified.

## Explicit visual preferences and anti-preferences

- Smooth, restrained CSS or Framer Motion transitions between lifecycle presentations; honor reduced motion (`UX-DR6`; original spec §6).
- A prominent progress treatment is expected; the original narrative explicitly calls it a **large progress bar**. Canonical requirements determine when it is determinate versus indeterminate.
- Standard OS window chrome is required; frameless styling is explicitly rejected by the Phase 1 correction and architecture.
- Named component seeds are `DropZone`, `StagedView`, and `TransferView`; these define responsibilities, not a mandated layout.
- No source selects color palette, typography, spacing scale, elevation, shape language, icon style, illustration style, dark mode, brand personality, or design system beyond Tailwind as implementation tooling.

## Unresolved UX questions

1. What visual identity should FairDrop use: brand attributes, colors, type, spacing, shapes, iconography, elevation, light/dark behavior, and platform-specific styling?
2. What exact layout and hierarchy should each public state use, especially QR versus URL prominence, warning placement, long-name handling, and Cancel prominence?
3. What are the approved user-facing strings for Idle instruction, pending Stage, validation failures, each stable error code, `beacon_warning`, trusted-LAN guidance, firewall guidance, success, and reset?
4. After a native browse dialog returns a path, should the frontend immediately call `StageTransfer` or present a confirmation step? The native dialog command itself must not stage.
5. What feedback follows copying the direct URL, and how is that feedback announced accessibly?
6. What exact visual treatment should a known-empty file use? It is `totalKnown=true` with zero total/percent, but only known-positive totals are explicitly determinate.
7. Where should focus move after Stage success, transfer start, terminal outcome, validation error, cancellation/reset, and second-instance restoration?
8. How should the three-second **Done/Error** lease provide enough reading/announcement time without contradicting backend-authoritative reset?
9. What size and throughput units/precision should be used? The original spec says MB/s; canonical stories require human-readable size and throughput.
10. Is firewall guidance onboarding, contextual staged copy, help text, or release documentation? The need is explicit; placement/timing is not.
11. Are any custom receiver-browser success/failure pages desired? Current contracts specify direct attachment plus generic HTTP outcomes, not a receiver UI.
12. Are dark mode, localization/i18n, high-contrast mode, touch-specific behavior, or notifications requirements? None are defined.

## Source conflicts and resolutions

| Conflict | Resolution |
| --- | --- |
| Original title/product/service names use **DeadDrop** / `_deaddrop._tcp`. | Phase 1 Correction 3 and canonical contracts require **FairDrop** / `_fairdrop._tcp`. |
| Original drop setup uses top-level `EnableFileDrop` and `wails_file_drop`. | Corrections 1-2, `AD-11`, and `UX-DR8` require nested `DragAndDrop.EnableFileDrop`, `OnFileDrop(callback, true)`, `OnFileDropOff()`, and inherited `--wails-drop-target: drop`. |
| Original execution plan permits removing window frames. | Phase 1 correction and canonical architecture require standard OS chrome, `Frameless: false`. |
| Original `FileMetadata` omits `sessionId`/`warnings`, and original events omit session/sequence/reset/typed errors/`totalKnown`. | `docs/fairdrop-contracts.md`, `AD-8`, `FR17`, `FR21`, and Stories 1.7-1.8 supply the binding public shapes and event grammar. |
| Original directory size is described as estimated transfer total. | Canonical contract distinguishes staged logical size from unknown ZIP wire total; directory progress is indeterminate with `totalKnown=false`. |
| Original component list foregrounds only Idle/Staged/Transferring. | `CAP-6`, `FR20`, and `UX-DR5` require distinct Done and Error presentations as well. |
| Original says React 18/Tailwind generically and names an inactive QR dependency. | `AD-10` fixes React 19, Tailwind v4, and `boombuler/barcode`; this constrains implementation but does not define visual style. |
| Original “compressed in memory” wording can imply payload-sized buffering. | `CAP-4`, `NFR1`, and `AD-6` require streamed ZIP output with O(buffer) memory and no temporary archive. |
| Original `transfer-error` is `{message}` and reset is implicit. | Canonical event payloads use safe `PublicError`, session/sequence correlation, and explicit `transfer-reset`; frontend lifecycle follows these events only. |
