package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

type receivedTextOwner struct {
	mu             sync.Mutex
	authorizations int
	commits        []string
	claim          func(context.Context) error
}

type gatedTextBody struct {
	entered chan struct{}
	release chan struct{}
	content string
	once    sync.Once
}

type countingTextBody struct {
	reader io.Reader
	read   int
}

func (b *countingTextBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.read += n
	return n, err
}
func (*countingTextBody) Close() error { return nil }

func (b *gatedTextBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	if b.content == "" {
		return 0, io.EOF
	}
	n := copy(p, b.content)
	b.content = b.content[n:]
	return n, nil
}
func (b *gatedTextBody) Close() error { return nil }

func (o *receivedTextOwner) AuthorizeClaim(ctx context.Context, _ transfer.SessionID) error {
	o.mu.Lock()
	o.authorizations++
	o.mu.Unlock()
	if o.claim != nil {
		return o.claim(ctx)
	}
	return nil
}

func (o *receivedTextOwner) CommitReceivedText(_ context.Context, _ transfer.SessionID, value string) error {
	o.mu.Lock()
	o.commits = append(o.commits, value)
	o.mu.Unlock()
	return nil
}

func (o *receivedTextOwner) snapshot() (int, []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.authorizations, append([]string(nil), o.commits...)
}

func startReceiveTextServer(t *testing.T, owner *receivedTextOwner) (string, transfer.ServerHandle, *Server) {
	t.Helper()
	s := newTestServer(t, nil)
	handle, err := s.Start(context.Background(), transfer.ServerStartRequest{
		SessionID: testSession, Token: testToken, ReceiveText: true,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return baseURL(handle.Port) + "/receive-text/" + string(testToken), handle, s
}

func postReceivedText(t *testing.T, target, contentType, body string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", contentType)
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return response, string(page)
}

func TestReceiveTextFormGETIsEmptyAndDoesNotClaim(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, _ := startReceiveTextServer(t, owner)
	response := do(t, http.MethodGet, target)
	page, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), `name="text"`) ||
		!strings.Contains(string(page), `method="post" action="" enctype="application/x-www-form-urlencoded"`) ||
		strings.Contains(string(page), "<script") || !strings.Contains(string(page), "65,536 UTF-8 bytes") {
		t.Fatalf("form status=%d body=%q", response.StatusCode, page)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Security-Policy") == "" ||
		response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("form security headers = %#v", response.Header)
	}
	if claims, commits := owner.snapshot(); claims != 0 || len(commits) != 0 {
		t.Fatalf("GET claims=%d commits=%v", claims, commits)
	}
}

func TestReceiveTextMethodPathAndTokenRefusalsLeaveClaimAvailable(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, _ := startReceiveTextServer(t, owner)
	for _, method := range []string{http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		response := do(t, method, target)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound || response.Header.Get("Allow") != "" {
			t.Fatalf("%s = %d", method, response.StatusCode)
		}
	}
	for _, wrong := range []string{target + "/", target + "//", baseURLFromReceiveText(target) + "/receive-text/wrong", strings.Replace(target, "/receive-text/", "/upload/", 1)} {
		response := do(t, http.MethodGet, wrong)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound || response.Header.Get("Location") != "" {
			t.Fatalf("wrong route = %d", response.StatusCode)
		}
	}
	if claims, _ := owner.snapshot(); claims != 0 {
		t.Fatalf("wrong requests claimed %d times", claims)
	}
}

func baseURLFromReceiveText(target string) string { return strings.Split(target, "/receive-text/")[0] }

func TestReceiveTextRejectsMalformedBodiesWithoutSpendingCode(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, server := startReceiveTextServer(t, owner)
	for _, testCase := range []struct {
		name, media, body string
		status            int
	}{
		{"blank", "application/x-www-form-urlencoded", "text=%20%0D%0A", 400},
		{"duplicate", "application/x-www-form-urlencoded", "text=a&text=b", 400},
		{"extra", "application/x-www-form-urlencoded", "text=a&other=b", 400},
		{"bad escape", "application/x-www-form-urlencoded", "text=%ZZ", 400},
		{"bad utf8", "application/x-www-form-urlencoded", "text=%FF", 400},
		{"control", "application/x-www-form-urlencoded", "text=a%00b", 400},
		{"wrong media", "text/plain", "text=a", 400},
		{"wrong charset", "application/x-www-form-urlencoded; charset=iso-8859-1", "text=a", 400},
		{"unknown parameter", "application/x-www-form-urlencoded; boundary=x", "text=a", 400},
		{"raw too long", "application/x-www-form-urlencoded", "text=" + strings.Repeat("a", 131073), 400},
		{"encoded too long", "application/x-www-form-urlencoded", "text=" + strings.Repeat("%61", maxReceiveTextFormBytes/3+1), 413},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response, page := postReceivedText(t, target, testCase.media, testCase.body)
			if response.StatusCode != testCase.status || strings.Contains(page, "text=a") || strings.Contains(page, "Text received by this computer") {
				t.Fatalf("status=%d page=%q", response.StatusCode, page)
			}
		})
	}
	chunked := httptest.NewRequest(http.MethodPost, target, strings.NewReader("text="+strings.Repeat("%61", maxReceiveTextFormBytes/3+1)))
	chunked.ContentLength = -1
	chunked.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	chunkedResponse := httptest.NewRecorder()
	server.active.mux.ServeHTTP(chunkedResponse, chunked)
	if chunkedResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked encoded oversize status=%d", chunkedResponse.Code)
	}
	if claims, commits := owner.snapshot(); claims != 0 || len(commits) != 0 {
		t.Fatalf("bad bodies claims=%d commits=%v", claims, commits)
	}
	response := do(t, http.MethodGet, target)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reusable GET = %d", response.StatusCode)
	}
}

func TestReceiveTextEncodedReaderStopsAtLiteralCapPlusOne(t *testing.T) {
	if maxReceiveTextFormBytes != 394240 {
		t.Fatalf("encoded cap=%d, want literal 394240", maxReceiveTextFormBytes)
	}
	owner := &receivedTextOwner{}
	target, _, server := startReceiveTextServer(t, owner)
	body := &countingTextBody{reader: strings.NewReader("text=" + strings.Repeat("%61", 200000))}
	request := httptest.NewRequest(http.MethodPost, target, body)
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.active.mux.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status=%d", response.Code)
	}
	if body.read != 394241 {
		t.Fatalf("encoded body read=%d, want literal 394241 bytes", body.read)
	}
	if claims, commits := owner.snapshot(); claims != 0 || len(commits) != 0 {
		t.Fatalf("oversize request claims=%d commits=%d", claims, len(commits))
	}
}

func TestReceiveTextRealFormEncodingCommitsExactNormalizedValueOnce(t *testing.T) {
	owner := &receivedTextOwner{}
	target, handle, server := startReceiveTextServer(t, owner)
	value := "\r\n<&/textarea><script>alert(1)</script>\rhttps://example.test/?a=1&b=2\n雪"
	body := url.Values{"text": {value}}.Encode()
	response, page := postReceivedText(t, target+"?text=ignored", "application/x-www-form-urlencoded; charset=UTF-8", body)
	if response.StatusCode != http.StatusOK || !strings.Contains(page, "Text received by this computer") ||
		strings.Contains(page, value) || strings.Contains(page, "<script") {
		t.Fatalf("receipt status=%d page=%q", response.StatusCode, page)
	}
	claims, commits := owner.snapshot()
	if claims != 1 || len(commits) != 1 || commits[0] != "\n<&/textarea><script>alert(1)</script>\nhttps://example.test/?a=1&b=2\n雪" {
		t.Fatalf("claims=%d commits=%q", claims, commits)
	}
	lockedRequest := httptest.NewRequest(http.MethodPost, target, strings.NewReader("text=second"))
	lockedResponse := httptest.NewRecorder()
	server.active.mux.ServeHTTP(lockedResponse, lockedRequest)
	if lockedResponse.Code != http.StatusLocked {
		t.Fatalf("second POST = %d", lockedResponse.Code)
	}
	select {
	case event := <-handle.Events:
		if event.Kind != transfer.ServerComplete {
			t.Fatalf("terminal = %s", event.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no terminal receipt")
	}
}

func TestReceiveTextNormalizedBoundaryAndChunkedEncodedExpansion(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, _ := startReceiveTextServer(t, owner)
	response, _ := postReceivedText(t, target, "application/x-www-form-urlencoded", "text="+strings.Repeat("x", 65537))
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("65,537 normalized bytes = %d", response.StatusCode)
	}
	// Percent expansion is three encoded bytes per accepted raw byte. A
	// request with no Content-Length must still obey the same bounded reader.
	body := "text=" + strings.Repeat("%41", 65536)
	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err = (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("65,536 decoded chunked bytes = %d", response.StatusCode)
	}
	claims, commits := owner.snapshot()
	if claims != 1 || len(commits) != 1 || commits[0] != strings.Repeat("A", 65536) {
		t.Fatalf("boundary commit claims=%d count=%d len=%d", claims, len(commits), len(strings.Join(commits, "")))
	}
}

func TestReceiveTextOneBodyReaderAndInterruptedBodyDoNotClaim(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, server := startReceiveTextServer(t, owner)
	blocked := &gatedTextBody{entered: make(chan struct{}), release: make(chan struct{}), content: "text=%ZZ"}
	first := httptest.NewRequest(http.MethodPost, target, blocked)
	first.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	firstResponse := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { server.active.mux.ServeHTTP(firstResponse, first); close(done) }()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("first reader did not enter")
	}
	second := httptest.NewRequest(http.MethodPost, target, strings.NewReader("text=%ZZ"))
	second.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondResponse := httptest.NewRecorder()
	server.active.mux.ServeHTTP(secondResponse, second)
	if secondResponse.Code != http.StatusLocked {
		t.Fatalf("competing POST = %d", secondResponse.Code)
	}
	close(blocked.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first reader did not finish")
	}
	if firstResponse.Code != http.StatusBadRequest {
		t.Fatalf("first refusal = %d", firstResponse.Code)
	}
	if claims, commits := owner.snapshot(); claims != 0 || len(commits) != 0 {
		t.Fatalf("refused requests claims=%d commits=%v", claims, commits)
	}

	interrupted := httptest.NewRequest(http.MethodPost, target, &failingTextBody{})
	interrupted.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	interruptedResponse := httptest.NewRecorder()
	server.active.mux.ServeHTTP(interruptedResponse, interrupted)
	if interruptedResponse.Code != http.StatusBadRequest {
		t.Fatalf("interrupted body = %d", interruptedResponse.Code)
	}
	if claims, _ := owner.snapshot(); claims != 0 {
		t.Fatalf("interrupted body claimed %d times", claims)
	}
}

func TestReceiveTextRetiredWhileBodyIsBlockedCannotCommit(t *testing.T) {
	owner := &receivedTextOwner{}
	target, _, server := startReceiveTextServer(t, owner)
	blocked := &gatedTextBody{entered: make(chan struct{}), release: make(chan struct{}), content: "text=stale"}
	request := httptest.NewRequest(http.MethodPost, target, blocked)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recovered := recover(); recovered != nil && recovered != http.ErrAbortHandler {
				panic(recovered)
			}
		}()
		server.active.mux.ServeHTTP(response, request)
	}()
	select {
	case <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("old request did not enter body read")
	}
	// Stop/recovery retires the run by cancelling this context before a
	// replacement listener is advertised. Release an already-entered old body.
	server.active.cancel()
	close(blocked.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retired body did not return")
	}
	if response.Code != http.StatusBadRequest && response.Code != http.StatusNotFound {
		t.Fatalf("retired request status=%d", response.Code)
	}
	if claims, commits := owner.snapshot(); claims != 0 || len(commits) != 0 {
		t.Fatalf("retired request claims=%d commits=%q", claims, commits)
	}
}

type failingTextBody struct{}

type claimRequestMarker struct{}

func (*failingTextBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (*failingTextBody) Close() error             { return nil }

func TestReceiveTextCancelledDuringAuthorizationCannotCommit(t *testing.T) {
	requestCtx, cancel := context.WithCancel(context.WithValue(context.Background(), claimRequestMarker{}, "request"))
	owner := &receivedTextOwner{claim: func(ctx context.Context) error {
		if ctx.Value(claimRequestMarker{}) != "request" {
			t.Fatal("claim context did not inherit the HTTP request")
		}
		cancel()
		if ctx.Err() != context.Canceled {
			t.Fatalf("claim context did not synchronously inherit request cancellation: %v", ctx.Err())
		}
		return nil
	}}
	target, _, server := startReceiveTextServer(t, owner)
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("text=ready")).WithContext(requestCtx)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	func() {
		// A defective implementation can reach net/http finalization with
		// this deliberately cancelled request. The real server recovers this
		// sentinel; keep the assertion focused on whether it committed.
		defer func() {
			if recovered := recover(); recovered != nil && recovered != http.ErrAbortHandler {
				panic(recovered)
			}
		}()
		server.active.mux.ServeHTTP(response, request)
	}()
	if response.Code != http.StatusNotFound {
		t.Fatalf("cancelled post = %d", response.Code)
	}
	claims, commits := owner.snapshot()
	if claims != 1 || len(commits) != 0 {
		t.Fatalf("cancelled authorization claims=%d commits=%v", claims, commits)
	}
}

func TestReceiveTextCommittedReceiptSurvivesPhoneAcknowledgementWriteFailure(t *testing.T) {
	owner := &receivedTextOwner{}
	server := newTestServer(t, nil)
	gate := installFinalWriteGate(server, 1, "write-error")
	t.Cleanup(gate.release)
	handle, err := server.Start(context.Background(), transfer.ServerStartRequest{
		SessionID: testSession, Token: testToken, ReceiveText: true,
	}, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	target := baseURL(handle.Port) + "/receive-text/" + string(testToken)
	result := make(chan error, 1)
	go func() {
		request, err := http.NewRequest(http.MethodPost, target, strings.NewReader("text=committed"))
		if err != nil {
			result <- err
			return
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
		if err == nil {
			_, err = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		result <- err
	}()
	_ = awaitFinalWrite(t, gate)
	if claims, commits := owner.snapshot(); claims != 1 || len(commits) != 1 || commits[0] != "committed" {
		t.Fatalf("before failed acknowledgement claims=%d commits=%q", claims, commits)
	}
	gate.release()
	select {
	case event := <-handle.Events:
		if event.Kind != transfer.ServerComplete {
			t.Fatalf("failed acknowledgement demoted receipt: %s", event.Kind)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no desktop receipt")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("phone saw complete acknowledgement despite write failure")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("phone request did not settle")
	}
}
