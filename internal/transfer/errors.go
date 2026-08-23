package transfer

import (
	"errors"
	"reflect"
)

// ErrorCode is a stable, boundary-safe failure classification.
type ErrorCode string

const (
	ErrInvalidSelection   ErrorCode = "invalid_selection"
	ErrBusy               ErrorCode = "busy"
	ErrCancelled          ErrorCode = "cancelled"
	ErrPathNotFound       ErrorCode = "path_not_found"
	ErrPathUnsupported    ErrorCode = "path_unsupported"
	ErrSourceChanged      ErrorCode = "source_changed"
	ErrNetworkUnavailable ErrorCode = "network_unavailable"
	ErrServerStartFailed  ErrorCode = "server_start_failed"
	ErrQRFailed           ErrorCode = "qr_failed"
	ErrBeaconWarning      ErrorCode = "beacon_warning"
	ErrTransferFailed     ErrorCode = "transfer_failed"
	ErrShuttingDown       ErrorCode = "shutting_down"
)

// PublicError is the fixed safe error shape exposed across the UI boundary.
type PublicError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// CodedError carries a stable ErrorCode through wrapping.
type CodedError interface {
	error
	Code() ErrorCode
}

// DomainError carries a stable code, safe local message, and optional cause.
// The cause is deliberately excluded from Error because filesystem causes
// routinely contain selected paths (and other adapters may contain tokens).
// It remains available to deliberate diagnostics only through Unwrap.
type DomainError struct {
	code        ErrorCode
	safeMessage string
	cause       error
}

var _ CodedError = (*DomainError)(nil)

// Error renders only safe local diagnostic information.
func (e *DomainError) Error() string {
	if e == nil {
		return string(ErrTransferFailed)
	}
	if e.safeMessage == "" {
		return string(e.code)
	}
	return string(e.code) + ": " + e.safeMessage
}

// Code returns the stable failure code.
func (e *DomainError) Code() ErrorCode {
	if e == nil {
		return ErrTransferFailed
	}
	return e.code
}

// Unwrap exposes the diagnostic cause to errors.Is and errors.As.
func (e *DomainError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// NewError constructs a coded error without an underlying cause.
func NewError(code ErrorCode, safeMessage string) error {
	return &DomainError{code: code, safeMessage: safeMessage}
}

// WrapError constructs a coded error that preserves cause through Unwrap.
func WrapError(code ErrorCode, safeMessage string, cause error) error {
	return &DomainError{code: code, safeMessage: safeMessage, cause: cause}
}

// ErrorCodeOf extracts a code through wrapping. Unknown non-nil errors use the
// fixed transfer failure fallback.
func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}

	var coded CodedError
	if errors.As(err, &coded) && !nilInterface(coded) {
		code := coded.Code()
		if _, known := publicMessages[code]; known {
			return code
		}
	}
	return ErrTransferFailed
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	kind := reflect.ValueOf(value).Kind()
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflect.ValueOf(value).IsNil()
	default:
		return false
	}
}

var publicMessages = map[ErrorCode]string{
	ErrInvalidSelection:   "Choose exactly one file or folder.",
	ErrBusy:               "Finish or cancel the current transfer before choosing another item.",
	ErrCancelled:          "Transfer canceled.",
	ErrPathNotFound:       "That file or folder is no longer available. Choose it again.",
	ErrPathUnsupported:    "FairDrop can use regular files and folders only. Choose another item.",
	ErrSourceChanged:      "The item changed after it was prepared. Cancel and create a fresh link.",
	ErrNetworkUnavailable: "FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again.",
	ErrServerStartFailed:  "FairDrop couldn’t open a local transfer connection. Check firewall access, then try again.",
	ErrQRFailed:           "FairDrop couldn’t create the QR code. Prepare the item again.",
	ErrBeaconWarning:      "Device discovery isn’t available. The QR code and download link still work.",
	ErrTransferFailed:     "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.",
	ErrShuttingDown:       "FairDrop is closing. Reopen it to start a transfer.",
}

// PublicErrorOf converts any error to fixed public copy without copying its
// diagnostic text.
func PublicErrorOf(err error) PublicError {
	if err == nil {
		return PublicError{}
	}

	code := ErrorCodeOf(err)
	message, ok := publicMessages[code]
	if !ok {
		code = ErrTransferFailed
		message = publicMessages[code]
	}
	return PublicError{Code: code, Message: message}
}
