package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// These gates intercept the actual connection write performed by net/http's
// finishRequest, after our handler returns. Small bodies remain buffered until
// then; chunked responses include their terminating zero chunk in that write.
func TestNaturalCompletionWaitsForHTTPFinalization(t *testing.T) {
	for _, known := range []bool{true, false} {
		name := "chunked"
		if known {
			name = "file"
		}
		for _, outcome := range []string{"success", "write-error", "short-write", "cancel", "disconnect"} {
			t.Run(name+"/"+outcome, func(t *testing.T) {
				body := []byte("final response bytes")
				payload := &stubPayload{name: "report", size: int64(len(body)), known: known, stream: bodyOf(body, len(body))}
				server := newTestServer(t, payloadsReturning(payload))
				gate := installFinalWriteGate(server, 2, outcome)
				handle := startTestServer(t, server, &stubAuthorizer{})
				t.Cleanup(gate.release)
				response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
				wire := awaitFinalWrite(t, gate)
				if known && !bytes.Equal(wire, body) {
					t.Fatal("gate did not intercept the buffered file body")
				}
				if !known && !bytes.HasSuffix(wire, []byte("\r\n0\r\n\r\n")) {
					t.Fatal("gate did not intercept final chunk framing")
				}
				assertNoEvents(t, handle.Events)
				payload.assertOwnedOnce(t)

				if outcome == "cancel" {
					stopped := make(chan error, 1)
					go func() { stopped <- server.Stop() }()
					select {
					case err := <-stopped:
						if err != nil {
							t.Fatalf("Stop during final write: %v", err)
						}
					case <-time.After(5 * time.Second):
						t.Fatal("Stop waited for a blocked final write instead of force-closing")
					}
					if events := drainEvents(t, handle.Events); len(events) != 0 {
						t.Fatal("cancelled finalization published a terminal outcome")
					}
					return
				}
				if outcome == "disconnect" {
					_ = response.Body.Close()
					select {
					case <-gate.peerGone:
					case <-time.After(5 * time.Second):
						t.Fatal("connection did not observe receiver departure")
					}
				}
				gate.release()
				terminal := awaitEvent(t, handle.Events, 5*time.Second)
				// Consume the outcome immediately, exactly as the coordinator does.
				if err := server.Stop(); err != nil {
					t.Fatal(err)
				}
				if outcome == "success" {
					if terminal.Kind != transfer.ServerComplete {
						t.Fatalf("finalized response reported %s", terminal.Kind)
					}
					if received := readBody(t, response); !bytes.Equal(received, body) {
						t.Fatal("immediate terminal teardown truncated the body")
					}
				} else {
					if terminal.Kind != transfer.ServerFailed || transfer.ErrorCodeOf(terminal.Err) != transfer.ErrTransferFailed {
						t.Fatal("failed final write was not reported as transfer_failed")
					}
					if outcome == "short-write" && !errors.Is(terminal.Err, io.ErrShortWrite) {
						t.Fatal("short final write did not retain io.ErrShortWrite")
					}
					if outcome != "disconnect" {
						if _, err := io.ReadAll(response.Body); err == nil {
							t.Fatal("failed finalization looked complete to the receiver")
						}
					}
				}
			})
		}
	}
}

func TestPreparationFailureFinalizes410BeforeTerminalTeardown(t *testing.T) {
	server := newTestServer(t, payloadsFailing(transfer.NewError(transfer.ErrSourceChanged, "source changed")))
	gate := installFinalWriteGate(server, 1, "success")
	handle := startTestServer(t, server, &stubAuthorizer{})
	t.Cleanup(gate.release)
	responseReady := make(chan *http.Response, 1)
	requestFailed := make(chan error, 1)
	go func() {
		response, err := testClient().Get(downloadURL(handle.Port, string(testToken)))
		if err != nil {
			requestFailed <- err
			return
		}
		responseReady <- response
	}()
	wire := awaitFinalWrite(t, gate)
	if !bytes.HasPrefix(wire, []byte("HTTP/1.1 410 Gone\r\n")) {
		t.Fatal("gate did not intercept the promised 410")
	}
	assertNoEvents(t, handle.Events)
	gate.release()
	terminal := awaitEvent(t, handle.Events, 5*time.Second)
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	if terminal.Kind != transfer.ServerFailed || transfer.ErrorCodeOf(terminal.Err) != transfer.ErrSourceChanged {
		t.Fatal("preparation failure lost its original coded cause")
	}
	select {
	case response := <-responseReady:
		defer response.Body.Close()
		if response.StatusCode != 410 || len(readBody(t, response)) != 0 {
			t.Fatal("terminal teardown changed the promised empty 410")
		}
	case <-requestFailed:
		t.Fatal("terminal teardown prevented the receiver reading 410")
	case <-time.After(5 * time.Second):
		t.Fatal("410 response did not finish")
	}
}

type finalWriteGate struct {
	entered  chan []byte
	proceed  chan struct{}
	peerGone chan struct{}
	release  func()
	outcome  string
	write    int
}

func installFinalWriteGate(server *Server, write int, outcome string) *finalWriteGate {
	gate := &finalWriteGate{entered: make(chan []byte, 1), proceed: make(chan struct{}), peerGone: make(chan struct{}), outcome: outcome, write: write}
	gate.release = sync.OnceFunc(func() { close(gate.proceed) })
	listen := server.listen
	server.listen = func(ctx context.Context, address string) (net.Listener, error) {
		listener, err := listen(ctx, address)
		if err != nil {
			return nil, err
		}
		return &gatedListener{Listener: listener, gate: gate}, nil
	}
	return gate
}

func awaitFinalWrite(t *testing.T, gate *finalWriteGate) []byte {
	t.Helper()
	select {
	case wire := <-gate.entered:
		return wire
	case <-time.After(5 * time.Second):
		t.Fatal("net/http did not reach its final connection write")
		return nil
	}
}

type gatedListener struct {
	net.Listener
	gate *finalWriteGate
}

func (l *gatedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &gatedConn{Conn: conn, gate: l.gate, closed: make(chan struct{})}, nil
}

type gatedConn struct {
	net.Conn
	gate      *finalWriteGate
	writes    int
	closed    chan struct{}
	closeOnce sync.Once
	readOnce  sync.Once
}

func (c *gatedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil {
		c.readOnce.Do(func() { close(c.gate.peerGone) })
	}
	return n, err
}

func (c *gatedConn) Write(p []byte) (int, error) {
	c.writes++
	if c.writes == c.gate.write {
		c.gate.entered <- bytes.Clone(p)
		select {
		case <-c.gate.proceed:
		case <-c.closed:
			return 0, net.ErrClosed
		}
		switch c.gate.outcome {
		case "write-error":
			return 0, io.ErrClosedPipe
		case "short-write":
			return 0, nil
		case "disconnect":
			// The native background read has observed the peer leaving. Force
			// the destination refusal here instead of relying on TCP buffer timing.
			return 0, net.ErrClosed
		}
	}
	return c.Conn.Write(p)
}

func (c *gatedConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}
