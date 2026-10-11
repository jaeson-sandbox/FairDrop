# Raise the collection cap to 10,000 with lazy member preparation: implementation evidence

Baseline: `d4dff1d` (v1.5.0 plus the re-derived specs). Spec: [spec-collection-cap-10000.md](spec-collection-cap-10000.md). Verified on macOS arm64 (Go 1.27.0, Node 24.19.0). Windows-native proof is the pull request's CI job. Full logs are in `evidence-collection-cap-10000-logs/`.

## Design in one paragraph

`Payloads.Prepare` prepares **only member 0** before response headers, so a first-member failure is a clean pre-stream refusal. `lazyCollection` (`internal/stream/collection.go`) owns the single current member. The ZIP worker calls `next(index)` to prepare each later member when the stream reaches it, writes it, then `release`s it (closing its descriptor or directory pin) before preparing the next. `archive.Close` calls `shutdown`, which releases the current member only. `install` refuses to hold a second member. The `CollectionSourcePort` and `*WithRetained` source methods are deleted; Stage uses a plain `Inspect`, which reserves one future pin per directory like the singleton path, and no longer reinspects directories. Overlap detection is a component-aware hash index (a pairwise scan of 10,000 roots would be tens of millions of comparisons).

## Matrix row to executed assertion

| Matrix row | Test (all assert literals) |
|---|---|
| 2 to 10,000 roots, one ZIP, ordinals padded to `max(2, digits(count))` | `TestCollectionOrdinalsArePaddedToTheWidthOfTheCount` (2, 16, 99, 100, 999, 1000, 9999, 10000 boundaries); `TestCollectionWideOrdinalStaysWithinTheRootNameBound` (200 runes / 255 bytes with a 5-digit ordinal, trailing-dot trim, fallback); `TestCollectionOfTenThousandMembersStreamsOneAtATime` (10,000 members, first `FairDrop/00001-item-0.txt`, last `FairDrop/10000-item-9999.txt`); `TestCollectionAdmitsTenThousandRootsAndInspectsEachOnce`; `TestStartAdmitsCollectionsOfTwoThroughTenThousandMembersOnly` |
| 0 or 10,001 roots: `invalid_selection` before filesystem work, never trimmed | `TestCollectionCountRefusalPrecedesFilesystemAndNetwork` (0 and 10001, zero external calls); `TestCollectionCapIsExactlyTenThousand`; `TestCollectionPrepareRefusesCountsOutsideTheContractBeforeTouchingTheSource`; `TestNativeCollectionOfHundredsOfItemsStagesAndDownloads` (10,001 paths through the bound command); frontend `refuses an initial 10,001-path drop without truncation` |
| 100 loose files + 100 folders stream completely, never more than one descriptor/pin held | `TestCollectionStreamsHundredFilesAndHundredFoldersWithThreeDigitOrdinals` (interleaved, `001-` to `200-`, independently read with `archive/zip`, exact bytes and empty directories, max held 1, 200 acquired, 200 closed); `TestCollectionHoldsAtMostOneMemberAtAnyMoment` (the destination samples the held count on every write); `TestNativeCollectionOfHundredsOfItemsStagesAndDownloads` (180 real items over HTTP) |
| 100 folders unchanged stream within the single-folder handle budget | `TestNativeCollectionManyFoldersAtTheSingleFolderBoundaryStreamTogether` (twelve folders each at the deepest depth a single folder is admitted, streamed together over HTTP; a folder one level deeper is refused alone and inside a collection, with no network) |
| First member changed after Stage: pre-stream refusal, nothing streamed | `TestCollectionFirstMemberChangedIsAPreStreamRefusal` (file removed, file resized, folder removed; no resource held, later members never inspected); `TestCollectionPreparesFirstMemberBeforeHeadersAndLaterMembersOnlyWhenReached` (after Prepare only member 1 acquired; at the first byte on the wire, later members still untouched) |
| Later member changed, replaced or removed: mid-stream abort, unfinalized, earlier resources already closed | `TestCollectionLaterMemberChangedReplacedOrRemovedAbortsUnfinalized` (file removed, file resized, folder removed, folder replaced by a file, last file removed: exact codes, no valid central directory, held 0 before `Close`, exact acquisition counts); `TestCollectionDirectoryIdentityIsPinnedAtItsOwnPreparation` (documents the trade-off and the walk refusal); `TestNativeCollectionReplacedLaterRootAbortsAuthorizedHTTP` |
| Cancel/Close mid-collection releases the current member only | `TestCollectionCancelMidMemberReleasesOnlyTheCurrentMember`; `TestCollectionCloseMidStreamReleasesTheCurrentMemberOnly`; `TestCollectionCloseBeforeWriteReleasesEveryMember`; `TestCollectionCancelledJustAfterALaterMemberOpensClosesIt`; `TestCollectionPrepareCancelledAfterTheFirstMemberOpensReleasesIt`; `TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit`; `TestLazyCollectionRefusesToHoldASecondMember` |
| Draft list with 10,000 rows scrolls, focus visible, count out of 10,000, 10,001st append refused with prior rows kept | `rejects a 10,001st append without losing the 10,000 editable rows`; `accepts appends up to exactly 10,000 rows`; `shows the count out of 10,000, grouped for large lists`; browser `scrolls a 10,000-row list inside the window and keeps keyboard focus visible and in view`; `draftListStyles.test.ts` (scroll container, focus-ring room, forced-colors outline) |
| Metadata `itemCount` 2 to 10,000 accepted, 10,001 refused | `accepts a coherent collection and refuses partial or inconsistent metadata` (2, 10000 accepted; 10001 refused); `stages a draft of exactly 10,000 paths as one batch` |
| Singleton file and folder flows unchanged | Every pre-existing singleton test passes unchanged; the source budget tests (`TestDirectoryHandleBudgetIncludesAncestorsAndPreparedPin` and the rest) are untouched, and `internal/source/source.go` and `prepared.go` are restored byte-for-byte to their pre-collection (c56a39e) form |

Tests that encoded 16 or pin accumulation and were replaced, not deleted: `TestCollectionPinsShareRetainedDirectoryBudget` (replaced by the native single-folder-boundary test), `TestBudgetedSelectionResolvesOnceAndTraversesOnce` (now `TestSelectionResolvesAndTraversesEachCollectionRootOnce`), `TestNativeCollectionSharedDirectoryBoundaryAndHTTP` (now `TestNativeCollectionManyFoldersAtTheSingleFolderBoundaryStreamTogether`), `TestCollectionLaterDirectoryReplacementAbortsWithoutCentralDirectory` (now the later-member abort test plus the identity test), and the `preparePins [1 1]` assertion in `TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit`.

## Verification (ordered, sequential, macOS arm64)

| Step | Result | Log |
|---|---|---|
| `wails build` | Not run: no bound command changed (`StageTransfers` and `SelectFiles` already existed), `frontend/wailsjs` untouched | n/a |
| `gofmt -l .` | empty | `gate-01-gofmt.log` |
| `go vet ./...` | clean | `gate-02-vet.log` |
| `go tool staticcheck ./...` | clean | `gate-03-staticcheck.log` |
| `go test -count=1 ./...` | 9 packages ok | `gate-04-go-test.log` |
| `CGO_ENABLED=1 go test -count=1 -race ./...` (`go env CGO_ENABLED` = 1) | 9 packages ok | `gate-06-go-race.log` |
| `npm run build` | passed | `gate-07-frontend-build.log` |
| `npm test` | 23 files, 869 tests passed | `gate-08-frontend-test.log` |
| `npm run test:browser` | 87 browser tests plus the live receiver fixture passed; `frontend/browser/captures/` reverted with `git checkout` | `gate-09-frontend-browser.log` |
| line-ending check (`git ls-files --eol`) | no `crlf`/`mixed` | n/a |
| `GOOS=darwin/linux/windows go build ./...` | all exit 0 | n/a |
| bare `staticcheck ./...` under `GOOS=darwin`, `windows`, `linux` | all exit 0; `GOOS=windows go vet ./...` exit 0 | n/a |

## Mutation totals

| Script | Result | Logs |
|---|---|---|
| `_bmad-output/implementation-artifacts/mutate-multiple-selected-items.py` (Go) | **33/33 killed**, every case `baseline=pass mutant=named assertion` | `go-mutations/`, `go-mutations-run.log` |
| `scripts/verify-collection-frontend-mutations.py` | **18/18 killed** | `frontend-mutations/`, `frontend-mutations-run.log` |
| `bash scripts/verify-native-mutations.sh` | exit 0; **51 baselines passed, 68/68 darwin mutations killed** (the Windows-only case runs in native CI); source restored | `native-mutations-run.log` |

The first Go-inventory run killed 30/33. The three rejections were harness faults, not product faults, and are kept as the record: `count_stream` used an assertion fragment the test does not print (`want code` instead of `want "setup_failed"`), `member_copy` needed the stream-time failure message to carry its named fragment (the caller's mutation now surfaces when the later member is prepared, not at Prepare), and `ordinal_width`'s mutant left `strconv` unused (build failure). All three were corrected and the whole inventory rerun from scratch.

The first native-mutations run stopped at `pin reservation mutation did not match`: the anchor `withSelectionRetained(ctx, absolutePath, 1+otherPins,` no longer exists because the source is back to its singleton form. The anchor is now `withSelectionRetained(ctx, absolutePath, 1,` mutated to `0`, still killed by `TestDirectoryHandleBudgetIncludesAncestorsAndPreparedPin`. The failing output is the earlier content of `native-mutations-run.log` (overwritten by the passing rerun); the quoted failure is the whole record.

### Go inventory: what each case guarantees

| Case | Guarantee |
|---|---|
| `count_cap`, `count_cap_lower` | The cap constant is exactly 10,000: 10,001 refused, 10,000 admitted. |
| `count_coordinator`, `count_server`, `count_stream` | Each layer that enforces the cap refuses 10,001 independently. |
| `overlap`, `overlap_duplicate`, `root_overlap`, `overlap_contains`, `component_boundary` | Duplicate, ancestor and descendant overlap are refused in either order, the filesystem root contains everything, and `/a/b` is not an ancestor of `/a/b2`. |
| `overflow`, `slice_copy`, `member_copy`, `handler_copy` | Checked aggregate sum, and caller paths, payload members and server metadata are independently owned (including against a mutation that lands before a lazily prepared member). |
| `stage_pin_reservation` | Stage reserves the one future pin per directory, so a folder at the single-folder boundary streams inside a collection. Replaces `shared_budget`, `coordinator_pin_reservation`, `stream_pin_reservation`. |
| `stage_single_inspect`, `decorator_single_inspect` | Each root is inspected and resolved exactly once (no second count-aware pass). |
| `numbered_names`, `ordinal_width`, `name_fallback`, `name_trim`, `name_bytes` | One-based ordinals padded to the width of the count; fallback, trim and the 255-byte bound apply after the ordinal. |
| `lazy_release`, `lazy_install_guard` | At most one member held at any moment; a second member is structurally refused. |
| `first_member_before_headers` | The first member is prepared in `Prepare`, later ones only when reached. |
| `lazy_later_abort`, `halt_finalization`, `short_file` | A later member that cannot be prepared aborts the ZIP, never finalizes it, and a short file cannot finalize. |
| `shutdown_releases_current`, `close_mid_stream`, `directory_member_close`, `cleanup`, `cancel_cleanup` | Close and cancellation release exactly the current member, directory pins included, and a member abandoned after opening is closed. Replace `cleanup`/`cancel_cleanup`/`directory_member_close` of the all-members design. |

## Documentation changed

`docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md` (including the dated decision entry 2026-10-11), `README.md`, UX `EXPERIENCE.md` and `DESIGN.md`, `AGENTS.md` (Collection handoff), and `ARCHITECTURE-SPINE.md` (not under `_bmad-output/specs/`). Canonical `_bmad-output/specs/**` was not edited.

## Items for the main session

- `selection-contract.md` says ordinals run `0001`-`10000` for 10,000 items. By its own formula (`max(2, digits in the item count)`) 10,000 items have 5 digits, so the range is `00001`-`10000`; `0001` is the width for 1,000 to 9,999 items. The implementation follows the formula, and the tests pin both literal ranges.
- `_bmad-output/specs/spec-multiple-selected-items/.memlog.md` and the architecture `.memlog.md` (line 92) still contain the superseded 16-root text. They are append-only logs, so they were left alone.
- A later folder is not compared with its staged size or modification time (the contract excludes that), so a folder replaced by another folder before its turn is streamed as found. This is the documented trade-off and is pinned by `TestCollectionDirectoryIdentityIsPinnedAtItsOwnPreparation`.

## Only native Windows CI can prove

`TestWindowsSelectionOverlapUsesCaseFoldAndVolumeComponents` (rewritten onto `overlapIndex`; skipped off Windows, and its UNC and volume-root key handling was derived by reading `filepath.Dir`, not run), the Windows-only native mutation case, the Windows handle-budget behavior underneath `TestNativeCollectionManyFoldersAtTheSingleFolderBoundaryStreamTogether` (the boundary depth is discovered at run time from the real inspector), and the creation cost of the 180-item native fixture on a Windows runner. Native macOS WKWebView behavior of the scrolling draft list (focus ring inside the scroller) is unobserved by a human; only Chromium and stylesheet assertions back it.
