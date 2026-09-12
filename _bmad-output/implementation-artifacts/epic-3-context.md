# Epic 3 Context: Run FairDrop Reliably on Supported Desktops

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Make FairDrop shippable through locked-toolchain native Windows/macOS verification, native release artifacts and reliable single-instance behavior. Automated checks gate personal releases; manual device observations are optional and support claims stay bounded by actual evidence. Re-planning and subsequent splits bring the epic to twelve stories: 3.10 owns platform/contract decisions, 3.11 residual copy/contracts, and 3.12 runner-produced accessibility evidence. These precede 3.9's release-evidence consolidation. Story 3.7 now also fixes response finalization (D-110) and Darwin metadata acquisition under the owner's 2026-09-11 approval. Story 3.8 retains directory/lifecycle hardening. Cross-compilation is preflight, not native proof; completed file/folder journeys must not regress.

## Stories

**2026-09-12 handoff:** Story 3.7 implementation and both independent review rounds
are complete; all in-scope patches passed native Verify 34678146639 at
5506a81663419b62c073c595c8c9e61ee79c7a82. The owner accepted the checkpoint with
"nice, lets keep going"; final documentation Verify 34679085296 also passed at
6a366121fa9fbcd218935aa2119970e3b4e6913a. Spec and sprint entry are now `done`.
D-111 (generic busy recovery
copy for an outstanding filesystem lookup) remains explicitly owned by Story 3.11;
wait/restart guidance is in docs/release-policy.md. Next implementation is Story
3.8, Harden the Directory Stream. The owner approved its smaller-checkpoint
approach after party discussion: folder safety first, full verification and push,
then cleanup/retry hardening. Root identity covers Prepare-to-WriteTo, not original
Stage; contents remain unsnapshotted and segment checks do not guarantee receiver
case/normalization collision safety. Checkpoint 1 code now uses PreparedDirectory,
a shared 64-retained-handle budget, synchronized borrowed-reader revocation,
portable segment rejection, explicit ZIP modes, and the archive-drain stall guard.
See `evidence-3-8-harden-the-directory-stream.md` for actual verification status;
cleanup D-096/099/101/102 and formal whole-story review remain outstanding.
Epic 3 is not complete.

**Owner-approved resumption (2026-09-11):** fix both newly found blockers in
Story 3.7: response finalization (D-110, brought forward from 3.8) and supported
Darwin no-follow metadata queries in place of O_EVTONLY (D-074). The owner approved
choosing the working macOS approach while retaining file-safety checks. Automated
gates remain mandatory; manual release observations are optional for personal use
under `docs/release-policy.md`. Historical mandatory-human wording is superseded.

Story 3.7 implementation uses Darwin parent-relative no-follow stat snapshots,
not pinned leaf descriptors; later search/enumeration/content opens compare
device/inode, generation and birth timestamp, with Linux O_PATH retained. A filesystem with identical/zero generation and birth fields retains residual snapshot fingerprint collision risk. HTTP natural completion waits for actual
final body/framing writes observed at connection closure, including write-error
and short-write checks. Preparation failure finalizes 410 before its terminal
event; cancellation and streaming failures retain forced closure. The connection
wrapper preserves TCP half-close for unread-body and oversized-header responses.
Selection resolution runs behind coordinator admission in a SourcePort decorator,
with cancellation-aware waits and at most one outstanding resolver call; retries
refuse busy until it returns. App delegation and the stream's raw inspector remain.

- Story 3.1: Enforce One Running FairDrop Instance
- Story 3.2: Automate Reproducible Cross-Platform Verification
- Story 3.3: Produce and Smoke-Test Native Release Artifacts
- Story 3.4: Bound Every Lifecycle Wait and Prove Quiescence
- Story 3.5: Reconcile Public Error Copy with the States It Describes
- Story 3.6: Make Lost and Malformed Events Visible
- Story 3.7: Execute the Native Platform Test Matrix
- Story 3.8: Harden the Directory Stream
- Story 3.9: Record Release Evidence and Optional Manual Checks
- Story 3.10: Settle the Release-Blocking Platform Decisions
- Story 3.11: Close the Residual Contract and Copy Gaps
- Story 3.12: Capture the Accessibility Evidence a Runner Can Produce

## Requirements & Constraints

- Exactly one FairDrop process runs. A second launch restores the existing window (unminimise, then show) with its session and focus intact, starts no competing coordinator, listener, or beacon, and closing still shuts the coordinator down exactly once. The existing Wails options contract (native drop, standard frame, start state, dimensions, lifecycle hooks, bindings, error formatter) is unchanged and its assertions must fail if the single-instance ID or restoration callback changes.
- Verification runs as native Windows and macOS jobs on pull requests and protected-branch pushes, cancelling superseded runs; Linux jobs and cross-compiled output are never release proof. Every Windows/macOS job runs the frontend suite, Go tests, vet, `gofmt -l`, and `wails build` sequentially, plus static analysis and a line-ending check. The race run needs a cgo-capable native runner and must fail loudly when cgo is absent rather than report clean.
- Toolchains are locked: the Go module floor and verified toolchain policy, Wails CLI exactly v2.15.0, Node 24 LTS, `npm ci` with dev dependencies from the committed lockfile. `npm ci --omit=dev`, an unpinned Wails CLI, and default UPX compression are build failures; UPX is opt-in (Apple Silicon and Windows antivirus risk) and disabling it changes no acceptance.
- Each native runner builds its own artifact through Wails after the full gate and publishes it with a checksum. Product name, executable, window title, version metadata, and platform identity all say FairDrop; no stale DeadDrop name or inactive QR dependency survives in shipped metadata or docs. A failing native build or check blocks the release with the platform and check recorded, and any contract or architecture change found during release feeds back into spec, architecture documents, owning story, tests, and `AGENTS.md` before retry.
- Product and release copy call V1 a trusted-LAN plain-HTTP transfer whose capability URL reduces blind discovery but does not protect against a LAN observer. No claim of end-to-end encryption, hostile-network safety, cloud relay, signing, notarization, auto-update, Linux packaging, resume, or multiple receivers; banned vocabulary stays out.
- Every wait the coordinator or server performs while holding a lock or lease ends on a documented bound with a coded failure. Stop and Start never deadlock each other, restart after Stop is a specified and tested contract, Cancel and Shutdown accept a context, and timing windows are forced deterministically in tests rather than left to scheduling. That context must be one a caller can actually cancel, never the runtime's permanently-open one and never a fabricated background context while a sibling command refuses one, and a test must prove cancelling it ends the wait rather than only checking the parameter is passed.
- A dropped, malformed, refused, or unroutable lifecycle event always reaches a visible surface: a refused terminal event renders as failure, never as the cancel-won summary; a panicking or blocking observer cannot hold the operation lease; an event lane closing mid-session synthesises a terminal outcome; undelivered events are logged where a test can observe them; every terminal outcome carries a control so a lost reset cannot strand the window.
- The registry's fixed public messages must describe the state they appear in and offer a recovery that applies; no state reads as an interrupted transfer unless one began. Registry, binding table, Go table, and TypeScript mirror move together under a cross-language pin that fails when any drifts. The same discipline extends to a Cancel against a staged-but-never-claimed session, a Cancel with no coordinator present, and a clipboard write failure, each of which must report what actually happened rather than borrowed copy; and a malformed Stage acknowledgement whose cleanup call fails must have that cleanup guaranteed or the failure surfaced, so the next Stage is never refused busy for a session the user was told did not exist.
- Platform-only guarantees (no-follow opens, special-file guards, the claimed path classes, archive-name predicates on both hosts, ZIP64 thresholds and large-entry archives, once-only writes under race) execute on the native OS they concern. The directory stream refuses at a documented depth bound, pins or documents root replacement between prepare and write, survives a misused borrowed reader under race, and hardens entry names and modes.
- Automated verification is the personal-release gate under `docs/release-policy.md`. Keep optional observations for firewall, native input, restoration, nearby downloads, browser compatibility and assistive technology, identifying platform, artifact, date and observer when available. Missing observations are optional/unverified, never invented passes; known functional failures still require resolution.

## Technical Decisions

- `docs/fairdrop-contracts.md` and `docs/fairdrop-architecture.md` are binding for domain values, error codes, port postconditions, the command/state table, event grammar, the frozen HTTP header matrix, and teardown guarantees. Any change evidence forces, including the pending CORS, `Accept-Ranges`, and `Access-Control-Expose-Headers` decision, amends contract, matching architecture decision, memlog, design guidance, spec, and tests together; a private adapter compatibility rule is never acceptable. Once that header decision is made, an error response reachable cross-origin must carry the same origin policy as the success response, so a receiver page gets a coded failure rather than an opaque one.
- Single-instance locking passes Wails one fixed project UUID, stable across builds and launches, with the second-launch callback wired to the runtime unminimise and show calls; both are pinned in the existing options test beside the frame, drop, and lifecycle assertions. Windows' lock also has fallthrough paths where a first instance goes undetected (an elevated owner, a tight double launch, or a second logged-in user); each either must be stopped from starting a competing coordinator, listener, and beacon, or the residual case is recorded with what the user would observe, without breaking the existing single-instance pins.
- The stack stays put: Wails v2 with no major migration, locked Go and npm dependency sets, `moduleResolution: "Bundler"` in both TypeScript projects. `wails build` regenerates the frontend bindings, so it precedes a standalone frontend build in a fresh tree and never runs concurrently with the frontend suite.
- Coordinator invariants hold through every fix: no external port is called under the state mutex; one per-session operation lease performs Start, Stop, and unwind; Stop is idempotent and force-closing, with quiescence proven on successful return; an elapsed teardown bound reports coded failure and never claims the underlying resource is quiescent; a dedicated drainer keeps server events from blocking teardown; the emission lane numbers events from 1 under the fixed success, failure, and cancellation grammars. The `ServerComplete` payload table also gets a row for its snapshot-less shape, stating what the coordinator publishes then, and `sanitizeProgress`'s known/unknown-total invariant is enforced at that same boundary or documented as the producer's to keep.
- The native window background tracks the canvas token in both themes so neither flashes at launch, read from the OS in Go through a build-tagged implementation per platform, ahead of the webview's first paint. Secure-context browser APIs are absent in the macOS webview: route them through Go or record the gap as a platform limit in the UX contract, and `AGENTS.md` warns against reaching for such an API before that routing exists.
- Deferred-work rule: the deferred-work ids a story's acceptance criteria name are in its scope. A story is not done while any id it names is open; closing one cites it in the story's evidence file and sets its owner to discharged or accepted, and a session that cannot close one re-owns it explicitly rather than leaving it.
- Evidence placement: mutation tables, matrix audits, review triage, loop evidence, and gate transcripts live in `evidence-<slug>.md` beside each spec, never in the spec, which links to it once.
- Release evidence: agents record machine-executed results with their run/artifact identity in `release-evidence.md`; human observations are recorded only when supplied. Never convert an unperformed manual check into a pass.

## UX & Interaction Patterns

- Second-instance restoration shows the existing window with its current state and focus; it neither resets nor duplicates any announcement.
- Firewall preflight and platform recovery copy stay in Idle ahead of the selection controls and reachable from Staged. FairDrop never predicts, restyles, or duplicates the OS prompt; its accessible name, buttons and focus-return order may be recorded in optional native observations.
- Each public error code has a fixed heading, exact message, sole announcement owner, and recovery. `cancelled` never renders as Error; the discovery warning is a Warning and must reach a screen reader through a reachable trigger or an announced region. New codes or strings enter the UX copy registry by stable key before any code emits them.
- Theme follows the OS with no control and no opposite-theme flash; forced colors supersede the palette, and only the tested QR substrate may opt out, after native scan evidence.
- Accessibility remains a product target: unrounded contrast, 320 CSS px reflow, 200% text with text-spacing overrides, 44 px targets and one announcement owner per transition. Automated checks remain required, including Story 3.12's layout and forced-colors evidence. Manual assistive-technology checks are optional/unverified, so CI alone does not establish full WCAG conformance.
- "Supported modern browser" is a gated claim: four sender-to-receiver combinations each need their named scenarios recorded before the support promise. Link-preview consumption of the single-use link is an accepted, disclosed limitation, never reinterpreted as protection. Done describes only sender-observed sending; receiver saving, opening, and Files integration are never claimed.

## Cross-Story Dependencies

Story 3.2 supplies the native gate, 3.3 produces native artifacts, and 3.7 extends native coverage plus Linux Go-only adapter verification. Story 3.7's approved fixes cover ancestor resolution, Darwin metadata/lock handling and response finalization; 3.8 retains directory hardening and lifecycle-bound findings. Stories 3.5/3.6 public error changes stay cross-language pinned. Stories 3.10/3.11/3.12 settle remaining platform, contract, copy and automatable accessibility findings before 3.9 consolidates epic release evidence. Story 3.9 retains the Epic 2 observation without inventing its missing receiver details; optional manual rows no longer block personal releases under `docs/release-policy.md`. Every contract change updates its binding documents and tests together.
