# Receiver protocol

The first implementation slice is CAP-1. CAP-2 follows separately.

| Request/state | Response | Side effect |
| --- | --- | --- |
| Exact valid GET, unclaimed | 200 HTML landing page | None: no claim, authorizer call, payload preparation/read or transfer events |
| Repeated/concurrent valid GET | Same metadata page | No reservation |
| Exact valid POST, unclaimed | Existing attachment stream | Atomically reserve, authorize, prepare, stream and finalize |
| Exact valid GET/POST, already reserved | Generic 423 while listener lives | No further claim/read |
| Wrong token/path, malformed path, HEAD or other method | Existing generic 404 | No claim or disclosure |
| POST, source changed or prepare failure | Existing generic pre-stream failure (410) | Existing sender error and teardown |
| Listener stopped/cancelled | Connection unavailable or existing race refusal | Never reopen a retired session |

Keep the methodless ServeMux route `/download/{token}` and constant-time token match. POST reuses the existing download pipeline and finalization semantics; GET must never enter it. The Download form uses a same-origin relative action, never a request-Host-derived URL; it submits an empty-body POST with no cookies or separate authenticated session. Possessing the unguessable URL remains the authority; the form is protection against ordinary GET previews, not against an intentional capability holder or hostile-network observer. Untrusted optional request data is never reflected. No new authentication promise.

Use Go html/template for escaped metadata. Display FairDrop, the display filename, formatted logical size for a file, a clear ZIP explanation for folders, an accessible Download button, and short trusted-local-network/no-copy guidance. Do not label folder logical size as final ZIP size. Use responsive Quartz-compatible styling, system fonts, visible keyboard focus, light/dark and forced-colors support, and no motion requirement. No external resource, script, favicon request dependency, cookies, path disclosure or payload preview. Isolate very long and bidirectional names without horizontal scrolling at 320 CSS pixels.

Set no-store and no-referrer policies, nosniff, and a restrictive CSP that disallows scripts, embedding and external resources while allowing the local form and required embedded style. Metadata HTML must not weaken the attachment stream headers. Render before sending successful headers so rendering errors cannot masquerade as success. Keep every response bounded and ordinary requests cancellation-aware.

Desktop copy changes to explain scan then Download and first downloader, replacing the preview warning. Reconcile README, current architecture/contracts, canonical SPEC CAP-3, UX copy registry and active verification expectations. Preserve historical story/evidence records as history and name their supersession where necessary. Existing tests that genuinely consume bytes must now POST; do not mechanically convert the negative-method tests or remove competing-claim/finalization coverage.

Required proof: real HTTP page visits leave the coordinator STAGED; real POST completes files and ZIPs, concurrent POSTs authorize once, GET/POST races preserve that rule, Cancel releases staged pages, metadata is escaped and paths absent, actual rendered form sends POST, and server-side finalization remains the completion boundary. Mutation proof must name the broken guarantee, with complete retained logs and no shared-worktree mutation races.
