package server

import (
	"context"
	stdhtml "html"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
	"golang.org/x/net/html"
)

func startTextTestServer(t *testing.T, value string, authorizer transfer.ClaimAuthorizer, payloads PayloadPort) (transfer.ServerHandle, *Server) {
	t.Helper()
	s := newTestServer(t, payloads)
	handle, err := s.Start(context.Background(), transfer.ServerStartRequest{SessionID: testSession, Token: testToken, Text: &value}, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return handle, s
}

func textAreaValue(t *testing.T, page string) string {
	t.Helper()
	node, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var walk func(*html.Node) (string, bool)
	walk = func(n *html.Node) (string, bool) {
		if n.Type == html.ElementNode && n.Data == "textarea" {
			if n.FirstChild == nil {
				return "", true
			}
			return n.FirstChild.Data, true
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if value, ok := walk(child); ok {
				return value, true
			}
		}
		return "", false
	}
	value, found := walk(node)
	if !found {
		t.Fatal("reveal has no textarea")
	}
	return value
}

func TestTextPreviewAndOneShotRevealExactDOM(t *testing.T) {
	value := "\n<&/textarea><script>alert(1)</script>\nhttps://example.test/?a=1&b=2"
	payloads := &stubPayloads{}
	authorizer := &stubAuthorizer{}
	handle, _ := startTextTestServer(t, value, authorizer, payloads)
	url := baseURL(handle.Port) + "/text/" + string(testToken)
	preview := do(t, http.MethodGet, url)
	previewBody, err := io.ReadAll(preview.Body)
	if err != nil {
		t.Fatal(err)
	}
	if preview.StatusCode != http.StatusOK || strings.Contains(stdhtml.UnescapeString(string(previewBody)), value) || strings.Contains(stdhtml.UnescapeString(string(previewBody)), "https://example.test/?a=1&b=2") || authorizer.calls.Load() != 0 {
		t.Fatalf("preview disclosed or claimed: status=%d body=%q", preview.StatusCode, previewBody)
	}
	if preview.Header.Get("Cache-Control") != "no-store" || preview.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("preview lacks receiver security headers")
	}
	for _, target := range []string{baseURL(handle.Port) + "/text/wrong", url + "/", baseURL(handle.Port) + "/download/" + string(testToken)} {
		response := do(t, http.MethodPost, target)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("wrong target %s = %d", target, response.StatusCode)
		}
	}
	response := do(t, http.MethodPost, url)
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || textAreaValue(t, string(body)) != value {
		t.Fatalf("reveal status=%d textarea=%q", response.StatusCode, textAreaValue(t, string(body)))
	}
	if strings.Contains(string(body), "<script>alert(1)</script>") || strings.Contains(string(body), `<a href=`) {
		t.Fatal("text became active markup")
	}
	if response.ContentLength != int64(len(body)) {
		t.Fatalf("content length=%d body=%d", response.ContentLength, len(body))
	}
	if payloads.calls.Load() != 0 || authorizer.calls.Load() != 1 {
		t.Fatalf("payload calls=%d claims=%d", payloads.calls.Load(), authorizer.calls.Load())
	}
	select {
	case event := <-handle.Events:
		if event.Kind != transfer.ServerComplete && event.Kind != transfer.ServerProgress {
			t.Fatalf("event=%v", event.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no completion event")
	}
}

func TestTextWrongMethodsAndRoutesDoNotReserveClaim(t *testing.T) {
	authorizer := &stubAuthorizer{}
	handle, _ := startTextTestServer(t, "method guard", authorizer, nil)
	url := baseURL(handle.Port) + "/text/" + string(testToken)
	for _, method := range []string{http.MethodHead, http.MethodPut, http.MethodOptions, http.MethodDelete} {
		response := do(t, method, url)
		if response.StatusCode != http.StatusNotFound || response.Header.Get("Allow") != "" || response.Header.Get("Location") != "" {
			t.Fatalf("%s refusal status=%d", method, response.StatusCode)
		}
	}
	for _, target := range []string{url + "/", baseURL(handle.Port) + "/text/wrong", baseURL(handle.Port) + "/download/" + string(testToken)} {
		response := do(t, http.MethodPost, target)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("wrong route %s status=%d", target, response.StatusCode)
		}
	}
	if authorizer.calls.Load() != 0 {
		t.Fatalf("rejected requests authorized %d claims", authorizer.calls.Load())
	}
	preview := do(t, http.MethodGet, url)
	if preview.StatusCode != http.StatusOK {
		t.Fatalf("valid GET after refusals=%d", preview.StatusCode)
	}
	post := do(t, http.MethodPost, url)
	if post.StatusCode != http.StatusOK || authorizer.calls.Load() != 1 {
		t.Fatalf("valid POST after refusals=%d claims=%d", post.StatusCode, authorizer.calls.Load())
	}
}

func TestTextRefusedClaimNeverWritesBody(t *testing.T) {
	value := "secret payload"
	handle, _ := startTextTestServer(t, value, refusingAuthorizer(transfer.NewError(transfer.ErrCancelled, "refused")), nil)
	response := do(t, http.MethodPost, baseURL(handle.Port)+"/text/"+string(testToken))
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotFound || len(body) != 0 {
		t.Fatalf("refusal=%d body=%q", response.StatusCode, body)
	}
}

func TestTextConcurrentClaimHasOneAuthorizer(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	authorizer := &stubAuthorizer{authorize: func(context.Context, transfer.SessionID) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}}
	handle, _ := startTextTestServer(t, "one receiver", authorizer, nil)
	url := baseURL(handle.Port) + "/text/" + string(testToken)
	first := make(chan *http.Response, 1)
	go func() { response, _ := testClient().Post(url, "", nil); first <- response }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("claim did not reach authorizer")
	}
	second := do(t, http.MethodPost, url)
	if second.StatusCode != http.StatusLocked {
		t.Fatalf("losing claim status=%d", second.StatusCode)
	}
	close(release)
	response := <-first
	if response == nil {
		t.Fatal("winning claim failed")
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("winning claim status=%d", response.StatusCode)
	}
	if authorizer.calls.Load() != 1 {
		t.Fatalf("authorizer calls=%d", authorizer.calls.Load())
	}
}

func TestTextStartCopiesInputAndRefusesMixedPayload(t *testing.T) {
	value := "original"
	s := newTestServer(t, nil)
	handle, err := s.Start(context.Background(), transfer.ServerStartRequest{SessionID: testSession, Token: testToken, Text: &value}, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	// A later change to the caller-owned string cannot rewrite the response.
	value = "changed"
	response := do(t, http.MethodPost, baseURL(handle.Port)+"/text/"+string(testToken))
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := textAreaValue(t, string(body)); got != "original" {
		t.Fatalf("run retained %q", got)
	}

	invalid := "valid"
	other := newTestServer(t, nil)
	_, err = other.Start(context.Background(), transfer.ServerStartRequest{SessionID: testSession, Token: testToken, Text: &invalid, Item: transfer.StagedItem{Name: "mixed"}}, &stubAuthorizer{})
	if err == nil {
		t.Fatal("mixed text and staged item was accepted")
	}
}

func TestTextCompletionWaitsForFullHTTPFinalization(t *testing.T) {
	for _, outcome := range []string{"success", "short-write"} {
		t.Run(outcome, func(t *testing.T) {
			value := "final text"
			s := newTestServer(t, nil)
			gate := installFinalWriteGate(s, 2, outcome)
			t.Cleanup(gate.release)
			handle, err := s.Start(context.Background(), transfer.ServerStartRequest{SessionID: testSession, Token: testToken, Text: &value}, &stubAuthorizer{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Stop() })
			response := do(t, http.MethodPost, baseURL(handle.Port)+"/text/"+string(testToken))
			wire := awaitFinalWrite(t, gate)
			if !strings.Contains(string(wire), "final text") {
				t.Fatal("gate did not intercept text body")
			}
			assertNoEvents(t, handle.Events)
			gate.release()
			terminal := awaitEvent(t, handle.Events, 5*time.Second)
			if outcome == "success" {
				if terminal.Kind != transfer.ServerComplete {
					t.Fatalf("terminal=%s", terminal.Kind)
				}
				if _, err := io.ReadAll(response.Body); err != nil {
					t.Fatal(err)
				}
			} else if terminal.Kind != transfer.ServerFailed {
				t.Fatalf("short final write terminal=%s", terminal.Kind)
			}
		})
	}
}

func TestTextRequestDisconnectAfterAuthorizationSettlesFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	authorizer := &stubAuthorizer{authorize: func(context.Context, transfer.SessionID) error { cancel(); return nil }}
	handle, server := startTextTestServer(t, "hidden", authorizer, nil)
	request := httptest.NewRequest(http.MethodPost, baseURL(handle.Port)+"/text/"+string(testToken), nil).WithContext(ctx)
	request.SetPathValue("token", string(testToken))
	writer := httptest.NewRecorder()
	// A direct handler call makes request cancellation occur exactly when
	// authorization returns, without depending on TCP close observation.
	server.active.textRoute(writer, request)
	if writer.Code != http.StatusNotFound || writer.Body.Len() != 0 {
		t.Fatalf("refusal=%d body=%q", writer.Code, writer.Body.String())
	}
	event := awaitEvent(t, handle.Events, 5*time.Second)
	if event.Kind != transfer.ServerFailed || transfer.ErrorCodeOf(event.Err) != transfer.ErrTransferFailed {
		t.Fatalf("disconnect event = %#v", event)
	}
}
