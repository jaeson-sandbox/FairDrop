---
title: 'Story 2.1: Validate and Stage One Directory'
type: 'feature'
created: '2026-09-01'
status: 'done'
review_loop_iteration: 2
baseline_commit: 'a7285e015826a87d1a962d53a96dcadfe1be4771'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop exposes folder selection but rejects directories, and temporarily describes a dropped folder as a file. This blocks Epic 2 and makes the UI promise false.

**Approach:** Extend `SourcePort` with bounded-batch directory preflight and no retained index. Reuse the coordinator transaction and show neutral pending copy until a dropped path's kind is known.

## Boundaries & Constraints

**Always:** Preserve the caller's path byte-for-byte; inspect root, ancestors, and nested entries with cancellation checks, `Lstat`, and native reparse detection. Before enumerating a directory, prove its opened-handle identity matches the inspected object. Source traversal sums non-negative regular-file sizes with `int64` overflow checks; the coordinator rejects every file or directory total outside `0..9007199254740991` before acquiring resources. Return root name/modtime and `ItemDirectory`; close every reader; bound memory by depth plus one fixed batch. Errors preserve coded causes without paths. Ordinary file staging and reverse unwind remain unchanged.

**Ask First:** Any exported contract or dependency change; weakening identity, batching, link/reparse refusal, cancellation precedence, representability, or path-preservation rules.

**Never:** Implement ZIP preparation/streaming, claim-time revalidation, server/progress changes, or temporary archives in this story. Do not use `filepath.WalkDir`, read-all `ReadDir(-1)`, shell commands, destructive path cleaning, a retained entry list, or a shadow source interface.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Safe tree | Nested or empty directory | Original path, basename, root modtime, `ItemDirectory`, exact sum (`0` when empty); no retained handles/index | N/A |
| Unsafe entry | Selected/nested symlink, junction/reparse point, or special file | Stop before descending/following or visiting later entries | `path_unsupported`; no path disclosure |
| Metadata race | Entry disappears, becomes unreadable, or checked directory is replaced before open | Abort, close readers, and never enumerate a mismatched handle | `path_not_found`, `path_unsupported`, or `source_changed`; no path disclosure |
| Cancellation | Cancelled before or during open/read/stat/reparse | Cancellation wins immediately after the active syscall and traversal stops | `cancelled`, preserving cause |
| Invalid size | Negative entry, `int64` overflow, or any item total above `9007199254740991` | Source rejects arithmetic faults; coordinator rejects unrepresentable metadata before setup | `transfer_failed` |
| Native paths | Spaces, Unicode, `..`, supported long path or reachable UNC root | Native APIs; source spelling preserved | Skip only when capability is genuinely absent |
| Trailing separator | Directory or Windows junction selected with trailing separators | Real directory stages; junction cannot bypass reparse refusal | `path_unsupported` for link-like root |

</frozen-after-approval>

## Code Map

- `internal/source/source.go` — parse the original absolute syntax; validate files and drive/share/POSIX roots; traverse through a consumer-owned handle API without reconstructed child paths.
- `internal/source/handle.go` plus platform files — separate metadata/search-only handles from enumeration handles; use atomic no-follow, parent-relative operations. Reuse the repository's pinned `x/sys` module/version.
- `internal/source/{source_test.go,handle_test.go,platform_windows_test.go}` — pin selection, traversal, ownership, mutation, and production defaults, including an ordinary loopback UNC fixture when the host exposes its administrative share.
- `internal/transfer/{coordinator.go,coordinator_stage_test.go}` — reject unsafe JavaScript integer metadata before setup for either item kind.
- `frontend/src/ui/{copy.ts,StagePendingCard.tsx}` and tests — use approved neutral pending copy until native-drop kind is known.
- `EXPERIENCE.md`, `docs/fairdrop-{architecture,contracts}.md`, `deferred-work.md`, and `sprint-status.yaml` — record exact guarantees, coded-error scope, ownership, and story state.

## Tasks & Acceptance

**Execution:**
- [x] Implement platform no-follow handles and component-wise selection. Metadata and lexical traversal request only attribute/search rights; list/read rights are acquired only when the selected or nested directory will be enumerated. `.` retains the current handle, `..` pops a validated handle stack without cleaning the returned string, and escape above the volume/share anchor fails.
- [x] Traverse with one enumeration handle per active depth and `ReadDir(1)`; inspect children through metadata-only parent-relative operations that cannot read devices, compare identities, reject native reparse/link/special entries and active-ancestor cycles, check cancellation after every syscall including close, sum safely, and attempt every owned close.
- [x] Define separator-free root labels: `root` for POSIX `/`, the drive designator without punctuation for a Windows drive root, and the share component for a UNC root. On Windows accept only drive and UNC-share volumes (including their supported extended spellings); reject device namespaces, alternate data streams, and non-local components using behavior equivalent to `filepath.IsLocal`. Ordinary directory names remain their inspected names; receiver download sanitization remains owned by Story 2.2.
- [x] Add deterministic seam tests plus real-default substitution, traverse-only ancestor, metadata-only special-file, parent-rename directory-open, traversal-close, per-operation cancellation, Windows namespace/device-name, trailing-junction, long-path, and loopback-UNC tests. A missing administrative share may skip with its native error; an unset environment variable may not be the only UNC path.
- [x] Add the coordinator safe-integer guard, neutral pending UI copy, and durable docs. Preserve all existing file, unwind, focus, and announcement tests.

**Acceptance Criteria:**
- Given a safe directory, when Stage completes, then exact metadata, URL, QR, session, and warnings return with `isDir=true`, while ordinary file behavior is unchanged.
- Given a path or nested directory is swapped, renamed, or made link-like between checks, when preflight continues, then handle-relative no-follow operations either retain the inspected object or refuse it before enumeration.
- Given wide/deep traversal or any exit, when inspection ends, then fixed-size-read assertions, live-handle counters, code-shape inspection, and a large-tree memory ceiling show one active-depth stack, one entry batch, no read-all call/index, and no leaked handle.
- Given a negative/overflowing size or a file/directory total above `9007199254740991`, when inspection/Stage reaches that boundary, then it fails before setup and releases owned handles.
- Given each load-bearing guarantee is deliberately broken, when its focused mutation runs, then a named behavioral test fails rather than compilation or an unrelated assertion.

## Spec Change Log

- 2026-09-02: Review loop 2 implementation replaced path-based inspection with native parent-relative no-follow handles, separated metadata/search/enumeration rights, rejected unsupported Windows namespaces and components before native opening, and added focused Windows ACL, namespace, parent-rename, loopback-UNC, cancellation, cleanup, and mutation evidence.
- 2026-09-02: Implemented directory preflight, safe-integer staging, neutral native-drop copy, tests, and durable contract/ownership documentation.
- 2026-09-02: Recorded assertion-level mutation evidence and restored review status; this prevents a green compile failure from masquerading as behavioral proof.
- 2026-09-02: Review loop 1 corrected a bad identity/traversal plan. Global Windows `os.Lstat` loads file IDs lazily, so a path replaced before `SameFile` could make both values describe the replacement; reconstructed child paths could also leave the opened parent tree. The plan now requires atomic platform no-follow opens and metadata relative to stable parent handles, precise filesystem-root names, a normal loopback UNC test, an honest memory test/claim, and expanded error documentation. This avoids staging or enumerating a substituted tree. **KEEP:** original path bytes; fixed one-entry reads and reverse close ownership; source overflow plus coordinator safe-integer checks; native reparse checks; active-ancestor cycle refusal; cancellation precedence; neutral pending copy; file behavior and transactional unwind; the ten previously demonstrated mutation targets, which remain minimum evidence rather than sufficient evidence.
- 2026-09-02: Re-derived the implementation from the corrected handle-relative plan. Step-3 audit then found that an early return while releasing lexical ancestors could abandon earlier handles after a close error or cancellation; ownership now flows through one reverse-order close helper, and two multi-ancestor tests plus an assertion-level leak mutation pin both exits. The loopback UNC fixture was also strengthened from path-only evidence to complete directory/file metadata with a nested size sum.
- 2026-09-02: Review loop 2 corrected a permission and namespace design gap. The first native-handle design requested list/read access on every lexical ancestor and opened Unix special entries with `O_RDONLY`, regressing ordinary files behind traverse-only directories and risking device access. It also admitted Windows device namespaces/alternate streams and hand-maintained an incomplete, version-sensitive DOS-device list. The plan now separates metadata/search handles from enumeration, validates supported Windows volumes/components before NT calls, specifies cleanup/cancellation precedence, and requires focused tests for each active operation. This avoids rejecting valid ordinary files or expanding source access beyond the product contract. **KEEP:** atomic no-follow parent-relative lookup; handle-derived identity; caller path bytes and `.`/`..` semantics; fixed one-entry reads; root labels; full loopback UNC metadata; safe size arithmetic; coordinator exact-integer guard; neutral pending copy; the unified all-handles cleanup lesson; all previously green tests and mutation targets, which must be rerun against the re-derived code.
- 2026-09-02: The loop-2 hardening pass removed the final `cmd.exe /c mklink` test fixture and now creates junctions through `FSCTL_SET_REPARSE_POINT`; the no-shell rule therefore applies to both product and verification code. It also split the loopback UNC capability check from FairDrop inspection so a reachable share followed by an inspection regression fails instead of being mislabeled as an unavailable host capability, and added the DOS-device superscript aliases covered by Go's platform contract.
- 2026-09-02: Formal review patch preserved native root labels after lexical `..`, cleared metadata on deferred failure, closed the Parse/cancellation gap, preserved every coded adapter error, and rejected unknown staged item kinds before setup. It also restored baseline file/link/special/cancellation coverage and added explicit enumeration-close, retained-memory, and build-tagged POSIX no-follow/O_PATH tests.

## Design Notes

Anchor traversal at a validated POSIX root, Windows drive root, or UNC share; reject Windows device namespaces before native opening. Linux metadata/search handles use descriptor-returning `O_PATH`; Darwin uses event-only `O_EVTONLY`; both add `O_NOFOLLOW`, while Windows uses attribute-only/search-only `NtCreateFile` handles. Descriptor-derived metadata is required because identity must remain tied to the opened object, while the metadata-only flags avoid reading devices. Lexical ancestors use search/traverse rights, not list/read rights. Only the final selected directory and nested directories acquire enumeration handles, using parent-relative `openat`/`O_NOFOLLOW` or `NtCreateFile`/`FILE_OPEN_REPARSE_POINT`, and identities are compared before `ReadDir(1)`. Cleanup always attempts every owned close; cancellation wins, otherwise the first primary or close failure remains. Retain only component/handle state for active depth plus one entry result—memory is independent of entry count, not claimed independent of path depth. Preflight is not a snapshot; Story 2.2 repeats claim/stream checks and owns ZIP/download-name behavior.

## Verification

**Commands:**
- `gofmt -l .`
- `go test -count=1 ./...` and `go vet ./...` and `go test -count=1 -race ./...`
- `cd frontend && npm test` and `npm run build`
- `wails build`
- Contract greps: exactly one production `SourcePort`; no production `NetworkManager`, `Streamer`, `TransferServer`, or `TransferStats` declaration; no `WalkDir`, `ReadDir(-1)`, or alternate source interface in the changed source package.
- Run focused source tests verbosely and record native long-path, trailing-junction, and loopback-UNC execution versus capability-based skips.
- Run and record assertion-level mutations for no-follow flags, parent-relative lookup, inspected/open identity comparison, post-open reparse refusal, batch size, cycle refusal, cancellation, size arithmetic, safe-integer staging, and root naming; restore after each.
- On Windows, execute table tests for supported drive/UNC and rejected device/ADS/DOS-device forms, plus traverse-only-ancestor behavior where the host can create it. On Unix, execute production no-follow/search-only tests when a native runner exists; otherwise cross-compile and keep the native-runner gap explicitly owned by Story 3.2.

## Evidence

Mutation tables, matrix-coverage audits, review-layer triage and gate transcripts live in
[evidence-2-1-validate-and-stage-one-directory.md](evidence-2-1-validate-and-stage-one-directory.md). An implementer reads this spec; a reviewer reads both.

## Suggested Review Order

**Directory preflight spine**

- Start here: one transaction preserves path bytes, cancellation, identity, and cleanup invariants.
  [`source.go:34`](../../internal/source/source.go#L34)

- Depth-bounded enumeration admits exactly one entry per read and rejects unsafe descendants.
  [`source.go:188`](../../internal/source/source.go#L188)

- Small consumer-owned handle contracts keep platform authority narrow and testable.
  [`handle.go:10`](../../internal/source/handle.go#L10)

**Native no-follow boundaries**

- Windows parses supported volumes before parent-relative, reparse-aware NT opens.
  [`handle_windows.go:39`](../../internal/source/handle_windows.go#L39)

- POSIX opens metadata, search, and enumeration handles with separate rights.
  [`handle_posix.go:82`](../../internal/source/handle_posix.go#L82)

- Platform tests pin substituted-link refusal and search-only ancestor compatibility.
  [`handle_posix_test.go:15`](../../internal/source/handle_posix_test.go#L15)

- Linux tests prove metadata opens cannot read or block on FIFO contents.
  [`handle_linux_test.go:17`](../../internal/source/handle_linux_test.go#L17)

**Coordinator and UI boundary**

- Coordinator refuses unsafe sizes and unknown kinds before acquiring network resources.
  [`coordinator.go:259`](../../internal/transfer/coordinator.go#L259)

- Unknown native-drop kinds use neutral copy until inspection resolves the item.
  [`StagePendingCard.tsx:22`](../../frontend/src/ui/StagePendingCard.tsx#L22)

**Durable contracts and defense evidence**

- Architecture records the handle-rights split and bounded traversal model.
  [`fairdrop-architecture.md:129`](../../docs/fairdrop-architecture.md#L129)

- Contracts define stable errors, path rules, identity checks, and integer bounds.
  [`fairdrop-contracts.md:209`](../../docs/fairdrop-contracts.md#L209)

- Behavioral tests pin coded-error preservation and all seven filesystem operations.
  [`source_test.go:209`](../../internal/source/source_test.go#L209)

- The wide-tree test measures retained heap and live handles over 50,000 entries.
  [`source_test.go:298`](../../internal/source/source_test.go#L298)

- Coordinator tests prove invalid metadata never reaches server, QR, or discovery setup.
  [`coordinator_stage_test.go:796`](../../internal/transfer/coordinator_stage_test.go#L796)
