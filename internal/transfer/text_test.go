package transfer

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNormalizeTextExactAndBoundary(t *testing.T) {
	input := " \t\r\n<&/textarea>\rΩ\n "
	got, err := NormalizeText(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != " \t\n<&/textarea>\nΩ\n " {
		t.Fatalf("normalized text = %q", got)
	}
	for _, bad := range []string{"", " \t\r\n\u00a0", "a\x00b", "a\x1fb", "a\x7fb", string([]byte{0xff}), strings.Repeat("a", 65537), strings.Repeat("a", 131073)} {
		if _, err := NormalizeText(bad); ErrorCodeOf(err) != ErrInvalidText {
			t.Errorf("NormalizeText(%q) error = %v", bad[:min(len(bad), 16)], err)
		}
	}
	if got, err := NormalizeText(strings.Repeat("a", 65536)); err != nil || len(got) != 65536 {
		t.Fatalf("exact cap: len=%d err=%v", len(got), err)
	}
}

// The raw cap is intentionally twice the normalized cap. Every raw-overlimit
// CRLF string would also fail the normalized cap, so output-only tests cannot
// detect removal of this early resource guard. Pin its placement before any
// UTF-8 scan or builder allocation as an explicit source contract.
func TestRawTextLimitPrecedesValidationAndAllocation(t *testing.T) {
	source, err := os.ReadFile("text.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "MaxRawTextBytes = 131072") {
		t.Fatal("raw text cap is not the literal 131072 bytes")
	}
	guard := strings.Index(text, "len(raw) > MaxRawTextBytes || !utf8.ValidString(raw)")
	allocation := strings.Index(text, "b.Grow(len(raw))")
	if guard < 0 || allocation < 0 || guard > allocation {
		t.Fatal("raw text guard missing before UTF-8 processing and allocation")
	}
}

func TestStageTextInvalidInputNeverStartsResources(t *testing.T) {
	h := newHarness(t)
	for _, raw := range []string{string([]byte{0xff}), "a\x01b", strings.Repeat("x", 65537), strings.Repeat("x", 131073), "\u00a0\r\n"} {
		if _, err := h.coordinator.StageText(context.Background(), raw); ErrorCodeOf(err) != ErrInvalidText {
			t.Fatalf("raw %q returned %v", raw[:min(len(raw), 16)], err)
		}
	}
	if got := h.calls.snapshot(); len(got) != 0 {
		t.Fatalf("invalid text acquired resources: %v", got)
	}
}

func TestStageTextUsesNoSourceAndOffersLiteralRoute(t *testing.T) {
	h := newHarness(t)
	metadata, err := h.coordinator.StageText(context.Background(), "hello\r\nworld")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ByteLength != 11 || metadata.URL != "http://192.168.1.50:45678/text/1112131415161718191a1b1c1d1e1f20" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if h.calls.count("source.Inspect") != 0 || h.calls.count("source.Walk") != 0 {
		t.Fatalf("text touched source: %v", h.calls.snapshot())
	}
	if live := h.liveSession(); live == nil || live.text == nil || *live.text != "hello\nworld" {
		t.Fatal("normalized text was not retained")
	}
}

func TestTextWaitingRefreshKeepsValueWithoutSourceAndRotatesIdentity(t *testing.T) {
	h := newHarness(t)
	first, err := h.coordinator.StageText(context.Background(), "same\r\ntext")
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.coordinator.RefreshConnection(context.Background(), first.SessionID, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "text" || result.Text == nil || result.Send != nil || result.Receive != nil {
		t.Fatalf("result = %#v", result)
	}
	if result.Text.SessionID == first.SessionID || result.Text.URL == first.URL || !strings.Contains(result.Text.URL, "/text/") {
		t.Fatalf("identity was not rotated: %#v", result.Text)
	}
	if h.calls.count("source.Inspect") != 0 {
		t.Fatalf("refresh inspected source: %v", h.calls.snapshot())
	}
	if live := h.liveSession(); live == nil || live.text == nil || *live.text != "same\ntext" {
		t.Fatal("refresh changed text")
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), first.SessionID); err == nil {
		t.Fatal("old identity authorized")
	}
}
