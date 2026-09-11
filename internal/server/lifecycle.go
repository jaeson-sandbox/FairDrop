package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fairdrop/internal/transfer"
)

const (
	// listenAddress binds every interface on an OS-assigned port. The address
	// is deliberately not the selected LAN address: a receiver may reach the
	// sender over any interface the router hands it, and the port is ephemeral
	// because the listener lives only as long as one staged transfer.
	listenAddress = "0.0.0.0:0"

	// maxHeaderBytes is far below net/http's 1 MiB default. The only request
	// this server answers is a bare GET of a fixed-shape path, so anything
	// larger is either a mistake or an attempt to make the sender hold memory.
	maxHeaderBytes = 8 << 10

	// readHeaderTimeout bounds a receiver that opens a connection and dribbles
	// its request line. Without it a handful of sockets can pin the one-shot
	// listener open indefinitely.
	readHeaderTimeout = 10 * time.Second

	// readTimeout bounds the whole request read. A download request carries no
	// body, so this is a ceiling on pathological clients, not on the transfer.
	// There is deliberately no WriteTimeout: a write deadline would cap the
	// transfer itself, killing a large file over a slow link mid-stream.
	//
	// Verified rather than assumed, because it is easy to read net/http the
	// other way: the server keeps a read deadline armed while a request is
	// being read, but startBackgroundRead clears it before the background
	// disconnect-detection read begins, so for a bodiless GET the deadline is
	// spent by the time the handler runs and never cancels request.Context().
	// TestATransferLongerThanEveryTimeoutStillCompletes streams a body longer
	// than every deadline through a real listener; it fails the moment a
	// WriteTimeout is added, and was used to disprove the read-deadline theory.
	readTimeout = 20 * time.Second

	// idleTimeout reaps a connection that claims nothing. Keep-alives are
	// disabled, so this covers the window between accept and request only.
	idleTimeout = 30 * time.Second

	// teardownBound caps how long a single teardown waits for the accept loop
	// to exit, every handler to return, and every tracked connection to close,
	// once the data-plane context is cancelled and the destination is
	// force-closed. A real payload is unblocked by that force-close within
	// milliseconds -- TestStopUnblocksAStalledPayload proves it against a
	// payload that ignores cancellation and writes forever -- so this bound
	// exists only for the one case a forced destination close cannot reach: a
	// source read that also ignores its context and never returns at all. Ten
	// seconds is generous headroom above that millisecond reality for a slow
	// host, never a ceiling a real transfer could approach, because teardown
	// only starts after Stop is called -- the transfer's own bytes are never
	// what this bound waits on.
	teardownBound = 10 * time.Second
)

// listenFunc is the bind seam. Tests use it to bind loopback instead of every
// interface, and to force a bind failure without occupying a real port.
type listenFunc func(ctx context.Context, address string) (net.Listener, error)

// Server is the ephemeral one-shot HTTP server: one listener, one capability
// token, one authorized download, then nothing -- for the run Start creates.
//
// The Server itself is reusable: Start after a completed Stop builds a fresh
// run from scratch, with no state surviving from the one before it. Every
// seam (listen, now, timeouts) is reapplied, and a run that never fully
// quiesced -- because its teardown hit its bound -- still lets a new Start
// proceed, because Stop clears s.active and releases s.mu before it waits on
// anything (see Stop below). What is not reusable is a *run*: once Stop has
// cleared it, the same run is never resumed or reattached.
//
// It owns no session state of its own. The coordinator decides whether a claim
// may proceed, and this type's whole job is to make that decision the only way
// through: reserve atomically, authorize synchronously, and open a payload
// only after both have succeeded.
type Server struct {
	payloads PayloadPort
	listen   listenFunc
	now      clock
	// timeouts is a seam so a test can shrink every net/http deadline and prove
	// a transfer that outlives all of them still completes. Production is
	// always defaultTimeouts.
	timeouts serverTimeouts
	// panicked is called, with no argument, exactly when net/http recovers a
	// handler panic other than http.ErrAbortHandler -- the one ErrorLog line
	// this package does not silence (D-021). It is a seam for the same
	// reason timeouts is one: production writes a fixed, safe line to
	// os.Stderr, and a test replaces it to observe the call without a real
	// panicking handler racing a real listener.
	panicked func()

	mu     sync.Mutex
	active *run
	// unresolved holds a run whose teardown ended without proving
	// quiescence, so a later Stop repeats that answer instead of reporting
	unresolved *run
}

// serverTimeouts are the net/http deadlines one server applies, plus the
// teardown bound below them. teardown is a seam for the same reason the three
// net/http deadlines are: a test shrinks it to prove the bound fires, and
// production always gets defaultTimeouts.
type serverTimeouts struct {
	readHeader time.Duration
	read       time.Duration
	idle       time.Duration
	teardown   time.Duration
}

func defaultTimeouts() serverTimeouts {
	return serverTimeouts{
		readHeader: readHeaderTimeout,
		read:       readTimeout,
		idle:       idleTimeout,
		teardown:   teardownBound,
	}
}

var _ transfer.ServerPort = (*Server)(nil)

// New returns a server that serves payloads through the given port. Its
// remaining dependencies -- the network bind and the clock -- are process
// defaults that tests replace in place.
func New(payloads PayloadPort) *Server {
	return &Server{
		payloads: payloads,
		listen: func(ctx context.Context, address string) (net.Listener, error) {
			var config net.ListenConfig
			return config.Listen(ctx, "tcp", address)
		},
		now:      time.Now,
		timeouts: defaultTimeouts(),
		panicked: logHandlerPanic,
	}
}

// logHandlerPanic is the production panicked seam: one fixed line to
// os.Stderr, carrying nothing net/http supplied. internal/server imports no
// application logging package -- there is no App here to hand a seam to, the
// way internal/transfer's Diagnose is wired through main.go -- so this is
// the same stderr surface reached directly, by the same reasoning app.go's
// own logf documents: a value that escaped a panic is adapter text, and
// adapter text is exactly where a path or a capability token would be
// (AD-9).
func logHandlerPanic() {
	_, _ = os.Stderr.WriteString("fairdrop: a request handler panicked; recovered, request details withheld\n")
}

// netHTTPPanicLinePrefix is the fixed prefix net/http's own conn.serve
// writes its ErrorLog line under when it recovers a handler panic other
// than http.ErrAbortHandler ("http: panic serving %v: %v\n%s" in
// net/http/server.go). Matching only this prefix, and never forwarding the
// bytes that follow it, is what lets one net/http mechanism serve both
// jobs: every other line this server could log through the same *log.Logger
// -- which can quote the request line, and therefore the capability token
// in it -- stays silenced exactly as before, while a genuine handler panic
// stops being swallowed along with them (D-021).
const netHTTPPanicLinePrefix = "http: panic serving "

// panicOnlyErrorLog is the io.Writer net/http's ErrorLog is bound to. It
// never forwards a byte net/http handed it: the remote address, the
// panic value, and the recovered stack trace can all vary in ways this
// package has no way to bound, so the disclosure guarantee holds by
// construction rather than by trusting net/http's formatting never changes.
// A recognized panic line instead triggers the fixed, safe report.
type panicOnlyErrorLog struct {
	report func()
}

func (w panicOnlyErrorLog) Write(p []byte) (int, error) {
	if w.report != nil && bytes.HasPrefix(p, []byte(netHTTPPanicLinePrefix)) {
		w.report()
	}
	// net/http's own logger never inspects this return; reporting the full
	// length written is what a normal io.Writer does with input it consumed
	// and chose to withhold rather than reject.
	return len(p), nil
}

// run is one started server: everything acquired by a single Start and
// released by a single teardown. A fresh Start builds a fresh run, so no state
// from a finished transfer can leak into the next one.
type run struct {
	sessionID  transfer.SessionID
	token      transfer.CapabilityToken
	item       transfer.StagedItem
	payloads   PayloadPort
	authorizer transfer.ClaimAuthorizer
	now        clock
	// timeouts carries the teardown bound from the Server that started this
	// run, captured once at Start so a later change to s.timeouts (a test
	// reusing one *Server across cases) cannot reach back into a run already
	// in flight.
	timeouts serverTimeouts

	// ctx is the data-plane context: it governs authorization, payload
	// preparation, and streaming for the whole serving lifetime, and
	// cancelling it is the first step of every teardown.
	ctx    context.Context
	cancel context.CancelFunc

	mux       *http.ServeMux
	http      *http.Server
	listener  *onceCloseListener
	lane      *eventLane
	serveDone chan struct{}

	// claimed is the linearization point of the claim race. Two receivers can
	// hit the same URL at the same instant; the compare-and-swap decides which
	// of them ever reaches the coordinator.
	claimed atomic.Bool

	mu        sync.Mutex
	stopping  bool
	conns     map[net.Conn]struct{}
	connsGone *sync.Cond
	handlers  sync.WaitGroup

	teardownOnce sync.Once
	teardownDone chan struct{}
	teardownErr  error
}

// Start binds the listener and makes the download route live. It returns only
// after the socket is bound and its accept loop is running, so a caller that
// receives a port may hand that port out immediately. Every failure closes
// whatever it had already acquired and returns a server_start_failed error.
func (s *Server) Start(
	ctx context.Context,
	request transfer.ServerStartRequest,
	authorizer transfer.ClaimAuthorizer,
) (transfer.ServerHandle, error) {
	if s == nil {
		return transfer.ServerHandle{}, startError("the transfer server is unavailable", nil)
	}
	if ctx == nil {
		return transfer.ServerHandle{}, startError("transfer server start requires a context", nil)
	}
	if err := startContextError(ctx); err != nil {
		return transfer.ServerHandle{}, err
	}
	if request.SessionID == "" {
		return transfer.ServerHandle{}, startError("transfer server start requires a session", nil)
	}
	if request.Token == "" {
		return transfer.ServerHandle{}, startError("transfer server start requires a capability token", nil)
	}
	if request.Item.Path == "" {
		return transfer.ServerHandle{}, startError("transfer server start requires a staged item", nil)
	}
	if authorizer == nil {
		return transfer.ServerHandle{}, startError("transfer server start requires a claim authorizer", nil)
	}
	if s.payloads == nil {
		return transfer.ServerHandle{}, startError("transfer server start requires a payload port", nil)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return transfer.ServerHandle{}, startError("the transfer server is already running", nil)
	}

	listener, err := s.listen(ctx, listenAddress)
	if err != nil {
		return transfer.ServerHandle{}, startError("the transfer server could not open a local listener", err)
	}
	if listener == nil {
		return transfer.ServerHandle{}, startError("the transfer server did not open a local listener", nil)
	}

	port, err := listenerPort(listener)
	if err != nil {
		return transfer.ServerHandle{}, discardListener(listener, err)
	}
	// A context that was cancelled while the bind was in flight must not leave
	// a listener behind: nothing is retained until every check has passed.
	if err := startContextError(ctx); err != nil {
		return transfer.ServerHandle{}, discardListener(listener, err)
	}

	dataCtx, cancel := context.WithCancel(ctx)
	active := &run{
		sessionID:    request.SessionID,
		token:        request.Token,
		item:         request.Item,
		payloads:     s.payloads,
		authorizer:   authorizer,
		now:          s.clock(),
		timeouts:     s.timeouts,
		ctx:          dataCtx,
		cancel:       cancel,
		mux:          http.NewServeMux(),
		listener:     &onceCloseListener{Listener: &finalizingListener{Listener: listener}},
		lane:         newEventLane(),
		serveDone:    make(chan struct{}),
		conns:        make(map[net.Conn]struct{}),
		teardownDone: make(chan struct{}),
	}
	active.connsGone = sync.NewCond(&active.mu)

	// The pattern is methodless on purpose. A method-qualified "GET /download/
	// {token}" would make ServeMux answer other methods with 405 and an Allow
	// header, and route HEAD into this handler -- both of which tell an
	// unauthorized caller that the resource exists. Handing every method to
	// the handler is what lets a wrong method look exactly like a wrong path.
	active.mux.HandleFunc(downloadPattern, active.download)

	active.http = &http.Server{
		Handler:           http.HandlerFunc(active.route),
		ReadHeaderTimeout: s.timeouts.readHeader,
		ReadTimeout:       s.timeouts.read,
		IdleTimeout:       s.timeouts.idle,
		MaxHeaderBytes:    maxHeaderBytes,
		ConnState:         active.trackConnection,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			return context.WithValue(ctx, responseConnectionKey{}, conn.(*finalizingConn))
		},
		// net/http logs connection and panic diagnostics that can quote a
		// request. Nothing about this server's traffic is safe to print: the
		// path carries the capability token. But net/http also writes a
		// recovered handler panic to this same logger, and discarding
		// everything discarded that too -- a genuine production defect,
		// silenced along with the request text that made silencing anything
		// necessary in the first place (D-021). panicOnlyErrorLog forwards
		// only the fact that a panic happened, through the fixed report
		// below, and drops every byte of the line itself.
		ErrorLog: log.New(panicOnlyErrorLog{report: s.panicked}, "", 0),
	}
	// One request, one response, one connection. Disabling keep-alives means a
	// finished receiver's socket closes instead of idling against a listener
	// that is about to disappear.
	active.http.SetKeepAlivesEnabled(false)

	go active.serve()

	s.active = active
	return transfer.ServerHandle{Port: port, Events: active.lane.channel()}, nil
}

// Stop force-closes the server and returns once it is quiescent -- no
// listener, no connection, no handler, no payload worker, and no event
// producer is still live, and the event channel is closed for good -- or once
// its teardown bound elapses, whichever comes first. It is safe before Start,
// after a failed Start, and when repeated.
//
// A returned error means one of two different things, and a caller must not
// conflate them: an error carrying a cleanup diagnostic (net/http's own
// Close error) means teardown finished and something merely went wrong along
// the way; an error naming a wait that hit its bound means teardown did NOT
// finish and quiescence is unproven -- the accept loop, a handler, or a
// connection may still be running. Neither ever means Stop is unsafe to call
// again: repeating it after either kind of error is still a no-op, because
// s.active is already cleared below.
//
// s.mu is held only long enough to take ownership of s.active and clear it --
// never across the wait itself. A teardown that hits its bound therefore
// cannot deadlock a later Start: by the time anything is waited on, s.active
// is already nil and s.mu is already free.
func (s *Server) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.active == nil {
		// A teardown that hit its bound left this behind. Answering nil here
		// would be the story's own rule broken by the idempotence clause: the
		// first call correctly reported that quiescence was unproven, and a
		// second call must not upgrade that to success just because the run
		// has already been detached. Whatever the first call concluded is what
		// every later caller gets.
		unresolved := s.unresolved
		s.mu.Unlock()
		if unresolved != nil {
			return unresolved.teardown()
		}
		return nil
	}
	active := s.active
	s.active = nil
	s.mu.Unlock()

	err := active.teardown()
	if err != nil {
		s.mu.Lock()
		s.unresolved = active
		s.mu.Unlock()
	}
	return err
}

func (s *Server) clock() clock {
	if s != nil && s.now != nil {
		return s.now
	}
	return time.Now
}

func (r *run) serve() {
	defer close(r.serveDone)
	// Serve returns when the listener closes, which happens either at teardown
	// or the moment a transfer reaches a terminal outcome. Both are expected,
	// so the accept error is not a diagnostic.
	_ = r.http.Serve(r.listener)
}

// teardown releases everything this run owns, in the one order that is safe:
// cancel the data-plane context so workers stop, force-close the destination
// so a blocked write unblocks, wait -- up to the bound -- for the handler and
// every connection to finish, then close the lane. Reversing any pair of
// those steps risks waiting forever on a write that will never complete, or
// closing a payload while it is still being read.
//
// The three waits that follow the forced close are the ones this story
// bounds: they can only be unblocked by the adapter itself returning, and
// nothing here can force a stuck source read to return. A wait that hits its
// bound means quiescence is unproven for whatever is still outstanding, and
// the returned error names it rather than pretending the resource is gone.
// The lane is still closed either way: closing it is always safe (eventLane
// guards every publish with the same mutex its close takes), and doing so
// unblocks the coordinator's own drainer promptly instead of leaving it to
// find out only from its own, independent bound.
func (r *run) teardown() error {
	r.teardownOnce.Do(func() {
		r.beginStop()
		r.cancel()

		closeErr := r.http.Close()
		quiesceErr := r.awaitQuiescence()
		r.lane.close()

		switch {
		case quiesceErr != nil:
			r.teardownErr = quiesceErr
		case closeErr != nil && !errors.Is(closeErr, net.ErrClosed):
			r.teardownErr = transfer.WrapError(
				transfer.ErrTransferFailed,
				"transfer server cleanup reported a problem",
				closeErr,
			)
		}
		close(r.teardownDone)
	})
	<-r.teardownDone
	return r.teardownErr
}

// awaitQuiescence waits, up to r.timeouts.teardown, for the accept loop to
// exit, every handler to return, and every tracked connection to close. A
// sync.WaitGroup and a sync.Cond cannot be selected on directly, so each is
// run on its own goroutine that closes a channel when it finishes, which is
// what makes a bounded select over all three possible.
//
// A bound that elapses leaks whichever of those goroutines is still blocked:
// there is no way in Go to force one to return. That is the documented cost
// of a bound at all, not a new one, and it is why the report names exactly
// what is still outstanding rather than only saying "timed out".
func (r *run) awaitQuiescence() error {
	handlersDone := doneChannel(r.handlers.Wait)
	connsDone := doneChannel(r.awaitConnections)

	bound := time.NewTimer(r.timeoutsOrDefault())
	defer bound.Stop()

	serveDone, thisHandlersDone, thisConnsDone := r.serveDone, handlersDone, connsDone
	for serveDone != nil || thisHandlersDone != nil || thisConnsDone != nil {
		select {
		case <-serveDone:
			serveDone = nil
		case <-thisHandlersDone:
			thisHandlersDone = nil
		case <-thisConnsDone:
			thisConnsDone = nil
		case <-bound.C:
			return teardownTimeoutError(serveDone != nil, thisHandlersDone != nil, thisConnsDone != nil)
		}
	}
	return nil
}

// timeoutsOrDefault covers a *run built without going through Server.Start,
// which no production path does but a focused unit test reasonably might.
func (r *run) timeoutsOrDefault() time.Duration {
	if r.timeouts.teardown > 0 {
		return r.timeouts.teardown
	}
	return teardownBound
}

// doneChannel runs a blocking wait on its own goroutine and reports
// completion by closing a channel, which is what makes a sync.WaitGroup.Wait
// or a sync.Cond.Wait -- neither selectable on its own -- usable inside a
// bounded select.
func doneChannel(wait func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	return done
}

// teardownTimeoutError names exactly which of the three waits was still
// outstanding when the bound elapsed, so a diagnostic reader learns which
// adapter did not return rather than only that something did not.
func teardownTimeoutError(acceptLoop, handlers, connections bool) error {
	var outstanding []string
	if acceptLoop {
		outstanding = append(outstanding, "the accept loop")
	}
	if handlers {
		outstanding = append(outstanding, "a request handler")
	}
	if connections {
		outstanding = append(outstanding, "a connection")
	}
	return transfer.NewError(
		transfer.ErrTransferFailed,
		"transfer server teardown did not finish before its bound: "+strings.Join(outstanding, ", ")+" did not return",
	)
}

// beginStop closes the door on new handlers before anything is torn down, so
// the handler count can only fall from here.
func (r *run) beginStop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopping = true
}

// enter admits one request. It refuses once teardown has begun, which is also
// what keeps the wait group from being incremented after it is waited on.
func (r *run) enter() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopping {
		return false
	}
	r.handlers.Add(1)
	return true
}

func (r *run) leave() {
	r.handlers.Done()
}

// trackConnection records connection liveness so teardown can prove there is
// no socket left, not merely that it asked for one to close. net/http reports
// StateClosed from the connection's own goroutine as it exits, which makes
// this the point where that goroutine is known to be gone.
func (r *run) trackConnection(conn net.Conn, state http.ConnState) {
	r.mu.Lock()
	stopping := r.stopping
	r.mu.Unlock()
	// Keep-alives are disabled. StateClosed follows net/http's finishRequest,
	// including its final buffer flush and terminating chunk, on the serving
	// goroutine. A handler return (or its Flush) is too early to claim success.
	// Keep the connection tracked until its final event has been published,
	// so a concurrent Stop cannot close the lane while this callback produces.
	if state == http.StateClosed && !stopping && r.ctx.Err() == nil {
		if tracked, ok := conn.(*finalizingConn); ok && tracked.terminal != nil {
			event := *tracked.terminal
			if tracked.writeErr != nil && event.Kind == transfer.ServerComplete {
				event = failedEvent(r.sessionID, *event.Progress, transfer.WrapError(
					transfer.ErrTransferFailed, "the HTTP response could not be finalized", tracked.writeErr))
			}
			r.finish(&event)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch state {
	case http.StateNew:
		r.conns[conn] = struct{}{}
	case http.StateHijacked, http.StateClosed:
		delete(r.conns, conn)
		if len(r.conns) == 0 {
			r.connsGone.Broadcast()
		}
	}
}

type responseConnectionKey struct{}

// finalizingConn observes every real connection write, including writes made
// by net/http after our handler returns. Its fields belong to the serving
// goroutine; concurrent Stop only closes the embedded connection.
type finalizingConn struct {
	net.Conn
	writeErr error
	terminal *transfer.ServerEvent
}

func (c *finalizingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil && c.writeErr == nil {
		c.writeErr = err
	}
	return n, err
}

type finalizingListener struct{ net.Listener }

func (l *finalizingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &finalizingConn{Conn: conn}, nil
}

func (r *run) awaitConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.conns) > 0 {
		r.connsGone.Wait()
	}
}

// closeListener ends the accept loop without touching the connection being
// served. A terminal outcome consumes the one-shot capability immediately,
// well before the coordinator gets around to calling Stop, and no HTTP status
// is promised to anyone who arrives after it.
func (r *run) closeListener() {
	_ = r.listener.Close()
}

// onceCloseListener makes the listener safe to close from two owners: the
// handler that finished a transfer and the teardown that closes everything.
// net/http's own wrapper only dedupes its own closes, so without this the
// second close would surface as a cleanup diagnostic on a healthy transfer.
type onceCloseListener struct {
	net.Listener
	once sync.Once
	err  error
}

func (l *onceCloseListener) Close() error {
	l.once.Do(func() { l.err = l.Listener.Close() })
	return l.err
}

func listenerPort(listener net.Listener) (int, error) {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address == nil {
		return 0, startError("the transfer server bound an unusable address", nil)
	}
	if address.Port < 1 || address.Port > 65535 {
		return 0, startError("the transfer server bound an unusable port", nil)
	}
	return address.Port, nil
}

func discardListener(listener net.Listener, cause error) error {
	_ = listener.Close()
	return cause
}

func startError(safeMessage string, cause error) error {
	if cause == nil {
		return transfer.NewError(transfer.ErrServerStartFailed, safeMessage)
	}
	return transfer.WrapError(transfer.ErrServerStartFailed, safeMessage, cause)
}

// startContextError keeps a cancelled start on the start-failure code rather
// than reporting cancellation: nothing was staged yet, so there is no transfer
// to have been cancelled.
func startContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return startError("the transfer server start was cancelled", err)
	}
	return nil
}
