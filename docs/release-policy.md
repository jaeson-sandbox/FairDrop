# Personal-project verification policy

Owner decision, 2026-09-11: FairDrop is being developed for personal use, not as a
client deliverable. Passing automated verification is the release gate. Manual
phone/browser, screen-reader, firewall, appearance and focus observations are
optional; their absence does not block development, story completion or a personal
release. This supersedes earlier mandatory-human-evidence wording in planning,
UX, architecture, completed-story and retrospective documents.

The automated gate still includes native Windows/macOS builds and tests, Go race
checks with cgo, frontend tests, static analysis, bindings and formatting checks,
plus the Linux adapter job introduced in Story 3.7. Linux and cross-compilation do
not substitute for native desktop verification. A known functional failure is not
waived by this policy and must not be hidden by retries or weaker assertions.

Keep evidence honest: record which tests ran, retain failing output, and label
unobserved native UI/browser behavior as unverified. Never invent a manual pass or
claim complete browser/accessibility certification from unit tests. Actual manual
observations remain useful and may be recorded when convenient.

Story 3.9 now consolidates automated release evidence, limitations, optional manual
checks and its existing documentation fixes; it no longer requires a person to
complete a device matrix. Story 3.12's automatable accessibility checks remain in
scope. Historical observations are retained rather than rewritten as successes.

## Recovery while a filesystem lookup is stuck

If selecting an unavailable network folder remains stuck, cancel the selection.
Cancellation stops FairDrop waiting, but cannot interrupt the operating system's
filesystem call. Until that call returns, FairDrop refuses another selection with
`busy` to prevent accumulating background work. Wait for the network lookup to
finish, or close and restart FairDrop, then choose a reachable item. Repeatedly
pressing Cancel will not terminate the underlying OS call. Public wording for this
case is tracked as D-111 in Story 3.11; this guidance is not a claim it is fixed in
the UI already.
