# Technology Currency and Reality-Check Review — Second Pass

> Historical gate pass. Its findings were resolved; the authoritative final verdict is `review-currency.md` (PASS).

**Artifacts reviewed:** `ARCHITECTURE-SPINE.md`, `docs/fairdrop-contracts.md`, and `docs/fairdrop-architecture.md`  
**Review date:** 2026-08-22  
**Lens:** current versions, upstream API existence/fit, live tool defaults, and working-tree truth  
**Verdict:** **CONDITIONAL PASS — all selected technologies and versions are current, real, and mutually compatible; one HTTP implementation ambiguity and two current-state wording gaps remain before the documents are fully safe as an agent handoff.**

This pass rechecked the revised documents against the working tree, exact npm lock entries, Go module/VCS origins, the installed toolchains, and primary upstream sources. No dependency upgrade or framework substitution is required.

## Remaining findings

### TC2-1 — The invalid colon route is gone, but the binding contract still does not name the actual `net/http` route implementation

**Severity:** Medium  
**Affected:** AD-5, `docs/fairdrop-contracts.md` claim/HTTP ordering, `docs/fairdrop-architecture.md` download transaction

The first-pass `/download/:token` issue is partly resolved: all three artifacts now use the conceptual route shape `GET /download/<token>`, and the binding contract correctly requires exact token/method/route behavior. There is no router dependency, and the durable architecture prohibits adding one without architecture review.

However, `<token>` is documentation notation, not valid Go 1.25 `ServeMux` wildcard syntax. A future agent still has to guess between two valid standard-library implementations:

1. register the session's already-known literal pattern, `"GET /download/" + token`, and require exact path equality; or
2. register `GET /download/{token}` and compare `r.PathValue("token")` with the session token.

Both can satisfy the contract. The binding document should choose one and label `<token>` as notation so nobody registers it literally or introduces a router to make it work. The simplest single-session design is the exact literal pattern.

**Primary evidence:** [Go `net/http` ServeMux](https://pkg.go.dev/net/http#ServeMux) defines method-aware patterns and `{name}` wildcards; [Request.PathValue](https://pkg.go.dev/net/http#Request.PathValue) retrieves named wildcard values.

### TC2-2 — The Wails API choice is valid, but AD-3 states future work as already implemented

**Severity:** Low  
**Affected:** AD-3

Wails v2.15.0 locally exposes all selected symbols:

- `options.SingleInstanceLock{UniqueId, OnSecondInstanceLaunch}`
- `runtime.WindowUnminimise(ctx)`
- `runtime.WindowShow(ctx)`

The revised outcome “restore the existing window” is appropriately narrower than promising OS focus. The durable architecture also correctly calls this “a new architecture requirement” that must be added and pinned.

Only the spine's tense is wrong: AD-3 says single-instance locking is “added and pinned in `main_test.go`,” while `main.go`, `app.go`, and `main_test.go` contain no lock, callback, or assertion. Change this to “must be added and pinned.” This is a handoff-accuracy issue, not a technology-fit failure.

**Primary evidence:** [Wails single-instance guide](https://wails.io/docs/v2.12.0/guides/single-instance-lock/) confirms the callback and restore/show requirement. The checked-in Wails v2.15.0 source confirms the exact selected runtime functions in `pkg/runtime/window.go`.

### TC2-3 — Node and Bundler decisions are sound future obligations, but are not yet repository pins

**Severity:** Low / implementation readiness  
**Affected:** AD-10, frontend convention, durable dependencies section

Node 24.19.0 is the current official Node 24 LTS and includes npm 11.17.0. It satisfies locked Vite 7.3.6 (`^20.19.0 || >=22.12.0`) and Vitest 4.1.11 (`^20 || ^22 || >=24`). The local Node 24.15.0/npm 11.12.1 also satisfies both and builds the frontend.

The documents now label Node 24.19.0 as a “planned pin,” and the architecture describes `moduleResolution: "Bundler"` as a required target. That is honest architecture intent. The repository still has no `.node-version`, `.nvmrc`, CI setup, `engines.node`, or `packageManager`, and both TypeScript configs still resolve `"Node"` to legacy `node10`.

A command-line probe showed that the frontend application type-checks successfully when overridden to Bundler resolution. The referenced `tsconfig.node.json` is not independently healthy under its current minimal config: it lacks a modern target/lib and Node types, while the current `npm run build` uses plain `tsc` and therefore does not build the referenced project. The implementation story must change and verify the complete Node-side config, not merely replace the `moduleResolution` string.

**Required implementation acceptance:** add the runtime pin and package metadata, migrate both configs with the supporting Node types/target needed by the Vite config, and make the build or a dedicated check compile both projects. Then run `npm ci`, frontend tests/build, and `wails build` from a clean tree.

**Primary evidence:** [Node 24.19.0 archive](https://nodejs.org/en/download/archive/v24.19.0) identifies it as Krypton LTS with npm 11.17.0; [TypeScript module resolution reference](https://www.typescriptlang.org/tsconfig/moduleResolution) identifies `bundler` as the bundler-oriented mode and `node10` as legacy; [Vite 7 migration guide](https://v7.vite.dev/guide/migration) documents the Node engine floor.

## Prior-finding disposition

| First-pass finding | Second-pass disposition |
| --- | --- |
| Invalid `/download/:token` route | **Substantially fixed.** Invalid syntax removed; TC2-1 asks the binding contract to select the exact standard-library form. |
| Single-instance lock claimed to focus automatically | **Technology issue fixed.** Callback plus `WindowUnminimise`/`WindowShow` are valid and the outcome is now “restore”; only implementation-status tense remains (TC2-2). |
| Bundler resolution asserted as current | **Converted to a future architecture obligation**, but implementation must cover the referenced Node-side project, not only the app config (TC2-3). |
| Node runtime omitted | **Decision fixed.** Node 24.19.0 LTS is current and compatible; repository pin artifacts remain implementation work (TC2-3). |
| Race gate not reproducible | **Fixed.** The durable architecture assigns it to a cgo/C-toolchain-provisioned native CI runner and explicitly says it is not assumed to work in every local Windows shell. |

## Version and API currency matrix

| Claim | Evidence checked | Result |
| --- | --- | --- |
| Go floor 1.25.0 / verified toolchain 1.26.7 | `go.mod`; local `go version go1.26.7 windows/amd64` | Confirmed |
| Wails 2.15.0; v2 stable | `go.mod`, `go.sum`, Go proxy latest metadata, upstream `v2.15.0` tag/release, local v2.15.0 source | Confirmed |
| Wails `WindowUnminimise` / `WindowShow` | Local v2.15.0 `pkg/runtime/window.go` | Confirmed |
| React / React DOM 19.2.8 | Exact lock entries; npm registry latest | Confirmed |
| TypeScript 5.9.3 | Exact lock entry; current app build and Bundler-resolution probe | Confirmed selected baseline; newer major exists but is not required |
| Vite 7.3.6 | Exact lock entry and engine metadata; current frontend build | Confirmed selected baseline; Vite 8 exists but migration is not required |
| Tailwind CSS / Vite plugin 4.3.3 | Lockfile and actual Vite plugin wiring | Confirmed |
| Framer Motion 13.1.1 | Exact lock entry; npm registry latest | Confirmed |
| Vitest 4.1.11 | Exact lock entry and engine metadata | Confirmed |
| Node 24.19.0 LTS planned pin | Official Node archive: current LTS, npm 11.17.0 | Confirmed |
| `hashicorp/mdns` v1.0.7 planned | Go proxy/VCS tag `52e9e65`; upstream maintenance release; repository not archived | Confirmed |
| `boombuler/barcode` v1.1.0 planned | Go proxy/VCS tag `11e32e4`; upstream release; repository not archived; QR encode/scale API exists | Confirmed |

Pinning the existing TypeScript/Vite major versions is reasonable: “technology current” does not require an unplanned major upgrade when the locked baseline is supported, compatible, and verified.

## Race and release reality check

The revised race-runner language matches upstream reality. The local environment reports `CGO_ENABLED=0`, so `go test -race` is not a valid local gate here. Go requires cgo and a C compiler for the race detector, with additional Windows compiler requirements. A provisioned native CI runner is valid; native Windows and macOS release/smoke runners remain separate release obligations.

**Primary evidence:** [Go race detector requirements](https://go.dev/doc/articles/race_detector) and [Go minimum requirements](https://go.dev/wiki/MinimumRequirements).

## Verification performed

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `npm run build`: pass with locked TypeScript 5.9.3 and Vite 7.3.6 on local Node 24.15.0/npm 11.12.1.
- Frontend app config with command-line `moduleResolution Bundler`: type-check pass.
- Standalone current `tsconfig.node.json`: fail, exposing the supporting target/lib/Node-types work described in TC2-3; current `npm run build` does not compile this referenced project.
- Current version/API checks used official Node, Go, Wails, TypeScript, Vite, and GitHub/Go-module upstream data.

## Gate recommendation

No stack or version decision blocks architecture finalization. Correct TC2-1's route implementation ambiguity and TC2-2's false current-state tense in the documents. Carry TC2-3 as an explicit frontend-foundation implementation acceptance criterion; it does not require redesigning the architecture or upgrading majors.
