# Deferred-Work Reconciliation

**Reviewed:** 2026-08-22  
**Source:** `_bmad-output/implementation-artifacts/deferred-work.md`  
**Against:** `ARCHITECTURE-SPINE.md`  
**Verdict:** **CHANGES NEEDED**

The spine gives an adequate architectural disposition for five of the nine validated
findings and partially addresses one. Three findings have no specific disposition.
The architecture should not be treated as fully reconciled with deferred work until
the four flagged rows below are made explicit in the spine or deliberately moved to a
companion with a concrete owner and revisit condition.

For this review, **resolved by the spine** means the design ambiguity is removed or a
specific implementation/verification obligation is established. It does not claim
that future-phase code or CI already exists. **Explicitly deferred** requires both a
bounded exclusion and a condition that triggers reconsideration. A broad stack or
capability binding is not enough to close a concrete finding.

## Finding-by-finding trace

| # | Validated deferred finding | Disposition | Evidence and assessment |
| --- | --- | --- | --- |
| 1 | Backend contracts accept one path while the frontend accepts multiple paths | **RESOLVED + EXPANSION EXPLICITLY DEFERRED** | AD-3 makes the v1 contract unambiguous: exactly one regular file or one directory; zero or multiple paths return typed errors. The Deferred section revisits multi-item staging only with explicit archive naming, collision, metadata, and UX contracts. This both closes the Phase 5 ambiguity and supplies a sound trigger for expanding scope. |
| 2 | `TransferStats.Percent` is undefined when `TotalBytes == 0` for an unknown-size directory stream | **RESOLVED BY SPINE** | AD-7 introduces `totalKnown`. Unknown directory totals use `false/0/0`; known zero-byte files use `true/0/0`; unknown totals render indeterminately. This avoids NaN/Inf and distinguishes unknown size from an empty payload. |
| 3 | No defined signal for a mid-stream failure after HTTP headers are written | **RESOLVED BY SPINE** | AD-6 requires reporting ERROR and aborting via `http.ErrAbortHandler` after a post-header failure. The connection failure prevents a truncated response from appearing to have completed normally. |
| 4 | `TransferServer.Stop` has no idempotency contract | **RESOLVED BY SPINE** | AD-4 requires a single idempotent teardown and adapter Stop operations that are safe before Start and after repeated calls. The cancellation convention reinforces this rule. |
| 5 | No CI runs the verification commands | **RESOLVED AS AN ARCHITECTURAL OBLIGATION; IMPLEMENTATION PENDING** | AD-10 requires race-enabled Go tests, frontend tests/build, Wails build, and native Windows/macOS release runners. The capability map assigns NFR10 to a `CI/release workflow`. This is sufficient architecture coverage, but the original repository-state observation remains true until a workflow is implemented and exercised from a clean checkout. |
| 6 | Drag-and-drop is the only input path; no keyboard/pointer-free staging or live announcement | **FLAGGED -- NO SPINE DISPOSITION** | The UI mapping and AD-8 describe event authority and session filtering, but no invariant requires a keyboard-reachable Browse action, semantic drop-zone controls, focus behavior, or an `aria-live` announcement. The finding appears in `epics.md` as UX-DR5, but the spine neither adopts it nor defers it with a revisit condition. |
| 7 | Path edge cases are untested: spaces, non-ASCII, long paths, UNC, symlinks, and zero-path drops | **FLAGGED -- PARTIAL COVERAGE** | AD-3 covers zero/multiple selections, while AD-6 rejects symlinks and non-regular entries. NFR11 is bound to `internal/transfer` and `internal/stream`, but the spine gives no required behavior or verification matrix for spaces, Unicode, paths beyond 260 characters, or UNC shares. A capability-map row alone does not settle whether each case must work or fail with a typed error. |
| 8 | TypeScript uses legacy `moduleResolution: "Node"` with an exports-map toolchain | **FLAGGED -- NO SPINE DISPOSITION** | AD-10 locks the stack generally, but does not require `"Bundler"` resolution or otherwise dispose of this known incompatibility. The working tree still uses `"Node"` in both frontend TypeScript configs, so this remains actionable rather than historical. |
| 9 | `npm ci --omit=dev` breaks `tsc` because tests are inside the production build type-check scope | **FLAGGED -- NO SPINE DISPOSITION** | Dependency locking and frontend build verification in AD-10 do not define install profiles or separate application and test TypeScript projects. The spine should either require dev dependencies for every build, split build/test type-check scopes, or explicitly state that production-style dependency omission is unsupported and why. |

## Required reconciliation

1. Add an accessibility invariant or convention covering a keyboard-reachable file
   picker fallback, semantic/focus behavior, and announcement of staged/transfer state.
   This should be an adopted v1 requirement rather than a later revisit: it is an
   alternate path into an existing core action, not a new product capability.
2. Expand NFR11 into an explicit path-compatibility rule and test matrix. For spaces,
   Unicode, long Windows paths, and UNC paths, state either supported behavior or a
   typed rejection. Retain the existing zero-selection and symlink rules.
3. Add a concrete TypeScript resolution convention (`moduleResolution: "Bundler"` for
   the Vite application and tooling configs unless a verified config requires
   otherwise), or explicitly assign the correction to the frontend implementation
   plan.
4. Define the supported frontend install/build contract. Prefer one reproducible CI
   contract that installs the dependencies required by `tsc`, tests, Vite, and Wails;
   if `--omit=dev` is intentionally unsupported, state that rather than leaving its
   behavior ambiguous.

## Reconciliation tally

- Resolved by an explicit architecture rule: **4** (#1-#4)
- Established as an architecture/verification obligation: **1** (#5)
- Explicit future expansion with a revisit condition: **1** (the multi-item portion of #1)
- Flagged partial or missing disposition: **4** (#6-#9)

No spine or design-document content was modified by this review.
