package transfer

import "errors"

// ErrorCode is a stable, wire-safe identifier for a failure class. Codes are
// the only failure vocabulary that crosses a package or process boundary:
// adapters never compare error strings, and the UI never parses adapter text.
type ErrorCode string

// The twelve stable domain error codes. Their string values are fixed by
// docs/fairdrop-contracts.md and mirrored by the frontend, so changing a value
// is a protocol change, not a rename.
const (
	// ErrInvalidSelection: zero/multiple paths or an empty path at an input boundary.
	ErrInvalidSelection ErrorCode = "invalid_selection"
	// ErrBusy: Stage requested outside IDLE.
	ErrBusy ErrorCode = "busy"
	// ErrCancelled: Stage/claim/transfer lost to Cancel or Shutdown.
	ErrCancelled ErrorCode = "cancelled"
	// ErrPathNotFound: the selected root no longer exists.
	ErrPathNotFound ErrorCode = "path_not_found"
	// ErrPathUnsupported: link, reparse point, special file, or host-unsupported path.
	ErrPathUnsupported ErrorCode = "path_unsupported"
	// ErrSourceChanged: a staged regular file's type/size/modtime changed before claim.
	ErrSourceChanged ErrorCode = "source_changed"
	// ErrNetworkUnavailable: no eligible LAN IPv4.
	ErrNetworkUnavailable ErrorCode = "network_unavailable"
	// ErrServerStartFailed: the listener could not become ready.
	ErrServerStartFailed ErrorCode = "server_start_failed"
	// ErrQRFailed: the capability QR could not be encoded.
	ErrQRFailed ErrorCode = "qr_failed"
	// ErrBeaconWarning: HTTP/QR are ready but mDNS publication failed; non-terminal.
	ErrBeaconWarning ErrorCode = "beacon_warning"
	// ErrTransferFailed: read, ZIP, connection, or post-header stream failure. Also
	// the fixed fallback for any unrecognized error.
	ErrTransferFailed ErrorCode = "transfer_failed"
	// ErrShuttingDown: command rejected after application shutdown begins.
	ErrShuttingDown ErrorCode = "shutting_down"
)

// PublicError is the only error shape that may cross the Wails boundary. Both
// fields are drawn from fixed tables: the code from this package's constants,
// the message from the copy registry below. Neither can carry an absolute path,
// a basename, a capability token, or raw adapter text.
type PublicError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// CodedError is any error that carries a stable ErrorCode. ErrorCodeOf finds it
// through %w wrappers with errors.As, which is what lets an adapter wrap a
// low-level cause without losing the classification the UI depends on.
type CodedError interface {
	error
	Code() ErrorCode
}

// DomainError is the single carrier for a coded failure. It holds a stable
// code, a safe local message written by the adapter, and an optional wrapped
// cause.
//
// Fields are private so nothing can rewrite a code after the fact. The value is
// always used as *DomainError behind the error interface; NewError and
// WrapError are the only constructors.
type DomainError struct {
	code    ErrorCode
	message string
	cause   error
}

// Compile-time proof that the carrier satisfies the contract's interface. If
// this ever fails, errors.As(err, &coded) would silently stop finding codes and
// every failure would degrade to transfer_failed.
var _ CodedError = (*DomainError)(nil)

// Error returns sender-private diagnostic text: the code, the adapter's safe
// message, and the wrapped cause when present.
//
// A wrapped cause may well contain an absolute path -- os.Lstat failures do --
// so this string is for local debugging only. It must never be emitted to the
// UI, an HTTP response, an mDNS record, or a log that leaves the process. The
// boundary-safe rendering is PublicErrorOf, which ignores this text entirely.
func (e *DomainError) Error() string {
	if e.cause == nil {
		return string(e.code) + ": " + e.message
	}
	return string(e.code) + ": " + e.message + ": " + e.cause.Error()
}

// Code returns the stable error code.
func (e *DomainError) Code() ErrorCode { return e.code }

// Unwrap exposes the wrapped cause so errors.Is and errors.As reach through it,
// letting callers ask both "what class of failure is this" (the code) and "what
// exactly went wrong" (for example errors.Is(err, fs.ErrNotExist)).
func (e *DomainError) Unwrap() error { return e.cause }

// NewError builds a coded error with no underlying cause.
//
// safeMessage is local diagnostic text. It must not contain an absolute path, a
// basename, or a capability token, because a future reader may log it; it is
// never what the UI shows regardless.
func NewError(code ErrorCode, safeMessage string) error {
	return &DomainError{code: code, message: safeMessage}
}

// WrapError builds a coded error that wraps cause with %w semantics, preserving
// it for errors.Is and errors.As while fixing the classification the UI sees.
func WrapError(code ErrorCode, safeMessage string, cause error) error {
	return &DomainError{code: code, message: safeMessage, cause: cause}
}

// ErrorCodeOf classifies any error.
//
// It finds the innermost-wrapping CodedError through %w chains and returns its
// code. Every non-nil error that carries no code maps to ErrTransferFailed, the
// fixed safe fallback, so an unclassified failure can never surface raw adapter
// text as a category. A nil error has no code and returns "".
func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var coded CodedError
	if errors.As(err, &coded) {
		return coded.Code()
	}
	return ErrTransferFailed
}

// publicMessages is the copy registry: the exact PublicError.Message string the
// experience spine fixes for each code, transcribed verbatim from the "Stable
// public error and warning copy" table in
// _bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md.
// (beacon_warning's cell names the key copy.discovery.warning; its literal is
// taken from the copy registry table in the same document.)
//
// The apostrophes are U+2019 RIGHT SINGLE QUOTATION MARK, exactly as the
// registry writes them -- "fixing" them to ASCII would break the
// character-for-character contract. Nothing here may be paraphrased,
// interpolated, or extended with adapter detail.
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

// PublicErrorOf renders any error as the boundary-safe pair the UI may show.
//
// It classifies with ErrorCodeOf and then looks the message up in the fixed
// registry; it never copies the error's own text, so no adapter can smuggle a
// path or token into the UI by writing a colorful message. A code with no
// registry entry -- which would mean a code invented outside this package --
// degrades to the transfer_failed pair rather than emitting an empty message.
// A nil error yields the zero PublicError.
func PublicErrorOf(err error) PublicError {
	if err == nil {
		return PublicError{}
	}
	code := ErrorCodeOf(err)
	message, known := publicMessages[code]
	if !known {
		code = ErrTransferFailed
		message = publicMessages[ErrTransferFailed]
	}
	return PublicError{Code: code, Message: message}
}
