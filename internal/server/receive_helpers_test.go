package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/sink"
	"fairdrop/internal/transfer"
)

const testBoundary = "fairdropboundary7d4a"

// fixtureDir is the only way a receive test gets a directory: the real sink
// refuses a link-like component anywhere on a destination path, and a raw
// t.TempDir() is spelled through a /var symlink on macOS. Same helper, same
// reason, as internal/source and internal/stream carry.
func fixtureDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving the fixture directory: %v", err)
	}
	return resolved
}

// testPart is one part of a hand-built multipart body. A nil filename makes it a
// plain form field.
type testPart struct {
	field    string
	filename *string
	content  string
	// rawHeaders replaces the whole Content-Disposition value when set, for
	// names the standard encoders would escape.
	rawDisposition string
}

func uploadFile(name, content string) testPart {
	return testPart{field: "files", filename: &name, content: content}
}

func fieldPart(name, content string) testPart {
	return testPart{field: name, content: content}
}

// multipartBody builds a body byte for byte, so a test controls exactly what
// the server parses: raw backslashes in a name, a missing terminator, a part
// header the standard writer would never emit.
func multipartBody(parts ...testPart) []byte {
	var body bytes.Buffer
	for _, part := range parts {
		fmt.Fprintf(&body, "--%s\r\n", testBoundary)
		switch {
		case part.rawDisposition != "":
			fmt.Fprintf(&body, "Content-Disposition: %s\r\n", part.rawDisposition)
		case part.filename != nil:
			fmt.Fprintf(&body, "Content-Disposition: form-data; name=%q; filename=\"%s\"\r\n", part.field, *part.filename)
		default:
			fmt.Fprintf(&body, "Content-Disposition: form-data; name=%q\r\n", part.field)
		}
		if part.filename != nil || part.rawDisposition != "" {
			body.WriteString("Content-Type: application/octet-stream\r\n")
		}
		body.WriteString("\r\n")
		body.WriteString(part.content)
		body.WriteString("\r\n")
	}
	fmt.Fprintf(&body, "--%s--\r\n", testBoundary)
	return body.Bytes()
}

const multipartType = "multipart/form-data; boundary=" + testBoundary

// receiveRig is a started receive server over a real sink and a real directory,
// bound to loopback.
type receiveRig struct {
	t      *testing.T
	server *Server
	handle transfer.ServerHandle
	auth   *stubAuthorizer
	dir    string
	dest   transfer.ReceiveDestination
	base   string
}

type rigOption func(*receiveRig, *Server)

// withTimeouts shrinks the net/http deadlines and the upload inactivity bound.
func withTimeouts(timeouts serverTimeouts) rigOption {
	return func(_ *receiveRig, server *Server) { server.timeouts = timeouts }
}

// withDestination wraps the real destination.
func withDestination(wrap func(transfer.ReceiveDestination) transfer.ReceiveDestination) rigOption {
	return func(rig *receiveRig, _ *Server) { rig.dest = wrap(rig.dest) }
}

func withAuthorizer(auth *stubAuthorizer) rigOption {
	return func(rig *receiveRig, _ *Server) { rig.auth = auth }
}

func startReceiveRig(t *testing.T, options ...rigOption) *receiveRig {
	t.Helper()
	dir := fixtureDir(t)
	dest, err := sink.New().OpenDestination(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenDestination = %v", err)
	}
	rig := &receiveRig{t: t, auth: &stubAuthorizer{}, dir: dir, dest: dest}
	server := newTestServer(t, nil)
	for _, option := range options {
		option(rig, server)
	}
	rig.server = server

	handle, err := server.Start(context.Background(), transfer.ServerStartRequest{
		SessionID:   testSession,
		Token:       testToken,
		Destination: rig.dest,
	}, rig.auth)
	if err != nil {
		t.Fatalf("Start = %v", err)
	}
	rig.handle = handle
	rig.base = baseURL(handle.Port)
	t.Cleanup(func() {
		_ = server.Stop()
		_, _ = dest.Close()
	})
	return rig
}

func (r *receiveRig) uploadURL() string { return r.base + "/upload/" + string(testToken) }

// post sends a complete multipart body with an exact Content-Length.
func (r *receiveRig) post(body []byte) (*http.Response, []byte) {
	r.t.Helper()
	return r.postAs(body, multipartType)
}

func (r *receiveRig) postAs(body []byte, contentType string) (*http.Response, []byte) {
	r.t.Helper()
	request, err := http.NewRequest(http.MethodPost, r.uploadURL(), bytes.NewReader(body))
	if err != nil {
		r.t.Fatalf("NewRequest = %v", err)
	}
	request.Header.Set("Content-Type", contentType)
	response, err := testClient().Do(request)
	if err != nil {
		r.t.Fatalf("POST error = %v", err)
	}
	defer response.Body.Close()
	page, err := io.ReadAll(response.Body)
	if err != nil {
		r.t.Fatalf("reading the response = %v", err)
	}
	return response, page
}

// rawConn opens a TCP connection to the server and writes a POST request line
// and headers declaring contentLength, leaving the body to the caller.
func (r *receiveRig) rawConn(contentLength int) net.Conn {
	r.t.Helper()
	conn, err := net.DialTimeout("tcp", hostPort(r.handle.Port), 5*time.Second)
	if err != nil {
		r.t.Fatalf("dial = %v", err)
	}
	r.t.Cleanup(func() { _ = conn.Close() })
	header := "POST /upload/" + string(testToken) + " HTTP/1.1\r\nHost: " + hostPort(r.handle.Port) +
		"\r\nContent-Type: " + multipartType + "\r\nContent-Length: " + strconv.Itoa(contentLength) + "\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(header)); err != nil {
		r.t.Fatalf("writing the request head = %v", err)
	}
	return conn
}

// subfolder returns the one FairDrop subfolder, failing if there is not exactly one.
func (r *receiveRig) subfolder() string {
	r.t.Helper()
	var folders []string
	for _, name := range listNames(r.t, r.dir) {
		if strings.HasPrefix(name, "FairDrop ") {
			folders = append(folders, name)
		}
	}
	if len(folders) != 1 {
		r.t.Fatalf("destination holds %v, want exactly one FairDrop subfolder", listNames(r.t, r.dir))
	}
	return filepath.Join(r.dir, folders[0])
}

func (r *receiveRig) assertNothingWritten() {
	r.t.Helper()
	if got := listNames(r.t, r.dir); len(got) != 0 {
		r.t.Fatalf("destination holds %v, want nothing written", got)
	}
}

func listNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %q: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %q: %v", path, err)
	}
	return string(data)
}

// awaitTerminal reads the lane until the one terminal event, returning every
// event seen. It fails rather than hangs, so a mutated guarantee shows up as a
// named assertion and not as a timed-out package.
func awaitTerminal(t *testing.T, events <-chan transfer.ServerEvent) ([]transfer.ServerEvent, transfer.ServerEvent) {
	t.Helper()
	var seen []transfer.ServerEvent
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event, open := <-events:
			if !open {
				t.Fatalf("the lane closed without a terminal event; saw %d events", len(seen))
			}
			seen = append(seen, event)
			if event.Kind == transfer.ServerComplete || event.Kind == transfer.ServerFailed {
				return seen, event
			}
		case <-deadline:
			t.Fatalf("no terminal event within 10s; saw %d events", len(seen))
		}
	}
}

// awaitNotice reads the lane until a notice arrives.
func awaitNotice(t *testing.T, events <-chan transfer.ServerEvent) transfer.ServerEvent {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event, open := <-events:
			if !open {
				t.Fatal("the lane closed before a notice")
			}
			if event.Kind == transfer.ServerNotice {
				return event
			}
			t.Fatalf("got a %s event while waiting for a notice", event.Kind)
		case <-deadline:
			t.Fatal("no notice within 10s")
		}
	}
}

// spyDestination wraps a destination so a test can refuse the space check or
// fail a write at will.
type spyDestination struct {
	transfer.ReceiveDestination
	checkSpace func(declared int64) error
	saveFile   func(ctx context.Context, name string, content io.Reader) (int64, error)

	checks   atomic.Int64
	declared atomic.Int64
	saves    atomic.Int64
}

func (s *spyDestination) CheckSpace(declared int64) error {
	s.checks.Add(1)
	s.declared.Store(declared)
	if s.checkSpace != nil {
		return s.checkSpace(declared)
	}
	return s.ReceiveDestination.CheckSpace(declared)
}

func (s *spyDestination) SaveFile(ctx context.Context, name string, content io.Reader) (int64, error) {
	s.saves.Add(1)
	if s.saveFile != nil {
		return s.saveFile(ctx, name, content)
	}
	return s.ReceiveDestination.SaveFile(ctx, name, content)
}

// readResponse reads one HTTP response off a raw connection.
func readResponse(t *testing.T, conn net.Conn) (*http.Response, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("reading the response = %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response, body
}
