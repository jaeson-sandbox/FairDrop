# Evidence: Story 3.11: Close the Residual Contract and Copy Gaps

Baseline: `bf925a6`.

## What the owner decided, and when

**2026-09-13, before implementation.** Four new codes and one revision, drafted in the registry's
voice and quoted verbatim in the spec change log so the implementation could not drift from what
was agreed. Rejected at the same time: collapsing them into two broader codes, on the grounds that
a message vague enough to cover three causes is what Story 3.5 spent a story removing.

**2026-09-13, mid-implementation.** The owner questioned the refusal behind `name_unsupported`
rather than its wording: *"I feel like we don't want to make this too restrictive because we DO
want this tool to work seamlessly."* That turned out to be the more important question, and it
changed the design — see below.

## D-112: the rule that was doing two jobs

`SafeArchiveSegment` refused 24 kinds of name. Twelve protected a receiver; twelve protected
nobody.

The second twelve — `< > : " | ? *`, a trailing dot or space, and the Windows device stems — are
ordinary filenames on macOS, on Linux, and on the phone that is this product's flagship receiver.
One of them anywhere inside a folder refused the whole transfer. No other archiver behaves that
way: they store the name and let the extractor decide.

FairDrop was already doing exactly that for one name and not the other. `sanitizeDownloadName`
**strips** offending characters from the archive's own name so the download always works;
`SafeArchiveSegment` **refused** the entire transfer for one entry inside it. Same class of
problem, opposite answers, and the entry path was the harsher one.

Split: `SafeArchiveSegment` keeps the refusals that describe a primitive — traversal, a separator,
a drive prefix a receiver would join onto its destination, a NUL that truncates the name an
extractor writes, U+202E which makes an executable display as a text file, invalid UTF-8.
`PortableArchiveSegment` answers the other question and refuses nothing: the walk counts, and Stage
raises `name_warning` beside the QR code, where Story 3.6's `aria-describedby` already reads it to
a screen reader.

**A security guard was nearly lost doing it.** Story 3.8 had folded the old `volumeQualified` check
into the blanket colon ban, so moving the colon to the portability half deleted the drive-prefix
refusal — and the comment written in the same edit claimed the guard was still there. Restored
explicitly, with a test requiring a drive-prefixed segment to stay refused however the portability
rule moves. The same test then caught `a:b` being used as an example of a safe-but-unportable name:
one letter then a colon is exactly the drive shape, so refusing it is correct.

**Where the warning is knowable.** `Inspect` already walks the whole tree to compute the folder's
logical size. The count rides on that walk, so the warning reaches Staged before the QR appears
rather than during the stream, when nobody is looking at the sender. It is a count, never the
names: AD-9 does not relax because the number is small.

## The registry, and two things the pin caught

Four codes and one revision moved through five places, not four. `internal/transfer/errors_test.go`
keeps its own list and its instruction named only four; it now names all of them, including the
frontend test's independent `backendCodes` and both halves of the contract.

**A pipe cannot live in a markdown table.** The approved `name_unsupported` draft enumerated the
forbidden characters including `|`. Escaping it in `EXPERIENCE.md` would have left the
cross-language pin comparing an escaped string against the unescaped one the code carries — the
exact drift that pin exists to catch. The message gives examples instead, which is also more
honest: the list was never complete.

**`encoding/json` HTML-escapes `<` and `>`.** The pin built its expectation by string concatenation
while the real path marshals, so a correct message containing either failed it. The copy changed
anyway, but the pin is fixed too: it marshals its expectation now, so a future message carrying one
of those characters fails only when it is genuinely wrong.

## The rest of the ids

| id | What changed |
|---|---|
| D-035 | The contract states what a snapshot-less `Complete` publishes: the unknown-total zero snapshot, because the bytes did arrive and `transfer_failed` would lie about a completed transfer |
| D-039 | `sanitizeProgress` enforces the known-total half of the invariant it documented, not just the unknown-total half it already did |
| D-048 | `delegate()` stops fabricating a context, so a pre-window command is refused rather than binding a listener whose events `publish` then drops |
| D-097 | Transfer commands get a cancellable derivation of the Wails context, cancelled when shutdown begins |
| D-103 | A Cancel whose unwind hit a bound for a session that never began reports `cleanup_unconfirmed` |
| D-104 | A missing coordinator reports `not_ready`; a failed clipboard write reports `clipboard_failed` |
| D-106 | A failed cleanup after a malformed acknowledgement is reported rather than swallowed |
| D-111 | `busy` loses "or cancel it", false when the outstanding work is an uninterruptible filesystem call |

**D-097 is the one that changed behaviour rather than words.** Story 3.4 built Cancel and Shutdown
to honour a caller's context and tested that they do; what no test could reach was whether a
shipped FairDrop ever cancels one. Wails builds its application context from `context.Background()`
plus `WithValue` and never wraps it, so `ctx.Done()` never fired and both commands were bounded in
production only by the internal lease bound. The derived context is cancelled by the shutdown hook
*before* the teardown, so a Cancel already waiting on the operation lease is released rather than
holding shutdown behind it for the whole bound. The lifetime context is deliberately not the one
cancelled — it is what events are emitted through, and cancelling it would lose the events shutdown
exists to let finish.

## Mutation table

| # | Mutation | Result |
|---|---|---|
| M1 | change one message in one of the five registry places | KILLED — the cross-language pin, naming all of them |
| M2 | `SafeArchiveSegment` refuses portability names again | KILLED — `a name a receiver may dislike was refused rather than warned about` |
| M3 | drop `volumeQualified` from the security half | KILLED — `"C:evil" is a drive prefix and must stay refused` |
| M4 | the staged-session Cancel keeps reporting `transfer_failed` | KILLED — `want the forced server stop bound re-coded for a session that never began` |
| M5 | re-code the unwind failure without carrying its text | KILLED — `Cancel reported …, which never names the beacon` (Story 3.10's test) |
| M6 | `useTransfer` reports `setup_failed` however the cleanup went | KILLED — `reports an unconfirmed cleanup when quiescing a malformed acknowledgement fails` |
| M7 | the command context is the uncancellable one again | KILLED — `shutdown did not end a Cancel waiting on its context` |
| M8 | shutdown stops cancelling commands | KILLED — same |

## One finding deferred rather than fixed

`app.go`'s `chooseWith` still reports `transfer_failed` when a native chooser fails to open — wrong,
because no transfer existed. None of this story's ids names it, and both candidate codes are also
wrong: `setup_failed` is a claim about the item the user chose, and none was chosen; `not_ready`
blames a second running instance, which a broken dialog is not. Inventing a fifth code would have
been exactly the unreviewed copy this story's Ask First boundary refuses. Recorded as **D-113**
against Story 4.1, which rebuilds the selection controls and has to answer it anyway.

An earlier draft of this change did re-code it to `not_ready` before that reasoning was done. It
was reverted.

## Verification

Read stage by stage:

- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...` — clean
- `go test -count=1 ./...` — 8 packages ok
- `go test -count=1 -race` over the root and `internal/transfer` — ok
- `cd frontend && npx tsc --noEmit` — clean; `npx vitest run` — 504 passing

## Deferrals

One: D-113, stated above with which of the three reasons applies (it needs a human decision, and
the story that will make it is already scoped).
