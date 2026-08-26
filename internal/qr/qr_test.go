package qr

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/boombuler/barcode"
	libqr "github.com/boombuler/barcode/qr"

	"fairdrop/internal/transfer"
)

const capabilityURL = "http://192.168.1.5:54321/download/f2f0b1c4d6e8a0b2c4d6e8a0b2c4d6e8"

// referencePNG encodes content the way a correct adapter must, independently of
// the adapter. Comparing against this is what pins the output to the *content*
// rather than merely to the shape of a PNG: a code built from a truncated or
// substituted string renders a perfectly valid image that differs here.
func referencePNG(t *testing.T, content string) []byte {
	t.Helper()
	code, err := libqr.Encode(content, recoveryLevel, libqr.Auto)
	if err != nil {
		t.Fatalf("reference encode failed: %v", err)
	}
	scaled, err := barcode.Scale(code, renderSize, renderSize)
	if err != nil {
		t.Fatalf("reference scale failed: %v", err)
	}
	var rendered bytes.Buffer
	if err := png.Encode(&rendered, scaled); err != nil {
		t.Fatalf("reference render failed: %v", err)
	}
	return rendered.Bytes()
}

func TestEncodePNGRendersTheExactContent(t *testing.T) {
	t.Parallel()

	var seen []string
	encoder := &Encoder{encode: func(content string) (barcode.Barcode, error) {
		seen = append(seen, content)
		return libqr.Encode(content, recoveryLevel, libqr.Auto)
	}}

	got, err := encoder.EncodePNG(context.Background(), capabilityURL)
	if err != nil {
		t.Fatalf("EncodePNG() error = %v", err)
	}

	// The seam proves nothing was mangled on the way in.
	if len(seen) != 1 || seen[0] != capabilityURL {
		t.Fatalf("encoder received %q, want exactly one byte-identical %q", seen, capabilityURL)
	}
	// The reference proves nothing was substituted on the way out.
	if !bytes.Equal(got, referencePNG(t, capabilityURL)) {
		t.Fatal("rendered PNG does not match a reference encoding of the same content")
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Fatalf("output is not a decodable PNG: %v", err)
	}

	// The assertions above run through the seam, so on their own they say
	// nothing about the path production takes. Check the default encoder
	// against the same reference: without this, an adapter that truncated or
	// rewrote the content inside encodeContent would keep every test green.
	production, err := New().EncodePNG(context.Background(), capabilityURL)
	if err != nil {
		t.Fatalf("EncodePNG() error on the default encoder = %v", err)
	}
	if !bytes.Equal(production, referencePNG(t, capabilityURL)) {
		t.Fatal("the default encoder did not render the exact content it was given")
	}
}

// Different content must produce different bytes, or the reference comparison
// above would pass for an adapter that ignored its input entirely.
func TestEncodePNGDistinguishesContent(t *testing.T) {
	t.Parallel()

	first, err := New().EncodePNG(context.Background(), capabilityURL)
	if err != nil {
		t.Fatalf("EncodePNG() error = %v", err)
	}
	second, err := New().EncodePNG(context.Background(), capabilityURL+"9")
	if err != nil {
		t.Fatalf("EncodePNG() error = %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two different capability URLs rendered identical images")
	}
}

func TestEncodePNGIsDeterministic(t *testing.T) {
	t.Parallel()

	first, err := New().EncodePNG(context.Background(), capabilityURL)
	if err != nil {
		t.Fatalf("EncodePNG() error = %v", err)
	}
	second, err := New().EncodePNG(context.Background(), capabilityURL)
	if err != nil {
		t.Fatalf("EncodePNG() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("the same content rendered different bytes")
	}
}

func TestEncodePNGRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	rendered, err := New().EncodePNG(context.Background(), "")
	assertNoImage(t, rendered, err, transfer.ErrQRFailed)
}

func TestEncodePNGHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	encoder := &Encoder{encode: func(string) (barcode.Barcode, error) {
		t.Error("the encoder ran for an already-cancelled context")
		return nil, errors.New("must not run")
	}}
	rendered, err := encoder.EncodePNG(ctx, capabilityURL)
	assertNoImage(t, rendered, err, transfer.ErrCancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation cause is not preserved")
	}
}

func TestEncodePNGRejectsNilContext(t *testing.T) {
	t.Parallel()

	//nolint:staticcheck // the nil context is the boundary under test
	rendered, err := New().EncodePNG(nil, capabilityURL)
	assertNoImage(t, rendered, err, transfer.ErrTransferFailed)
}

func TestEncodePNGReportsEncoderRefusal(t *testing.T) {
	t.Parallel()

	cause := errors.New("encoder refused")
	encoder := &Encoder{encode: func(string) (barcode.Barcode, error) { return nil, cause }}

	rendered, err := encoder.EncodePNG(context.Background(), capabilityURL)
	assertNoImage(t, rendered, err, transfer.ErrQRFailed)
	if !errors.Is(err, cause) {
		t.Fatal("encoder cause is not preserved through Unwrap")
	}
}

// A typed-nil or plain nil barcode must not reach the scaler.
func TestEncodePNGRejectsAnEmptyEncoderResult(t *testing.T) {
	t.Parallel()

	encoder := &Encoder{encode: func(string) (barcode.Barcode, error) { return nil, nil }}
	rendered, err := encoder.EncodePNG(context.Background(), capabilityURL)
	assertNoImage(t, rendered, err, transfer.ErrQRFailed)
}

// Content past what a QR code can hold must fail rather than silently encode a
// truncated capability URL, which would scan to a link that does not work.
func TestEncodePNGRefusesContentBeyondCapacity(t *testing.T) {
	t.Parallel()

	rendered, err := New().EncodePNG(context.Background(), strings.Repeat("A", 8192))
	assertNoImage(t, rendered, err, transfer.ErrQRFailed)
}

// The public message crosses to the UI, so it must never carry the capability
// URL or the token inside it.
func assertNoImage(t *testing.T, rendered []byte, err error, want transfer.ErrorCode) {
	t.Helper()
	if rendered != nil {
		t.Fatalf("%d bytes were returned alongside a failure", len(rendered))
	}
	if err == nil {
		t.Fatalf("no error returned, want code %q", want)
	}
	if got := transfer.ErrorCodeOf(err); got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	token := capabilityURL[strings.LastIndex(capabilityURL, "/")+1:]
	for label, secret := range map[string]string{"url": capabilityURL, "token": token} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error text disclosed the %s", label)
		}
		if strings.Contains(transfer.PublicErrorOf(err).Message, secret) {
			t.Fatalf("public message disclosed the %s", label)
		}
	}
}
