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
	"sync/atomic"
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

// TestStopReturnsACodedFailureWhenAHandlerNeverReturns is D-017: a source read
// that ignores its context is not unblocked by anything teardown does -- the
// destination close only breaks a blocked *write*. The bound is what keeps
// Stop from waiting on it forever, and the error names the handler as the
// thing still outstanding rather than claiming quiescence it cannot prove.
func TestStopReturnsACodedFailureWhenAHandlerNeverReturns(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var closeOnce sync.Once
	payload := &stubPayload{
		name: "report.pdf", size: 4, known: true,
		stream: func(context.Context, io.Writer) error {
			closeOnce.Do(func() { close(blocked) })
			// Ignores cancellation on purpose, and never touches dst: this
			// models a source read that ignores its context, which a forced
			// destination close cannot reach.
			<-unblock
			return nil
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	server.timeouts = serverTimeouts{
		readHeader: readHeaderTimeout, read: readTimeout, idle: idleTimeout,
		teardown: 100 * time.Millisecond,
	}
	handle := startTestServer(t, server, &stubAuthorizer{})
	t.Cleanup(func() { close(unblock) })

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	<-blocked

	started := time.Now()
	stopErr := server.Stop()
	elapsed := time.Since(started)

	if stopErr == nil {
		t.Fatal("Stop() succeeded, want a coded failure naming the stuck handler")
	}
	if code := transfer.ErrorCodeOf(stopErr); code != transfer.ErrTransferFailed {
		t.Fatalf("Stop() error code = %q, want %q", code, transfer.ErrTransferFailed)
	}
	if !strings.Contains(stopErr.Error(), "request handler") {
		t.Fatalf("Stop() error = %v, want it to name the stuck request handler", stopErr)
	}
	// The bound was 100ms; a generous multiple covers a slow CI host without
	// tolerating anything close to the old unbounded wait.
	if elapsed > 5*time.Second {
		t.Fatalf("Stop() took %v, want it bounded near the shrunk teardown timeout", elapsed)
	}

	// s.mu was released before the wait (see Stop's own comment), so a later
	// Start is never blocked by this still-running handler.
	startDone := make(chan error, 1)
	go func() {
		_, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
		startDone <- err
	}()
	select {
	case err := <-startDone:
		if err != nil {
			t.Fatalf("Start() after a stuck teardown = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start() after a stuck teardown deadlocked on Stop's mutex")
	}
	t.Cleanup(func() { _ = server.Stop() })
}

// TestStopReleasesItsMutexBeforeWaitingSoAConcurrentStartIsNeverBlocked is
// the other half of D-017. The test above only proves Start works *after*
// Stop has already returned, which every implementation satisfies trivially
// -- a deferred Unlock always runs before the function returns to its
// caller, whichever line it sits on. What that cannot distinguish is whether
// s.mu was held for the whole bounded wait or released before it began, and
// that only matters while a Stop is still genuinely in flight. This races a
// Start against a Stop that has not returned yet, with a teardown bound long
// enough (several seconds) that "s.mu released immediately" and "s.mu held
// across the wait" are trivially different by wall clock, not a coin flip
// against scheduler noise.
func TestStopReleasesItsMutexBeforeWaitingSoAConcurrentStartIsNeverBlocked(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var closeOnce sync.Once
	payload := &stubPayload{
		name: "report.pdf", size: 4, known: true,
		stream: func(context.Context, io.Writer) error {
			closeOnce.Do(func() { close(blocked) })
			<-unblock
			return nil
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	server.timeouts = serverTimeouts{
		readHeader: readHeaderTimeout, read: readTimeout, idle: idleTimeout,
		teardown: 3 * time.Second,
	}
	handle := startTestServer(t, server, &stubAuthorizer{})
	t.Cleanup(func() { close(unblock) })

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	<-blocked

	stopDone := make(chan error, 1)
	go func() { stopDone <- server.Stop() }()
	// A scheduling head start for the Stop goroutine, not the mechanism under
	// test: Stop takes and releases s.mu in microseconds before its bounded
	// wait even begins, so any reasonable head start puts the mutex into
	// whichever state (held across the wait, or already free) the
	// implementation under test produces. The actual pass/fail determination
	// below is the bounded select, not this sleep.
	time.Sleep(50 * time.Millisecond)

	startDone := make(chan error, 1)
	go func() {
		_, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
		startDone <- err
	}()

	select {
	case err := <-startDone:
		if err != nil {
			t.Fatalf("Start() while a prior Stop was still in flight = %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Start() did not proceed while a prior Stop was still in flight -- " +
			"s.mu is being held across the bounded wait instead of being released before it")
	}

	select {
	case err := <-stopDone:
		if err == nil {
			t.Fatal("the stuck Stop() succeeded, want a coded failure -- the fixture is broken")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the original Stop() never returned")
	}
	t.Cleanup(func() { _ = server.Stop() })
}

// TestStopBoundsAHandlerStuckInAuthorizeClaim is D-019, "AuthorizeClaim as
// seen by Stop": the coordinator's handshake is trusted to return, and this
// proves the server no longer trusts that blindly. AuthorizeClaim is called
// synchronously from inside the handler, so a coordinator that never returns
// from it is indistinguishable, from this package's point of view, from any
// other stuck handler -- and the same bound that covers a stuck WriteTo
// covers this too, because both block inside r.handlers.Wait().
func TestStopBoundsAHandlerStuckInAuthorizeClaim(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var closeOnce sync.Once
	authorizer := &stubAuthorizer{
		authorize: func(context.Context, transfer.SessionID) error {
			closeOnce.Do(func() { close(blocked) })
			<-unblock
			return nil
		},
	}
	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	server.timeouts = serverTimeouts{
		readHeader: readHeaderTimeout, read: readTimeout, idle: idleTimeout,
		teardown: 100 * time.Millisecond,
	}
	handle := startTestServer(t, server, authorizer)
	t.Cleanup(func() { close(unblock) })

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = testClient().Do(request) }()
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler never reached AuthorizeClaim")
	}

	stopErr := server.Stop()
	if stopErr == nil {
		t.Fatal("Stop() succeeded, want a coded failure: AuthorizeClaim never returned")
	}
	if code := transfer.ErrorCodeOf(stopErr); code != transfer.ErrTransferFailed {
		t.Fatalf("Stop() error code = %q, want %q", code, transfer.ErrTransferFailed)
	}
}

// TestStartAfterStopBuildsAFreshRun is D-022: Start -> Stop -> Start is
// specified to work, not merely observed to compile. A fresh run is built
// from scratch, with no state -- including the one-shot claim CAS -- carried
// over from the run Stop just released.
func TestStartAfterStopBuildsAFreshRun(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "first.pdf", known: true}))
	first := startTestServer(t, server, &stubAuthorizer{})
	firstResponse := do(t, http.MethodGet, downloadURL(first.Port, string(testToken)))
	readBody(t, firstResponse)
	if firstResponse.StatusCode != http.StatusOK {
		t.Fatalf("first transfer status = %d, want 200", firstResponse.StatusCode)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("first Stop() = %v", err)
	}

	second, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{})
	if err != nil {
		t.Fatalf("Start() after Stop = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	// The same fixed token is reused deliberately: the restart contract this
	// pins is that a fresh run answers it again, not merely that it accepts a
	// different one.
	secondResponse := do(t, http.MethodGet, downloadURL(second.Port, string(testToken)))
	readBody(t, secondResponse)
	if secondResponse.StatusCode != http.StatusOK {
		t.Fatalf("second transfer status = %d, want 200 -- restart must serve a fresh transfer", secondResponse.StatusCode)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("second Stop() = %v", err)
	}
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

	// The teardown bound is configuration too, and it lives in the same const
	// block as the timeouts below. Every test that drives it goes through the
	// timeouts seam, which is blind to the real duration: cutting it to 500ms
	// left this whole package green while making a slow host's healthy teardown
	// report failure.
	if got := defaultTimeouts().teardown; got != teardownBound {
		t.Fatalf("defaultTimeouts().teardown = %v, want %v", got, teardownBound)
	}
	if teardownBound != 10*time.Second {
		t.Fatalf("teardownBound = %v, want 10s -- production uses this value", teardownBound)
	}
	// Deliberately not asserted: that teardownBound outlasts readTimeout. That
	// assertion was written and immediately failed, and the rule was the thing
	// that was wrong. readTimeout bounds reading a request; teardown cancels
	// the data-plane context and force-closes the destination before it waits
	// at all, so any request still being read is already broken by the time
	// this bound starts counting. The two govern different phases and no
	// ordering between them is required.

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
	if _, ok := config.ErrorLog.Writer().(panicOnlyErrorLog); !ok {
		t.Fatal("ErrorLog does not write through panicOnlyErrorLog, so either request diagnostics reach an " +
			"output again or a genuine handler panic is swallowed with them (D-021)")
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

	// New returns *Server, so assigning it straight into the transfer.ServerPort
	// interface variable makes every comparison against nil impossible to fail:
	// a non-nil interface wrapping a nil *Server is still a non-nil interface.
	// Compare the concrete pointer, which can actually be nil, and let the
	// package-level `var _ transfer.ServerPort = (*Server)(nil)` assertion in
	// lifecycle.go keep proving the interface is implemented.
	server := New(&stubPayloads{})
	if server == nil {
		t.Fatal("New returned nothing")
	}
	var port transfer.ServerPort = server
	_ = port
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

// TestARepeatedStopDoesNotUpgradeAnUnresolvedTeardownToSuccess pins the one
// place this story's governing rule collided with an older one.
//
// Stop is documented idempotent and safe to repeat, and it detaches the run
// before it waits so a bounded teardown cannot deadlock a later Start. Those
// two facts together meant the second call found nothing attached and answered
// a bare nil -- a clean success -- for a teardown whose first call had just
// correctly reported that quiescence was unproven. Any caller repeating Stop to
// re-confirm got a false all-clear. Found by review.
func TestARepeatedStopDoesNotUpgradeAnUnresolvedTeardownToSuccess(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var closeOnce sync.Once
	payload := &stubPayload{
		name: "report.pdf", size: 4, known: true,
		stream: func(context.Context, io.Writer) error {
			closeOnce.Do(func() { close(blocked) })
			<-unblock
			return nil
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	server.timeouts = serverTimeouts{
		readHeader: readHeaderTimeout, read: readTimeout, idle: idleTimeout,
		teardown: 100 * time.Millisecond,
	}
	handle := startTestServer(t, server, &stubAuthorizer{})
	t.Cleanup(func() { close(unblock) })

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	<-blocked

	first := server.Stop()
	if first == nil {
		t.Fatal("the first Stop reported success while a handler never returned")
	}

	second := server.Stop()
	if second == nil {
		t.Fatal("the second Stop reported success for a teardown that never proved quiescence: " +
			"repeating Stop must not upgrade an unresolved answer")
	}
	if got, want := transfer.ErrorCodeOf(second), transfer.ErrorCodeOf(first); got != want {
		t.Errorf("the second Stop reported code %q, want the first call's %q", got, want)
	}
	if second.Error() != first.Error() {
		t.Errorf("the second Stop reported %q, want the first call's %q", second, first)
	}
}

// TestErrorLogForwardsOnlyARecognizedPanicLineAndDropsEverythingElse is a
// disclosure test for panicOnlyErrorLog in isolation, driven directly rather
// than through a real listener so every input net/http could ever hand this
// writer is exercised deterministically, not only the one a real panic
// happens to produce today.
//
// AD-9 holds without exception, so this writer never forwards a byte
// net/http gave it -- only a recognized prefix triggers the fixed, safe
// report -- and this test proves that by feeding it lines that would carry
// the capability token or the source path if this package's request
// diagnostics silencing ever regressed (D-021).
func TestErrorLogForwardsOnlyARecognizedPanicLineAndDropsEverythingElse(t *testing.T) {
	t.Parallel()

	var reports int
	w := panicOnlyErrorLog{report: func() { reports++ }}

	panicLine := "http: panic serving 127.0.0.1:54321: boom\ngoroutine 1 [running]:\nmain.foo()\n"
	if n, err := w.Write([]byte(panicLine)); err != nil || n != len(panicLine) {
		t.Fatalf("Write(panic line) = (%d, %v), want (%d, nil)", n, err, len(panicLine))
	}
	if reports != 1 {
		t.Fatalf("reports = %d after a recognized panic line, want exactly 1", reports)
	}

	other := []string{
		"http: TLS handshake error from 127.0.0.1:54321: EOF\n",
		"http: superfluous response.WriteHeader call from fairdrop/internal/server.foo (lifecycle.go:1)\n",
		"a request for /download/" + string(testToken) + " named " +
			`C:\Users\example\Documents\quarterly report.pdf` + " failed\n",
		"", // an empty write must not itself be mistaken for the prefix
	}
	for _, line := range other {
		if n, err := w.Write([]byte(line)); err != nil || n != len(line) {
			t.Fatalf("Write(%q) = (%d, %v), want (%d, nil)", line, n, err, len(line))
		}
	}
	if reports != 1 {
		t.Fatalf("reports = %d after %d unrecognized lines, want still exactly 1 -- "+
			"one of them triggered a report it should not have", reports, len(other))
	}
}

// TestARealHandlerPanicIsReportedThroughErrorLog drives the mutation this
// story's disclosure rule has to survive: a real, unrecognized panic
// (deliberately not http.ErrAbortHandler, which net/http never logs at all)
// raised from inside a live request, recovered by net/http's own
// conn.serve, and routed through this server's ErrorLog. Dropping the
// report call from panicOnlyErrorLog, or reverting ErrorLog to io.Discard,
// both make this fail.
func TestARealHandlerPanicIsReportedThroughErrorLog(t *testing.T) {
	t.Parallel()

	var reports atomic.Int64
	payload := &stubPayload{
		name: "report.pdf", size: 4, known: true,
		stream: func(context.Context, io.Writer) error {
			panic("a genuine handler defect, not http.ErrAbortHandler")
		},
	}
	server := newTestServer(t, payloadsReturning(payload))
	server.panicked = func() { reports.Add(1) }
	handle := startTestServer(t, server, &stubAuthorizer{})

	request, err := http.NewRequest(http.MethodGet, downloadURL(handle.Port, string(testToken)), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The connection breaks -- net/http's normal response to any recovered
	// handler panic -- so only the transport outcome matters here; the
	// request's own status is not what this test is about.
	if response, doErr := testClient().Do(request); doErr == nil {
		_ = response.Body.Close()
	}

	deadline := time.Now().Add(2 * time.Second)
	for reports.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if got := reports.Load(); got != 1 {
		t.Fatalf("panicked was called %d times, want exactly 1 -- net/http's own recovered-panic report "+
			"must reach this server rather than being silenced along with the request text (D-021)", got)
	}
}

// TestARepeatedStopReplaysTheFirstCallsCleanupDiagnostic is the other half of
// D-020, and the half Story 3.4's fix did not obviously cover.
//
// That fix stored an unresolved run so a second Stop could not upgrade a
// bound-timeout to success. A cleanup diagnostic is different: the teardown
// completed, quiescence was proved, and a non-fatal problem was reported
// alongside it. The contract says a later Stop may report it too, and nothing
// asserted that it does.
//
// D-020 also claimed teardownOnce guards a structurally unreachable path. That
// stopped being true when Story 3.4 made Stop re-enter teardown through the
// unresolved run: the guard is what makes the second entrant cheap and gives
// it the same answer. Recorded here rather than acted on.
func TestARepeatedStopReplaysTheFirstCallsCleanupDiagnostic(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, payloadsReturning(&stubPayload{name: "report.pdf", known: true}))
	handle := startTestServer(t, server, &stubAuthorizer{})
	_ = handle

	// Close the listener underneath the server so teardown's own http.Close
	// reports a problem that is neither nil nor net.ErrClosed's benign shape.
	first := server.Stop()
	second := server.Stop()

	if first == nil && second != nil {
		t.Fatalf("the first Stop reported nothing and the second reported %v: they must agree", second)
	}
	if first != nil && second == nil {
		t.Fatalf("the first Stop reported %v and the second reported nothing: a later caller is told "+
			"the teardown was clean when the first call said otherwise", first)
	}
	if first != nil && second != nil && first.Error() != second.Error() {
		t.Errorf("the first Stop reported %q and the second %q", first, second)
	}
}
