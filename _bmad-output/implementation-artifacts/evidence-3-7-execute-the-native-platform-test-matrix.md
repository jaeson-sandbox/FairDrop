# Story 3.7 evidence

## Owner-approved fix continuation — 2026-09-11

The owner approved fixing both blockers, choosing the working macOS permissions
approach, and making manual release observations optional for this personal
project. `docs/release-policy.md` records that decision; active planning/context
and top-level canonical documents point to it. Historical unperformed checks are
not rewritten as passes. Automated native gates and known-defect resolution remain
mandatory. D-110 moves from 3.8 to 3.7 with both deferred ownership and the epic
Closes line changed. The original baseline and the failed-checkpoint transcript
below remain intact.

### Implementation and reasoning

- HTTP: defer natural Complete until `net/http` finishes its actual connection
  writes, including the final zero chunk. Keep-alives are disabled, so StateClosed
  is the observation point; a wrapper records write errors and short writes.
  Request context is unsuitable because Go cancels it before finishRequest;
  cancellation uses the run context. Track the connection through terminal
  publication so Stop cannot close the event lane first. Payloads close before
  finalization; cancellation still force-closes a blocked final write. Prepare's
  empty 410 also finalizes before publishing its original failure. This is
  sender-observed transport success, not receiver saving/opening proof.
- Darwin: use parent-relative `fstatat(AT_SYMLINK_NOFOLLOW)` snapshots, separate
  search/enumeration/content opens and device/inode identity comparisons. A
  snapshot does not pin the leaf. Go's `os.SameFile` cannot compare custom FileInfo,
  so the Darwin identity helper accepts both unix and syscall stat representations.
  No private entitlement, new dependency or content permission is required for
  metadata. Linux O_PATH and all replacement/link/special-file guards remain.
- Native lock smoke observes a real process and fixed private warning, not window
  visibility. Its exact child receives TERM, a five-second grace, then KILL if
  necessary; cleanup no longer waits indefinitely for graceful termination.

### Focused verification, restored implementation

- `go test -count=40 -race -timeout 120s -run '^TestNativePathClassesStageAndDownload$' .`:
  PASS, `ok fairdrop 20.493s`, 240 real downloads. The earlier 54/240 failures below
  are preserved, not excluded from the record.
- `go test -count=10 -race ./internal/server`: PASS, 31.687s.
- Final focused server/source checks: PASS, 3.443s / 0.335s; focused vet/staticcheck,
  shell syntax and diff checks pass. Darwin/Linux source-test compilation and vet
  pass; these are preflight, not native execution.
- Early-terminal mutation: named finalization test fails `unexpected complete event`.
- Ignored-short-write mutation: same test fails `failed final write was not reported
  as transfer_failed`.
- Early-410 mutation: preparation-finalization test fails `unexpected failed event`.
  All three were restored. The native mutation script also covers metadata
  no-follow, representation compatibility, replacement identity and permission
  separation; native results remain required.

Current checkpoint: full sequential verification and native CI are underway;
formal BMAD review and the ten-id closure have not yet been claimed.

### Local full gate after fixes — 2026-09-11

Sequentially executed: Wails v2.15.0 production Windows build (5.303s), unchanged
generated bindings and restored `.gitkeep`, clean gofmt, vet and pinned staticcheck,
all seven Go packages without cache (stream 10.181s), cgo explicitly `1` with gcc
16.1.0, all seven Go packages under race (stream 219.317s), frontend 17 files / 498
tests, line-ending and diff checks. All passed. Foreign CGO-disabled Darwin arm64
and Linux amd64 builds/vet plus native-host bare staticcheck targeting Darwin passed.

Local verbose path coverage passed spaces/Unicode/>260 file and folder downloads,
empty selection, Windows namespace refusal and private lock diagnostic assertions.
UNC fixtures and directory-symlink privileges are unavailable locally, so those
three tests explicitly skipped; CI provisions UNC and requires symlink capability.
These skips do not close their matrix rows. No manual observation is claimed.

This is a verified local milestone, committed/pushed under AGENTS.md's handoff
rule so native CI can run. Story status stays in-progress pending actual native
results, full matrix acceptance and formal review.

## Resumption audit — 2026-09-11

Baseline: `d43aa69db42c19324bae9f837b909649ca608099` on
`epic-3-run-reliably-on-supported-desktops`, clean and matching origin before
this audit. Main is `627191466ebd0a14b17aeddf5e576cf19a2e703d`.
This is a resumption of an approved story, not a new architecture or scope decision.

### Current project state

- Sprint tracking records Epics 1 and 2 and their retrospectives done. Epic 3
  Stories 3.1–3.6 are done; 3.7 is in progress with an approved ready-for-dev
  spec, and 3.8–3.12 are backlog. The spec now records its implementation baseline.
- Stories 3.1–3.3 supplied single-instance wiring, native verification and release
  artifacts. Stories 3.4–3.6 bounded lifecycle waits with honest failure diagnostics,
  revised public error copy, and made lost/malformed events observable. Existing
  consumer-owned ports, session/sequence validation, and file/folder streaming
  remain the baseline; do not resurrect Phase 1 interfaces.
- GitHub Verify run `34648696619` at this exact HEAD is successful. Its conclusion
  and both Windows/macOS job conclusions were read explicitly. The native jobs
  already execute build, bindings checks, vet/staticcheck, ordinary and race Go
  tests, the frontend suite, and line-ending checks. No fresh local baseline run
  is claimed. Linux native adapter execution is still absent.
- Local tools: Go 1.26.7 windows/amd64; cgo enabled (`1`); Node v24.15.0 satisfies
  `.nvmrc` major `24`; Wails, staticcheck, gcc and gh are available. The module
  floor is Go 1.26.0. `.gitattributes` now normalizes text to LF, superseding old
  assumptions that a normal checkout necessarily contains CRLF.
- `gh release list` reports published latest `v0.1.0` on 2026-09-10. The successful
  release workflow run `34540749917` used `cc9ce580aa63281b5bd8cc4c6de685b0ac79068f`,
  not this branch's current HEAD. Publication is not acceptance evidence for this
  checkpoint. No release or tag was modified during this audit.
- `release-evidence.md` still lacks the receiver OS/browser and confirmation that
  the folder ZIP opened. Native second-launch restoration/accessibility evidence
  is pending. The September 4 phone failure remains unexplained, not fixed.
  These are Story 3.9's human evidence obligations, not automated passes.

### Reconciled drift and reasoning

- Epic context still said eleven stories and omitted the approved 3.12 split.
  The current epics and sprint file agree on twelve. Preserve numbering: 3.10,
  3.11 and 3.12 must finish before 3.9's release evidence, despite their numbers.
- The approved 3.7 scope includes two product fixes (ancestor-only path resolution
  and Darwin lock preflight), so the old context's "no product behaviour" summary
  was inaccurate. Approval remains authoritative; the frozen spec is unchanged.
- The frozen problem statement and old Code Map said POSIX tests never execute.
  Current macOS CI disproves that blanket claim. The implementation gap is Linux
  execution and specific missing platform assertions. Corrected outside the
  frozen section so a new agent does not reason from obsolete evidence.
- Context's twelve-code count was stale: the domain registry now has thirteen,
  including `shutting_down`. Refer to the synchronized registry, not a historic
  count. The residual public-copy work stays with 3.11.
- Context also claimed Stop was quiescent on every return, contradicting 3.4:
  successful teardown proves quiescence; an elapsed bound reports coded failure
  and never claims the resource stopped. Preserve that distinction.

### Scope and acceptance guardrails

Story 3.7 owns D-007, D-014, D-074, D-076, D-078, D-084, D-089, D-094 and D-095.
All nine remain open until implemented and evidenced. D-065/D-068 belong to 3.12;
D-099/D-102 to 3.8. No dependencies, public strings, release promises or refusal
semantics beyond the approved ancestor-resolution decision are silently added.
Native-only rows require native execution; cross-builds and skipped tests do not
close them. Windows UNC syntax and macOS mounted network paths must be described
honestly, without treating a host-inapplicable case as a successful transfer.

BMAD Build step 03 requires a context-free implementation handoff, followed by
matrix audit and adversarial review. Gate execution is sequential per AGENTS.md;
in particular Wails build precedes, and never overlaps, frontend tests. Persist
implementation, mutation and review findings here for the next agent.

### Recovered id-less findings

Two legacy entries still named the completed Story 3.2 without an id. Its evidence
section 4 explicitly left them alone because they were not among its named ids.
The owner/citation test silently skipped `id == ""`, so green tracking tests did
not mean all findings were routed. Entry parsing now validates each record's
unique D-NNN id and single owner before checking citations, independently of field
order. Before repairing the records, the focused test failed with:

```text
--- FAIL: TestEveryOpenDeferredEntryIsCitedByItsOwningStory (0.00s)
    main_test.go:691: deferred entry 69 must have exactly one stable D-NNN id (got "", count 0)
FAIL
FAIL fairdrop 0.075s
```

- D-108: the lost-failure-output finding is discharged by Story 3.2's direct Go
  test commands and retained CI job logs (no tail/truncation pipeline). This closes
  the evidence-retention requirement, **not** the cause of the unreproduced 1.10
  failure; no root cause is claimed. Original observation is preserved unchanged.
- D-109: Story 1.10's missing Blind Hunter and verification-gap layers remain
  outstanding. Story 3.2's section 6 reviews **1.6**, so it does not discharge this
  entry. Re-owned to 3.12, cited in its Closes line and explicit acceptance row.
  This is recovery of existing review debt, not a new product feature.

## Implementation checkpoint — not accepted

BMAD step 03 remains incomplete. The implementation agent added ancestor-only
resolution, portable entry-name refusal, Darwin lock preflight, Linux adapter CI,
native path/rights tests, concurrent WriteTo tests, ZIP64 readback, native mutation
scripts and a macOS process-survival smoke. Parent independently inspected the
boundary changes and ran the full local gate sequentially. No story-owned id is
discharged yet; no formal step-04 review, commit, push, or new native CI run occurred.

### Local gate (2026-09-11)

| Check | Result |
| --- | --- |
| Wails v2.15.0 production Windows build | pass, 5.333 s; bindings regenerated |
| Bindings drift / restored `.gitkeep` | pass |
| gofmt, vet, pinned staticcheck | pass |
| `go test -count=1 -timeout 240s ./...` | all 7 packages pass; stream 9.799 s |
| cgo / compiler | `1`; gcc 16.1.0 UCRT POSIX |
| `go test -count=1 -race -timeout 420s ./...` | **FAIL**: real HTTP folder body incomplete; other packages passed, stream 205.197 s |
| Frontend | 17 files / 498 tests pass |
| LF / `git diff --check` | pass |
| Darwin arm64 + Linux amd64 build/vet, Darwin bare staticcheck | pass with CGO_ENABLED=0; type checking only, **not** native or Foundation bridge proof |

The initial full race failure is preserved here in full, with its final package
output (no race detector warning was emitted; the test failed under race scheduling):

```text
2026/09/11 17:52:25 fairdrop: shutdown begin
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:148: native download incomplete: status=200
FAIL
FAIL fairdrop 1.323s
ok fairdrop/internal/network 1.168s
ok fairdrop/internal/qr 1.460s
ok fairdrop/internal/server 4.300s
ok fairdrop/internal/source 1.356s
ok fairdrop/internal/stream 205.197s
ok fairdrop/internal/transfer 1.522s
FAIL
```

### Response-finalization defect (D-110)

Repeated-run tally: 30 of 40 iterations failed; 54 of 240 downloads ended in
unexpected EOF (48 folder ZIPs, 6 files). Every recorded failure received HTTP 200
but zero body bytes before EOF. This tally is observational, not a probabilistic
test assertion or a claim that every premature close has the same result.

The new full-stack test drives App → coordinator → real source/payload/server over
HTTP. The initial assertion obscured the body-read error, so the parent improved
it to record status, byte count, error type and `errors.Is(io.ErrUnexpectedEOF)`
without a selected path or capability URL. Repeated execution reproduces premature
EOF on **files and folders**, across path classes; it is not a long-path-specific
filesystem defect. The complete repeated-run output is retained below.

Read-only diagnosis by the implementer, independently checked by the parent:
`internal/server/handler.go` closes the payload and calls `r.finish(ServerComplete)`
before returning. Coordinator terminal handling calls Stop; `run.teardown` calls
`http.Server.Close` before awaiting quiescence. Go 1.26.7's `net/http/server.go`
calls `response.finishRequest` only **after** ServeHTTP returns; that flushes the
body, closes chunk framing, and flushes the connection. Thus a completion can
cause forced socket closure while response bytes are still buffered. Existing
server success tests read the entire response before calling Stop, excluding this
ordering from their checks. A handler-level Flush alone does not close chunk
framing and is not a sufficient fix.

Next reproduction should be deterministic: wrap an accepted connection, pass the
initial headers, block its next body/framing write, and observe that Complete has
already been published and Stop closes it. Preserve forced-close cancellation and
failure semantics when fixing natural-completion ordering. This is routed to 3.8
because the defect lives in server/coordinator teardown, outside 3.7's approved
Code Map. It blocks 3.7's matrix acceptance too: re-owning it does not make the
failing HTTP assertion pass. No link to the earlier phone failure is proven.

### macOS metadata rights — native decision still pending (D-074)

The new Darwin test retains the approved expectation that metadata inspection
does not require or grant read access. Apple's kernel source contradicts the
existing O_EVTONLY assumption unless a private process policy is enabled. Parent
independently checked these primary sources at XNU commit
`f6217f891ac0bb64f3d375211650a4c1ff8ca1ea`:

- [open flags / conditional read-right removal](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/vfs/vfs_syscalls.c#L4609).
- [private entitlement required to enable that policy](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_resource.c#L2913).

This is a source-backed concern, **not** a claimed native failure: no new macOS
runner has executed this checkpoint. Do not weaken the test, enable a private
entitlement, or redesign metadata silently. A parent-relative no-follow metadata
query is a candidate design to investigate, not an approved implementation. The
story's Ask First gate covers changes to refusal/rights semantics beyond ancestor
resolution. Obtain the human decision and native evidence before closing D-074.

### Remaining acceptance evidence

- Linux/Darwin native filesystem assertions, capability-skip report, SMB setup,
  mutation scripts and Foundation compilation are pending CI. Local symlink/UNC
  tests skip for missing privileges/fixtures; this is not a pass for those rows.
- ZIP64 entry count and >4 GiB logical-entry/total readback pass locally, including
  the full race package run. A >4 GiB **compressed archive offset** was not tested.
- Concurrent file and archive WriteTo tests passed under race.
- macOS smoke asserts a real built process survives with the fixed degraded-lock
  diagnostic; even when it runs, it does not observe a visible window or focus.
- Selected leaf / traversal refusal mutations and missing-Linux-job mutation have
  not executed. Only tracking's original missing-id failure has been demonstrated.
- The preflight releases its probe before Wails acquires the lock; the remaining
  TOCTOU window is documented, not eliminated.

## Full targeted reproduction output

Tracking hardening was also mutation-checked: temporarily changing D-109 to
D-108 made `TestEveryOpenDeferredEntryIsCitedByItsOwningStory` fail with
`duplicate deferred id D-108`. The mutation was restored and the focused deferred
tests passed. The original id-less records supplied the missing-id red test.

### Resume instructions

1. Read this evidence and the in-progress spec; all changes are local and
   uncommitted on the original epic branch. Nothing has been pushed or released.
2. Obtain approval to bring the D-110 fix forward and investigate/revise Darwin
   metadata acquisition without weakening no-follow/no-read guarantees. Existing
   D-074 and newly routed D-110 remain open; no new pass may paper over them.
3. Add deterministic response-finalization coverage, then fix ordering; a retry,
   delay, handler-only Flush or moving publication into a handler defer is not
   proof of final HTTP framing. Re-run the new full-stack matrix and full gate.
4. Resolve Darwin's design decision in the spec/architecture/contract together,
   then obtain actual Windows/macOS/Linux native results, including all capability
   skips and native mutations. Cross-compilation cannot answer the rights question.
5. Complete step-03 matrix audit before step-04 adversarial review. Do not mark
   3.7 done or merge to main before acceptance. The human release evidence and
   Story 1.10 review debt are still open under their recorded owners.

Command: `go test -count=40 -race -timeout 120s -run '^TestNativePathClassesStageAndDownload$' .`

```text
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.42s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.42s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.43s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.47s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.46s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.49s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.17s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.47s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.46s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.49s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.48s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.08s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.16s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.08s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.44s)
    --- FAIL: TestNativePathClassesStageAndDownload/spaces (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/spaces/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.14s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
--- FAIL: TestNativePathClassesStageAndDownload (0.45s)
    --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/file (0.07s)
            native_matrix_test.go:153: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
        --- FAIL: TestNativePathClassesStageAndDownload/non-ASCII/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
    --- FAIL: TestNativePathClassesStageAndDownload/over-260 (0.15s)
        --- FAIL: TestNativePathClassesStageAndDownload/over-260/folder (0.07s)
            native_matrix_test.go:154: native download body incomplete: status=200 bytes=0 read_error_type=*errors.errorString unexpected_eof=true
FAIL
FAIL	fairdrop	18.599s
FAIL
```
