package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// TestRejectedRequestsNeverReachClaimLogic walks the four rejection rows of
// the matrix -- wrong method, wrong route, malformed route, wrong token -- and
// proves each one is answered without reserving, authorizing, opening a
// payload, or emitting an event. The successful claim at the end is the proof
// that none of them consumed the one-shot capability.
func TestRejectedRequestsNeverReachClaimLogic(t *testing.T) {
	t.Parallel()

	payloads := payloadsReturning(&stubPayload{name: "report.pdf", known: true})
	authorizer := &stubAuthorizer{}
	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)

	valid := downloadURL(handle.Port, string(testToken))
	rejected := []struct {
		name   string
		method string
		url    string
	}{
		{"post on the exact route", http.MethodPost, valid},
		{"put on the exact route", http.MethodPut, valid},
		{"delete on the exact route", http.MethodDelete, valid},
		// HEAD is the reason the route pattern carries no method: a
		// method-qualified pattern would route HEAD into the GET handler.
		{"head on the exact route", http.MethodHead, valid},
		{"options on the exact route", http.MethodOptions, valid},
		{"root", http.MethodGet, baseURL(handle.Port) + "/"},
		{"route prefix", http.MethodGet, baseURL(handle.Port) + "/download"},
		{"empty token segment", http.MethodGet, baseURL(handle.Port) + "/download/"},
		{"trailing slash", http.MethodGet, valid + "/"},
		{"extra segment", http.MethodGet, valid + "/extra"},
		{"doubled separator", http.MethodGet, baseURL(handle.Port) + "/download//" + string(testToken)},
		{"dot-dot traversal", http.MethodGet, baseURL(handle.Port) + "/download/../download/" + string(testToken)},
		{"unrelated path", http.MethodGet, baseURL(handle.Port) + "/healthz"},
		{"oversized token", http.MethodGet, downloadURL(handle.Port, strings.Repeat("a", 4096))},
		{"empty token", http.MethodGet, downloadURL(handle.Port, "")},
		{"token prefix", http.MethodGet, downloadURL(handle.Port, string(testToken)[:8])},
		{"token with one wrong byte", http.MethodGet, downloadURL(handle.Port, string(testToken)[:31]+"0")},
	}

	for _, test := range rejected {
		t.Run(test.name, func(t *testing.T) {
			response := do(t, test.method, test.url)
			body := readBody(t, response)

			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
			}
			if location := response.Header.Get("Location"); location != "" {
				t.Fatalf("rejection redirected to %q", location)
			}
			if allow := response.Header.Get("Allow"); allow != "" {
				t.Fatalf("rejection advertised methods: %q", allow)
			}
			// A HEAD response legitimately carries no body; every other
			// rejection must be empty for the same reason.
			if test.method != http.MethodHead {
				assertNoDisclosure(t, response, body)
			}
		})
	}

	if got := authorizer.calls.Load(); got != 0 {
		t.Fatalf("AuthorizeClaim ran %d times for rejected requests, want 0", got)
	}
	if got := payloads.calls.Load(); got != 0 {
		t.Fatalf("Prepare ran %d times for rejected requests, want 0", got)
	}
	assertNoEvents(t, handle.Events)

	// Nothing above reserved the capability, so the real receiver still works.
	response := do(t, http.MethodGet, valid)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status after rejected requests = %d, want %d", response.StatusCode, http.StatusOK)
	}
	readBody(t, response)
}

// TestRejectionsAreIndistinguishable pins the disclosure rule: a caller with a
// wrong token learns nothing a caller with a wrong path does not.
func TestRejectionsAreIndistinguishable(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, &stubPayloads{})
	handle := startTestServer(t, server, &stubAuthorizer{})

	shapes := map[string]string{
		"wrong token":               downloadURL(handle.Port, strings.Repeat("b", len(testToken))),
		"wrong path":                baseURL(handle.Port) + "/nope",
		"right token, wrong method": downloadURL(handle.Port, string(testToken)),
	}

	var reference string
	for name, url := range shapes {
		method := http.MethodGet
		if strings.Contains(name, "wrong method") {
			method = http.MethodPost
		}
		response := do(t, method, url)
		rendered := renderResponse(t, response)
		if reference == "" {
			reference = rendered
			continue
		}
		if rendered != reference {
			t.Fatalf("%s answered differently:\n%s\nwant:\n%s", name, rendered, reference)
		}
	}
}

// TestFirstClaimAuthorizesOnceBeforeOpeningPayload pins the six-step order:
// the authorization handshake completes before any payload is opened and
// before any header is written.
func TestFirstClaimAuthorizesOnceBeforeOpeningPayload(t *testing.T) {
	t.Parallel()

	payloads := &stubPayloads{}
	authorizer := &stubAuthorizer{}
	authorizer.authorize = func(context.Context, transfer.SessionID) error {
		if got := payloads.calls.Load(); got != 0 {
			t.Errorf("Prepare ran %d times before authorization returned, want 0", got)
		}
		return nil
	}
	payloads.prepare = func(context.Context, transfer.StagedItem) (PreparedPayload, error) {
		if got := authorizer.calls.Load(); got != 1 {
			t.Errorf("AuthorizeClaim ran %d times before Prepare, want 1", got)
		}
		return &stubPayload{name: "report.pdf", known: true}, nil
	}

	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	readBody(t, response)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := authorizer.calls.Load(); got != 1 {
		t.Fatalf("AuthorizeClaim ran %d times, want exactly 1", got)
	}
	sessions := authorizer.claimedSessions()
	if len(sessions) != 1 || sessions[0] != testSession {
		t.Fatalf("authorized sessions = %v, want [%s]", sessions, testSession)
	}
	if got := payloads.calls.Load(); got != 1 {
		t.Fatalf("Prepare ran %d times, want exactly 1", got)
	}
	items := payloads.preparedItems()
	if len(items) != 1 || items[0] != startRequest().Item {
		t.Fatalf("Prepare received %+v, want the staged item", items[0])
	}
}

// TestCompetingClaimsAuthorizeExactlyOnce drives the claim race concurrently,
// which is the only way to exercise the reservation's compare-and-swap. The
// winner is held mid-stream so the loser meets a live listener and a consumed
// capability rather than a closed port.
func TestCompetingClaimsAuthorizeExactlyOnce(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("fairdrop"), 512)
	release := make(chan struct{})
	// Every exit from this test must unpark the winner. Without it a failed
	// assertion leaves WriteTo blocked, Stop waits for a handler that can never
	// return, and the failure surfaces as a package-wide timeout instead.
	releaseStream := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseStream)
	payload := &stubPayload{
		name:  "report.pdf",
		size:  int64(len(body)),
		known: true,
		stream: func(ctx context.Context, dst io.Writer) error {
			<-release
			return bodyOf(body, 256)(ctx, dst)
		},
	}
	payloads := payloadsReturning(payload)
	authorizer := &stubAuthorizer{}

	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)
	url := downloadURL(handle.Port, string(testToken))

	const claims = 2
	start := make(chan struct{})
	statuses := make(chan int, claims)
	winnerBody := make(chan []byte, claims)
	var group sync.WaitGroup
	for range claims {
		group.Add(1)
		go func() {
			defer group.Done()
			request, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Error(err)
				return
			}
			<-start
			response, err := testClient().Do(request)
			if err != nil {
				t.Errorf("competing claim failed: %v", err)
				return
			}
			defer func() { _ = response.Body.Close() }()
			statuses <- response.StatusCode
			if response.StatusCode != http.StatusOK {
				return
			}
			received, readErr := io.ReadAll(response.Body)
			if readErr != nil {
				t.Errorf("winner body: %v", readErr)
				return
			}
			winnerBody <- received
		}()
	}
	close(start)

	// The winner's headers are flushed before WriteTo parks, deliberately, so
	// its 200 may reach the client first. Ordering therefore proves nothing.
	// What does is that the loser is answered 423 WHILE the winner is still
	// parked: that is what shows it met a live listener holding a consumed
	// capability rather than a finished or torn-down transfer.
	locked, ok := awaitStatusFor(t, statuses, http.StatusLocked, 30*time.Second)
	if !ok {
		t.Fatalf("no losing claim was answered %d while the winner was parked mid-stream; saw %v",
			http.StatusLocked, locked)
	}
	if payload.writeReturned.Load() {
		t.Fatal("the winner finished streaming before the loser was answered")
	}
	releaseStream()

	// Whatever did not arrive above must now arrive, and the two claims must
	// between them be exactly one 423 and one 200.
	seen := append([]int{http.StatusLocked}, locked...)
	for len(seen) < claims {
		seen = append(seen, awaitStatus(t, statuses))
	}
	group.Wait()
	sort.Ints(seen)
	if want := []int{http.StatusOK, http.StatusLocked}; !slices.Equal(seen, want) {
		t.Fatalf("claim statuses = %v, want exactly one %d and one %d", seen, http.StatusOK, http.StatusLocked)
	}

	if got := authorizer.calls.Load(); got != 1 {
		t.Fatalf("AuthorizeClaim ran %d times, want exactly 1", got)
	}
	if got := payloads.calls.Load(); got != 1 {
		t.Fatalf("Prepare ran %d times, want exactly 1", got)
	}
	select {
	case received := <-winnerBody:
		if !bytes.Equal(received, body) {
			t.Fatal("the winning claim did not receive the whole payload")
		}
	default:
		t.Fatal("the winning claim delivered no body")
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	payload.assertOwnedOnce(t)
	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerComplete)
	}
}

// TestRefusedAuthorizationOpensNoPayload covers the denied, cancelled, stale,
// and shutting-down row: the coordinator owns that outcome, so nothing is
// opened, nothing is written, and the server reports no event of its own.
func TestRefusedAuthorizationOpensNoPayload(t *testing.T) {
	t.Parallel()

	refusals := map[string]error{
		"cancelled":     transfer.NewError(transfer.ErrCancelled, "claim lost to cancel"),
		"shutting down": transfer.NewError(transfer.ErrShuttingDown, "claim arrived during shutdown"),
		"stale session": transfer.NewError(transfer.ErrCancelled, "claim names a stale session"),
	}

	for name, refusal := range refusals {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			payloads := &stubPayloads{}
			server := newTestServer(t, payloads)
			handle := startTestServer(t, server, refusingAuthorizer(refusal))

			response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
			body := readBody(t, response)

			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
			}
			assertNoDisclosure(t, response, body)
			if got := payloads.calls.Load(); got != 0 {
				t.Fatalf("Prepare ran %d times after a refused claim, want 0", got)
			}
			assertNoEvents(t, handle.Events)
		})
	}
}

// TestPrepareFailureIsGenericGone covers the last moment a failure can still
// choose a status: the receiver gets a bare 410 while the coded cause reaches
// the coordinator intact.
func TestPrepareFailureIsGenericGone(t *testing.T) {
	t.Parallel()

	cause := errors.New("stat C:\\Users\\example\\Documents\\quarterly report.pdf: no such file")
	failure := transfer.WrapError(transfer.ErrSourceChanged, "payload source changed after it was staged", cause)

	server := newTestServer(t, payloadsFailing(failure))
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	body := readBody(t, response)

	if response.StatusCode != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusGone)
	}
	assertNoDisclosure(t, response, body)

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerFailed)
	}
	if terminal.Progress != nil {
		t.Fatalf("failure before the first byte carried progress: %+v", terminal.Progress)
	}
	if code := transfer.ErrorCodeOf(terminal.Err); code != transfer.ErrSourceChanged {
		t.Fatalf("failure code = %q, want %q", code, transfer.ErrSourceChanged)
	}
	if !errors.Is(terminal.Err, cause) {
		t.Fatal("the coded cause was not preserved for the coordinator")
	}
	if terminal.SessionID != testSession {
		t.Fatalf("event session = %q, want %q", terminal.SessionID, testSession)
	}
}

// TestPrepareFailureClosesTheListener pins the "no replay status" rule: the
// capability is spent, so the next receiver finds nothing listening.
func TestPrepareFailureClosesTheListener(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsFailing(transfer.NewError(transfer.ErrPathNotFound, "gone")))
	handle := startTestServer(t, server, &stubAuthorizer{})
	url := downloadURL(handle.Port, string(testToken))

	if response := do(t, http.MethodGet, url); response.StatusCode != http.StatusGone {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusGone)
	}

	waitForClosedListener(t, url)
}

// TestSuccessfulDownloadServesHeadersBodyAndOneCompleteEvent covers the
// success-headers, known-length, and natural-completion rows together.
func TestSuccessfulDownloadServesHeadersBodyAndOneCompleteEvent(t *testing.T) {
	t.Parallel()

	body := bytes.Repeat([]byte("paper relay "), 4096)
	payload := &stubPayload{
		name:   "quarterly report 🌍.pdf",
		size:   int64(len(body)),
		known:  true,
		stream: bodyOf(body, 4096),
	}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	received := readBody(t, response)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !bytes.Equal(received, body) {
		t.Fatalf("received %d bytes, want %d", len(received), len(body))
	}

	headers := map[string]string{
		"Cache-Control":               "no-store",
		"Access-Control-Allow-Origin": "*",
		"X-Content-Type-Options":      "nosniff",
		"Content-Type":                "application/octet-stream",
		"Content-Length":              strconv.Itoa(len(body)),
	}
	for name, want := range headers {
		if got := response.Header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	disposition := response.Header.Get("Content-Disposition")
	// Both forms, because a receiver that understands neither RFC 5987 nor
	// UTF-8 still has to be handed a usable name.
	if !strings.HasPrefix(disposition, `attachment; filename="quarterly report _.pdf"`) {
		t.Fatalf("Content-Disposition ASCII fallback = %q", disposition)
	}
	if !strings.Contains(disposition, `filename*=UTF-8''quarterly%20report%20%F0%9F%8C%8D.pdf`) {
		t.Fatalf("Content-Disposition RFC 5987 form = %q", disposition)
	}
	if strings.Contains(disposition, startRequest().Item.Path) {
		t.Fatal("Content-Disposition disclosed the source path")
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	payload.assertOwnedOnce(t)

	events := drainEvents(t, handle.Events)
	terminal := terminalEvent(t, events)
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerComplete)
	}
	if terminal.Err != nil {
		t.Fatalf("complete event carried an error: %v", terminal.Err)
	}
	if terminal.Progress == nil {
		t.Fatal("complete event carried no authoritative snapshot")
	}
	snapshot := *terminal.Progress
	if snapshot.BytesSent != int64(len(body)) || snapshot.TotalBytes != int64(len(body)) || !snapshot.TotalKnown {
		t.Fatalf("terminal snapshot = %+v, want the prepared length", snapshot)
	}
	if snapshot.Percent != 100 {
		t.Fatalf("terminal percent = %v, want exactly 100", snapshot.Percent)
	}
}

// TestUnknownLengthOmitsContentLength is the directory-shaped payload Epic 2
// will bring: an unknown wire length reports no Content-Length and a zero
// percentage rather than guessing one.
func TestUnknownLengthOmitsContentLength(t *testing.T) {
	t.Parallel()

	body := []byte("streamed without a known length")
	payload := &stubPayload{name: "folder.zip", known: false, stream: bodyOf(body, 8)}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	received := readBody(t, response)

	if !bytes.Equal(received, body) {
		t.Fatalf("received %q, want %q", received, body)
	}
	if got, ok := response.Header["Content-Length"]; ok {
		t.Fatalf("Content-Length = %v, want it absent for an unknown length", got)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	snapshot := *terminal.Progress
	if snapshot.TotalKnown || snapshot.TotalBytes != 0 || snapshot.Percent != 0 {
		t.Fatalf("unknown-total snapshot = %+v, want TotalKnown false with zeroed total and percent", snapshot)
	}
	if snapshot.BytesSent != int64(len(body)) {
		t.Fatalf("BytesSent = %d, want %d", snapshot.BytesSent, len(body))
	}
}

// TestKnownEmptyPayloadCompletesAtZeroPercent covers the known-empty row: a
// real, complete, zero-byte transfer is not a failure and is not 100%.
func TestKnownEmptyPayloadCompletesAtZeroPercent(t *testing.T) {
	t.Parallel()

	payload := &stubPayload{name: "empty.bin", size: 0, known: true}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	received := readBody(t, response)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if len(received) != 0 {
		t.Fatalf("received %d bytes, want 0", len(received))
	}
	if got := response.Header.Get("Content-Length"); got != "0" {
		t.Fatalf("Content-Length = %q, want %q", got, "0")
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerComplete)
	}
	snapshot := *terminal.Progress
	if snapshot.BytesSent != 0 || snapshot.TotalBytes != 0 || !snapshot.TotalKnown || snapshot.Percent != 0 {
		t.Fatalf("empty-file snapshot = %+v, want a known zero total at 0%%", snapshot)
	}
}

// TestStreamFailureAfterHeadersAbortsTheConnection covers the post-header
// failure row. The status and Content-Length are already on the wire, so the
// only honest signal left is a broken connection: a receiver must never be
// handed a truncated file that looks complete.
func TestStreamFailureAfterHeadersAbortsTheConnection(t *testing.T) {
	t.Parallel()

	const written = 32 << 10
	cause := errors.New("read C:\\Users\\example\\Documents\\quarterly report.pdf: input/output error")
	// The prefix is larger than net/http's response buffers on purpose:
	// without it the headers would never leave the server and the failure
	// would look like a connection that was never answered.
	payload := &stubPayload{
		name:  "report.pdf",
		size:  1 << 20,
		known: true,
		stream: func(_ context.Context, dst io.Writer) error {
			if _, err := dst.Write(bytes.Repeat([]byte("x"), written)); err != nil {
				return transfer.WrapError(transfer.ErrTransferFailed, "destination failed", err)
			}
			return transfer.WrapError(transfer.ErrTransferFailed, "payload source failed mid-stream", cause)
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	received, err := io.ReadAll(response.Body)
	if err == nil {
		t.Fatal("the receiver read a complete body from a failed transfer")
	}
	if !isConnectionFailure(err) {
		t.Fatalf("body error = %v, want an aborted connection", err)
	}
	if len(received) > written {
		t.Fatalf("received %d bytes, want at most the %d written before the failure", len(received), written)
	}
	if !bytes.Equal(received, bytes.Repeat([]byte("x"), len(received))) {
		t.Fatal("an error body was appended to the payload bytes")
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	payload.assertOwnedOnce(t)

	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerFailed)
	}
	if code := transfer.ErrorCodeOf(terminal.Err); code != transfer.ErrTransferFailed {
		t.Fatalf("failure code = %q, want %q", code, transfer.ErrTransferFailed)
	}
	if !errors.Is(terminal.Err, cause) {
		t.Fatal("the stream failure's cause was not preserved")
	}
	if terminal.Progress == nil || terminal.Progress.BytesSent != written {
		t.Fatalf("failed snapshot = %+v, want the %d bytes the connection accepted", terminal.Progress, written)
	}
}

// TestReceiverDisconnectFailsTheTransfer is the same row from the other side:
// the receiver drops, so the destination write fails rather than the source.
func TestReceiverDisconnectFailsTheTransfer(t *testing.T) {
	t.Parallel()

	const budget = 64 << 20
	chunk := bytes.Repeat([]byte("y"), 32<<10)
	overrun := errors.New("destination accepted the whole budget after the receiver left")
	payload := &stubPayload{
		name:  "report.pdf",
		size:  budget,
		known: true,
		stream: func(_ context.Context, dst io.Writer) error {
			for sent := 0; sent < budget; sent += len(chunk) {
				if _, err := dst.Write(chunk); err != nil {
					return transfer.WrapError(transfer.ErrTransferFailed, "payload destination failed mid-stream", err)
				}
			}
			return overrun
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	if _, err := io.CopyN(io.Discard, response.Body, 1024); err != nil {
		t.Fatalf("reading the first bytes: %v", err)
	}
	// The receiver leaves mid-stream.
	_ = response.Body.Close()

	terminal := awaitEvent(t, handle.Events, 60*time.Second)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerFailed)
	}
	if errors.Is(terminal.Err, overrun) {
		t.Fatal("the destination never failed after the receiver disconnected")
	}
	if code := transfer.ErrorCodeOf(terminal.Err); code != transfer.ErrTransferFailed {
		t.Fatalf("failure code = %q, want %q", code, transfer.ErrTransferFailed)
	}
	if terminal.Progress == nil || terminal.Progress.BytesSent <= 0 {
		t.Fatalf("failed snapshot = %+v, want the bytes written before the disconnect", terminal.Progress)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	payload.assertOwnedOnce(t)
}

// TestProgressIsCappedAndCountsOnlyAcceptedBytes drives the cadence with a
// deterministic clock: every reading advances 100ms, so the 4 Hz floor of
// 250ms admits one snapshot per three writes.
func TestProgressIsCappedAndCountsOnlyAcceptedBytes(t *testing.T) {
	t.Parallel()

	const (
		chunks    = 9
		chunkSize = 1024
	)
	body := bytes.Repeat([]byte("z"), chunks*chunkSize)
	payload := &stubPayload{
		name:   "report.pdf",
		size:   int64(len(body)),
		known:  true,
		stream: bodyOf(body, chunkSize),
	}
	server := newTestServer(t, payloadsReturning(payload))
	server.now = newTestClock(100 * time.Millisecond).now
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	if received := readBody(t, response); len(received) != len(body) {
		t.Fatalf("received %d bytes, want %d", len(received), len(body))
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	events := drainEvents(t, handle.Events)
	var progress []transfer.ProgressSnapshot
	for _, event := range events {
		if event.Kind == transfer.ServerProgress {
			if event.Progress == nil {
				t.Fatal("progress event carried no snapshot")
			}
			progress = append(progress, *event.Progress)
		}
	}
	// One clock reading starts the meter and one more accompanies each of the
	// nine accepted writes: snapshots fall at +300ms, +600ms, and +900ms.
	if len(progress) != 3 {
		t.Fatalf("got %d progress events for %d writes, want 3 under the 4 Hz cap", len(progress), chunks)
	}
	for index, snapshot := range progress {
		wantBytes := int64((index + 1) * 3 * chunkSize)
		if snapshot.BytesSent != wantBytes {
			t.Fatalf("progress[%d].BytesSent = %d, want %d", index, snapshot.BytesSent, wantBytes)
		}
		if snapshot.Percent <= 0 || snapshot.Percent > 100 {
			t.Fatalf("progress[%d].Percent = %v, want a clamped positive percentage", index, snapshot.Percent)
		}
	}
	if terminal := terminalEvent(t, events); terminal.Progress.BytesSent != int64(len(body)) {
		t.Fatalf("terminal BytesSent = %d, want %d", terminal.Progress.BytesSent, len(body))
	}
}

// TestEventDeliveryNeverBlocksTheHandler fills the lane with nobody draining
// it. The transfer must still finish and the terminal event must still be
// delivered, because the coordinator's drainer may not be scheduled yet.
func TestEventDeliveryNeverBlocksTheHandler(t *testing.T) {
	t.Parallel()

	const chunks = laneCapacity * 8
	body := bytes.Repeat([]byte("q"), chunks*64)
	payload := &stubPayload{
		name:   "report.pdf",
		size:   int64(len(body)),
		known:  true,
		stream: bodyOf(body, 64),
	}
	server := newTestServer(t, payloadsReturning(payload))
	// Every reading jumps a full second, so every write emits a snapshot and
	// the lane overflows many times over.
	server.now = newTestClock(time.Second).now
	handle := startTestServer(t, server, &stubAuthorizer{})

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	if received := readBody(t, response); len(received) != len(body) {
		t.Fatalf("received %d bytes, want %d", len(received), len(body))
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	events := drainEvents(t, handle.Events)
	if len(events) > laneCapacity {
		t.Fatalf("lane held %d events, want at most its %d-slot capacity", len(events), laneCapacity)
	}
	terminal := terminalEvent(t, events)
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerComplete)
	}
	if terminal.Progress.BytesSent != int64(len(body)) {
		t.Fatalf("terminal BytesSent = %d, want %d", terminal.Progress.BytesSent, len(body))
	}
}

// TestContentDispositionResistsHeaderInjection is defense in depth: the
// payload owns sanitization, but a payload implementation that forgets must
// not be able to write a second header.
func TestContentDispositionResistsHeaderInjection(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		name       string
		wantASCII  string
		wantEncode string
	}{
		"plain": {name: "report.pdf", wantASCII: "report.pdf", wantEncode: "report.pdf"},
		"unicode": {
			name:       "報告 2026.pdf",
			wantASCII:  "__ 2026.pdf",
			wantEncode: "%E5%A0%B1%E5%91%8A%202026.pdf",
		},
		"quote and semicolon": {
			name:       `a";b.pdf`,
			wantASCII:  "ab.pdf",
			wantEncode: "a%22%3Bb.pdf",
		},
		"newline": {
			name:       "a\r\nX-Injected: 1.pdf",
			wantASCII:  "aX-Injected: 1.pdf",
			wantEncode: "a%0D%0AX-Injected%3A%201.pdf",
		},
		"empty": {name: "", wantASCII: fallbackDownloadName, wantEncode: fallbackDownloadName},
	}

	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			got := contentDisposition(test.name)
			wantPrefix := `attachment; filename="` + test.wantASCII + `"; filename*=UTF-8''` + test.wantEncode
			if got != wantPrefix {
				t.Fatalf("contentDisposition(%q) = %q, want %q", test.name, got, wantPrefix)
			}
			if strings.ContainsAny(got, "\r\n") {
				t.Fatalf("contentDisposition(%q) kept a line break", test.name)
			}
		})
	}
}

func awaitStatus(t *testing.T, statuses <-chan int) int {
	t.Helper()
	select {
	case status := <-statuses:
		return status
	case <-time.After(30 * time.Second):
		t.Fatal("a competing claim never answered")
		return 0
	}
}

// awaitStatusFor waits for one specific status without assuming it arrives
// first, returning the other statuses it drained along the way so the caller
// can still account for every claim. Ordering between concurrent claims is not
// a property worth asserting; which statuses appear is.
func awaitStatusFor(t *testing.T, statuses <-chan int, want int, within time.Duration) ([]int, bool) {
	t.Helper()
	var others []int
	deadline := time.After(within)
	for {
		select {
		case status := <-statuses:
			if status == want {
				return others, true
			}
			others = append(others, status)
		case <-deadline:
			return others, false
		}
	}
}

// awaitEvent waits for one event rather than for the lane to close, so a test
// can observe a terminal outcome while the server is still live.
func awaitEvent(t *testing.T, events <-chan transfer.ServerEvent, within time.Duration) transfer.ServerEvent {
	t.Helper()
	select {
	case event, open := <-events:
		if !open {
			t.Fatal("event lane closed before a terminal event arrived")
		}
		return event
	case <-time.After(within):
		t.Fatal("no event arrived")
		return transfer.ServerEvent{}
	}
}

// waitForClosedListener proves the accept loop is gone: a terminal outcome
// promises no replay status to anyone who arrives afterwards.
func waitForClosedListener(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := testClient().Do(request)
		if err != nil {
			return
		}
		_ = response.Body.Close()
		if time.Now().After(deadline) {
			t.Fatalf("the listener still answered with %d after a terminal outcome", response.StatusCode)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func renderResponse(t *testing.T, response *http.Response) string {
	t.Helper()
	body := readBody(t, response)
	var rendered strings.Builder
	rendered.WriteString(strconv.Itoa(response.StatusCode))
	for _, name := range []string{
		"Content-Type", "Content-Length", "Cache-Control", "X-Content-Type-Options",
		"Content-Disposition", "Location", "Allow", "Access-Control-Allow-Origin",
	} {
		rendered.WriteString("|" + name + "=" + response.Header.Get(name))
	}
	rendered.WriteString("|body=" + string(body))
	return rendered.String()
}
