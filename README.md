# FairDrop

Ephemeral, trusted-LAN file and folder handoff from a Windows or macOS desktop to a nearby
browser, and from a nearby phone browser back to the desktop. Drop a file, a folder, or up to
10,000 of them together, scan the QR code on the receiving device, download once.
No accounts, no cloud, no settings, no logs, no staged copies. The only thing FairDrop ever
writes is files you choose to receive, into a folder you pick.

Go + Wails v2 on the desktop side; React 19 / TypeScript / Tailwind v4 in the window.

**Current release: [v1.6.0](https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.6.0).**
A receiver can open a link to see the offered item, then press **Download** to claim the
one-shot transfer. After a completed transfer, **Send Again**
revalidates the same selection and offers it with a fresh link and QR code. FairDrop still sends
to one receiver over plain HTTP on a trusted LAN and keeps no state of its own.

**Send multiple items** (new in 1.5.0; up to 10,000 items since 1.5.1): a native drop of
2-10,000 paths, or the **Send multiple items** list with **Add Files** and **Add Folder**,
combines files and folders into one handoff. The receiver sees the item count and total logical
size, then downloads one streamed `FairDrop.zip` whose members are numbered and zero-padded to
the width of the count (`FairDrop/01-<name>` ... for up to 99 items, `FairDrop/001-<name>` ...
for 100 to 999, and so on). The whole selection is checked before anything is offered. Members
are then opened one at a time as the ZIP reaches them, so the number of items does not change
how many files or folders FairDrop holds open (at most one); the first is opened before the
download starts, and a later member that changed or vanished after you pressed Send fails the
whole transfer partway rather than being skipped. One file or one folder still uses the
single-item flow unchanged.

**Receive files from a phone** (new in 1.6.0): choose **Receive** in Idle and pick a destination folder; FairDrop then shows a
one-time QR code. A phone on the same Wi-Fi opens the link, chooses files, and uploads them
once through a plain script-free web form. The files are **saved to the folder you chose**,
into a new `FairDrop YYYY-MM-DD HH.MM` subfolder created when the first file arrives, and the
desktop reports exactly how many files were saved. If the upload stops or you cancel, files
that had finished saving are kept, the file in progress is removed, and the result says so.
**Show in Folder** opens the new subfolder. Receiving is the first FairDrop feature that
writes to your disk, so the "persists nothing" promise above describes sending and
FairDrop's own state: a receive session saves the files you chose to receive and still stores
no settings, history, logs, or remembered folder.

Limits worth knowing before you rely on it:

- The destination must be a plain, writable folder. A folder that is, or sits under, a link or
  reparse point is refused, which includes OneDrive-style placeholder folders.
- A volume that cannot rename a file without replacing another, or create a hard link, cannot be
  received into safely. This includes **FAT and exFAT volumes on macOS**: FairDrop fails closed
  there and reports the upload as incomplete rather than risk overwriting a file.
- An upload must leave at least 3 GiB free on the destination volume; a larger one is refused
  and the session keeps waiting.
- Marking received files as downloaded from the internet (macOS quarantine, Windows
  Zone.Identifier) is best-effort. Where a volume cannot store it, the file is kept and the
  result warns about it.
- Like sending, receiving is plain HTTP on a trusted network with no encryption or
  authentication, and the link works for one upload.

## Using FairDrop

1. Launch FairDrop. The first run asks for firewall access — allow it on **Private networks
   only**, and leave Public off. Only one copy runs at a time; launching it again restores the
   window you already have.
2. Give it one file or folder, either by dropping it on the drop zone or through the
   **Choose File or Folder** button inside it. That button opens a small menu because Windows'
   native dialog cannot offer both kinds at once; either item leads to the matching chooser.
   To send several items, drop 2-10,000 paths together or choose **Send multiple items**, build the
   list with **Add Files** and **Add Folder**, and press **Send**.
3. Scan the QR code from a browser on the same Wi-Fi, or use **Copy Link** or **Show Link** to
   open the direct link instead. The receiving browser shows the item first. Tap **Download** there;
   the first valid POST claims the one-shot transfer.
4. A folder, or a selection of several items, arrives as a ZIP, streamed rather than staged, so
   nothing extra is written on the sending side.
5. After a completed transfer, choose **Send Again** to offer the same file, folder or selection
   with a fresh QR code. FairDrop checks every item again; if that check fails, choose them again.
   **Send Another** opens the file or folder chooser, and **Done** dismisses the result.

Cancel at any point. The window returns to idle and forgets the selection. Send Again remembers
only the path or paths of the completed item in memory until it is dismissed or replaced; there is
no persistent history or staged copy.

## Trust model

V1 is plain HTTP on a trusted LAN. The single-use capability URL prevents blind discovery of a
transfer; it does not protect against an observer on the same network. There is no encryption,
no authentication, no relay, and no receiver app — a modern browser on the same Wi-Fi is the
receiver.

Release artifacts match that limit: no code signing, no notarization, no auto-update, and no
Linux packaging. The macOS build carries only Wails' ad-hoc `codesign --sign -`, present so the
OS will launch it, not a developer-identity signature or notarization.

## Build and verify

`.github/workflows/verify.yml` is the canonical gate: it runs natively on `windows-latest` and
`macos-latest` for every pull request and every push to `main`/`epic-*`, and
`verify_workflow_test.go` fails, naming the break, if a step or a pin is removed. Run the same
commands locally, in the same order, before pushing.

Go and the Wails CLI are not on the default PATH here, and `-race` needs a C toolchain — see
"Environment and verification pitfalls" in `AGENTS.md` first.

```sh
wails build                          # frontend + Go, emits build/bin/fairdrop.exe
./build/bin/fairdrop.exe             # run from a shell to see the stderr lifecycle log

gofmt -l . && go vet ./...           # must be clean
go tool staticcheck ./...            # go.mod tool directive, not golangci-lint
go test -count=1 ./...
go test -count=1 -race ./...         # requires cgo; see AGENTS.md
cd frontend && npm test              # jsdom suite
cd frontend && npm run test:browser  # real Chromium: reflow, 200% text, targets, forced colors
cd frontend && npm run build
```

A live folder download that fails on a phone leaves no trail inside the app. Run the production
staging-and-archive path over the same folder to rule the archive in or out:

```sh
FAIRDROP_DIAGNOSE='C:\path\to\folder' go test -count=1 -run TestDiagnoseRealFolder -v ./internal/stream/
```

## How it was built

FairDrop was built by a sequence of AI agents, so the documents are the memory rather than a
by-product. If you are picking the project up — human or otherwise — read in this order:

1. `AGENTS.md` — conventions, environment pitfalls, and the testing standards this repo learned
   the hard way. Read it before running anything.
2. `_bmad-output/specs/spec-fairdrop/SPEC.md` — the canonical contract, with its companions:
   `docs/fairdrop-contracts.md` (types, ports, events, error codes, HTTP matrix),
   `docs/fairdrop-architecture.md` (as-built), and the architecture spine's invariants
   (AD-1 … AD-12) under `_bmad-output/planning-artifacts/architecture/`.
3. `_bmad-output/planning-artifacts/epics.md` — the requirements inventory and every story's
   acceptance criteria.
4. `_bmad-output/implementation-artifacts/` — per-story specs and their mutation evidence,
   `sprint-status.yaml`, `deferred-work.md`, the epic retrospectives, and
   `release-evidence.md`, which records what each release actually verified and what it did not.
5. The UX spine under `_bmad-output/planning-artifacts/ux-designs/` — `EXPERIENCE.md` (copy
   registry, flows, announcement ownership) and `DESIGN.md` (tokens, contrast evidence).

Stories are built with the `bmad-build` workflow: a frozen spec, an implementation, three
adversarial review layers, and mutation testing as the acceptance bar — a load-bearing guarantee
that no test fails when you break it is not considered verified. A branch per epic, never a
commit straight to `main`, and a retrospective before the epic closes.

This is a personal project. [Automated verification is the release
gate](docs/release-policy.md); manual device, browser and accessibility observations are
optional and are recorded at the strength they were actually observed, never rounded up to a
pass. Known test failures still block acceptance.

`docs/fairdrop-spec.md` is the original narrative spec, superseded and kept only for
traceability.
