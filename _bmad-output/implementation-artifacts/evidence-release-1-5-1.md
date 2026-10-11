# FairDrop 1.5.1 — release evidence

Spec: [spec-release-1-5-1.md](spec-release-1-5-1.md). Feature: [spec-collection-cap-10000.md](spec-collection-cap-10000.md), evidence [evidence-collection-cap-10000.md](evidence-collection-cap-10000.md).

## Scope

The branch `epic-collection-cap-1-5-1` was cut from tag v1.5.0 (`e28cc30`). It contains only:
- the 10,000-item collection cap with lazy member preparation and its review patches;
- spec re-derivation, docs and evidence;
- the product version bump (`wails.json`, `frontend/package.json` and the two root fields of `frontend/package-lock.json`; a structured JSON comparison confirmed nothing else in the lockfile changed);
- README and the release notes.

Phone-to-desktop receiving is excluded, as the owner directed on 2026-10-11.

## Verification

- **Local gate, final tree (macOS arm64):** canonical ordered gate passed (Wails build with bindings unchanged, drift, gofmt, vet, staticcheck, Go tests and race tests, 875 frontend tests, 88 browser tests plus the live receiver fixture, line endings, cross-builds).
- **Mutations:** collection Go 35/35, collection frontend 27/27, native 68/68 (darwin).
- **Independent three-lens review:** no high-severity findings. Five fixes were applied and eight findings accepted with reasons (see the collection evidence).
- **Exact-tip CI:** run https://github.com/jaeson-sandbox/FairDrop/actions/runs/38164436550 at `a282903b74eb1db0cdf183c4e40e20f246efb0fc`; all three jobs passed.
- **Tag:** v1.5.1 points at `a282903`.
- **Release run:** https://github.com/jaeson-sandbox/FairDrop/actions/runs/38165726133; all six jobs passed (three gates, two native builds, release). Every job's conclusion was read from GitHub JSON.
- **Assets:** all four were downloaded, and both `shasum -a 256 -c` checks passed.
  - macOS ZIP: `abf54cbb3f6a39d82c99892de80d2b85b933d75113f68161ab992ad66a29f3a8`
  - Windows exe: `1d976f9d8214be7b086e6c0834c338b8d640a4af7ad551dd9af3e4a68da4ba5e`
- **macOS bundle:** both version fields read 1.5.1, and `codesign --verify --deep --strict` passed (ad-hoc signature). The Windows asset is a PE32+ x86-64 GUI executable.
- **Release body:** set to `docs/release-notes-1.5.1.md` and confirmed equal before publication.

## Published — 2026-10-11

https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.5.1, published 2026-10-11T19:12:38Z. It is non-draft and non-prerelease, and the `releases/latest` API returns v1.5.1. The owner authorized publication on 2026-10-11.

Not observed: the downloaded app was not launched by hand, no real large selection was sent from the built app, and WKWebView scrolling in the draft list was not checked. These checks are optional under `docs/release-policy.md`.
