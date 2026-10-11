---
title: 'Receive files from a phone browser onto the desktop'
type: 'feature'
created: '2026-10-10'
status: 'in-progress'
baseline_commit: 'f3077fb'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-receiving/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-receiving/receive-contract.md'
  - '{project-root}/_bmad-output/specs/spec-receiver-handoff/receiver-protocol.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md'
---

<frozen-after-approval reason="owner decisions recorded 2026-10-10 in the sibling spec memlog">

## Intent

**Problem:** FairDrop only sends. Getting files from a phone onto the computer needs another tool.

**Approach:** Add a receive session. The desktop picks a destination folder, then shows a one-time QR code. A phone browser uploads files once through a script-free multipart form. Files stream into a fresh time-named subfolder under the rules in receive-contract.md. The coordinator lifecycle, capability tokens, QR, beacon, server and bounded teardown are reused. Sending is unchanged.

The work is delivered in two stories, implemented in this order:
- **Story A (backend):** domain, ports, coordinator, server route and upload streaming, storage sink, OS marking, bound commands and bindings.
- **Story B (frontend):** Receive entry, waiting/progress/outcome views, Show in Folder, copy and announcements.

## Boundaries & Constraints

**Always:** Follow SPEC.md and receive-contract.md exactly: the chosen folder per session, exclusive subfolder and file creation, temp → flush → close → no-replace rename, the 3 GiB free-space reserve on the declared size, no file-count cap, sanitize-and-dedupe names, keeping completed files on failure, best-effort OS download marking, files never executable, and an honest saved count. Stream multipart parts with `request.MultipartReader()`; never use `ParseMultipartForm`/`ReadForm`. Replace the whole-request read deadline for upload bodies with a progress-based inactivity deadline (e.g. via `http.ResponseController.SetReadDeadline`) without changing download behaviour or `TestATransferLongerThanEveryTimeoutStillCompletes`. Keep the coordinator rules: no mutex across adapter calls, generation-checked commits, one operation lease, bounded waits. Sonnet subagents implement; the main session reviews and integrates.

**Ask First:** Any weaker write-safety, resource or disclosure guarantee; a new dependency; persisting anything besides the received files; or scope beyond the contract.

**Never:** Overwrite, append to or open an existing file for writing. Write outside the new subfolder, or use OS temp storage. Report an incomplete upload as complete. Expose the destination path over HTTP, mDNS, diagnostics or the phone page. Interpolate paths into a shell. Commit, push or bump versions from implementation. Hand-edit generated bindings. Relax existing tests.

## I/O & Edge-Case Matrix

| Input/state | Expected behavior | Failure handling |
|---|---|---|
| Receive, chooser dismissed | No change, no listener | No-op |
| Receive with valid folder from IDLE | RECEIVING-WAIT session with token/QR; nothing written | Missing/non-dir/link-like folder → typed refusal before networking |
| Receive outside IDLE | `busy` | No state change |
| GET valid token | Script-free upload page | Wrong token/route/method → generic 404 |
| POST no Content-Length / non-multipart / no file parts | 411 / 400 page; session keeps waiting | Nothing written |
| POST declared size leaves < 3 GiB free | 413 page; session keeps waiting; desktop notified | Nothing written |
| POST valid, 3 files incl. duplicate name | Subfolder `FairDrop YYYY-MM-DD HH.MM`, `x.jpg`, `x (1).jpg`, result page "3 files saved", desktop complete with count | — |
| Subfolder name exists | ` (n)` appended | Exclusive mkdir; never reuse an existing dir |
| Unsafe/empty/path-bearing phone name | Sanitized final component, `file` fallback | Never refused |
| Second POST after claim | 423 while listener lives | No write |
| Connection drop / body past declared size / inactivity / write error / disk full | Partial temp file removed; completed files kept; outcome "incomplete — N saved" | Empty subfolder removed when N = 0 |
| Desktop Cancel mid-upload | Connection closed within bounds; same keep/remove rule; outcome "cancelled — N saved" | Unquiescent bound reported, never success |
| Marking unsupported on volume | File kept; outcome warning | — |
| Show in Folder | Opens subfolder via OS without a shell | Refused when no subfolder |
| Existing send flows | Unchanged | Full existing suites pass |

</frozen-after-approval>

## Code Map

- `internal/transfer/{types,ports,coordinator,session}.go`: session kind (send vs receive); receive admission from IDLE; receive metadata DTO (session ID, URL, QR, destination basename); progress snapshot gains a saved-file count and declared total; terminal outcome carries the saved count, completeness, cancellation, subfolder presence and a marking warning. `ServerStartRequest` gains a receive variant instead of a fake `StagedItem`.
- New consumer-owned `SinkPort` (or equivalent) in `internal/transfer` and a concrete adapter package (e.g. `internal/sink`). It covers destination validation, the free-space query (statfs on POSIX, GetDiskFreeSpaceEx on Windows), lazy exclusive subfolder creation, per-file temp create → write → fsync → close → no-replace rename (`renameatx_np RENAME_EXCL` on darwin, `renameat2 RENAME_NOREPLACE` or link+unlink on linux, `MoveFileEx` without `REPLACE_EXISTING` on windows), name sanitization and dedupe, cleanup and marking. Platform files follow the existing `handle_*` patterns. Never use `t.TempDir()` directly where no-follow rules apply; follow the `fixtureDir(t)` precedent.
- `internal/server/{handler,lifecycle,landing}.go`: a new methodless receive route with the same canonical-path and token checks; an upload page and result pages through `html/template` with the receiver page's headers plus `form-action 'self'`; streaming multipart with an inactivity deadline; claim through the existing authorizer; events on the existing lane.
- `app.go`, `main.go`: `SelectReceiveFolder`, `StartReceive(folder)`, `ShowReceivedFolder()` (exec `open`/`explorer.exe` with an argument vector); `CancelTransfer` reused. Regenerate bindings with `wails build`.
- `frontend/src/transfer/*`, `frontend/src/App.tsx`, `frontend/src/ui/*`: the receive controller path, metadata validation, views and copy (Story B).
- `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, UX DESIGN/EXPERIENCE, README development section: amend in the story that changes the behaviour.

## Tasks & Acceptance

**Execution:**
- [ ] Story A: domain/ports/coordinator receive lifecycle with coordinator tests for admission, cancel races and outcome grammar.
- [ ] Story A: sink adapter with platform no-replace rename, free-space check, sanitization, dedupe, cleanup and marking, with native tests.
- [ ] Story A: server receive route, pages, multipart streaming and inactivity deadline; real-HTTP tests for every matrix row; bound commands and regenerated bindings.
- [ ] Story B: frontend receive flow, views, Show in Folder, copy and announcements, with jsdom and rendered-browser tests.
- [ ] Contracts, architecture decision log, Quartz UX and README updated; evidence file with matrix coverage and scoped mutation proof from one canonical inventory.

**Acceptance Criteria:**
- Given a chosen folder and a real multipart POST of mixed files with duplicate names, when the upload completes, then the new subfolder contains exactly the expected bytes under the sanitized/deduped names, no temp files remain, no pre-existing file changed, and the result page and desktop report the exact count.
- Given a drop, oversize body, inactivity timeout or desktop Cancel partway through a later file, when the session ends, then completed files remain, the partial file is gone, the outcome says incomplete or cancelled with the exact count, and no success is claimed.
- Given a declared size that would leave less than 3 GiB free, when it is POSTed, then nothing is written and the session keeps waiting.
- Given the existing send features, when the full native gate runs, then every existing test passes unchanged.

## Evidence

See `evidence-phone-to-desktop-receiving.md` (created by the implementer).

## Verification

Run Wails before frontend checks, then the canonical ordered gate and platform preflight; never run Wails concurrently with frontend tests. Native Windows and macOS CI is the proof for rename-no-replace, free-space and marking. The synthetic browser suite does not prove WKWebView interaction.
