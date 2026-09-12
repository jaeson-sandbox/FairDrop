# FairDrop

Ephemeral, trusted-LAN file and folder handoff from a Windows or macOS desktop to a nearby
browser. Drop one file or folder, scan the QR code on the receiving device, download once.
Nothing is persisted: no accounts, no cloud, no settings, no logs, no staged copies.

Go + Wails v2 on the desktop side; React 19 / TypeScript / Tailwind v4 in the window.

## Status

| Epic | What it delivers | State |
| --- | --- | --- |
| 1 — Share one file | Native drop or browse, QR/direct URL, one-shot download, honest progress, cancel, accessibility contract | Done, verified on a real phone, merged to `main` |
| 2 — Share one folder | Safe directory staging through native no-follow handles; streamed ZIP with no temp archive | Done and merged; receiver-side observations remain recorded as incomplete |
| 3 — Run reliably on supported desktops | Twelve stories covering native CI/releases, lifecycle and platform hardening, error/event handling, and verification evidence | Stories 3.1–3.6 done; 3.7 implemented and verified, at BMAD review checkpoint |

This is a personal project. [Automated verification is the release gate](docs/release-policy.md);
manual device/browser and accessibility observations are optional, not invented
passes. Known test failures still block acceptance.

## Read these first

The project is built by a sequence of agents, so the documents are the memory. In order:

1. `AGENTS.md` — conventions, environment pitfalls, and the testing standards this repo learned the hard way. Read it before running anything.
2. `_bmad-output/specs/spec-fairdrop/SPEC.md` — the canonical product contract, with its companions:
   - `docs/fairdrop-contracts.md` — binding types, ports, events, error codes, HTTP matrix, source-mutation policy
   - `docs/fairdrop-architecture.md` — the as-built architecture
   - `_bmad-output/planning-artifacts/architecture/.../ARCHITECTURE-SPINE.md` — the invariants (AD-1 … AD-12)
3. `_bmad-output/planning-artifacts/epics.md` — requirements inventory and every story's acceptance criteria.
4. `_bmad-output/implementation-artifacts/` — per-story specs (`spec-*.md`) with their mutation evidence and review triage, `sprint-status.yaml`, `deferred-work.md` (every open finding, each with an owning story), and the epic retrospectives.
5. The UX spine: `_bmad-output/planning-artifacts/ux-designs/.../EXPERIENCE.md` (copy registry, flows, announcement ownership) and `DESIGN.md` (tokens, contrast evidence).

`docs/fairdrop-spec.md` is the original narrative spec and is superseded; it is kept for traceability only.

## Build, run, verify

`.github/workflows/verify.yml` is the canonical gate: it runs natively on `windows-latest`
and `macos-latest` for every pull request and every push to `main`/`epic-*`, in the fixed
order below, and `verify_workflow_test.go` fails, naming the break, if a step or a pin is
removed. Run the same commands locally, in the same order, before pushing.

Go and the Wails CLI are not on the default PATH here, and `-race` needs a C toolchain — see
"Environment and verification pitfalls" in `AGENTS.md` before running these.

```sh
wails build                          # builds frontend + Go, emits build/bin/fairdrop.exe
./build/bin/fairdrop.exe             # run from a shell to see the stderr lifecycle log

gofmt -l . && go vet ./...           # must be clean
go tool staticcheck ./...            # must be clean; go.mod tool directive, not golangci-lint
go test -count=1 ./...               # Go suite
go test -count=1 -race ./...         # requires cgo; see AGENTS.md
cd frontend && npm test && npm run build
```

A live folder download that fails on a phone leaves no trail inside the app. Run the
production staging-and-archive path over the same folder to rule the archive in or out:

```sh
FAIRDROP_DIAGNOSE='C:\path\to\folder' go test -count=1 -run TestDiagnoseRealFolder -v ./internal/stream/
```

## Workflow

Branch per epic (`epic-2-share-one-folder`); never commit to `main` directly. Every
verified-green milestone is committed and pushed. A finished epic gets a retrospective
(`epic-N-retro-*.md`), then merges to `main` with `--no-ff`. Stories are built with the
`bmad-build` workflow: a frozen spec, an implementation, three adversarial review layers, and
mutation testing as the acceptance bar — a load-bearing guarantee that no test fails when broken
is not considered verified.

## Trust model

V1 is plain HTTP on a trusted LAN. The single-use capability URL prevents blind discovery of a
transfer; it does not protect against an observer on the same network. There is no encryption,
no authentication, no relay, and no receiver app — a modern browser on the same Wi-Fi is the
receiver.

Release artifacts match that limit: no code signing, no notarization, no auto-update, and no
Linux packaging. The macOS build carries only Wails' ad-hoc `codesign --sign -`, present so the
OS will launch it, not a developer-identity signature or notarization.
