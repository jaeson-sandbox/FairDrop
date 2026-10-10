package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"fairdrop/internal/transfer"
)

func TestLandingVisitsDoNotClaimOrRead(t *testing.T) {
	t.Parallel()
	payloads := &stubPayloads{}
	authorizer := &stubAuthorizer{}
	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)
	url := downloadURL(handle.Port, string(testToken))

	var visits sync.WaitGroup
	for range 12 {
		visits.Add(1)
		go func() {
			defer visits.Done()
			response := do(t, http.MethodGet, url)
			body := string(readBody(t, response))
			if response.StatusCode != http.StatusOK || !strings.Contains(body, "quarterly report.pdf") || !strings.Contains(body, `<form method="post" action="">`) {
				t.Errorf("GET did not render the real metadata and POST form: status %d, body %q", response.StatusCode, body)
			}
		}()
	}
	visits.Wait()
	if authorizer.calls.Load() != 0 || payloads.calls.Load() != 0 || server.active.claimed.Load() {
		t.Fatal("GET reserved, authorized or prepared the payload")
	}
	assertNoEvents(t, handle.Events)
	response := do(t, http.MethodPost, url)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST after repeated visits = %d, want 200", response.StatusCode)
	}
	readBody(t, response)
	if authorizer.calls.Load() != 1 || payloads.calls.Load() != 1 {
		t.Fatal("POST did not authorize and prepare exactly once")
	}
}

func TestLandingEscapesMetadataAndCarriesPagePolicies(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, &stubPayloads{})
	request := startRequest()
	request.Item.Name = `<img src=x onerror=alert(1)>שלום` + strings.Repeat("長", 240)
	request.Item.Path = "/private/secret/source"
	handle, err := server.Start(context.Background(), request, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	body := string(readBody(t, response))
	if response.StatusCode != http.StatusOK || !strings.Contains(body, "&lt;img") || strings.Contains(body, "<img") || strings.Contains(body, request.Item.Path) || strings.Contains(body, "alert(1)>") {
		t.Fatalf("metadata was not safely escaped: status %d, body %q", response.StatusCode, body)
	}
	for name, want := range map[string]string{
		"Content-Type": "text/html; charset=utf-8", "Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff",
	} {
		if got := response.Header.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if csp := response.Header.Get("Content-Security-Policy"); csp != "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'" {
		t.Fatalf("CSP = %q, want script/resource refusal and same-origin form", csp)
	}
	if strings.Contains(body, "<script") || strings.Contains(body, "<link") || !strings.Contains(body, "overflow-wrap:anywhere") {
		t.Fatal("page requested resources or lacks long-name wrapping")
	}
}

func TestGetDuringReservedPostIsLockedWithoutAnotherClaim(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	authorizer := &stubAuthorizer{authorize: func(context.Context, transfer.SessionID) error {
		close(entered)
		<-release
		return nil
	}}
	payloads := &stubPayloads{}
	server := newTestServer(t, payloads)
	handle := startTestServer(t, server, authorizer)
	url := downloadURL(handle.Port, string(testToken))
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		response, err := testClient().Post(url, "", nil)
		if err == nil {
			_ = response.Body.Close()
		}
	}()
	<-entered
	response := do(t, http.MethodGet, url)
	if response.StatusCode != http.StatusLocked {
		t.Fatalf("GET during reserved POST = %d, want 423", response.StatusCode)
	}
	readBody(t, response)
	if authorizer.calls.Load() != 1 || payloads.calls.Load() != 0 {
		t.Fatal("reserved GET caused another claim or payload preparation")
	}
	close(release)
	<-finished
}

func TestFolderLandingExplainsZIPWithoutPromisingArchiveSize(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, &stubPayloads{})
	request := startRequest()
	request.Item.Kind = transfer.ItemDirectory
	handle, err := server.Start(context.Background(), request, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	body := string(readBody(t, response))
	if !strings.Contains(body, "Downloads as a ZIP") || strings.Contains(body, "File ·") || strings.Contains(body, "12 bytes") {
		t.Fatalf("folder page = %q", body)
	}
}

func TestCollectionLandingShowsCountAndLogicalSizeWithoutPaths(t *testing.T) {
	payloads := &stubPayloads{}
	server := newTestServer(t, payloads)
	request := startRequest()
	request.Item = transfer.StagedItem{
		Kind: transfer.ItemCollection, Name: "2 items", LogicalSize: 42,
		Collection: &transfer.StagedCollection{Members: []transfer.StagedItem{
			{Path: "/private/first", Kind: transfer.ItemFile},
			{Path: "/private/second", Kind: transfer.ItemFile},
		}},
	}
	handle, err := server.Start(context.Background(), request, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	request.Item.Collection.Members[0].Path = "/mutated/after/start"
	if got := server.active.item.Collection.Members[0].Path; got != "/private/first" {
		t.Fatalf("caller mutated server-owned collection: %q", got)
	}
	t.Cleanup(func() { _ = server.Stop() })
	response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
	body := string(readBody(t, response))
	if response.StatusCode != http.StatusOK || !strings.Contains(body, "2 items") || !strings.Contains(body, "42 bytes") || !strings.Contains(body, "Downloads as a ZIP") || strings.Contains(body, "/private/") {
		t.Fatalf("collection page status %d, body %q", response.StatusCode, body)
	}
	if payloads.calls.Load() != 0 {
		t.Fatal("GET prepared collection payload")
	}
}

func TestCollectionPayloadAdapterCannotMutateServerMetadata(t *testing.T) {
	payloads := &stubPayloads{prepare: func(_ context.Context, item transfer.StagedItem) (PreparedPayload, error) {
		item.Collection.Members[0].Path = "/mutated/by/payload"
		return &stubPayload{name: "FairDrop.zip", known: false}, nil
	}}
	server := newTestServer(t, payloads)
	request := startRequest()
	request.Item = transfer.StagedItem{Kind: transfer.ItemCollection, Name: "2 items", Collection: &transfer.StagedCollection{Members: []transfer.StagedItem{
		{Path: "/private/first", Kind: transfer.ItemFile}, {Path: "/private/second", Kind: transfer.ItemFile},
	}}}
	handle, err := server.Start(context.Background(), request, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Stop() })
	owned := server.active
	response := do(t, http.MethodPost, downloadURL(handle.Port, string(testToken)))
	readBody(t, response)
	if got := owned.item.Collection.Members[0].Path; got != "/private/first" {
		t.Fatalf("payload adapter mutated server metadata: %q", got)
	}
}

func TestFileLandingRendersStagedSize(t *testing.T) {
	for _, test := range []struct {
		name string
		size int64
		want string
	}{
		{"empty", 0, "File · 0 bytes"},
		{"singular", 1, "File · 1 byte"},
		{"logical bytes", 12, "File · 12 bytes"},
		{"kilobyte boundary", 1000, "File · 1.0 KB"},
		{"megabyte boundary", 1_000_000, "File · 1.0 MB"},
		{"gigabyte boundary", 1_000_000_000, "File · 1.0 GB"},
		{"terabyte boundary", 1_000_000_000_000, "File · 1.0 TB"},
		{"unavailable", -1, "File · Size unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t, &stubPayloads{})
			request := startRequest()
			request.Item.LogicalSize = test.size
			handle, err := server.Start(context.Background(), request, &stubAuthorizer{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Stop() })
			response := do(t, http.MethodGet, downloadURL(handle.Port, string(testToken)))
			body := string(readBody(t, response))
			if response.StatusCode != http.StatusOK || !strings.Contains(body, test.want) {
				t.Fatalf("staged size %d rendered without %q: status=%d body=%q", test.size, test.want, response.StatusCode, body)
			}
		})
	}
}
