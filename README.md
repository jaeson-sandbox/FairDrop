# FairDrop

Ephemeral, trusted-LAN file and folder handoff from a Windows or macOS desktop to a nearby
browser. Drop one file or folder, scan the QR code on the receiving device, download once.
Nothing is persisted: no accounts, no cloud, no settings, no logs, no staged copies.

Go + Wails v2 on the desktop side; React 19 / TypeScript / Tailwind v4 in the window.

**Current release: v1.3.0.** A redesign of every screen on the Quartz spine that 1.2.0
introduced: smooth entrance motion, a Ready screen built around the QR code with the link
revealed only when asked for, a progress ring, and one clear card when a transfer finishes
or fails — where **Try Again** re-prepares the same item instead of sending you back to
the chooser. Transfer behavior remains the same complete FR1-FR24 scope proven for v1.0.0:
one file or folder, one receiver, plain HTTP on a trusted LAN, nothing persisted.

## Using it

1. Launch FairDrop. The first run asks for firewall access — allow it on **Private networks
   only**, and leave Public off. Only one copy runs at a time; launching it again restores the
   window you already have.
2. Give it one file or folder, either by dropping it on the drop zone or through the
   **Choose File or Folder** button inside it. That button opens a small menu because Windows'
   native dialog cannot offer both kinds at once; either item leads to the matching chooser.
3. Scan the QR code from a browser on the same Wi-Fi, or use **Copy Link** or **Show Link** to
   open the direct link instead. **The first device to open the link gets the download** —
   including a link preview, so avoid pasting it into a chat that fetches URLs.
4. A folder arrives as a ZIP, streamed rather than staged, so nothing extra is written on the
   sending side.

Cancel at any point. The window returns to idle and everything it held is released — there is
no history, because nothing was kept.

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
