package transfer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStageCommitsOnlyWhenEveryResourceIsLive(t *testing.T) {
	h := newHarness(t)

	metadata := h.stageSuccessfully()

	if metadata.SessionID != testSessionID {
		t.Errorf("session id is %q, want %q", metadata.SessionID, testSessionID)
	}
	if metadata.Name != testName {
		t.Errorf("name is %q, want %q", metadata.Name, testName)
	}
	if metadata.Size != testSize {
		t.Errorf("size is %d, want %d", metadata.Size, testSize)
	}
	if metadata.IsDir {
		t.Error("a staged regular file reported isDir true")
	}
	if metadata.URL != testURL {
		t.Errorf("url is %q, want %q", metadata.URL, testURL)
	}
	if want := base64.StdEncoding.EncodeToString(testPNG); metadata.QR != want {
		t.Errorf("qr is %q, want %q", metadata.QR, want)
	}
	if metadata.Warnings == nil {
		t.Error("warnings is nil, want an empty slice so it serializes as []")
	}
	if len(metadata.Warnings) != 0 {
		t.Errorf("warnings is %+v, want none", metadata.Warnings)
	}
	if got := h.state(); got != stateStaged {
		t.Errorf("state is %q, want %q", got, stateStaged)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the operation lease is still held after a committed Stage")
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("Stage published %+v, want no lifecycle event before acknowledgement", events)
	}
}

func TestStageAcquiresResourcesInContractOrder(t *testing.T) {
	h := newHarness(t)

	h.stageSuccessfully()

	want := []string{
		"entropy.Read",
		"entropy.Read",
		"source.Inspect",
		"network.GetLocalIP",
		"server.Start",
		"qr.EncodePNG",
		"network.StartBeacon",
		"clock.Now",
	}
	if got := h.calls.snapshot(); !slices.Equal(got, want) {
		t.Errorf("acquisition order is %v, want %v", got, want)
	}

	if paths := h.source.inspected(); len(paths) != 1 || paths[0] != testPath {
		t.Errorf("source saw %v, want exactly [%q]", paths, testPath)
	}
	if encoded := h.qr.encoded(); len(encoded) != 1 || encoded[0] != testURL {
		t.Errorf("QR encoded %v, want exactly [%q] -- the URL must be built before it", encoded, testURL)
	}

	requests := h.server.startRequests()
	if len(requests) != 1 {
		t.Fatalf("server started %d times, want exactly 1", len(requests))
	}
	if requests[0].SessionID != testSessionID || requests[0].Token != testToken {
		t.Errorf("server start request carried %q/%q, want %q/%q",
			requests[0].SessionID, requests[0].Token, testSessionID, testToken)
	}
	if requests[0].Item != testItem() {
		t.Errorf("server start request carried item %+v, want %+v", requests[0].Item, testItem())
	}
	if h.server.claimAuthorizer() != ClaimAuthorizer(h.coordinator) {
		t.Error("the server was started with an authorizer that is not the coordinator")
	}

	beacons := h.network.beaconRequests()
	if len(beacons) != 1 {
		t.Fatalf("beacon started %d times, want exactly 1", len(beacons))
	}
	want0 := BeaconRequest{
		SessionID: testSessionID,
		Service:   BeaconService,
		Instance:  "fairdrop",
		Port:      testPort,
		TXT:       []string{BeaconVersionTXT},
	}
	if beacons[0].SessionID != want0.SessionID || beacons[0].Service != want0.Service ||
		beacons[0].Instance != want0.Instance || beacons[0].Port != want0.Port ||
		!slices.Equal(beacons[0].TXT, want0.TXT) {
		t.Errorf("beacon request is %+v, want %+v", beacons[0], want0)
	}
}

func TestStageDrawsTwoIndependentIdentifiers(t *testing.T) {
	h := newHarness(t)

	metadata := h.stageSuccessfully()

	draws := h.entropy.draws()
	if len(draws) != 2 {
		t.Fatalf("entropy was read %d times, want two independent draws", len(draws))
	}
	for index, draw := range draws {
		if len(draw)*8 < 128 {
			t.Errorf("draw %d is %d bits, want at least 128", index, len(draw)*8)
		}
	}
	if slices.Equal(draws[0], draws[1]) {
		t.Error("the session id and the capability token came from the same bytes")
	}

	token := h.liveSession().token
	if SessionID(token) == metadata.SessionID {
		t.Error("the capability token equals the session id")
	}
	if len(metadata.SessionID) != 2*identityBytes || len(token) != 2*identityBytes {
		t.Errorf("identifiers are %d/%d hex characters, want %d each",
			len(metadata.SessionID), len(token), 2*identityBytes)
	}
	if !strings.Contains(metadata.URL, string(token)) {
		t.Error("the capability URL does not carry the token")
	}
}

func TestStageFailsClosedWhenEntropyFails(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		failAfter int
	}{
		{"first draw", 0},
		{"second draw", 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			h.entropy.failure = errors.New("entropy pool exhausted")
			h.entropy.failAfter = testCase.failAfter

			metadata, err := h.stage()

			if got := ErrorCodeOf(err); got != ErrTransferFailed {
				t.Errorf("error code is %q, want %q", got, ErrTransferFailed)
			}
			if metadata.SessionID != "" {
				t.Errorf("a failed Stage returned metadata %+v", metadata)
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state is %q, want %q", got, stateIdle)
			}
			if h.liveSession() != nil {
				t.Error("a session survived an entropy failure")
			}
			if h.coordinator.leaseHeld() {
				t.Error("the operation lease was not returned")
			}
			for _, call := range h.calls.snapshot() {
				if call != "entropy.Read" {
					t.Errorf("%s was called, want no resource acquired at all", call)
				}
			}
		})
	}
}

func TestStageRefusesOutsideIdle(t *testing.T) {
	t.Run("already staged", func(t *testing.T) {
		h := newHarness(t)
		h.stageSuccessfully()
		before := h.calls.snapshot()

		metadata, err := h.stage()

		if got := ErrorCodeOf(err); got != ErrBusy {
			t.Errorf("error code is %q, want %q", got, ErrBusy)
		}
		if metadata.SessionID != "" {
			t.Errorf("a refused Stage returned metadata %+v", metadata)
		}
		if got := h.state(); got != stateStaged {
			t.Errorf("state is %q, want the first session left in %q", got, stateStaged)
		}
		// The refusal costs two entropy draws and nothing else: no adapter is
		// touched, so no state and no resource changed.
		after := h.calls.snapshot()
		for _, call := range after[len(before):] {
			if call != "entropy.Read" {
				t.Errorf("a refused Stage called %s", call)
			}
		}
	})

	t.Run("still staging", func(t *testing.T) {
		h := newHarness(t)
		var reentrantErr error
		h.source.inspect = func(context.Context, string) (StagedItem, error) {
			_, reentrantErr = h.stage()
			return testItem(), nil
		}

		h.stageSuccessfully()

		if got := ErrorCodeOf(reentrantErr); got != ErrBusy {
			t.Errorf("a Stage during STAGING returned %q, want %q", got, ErrBusy)
		}
		if got := h.calls.count("source.Inspect"); got != 1 {
			t.Errorf("source was inspected %d times, want 1", got)
		}
	})

	t.Run("idle while a teardown still owns the lease", func(t *testing.T) {
		h := newHarness(t)
		h.coordinator.mu.Lock()
		took := h.coordinator.acquireLease()
		h.coordinator.mu.Unlock()
		if !took {
			t.Fatal("the lease was not free on a fresh coordinator")
		}
		defer h.coordinator.releaseLease()

		_, err := h.stage()

		if got := ErrorCodeOf(err); got != ErrBusy {
			t.Errorf("error code is %q, want %q", got, ErrBusy)
		}
		if got := h.state(); got != stateIdle {
			t.Errorf("state is %q, want %q", got, stateIdle)
		}
	})
}

func TestStageRefusesWhileClosing(t *testing.T) {
	h := newHarness(t)
	h.coordinator.beginClosing()

	_, err := h.stage()

	if got := ErrorCodeOf(err); got != ErrShuttingDown {
		t.Errorf("error code is %q, want %q", got, ErrShuttingDown)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
}

func TestStageUnwindsEverySetupFailure(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		arm        func(h *harness)
		wantCode   ErrorCode
		wantUnwind []string
	}{
		{
			name: "source refuses the selection",
			arm: func(h *harness) {
				h.source.inspect = func(context.Context, string) (StagedItem, error) {
					return StagedItem{}, WrapError(ErrPathNotFound, "the selection is gone", errors.New(testPath))
				}
			},
			wantCode: ErrPathNotFound,
		},
		{
			name: "no usable local address",
			arm: func(h *harness) {
				h.network.getLocalIP = func(context.Context) (netip.Addr, error) {
					return netip.Addr{}, NewError(ErrNetworkUnavailable, "no eligible interface")
				}
			},
			wantCode: ErrNetworkUnavailable,
		},
		{
			name: "an invalid address is refused rather than used",
			arm: func(h *harness) {
				h.network.getLocalIP = func(context.Context) (netip.Addr, error) {
					return netip.Addr{}, nil
				}
			},
			wantCode: ErrNetworkUnavailable,
		},
		{
			name: "the listener never becomes ready",
			arm: func(h *harness) {
				h.server.start = func(context.Context, ServerStartRequest, ClaimAuthorizer) (ServerHandle, error) {
					return ServerHandle{}, NewError(ErrServerStartFailed, "the listener could not bind")
				}
			},
			wantCode: ErrServerStartFailed,
		},
		{
			name: "the server reports no event lane",
			arm: func(h *harness) {
				h.server.start = func(context.Context, ServerStartRequest, ClaimAuthorizer) (ServerHandle, error) {
					return ServerHandle{Port: testPort}, nil
				}
			},
			wantCode:   ErrServerStartFailed,
			wantUnwind: []string{"server.Stop"},
		},
		{
			name: "the server reports an unusable port",
			arm: func(h *harness) {
				h.server.start = func(_ context.Context, _ ServerStartRequest, _ ClaimAuthorizer) (ServerHandle, error) {
					return ServerHandle{Port: 0, Events: h.server.events}, nil
				}
			},
			wantCode:   ErrServerStartFailed,
			wantUnwind: []string{"server.Stop"},
		},
		{
			name: "the QR encoder refuses the capability URL",
			arm: func(h *harness) {
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					return nil, NewError(ErrQRFailed, "the code could not be encoded")
				}
			},
			wantCode:   ErrQRFailed,
			wantUnwind: []string{"server.Stop"},
		},
		{
			name: "the QR encoder returns no image",
			arm: func(h *harness) {
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					return nil, nil
				}
			},
			wantCode:   ErrQRFailed,
			wantUnwind: []string{"server.Stop"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			testCase.arm(h)

			metadata, err := h.stage()

			if got := ErrorCodeOf(err); got != testCase.wantCode {
				t.Errorf("error code is %q, want %q", got, testCase.wantCode)
			}
			if metadata.SessionID != "" || metadata.URL != "" {
				t.Errorf("a failed Stage returned metadata %+v", metadata)
			}
			if got := h.calls.teardownCalls(); !slices.Equal(got, testCase.wantUnwind) {
				t.Errorf("unwind released %v, want %v", got, testCase.wantUnwind)
			}
			assertUnwoundToIdle(t, h)
		})
	}
}

func TestStageUnwindsWhenCancelledAfterEachStep(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		arm        func(h *harness)
		wantUnwind []string
	}{
		{
			name: "after the source inspection",
			arm: func(h *harness) {
				h.source.inspect = func(context.Context, string) (StagedItem, error) {
					defer h.coordinator.cancelSession()
					return testItem(), nil
				}
			},
		},
		{
			name: "after the address is resolved",
			arm: func(h *harness) {
				h.network.getLocalIP = func(context.Context) (netip.Addr, error) {
					defer h.coordinator.cancelSession()
					return testAddr(), nil
				}
			},
		},
		{
			name: "after the listener is ready",
			arm: func(h *harness) {
				h.server.start = func(_ context.Context, _ ServerStartRequest, _ ClaimAuthorizer) (ServerHandle, error) {
					defer h.coordinator.cancelSession()
					return ServerHandle{Port: testPort, Events: h.server.events}, nil
				}
			},
			wantUnwind: []string{"server.Stop"},
		},
		{
			name: "after the QR is encoded",
			arm: func(h *harness) {
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					defer h.coordinator.cancelSession()
					return append([]byte(nil), testPNG...), nil
				}
			},
			wantUnwind: []string{"server.Stop"},
		},
		{
			name: "after the beacon is published",
			arm: func(h *harness) {
				h.network.startBeacon = func(context.Context, BeaconRequest) error {
					defer h.coordinator.cancelSession()
					return nil
				}
			},
			wantUnwind: []string{"network.StopBeacon", "server.Stop"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			testCase.arm(h)

			metadata, err := h.stage()

			if got := ErrorCodeOf(err); got != ErrCancelled {
				t.Errorf("error code is %q, want %q", got, ErrCancelled)
			}
			if metadata.SessionID != "" || metadata.URL != "" || metadata.QR != "" {
				t.Errorf("Stage returned metadata %+v after losing to a cancellation", metadata)
			}
			if got := h.calls.teardownCalls(); !slices.Equal(got, testCase.wantUnwind) {
				t.Errorf("unwind released %v, want %v in reverse acquisition order", got, testCase.wantUnwind)
			}
			if events := h.observer.published(); len(events) != 0 {
				t.Errorf("a cancelled Stage published %+v, want no lifecycle event", events)
			}
			assertUnwoundToIdle(t, h)
		})
	}
}

func TestStageDiscardsAStaleResult(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		arm   func(h *harness)
		after func(t *testing.T, h *harness)
	}{
		{
			name: "the generation moved on",
			arm: func(h *harness) {
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					c := h.coordinator
					c.mu.Lock()
					c.session.generation++
					c.mu.Unlock()
					return append([]byte(nil), testPNG...), nil
				}
			},
			// The session is still this call's own, so clearing it and
			// returning to IDLE is correct.
			after: assertUnwoundToIdle,
		},
		{
			name: "the session was replaced",
			arm: func(h *harness) {
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					c := h.coordinator
					c.mu.Lock()
					c.session = &session{id: "a-different-session", generation: c.session.generation}
					c.mu.Unlock()
					return append([]byte(nil), testPNG...), nil
				}
			},
			// A replacement is NOT this call's to clear. The stale Stage must
			// release what it acquired and return cancelled while leaving the
			// installed session alone -- clearing it would deregister a live
			// session and force IDLE with its resources still running. This
			// expectation previously asserted the clobber and so passed
			// against the defect.
			after: func(t *testing.T, h *harness) {
				t.Helper()
				c := h.coordinator
				c.mu.Lock()
				defer c.mu.Unlock()
				if c.session == nil {
					t.Fatal("the stale Stage cleared a session it did not own")
				}
				if c.session.id != "a-different-session" {
					t.Fatalf("installed session is %q, want the replacement left untouched", c.session.id)
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			testCase.arm(h)

			metadata, err := h.stage()

			if got := ErrorCodeOf(err); got != ErrCancelled {
				t.Errorf("error code is %q, want %q", got, ErrCancelled)
			}
			if metadata.URL != "" {
				t.Errorf("a stale result was committed: %+v", metadata)
			}
			if got := h.calls.teardownCalls(); !slices.Equal(got, []string{"server.Stop"}) {
				t.Errorf("unwind released %v, want [server.Stop]", got)
			}
			testCase.after(t, h)
		})
	}
}

func TestStageCommitsWithAWarningWhenOnlyTheBeaconFails(t *testing.T) {
	h := newHarness(t)
	h.network.startBeacon = func(context.Context, BeaconRequest) error {
		return WrapError(ErrBeaconWarning, "device discovery could not register", errors.New(testPath))
	}

	metadata := h.stageSuccessfully()

	if got := h.state(); got != stateStaged {
		t.Errorf("state is %q, want %q -- a discovery failure alone is survivable", got, stateStaged)
	}
	if len(metadata.Warnings) != 1 {
		t.Fatalf("warnings are %+v, want exactly one", metadata.Warnings)
	}
	// A literal, not beaconWarning(): asserting the function against itself
	// let the code drift to any other message. The user of a perfectly
	// usable session -- HTTP and QR live, only discovery down -- must not be
	// told a transfer failed that never started.
	want := Warning{
		Code:    ErrBeaconWarning,
		Message: PublicErrorOf(NewError(ErrBeaconWarning, "")).Message,
	}
	if metadata.Warnings[0] != want {
		t.Errorf("warning is %+v, want the fixed %+v", metadata.Warnings[0], want)
	}
	if metadata.URL != testURL || metadata.QR == "" {
		t.Error("the session is not usable even though HTTP and QR are ready")
	}
	if got := h.calls.teardownCalls(); len(got) != 0 {
		t.Errorf("a survivable beacon failure released %v", got)
	}

	live := h.liveSession()
	if len(live.acquired) != 1 || live.acquired[0] != resourceServer {
		t.Errorf("the session holds %v, want only the server -- no beacon is live", live.acquired)
	}
}

func TestStageStampsTheCommitFromTheInjectedClock(t *testing.T) {
	h := newHarness(t)

	h.stageSuccessfully()

	if got := h.clock.reads(); got != 1 {
		t.Fatalf("the clock was read %d times during Stage, want 1", got)
	}
	live := h.liveSession()
	if live.stagedAt.IsZero() {
		t.Error("the commit was not stamped")
	}
	if want := h.clock.current; !live.stagedAt.Equal(want) {
		t.Errorf("the commit is stamped %v, want the injected %v", live.stagedAt, want)
	}
}

func TestStagedMetadataSerializesWarningsAsAnEmptyArray(t *testing.T) {
	h := newHarness(t)

	metadata := h.stageSuccessfully()

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshalling staged metadata failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"warnings":[]`) {
		t.Errorf("metadata serialized as %s, want an empty warnings array", encoded)
	}
	if !strings.Contains(string(encoded), `"qrBase64":"`) || !strings.Contains(string(encoded), `"sessionId":"`) {
		t.Errorf("metadata serialized as %s, want the contract's field names", encoded)
	}
}

func TestStagedQRIsPaddedBase64WithoutADataURIPrefix(t *testing.T) {
	h := newHarness(t)

	metadata := h.stageSuccessfully()

	if strings.HasPrefix(metadata.QR, "data:") {
		t.Error("the QR carries a data-URI prefix, which belongs to the renderer")
	}
	if len(metadata.QR)%4 != 0 {
		t.Errorf("the QR is %d characters, which is not padded base64", len(metadata.QR))
	}
	decoded, err := base64.StdEncoding.DecodeString(metadata.QR)
	if err != nil {
		t.Fatalf("the QR is not standard base64: %v", err)
	}
	if !slices.Equal(decoded, testPNG) {
		t.Errorf("the QR decodes to %v, want the encoder's bytes %v", decoded, testPNG)
	}
}

func TestStagedServerEventsAreDrainedWhileStaged(t *testing.T) {
	h := newHarness(t)
	h.stageSuccessfully()

	// A blocked lane would block teardown, so something reads it from the
	// moment the listener exists.
	for index := range 3 {
		select {
		case h.server.events <- ServerEvent{SessionID: testSessionID, Kind: ServerProgress}:
		case <-time.After(5 * time.Second):
			t.Fatalf("event %d was never consumed", index)
		}
	}

	live := h.liveSession()
	h.server.closeEvents()
	<-live.drainerDone
}

func TestStagedSessionNeverDisclosesTheTokenOrThePath(t *testing.T) {
	h := newHarness(t)
	h.network.stopBeacon = func() error {
		return WrapError(ErrBeaconWarning, "cleanup reported a problem", errors.New(testPath))
	}

	metadata := h.stageSuccessfully()
	token := string(h.liveSession().token)

	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("AuthorizeClaim returned %v, want a committed transfer", err)
	}

	for _, request := range h.network.beaconRequests() {
		assertSafe(t, "beacon request", fmt.Sprintf("%+v", request), token)
	}
	for _, event := range h.observer.published() {
		assertSafe(t, "published event", fmt.Sprintf("%+v", event), token)
	}
	for _, warning := range metadata.Warnings {
		assertSafe(t, "warning", fmt.Sprintf("%+v", warning), token)
	}
	for _, entry := range h.coordinator.diagnostics.snapshot() {
		assertSafe(t, "diagnostic", fmt.Sprintf("%s %s", entry.code, entry.message), token)
	}
	// Metadata may carry the token in the URL and the QR, and nowhere else.
	assertSafe(t, "metadata name", metadata.Name, token)
	if strings.Contains(fmt.Sprintf("%+v", metadata), testPath) {
		t.Errorf("staged metadata carries the source path: %+v", metadata)
	}
}

func TestStageErrorsNeverCarryTheTokenOrThePath(t *testing.T) {
	for _, testCase := range []struct {
		name string
		arm  func(h *harness)
	}{
		{"source failure", func(h *harness) {
			h.source.inspect = func(context.Context, string) (StagedItem, error) {
				return StagedItem{}, WrapError(ErrPathNotFound, "the selection is gone", errors.New(testPath))
			}
		}},
		{"server failure", func(h *harness) {
			h.server.start = func(context.Context, ServerStartRequest, ClaimAuthorizer) (ServerHandle, error) {
				return ServerHandle{}, WrapError(ErrServerStartFailed, "bind refused", errors.New(testPath))
			}
		}},
		{"qr failure", func(h *harness) {
			h.qr.encode = func(_ context.Context, content string) ([]byte, error) {
				return nil, WrapError(ErrQRFailed, "encoder refused", errors.New(content))
			}
		}},
		{"cancellation", func(h *harness) {
			h.source.inspect = func(context.Context, string) (StagedItem, error) {
				defer h.coordinator.cancelSession()
				return testItem(), nil
			}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			testCase.arm(h)

			_, err := h.stage()
			if err == nil {
				t.Fatal("Stage succeeded, want a failure to inspect")
			}

			// The token this attempt drew is the second entropy draw.
			draws := h.entropy.draws()
			token := ""
			if len(draws) == 2 {
				token = fmt.Sprintf("%x", draws[1])
			}
			assertSafe(t, "returned error", err.Error(), token)
			assertSafe(t, "public error", fmt.Sprintf("%+v", PublicErrorOf(err)), token)
		})
	}
}

// A committed session must outlive the command that created it. Stage detaches
// the session context from the caller's with context.WithoutCancel, because the
// listener keeps serving long after StageTransfer returns -- without it, the
// receiver's download dies the moment the Wails call completes.
func TestStagedSessionOutlivesTheCallersContext(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := h.stageWithContext(ctx); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	served := h.server.serverStartContext()
	if served == nil {
		t.Fatal("the server was started without a context")
	}

	cancel()

	select {
	case <-served.Done():
		t.Fatal("cancelling the caller's context killed the committed session's server")
	case <-time.After(100 * time.Millisecond):
	}
}

// The mirror: a caller that abandons Stage mid-setup must not end up with a
// committed session anyway. Without the AfterFunc watch on the caller's
// context, an abandoned command still leaves a live listener and a live
// advertisement behind.
func TestStageAbortsWhenTheCallerAbandonsIt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.qr.encode = func(context.Context, string) ([]byte, error) {
		cancel()
		return testPNG, nil
	}

	metadata, err := h.stageWithContext(ctx)

	if err == nil {
		t.Fatalf("Stage() committed %+v after the caller abandoned it", metadata)
	}
	if got := ErrorCodeOf(err); got != ErrCancelled {
		t.Fatalf("code = %q, want %q", got, ErrCancelled)
	}
	assertUnwoundToIdle(t, h)
}

// isDir is a mapping, and a mapping with only one input tested is a constant.
func TestStagedDirectoryReportsIsDir(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	folder := testItem()
	folder.Kind = ItemDirectory
	folder.Name = "holiday-photos"
	h.source.inspect = func(context.Context, string) (StagedItem, error) { return folder, nil }

	metadata, err := h.stage()
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if !metadata.IsDir {
		t.Fatal("a staged directory reported isDir false")
	}
	if metadata.Name != folder.Name {
		t.Fatalf("Name = %q, want %q", metadata.Name, folder.Name)
	}
}

// The capability path is asserted as a literal here on purpose, and separately
// from testURL, so that this test names the exact string internal/server
// registers. If the two ever diverge every QR code points at a bare 404.
func TestCapabilityURLUsesTheRouteTheServerRegisters(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	metadata := h.stageSuccessfully()

	const registeredPrefix = "/download/"
	parsed, err := url.Parse(metadata.URL)
	if err != nil {
		t.Fatalf("capability URL does not parse: %v", err)
	}
	if !strings.HasPrefix(parsed.Path, registeredPrefix) {
		t.Fatalf("capability path = %q, want the %q route internal/server registers",
			parsed.Path, registeredPrefix)
	}
	if got := strings.TrimPrefix(parsed.Path, registeredPrefix); got != string(testToken) {
		t.Fatalf("capability path carried %q after the route, want the token", got)
	}
}

func TestStageRefusesWithoutItsPorts(t *testing.T) {
	coordinator := NewCoordinator(Dependencies{})

	_, err := coordinator.Stage(context.Background(), testPath)

	if got := ErrorCodeOf(err); got != ErrTransferFailed {
		t.Errorf("error code is %q, want %q", got, ErrTransferFailed)
	}
	if err := coordinator.AuthorizeClaim(context.Background(), testSessionID); ErrorCodeOf(err) != ErrTransferFailed {
		t.Errorf("AuthorizeClaim returned %q, want %q", ErrorCodeOf(err), ErrTransferFailed)
	}
}

func assertUnwoundToIdle(t *testing.T, h *harness) {
	t.Helper()
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if h.liveSession() != nil {
		t.Error("a session survived the unwind")
	}
	if h.coordinator.leaseHeld() {
		t.Error("the operation lease was not returned by the unwind")
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("a failed Stage published %+v, want no lifecycle event", events)
	}
}

// assertSafe fails when a value that crosses a boundary carries the capability
// token or the absolute source path.
func assertSafe(t *testing.T, what, value, token string) {
	t.Helper()
	if token != "" && strings.Contains(value, token) {
		t.Errorf("%s carries the capability token: %s", what, value)
	}
	if strings.Contains(value, testPath) {
		t.Errorf("%s carries the source path: %s", what, value)
	}
}

// The three-second terminal lease holds a session with the operation lease
// free, so the busy guard is the only thing refusing a Stage during it. Nothing
// covered that: widening the guard to admit DONE and ERROR left the suite green
// while a replacement session displaced the one the UI was still displaying,
// whose reset then failed its id check and never arrived.
func TestStageIsRefusedDuringTheTerminalLease(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		outcome func(id SessionID) ServerEvent
		want    sessionState
	}{
		{"DONE", func(id SessionID) ServerEvent {
			return completeEvent(id, testProgress(testSize, 100))
		}, stateDone},
		{"ERROR", func(id SessionID) ServerEvent {
			return failedEvent(id, nil, NewError(ErrTransferFailed, "the stream broke"))
		}, stateError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()
			h.emit(testCase.outcome(metadata.SessionID))
			h.awaitDrainer()

			if got := h.state(); got != testCase.want {
				t.Fatalf("state is %q, want %q", got, testCase.want)
			}
			before := h.calls.snapshot()
			eventsBefore := len(h.observer.published())
			session := h.liveSession()

			second, err := h.stage()

			if got := ErrorCodeOf(err); got != ErrBusy {
				t.Errorf("Stage during the terminal lease returned %q, want %q", got, ErrBusy)
			}
			if second.SessionID != "" {
				t.Errorf("a refused Stage returned metadata %+v", second)
			}
			if got := h.state(); got != testCase.want {
				t.Errorf("state is %q, want the terminal session left in %q", got, testCase.want)
			}
			if h.liveSession() != session {
				t.Error("a refused Stage replaced the session the UI is still displaying")
			}
			if got := len(h.observer.published()); got != eventsBefore {
				t.Errorf("a refused Stage published %d extra events", got-eventsBefore)
			}
			// Two entropy draws and nothing else: no adapter is touched.
			for _, call := range h.calls.snapshot()[len(before):] {
				if call != "entropy.Read" {
					t.Errorf("a refused Stage called %s", call)
				}
			}
		})
	}
}
