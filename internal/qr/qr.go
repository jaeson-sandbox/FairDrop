// Package qr implements transfer.QRPort. It encodes one capability URL to PNG
// bytes entirely in memory: nothing is written to disk, nothing is cached, and
// the content is never logged, because that content carries the capability
// token.
package qr

import (
	"bytes"
	"context"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"

	"fairdrop/internal/transfer"
)

const (
	// recoveryLevel trades payload capacity for scannability. A capability URL
	// is short enough that medium recovery costs nothing here, and it keeps the
	// code readable on a screen that is angled, glossy, or partly glared out.
	recoveryLevel = qr.M
	// renderSize is the square pixel size the code is scaled to. Large enough
	// that each module survives a phone camera at arm's length, small enough
	// that the base64 of it stays a reasonable event payload.
	renderSize = 512
)

// encodeFunc is the seam over the barcode library so a test can force an
// encoder refusal without needing content the real encoder would reject.
type encodeFunc func(content string) (barcode.Barcode, error)

// Encoder is the in-memory QR adapter. Its zero value uses the real library.
type Encoder struct {
	encode encodeFunc
}

var _ transfer.QRPort = (*Encoder)(nil)

// New returns a QR encoder.
func New() *Encoder { return &Encoder{} }

// EncodePNG renders content as a QR code and returns the PNG bytes.
func (e *Encoder) EncodePNG(ctx context.Context, content string) ([]byte, error) {
	if ctx == nil {
		return nil, transfer.NewError(
			transfer.ErrTransferFailed,
			"QR encoding requires a context",
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, transfer.WrapError(
			transfer.ErrCancelled,
			"QR encoding cancelled",
			err,
		)
	}
	// An empty code would scan to nothing and would look like a working QR to
	// the sender, so refuse rather than encode a meaningless one.
	if content == "" {
		return nil, transfer.NewError(
			transfer.ErrQRFailed,
			"QR encoding requires content",
		)
	}

	code, err := e.encodeContent(content)
	if err != nil {
		return nil, transfer.WrapError(
			transfer.ErrQRFailed,
			"capability code could not be encoded",
			err,
		)
	}
	if code == nil {
		return nil, transfer.NewError(
			transfer.ErrQRFailed,
			"capability code encoder returned nothing",
		)
	}

	scaled, err := barcode.Scale(code, renderSize, renderSize)
	if err != nil {
		return nil, transfer.WrapError(
			transfer.ErrQRFailed,
			"capability code could not be sized",
			err,
		)
	}

	// Re-check before spending the encode: a cancellation that landed while the
	// code was being built should not produce bytes nobody will use.
	if err := ctx.Err(); err != nil {
		return nil, transfer.WrapError(
			transfer.ErrCancelled,
			"QR encoding cancelled",
			err,
		)
	}

	var rendered bytes.Buffer
	if err := png.Encode(&rendered, scaled); err != nil {
		return nil, transfer.WrapError(
			transfer.ErrQRFailed,
			"capability code could not be rendered",
			err,
		)
	}
	return rendered.Bytes(), nil
}

func (e *Encoder) encodeContent(content string) (barcode.Barcode, error) {
	if e != nil && e.encode != nil {
		return e.encode(content)
	}
	return qr.Encode(content, recoveryLevel, qr.Auto)
}
