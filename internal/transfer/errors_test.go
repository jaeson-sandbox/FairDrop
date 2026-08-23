package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

// allCodes is transcribed independently of the production tables so that a
// dropped or renamed constant fails a test instead of quietly shrinking the
// contract. Order matches docs/fairdrop-contracts.md.
var allCodes = []ErrorCode{
	ErrInvalidSelection,
	ErrBusy,
	ErrCancelled,
	ErrPathNotFound,
	ErrPathUnsupported,
	ErrSourceChanged,
	ErrNetworkUnavailable,
	ErrServerStartFailed,
	ErrQRFailed,
	ErrBeaconWarning,
	ErrTransferFailed,
	ErrShuttingDown,
}

// TestErrorCodeValuesMatchContract pins the wire values. The frontend reducer
// and the Wails error formatter both switch on these strings, so a rename that
// still compiles here would break the UI silently.
func TestErrorCodeValuesMatchContract(t *testing.T) {
	want := map[ErrorCode]string{
		ErrInvalidSelection:   "invalid_selection",
		ErrBusy:               "busy",
		ErrCancelled:          "cancelled",
		ErrPathNotFound:       "path_not_found",
		ErrPathUnsupported:    "path_unsupported",
		ErrSourceChanged:      "source_changed",
		ErrNetworkUnavailable: "network_unavailable",
		ErrServerStartFailed:  "server_start_failed",
		ErrQRFailed:           "qr_failed",
		ErrBeaconWarning:      "beacon_warning",
		ErrTransferFailed:     "transfer_failed",
		ErrShuttingDown:       "shutting_down",
	}

	if len(allCodes) != 12 {
		t.Fatalf("allCodes has %d entries, want the 12 codes the contract fixes", len(allCodes))
	}
	for code, value := range want {
		if string(code) != value {
			t.Errorf("code value = %q, want %q", string(code), value)
		}
	}
	seen := make(map[ErrorCode]bool, len(allCodes))
	for _, code := range allCodes {
		if seen[code] {
			t.Errorf("duplicate code %q in allCodes: two constants share a value", code)
		}
		seen[code] = true
		if _, ok := want[code]; !ok {
			t.Errorf("code %q is not in the contract table", code)
		}
	}
}

// TestPublicErrorOfMatchesCopyRegistry is the character-for-character check
// against the UX copy registry. The wants below are transcribed from the
// "Stable public error and warning copy" table (beacon_warning from the
// copy.discovery.warning entry in the same document) and use U+2019, not ASCII
// apostrophes.
func TestPublicErrorOfMatchesCopyRegistry(t *testing.T) {
	want := map[ErrorCode]string{
		ErrInvalidSelection:   "Choose exactly one file or folder.",
		ErrBusy:               "Finish or cancel the current transfer before choosing another item.",
		ErrCancelled:          "Transfer canceled.",
		ErrPathNotFound:       "That file or folder is no longer available. Choose it again.",
		ErrPathUnsupported:    "FairDrop can use regular files and folders only. Choose another item.",
		ErrSourceChanged:      "The item changed after it was prepared. Cancel and create a fresh link.",
		ErrNetworkUnavailable: "FairDrop couldn\u2019t find a usable local network. Connect to local Wi-Fi, then try again.",
		ErrServerStartFailed:  "FairDrop couldn\u2019t open a local transfer connection. Check firewall access, then try again.",
		ErrQRFailed:           "FairDrop couldn\u2019t create the QR code. Prepare the item again.",
		ErrBeaconWarning:      "Device discovery isn\u2019t available. The QR code and download link still work.",
		ErrTransferFailed:     "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.",
		ErrShuttingDown:       "FairDrop is closing. Reopen it to start a transfer.",
	}

	for _, code := range allCodes {
		t.Run(string(code), func(t *testing.T) {
			got := PublicErrorOf(NewError(code, "adapter detail that must not surface"))
			if got.Code != code {
				t.Errorf("Code = %q, want %q", got.Code, code)
			}
			if got.Message != want[code] {
				t.Errorf("Message mismatch\n got: %q\nwant: %q", got.Message, want[code])
			}
		})
	}
}

// TestPublicMessagesUseTypographicApostrophe guards the four registry strings
// whose apostrophe is U+2019. A well-meaning "fix" to ASCII ' would still read
// fine in a diff but would break the character-for-character contract.
func TestPublicMessagesUseTypographicApostrophe(t *testing.T) {
	for _, code := range []ErrorCode{ErrNetworkUnavailable, ErrServerStartFailed, ErrQRFailed, ErrBeaconWarning} {
		message := PublicErrorOf(NewError(code, "x")).Message
		if !strings.ContainsRune(message, '\u2019') {
			t.Errorf("%s message %q has no U+2019; the registry spells it with a typographic apostrophe", code, message)
		}
		if strings.ContainsRune(message, '\'') {
			t.Errorf("%s message %q contains an ASCII apostrophe; the registry uses U+2019", code, message)
		}
	}
}

// TestPublicMessagesDiscloseNothing enforces the disclosure matrix on the fixed
// copy itself: no message may look like a path, a basename, or a token.
func TestPublicMessagesDiscloseNothing(t *testing.T) {
	seen := make(map[string]ErrorCode, len(allCodes))
	for _, code := range allCodes {
		message := PublicErrorOf(NewError(code, "x")).Message

		if message == "" {
			t.Errorf("%s has an empty message: the UI would render a blank error panel", code)
		}
		if other, dup := seen[message]; dup {
			t.Errorf("%s and %s share the message %q: the user cannot tell the two failures apart", code, other, message)
		}
		seen[message] = code

		for _, forbidden := range []string{"/", "\\", ":", "%", "..", "0x"} {
			if strings.Contains(message, forbidden) {
				t.Errorf("%s message %q contains %q, which is path- or token-shaped", code, message, forbidden)
			}
		}
		// A token is >=128 bits of encoded randomness, so any long unbroken run
		// of token-alphabet characters is suspicious in human copy.
		for _, word := range strings.Fields(message) {
			if len(word) > 24 {
				t.Errorf("%s message %q contains the %d-character run %q, which is token-shaped", code, message, len(word), word)
			}
		}
	}
}

// TestPublicErrorOfIgnoresAdapterText is the core disclosure guarantee: the
// safe message and the wrapped cause may both name the file, and neither
// reaches the UI.
func TestPublicErrorOfIgnoresAdapterText(t *testing.T) {
	const (
		secretPath  = `C:\Users\jaeso\Documents\salary-review.xlsx`
		secretToken = "k7Qv3nR8sT1uW5xY9zA2bC4dE6fG8hJ0"
	)
	cause := fmt.Errorf("lstat %s: token %s rejected", secretPath, secretToken)
	err := WrapError(ErrPathNotFound, "cannot stat "+secretPath, cause)

	got := PublicErrorOf(err)

	if got.Code != ErrPathNotFound {
		t.Errorf("Code = %q, want %q", got.Code, ErrPathNotFound)
	}
	if got.Message != "That file or folder is no longer available. Choose it again." {
		t.Errorf("Message = %q, want the fixed registry string", got.Message)
	}
	for _, secret := range []string{secretPath, secretToken, "salary-review.xlsx", "jaeso", "lstat"} {
		if strings.Contains(got.Message, secret) {
			t.Errorf("public message leaked %q: %q", secret, got.Message)
		}
	}
	// The local rendering is allowed to carry the detail; that is the point of
	// having two renderings.
	if !strings.Contains(err.Error(), secretPath) {
		t.Errorf("Error() = %q, want it to keep the diagnostic detail for local debugging", err.Error())
	}
}

// TestErrorCodeOfThroughWrappers covers the matrix's "wrapped domain error"
// row: %w nesting must not change the classification.
func TestErrorCodeOfThroughWrappers(t *testing.T) {
	for _, depth := range []int{0, 1, 2, 5} {
		t.Run(fmt.Sprintf("depth=%d", depth), func(t *testing.T) {
			err := NewError(ErrPathUnsupported, "selection is not a regular file")
			for i := 0; i < depth; i++ {
				err = fmt.Errorf("layer %d: %w", i, err)
			}

			if got := ErrorCodeOf(err); got != ErrPathUnsupported {
				t.Errorf("ErrorCodeOf = %q, want %q", got, ErrPathUnsupported)
			}
			got := PublicErrorOf(err)
			if got.Code != ErrPathUnsupported {
				t.Errorf("PublicErrorOf().Code = %q, want %q", got.Code, ErrPathUnsupported)
			}
			if got.Message != "FairDrop can use regular files and folders only. Choose another item." {
				t.Errorf("PublicErrorOf().Message = %q, want the fixed registry string", got.Message)
			}
			if strings.Contains(got.Message, "layer") {
				t.Errorf("wrapper text leaked into the public message: %q", got.Message)
			}
		})
	}
}

// TestErrorsAsFindsCarrier proves the contract's "errors.As works through
// Unwrap" claim for both the interface and the concrete type.
func TestErrorsAsFindsCarrier(t *testing.T) {
	base := WrapError(ErrQRFailed, "encode failed", errors.New("root cause"))
	wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", base))

	var coded CodedError
	if !errors.As(wrapped, &coded) {
		t.Fatal("errors.As did not find a CodedError through two %w layers")
	}
	if coded.Code() != ErrQRFailed {
		t.Errorf("coded.Code() = %q, want %q", coded.Code(), ErrQRFailed)
	}

	var domain *DomainError
	if !errors.As(wrapped, &domain) {
		t.Fatal("errors.As did not find a *DomainError through two %w layers")
	}
	if domain.Code() != ErrQRFailed {
		t.Errorf("domain.Code() = %q, want %q", domain.Code(), ErrQRFailed)
	}
}

// TestWrapErrorPreservesCause keeps %w semantics: the classification is added,
// the underlying identity is not destroyed.
func TestWrapErrorPreservesCause(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := WrapError(ErrTransferFailed, "stream aborted", sentinel)

	if !errors.Is(err, sentinel) {
		t.Error("errors.Is(err, sentinel) = false, want true: WrapError must wrap with %w semantics")
	}
	if got := errors.Unwrap(err); got != sentinel {
		t.Errorf("errors.Unwrap = %v, want the sentinel", got)
	}

	fsErr := WrapError(ErrPathNotFound, "selection does not exist", fs.ErrNotExist)
	if !errors.Is(fsErr, fs.ErrNotExist) {
		t.Error("errors.Is(err, fs.ErrNotExist) = false, want true")
	}
}

// TestNewErrorHasNoCause: an error with nothing underneath unwraps to nil
// rather than to itself, so errors.Is loops terminate.
func TestNewErrorHasNoCause(t *testing.T) {
	err := NewError(ErrBusy, "stage requested outside IDLE")

	if got := errors.Unwrap(err); got != nil {
		t.Errorf("errors.Unwrap = %v, want nil", got)
	}
	if got := ErrorCodeOf(err); got != ErrBusy {
		t.Errorf("ErrorCodeOf = %q, want %q", got, ErrBusy)
	}
	if !strings.Contains(err.Error(), "busy") || !strings.Contains(err.Error(), "stage requested outside IDLE") {
		t.Errorf("Error() = %q, want it to name both the code and the safe message", err.Error())
	}
}

// TestUnknownErrorFallsBackSafely covers the matrix's "unknown error" row: an
// error from outside this vocabulary must be classified, never echoed.
func TestUnknownErrorFallsBackSafely(t *testing.T) {
	unknown := errors.New("dial tcp 192.168.1.44:57312: connection refused")

	if got := ErrorCodeOf(unknown); got != ErrTransferFailed {
		t.Errorf("ErrorCodeOf = %q, want %q", got, ErrTransferFailed)
	}
	got := PublicErrorOf(unknown)
	if got.Code != ErrTransferFailed {
		t.Errorf("Code = %q, want %q", got.Code, ErrTransferFailed)
	}
	if got.Message != "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link." {
		t.Errorf("Message = %q, want the fixed transfer_failed string", got.Message)
	}
	if strings.Contains(got.Message, "192.168") || strings.Contains(got.Message, "dial tcp") {
		t.Errorf("adapter text leaked into the public message: %q", got.Message)
	}

	// Wrapping an uncoded error keeps it uncoded, rather than inventing one.
	if got := ErrorCodeOf(fmt.Errorf("context: %w", unknown)); got != ErrTransferFailed {
		t.Errorf("wrapped unknown ErrorCodeOf = %q, want %q", got, ErrTransferFailed)
	}
}

// TestUnknownCodeFallsBackSafely: a code invented outside this package has no
// registry entry, and an empty error panel is worse than a generic one.
func TestUnknownCodeFallsBackSafely(t *testing.T) {
	err := NewError(ErrorCode("totally_made_up"), "adapter invented a code")

	if got := ErrorCodeOf(err); got != ErrorCode("totally_made_up") {
		t.Errorf("ErrorCodeOf = %q, want the carried code verbatim", got)
	}
	got := PublicErrorOf(err)
	if got.Code != ErrTransferFailed {
		t.Errorf("Code = %q, want the %q fallback", got.Code, ErrTransferFailed)
	}
	if got.Message == "" {
		t.Error("Message is empty: the fallback must still render something")
	}
	if strings.Contains(got.Message, "totally_made_up") || strings.Contains(got.Message, "adapter invented") {
		t.Errorf("invented text leaked into the public message: %q", got.Message)
	}
}

// TestNilErrorHasNoCode: "no error" is not a failure. The contract maps every
// unknown *non-nil* error to transfer_failed, so nil must not be forced into a
// code that would render an error panel for a success.
func TestNilErrorHasNoCode(t *testing.T) {
	if got := ErrorCodeOf(nil); got != "" {
		t.Errorf("ErrorCodeOf(nil) = %q, want the empty code", got)
	}
	if got := PublicErrorOf(nil); got != (PublicError{}) {
		t.Errorf("PublicErrorOf(nil) = %+v, want the zero PublicError", got)
	}
}

// TestPublicErrorJSONShape pins the wire field names the frontend's
// parseCommandError reads.
func TestPublicErrorJSONShape(t *testing.T) {
	encoded, err := json.Marshal(PublicErrorOf(NewError(ErrShuttingDown, "closing")))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	want := `{"code":"shutting_down","message":"FairDrop is closing. Reopen it to start a transfer."}`
	if string(encoded) != want {
		t.Errorf("JSON =\n %s\nwant\n %s", encoded, want)
	}
}
