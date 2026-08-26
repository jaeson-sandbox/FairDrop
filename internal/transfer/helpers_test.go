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

	testURL = "http://" + testAddress + ":45678" + downloadPathPrefix + string(testToken)

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

	coordinator *Coordinator

	source   *fakeSource
	network  *fakeNetwork
	server   *fakeServer
	qr       *fakeQR
	observer *fakeObserver
	entropy  *fakeEntropy
	clock    *fakeClock
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

	h.coordinator = NewCoordinator(Dependencies{
		Source:   h.source,
		Network:  h.network,
		Server:   h.server,
		QR:       h.qr,
		Observer: h.observer,
		Entropy:  h.entropy,
		Now:      h.clock.Now,
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
	if live == nil {
		return
	}
	if live.drainerDone != nil {
		<-live.drainerDone
	}
	if live.cancel != nil {
		live.cancel()
	}
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

// stageSuccessfully runs Stage and fails the test unless it commits.
func (h *harness) stageSuccessfully() FileMetadata {
	h.t.Helper()
	metadata, err := h.stage()
	if err != nil {
		h.t.Fatalf("Stage returned %v, want a committed session", err)
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

	mu    sync.Mutex
	paths []string
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

	events    chan ServerEvent
	closeOnce sync.Once

	mu         sync.Mutex
	requests   []ServerStartRequest
	authorizer ClaimAuthorizer
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
	f.mu.Unlock()
	if f.start != nil {
		return f.start(ctx, request, authorizer)
	}
	return ServerHandle{Port: testPort, Events: f.events}, nil
}

func (f *fakeServer) Stop() error {
	f.h.enter("server.Stop")
	// The real port closes its event lane on every Stop return, which is what
	// lets the coordinator's drainer finish.
	f.closeEvents()
	if f.stop != nil {
		return f.stop()
	}
	return nil
}

func (f *fakeServer) closeEvents() {
	f.closeOnce.Do(func() { close(f.events) })
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
