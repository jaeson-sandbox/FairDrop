---
title: 'Story 1.5a — Encode a Capability QR Code'
type: 'feature'
created: '2026-08-24'
status: 'done'
review_loop_iteration: 0
baseline_commit: '82fc2b218d7c2a3951006c3d156b267e49799e3d'
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
| Typical capability URL | `http://192.168.1.5:54321/download/<32-hex>` | Valid PNG whose bytes equal a reference encoding of that same exact content | N/A |
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

- [x] `internal/transfer/ports.go` — add `QRPort` verbatim from the contract.
- [x] `internal/qr/qr.go` — implement it over `boombuler/barcode` with an injectable encoder seam.
- [x] `go.mod`, `go.sum` — add, tidy, and verify `github.com/boombuler/barcode` v1.1.0 with no unrelated drift.
- [x] `internal/qr/qr_test.go` — cover every matrix row; capture the seam argument and compare the PNG against a reference encoding of the same content.

**Acceptance Criteria:**
- Given a typical capability URL, when `EncodePNG` returns, then the encoder received that string byte-for-byte through the seam, and the PNG equals a reference produced by encoding the same content directly.
- Given any failure, when it returns, then the error carries `qr_failed`, preserves its cause through `errors.Is`, and its public message contains neither the URL nor the token.
- Given the finished module, when build, vet, tests, race, formatting, and dependency verification run, then all pass with the dependency pinned at v1.1.0.

## Spec Change Log

- **Frozen-row amendment before implementation (2026-08-24):** The matrix required the PNG to "decode back to the exact input", which `boombuler/barcode` cannot do -- it encodes only. Adding a decoder was weighed against this project's deliberately small pinned dependency set and declined by the human, so the row now states what is actually proven: the encoder receives the content byte-for-byte through the seam, and the PNG equals a reference encoding of that same content. Amended before any code was written rather than left overstating what the tests check.

## Design Notes

A true round-trip would need a decoder, and `boombuler/barcode` ships none; adding one for tests alone was weighed against this project's deliberately small pinned dependency set and declined. What is proven instead: the encoder receives the content byte-for-byte through the seam, and the emitted PNG equals a reference encoding of that same content. Together those exclude the failure this layer can actually cause -- truncating, re-encoding, or substituting the content -- and leave QR correctness itself resting on the pinned library. A byte-length or header check alone would pass for a QR of the wrong string, which is why the reference comparison is against content rather than shape.

## Verification

**Commands:**
- `go mod tidy && go mod verify` — graph valid, dependency pinned at v1.1.0.
- `go build ./... && go vet ./... && go test -count=1 ./...` — all pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — race clean (see AGENTS.md for the PATH requirement).
- `gofmt -l .` and `git diff --check` — clean.
- `rg -n 'os.Create|os.WriteFile|base64' internal/qr` — no output: nothing written to disk, no encoding done at this layer.

## Suggested Review Order

- The whole adapter: in-memory only, context honoured at both ends, coded failures.
  [`qr.go:44`](../../internal/qr/qr.go#L44)

- Recovery level and render size are the two choices that decide scannability.
  [`qr.go:20`](../../internal/qr/qr.go#L20)

- The port this implements; base64 belongs to the coordinator, not here.
  [`ports.go:92`](../../internal/transfer/ports.go#L92)

- The content pin: seam argument plus a reference encoding, including the default path.
  [`qr_test.go:40`](../../internal/qr/qr_test.go#L40)
