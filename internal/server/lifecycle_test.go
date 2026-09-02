package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// TestStartBindsAListenerThatIsReadyOnReturn pins the readiness postcondition:
// the port handed back is already accepting, so a caller may put it in a URL
// and a QR code the instant Start returns.
func TestStartBindsAListenerThatIsReadyOnReturn(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, &stubPayloads{})
	handle := startTestServer(t, server, &stubAuthorizer{})

	if handle.Port < 1 || handle.Port > 65535 {
		t.Fatalf("Start() port = %d, want an assigned port", handle.Port)
	}
	if handle.Events == nil {
		t.Fatal("Start() returned no event channel")
	}

	connection, err := net.Dial("tcp", hostPort(handle.Port))
	if err != nil {
		t.Fatalf("the advertised port was not accepting on return: %v", err)
	}
	_ = connection.Close()
}

// TestStartRefusesAnIncompleteRequest keeps every unusable start on one stable
// code, and leaves nothing bound behind it.
func TestStartRefusesAnIncompleteRequest(t *testing.T) {
	t.Parallel()

	valid := startRequest()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := map[string]struct {
		ctx        context.Context
		request    transfer.ServerStartRequest
		authorizer transfer.ClaimAuthorizer
		payloads   PayloadPort
	}{
		"no context":    {ctx: nil, request: valid, authorizer: &stubAuthorizer{}, payloads: &stubPayloads{}},
		"cancelled":     {ctx: cancelled, request: valid, authorizer: &stubAuthorizer{}, payloads: &stubPayloads{}},
		"no authorizer": {ctx: context.Background(), request: valid, authorizer: nil, payloads: &stubPayloads{}},
		"no payload port": {
			ctx: context.Background(), request: valid, authorizer: &stubAuthorizer{}, payloads: nil,
		},
		"no session": {
			ctx:        context.Background(),
			request:    transfer.ServerStartRequest{Token: valid.Token, Item: valid.Item},
			authorizer: &stubAuthorizer{},
			payloads:   &stubPayloads{},
		},
		"no token": {
			ctx:        context.Background(),
			request:    transfer.ServerStartRequest{SessionID: valid.SessionID, Item: valid.Item},
			authorizer: &stubAuthorizer{},
			payloads:   &stubPayloads{},
		},
		"no staged item": {
			ctx:        context.Background(),
			request:    transfer.ServerStartRequest{SessionID: valid.SessionID, Token: valid.Token},
			authorizer: &stubAuthorizer{},
			payloads:   &stubPayloads{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, test.payloads)
			bound := false
			server.listen = func(ctx context.Context, _ string) (net.Listener, error) {
				bound = true
				var config net.ListenConfig
				return config.Listen(ctx, "tcp", "127.0.0.1:0")
			}

			handle, err := server.Start(test.ctx, test.request, test.authorizer)
			assertStartFailed(t, handle, err)
			if bound {
				t.Fatal("an unusable start still bound a listener")
			}
			if err := server.Stop(); err != nil {
				t.Fatalf("Stop() after a failed start = %v", err)
			}
		})
	}
}

// TestStartRefusesASecondSession keeps the one-process, one-transfer rule from
// silently replacing a live listener.
func TestStartRefusesASecondSession(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, &stubPayloads{})
	handle := startTestServer(t, server, &stubAuthorizer{})

	second, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
	assertStartFailed(t, second, err)

	// The first server is untouched.
	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the live server stopped answering: status = %d", response.StatusCode)
	}
	readBody(t, response)
}

// TestFailedBindLeavesNothingBehind covers the "Start fails" row: the coded
// failure carries the bind cause for diagnosis, the handle is empty, and Stop
// stays safe afterwards.
func TestFailedBindLeavesNothingBehind(t *testing.T) {
	t.Parallel()

	cause := errors.New("bind: address already in use")
	server := newTestServer(t, &stubPayloads{})
	server.listen = func(context.Context, string) (net.Listener, error) { return nil, cause }

	handle, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
	assertStartFailed(t, handle, err)
	if !errors.Is(err, cause) {
		t.Fatal("the bind cause was not preserved through Unwrap")
	}
	if strings.Contains(err.Error(), string(testToken)) {
		t.Fatal("the start failure disclosed the capability token")
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() after a failed start = %v", err)
	}
}

// TestStartClosesTheListenerWhenSetupLosesToCancellation proves the
// transactional rule at the one point it can actually be lost: after the
// socket exists.
func TestStartClosesTheListenerWhenSetupLosesToCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var port int
	server := newTestServer(t, &stubPayloads{})
	server.listen = func(_ context.Context, _ string) (net.Listener, error) {
		var config net.ListenConfig
		listener, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port = listener.Addr().(*net.TCPAddr).Port
		// The caller cancels while the bind is in flight.
		cancel()
		return listener, nil
	}

	handle, err := server.Start(ctx, startRequest(), &stubAuthorizer{})
	assertStartFailed(t, handle, err)
	if port == 0 {
		t.Fatal("the test never bound a listener")
	}

	connection, dialErr := net.DialTimeout("tcp", hostPort(port), time.Second)
	if dialErr == nil {
		_ = connection.Close()
		t.Fatal("a cancelled start left its listener bound")
	}
}

// TestStopIsSafeAtEveryPointInTheLifecycle covers the whole Stop row: before
// Start, after a completed transfer, and repeated. Every return is quiescent
// and the event channel stays closed for good.
func TestStopIsSafeAtEveryPointInTheLifecycle(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() before Start = %v", err)
	}

	handle := startTestServer(t, server, &stubAuthorizer{})
	active := server.active

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	readBody(t, response)

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() after a completed transfer = %v", err)
	}
	assertQuiescent(t, active)
	if terminal := terminalEvent(t, drainEvents(t, handle.Events)); terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s, want %s", terminal.Kind, transfer.ServerComplete)
	}

	// Repeating Stop is a no-op, and the lane stays closed rather than being
	// reopened for a second teardown.
	for attempt := range 3 {
		if err := server.Stop(); err != nil {
			t.Fatalf("repeated Stop() call %d = %v", attempt+1, err)
		}
		assertQuiescent(t, active)
		if _, open := <-handle.Events; open {
			t.Fatal("the event lane produced an event after it was closed")
		}
	}
}

// TestStopMidTransferIsQuiescentAndSilent covers the mid-request Stop: the
// coordinator owns cancellation, so the server tears down without reporting
// the coordinator's own decision back to it.
func TestStopMidTransferIsQuiescentAndSilent(t *testing.T) {
	t.Parallel()

	streaming := make(chan struct{})
	payload := &stubPayload{
		name:  "report.pdf",
		size:  1 << 20,
		known: true,
		stream: func(ctx context.Context, dst io.Writer) error {
			// Enough bytes to flush the response headers, then park until the
			// data-plane context is cancelled -- exactly what a real payload
			// does when the receiver stops reading.
			if _, err := dst.Write(make([]byte, 32<<10)); err != nil {
				return transfer.WrapError(transfer.ErrTransferFailed, "destination failed", err)
			}
			close(streaming)
			<-ctx.Done()
			return transfer.WrapError(transfer.ErrCancelled, "payload operation was cancelled", ctx.Err())
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})
	active := server.active

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	<-streaming

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() mid-transfer = %v", err)
	}
	assertQuiescent(t, active)
	payload.assertOwnedOnce(t)

	// A cancelled transfer is the coordinator's outcome: the lane closes
	// without a terminal event of its own.
	for _, event := range drainEvents(t, handle.Events) {
		if event.Kind != transfer.ServerProgress {
			t.Fatalf("cancelled teardown emitted a %s event", event.Kind)
		}
	}
}

// TestStopUnblocksAStalledPayload is the reason the teardown order is fixed:
// a payload parked on a write only returns once the destination is
// force-closed, and Stop must not return before it does.
func TestStopUnblocksAStalledPayload(t *testing.T) {
	t.Parallel()

	writing := make(chan struct{})
	payload := &stubPayload{
		name:  "report.pdf",
		size:  1 << 30,
		known: true,
		stream: func(_ context.Context, dst io.Writer) error {
			chunk := make([]byte, 64<<10)
			close(writing)
			for {
				// Ignores cancellation on purpose: this payload can only be
				// stopped by the destination it is blocked on.
				if _, err := dst.Write(chunk); err != nil {
					return transfer.WrapError(transfer.ErrTransferFailed, "destination failed", err)
				}
			}
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	handle := startTestServer(t, server, &stubAuthorizer{})
	active := server.active

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	// The receiver stops reading without closing: the socket buffers fill and
	// the payload's write parks in the kernel, which is the state a whole-
	// transfer write deadline would otherwise be needed to break.
	defer func() { _ = response.Body.Close() }()
	<-writing

	done := make(chan error, 1)
	go func() { done <- server.Stop() }()
	select {
	case stopErr := <-done:
		if stopErr != nil {
			t.Fatalf("Stop() = %v", stopErr)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("Stop() did not return while a payload was stalled on a write")
	}
	assertQuiescent(t, active)
	payload.assertOwnedOnce(t)
}

// TestServerConfigurationIsPinned guards the values compilation cannot check.
// A wrong bind address publishes nothing to the LAN; a missing header or read
// timeout lets a handful of sockets hold the listener open; a write timeout
// would kill a large transfer mid-stream.
func TestServerConfigurationIsPinned(t *testing.T) {
	t.Parallel()

	if listenAddress != "0.0.0.0:0" {
		t.Fatalf("listenAddress = %q, want every interface on an assigned port", listenAddress)
	}

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	handle := startTestServer(t, server, &stubAuthorizer{})
	config := server.active.http

	if config.ReadHeaderTimeout != readHeaderTimeout || config.ReadHeaderTimeout <= 0 {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", config.ReadHeaderTimeout, readHeaderTimeout)
	}
	if config.ReadTimeout != readTimeout || config.ReadTimeout <= 0 {
		t.Fatalf("ReadTimeout = %v, want %v", config.ReadTimeout, readTimeout)
	}
	if config.IdleTimeout != idleTimeout || config.IdleTimeout <= 0 {
		t.Fatalf("IdleTimeout = %v, want %v", config.IdleTimeout, idleTimeout)
	}
	if config.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want none: it would cap the transfer itself", config.WriteTimeout)
	}
	if config.MaxHeaderBytes != maxHeaderBytes || config.MaxHeaderBytes >= http.DefaultMaxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", config.MaxHeaderBytes, maxHeaderBytes)
	}
	// Presence is not the property: a logger pointed anywhere else still prints
	// the diagnostics this asserts are silenced.
	if config.ErrorLog == nil {
		t.Fatal("ErrorLog is nil, so net/http would print request diagnostics carrying the token")
	}
	if config.ErrorLog.Writer() != io.Discard {
		t.Fatal("ErrorLog does not write to io.Discard, so net/http request diagnostics reach an output again")
	}

	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	readBody(t, response)
	if !response.Close {
		t.Fatal("the response kept the connection alive after a one-shot download")
	}
}

// TestOversizedRequestHeadersAreRefused covers the connection-limits row from
// the wire, where a slow or bloated request never reaches the handler.
func TestOversizedRequestHeadersAreRefused(t *testing.T) {
	t.Parallel()

	payloads := &stubPayloads{}
	authorizer := &stubAuthorizer{}
	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)

	connection, err := net.Dial("tcp", hostPort(handle.Port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}

	request := "GET /download/" + string(testToken) + " HTTP/1.1\r\nHost: 127.0.0.1\r\n" +
		"X-Bloat: " + strings.Repeat("a", 4*maxHeaderBytes) + "\r\n\r\n"
	if _, err := io.WriteString(connection, request); err != nil {
		// A server that closed the connection mid-write has already refused it.
		if !isConnectionFailure(err) {
			t.Fatalf("writing the oversized request: %v", err)
		}
	}

	status, readErr := bufio.NewReader(connection).ReadString('\n')
	switch {
	case readErr != nil:
		if !isConnectionFailure(readErr) {
			t.Fatalf("reading the response: %v", readErr)
		}
	case strings.Contains(status, "200"):
		t.Fatalf("an oversized request was served: %q", status)
	}
	if got := authorizer.calls.Load(); got != 0 {
		t.Fatalf("AuthorizeClaim ran %d times for an oversized request, want 0", got)
	}
	if got := payloads.calls.Load(); got != 0 {
		t.Fatalf("Prepare ran %d times for an oversized request, want 0", got)
	}
}

// TestServerImplementsThePort is the compile-time half of the contract; the
// runtime half is every test above.
// TestStartRequestsEveryInterface pins the one production value the rest of
// this package cannot observe. Every other test replaces the listen seam with a
// loopback binder that discards its argument -- necessary, or the fixture would
// face the LAN and trip a firewall prompt -- so nothing else notices what
// address Start actually asks for. Binding loopback in production would leave
// the QR code and URL advertising a LAN address whose port is not listening,
// with the suite fully green.
func TestStartRequestsEveryInterface(t *testing.T) {
	t.Parallel()

	server := New(payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	var requested []string
	var mu sync.Mutex
	server.listen = func(ctx context.Context, address string) (net.Listener, error) {
		mu.Lock()
		requested = append(requested, address)
		mu.Unlock()
		var config net.ListenConfig
		return config.Listen(ctx, "tcp", "127.0.0.1:0")
	}
	server.now = newTestClock(0).now

	handle, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	_ = handle

	mu.Lock()
	defer mu.Unlock()
	if len(requested) != 1 {
		t.Fatalf("listen called %d times, want exactly 1", len(requested))
	}
	if requested[0] != listenAddress {
		t.Fatalf("Start bound %q, want %q -- a loopback bind is unreachable from the receiver",
			requested[0], listenAddress)
	}
}

// TestEnterRefusesOnceTeardownHasBegun pins the gate directly, because the
// integration test below cannot reach it reliably: the window between
// beginStop and the listener closing is narrow enough that concurrent requests
// almost always fail at connect instead. Driven at this level the gate is
// deterministic, and it is what keeps handlers.Add from racing handlers.Wait --
// which is either a WaitGroup misuse panic or a handler admitted after Stop
// returned, still holding a payload descriptor.
func TestEnterRefusesOnceTeardownHasBegun(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	startTestServer(t, server, &stubAuthorizer{})
	active := server.active

	if !active.enter() {
		t.Fatal("enter() refused before teardown began")
	}
	active.leave()

	active.beginStop()

	if active.enter() {
		active.leave()
		t.Fatal("enter() admitted a request after teardown began, so a handler can join the WaitGroup Stop is already waiting on")
	}
	// Still refused on a second attempt: the gate is a state, not a one-shot.
	if active.enter() {
		active.leave()
		t.Fatal("enter() admitted a later request after teardown began")
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	assertQuiescent(t, active)
}

// TestRequestArrivingDuringStopIsRefused drives the one contended path behind
// the quiescence postcondition. The gate in enter() is what stops a request
// from joining the WaitGroup that Stop is already waiting on; without it the
// race is either a WaitGroup misuse panic or a handler admitted after Stop
// returned, still holding a payload descriptor.
func TestRequestArrivingDuringStopIsRefused(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	handle := startTestServer(t, server, &stubAuthorizer{})
	url := downloadURL(handle.Port, string(testToken))
	// Stop clears active, so capture the run to inspect afterwards.
	active := server.active

	var requests sync.WaitGroup
	for range 8 {
		requests.Add(1)
		go func() {
			defer requests.Done()
			request, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Error(err)
				return
			}
			// Either answer is correct: a refusal, or a connection error once
			// the listener is gone. Neither may hang or panic.
			response, err := testClient().Do(request)
			if err != nil {
				return
			}
			_ = response.Body.Close()
		}()
	}

	stopErr := server.Stop()
	requests.Wait()

	if stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}
	assertQuiescent(t, active)
	if err := server.Stop(); err != nil {
		t.Fatalf("repeated Stop() error = %v", err)
	}
}

func TestServerImplementsThePort(t *testing.T) {
	t.Parallel()

	var port transfer.ServerPort = New(&stubPayloads{})
	if port == nil {
		t.Fatal("New returned nothing")
	}
}

func assertStartFailed(t *testing.T, handle transfer.ServerHandle, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Start() succeeded, want a coded failure")
	}
	if code := transfer.ErrorCodeOf(err); code != transfer.ErrServerStartFailed {
		t.Fatalf("Start() error code = %q, want %q", code, transfer.ErrServerStartFailed)
	}
	if handle.Port != 0 || handle.Events != nil {
		t.Fatalf("failed Start() returned a live handle: %+v", handle)
	}
}

// assertQuiescent reads the run's own bookkeeping: the accept loop returned,
// no handler is in flight, and no connection is still tracked.
func assertQuiescent(t *testing.T, active *run) {
	t.Helper()
	if active == nil {
		t.Fatal("no run to inspect")
	}
	select {
	case <-active.serveDone:
	default:
		t.Fatal("the accept loop was still running when Stop returned")
	}
	active.mu.Lock()
	live := len(active.conns)
	stopping := active.stopping
	active.mu.Unlock()
	if live != 0 {
		t.Fatalf("%d connections were still live when Stop returned", live)
	}
	if !stopping {
		t.Fatal("the run still admits new handlers after Stop returned")
	}
	if active.ctx.Err() == nil {
		t.Fatal("the data-plane context was still live when Stop returned")
	}
}
