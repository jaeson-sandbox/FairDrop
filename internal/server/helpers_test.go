package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

const (
	testSession = transfer.SessionID("session-1")
	testToken   = transfer.CapabilityToken("f2f0b1c4d6e8a0b2c4d6e8a0b2c4d6e8")
)

// stubPayload is a PreparedPayload that records the ownership rules the server
// promises: exactly one Close, never before WriteTo returns, and never
// concurrent with it.
type stubPayload struct {
	name  string
	size  int64
	known bool
	// stream is the body. A nil stream writes nothing and succeeds, which is
	// the known-empty payload.
	stream func(ctx context.Context, dst io.Writer) error

	writeStarted  atomic.Bool
	writing       atomic.Bool
	writeReturned atomic.Bool

	closes          atomic.Int64
	onClose         func()
	closedDuringRun atomic.Bool
	closedTooEarly  atomic.Bool
}

func (p *stubPayload) DownloadName() string { return p.name }

func (p *stubPayload) Size() (int64, bool) { return p.size, p.known }

func (p *stubPayload) WriteTo(ctx context.Context, dst io.Writer) error {
	p.writeStarted.Store(true)
	p.writing.Store(true)
	defer func() {
		p.writing.Store(false)
		p.writeReturned.Store(true)
	}()
	if p.stream == nil {
		return nil
	}
	return p.stream(ctx, dst)
}

func (p *stubPayload) Close() error {
	if p.onClose != nil {
		p.onClose()
	}
	p.closes.Add(1)
	if p.writing.Load() {
		p.closedDuringRun.Store(true)
	}
	if p.writeStarted.Load() && !p.writeReturned.Load() {
		p.closedTooEarly.Store(true)
	}
	return nil
}

// assertOwnedOnce checks the payload ownership contract from the payload's own
// point of view.
func (p *stubPayload) assertOwnedOnce(t *testing.T) {
	t.Helper()
	if got := p.closes.Load(); got != 1 {
		t.Fatalf("payload Close ran %d times, want exactly 1", got)
	}
	if p.closedDuringRun.Load() {
		t.Fatal("payload Close ran concurrently with WriteTo")
	}
	if p.closedTooEarly.Load() {
		t.Fatal("payload Close ran before WriteTo returned")
	}
}

// bodyOf returns a stream that writes body in chunk-sized pieces.
func bodyOf(body []byte, chunk int) func(context.Context, io.Writer) error {
	return func(_ context.Context, dst io.Writer) error {
		for offset := 0; offset < len(body); offset += chunk {
			end := min(offset+chunk, len(body))
			if _, err := dst.Write(body[offset:end]); err != nil {
				return transfer.WrapError(transfer.ErrTransferFailed, "stub payload destination failed", err)
			}
		}
		return nil
	}
}

// stubPayloads is the PayloadPort seam. It records how often Prepare ran and
// with which item.
type stubPayloads struct {
	prepare func(ctx context.Context, item transfer.StagedItem) (PreparedPayload, error)

	calls atomic.Int64

	mu    sync.Mutex
	items []transfer.StagedItem
}

func (p *stubPayloads) Prepare(ctx context.Context, item transfer.StagedItem) (PreparedPayload, error) {
	p.calls.Add(1)
	p.mu.Lock()
	p.items = append(p.items, item)
	p.mu.Unlock()
	if p.prepare == nil {
		return &stubPayload{name: item.Name, known: true}, nil
	}
	return p.prepare(ctx, item)
}

func (p *stubPayloads) preparedItems() []transfer.StagedItem {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]transfer.StagedItem(nil), p.items...)
}

func payloadsReturning(payload PreparedPayload) *stubPayloads {
	return &stubPayloads{
		prepare: func(context.Context, transfer.StagedItem) (PreparedPayload, error) {
			return payload, nil
		},
	}
}

func payloadsFailing(err error) *stubPayloads {
	return &stubPayloads{
		prepare: func(context.Context, transfer.StagedItem) (PreparedPayload, error) {
			return nil, err
		},
	}
}

// stubAuthorizer is the coordinator seam. It records every claim it was asked
// to authorize so a test can prove the handshake ran exactly once.
type stubAuthorizer struct {
	authorize func(ctx context.Context, sessionID transfer.SessionID) error

	calls atomic.Int64

	mu       sync.Mutex
	sessions []transfer.SessionID
}

func (a *stubAuthorizer) AuthorizeClaim(ctx context.Context, sessionID transfer.SessionID) error {
	a.calls.Add(1)
	a.mu.Lock()
	a.sessions = append(a.sessions, sessionID)
	a.mu.Unlock()
	if a.authorize == nil {
		return nil
	}
	return a.authorize(ctx, sessionID)
}

func (a *stubAuthorizer) claimedSessions() []transfer.SessionID {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]transfer.SessionID(nil), a.sessions...)
}

func refusingAuthorizer(err error) *stubAuthorizer {
	return &stubAuthorizer{
		authorize: func(context.Context, transfer.SessionID) error { return err },
	}
}

// testClock is a manual clock. Cadence is behavior under test, so no test
// sleeps to observe it.
type testClock struct {
	mu   sync.Mutex
	at   time.Time
	step time.Duration
}

func newTestClock(step time.Duration) *testClock {
	return &testClock{at: time.Unix(1_700_000_000, 0), step: step}
}

// now advances by a fixed step per reading, so a payload that writes N chunks
// produces a deterministic timeline.
func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	at := c.at
	c.at = c.at.Add(c.step)
	return at
}

// newTestServer builds a server bound to loopback. Production binds every
// interface; a test that did the same would trip a host firewall prompt and
// expose the fixture to the LAN.
func newTestServer(t *testing.T, payloads PayloadPort) *Server {
	t.Helper()
	server := New(payloads)
	server.listen = func(ctx context.Context, _ string) (net.Listener, error) {
		var config net.ListenConfig
		return config.Listen(ctx, "tcp", "127.0.0.1:0")
	}
	server.now = newTestClock(0).now
	return server
}

func startTestServer(t *testing.T, server *Server, authorizer transfer.ClaimAuthorizer) transfer.ServerHandle {
	t.Helper()
	handle, err := server.Start(context.Background(), startRequest(), authorizer)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	return handle
}

func startRequest() transfer.ServerStartRequest {
	return transfer.ServerStartRequest{
		SessionID: testSession,
		Token:     testToken,
		Item: transfer.StagedItem{
			Path:        `C:\Users\example\Documents\quarterly report.pdf`,
			Name:        "quarterly report.pdf",
			Kind:        transfer.ItemFile,
			LogicalSize: 12,
		},
	}
}

func downloadURL(port int, token string) string {
	return baseURL(port) + "/download/" + token
}

func baseURL(port int) string {
	return "http://" + hostPort(port)
}

func hostPort(port int) string {
	return "127.0.0.1:" + strconv.Itoa(port)
}

// testClient never follows redirects: a redirect is a distinct outcome this
// server must never produce, so following one would hide it.
func testClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       30 * time.Second,
	}
}

func do(t *testing.T, method, url string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("NewRequest(%q, %q) error = %v", method, url, err)
	}
	response, err := testClient().Do(request)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, url, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// drainEvents collects every event until the lane closes.
func drainEvents(t *testing.T, events <-chan transfer.ServerEvent) []transfer.ServerEvent {
	t.Helper()
	var collected []transfer.ServerEvent
	deadline := time.After(30 * time.Second)
	for {
		select {
		case event, open := <-events:
			if !open {
				return collected
			}
			collected = append(collected, event)
		case <-deadline:
			t.Fatal("event lane did not close")
			return collected
		}
	}
}

func assertNoEvents(t *testing.T, events <-chan transfer.ServerEvent) {
	t.Helper()
	select {
	case event, open := <-events:
		if !open {
			t.Fatal("event lane closed while the server was still live")
		}
		t.Fatalf("unexpected %s event", event.Kind)
	default:
	}
}

func terminalEvent(t *testing.T, collected []transfer.ServerEvent) transfer.ServerEvent {
	t.Helper()
	var terminal []transfer.ServerEvent
	for index, event := range collected {
		switch event.Kind {
		case transfer.ServerComplete, transfer.ServerFailed:
			terminal = append(terminal, event)
			if index != len(collected)-1 {
				t.Fatalf("%s event was followed by %s", event.Kind, collected[index+1].Kind)
			}
		}
	}
	if len(terminal) != 1 {
		t.Fatalf("got %d terminal events, want exactly 1", len(terminal))
	}
	return terminal[0]
}

// assertNoDisclosure fails if a response reveals the token, the source path,
// or the staged name anywhere a receiver can read.
func assertNoDisclosure(t *testing.T, response *http.Response, body []byte) {
	t.Helper()
	secrets := []string{string(testToken), startRequest().Item.Path, "quarterly"}
	var rendered strings.Builder
	for name, values := range response.Header {
		rendered.WriteString(name)
		rendered.WriteString(": ")
		rendered.WriteString(strings.Join(values, ","))
		rendered.WriteString("\n")
	}
	rendered.Write(body)
	for _, secret := range secrets {
		if strings.Contains(rendered.String(), secret) {
			t.Fatalf("response disclosed %q", secret)
		}
	}
	if len(body) != 0 {
		t.Fatalf("rejection carried a body: %q", body)
	}
}

func readBody(t *testing.T, response *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return body
}

func isConnectionFailure(err error) bool {
	return err != nil && (errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		strings.Contains(err.Error(), "connection"))
}
