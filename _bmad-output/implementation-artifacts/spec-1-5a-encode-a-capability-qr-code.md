---
title: 'Story 1.5a — Encode a Capability QR Code'
type: 'feature'
created: '2026-08-24'
status: 'in-progress'
review_loop_iteration: 0
baseline_commit: 'PENDING'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Staging must hand the frontend a QR code for the capability URL, and no `QRPort` implementation exists. Carved out of Story 1.5 so the coordinator is built against a real port rather than a stub, and so its spec stays about concurrency.

**Approach:** A small in-memory adapter over `github.com/boombuler/barcode` v1.1.0 that encodes one string to PNG bytes and never touches disk.

## Boundaries & Constraints

**Always:** Encode entirely in memory and return PNG bytes. Use error correction and a module size that keep a capability URL scannable from a phone at arm's length. Honour the context. Return the existing `qr_failed` code, wrapping the cause with `%w`, and disclose no URL or token in the public message.

**Ask First:** Any dependency other than `github.com/boombuler/barcode` v1.1.0, any change to the `QRPort` shape, writing an image to disk, or returning a data-URI-prefixed string rather than raw bytes.

**Never:** No file writes, no caching, no logging of the encoded content, and no base64 here — the contract puts padded base64 at the coordinator boundary, and this port returns bytes.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Typical capability URL | `http://192.168.1.5:54321/download/<32-hex>` | Valid PNG bytes that decode back to the exact input | N/A |
| Determinism | The same content twice | Byte-identical output | N/A |
| Long content | A URL at the practical upper bound | Encodes, or fails cleanly rather than truncating | `qr_failed` |
| Empty content | `""` | Refused rather than encoding a meaningless code | `qr_failed` |
| Cancelled context | Context already done | No encoding work is used | `cancelled` |
| Nil context | `nil` | Safe typed failure, no panic | `transfer_failed` |
| Encoder refusal | Underlying library returns an error | No partial bytes returned | `qr_failed` wrapping the cause |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:149-151` — **read-only, binding.** The exact `QRPort` shape: `EncodePNG(ctx, content) ([]byte, error)`.
- `docs/fairdrop-contracts.md:124` — `qr_failed` is the code for this port; all codes already exist.
- `internal/transfer/ports.go` — add `QRPort` beside the existing ports.
- `internal/qr/qr.go` — **new.** The adapter.
- `internal/source/source.go:33` — the seam-injection pattern to follow for the encoder.
- `internal/transfer/errors.go:78-85` — `NewError`/`WrapError`; prefer `WrapError` wherever a cause exists.
- `go.mod`, `go.sum` — add only the pinned direct dependency, through Go tooling.

## Tasks & Acceptance

- [ ] `internal/transfer/ports.go` — add `QRPort` verbatim from the contract.
- [ ] `internal/qr/qr.go` — implement it over `boombuler/barcode` with an injectable encoder seam.
- [ ] `go.mod`, `go.sum` — add, tidy, and verify `github.com/boombuler/barcode` v1.1.0 with no unrelated drift.
- [ ] `internal/qr/qr_test.go` — cover every matrix row; decode the output back and compare to the input.

**Acceptance Criteria:**
- Given a typical capability URL, when `EncodePNG` returns, then the bytes are a valid PNG that decodes back to that exact string.
- Given any failure, when it returns, then the error carries `qr_failed`, preserves its cause through `errors.Is`, and its public message contains neither the URL nor the token.
- Given the finished module, when build, vet, tests, race, formatting, and dependency verification run, then all pass with the dependency pinned at v1.1.0.

## Design Notes

Decoding the output back to the input is the only assertion that proves the code is *correct* rather than merely PNG-shaped; a byte-length or header check would pass for a QR encoding the wrong string.

## Verification

**Commands:**
- `go mod tidy && go mod verify` — graph valid, dependency pinned at v1.1.0.
- `go build ./... && go vet ./... && go test -count=1 ./...` — all pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — race clean (see AGENTS.md for the PATH requirement).
- `gofmt -l .` and `git diff --check` — clean.
- `rg -n 'os.Create|os.WriteFile|base64' internal/qr` — no output: nothing written to disk, no encoding done at this layer.
