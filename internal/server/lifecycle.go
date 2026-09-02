package server

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
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
	readTimeout = 20 * time.Second

	// idleTimeout reaps a connection that claims nothing. Keep-alives are
	// disabled, so this covers the window between accept and request only.
	idleTimeout = 30 * time.Second
)

// listenFunc is the bind seam. Tests use it to bind loopback instead of every
// interface, and to force a bind failure without occupying a real port.
type listenFunc func(ctx context.Context, address string) (net.Listener, error)

// Server is the ephemeral one-shot HTTP server: one listener, one capability
// token, one authorized download, then nothing.
//
// It owns no session state of its own. The coordinator decides whether a claim
// may proceed, and this type's whole job is to make that decision the only way
// through: reserve atomically, authorize synchronously, and open a payload
// only after both have succeeded.
type Server struct {
	payloads PayloadPort
	listen   listenFunc
	now      clock

	mu     sync.Mutex
	active *run
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
		now: time.Now,
	}
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
		ctx:          dataCtx,
		cancel:       cancel,
		mux:          http.NewServeMux(),
		listener:     &onceCloseListener{Listener: listener},
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
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ConnState:         active.trackConnection,
		// net/http logs connection and panic diagnostics that can quote a
		// request. Nothing about this server's traffic is safe to print: the
		// path carries the capability token.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	// One request, one response, one connection. Disabling keep-alives means a
	// finished receiver's socket closes instead of idling against a listener
	// that is about to disappear.
	active.http.SetKeepAlivesEnabled(false)

	go active.serve()

	s.active = active
	return transfer.ServerHandle{Port: port, Events: active.lane.channel()}, nil
}

// Stop force-closes the server and returns only once it is quiescent: no
// listener, no connection, no handler, no payload worker, and no event
// producer is still live, and the event channel is closed for good. It is safe
// before Start, after a failed Start, and when repeated. A returned error is a
// cleanup diagnostic; it never means something is still running.
func (s *Server) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil
	}

	active := s.active
	s.active = nil
	return active.teardown()
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
// so a blocked write unblocks, wait for the handler -- and therefore for
// WriteTo and the payload Close it owns -- to finish, then close the lane.
// Reversing any pair of those steps risks waiting forever on a write that will
// never complete, or closing a payload while it is still being read.
func (r *run) teardown() error {
	r.teardownOnce.Do(func() {
		r.beginStop()
		r.cancel()

		closeErr := r.http.Close()
		<-r.serveDone
		r.handlers.Wait()
		r.awaitConnections()
		r.lane.close()

		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
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
