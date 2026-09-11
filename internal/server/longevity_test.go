package server

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

/*
A transfer that outlives every net/http deadline still completes.

The server sets ReadHeaderTimeout, ReadTimeout and IdleTimeout and
deliberately no WriteTimeout, on the reasoning that a download request has
no body so the read deadlines bound only pathological clients. This test is
the proof of that reasoning: every deadline is shrunk below the length of the
body, and the body still arrives whole through a real listener and a real
client. Adding a WriteTimeout fails it immediately.

It exists because of a wrong theory, recorded so it is not re-derived. While
chasing a live folder download that died on a phone, net/http was read as
keeping ReadTimeout armed through the handler and cancelling
request.Context() once the response outlived it. Mutating the handler to
stream from request.Context() under this test did not fail it: net/http
clears the read deadline in startBackgroundRead before its disconnect read
begins, so for a bodiless GET the deadline is spent before the handler runs.
A twenty-second ReadTimeout cannot cap a transfer; a WriteTimeout would.
*/
func TestATransferLongerThanEveryTimeoutStillCompletes(t *testing.T) {
	t.Parallel()

	const (
		chunks   = 12
		pause    = 100 * time.Millisecond
		deadline = 200 * time.Millisecond
	)
	chunk := []byte("0123456789abcdef")
	want := chunks * len(chunk)

	payload := &stubPayload{name: "folder.zip", known: false, stream: func(ctx context.Context, dst io.Writer) error {
		for index := 0; index < chunks; index++ {
			if err := ctx.Err(); err != nil {
				return transfer.WrapError(transfer.ErrCancelled, "stream cancelled", err)
			}
			if _, err := dst.Write(chunk); err != nil {
				return transfer.WrapError(transfer.ErrTransferFailed, "destination failed", err)
			}
			time.Sleep(pause)
		}
		return nil
	}}

	server := newTestServer(t, payloadsReturning(payload))
	// 1.2 seconds of body against 200 milliseconds of every deadline.
	server.timeouts = serverTimeouts{readHeader: deadline, read: deadline, idle: deadline}
	handle := startTestServer(t, server, &stubAuthorizer{})

	// No client timeout: the client must be able to wait as long as a phone
	// would, so the only thing that can end the body early is the server.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Get(downloadURL(handle.Port, string(testToken)))
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got, ok := response.Header["Content-Length"]; ok {
		t.Fatalf("Content-Length = %v, want it absent for an unknown length", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("body ended after %d of %d bytes: %v", len(body), want, err)
	}
	if len(body) != want {
		t.Fatalf("received %d bytes, want %d", len(body), want)
	}

	awaitNaturalCompletion(t, server)
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	terminal := terminalEvent(t, drainEvents(t, handle.Events))
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %v, want %v", terminal.Kind, transfer.ServerComplete)
	}
	if terminal.Progress == nil || terminal.Progress.BytesSent != int64(want) {
		t.Fatalf("terminal progress = %+v, want BytesSent %d", terminal.Progress, want)
	}
}
