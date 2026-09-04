package transfer

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"
)

const (
	// testPath is deliberately a full absolute path with a space in it: every
	// disclosure assertion searches for this exact string, so a leak anywhere
	// is detected rather than argued about.
	testPath = `C:\Users\sender\Documents\quarterly report.pdf`
	testName = "quarterly report.pdf"
	testSize = int64(4096)

	testAddress = "192.168.1.50"
	testPort    = 45678

	// testSessionID and testToken are what the deterministic entropy below
	// produces: the first sixteen bytes and the second sixteen bytes.
	testSessionID = SessionID("0102030405060708090a0b0c0d0e0f10")
	testToken     = CapabilityToken("1112131415161718191a1b1c1d1e1f20")

	// Spelled out rather than built from downloadPathPrefix. Deriving it from
	// the constant under test made the URL assertion self-referential: the
	// path could be changed to anything and both sides moved together, so a
	// capability link pointing at a route the server answers with 404 would
	// have shipped green.
	testURL = "http://" + testAddress + ":45678" + "/download/" + string(testToken)

	// mutexProbeTimeout bounds the entry check every fake performs. A state
	// mutex held legitimately by another goroutine is released in
	// microseconds; one held across an adapter call is held for the whole
	// call, so only a real violation can outlast this.
	mutexProbeTimeout = 2 * time.Second
)

// testPNG stands in for encoded QR bytes. Its exact value matters: the
// coordinator must base64 it unchanged.
var testPNG = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01, 0x02}

func testItem() StagedItem {
	return StagedItem{
		Path:        testPath,
		Name:        testName,
		Kind:        ItemFile,
		LogicalSize: testSize,
		ModTime:     time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
	}
}

func testAddr() netip.Addr {
	return netip.MustParseAddr(testAddress)
}

// recorder is the single ordered call log every fake writes to. Acquisition
// order, unwind order, and "this adapter was never reached" are all read out
// of it.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// teardownCalls returns only the release calls, in the order they happened.
// That is the sequence the reverse-unwind rule is about.
func (r *recorder) teardownCalls() []string {
	var out []string
	for _, call := range r.snapshot() {
		if call == "server.Stop" || call == "network.StopBeacon" {
			out = append(out, call)
		}
	}
	return out
}

func (r *recorder) count(name string) int {
	total := 0
	for _, call := range r.snapshot() {
		if call == name {
			total++
		}
	}
	return total
}

// harness owns the coordinator under test and every fake wired into it.
type harness struct {
	t     *testing.T
	calls *recorder

	// seen records every session the harness ever staged, so teardown can join
	// a drainer belonging to a session that has since been cleared. close used
	// to read only the live session, which is nil after every reset and every
	// Cancel -- exactly the cases where a leaked drainer would hide.
	seen []*session

	coordinator *Coordinator

	source   *fakeSource
	network  *fakeNetwork
	server   *fakeServer
	qr       *fakeQR
	observer *fakeObserver
	entropy  *fakeEntropy
	clock    *fakeClock
	timer    *fakeTimer
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{t: t, calls: &recorder{}}
	h.source = &fakeSource{h: h}
	h.network = &fakeNetwork{h: h}
	h.server = &fakeServer{h: h, events: make(chan ServerEvent)}
	h.qr = &fakeQR{h: h}
	h.observer = &fakeObserver{h: h}
	h.entropy = &fakeEntropy{h: h}
	h.clock = &fakeClock{h: h, current: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC), step: time.Second}
	h.timer = &fakeTimer{h: h}

	h.coordinator = NewCoordinator(Dependencies{
		Source:    h.source,
		Network:   h.network,
		Server:    h.server,
		QR:        h.qr,
		Observer:  h.observer,
		Entropy:   h.entropy,
		Now:       h.clock.Now,
		AfterFunc: h.timer.afterFunc,
	})

	t.Cleanup(h.close)
	return h
}

// close releases whatever a test left staged, so no drainer or session context
// outlives the test that created it.
func (h *harness) close() {
	h.server.closeEvents()

	h.coordinator.mu.Lock()
	live := h.coordinator.session
	h.coordinator.mu.Unlock()
	if live != nil {
		h.track(live)
	}
	for _, session := range h.seen {
		if session.drainerDone != nil {
			<-session.drainerDone
		}
		if session.cancel != nil {
			session.cancel()
		}
	}
}

// track remembers a session so close can join its drainer later.
func (h *harness) track(live *session) {
	for _, known := range h.seen {
		if known == live {
			return
		}
	}
	h.seen = append(h.seen, live)
}

// enter is the gate every fake passes through. It proves the no-lock-across-an
// -adapter-call rule executably: if the coordinator ever calls out while
// holding its state mutex, the call that does it fails the test by name.
func (h *harness) enter(name string) {
	h.t.Helper()
	h.assertMutexUnheld(name)
	h.calls.add(name)
}

func (h *harness) assertMutexUnheld(name string) {
	h.t.Helper()
	coordinator := h.coordinator
	if coordinator == nil {
		return
	}
	deadline := time.Now().Add(mutexProbeTimeout)
	for {
		if coordinator.mu.TryLock() {
			coordinator.mu.Unlock()
			return
		}
		if time.Now().After(deadline) {
			h.t.Errorf("%s was called while the coordinator held its state mutex", name)
			return
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// stage runs the coordinator's Stage with a background context.
func (h *harness) stage() (FileMetadata, error) {
	return h.coordinator.Stage(context.Background(), testPath)
}

// stageWithContext runs Stage with a caller-supplied context, which is the only
// way to exercise the two deliberate context decisions Stage makes: the session
// context is detached from the caller, and an abandoned caller aborts setup.
func (h *harness) stageWithContext(ctx context.Context) (FileMetadata, error) {
	return h.coordinator.Stage(ctx, testPath)
}

// serverStartContext returns the context the coordinator handed ServerPort.Start.
func (f *fakeServer) serverStartContext() context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCtx
}

// stageSuccessfully runs Stage and fails the test unless it commits.
func (h *harness) stageSuccessfully() FileMetadata {
	h.t.Helper()
	metadata, err := h.stage()
	if err != nil {
		h.t.Fatalf("Stage returned %v, want a committed session", err)
	}
	if live := h.liveSession(); live != nil {
		h.track(live)
	}
	return metadata
}

func (h *harness) state() sessionState {
	h.coordinator.mu.Lock()
	defer h.coordinator.mu.Unlock()
	return h.coordinator.state
}

func (h *harness) liveSession() *session {
	h.coordinator.mu.Lock()
	defer h.coordinator.mu.Unlock()
	return h.coordinator.session
}

type fakeSource struct {
	h       *harness
	inspect func(ctx context.Context, absolutePath string) (StagedItem, error)
	walk    func(ctx context.Context, absolutePath string, visit SourceVisitor) error

	mu    sync.Mutex
	paths []string
	walks []string
}

func (f *fakeSource) Inspect(ctx context.Context, absolutePath string) (StagedItem, error) {
	f.h.enter("source.Inspect")
	f.mu.Lock()
	f.paths = append(f.paths, absolutePath)
	f.mu.Unlock()
	if f.inspect != nil {
		return f.inspect(ctx, absolutePath)
	}
	return testItem(), nil
}

// Walk exists because the coordinator holds a SourcePort, not because the
// coordinator calls it: staging never walks a tree. A call arriving here is
// itself the finding, so the default fails rather than quietly succeeding.
func (f *fakeSource) Walk(ctx context.Context, absolutePath string, visit SourceVisitor) error {
	f.h.enter("source.Walk")
	f.mu.Lock()
	f.walks = append(f.walks, absolutePath)
	f.mu.Unlock()
	if f.walk != nil {
		return f.walk(ctx, absolutePath, visit)
	}
	return NewError(ErrTransferFailed, "coordinator must not walk a source tree")
}

func (f *fakeSource) walked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.walks))
	copy(out, f.walks)
	return out
}

func (f *fakeSource) inspected() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.paths))
	copy(out, f.paths)
	return out
}

type fakeNetwork struct {
	h           *harness
	getLocalIP  func(ctx context.Context) (netip.Addr, error)
	startBeacon func(ctx context.Context, request BeaconRequest) error
	stopBeacon  func() error

	mu       sync.Mutex
	requests []BeaconRequest
}

func (f *fakeNetwork) GetLocalIP(ctx context.Context) (netip.Addr, error) {
	f.h.enter("network.GetLocalIP")
	if f.getLocalIP != nil {
		return f.getLocalIP(ctx)
	}
	return testAddr(), nil
}

func (f *fakeNetwork) StartBeacon(ctx context.Context, request BeaconRequest) error {
	f.h.enter("network.StartBeacon")
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	if f.startBeacon != nil {
		return f.startBeacon(ctx, request)
	}
	return nil
}

func (f *fakeNetwork) StopBeacon() error {
	f.h.enter("network.StopBeacon")
	if f.stopBeacon != nil {
		return f.stopBeacon()
	}
	return nil
}

func (f *fakeNetwork) beaconRequests() []BeaconRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]BeaconRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

type fakeServer struct {
	h     *harness
	start func(ctx context.Context, request ServerStartRequest, authorizer ClaimAuthorizer) (ServerHandle, error)
	stop  func() error

	events chan ServerEvent

	mu         sync.Mutex
	closed     bool
	requests   []ServerStartRequest
	authorizer ClaimAuthorizer
	// startCtx is the context the coordinator hands the server. It outlives
	// Stage by design, so a test can assert it is NOT cancelled when the
	// caller's command context ends.
	startCtx context.Context
}

func (f *fakeServer) Start(
	ctx context.Context,
	request ServerStartRequest,
	authorizer ClaimAuthorizer,
) (ServerHandle, error) {
	f.h.enter("server.Start")
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.authorizer = authorizer
	f.startCtx = ctx
	f.mu.Unlock()
	if f.start != nil {
		return f.start(ctx, request, authorizer)
	}
	return ServerHandle{Port: testPort, Events: f.events}, nil
}

func (f *fakeServer) Stop() error {
	f.h.enter("server.Stop")
	var err error
	if f.stop != nil {
		err = f.stop()
	}
	// Closed last, which is the order the real port tears down in: handlers and
	// producers end first and lane closure is the final step. The order is
	// load-bearing for tests, because it leaves the stop hook as the one moment
	// when an outcome has been accepted and the lane is still open.
	f.closeEvents()
	return err
}

// closeEvents closes the lane exactly once, under the same mutex publish uses,
// so a producer racing a Stop can never send on a closed channel -- which is
// the property the real eventLane has and the reason it holds a mutex too.
func (f *fakeServer) closeEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	close(f.events)
}

// publish is the non-blocking producer, modelled on the real event lane: it
// never blocks and never sends after close, so a test may race it against a
// Stop the way a live handler does. It needs a buffered lane to land anything,
// so callers pair it with bufferLane; emit is the deterministic alternative.
func (f *fakeServer) publish(event ServerEvent) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return false
	}
	select {
	case f.events <- event:
		return true
	default:
		return false
	}
}

func (f *fakeServer) startRequests() []ServerStartRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ServerStartRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

func (f *fakeServer) claimAuthorizer() ClaimAuthorizer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authorizer
}

type fakeQR struct {
	h      *harness
	encode func(ctx context.Context, content string) ([]byte, error)

	mu       sync.Mutex
	contents []string
}

func (f *fakeQR) EncodePNG(ctx context.Context, content string) ([]byte, error) {
	f.h.enter("qr.EncodePNG")
	f.mu.Lock()
	f.contents = append(f.contents, content)
	f.mu.Unlock()
	if f.encode != nil {
		return f.encode(ctx, content)
	}
	return append([]byte(nil), testPNG...), nil
}

func (f *fakeQR) encoded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.contents))
	copy(out, f.contents)
	return out
}

type fakeObserver struct {
	h       *harness
	publish func(event Event)

	mu         sync.Mutex
	events     []Event
	underLease []bool
}

func (f *fakeObserver) Publish(event Event) {
	f.h.enter("observer.Publish")
	// The lease must still be held here: that ordering is the whole reason
	// authorization does not release at the TRANSFERRING commit.
	held := f.h.coordinator.leaseHeld()
	f.mu.Lock()
	f.events = append(f.events, event)
	f.underLease = append(f.underLease, held)
	f.mu.Unlock()
	if f.publish != nil {
		f.publish(event)
	}
}

func (f *fakeObserver) published() []Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Event, len(f.events))
	copy(out, f.events)
	return out
}

func (f *fakeObserver) leaseHeldAt(index int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.underLease) {
		return false
	}
	return f.underLease[index]
}

// fakeEntropy is a deterministic, never-repeating byte source: every read
// continues the counter, so two draws can never coincidentally match and a
// test can prove the two identifiers came from separate reads.
type fakeEntropy struct {
	h *harness

	mu        sync.Mutex
	next      byte
	reads     [][]byte
	failAfter int
	failure   error
}

func (f *fakeEntropy) Read(p []byte) (int, error) {
	f.h.enter("entropy.Read")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failure != nil && len(f.reads) >= f.failAfter {
		return 0, f.failure
	}
	for index := range p {
		f.next++
		p[index] = f.next
	}
	f.reads = append(f.reads, append([]byte(nil), p...))
	return len(p), nil
}

func (f *fakeEntropy) draws() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.reads))
	copy(out, f.reads)
	return out
}

type fakeClock struct {
	h *harness

	mu      sync.Mutex
	current time.Time
	step    time.Duration
	calls   int
}

func (f *fakeClock) Now() time.Time {
	f.h.enter("clock.Now")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.current = f.current.Add(f.step)
	return f.current
}

func (f *fakeClock) reads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// bufferLane replaces the server's event lane with a buffered one, the shape
// the real lane has. Tests that need an event to sit queued while something
// else happens -- a second terminal event, or a producer racing a Stop -- call
// it before Stage, which is when the lane is handed over.
func (h *harness) bufferLane(depth int) {
	h.t.Helper()
	if h.liveSession() != nil {
		h.t.Fatal("bufferLane must be called before Stage hands the lane to the coordinator")
	}
	h.server.events = make(chan ServerEvent, depth)
}

// emit hands one event to the drainer and blocks until it is taken. On an
// unbuffered lane that makes the drainer's processing observable: a second
// emit can only be received once the first has been fully handled, which is
// how a test proves a dropped or refused event really was processed.
func (h *harness) emit(event ServerEvent) {
	h.t.Helper()
	// A lane closed underneath this send means a teardown ran that the test did
	// not expect, which is a finding rather than a crash: report it by name.
	defer func() {
		if recovered := recover(); recovered != nil {
			h.t.Fatalf("the lane closed before %v could be delivered: %v", event.Kind, recovered)
		}
	}()
	select {
	case h.server.events <- event:
	case <-time.After(mutexProbeTimeout):
		h.t.Fatalf("the drainer never took %v", event.Kind)
	}
}

// transferring stages and claims, leaving the coordinator in TRANSFERRING with
// exactly the started event published.
func (h *harness) transferring() FileMetadata {
	h.t.Helper()
	metadata := h.stageSuccessfully()
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		h.t.Fatalf("AuthorizeClaim returned %v, want a committed transfer", err)
	}
	return metadata
}

// awaitEvents waits for at least want published events and returns them.
// Publication happens on the drainer or a timer goroutine, so a test cannot
// simply read the observer and expect the event to have arrived.
func (h *harness) awaitEvents(want int) []Event {
	h.t.Helper()
	deadline := time.Now().Add(mutexProbeTimeout)
	for {
		events := h.observer.published()
		if len(events) >= want {
			return events
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("published %d events, want at least %d: %+v", len(events), want, events)
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// awaitCancelled blocks until the live session carries the cancellation
// marker. It is what turns "Cancel probably got there first" into a forced
// race outcome: a fake calls it mid-step, so the step that follows is
// guaranteed to run against a cancelled generation.
func (h *harness) awaitCancelled() {
	h.t.Helper()
	deadline := time.Now().Add(mutexProbeTimeout)
	for {
		h.coordinator.mu.Lock()
		live := h.coordinator.session
		marked := live != nil && live.cancelled
		h.coordinator.mu.Unlock()
		if marked {
			return
		}
		if time.Now().After(deadline) {
			h.t.Error("no cancellation was marked before the deadline")
			return
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// awaitDrainer waits for the live session's drainer goroutine to end, which is
// the join a terminal outcome deliberately cannot perform on itself.
func (h *harness) awaitDrainer() {
	h.t.Helper()
	live := h.liveSession()
	if live == nil || live.drainerDone == nil {
		return
	}
	select {
	case <-live.drainerDone:
	case <-time.After(mutexProbeTimeout):
		h.t.Fatal("the drainer did not finish")
	}
}

// testProgress is the snapshot shape the fakes report. The percentages are
// written out at each call site rather than computed here, so a test pins a
// value instead of repeating the formula under test.
func testProgress(sent int64, percent float64) ProgressSnapshot {
	return ProgressSnapshot{
		BytesSent:        sent,
		TotalBytes:       testSize,
		TotalKnown:       true,
		Percent:          percent,
		SpeedBytesPerSec: 1024,
	}
}

func progressEvent(id SessionID, snapshot ProgressSnapshot) ServerEvent {
	return ServerEvent{SessionID: id, Kind: ServerProgress, Progress: &snapshot}
}

func completeEvent(id SessionID, snapshot ProgressSnapshot) ServerEvent {
	return ServerEvent{SessionID: id, Kind: ServerComplete, Progress: &snapshot}
}

func failedEvent(id SessionID, snapshot *ProgressSnapshot, cause error) ServerEvent {
	return ServerEvent{SessionID: id, Kind: ServerFailed, Progress: snapshot, Err: cause}
}

// fakeTimer is the injected reset scheduler. It never runs a callback on its
// own: a test fires it explicitly, which is what makes the timer-versus-Cancel
// race forceable in both directions instead of merely likely.
type fakeTimer struct {
	h *harness

	// withoutStop makes afterFunc schedule and then hand back no stop
	// function, which is the one shape that can turn armReset's cancellation
	// path into a nil call on the drainer goroutine.
	withoutStop bool

	mu        sync.Mutex
	scheduled []*scheduledCall
}

// scheduledCall is one armed reset.
type scheduledCall struct {
	delay time.Duration
	run   func()

	mu      sync.Mutex
	stopped bool
	fired   bool
}

func (f *fakeTimer) afterFunc(delay time.Duration, run func()) StopTimer {
	f.h.enter("timer.AfterFunc")
	call := &scheduledCall{delay: delay, run: run}
	f.mu.Lock()
	f.scheduled = append(f.scheduled, call)
	f.mu.Unlock()
	if f.withoutStop {
		return nil
	}
	return call.stop
}

func (c *scheduledCall) stop() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fired || c.stopped {
		return false
	}
	c.stopped = true
	return true
}

func (c *scheduledCall) wasStopped() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

func (f *fakeTimer) calls() []*scheduledCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*scheduledCall, len(f.scheduled))
	copy(out, f.scheduled)
	return out
}

func (f *fakeTimer) armed() int {
	return len(f.calls())
}

func (f *fakeTimer) stops() int {
	total := 0
	for _, call := range f.calls() {
		if call.wasStopped() {
			total++
		}
	}
	return total
}

// fire runs the most recently armed callback, whether or not it was stopped.
// A timer that has already fired cannot be un-fired, only outrun, so driving a
// stopped one is exactly how the stale-reset guard has to be tested: the
// callback must decide for itself that it lost.
func (f *fakeTimer) fire() {
	f.h.t.Helper()
	if !f.fireIfArmed() {
		f.h.t.Fatal("no reset was armed to fire")
	}
}

func (f *fakeTimer) fireIfArmed() bool {
	calls := f.calls()
	if len(calls) == 0 {
		return false
	}
	call := calls[len(calls)-1]
	call.mu.Lock()
	if call.fired {
		call.mu.Unlock()
		return false
	}
	call.fired = true
	call.mu.Unlock()

	call.run()
	return true
}

// blockUntilCancelled makes the QR seam hold its step open until the context it
// was handed is cancelled, and reports which happened first.
//
// It is how a test distinguishes Cancel *interrupting* a setup step from Cancel
// merely waiting for one to finish on its own. Every other fake returns
// immediately, so without this the two are indistinguishable and the session
// context cancellation in Cancel is unproven.
func (h *harness) blockQRUntilCancelled() <-chan error {
	released := make(chan error, 1)
	abort := make(chan struct{})
	h.t.Cleanup(func() { close(abort) })

	h.qr.encode = func(ctx context.Context, content string) ([]byte, error) {
		select {
		case <-ctx.Done():
			released <- ctx.Err()
		case <-abort:
			// The test is tearing down without the context ever being
			// cancelled, which is the failure this helper exists to catch.
			released <- nil
		}
		return nil, NewError(ErrCancelled, "the capability code encoder was cancelled")
	}
	return released
}

// cancelSession marks the live session cancelled and cancels its data-plane
// context, returning the session it marked.
//
// It lives here rather than in the coordinator because no production path
// wants it: Cancel and Shutdown mark and then join, and this is the
// interruption half without the join. Tests need exactly that -- a setup or
// claim step has to observe the marker mid-flight, not wait for a teardown it
// is racing.
func (c *Coordinator) cancelSession() *session {
	c.mu.Lock()
	live := c.markCancelledLocked()
	c.mu.Unlock()

	if live != nil {
		live.stop()
	}
	return live
}

// kindsOf reduces a published stream to its event kinds, which is what most
// grammar assertions actually compare.
func kindsOf(events []Event) []EventKind {
	kinds := make([]EventKind, len(events))
	for index, event := range events {
		kinds[index] = event.Kind
	}
	return kinds
}

// awaitClosing blocks until the application-lifetime closing flag is raised. It
// is how a test parks inside an adapter call until a Shutdown running on
// another goroutine has committed to closing, which is the only way to reach
// the window where an outcome already owns the lease Shutdown is waiting for.
func (h *harness) awaitClosing() {
	h.t.Helper()
	deadline := time.Now().Add(mutexProbeTimeout)
	for {
		h.coordinator.mu.Lock()
		closing := h.coordinator.closing
		h.coordinator.mu.Unlock()
		if closing {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatal("the closing flag was never raised")
		}
		time.Sleep(200 * time.Microsecond)
	}
}
