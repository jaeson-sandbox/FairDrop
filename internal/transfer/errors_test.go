package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPublicErrorOfExactRegistryCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code    ErrorCode
		message string
	}{
		{ErrInvalidSelection, "Choose exactly one file or folder."},
		{ErrBusy, "Finish or cancel the current transfer before choosing another item."},
		{ErrCancelled, "Transfer canceled."},
		{ErrPathNotFound, "That file or folder is no longer available. Choose it again."},
		{ErrPathUnsupported, "FairDrop can use regular files and folders only. Choose another item."},
		{ErrSourceChanged, "The item changed after it was prepared. Cancel and create a fresh link."},
		{ErrNetworkUnavailable, "FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again."},
		{ErrServerStartFailed, "FairDrop couldn’t open a local transfer connection. Check firewall access, then try again."},
		{ErrQRFailed, "FairDrop couldn’t create the QR code. Prepare the item again."},
		{ErrBeaconWarning, "Device discovery isn’t available. The QR code and download link still work."},
		{ErrTransferFailed, "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link."},
		{ErrShuttingDown, "FairDrop is closing. Reopen it to start a transfer."},
	}

	for _, test := range tests {
		test := test
		t.Run(string(test.code), func(t *testing.T) {
			t.Parallel()
			got := PublicErrorOf(NewError(test.code, "adapter detail that must be ignored"))
			if got.Code != test.code || got.Message != test.message {
				t.Fatalf("PublicErrorOf() = %#v, want code %q and message %q", got, test.code, test.message)
			}
		})
	}
}

func TestDomainErrorWrappingPreservesCodeAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("filesystem failure")
	domainErr := WrapError(ErrPathUnsupported, "metadata unavailable", cause)
	err := fmt.Errorf("outer one: %w", fmt.Errorf("outer two: %w", domainErr))

	if got := ErrorCodeOf(err); got != ErrPathUnsupported {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrPathUnsupported)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not reachable with errors.Is")
	}
	var target *DomainError
	if !errors.As(err, &target) {
		t.Fatal("DomainError is not reachable with errors.As")
	}
	if target.Code() != ErrPathUnsupported {
		t.Fatalf("DomainError.Code() = %q, want %q", target.Code(), ErrPathUnsupported)
	}
}

func TestPublicErrorOfMultiplyWrappedDomainErrorIsExactAndSafe(t *testing.T) {
	t.Parallel()

	const secret = `C:\private\payroll.txt?token=secret`
	err := fmt.Errorf("outer: %w", fmt.Errorf("middle: %w", WrapError(
		ErrPathNotFound,
		"selection does not exist",
		errors.New(secret),
	)))
	want := PublicError{
		Code:    ErrPathNotFound,
		Message: "That file or folder is no longer available. Choose it again.",
	}
	got := PublicErrorOf(err)
	if got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
	if strings.Contains(got.Message, secret) || strings.Contains(got.Message, "payroll.txt") {
		t.Fatalf("PublicErrorOf() leaked wrapped detail: %#v", got)
	}
}

func TestIndependentCodedErrorSurvivesWrapping(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("outer: %w", independentCodedError{code: ErrBusy})
	if got := ErrorCodeOf(err); got != ErrBusy {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrBusy)
	}
	want := PublicError{
		Code:    ErrBusy,
		Message: "Finish or cancel the current transfer before choosing another item.",
	}
	if got := PublicErrorOf(err); got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
}

func TestUnknownErrorsUseFixedFallback(t *testing.T) {
	t.Parallel()

	err := errors.New("unknown adapter detail")
	if got := ErrorCodeOf(err); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrTransferFailed)
	}
	got := PublicErrorOf(err)
	want := PublicError{
		Code:    ErrTransferFailed,
		Message: "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.",
	}
	if got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
}

func TestUnknownCodedErrorUsesPublicFallback(t *testing.T) {
	t.Parallel()

	err := NewError(ErrorCode("not_registered"), "unknown detail")
	if got := ErrorCodeOf(err); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf() = %q, want transfer_failed fallback", got)
	}
	got := PublicErrorOf(err)
	if got.Code != ErrTransferFailed || got.Message != publicMessages[ErrTransferFailed] {
		t.Fatalf("PublicErrorOf() = %#v, want transfer_failed fallback", got)
	}
}

func TestNilErrorsAreSafe(t *testing.T) {
	t.Parallel()

	if got := ErrorCodeOf(nil); got != "" {
		t.Fatalf("ErrorCodeOf(nil) = %q, want empty", got)
	}
	if got := PublicErrorOf(nil); got != (PublicError{}) {
		t.Fatalf("PublicErrorOf(nil) = %#v, want zero value", got)
	}

	var domain *DomainError
	var typedNil error = domain
	if got := ErrorCodeOf(typedNil); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf(typed nil) = %q, want %q", got, ErrTransferFailed)
	}
	if got := PublicErrorOf(typedNil); got.Code != ErrTransferFailed {
		t.Fatalf("PublicErrorOf(typed nil) = %#v, want transfer_failed", got)
	}
	if got := domain.Error(); got != string(ErrTransferFailed) {
		t.Fatalf("nil DomainError.Error() = %q, want %q", got, ErrTransferFailed)
	}
	if got := domain.Code(); got != ErrTransferFailed {
		t.Fatalf("nil DomainError.Code() = %q, want %q", got, ErrTransferFailed)
	}
	if cause := domain.Unwrap(); cause != nil {
		t.Fatalf("nil DomainError.Unwrap() = %v, want nil", cause)
	}
}

func TestDomainErrorStringDoesNotRenderWrappedSecrets(t *testing.T) {
	t.Parallel()

	const path = `C:\Users\sender\private report.txt`
	const token = "capability-token-that-must-not-leak"
	err := WrapError(
		ErrPathUnsupported,
		"selection metadata could not be read",
		fmt.Errorf("open %s with %s: denied", path, token),
	)

	for _, rendered := range []string{err.Error(), PublicErrorOf(err).Message} {
		if strings.Contains(rendered, path) || strings.Contains(rendered, token) {
			t.Fatalf("rendered error leaked path or token: %q", rendered)
		}
	}
	if got, want := err.Error(), "path_unsupported: selection metadata could not be read"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestNewErrorHasNoCause(t *testing.T) {
	t.Parallel()

	var domain *DomainError
	if !errors.As(NewError(ErrBusy, "already active"), &domain) {
		t.Fatal("NewError did not return a DomainError")
	}
	if domain.Unwrap() != nil {
		t.Fatalf("NewError cause = %v, want nil", domain.Unwrap())
	}
}

func TestPublicErrorJSONWireShape(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(PublicError{Code: ErrCancelled, Message: "Transfer canceled."})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"code":"cancelled","message":"Transfer canceled."}`; got != want {
		t.Fatalf("json.Marshal(PublicError) = %s, want %s", got, want)
	}
}

type independentCodedError struct{ code ErrorCode }

func (e independentCodedError) Error() string   { return "independent coded error detail" }
func (e independentCodedError) Code() ErrorCode { return e.code }
