package transfer

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/netip"
	"slices"
	"sync"
	"time"
)

const (
	// identityBytes is the width of each independent random identifier. Two
	// separate draws of this many bytes give a session ID and a capability
	// token of 128 bits each, which is the contract's floor.
	identityBytes = 16

	// downloadPathPrefix must stay identical to the route internal/server
	// registers as "/download/{token}" in handler.go. The two cannot share a
	// constant without inverting the dependency direction -- the server
	// imports this package -- so a change to either one has to move both.
	downloadPathPrefix = "/download/"

	// beaconInstanceBase is the only instance text this coordinator supplies.
	// It is a fixed literal on purpose: the network adapter appends a host
	// label and a random per-process suffix, and nothing about the selected
	// item may reach a discovery record.
	beaconInstanceBase = "fairdrop"

	// maxDiagnostics bounds the internal cleanup record. A session produces a
	// handful at most, and the sink exists to be inspected, not to grow.
	maxDiagnostics = 32
)

// sessionState is the coordinator's lifecycle state. STAGING and CLAIMING are
// internal: they exist so a long setup or handshake stays interruptible, and
// the UI never sees them. DONE and ERROR are terminal holding states: the
// session's resources are already released there, and only the reset that
// clears the session still has to happen.
type sessionState string

const (
	stateIdle         sessionState = "IDLE"
	stateStaging      sessionState = "STAGING"
	stateStaged       sessionState = "STAGED"
	stateClaiming     sessionState = "CLAIMING"
	stateTransferring sessionState = "TRANSFERRING"
	stateDone         sessionState = "DONE"
	stateError        sessionState = "ERROR"
)

// resource names one thing a session acquires from an adapter. Stage appends
// each in acquisition order and unwind walks the list backwards, so reverse
// release is structural rather than a comment somebody has to keep true.
type resource int

const (
	resourceServer resource = iota
	resourceBeacon
)

// diagnostic is one internal cleanup note. It carries a stable code and a
// message this package chose, never adapter text: adapter text is exactly
// where absolute paths and capability tokens live.
type diagnostic struct {
	code    ErrorCode
	message string
}

type diagnosticSink struct {
	mu      sync.Mutex
	entries []diagnostic
}

func (s *diagnosticSink) record(entry diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) >= maxDiagnostics {
		return
	}
	s.entries = append(s.entries, entry)
}

func (s *diagnosticSink) snapshot() []diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]diagnostic, len(s.entries))
	copy(out, s.entries)
	return out
}

// session is everything one staged transfer owns.
//
// Field ownership splits three ways, and mixing them is what a race here would
// look like:
//
//   - id, token, generation, ctx and cancel are set before the session is
//     installed and never change, so any goroutine may read them.
//   - cancelled, terminal, stopReset, seq, stagedAt and startedAt are guarded
//     by Coordinator.mu.
//   - everything else belongs to whichever operation holds the lease. The
//     lease is a channel handoff, so it carries the happens-before edge that
//     lets the claim path read what Stage wrote.
type session struct {
	id         SessionID
	token      CapabilityToken
	generation uint64
	ctx        context.Context
	cancel     context.CancelFunc

	cancelled bool
	// terminal records that this session's one Complete or Failed outcome has
	// been accepted. It is set the moment the outcome is taken, before the
	// resources are released and the settled state is committed, so
	// exactly-once acceptance does not depend on where that transition lands.
	terminal bool
	// stopReset cancels the armed three-second reset. It is nil whenever no
	// reset is pending, so a Cancel that finds it nil has nothing to stop.
	stopReset StopTimer
	seq       uint64
	stagedAt  time.Time
	startedAt time.Time

	item     StagedItem
	url      string
	qrBase64 string
	warnings []Warning
	acquired []resource

	drainerDone chan struct{}
}

// hold records a resource as live, in acquisition order.
func (s *session) hold(held resource) {
	s.acquired = append(s.acquired, held)
}

// stop cancels the session's data-plane context. The caller must not hold the
// state mutex: cancelling runs whatever is waiting on the context, and the
// no-lock-across-a-call rule covers those continuations too.
func (s *session) stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// release forgets a resource the operation just gave up, so a later unwind
// does not try to release it twice.
func (s *session) release(freed resource) {
	kept := s.acquired[:0]
	for _, held := range s.acquired {
		if held != freed {
			kept = append(kept, held)
		}
	}
	s.acquired = kept
}

// StopTimer cancels a scheduled callback. Like time.Timer.Stop it reports
// whether it stopped the callback before it ran, and calling it after the
// callback has already run is safe.
type StopTimer func() bool

// Dependencies are the ports and test seams the coordinator composes. Entropy,
// Now and AfterFunc default to the process sources when omitted.
type Dependencies struct {
	Source   SourcePort
	Network  NetworkPort
	Server   ServerPort
	QR       QRPort
	Observer Observer
	Entropy  io.Reader
	Now      func() time.Time

	// AfterFunc schedules the terminal reset. It must behave like
	// time.AfterFunc in the one way the coordinator depends on: run must not
	// be invoked before AfterFunc returns. The reset is armed on the drainer
	// goroutine, and the callback joins that drainer, so a seam that called
	// back synchronously would make the drainer wait for itself.
	AfterFunc func(delay time.Duration, run func()) StopTimer
}

// Coordinator owns FairDrop's transfer lifecycle. It is framework-independent:
// it imports no adapter and no UI toolkit, and every side effect it has runs
// through an injected port.
//
// Two locks with different jobs guard it. mu protects state and session
// identity for microseconds at a time and is never held across a call into an
// adapter -- an mDNS registration or a blocked listener under that lock would
// stall a cancellation that has to be able to win at any moment. The operation
// lease serializes the long adapter work itself, so a cancellation joins the
// teardown already in flight instead of racing a second one.
type Coordinator struct {
	source    SourcePort
	network   NetworkPort
	server    ServerPort
	qr        QRPort
	observer  Observer
	entropy   io.Reader
	now       func() time.Time
	afterFunc func(delay time.Duration, run func()) StopTimer

	// lease holds exactly one token. Whoever receives it may call adapter
	// Start/Stop/unwind methods; nobody else may.
	lease chan struct{}

	mu         sync.Mutex
	state      sessionState
	generation uint64
	closing    bool
	session    *session

	diagnostics diagnosticSink
}

var _ ClaimAuthorizer = (*Coordinator)(nil)

// NewCoordinator returns an idle coordinator wired to the given ports.
func NewCoordinator(deps Dependencies) *Coordinator {
	lease := make(chan struct{}, 1)
	lease <- struct{}{}

	entropy := deps.Entropy
	if entropy == nil {
		entropy = rand.Reader
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	afterFunc := deps.AfterFunc
	if afterFunc == nil {
		afterFunc = func(delay time.Duration, run func()) StopTimer {
			return time.AfterFunc(delay, run).Stop
		}
	}

	return &Coordinator{
		source:    deps.Source,
		network:   deps.Network,
		server:    deps.Server,
		qr:        deps.QR,
		observer:  deps.Observer,
		entropy:   entropy,
		now:       now,
		afterFunc: afterFunc,
		lease:     lease,
		state:     stateIdle,
	}
}

// Stage validates one selected path, acquires every resource the transfer
// needs, and commits STAGED only when all of them are live. Any failure or
// cancellation unwinds in reverse acquisition order, returns to IDLE, emits no
// lifecycle event, and reports the coded cause instead of metadata.
func (c *Coordinator) Stage(ctx context.Context, absolutePath string) (FileMetadata, error) {
	if ctx == nil {
		return FileMetadata{}, NewError(ErrTransferFailed, "staging requires a context")
	}
	if err := c.ready(); err != nil {
		return FileMetadata{}, err
	}

	// Identity is drawn before any state changes hands. A CSPRNG failure then
	// costs nothing: no resource was acquired and the coordinator never left
	// IDLE, which is exactly what that failure has to look like.
	id, token, err := c.newIdentity()
	if err != nil {
		return FileMetadata{}, err
	}

	c.mu.Lock()
	if c.closing {
		c.mu.Unlock()
		return FileMetadata{}, NewError(ErrShuttingDown, "FairDrop is closing")
	}
	if c.state != stateIdle {
		c.mu.Unlock()
		return FileMetadata{}, NewError(ErrBusy, "a transfer is already in progress")
	}
	if !c.acquireLease() {
		// IDLE while the lease is still held means the previous session's
		// teardown has not finished. The refusal is the same one, and it
		// changes no state and touches no resource.
		c.mu.Unlock()
		return FileMetadata{}, NewError(ErrBusy, "the previous transfer is still being released")
	}
	c.generation++
	generation := c.generation
	// The session context outlives this call: the listener started below is
	// still serving long after Stage returns, so it cannot hang off the
	// caller's command context.
	sessionCtx, sessionCancel := context.WithCancel(context.WithoutCancel(ctx))
	live := &session{
		id:         id,
		token:      token,
		generation: generation,
		ctx:        sessionCtx,
		cancel:     sessionCancel,
		warnings:   make([]Warning, 0, 1),
	}
	c.state = stateStaging
	c.session = live
	c.mu.Unlock()

	// Setup calls hang off a context of their own, so an abandoned Stage stops
	// them without cancelling the session a successful commit keeps.
	setupCtx, stopSetup := context.WithCancel(sessionCtx)
	defer stopSetup()
	stopCallerWatch := context.AfterFunc(ctx, stopSetup)
	defer stopCallerWatch()

	// 1. Inspect the selection. Nothing on the network is touched until the
	//    source has proven itself.
	item, err := c.source.Inspect(setupCtx, absolutePath)
	if err != nil {
		return c.failStage(live, err)
	}
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failStage(live, err)
	}
	// JavaScript numbers can represent integers exactly only through 2^53-1.
	// Refuse invalid metadata before any network, server, QR, or beacon resource
	// is acquired, regardless of whether the source is a file or directory.
	const maxSafeInteger int64 = 9007199254740991
	if item.LogicalSize < 0 || item.LogicalSize > maxSafeInteger {
		return c.failStage(live, NewError(ErrTransferFailed, "selection logical size cannot be represented safely"))
	}
	if item.Kind != ItemFile && item.Kind != ItemDirectory {
		return c.failStage(live, NewError(ErrTransferFailed, "selection kind is unsupported"))
	}
	live.item = item

	// 2. Resolve the address the receiver will dial.
	address, err := c.network.GetLocalIP(setupCtx)
	if err != nil {
		return c.failStage(live, err)
	}
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failStage(live, err)
	}
	address = address.Unmap()
	if !address.IsValid() || address.IsUnspecified() {
		return c.failStage(live, NewError(ErrNetworkUnavailable, "no usable local network address was selected"))
	}

	// 3. Start the server and its drainer together. A started server whose
	//    event lane nobody reads could block its own teardown, so the reader
	//    exists from the moment the listener does.
	handle, err := c.server.Start(sessionCtx, ServerStartRequest{SessionID: id, Token: token, Item: item}, c)
	if err != nil {
		return c.failStage(live, err)
	}
	if handle.Events == nil || handle.Port < 1 || handle.Port > 65535 {
		// Refusing a handle still means owning it: Stop is safe after any
		// Start, and leaving it be would strand a listener.
		c.stopServer()
		return c.failStage(live, NewError(ErrServerStartFailed, "the transfer server did not report a usable listener"))
	}
	live.drainerDone = make(chan struct{})
	go c.drain(live, handle.Events)
	live.hold(resourceServer)
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failStage(live, err)
	}

	// 4. Build the capability URL. This is the first value that carries the
	//    token, and it goes only here and into the QR.
	live.url = capabilityURL(address, handle.Port, token)

	// 5. Encode the QR.
	png, err := c.qr.EncodePNG(setupCtx, live.url)
	if err != nil {
		return c.failStage(live, err)
	}
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failStage(live, err)
	}
	if len(png) == 0 {
		return c.failStage(live, NewError(ErrQRFailed, "the capability code encoder returned no image"))
	}
	// Standard padded base64 of the PNG bytes with no data-URI prefix: the
	// prefix belongs to the renderer, and adding it here would make the value
	// wrong for every other consumer.
	live.qrBase64 = base64.StdEncoding.EncodeToString(png)

	// 6. Publish the beacon last, because it is the only resource whose
	//    failure is survivable. HTTP and QR ready with discovery down is a
	//    usable session with a warning; the reverse is not a session at all.
	beaconErr := c.network.StartBeacon(setupCtx, BeaconRequest{
		SessionID: id,
		Service:   BeaconService,
		Instance:  beaconInstanceBase,
		Port:      handle.Port,
		TXT:       []string{BeaconVersionTXT},
	})
	if beaconErr == nil {
		live.hold(resourceBeacon)
	}
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failStage(live, err)
	}
	if beaconErr != nil {
		// The adapter has already cleaned up its partial registration, so the
		// session simply records no beacon. The warning's code is fixed here
		// rather than taken from the adapter, which keeps adapter text out of
		// a value that reaches the UI.
		c.recordDiagnostic(beaconErr, "device discovery could not be published")
		live.warnings = append(live.warnings, beaconWarning())
	}

	stagedAt := c.now()

	c.mu.Lock()
	if err := c.revalidateLocked(setupCtx, id, generation, stateStaging); err != nil {
		c.mu.Unlock()
		return c.failStage(live, err)
	}
	live.stagedAt = stagedAt
	c.state = stateStaged
	warnings := make([]Warning, len(live.warnings))
	copy(warnings, live.warnings)
	metadata := FileMetadata{
		SessionID: id,
		Name:      item.Name,
		Size:      item.LogicalSize,
		IsDir:     item.Kind == ItemDirectory,
		URL:       live.url,
		QR:        live.qrBase64,
		Warnings:  warnings,
	}
	// The lease is handed back inside the same critical section that commits
	// STAGED, so a claim that observes STAGED can never find this Stage still
	// holding it.
	c.releaseLease()
	c.mu.Unlock()

	return metadata, nil
}

// AuthorizeClaim is the synchronous handshake between a reserved HTTP claim
// and the coordinator. It runs on the serving goroutine and returns only after
// the transfer is committed or refused; the server opens no payload and writes
// no header until it succeeds.
func (c *Coordinator) AuthorizeClaim(ctx context.Context, sessionID SessionID) error {
	if ctx == nil {
		return NewError(ErrCancelled, "claim authorization requires a context")
	}
	if err := c.ready(); err != nil {
		return err
	}

	c.mu.Lock()
	if err := c.revalidateLocked(ctx, sessionID, 0, stateStaged); err != nil {
		c.mu.Unlock()
		return err
	}
	if !c.acquireLease() {
		// Someone else owns this session's adapters, which can only be a
		// teardown. The claim has lost, and it never waits: the teardown it
		// would wait for is the one stopping this very server.
		c.mu.Unlock()
		return NewError(ErrCancelled, "the transfer was cancelled")
	}
	live := c.session
	generation := live.generation
	c.state = stateClaiming
	c.mu.Unlock()

	// Stop the beacon before committing, without the mutex. The port
	// guarantees no advertisement remains on every return, so a diagnostic
	// here is a cleanup note and never evidence that the beacon is still up.
	// The call is unconditional because it is idempotent and safe before a
	// start: proving the advertisement is gone matters more than remembering
	// whether it was ever there.
	if err := c.network.StopBeacon(); err != nil {
		c.recordDiagnostic(err, "device discovery cleanup reported a problem")
	}
	live.release(resourceBeacon)

	startedAt := c.now()

	c.mu.Lock()
	if err := c.revalidateLocked(ctx, sessionID, generation, stateClaiming); err != nil {
		// Cancellation linearized first. Hand the lease back so the teardown
		// that owns this outcome can take it, and publish nothing at all.
		c.releaseLease()
		c.mu.Unlock()
		return err
	}
	live.startedAt = startedAt
	c.state = stateTransferring
	live.seq++
	event := Event{SessionID: live.id, Seq: live.seq, Kind: TransferStarted}
	c.mu.Unlock()

	// Published while the lease is still held. Releasing at the commit instead
	// would leave a window where a reset could reach the UI ahead of the
	// started event for the transfer it terminates.
	c.publish(event)
	c.releaseLease()
	return nil
}

// failStage unwinds everything the attempt acquired, returns the coordinator
// to IDLE, and reports the cause. It emits no lifecycle event: nothing was
// acknowledged, so there is nothing for the UI to terminate.
func (c *Coordinator) failStage(live *session, cause error) (FileMetadata, error) {
	c.unwind(live)
	live.stop()

	c.mu.Lock()
	// Only the lease owner installs or clears a session, and this call still
	// owns the lease, so the live session should still be ours -- checked
	// rather than assumed. Clearing a session this call does not own would
	// deregister a replacement and force IDLE while its resources stay live,
	// which is exactly the invariant an assumption in a comment cannot hold.
	if c.session == live {
		c.session = nil
		c.state = stateIdle
	}
	c.releaseLease()
	c.mu.Unlock()

	return FileMetadata{}, cause
}

// unwind releases every live resource and then waits for the drainer to end.
// The caller owns the operation lease and must not hold the state mutex: Stop
// and StopBeacon are adapter calls like any other.
//
// Only an operation that is not itself the drainer may call this. Terminal
// handling runs on the drainer goroutine and uses releaseAcquired directly,
// because joining itself would be a guaranteed deadlock.
func (c *Coordinator) unwind(live *session) {
	c.releaseAcquired(live)
	c.joinDrainer(live)
}

// releaseAcquired releases every live resource in reverse acquisition order.
// The caller owns the operation lease and must not hold the state mutex.
func (c *Coordinator) releaseAcquired(live *session) {
	for index := len(live.acquired) - 1; index >= 0; index-- {
		switch live.acquired[index] {
		case resourceBeacon:
			if err := c.network.StopBeacon(); err != nil {
				c.recordDiagnostic(err, "device discovery cleanup reported a problem")
			}
		case resourceServer:
			c.stopServer()
		}
	}
	live.acquired = nil
}

// joinDrainer waits for this session's drainer goroutine to end, which is what
// keeps a session's goroutine from outliving the session. Calling it twice is
// safe, and so is calling it after the drainer has already gone: Stop closed
// the event lane, so the loop is on its way out, and a closed done channel
// receives forever.
//
// The wait is deliberately unbounded. ServerPort.Stop is quiescent on every
// return, so the lane is closed by the time this runs; a watchdog here would
// let Cancel report success while a drainer -- and therefore a publication --
// was still in flight.
func (c *Coordinator) joinDrainer(live *session) {
	if live.drainerDone != nil {
		<-live.drainerDone
	}
}

func (c *Coordinator) stopServer() {
	if err := c.server.Stop(); err != nil {
		c.recordDiagnostic(err, "transfer server cleanup reported a problem")
	}
}

// afterStep reacquires the state mutex and revalidates the operation after an
// unlocked adapter call. Every external call is followed by exactly one of
// these, which is what makes committing a stale result impossible rather than
// merely unlikely.
// afterStep revalidates against BOTH contexts, and the caller's is checked
// first. context.AfterFunc runs stopSetup on a new goroutine, so between the
// caller cancelling and that goroutine running, setupCtx.Err() is still nil --
// a window in which a step could pass revalidation and the whole Stage could
// commit for a command the user already abandoned. The caller's context is
// immediate; the derived one is only eventually consistent with it.
func (c *Coordinator) afterStep(
	callerCtx context.Context,
	setupCtx context.Context,
	id SessionID,
	generation uint64,
) error {
	if err := callerCtx.Err(); err != nil {
		return WrapError(ErrCancelled, "the transfer was cancelled", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revalidateLocked(setupCtx, id, generation, stateStaging)
}

// revalidateLocked answers one question: may this operation still use what it
// just got back? A zero generation skips the generation check, because the
// first session is generation 1 and a claim arrives without one.
//
// One or more acceptable states may be named, because the reset timer is valid
// from either terminal state. Naming none refuses everything rather than
// meaning "any state will do": a zero-value "skip this check" convention is
// exactly how an empty session id once became a wildcard.
//
// The caller holds c.mu.
func (c *Coordinator) revalidateLocked(ctx context.Context, id SessionID, generation uint64, want ...sessionState) error {
	if c.closing {
		return NewError(ErrShuttingDown, "FairDrop is closing")
	}
	if c.session == nil {
		return NewError(ErrCancelled, "the transfer is no longer active")
	}
	// An empty id is refused rather than skipped. Every call site passes a real
	// session id, so an empty one is a caller defect -- and ClaimAuthorizer is
	// a public interface, so treating "no id given" as "matches whatever is
	// staged" would let a wrong caller authorize a session it cannot name.
	if id == "" || c.session.id != id {
		return NewError(ErrCancelled, "the transfer is no longer active")
	}
	if generation != 0 && c.session.generation != generation {
		return NewError(ErrCancelled, "the transfer was replaced")
	}
	if c.session.cancelled {
		return NewError(ErrCancelled, "the transfer was cancelled")
	}
	if !slices.Contains(want, c.state) {
		return NewError(ErrCancelled, "the transfer is no longer active")
	}
	if err := ctx.Err(); err != nil {
		return WrapError(ErrCancelled, "the transfer was cancelled", err)
	}
	return nil
}

// markCancelledLocked flags the live session's generation as cancelled and
// returns it, or nil when nothing is staged. It is the marker every setup and
// claim step revalidates against, and it is deliberately only half of a
// cancellation: the caller still has to cancel the returned session's context
// (outside the mutex) and then join its teardown through the lease.
//
// The caller holds c.mu.
func (c *Coordinator) markCancelledLocked() *session {
	live := c.session
	if live == nil {
		return nil
	}
	live.cancelled = true
	return live
}

// beginClosing raises the application-lifetime closing flag and marks any live
// session cancelled. The flag is what refuses every later command; the marker
// is what stops a setup or claim already in flight from committing.
func (c *Coordinator) beginClosing() *session {
	c.mu.Lock()
	c.closing = true
	live := c.markCancelledLocked()
	c.mu.Unlock()

	if live != nil {
		live.stop()
	}
	return live
}

// acquireLease takes the operation lease without blocking. Callers hold c.mu,
// so the decision to proceed and the state transition that records it are one
// atomic step.
func (c *Coordinator) acquireLease() bool {
	select {
	case <-c.lease:
		return true
	default:
		return false
	}
}

// releaseLease hands the operation lease back. The send cannot block: the
// lease holds one token and only its owner returns it.
// releaseLease hands the single token back. The send cannot block when the
// caller genuinely owns the lease, so a full channel means two callers believe
// they do -- a bug that must be loud rather than absorbed. Panicking here is
// the honest response: continuing would let two operations drive one session's
// adapters concurrently, which is the exact thing the lease exists to prevent.
func (c *Coordinator) releaseLease() {
	select {
	case c.lease <- struct{}{}:
	default:
		panic("transfer: operation lease released twice; two callers own one session")
	}
}

// leaseHeld reports whether some operation currently owns the lease.
func (c *Coordinator) leaseHeld() bool {
	return len(c.lease) == 0
}

// publish delivers one event on the coordinator's single emission lane.
//
// Only the holder of the operation lease may publish, so emission order
// follows causality rather than goroutine scheduling: a progress snapshot
// cannot overtake the started event for the transfer it belongs to, and reset
// cannot precede the outcome it terminates. Publishing without the lease is a
// programming error of the same class as releasing the lease twice, and is
// just as loud -- absorbing it would let the UI observe an order the contract
// forbids.
//
// The check is deliberately the weaker of the two available: it proves some
// operation holds the lease, not that this caller is the one holding it. Go
// offers no cheap goroutine identity, and the failure it does catch -- a
// publication from a path that never took the lease at all -- is the one that
// actually happens.
func (c *Coordinator) publish(event Event) {
	if !c.leaseHeld() {
		panic("transfer: an event was published without the operation lease")
	}
	if c.observer == nil {
		return
	}
	c.observer.Publish(event)
}

func (c *Coordinator) recordDiagnostic(cause error, message string) {
	c.diagnostics.record(diagnostic{code: ErrorCodeOf(cause), message: message})
}

// ready reports whether every port the coordinator needs was injected.
func (c *Coordinator) ready() error {
	if c == nil || c.source == nil || c.network == nil || c.server == nil || c.qr == nil || c.observer == nil {
		return NewError(ErrTransferFailed, "FairDrop is not ready to stage a transfer")
	}
	return nil
}

// newIdentity draws the session ID and the capability token as two independent
// values. Neither is derived from the other: the token is an HTTP capability
// and the session ID is correlation the UI is shown, so learning either one
// must teach nothing about the other.
func (c *Coordinator) newIdentity() (SessionID, CapabilityToken, error) {
	id, err := c.randomHex()
	if err != nil {
		return "", "", err
	}
	token, err := c.randomHex()
	if err != nil {
		return "", "", err
	}
	return SessionID(id), CapabilityToken(token), nil
}

func (c *Coordinator) randomHex() (string, error) {
	source := c.entropy
	if source == nil {
		source = rand.Reader
	}
	raw := make([]byte, identityBytes)
	if _, err := io.ReadFull(source, raw); err != nil {
		// No stable code describes an exhausted CSPRNG, and the contract maps
		// everything it does not recognize to the transfer failure fallback.
		return "", WrapError(ErrTransferFailed, "FairDrop could not create a transfer session", err)
	}
	return hex.EncodeToString(raw), nil
}

// capabilityURL is the one place the token becomes a shareable string.
func capabilityURL(address netip.Addr, port int, token CapabilityToken) string {
	endpoint := netip.AddrPortFrom(address, uint16(port))
	return "http://" + endpoint.String() + downloadPathPrefix + string(token)
}

// beaconWarning is the fixed non-fatal warning for a discovery failure. Its
// copy comes from the public registry rather than from the adapter, so no
// adapter text can reach the UI through it.
func beaconWarning() Warning {
	public := PublicErrorOf(NewError(ErrBeaconWarning, "device discovery is unavailable"))
	return Warning{Code: public.Code, Message: public.Message}
}
