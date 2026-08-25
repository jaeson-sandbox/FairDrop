package server

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"fairdrop/internal/transfer"
)

// downloadPattern is the one route this server answers. It is methodless, and
// the token is a wildcard rather than anything this package parses out of the
// path itself.
const downloadPattern = "/download/{token}"

// fallbackDownloadName stands in when a payload offers a name that cannot
// survive a header, so the receiver is never handed an empty filename.
const fallbackDownloadName = "download"

// route is the server's only entry point. ServeMux stays the sole router --
// nothing here splits a path or reads a segment -- but two of its answers are
// wrong for a capability URL and are replaced with the same bare 404 every
// other rejection gets:
//
//   - a non-canonical path, which ServeMux answers with 307 and a Location
//     header that would echo the supplied capability token back on the wire;
//   - any path that resolves to some other pattern, whose default 404 body
//     would read differently from this server's own rejections.
func (r *run) route(writer http.ResponseWriter, request *http.Request) {
	if !r.enter() {
		// Teardown has begun. The connection is about to be force-closed
		// anyway; answering 404 keeps a racing request indistinguishable from
		// one that arrived at a path that never existed.
		writeStatus(writer, http.StatusNotFound)
		return
	}
	defer r.leave()

	if !isCanonicalPath(request.URL.EscapedPath()) {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	if _, pattern := r.mux.Handler(request); pattern != downloadPattern {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	r.mux.ServeHTTP(writer, request)
}

// isCanonicalPath reports whether a path is already the form ServeMux would
// route, mirroring the normalization net/http applies before matching. It
// decides nothing about routing: a canonical path is still matched by the mux
// and a non-canonical one is refused rather than rewritten, because rewriting
// is what puts the token in a Location header.
func isCanonicalPath(escapedPath string) bool {
	if escapedPath == "" || escapedPath[0] != '/' {
		return false
	}
	canonical := path.Clean(escapedPath)
	// path.Clean drops a trailing separator; net/http keeps it, and so does
	// this, so a trailing slash stays a distinct path rather than a redirect.
	if canonical != "/" && strings.HasSuffix(escapedPath, "/") {
		canonical += "/"
	}
	return escapedPath == canonical
}

// download runs the claim-to-bytes sequence in the one order that keeps a
// wrong guess indistinguishable from a nonexistent resource and keeps the
// coordinator the only thing that can authorize a transfer.
func (r *run) download(writer http.ResponseWriter, request *http.Request) {
	// The route is registered without a method, so every method arrives here
	// and this check -- not the router -- is what makes HEAD, POST, and the
	// rest answer exactly like a path that does not exist.
	if request.Method != http.MethodGet {
		writeStatus(writer, http.StatusNotFound)
		return
	}
	// PathValue is the only way the token is read: the mux owns the routing,
	// so no path splitting here can disagree with what it matched.
	if !tokenMatches(request.PathValue("token"), r.token) {
		writeStatus(writer, http.StatusNotFound)
		return
	}

	// The reservation is the linearization point of the claim race, and it
	// comes before authorization on purpose: two receivers can open the same
	// URL at the same instant, and only one of them may ever reach the
	// coordinator. The loser is told the capability is taken, which is not a
	// disclosure -- it already proved it holds the token.
	if !r.claimed.CompareAndSwap(false, true) {
		writeStatus(writer, http.StatusLocked)
		return
	}

	// Synchronous by contract: the coordinator commits the transfer and
	// publishes its started event before this returns. Nothing is opened and
	// no header is written until it succeeds.
	if err := r.authorizer.AuthorizeClaim(r.ctx, r.sessionID); err != nil {
		// A refusal means the coordinator cancelled, is shutting down, or no
		// longer recognizes this session -- outcomes it already owns, so this
		// server reports no event and the receiver learns nothing. The
		// reservation is not released: the capability was single-use and it
		// has been used.
		writeStatus(writer, http.StatusNotFound)
		return
	}

	payload, err := r.payloads.Prepare(r.ctx, r.item)
	if err == nil && payload == nil {
		err = transfer.NewError(transfer.ErrTransferFailed, "payload preparation returned no payload")
	}
	if err != nil {
		// Preparation is the last moment a failure can still choose a status.
		// The receiver gets a bare 410 -- the reason names a source path or a
		// filesystem cause and belongs only to the sender -- while the coded
		// cause reaches the coordinator unchanged.
		writeStatus(writer, http.StatusGone)
		event := failedEvent(r.sessionID, transfer.ProgressSnapshot{}, err)
		r.finish(&event)
		return
	}
	// From here the payload has exactly one owner and exactly one Close: this
	// handler, on this goroutine, after WriteTo has returned. The deferred
	// call also covers the abort path below, where it runs during the panic's
	// unwind rather than being skipped.
	defer func() { _ = payload.Close() }()

	total, totalKnown := payload.Size()
	if !totalKnown || total < 0 {
		total, totalKnown = 0, false
	}
	writeDownloadHeaders(writer, payload.DownloadName(), total, totalKnown)
	// Written explicitly because a known-empty payload never calls Write, and
	// flushed immediately for two reasons. An unknown length must stay
	// unknown: net/http synthesizes a Content-Length for any response it
	// managed to buffer whole, which would put a promised length on a payload
	// that has none. And a failure after this point must look like a broken
	// download rather than a connection that answered nothing, which requires
	// the response to have already started.
	writer.WriteHeader(http.StatusOK)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}

	progress := newMeter(total, totalKnown, r.now, func(snapshot transfer.ProgressSnapshot) {
		r.lane.publishProgress(progressEvent(r.sessionID, snapshot))
	})
	writeErr := payload.WriteTo(r.ctx, &countingWriter{dst: writer, meter: progress})
	snapshot := progress.snapshot()

	if writeErr == nil {
		event := completeEvent(r.sessionID, snapshot)
		r.finish(&event)
		return
	}

	// A cancelled stream is the coordinator's own outcome and it already knows,
	// so that teardown is silent; anything else is a genuine failure only this
	// server saw.
	if isCancellation(writeErr) {
		r.finish(nil)
	} else {
		event := failedEvent(r.sessionID, snapshot, writeErr)
		r.finish(&event)
	}
	// The status and Content-Length are already on the wire, so no error body
	// can be appended and no status can be corrected. Breaking the connection
	// is the only honest signal left: it turns a truncated file that looks
	// complete into a failed download the receiver's browser reports.
	panic(http.ErrAbortHandler)
}

// finish ends the transfer: cancel the data-plane context, retire the listener
// so no one else is promised a status, and deliver the single terminal event.
// A nil event means the outcome belongs to the coordinator, which closes
// silently rather than reporting the coordinator's own decision back to it.
func (r *run) finish(event *transfer.ServerEvent) {
	r.cancel()
	r.closeListener()
	if event != nil {
		r.lane.publishTerminal(*event)
	}
}

// tokenMatches compares in constant time so a near-miss cannot be told from a
// wild guess by timing it. Length still differs observably, which is
// acceptable: the token's length is fixed and public, and only its value is
// secret.
func tokenMatches(candidate string, token transfer.CapabilityToken) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}

func isCancellation(err error) bool {
	return transfer.ErrorCodeOf(err) == transfer.ErrCancelled
}

// writeStatus answers with a status and nothing else. Every rejected
// scenario -- wrong method, wrong route, wrong token, refused claim, failed
// preparation -- produces a byte-identical body, so no response distinguishes
// "you guessed wrong" from "you are too late".
func writeStatus(writer http.ResponseWriter, status int) {
	header := writer.Header()
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
}

func writeDownloadHeaders(writer http.ResponseWriter, name string, total int64, totalKnown bool) {
	header := writer.Header()
	// Opaque bytes with sniffing disabled: the receiver's browser must save
	// the file, never render it as whatever its bytes resemble.
	header.Set("Content-Type", "application/octet-stream")
	header.Set("Content-Disposition", contentDisposition(name))
	// Nothing about a one-shot capability may be cached or revalidated: the
	// URL is dead the moment it is used.
	header.Set("Cache-Control", "no-store")
	// The capability is the token, not the origin, so a receiver page fetched
	// from anywhere may download it.
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("X-Content-Type-Options", "nosniff")
	if totalKnown {
		header.Set("Content-Length", strconv.FormatInt(total, 10))
	}
}

// contentDisposition offers the name twice, as RFC 6266 requires: a quoted
// ASCII form every client understands, and the RFC 5987 form that carries the
// real Unicode name. The payload owns sanitization and this places its value
// as given; the extra filtering here is defense in depth against a payload
// implementation that does not, since a quote or a newline in a header value
// is a header-injection primitive.
func contentDisposition(name string) string {
	var builder strings.Builder
	builder.WriteString(`attachment; filename="`)
	builder.WriteString(asciiFilename(name))
	builder.WriteString(`"; filename*=UTF-8''`)
	builder.WriteString(encodeExtendedFilename(name))
	return builder.String()
}

// asciiFilename is the legacy fallback: a non-ASCII rune becomes an
// underscore so the name keeps its shape and extension, and anything that
// could terminate the quoted parameter or the header line is dropped.
func asciiFilename(name string) string {
	var builder strings.Builder
	for _, char := range name {
		switch {
		case char < 0x20 || char == 0x7f:
			continue
		case char == '"' || char == '\\' || char == ';' || char == '\'':
			continue
		case char > 0x7f:
			builder.WriteByte('_')
		default:
			builder.WriteRune(char)
		}
	}
	cleaned := strings.TrimSpace(builder.String())
	if cleaned == "" {
		return fallbackDownloadName
	}
	return cleaned
}

// encodeExtendedFilename percent-encodes to RFC 5987's attr-char set. It is
// deliberately not url.PathEscape: that leaves characters this grammar
// forbids, and an over-encoded name still decodes to exactly the right bytes.
func encodeExtendedFilename(name string) string {
	if name == "" {
		name = fallbackDownloadName
	}
	const attrChars = "!#$&+-.^_`|~"
	var builder strings.Builder
	for _, char := range []byte(name) {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			strings.IndexByte(attrChars, char) >= 0:
			builder.WriteByte(char)
		default:
			// Two hex digits always: "%A" would be a malformed escape that a
			// client could decode as the literal characters.
			builder.WriteString(fmt.Sprintf("%%%02X", char))
		}
	}
	return builder.String()
}
