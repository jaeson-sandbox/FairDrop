package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// Covers the server half of the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-phone-to-desktop-receiving.md. Every
// case is a real HTTP request against a real listener, a real sink and a real
// directory; expectations are literals written out at the assertion site.

// --- GET and the generic rejections ----------------------------------------

func TestTheUploadPageIsScriptFreeAndLeavesTheSessionWaiting(t *testing.T) {
	rig := startReceiveRig(t)

	for round := 0; round < 2; round++ {
		response := do(t, http.MethodGet, rig.uploadURL())
		body := readBody(t, response)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET status = %d, want 200", response.StatusCode)
		}
		page := string(body)
		for _, want := range []string{
			`<form method="post" action="" enctype="multipart/form-data">`,
			`<input id="files" type="file" name="files" multiple required>`,
			`<button type="submit">Upload</button>`,
			`<title>Send files with FairDrop</title>`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("the upload page lacks %s", want)
			}
		}
		for _, forbidden := range []string{"<script", "javascript:", "http://", "https://", " src=", "onsubmit", "onchange", "<iframe", "<link", "<img", "<base", rig.dir, string(testToken), string(testSession)} {
			if strings.Contains(page, forbidden) {
				t.Errorf("the upload page contains %q", forbidden)
			}
		}
		header := response.Header
		for name, want := range map[string]string{
			"Content-Type":            "text/html; charset=utf-8",
			"Cache-Control":           "no-store",
			"Referrer-Policy":         "no-referrer",
			"X-Content-Type-Options":  "nosniff",
			"Content-Security-Policy": "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'",
			"Content-Length":          fmt.Sprint(len(body)),
		} {
			if got := header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
	}

	if calls := rig.auth.calls.Load(); calls != 0 {
		t.Errorf("GET called the authorizer %d times, want 0", calls)
	}
	assertNoEvents(t, rig.handle.Events)
	rig.assertNothingWritten()
}

func TestEveryRouteOrMethodThatIsNotTheExactUploadIsTheGenericNotFound(t *testing.T) {
	rig := startReceiveRig(t)
	token := string(testToken)
	other := strings.Repeat("0", len(token))

	for _, testCase := range []struct{ method, path string }{
		{"GET", "/upload/" + other},
		{"POST", "/upload/" + other},
		{"GET", "/upload/" + token[:len(token)-1]},
		{"GET", "/upload/"},
		{"GET", "/upload"},
		{"GET", "/"},
		{"GET", "/download/" + token},
		{"POST", "/download/" + token},
		{"GET", "/upload/" + token + "/"},
		{"GET", "/upload/" + token + "/extra"},
		{"GET", "//upload/" + token},
		{"GET", "/upload/./" + token},
		{"GET", "/upload/../upload/" + token},
		{"GET", "/Upload/" + token},
		{"HEAD", "/upload/" + token},
		{"PUT", "/upload/" + token},
		{"DELETE", "/upload/" + token},
		{"PATCH", "/upload/" + token},
		{"OPTIONS", "/upload/" + token},
	} {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			request, err := http.NewRequest(testCase.method, rig.base+testCase.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := testClient().Do(request)
			if err != nil {
				t.Fatalf("request error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want the generic 404", response.StatusCode)
			}
			assertNoDisclosure(t, response, readBody(t, response))
			if location := response.Header.Get("Location"); location != "" {
				t.Fatalf("the rejection redirected to %q", location)
			}
		})
	}

	if calls := rig.auth.calls.Load(); calls != 0 {
		t.Errorf("rejections called the authorizer %d times", calls)
	}
	rig.assertNothingWritten()
}

// A send session's server never answers the upload route, and a receive
// session's never answers the download route: the URL's route is the session
// kind, nothing else.
func TestASendServerDoesNotAnswerTheUploadRoute(t *testing.T) {
	server := newTestServer(t, payloadsReturning(&stubPayload{name: "x", known: true}))
	handle := startTestServer(t, server, &stubAuthorizer{})

	for _, method := range []string{"GET", "POST"} {
		response := do(t, method, baseURL(handle.Port)+"/upload/"+string(testToken))
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s /upload on a send server = %d, want 404", method, response.StatusCode)
		}
	}
}

// --- A successful upload ---------------------------------------------------

func TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames(t *testing.T) {
	rig := startReceiveRig(t)
	// A pre-existing file in the destination must be the same afterwards.
	if err := os.WriteFile(filepath.Join(rig.dir, "existing.txt"), []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}

	photoA := strings.Repeat("A", 100_000)
	photoB := strings.Repeat("B", 70_000)
	body := multipartBody(
		fieldPart("caption", strings.Repeat("c", 5000)),
		uploadFile("IMG_0001.jpg", photoA),
		uploadFile("IMG_0001.jpg", photoB),
		uploadFile(`..\..\evil\dropper.sh`, "#!/bin/sh\necho owned\n"),
	)
	response, page := rig.post(body)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", response.StatusCode)
	}
	text := string(page)
	for _, want := range []string{"Upload complete", "3 files saved to this computer."} {
		if !strings.Contains(text, want) {
			t.Errorf("the result page lacks %q", want)
		}
	}
	for _, leak := range []string{rig.dir, filepath.Base(rig.dir), string(testToken), "IMG_0001", "dropper", "FairDrop 2"} {
		if strings.Contains(text, leak) {
			t.Errorf("the result page disclosed %q", leak)
		}
	}
	if got := response.Header.Get("Content-Security-Policy"); !strings.Contains(got, "form-action 'self'") {
		t.Errorf("the result page CSP = %q, want form-action 'self'", got)
	}

	events, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal event = %s (%v), want complete", terminal.Kind, terminal.Err)
	}
	if terminal.Progress == nil || terminal.Progress.BytesSent != int64(len(body)) || terminal.Progress.TotalBytes != int64(len(body)) || !terminal.Progress.TotalKnown {
		t.Errorf("terminal progress = %+v, want %d of %d bytes", terminal.Progress, len(body), len(body))
	}
	savedProgress := 0
	for _, event := range events {
		if event.Kind == transfer.ServerProgress && event.Receive != nil && event.Receive.FilesSaved > savedProgress {
			savedProgress = event.Receive.FilesSaved
		}
	}
	if savedProgress != 3 {
		t.Errorf("the highest FilesSaved on a progress event was %d, want 3", savedProgress)
	}
	if calls := rig.auth.claimedSessions(); len(calls) != 1 || calls[0] != testSession {
		t.Errorf("authorizer saw %v, want exactly the session once", calls)
	}

	if got := listNames(t, rig.dir); len(got) != 2 || got[1] != "existing.txt" || !regexp.MustCompile(`^FairDrop \d{4}-\d{2}-\d{2} \d{2}\.\d{2}( \(\d+\))?$`).MatchString(got[0]) {
		t.Fatalf("destination holds %v, want existing.txt and one time-named subfolder", got)
	}
	if got := readText(t, filepath.Join(rig.dir, "existing.txt")); got != "ORIGINAL" {
		t.Fatalf("a pre-existing file was changed: %q", got)
	}
	sub := rig.subfolder()
	want := []string{"IMG_0001 (1).jpg", "IMG_0001.jpg", "dropper.sh"}
	if got := listNames(t, sub); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("subfolder holds %v, want %v and no temporary file", got, want)
	}
	if got := readText(t, filepath.Join(sub, "IMG_0001.jpg")); got != photoA {
		t.Error("IMG_0001.jpg differs from the first uploaded file")
	}
	if got := readText(t, filepath.Join(sub, "IMG_0001 (1).jpg")); got != photoB {
		t.Error("IMG_0001 (1).jpg differs from the second uploaded file")
	}
	if got := readText(t, filepath.Join(sub, "dropper.sh")); got != "#!/bin/sh\necho owned\n" {
		t.Error("dropper.sh differs from the third uploaded file")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(sub, "dropper.sh"))
		if err != nil || info.Mode().Perm()&0o111 != 0 {
			t.Errorf("a received script is executable: %v %v", info, err)
		}
	}
	if snapshot := rig.dest.Snapshot(); snapshot.FilesSaved != 3 || !snapshot.SubfolderExists {
		t.Errorf("destination snapshot = %+v, want 3 files and a subfolder", snapshot)
	}
	if got := listNames(t, filepath.Dir(rig.dir)); len(got) != 1 {
		t.Errorf("something was written outside the destination: %v", got)
	}
}

func TestThereIsNoCapOnTheNumberOfFiles(t *testing.T) {
	rig := startReceiveRig(t)
	parts := make([]testPart, 0, 300)
	for index := 0; index < 300; index++ {
		parts = append(parts, uploadFile(fmt.Sprintf("photo-%03d.jpg", index), fmt.Sprintf("body %d", index)))
	}
	response, page := rig.post(multipartBody(parts...))
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), "300 files saved to this computer.") {
		t.Fatalf("status %d, page %q, want 300 files saved", response.StatusCode, page)
	}
	if got := len(listNames(t, rig.subfolder())); got != 300 {
		t.Fatalf("subfolder holds %d files, want 300", got)
	}
}

func TestPartsOutsideTheFilesFieldAreDiscardedNeverWritten(t *testing.T) {
	rig := startReceiveRig(t)
	notUploadField := "decoy.txt"
	emptyName := ""
	body := multipartBody(
		fieldPart("caption", strings.Repeat("c", 1<<20)),
		testPart{field: "other", filename: &notUploadField, content: "DECOY"},
		testPart{field: "files", filename: &emptyName, content: ""},
		uploadFile("real.txt", "REAL"),
	)
	response, page := rig.post(body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), "1 file saved to this computer.") {
		t.Fatalf("status %d, page %q, want exactly 1 file saved", response.StatusCode, page)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "real.txt" {
		t.Fatalf("subfolder holds %v, want only real.txt", got)
	}
	_, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerComplete || terminal.Progress.BytesSent != int64(len(body)) {
		t.Fatalf("terminal = %s with %+v, want complete having read all %d bytes", terminal.Kind, terminal.Progress, len(body))
	}
}

// --- Requests that must leave the session waiting ---------------------------

func TestRejectedRequestsWriteNothingAndLeaveTheSessionWaiting(t *testing.T) {
	emptyName := ""
	cases := []struct {
		name        string
		status      int
		heading     string
		contentType string
		body        func() []byte
		chunked     bool
	}{
		{"no declared length (chunked)", http.StatusLengthRequired, "Upload not accepted", multipartType, func() []byte { return multipartBody(uploadFile("a.txt", "a")) }, true},
		{"not multipart", http.StatusBadRequest, "Nothing to upload", "application/x-www-form-urlencoded", func() []byte { return []byte("files=a.txt") }, false},
		{"multipart without a boundary", http.StatusBadRequest, "Nothing to upload", "multipart/form-data", func() []byte { return multipartBody(uploadFile("a.txt", "a")) }, false},
		{"multipart with no parts", http.StatusBadRequest, "Nothing to upload", multipartType, func() []byte { return multipartBody() }, false},
		{"multipart with only a text field", http.StatusBadRequest, "Nothing to upload", multipartType, func() []byte { return multipartBody(fieldPart("caption", "hello")) }, false},
		{"multipart whose only file part is empty-named", http.StatusBadRequest, "Nothing to upload", multipartType, func() []byte {
			return multipartBody(testPart{field: "files", filename: &emptyName, content: ""})
		}, false},
		{"garbage under a multipart type", http.StatusBadRequest, "Nothing to upload", multipartType, func() []byte { return []byte("this is not multipart at all") }, false},
		{"an empty body", http.StatusBadRequest, "Nothing to upload", multipartType, func() []byte { return nil }, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rig := startReceiveRig(t)
			request, err := http.NewRequest(http.MethodPost, rig.uploadURL(), io.NopCloser(bytes.NewReader(testCase.body())))
			if err != nil {
				t.Fatal(err)
			}
			if !testCase.chunked {
				request.Body = io.NopCloser(bytes.NewReader(testCase.body()))
				request.ContentLength = int64(len(testCase.body()))
				request.GetBody = nil
				if request.ContentLength == 0 {
					request.Body = http.NoBody
				}
			} else {
				request.ContentLength = -1
			}
			request.Header.Set("Content-Type", testCase.contentType)
			response, err := testClient().Do(request)
			if err != nil {
				t.Fatalf("POST error = %v", err)
			}
			page := readBody(t, response)
			_ = response.Body.Close()

			if response.StatusCode != testCase.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, testCase.status)
			}
			if !strings.Contains(string(page), testCase.heading) || !strings.Contains(string(page), `<input id="files" type="file" name="files" multiple required>`) {
				t.Errorf("the page lacks the %q heading or the form to try again: %s", testCase.heading, page)
			}
			if got := response.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q", got)
			}
			rig.assertNothingWritten()
			if calls := rig.auth.calls.Load(); calls != 0 {
				t.Errorf("a rejected request called the authorizer %d times", calls)
			}
			assertNoEvents(t, rig.handle.Events)

			// Proof the session is still waiting: the same link still works.
			again, text := rig.post(multipartBody(uploadFile("retry.txt", "retried")))
			if again.StatusCode != http.StatusOK || !strings.Contains(string(text), "1 file saved to this computer.") {
				t.Fatalf("the retry after a rejection = %d %q, want it accepted", again.StatusCode, text)
			}
			if got := readText(t, filepath.Join(rig.subfolder(), "retry.txt")); got != "retried" {
				t.Fatalf("retry.txt = %q", got)
			}
		})
	}
}

// A body whose declared length is zero is declared, just empty: it has no file
// part, which is a 400, not a 411.
func TestAZeroLengthBodyIsAnEmptyUploadNotAMissingLength(t *testing.T) {
	rig := startReceiveRig(t)
	conn := rig.rawConn(0)
	response, _ := readResponse(t, conn)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("Content-Length: 0 = %d, want 400", response.StatusCode)
	}
	conn2, err := net.Dial("tcp", hostPort(rig.handle.Port))
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()
	fmt.Fprintf(conn2, "POST /upload/%s HTTP/1.1\r\nHost: x\r\nContent-Type: %s\r\nConnection: close\r\n\r\n", testToken, multipartType)
	response, _ = readResponse(t, conn2)
	if response.StatusCode != http.StatusLengthRequired {
		t.Fatalf("no Content-Length header at all = %d, want 411", response.StatusCode)
	}
	rig.assertNothingWritten()
}

func TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting(t *testing.T) {
	var spy *spyDestination
	refuse := true
	var mu sync.Mutex
	rig := startReceiveRig(t, withDestination(func(real transfer.ReceiveDestination) transfer.ReceiveDestination {
		spy = &spyDestination{ReceiveDestination: real, checkSpace: func(int64) error {
			mu.Lock()
			defer mu.Unlock()
			if refuse {
				return transfer.ErrInsufficientSpace
			}
			return nil
		}}
		return spy
	}))

	body := multipartBody(uploadFile("huge.mov", "tiny in fact"))
	response, page := rig.post(body)

	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", response.StatusCode)
	}
	if !strings.Contains(string(page), "Too large for this computer") {
		t.Errorf("the 413 page = %s, want the fixed too-large copy", page)
	}
	if got := spy.declared.Load(); got != int64(len(body)) {
		t.Errorf("CheckSpace saw %d, want the declared Content-Length %d", got, len(body))
	}
	notice := awaitNotice(t, rig.handle.Events)
	if notice.Notice != transfer.NoticeReceiveTooLarge || notice.SessionID != testSession {
		t.Errorf("notice = %+v, want receive_too_large for the session", notice)
	}
	rig.assertNothingWritten()
	if calls := rig.auth.calls.Load(); calls != 0 {
		t.Errorf("a space refusal called the authorizer %d times", calls)
	}
	if spy.saves.Load() != 0 {
		t.Error("a space refusal reached SaveFile")
	}

	// Waiting: with room, the same link accepts.
	mu.Lock()
	refuse = false
	mu.Unlock()
	again, text := rig.post(body)
	if again.StatusCode != http.StatusOK || !strings.Contains(string(text), "1 file saved to this computer.") {
		t.Fatalf("the retry with room = %d %q", again.StatusCode, text)
	}
}

// Unknown free space is not permission to write.
func TestAnUnanswerableSpaceCheckRefusesTheUploadToo(t *testing.T) {
	rig := startReceiveRig(t, withDestination(func(real transfer.ReceiveDestination) transfer.ReceiveDestination {
		return &spyDestination{ReceiveDestination: real, checkSpace: func(int64) error { return errors.New("test: statfs failed") }}
	}))
	response, _ := rig.post(multipartBody(uploadFile("a.txt", "a")))
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want a 413 refusal", response.StatusCode)
	}
	if notice := awaitNotice(t, rig.handle.Events); notice.Notice != transfer.NoticeReceiveTooLarge {
		t.Fatalf("notice = %+v", notice)
	}
	rig.assertNothingWritten()
}

// --- Claimed sessions --------------------------------------------------------

func TestASecondRequestWhileAnUploadIsInFlightGets423AndWritesNothing(t *testing.T) {
	rig := startReceiveRig(t)
	firstFile := strings.Repeat("F", 50_000)
	full := multipartBody(uploadFile("first.bin", firstFile), uploadFile("second.bin", "S"))
	cut := bytes.Index(full, []byte("second.bin"))
	conn := rig.rawConn(len(full))
	if _, err := conn.Write(full[:cut]); err != nil {
		t.Fatal(err)
	}

	// The claim happened once the first file part was seen; wait for it to save.
	deadline := time.Now().Add(10 * time.Second)
	for rig.dest.Snapshot().FilesSaved == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the first file was never saved")
		}
		time.Sleep(5 * time.Millisecond)
	}

	for _, method := range []string{"GET", "POST"} {
		response := do(t, method, rig.uploadURL())
		if response.StatusCode != http.StatusLocked {
			t.Errorf("%s during an upload = %d, want 423", method, response.StatusCode)
		}
		assertNoDisclosure(t, response, readBody(t, response))
	}
	second, _ := rig.post(multipartBody(uploadFile("intruder.bin", "INTRUDER")))
	if second.StatusCode != http.StatusLocked {
		t.Fatalf("a second multipart POST = %d, want 423", second.StatusCode)
	}
	if calls := rig.auth.calls.Load(); calls != 1 {
		t.Fatalf("the authorizer ran %d times, want exactly 1", calls)
	}

	if _, err := conn.Write(full[cut:]); err != nil {
		t.Fatal(err)
	}
	response, _ := readResponse(t, conn)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the first upload's status = %d, want 200", response.StatusCode)
	}
	if _, terminal := awaitTerminal(t, rig.handle.Events); terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal = %s (%v), want complete", terminal.Kind, terminal.Err)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "first.bin|second.bin" {
		t.Fatalf("subfolder holds %v, want only the first upload's files", got)
	}
}

// --- Failure keeps completed files and removes the partial one --------------

func TestADroppedConnectionKeepsCompletedFilesAndRemovesThePartial(t *testing.T) {
	rig := startReceiveRig(t)
	full := multipartBody(uploadFile("done.bin", "COMPLETE FILE"), uploadFile("cut.bin", strings.Repeat("P", 200_000)))
	cut := bytes.Index(full, []byte("cut.bin")) + 4000
	conn := rig.rawConn(len(full))
	if _, err := conn.Write(full[:cut]); err != nil {
		t.Fatal(err)
	}
	// Wait until the partial file is genuinely on disk before dropping.
	waitForPartial(t, rig)
	_ = conn.Close()

	events, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	if transfer.ErrorCodeOf(terminal.Err) != transfer.ErrTransferFailed {
		t.Errorf("terminal code = %q, want transfer_failed", transfer.ErrorCodeOf(terminal.Err))
	}
	if terminal.Progress == nil || terminal.Progress.BytesSent == 0 || terminal.Progress.BytesSent >= int64(len(full)) {
		t.Errorf("terminal progress = %+v, want some but not all of %d bytes", terminal.Progress, len(full))
	}
	_ = events

	sub := rig.subfolder()
	if got := listNames(t, sub); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want only the completed file", got)
	}
	if got := readText(t, filepath.Join(sub, "done.bin")); got != "COMPLETE FILE" {
		t.Errorf("the completed file = %q", got)
	}
	if got := rig.dest.Snapshot().FilesSaved; got != 1 {
		t.Errorf("FilesSaved = %d, want exactly 1", got)
	}
}

// waitForPartial blocks until a temporary file exists in the session subfolder.
func waitForPartial(t *testing.T, rig *receiveRig) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		for _, name := range listNames(t, rig.dir) {
			if strings.HasPrefix(name, "FairDrop ") {
				for _, inner := range listNames(t, filepath.Join(rig.dir, name)) {
					if strings.HasSuffix(inner, ".part") {
						return
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the partial file never appeared")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestADroppedConnectionBeforeAnyFileSavedLeavesNoSubfolder(t *testing.T) {
	rig := startReceiveRig(t)
	full := multipartBody(uploadFile("only.bin", strings.Repeat("P", 200_000)))
	conn := rig.rawConn(len(full))
	if _, err := conn.Write(full[:5000]); err != nil {
		t.Fatal(err)
	}
	waitForPartial(t, rig)
	_ = conn.Close()

	_, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	// The server leaves the empty-subfolder removal to the coordinator's Close.
	result, err := rig.dest.Close()
	if err != nil || result.FilesSaved != 0 || result.SubfolderExists {
		t.Fatalf("Close() = %+v, %v, want nothing saved and no subfolder", result, err)
	}
	rig.assertNothingWritten()
}

// A body longer than the connection declared is cut off at the declared length
// by net/http, so what arrives is a multipart stream that ends mid-part: an
// incomplete upload, with the file that was cut off removed.
func TestABodyPastTheDeclaredLengthIsCutOffAndReportedIncomplete(t *testing.T) {
	rig := startReceiveRig(t)
	full := multipartBody(uploadFile("done.bin", "COMPLETE FILE"), uploadFile("long.bin", strings.Repeat("L", 100_000)))
	declared := bytes.Index(full, []byte("long.bin")) + 30_000
	conn := rig.rawConn(declared)
	if _, err := conn.Write(full); err != nil {
		t.Fatal(err)
	}

	_, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want only the completed file", got)
	}
}

// net/http will not deliver bytes past Content-Length, so the server's own
// overrun guard is driven here through the handler directly, with a request
// whose body really is longer than it declares.
func TestTheServerItselfAbortsWhenTheBodyExceedsItsDeclaredLength(t *testing.T) {
	rig := startReceiveRig(t)
	rig.server.mu.Lock()
	active := rig.server.active
	rig.server.mu.Unlock()

	full := multipartBody(uploadFile("done.bin", "COMPLETE FILE"), uploadFile("long.bin", strings.Repeat("L", 100_000)))
	declared := int64(bytes.Index(full, []byte("long.bin")) + 30_000)
	request := httptest.NewRequest(http.MethodPost, "/upload/"+string(testToken), bytes.NewReader(full))
	request.SetPathValue("token", string(testToken))
	request.Header.Set("Content-Type", multipartType)
	request.ContentLength = declared
	recorder := httptest.NewRecorder()

	active.upload(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the incomplete-upload page", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Upload incomplete") || !strings.Contains(recorder.Body.String(), "1 file saved to this computer.") {
		t.Errorf("page = %s, want incomplete with exactly 1 file saved", recorder.Body.String())
	}
	_, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want the overrunning file removed and the completed one kept", got)
	}
	if got := rig.dest.Snapshot().FilesSaved; got != 1 {
		t.Fatalf("FilesSaved = %d, want 1: bytes past the declared length must never be saved as a file", got)
	}
}

func TestAWriteErrorMidUploadIsIncompleteWithTheExactCount(t *testing.T) {
	var spy *spyDestination
	rig := startReceiveRig(t, withDestination(func(real transfer.ReceiveDestination) transfer.ReceiveDestination {
		spy = &spyDestination{ReceiveDestination: real}
		spy.saveFile = func(ctx context.Context, name string, content io.Reader) (int64, error) {
			if spy.saves.Load() == 2 {
				_, _ = io.CopyN(io.Discard, content, 10)
				return 0, transfer.NewError(transfer.ErrTransferFailed, "disk full")
			}
			return real.SaveFile(ctx, name, content)
		}
		return spy
	}))

	response, page := rig.post(multipartBody(uploadFile("one.txt", "1"), uploadFile("two.txt", "2"), uploadFile("three.txt", "3")))
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the incomplete page", response.StatusCode)
	}
	text := string(page)
	if !strings.Contains(text, "Upload incomplete") || !strings.Contains(text, "1 file saved to this computer.") {
		t.Errorf("page = %s, want incomplete with exactly 1 file saved", text)
	}
	if strings.Contains(text, "Upload complete") {
		t.Error("an incomplete upload was reported as complete")
	}
	if _, terminal := awaitTerminal(t, rig.handle.Events); terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "one.txt" {
		t.Fatalf("subfolder holds %v, want only the first file", got)
	}
}

// --- Inactivity and long uploads ----------------------------------------------

func TestAnUploadThatStallsIsEndedByTheInactivityBound(t *testing.T) {
	rig := startReceiveRig(t, withTimeouts(serverTimeouts{
		readHeader: readHeaderTimeout, read: readTimeout, idle: idleTimeout, teardown: TeardownBound,
		uploadIdle: 300 * time.Millisecond,
	}))
	full := multipartBody(uploadFile("done.bin", "COMPLETE FILE"), uploadFile("stalled.bin", strings.Repeat("S", 200_000)))
	cut := bytes.Index(full, []byte("stalled.bin")) + 3000
	conn := rig.rawConn(len(full))
	if _, err := conn.Write(full[:cut]); err != nil {
		t.Fatal(err)
	}
	// The connection stays open and silent: only the inactivity bound can end it.

	started := time.Now()
	_, terminal := awaitTerminal(t, rig.handle.Events)
	if terminal.Kind != transfer.ServerFailed {
		t.Fatalf("terminal = %s, want failed", terminal.Kind)
	}
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("the stall ended after %v, want the 300ms inactivity bound to fire long before the 20s whole-request deadline", elapsed)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want the completed file kept and the stalled one removed", got)
	}
	if got := rig.dest.Snapshot().FilesSaved; got != 1 {
		t.Fatalf("FilesSaved = %d, want 1", got)
	}
}

// The whole-request read deadline is the very thing that would cut off a real
// phone upload, so it is shrunk here far below the length of the upload. Bytes
// keep arriving, so the inactivity bound -- re-armed on every read -- never
// fires and the upload completes.
func TestAnUploadLongerThanTheWholeRequestDeadlineStillCompletes(t *testing.T) {
	rig := startReceiveRig(t, withTimeouts(serverTimeouts{
		readHeader: readHeaderTimeout, read: 400 * time.Millisecond, idle: idleTimeout, teardown: TeardownBound,
		uploadIdle: 700 * time.Millisecond,
	}))
	content := strings.Repeat("0123456789abcdef", 4096) // 64 KiB per chunk
	const chunks = 24
	full := multipartBody(uploadFile("steady.bin", strings.Repeat(content, chunks)))
	head := bytes.Index(full, []byte("0123456789abcdef"))
	conn := rig.rawConn(len(full))

	started := time.Now()
	var writeErr error
	if _, err := conn.Write(full[:head]); err != nil {
		writeErr = err
	}
	remaining := full[head:]
	step := len(remaining)/chunks + 1
	for len(remaining) > 0 && writeErr == nil {
		n := min(step, len(remaining))
		if _, err := conn.Write(remaining[:n]); err != nil {
			writeErr = err
			break
		}
		remaining = remaining[n:]
		time.Sleep(100 * time.Millisecond)
	}
	// Whether the server cut the connection mid-stream is read from the one
	// outcome it reports, not from where a client write happened to fail.
	if _, terminal := awaitTerminal(t, rig.handle.Events); terminal.Kind != transfer.ServerComplete {
		t.Fatalf("terminal = %s (%v) after %v, want complete: bytes kept arriving, so the inactivity bound must never have fired", terminal.Kind, terminal.Err, time.Since(started))
	}
	if writeErr != nil {
		t.Fatalf("writing the body failed after %v: %v", time.Since(started), writeErr)
	}
	response, _ := readResponse(t, conn)
	elapsed := time.Since(started)
	if elapsed < 1500*time.Millisecond {
		t.Fatalf("the upload took only %v; it must outlast both the 400ms and the 700ms bounds to prove anything", elapsed)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d after %v, want 200", response.StatusCode, elapsed)
	}
	if got := readText(t, filepath.Join(rig.subfolder(), "steady.bin")); got != strings.Repeat(content, chunks) {
		t.Fatal("the saved bytes differ from the uploaded bytes")
	}
}

// --- Desktop Cancel -------------------------------------------------------------

func TestStopMidFileKeepsCompletedFilesAndRemovesThePartialFile(t *testing.T) {
	rig := startReceiveRig(t)
	full := multipartBody(uploadFile("done.bin", "COMPLETE FILE"), uploadFile("partial.bin", strings.Repeat("P", 200_000)))
	cut := bytes.Index(full, []byte("partial.bin")) + 3000
	conn := rig.rawConn(len(full))
	if _, err := conn.Write(full[:cut]); err != nil {
		t.Fatal(err)
	}
	waitForPartial(t, rig)

	stopped := make(chan error, 1)
	go func() { stopped <- rig.server.Stop() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("Stop = %v, want a quiescent teardown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not return while an upload was stalled mid-file")
	}

	result, _ := rig.dest.Close()
	if result.FilesSaved != 1 {
		t.Fatalf("Close() = %+v, want exactly the completed file saved", result)
	}
	if got := listNames(t, rig.subfolder()); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want the partial file removed", got)
	}
	// The coordinator owns a cancelled upload's outcome; the server stays silent.
	for _, event := range drainEvents(t, rig.handle.Events) {
		if event.Kind == transfer.ServerComplete || event.Kind == transfer.ServerFailed {
			t.Fatalf("a cancelled upload reported a %s event", event.Kind)
		}
	}
}

// --- Memory ---------------------------------------------------------------------

// patternReader streams n bytes of a repeating pattern without holding them.
type patternReader struct{ remaining int64 }

func (r *patternReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for index := range p[:n] {
		p[index] = byte('a' + index%26)
	}
	r.remaining -= n
	return int(n), nil
}

func TestALargeUploadStreamsWithBoundedMemoryAndNothingInOSTempStorage(t *testing.T) {
	osTemp := t.TempDir()
	t.Setenv("TMPDIR", osTemp)
	t.Setenv("TMP", osTemp)
	t.Setenv("TEMP", osTemp)
	rig := startReceiveRig(t)

	const size = 64 << 20
	prefix := []byte("--" + testBoundary + "\r\nContent-Disposition: form-data; name=\"files\"; filename=\"big.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n")
	suffix := []byte("\r\n--" + testBoundary + "--\r\n")
	body := io.MultiReader(bytes.NewReader(prefix), &patternReader{remaining: size}, bytes.NewReader(suffix))
	request, err := http.NewRequest(http.MethodPost, rig.uploadURL(), body)
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = int64(len(prefix)) + size + int64(len(suffix))
	request.Header.Set("Content-Type", multipartType)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak uint64
	var peakMu sync.Mutex
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var now runtime.MemStats
				runtime.ReadMemStats(&now)
				peakMu.Lock()
				peak = max(peak, now.HeapAlloc)
				peakMu.Unlock()
			}
		}
	}()

	response, err := testClient().Do(request)
	close(stop)
	<-sampled
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}

	peakMu.Lock()
	growth := int64(peak) - int64(before.HeapAlloc)
	peakMu.Unlock()
	t.Logf("peak heap growth during a %d MiB upload: %d KiB", size>>20, growth>>10)
	if growth > 24<<20 {
		t.Errorf("the heap grew %d MiB during a %d MiB upload, want it bounded far below the upload size", growth>>20, size>>20)
	}
	info, err := os.Stat(filepath.Join(rig.subfolder(), "big.bin"))
	if err != nil || info.Size() != size {
		t.Fatalf("big.bin = %v, %v, want %d bytes", info, err, size)
	}
	if got := listNames(t, osTemp); len(got) != 0 {
		t.Errorf("the upload wrote %v to OS temp storage", got)
	}
}

// --- Part headers ------------------------------------------------------------------

func TestAPartHeaderBeyondItsBoundIsRefusedBeforeTheClaim(t *testing.T) {
	rig := startReceiveRig(t)
	var body bytes.Buffer
	fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"files\"; filename=\"a.txt\"\r\n", testBoundary)
	for index := 0; index < 3000; index++ {
		fmt.Fprintf(&body, "X-Pad-%04d: %s\r\n", index, strings.Repeat("p", 20))
	}
	fmt.Fprintf(&body, "\r\nA\r\n--%s--\r\n", testBoundary)

	response, _ := rig.post(body.Bytes())
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a part header past its bound", response.StatusCode)
	}
	rig.assertNothingWritten()
	if calls := rig.auth.calls.Load(); calls != 0 {
		t.Fatalf("an oversized part header reached the authorizer %d times", calls)
	}
}

// --- Authorizer ----------------------------------------------------------------------

func TestARefusedClaimWritesNothingAndLooksLikeAnyOtherNotFound(t *testing.T) {
	rig := startReceiveRig(t, withAuthorizer(refusingAuthorizer(transfer.NewError(transfer.ErrCancelled, "cancelled"))))
	response, page := rig.post(multipartBody(uploadFile("a.txt", "a")))
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want the generic 404", response.StatusCode)
	}
	assertNoDisclosure(t, response, page)
	rig.assertNothingWritten()
	assertNoEvents(t, rig.handle.Events)
}

// --- Terminal event rules ------------------------------------------------------------

// A receive upload is complete when its files are saved. A failed write of the
// result page must not turn a fully saved upload into a failed one.
func TestAFailedResultPageWriteDoesNotDemoteACompleteUpload(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		receiving bool
		want      transfer.ServerEventKind
	}{
		{"a receive upload stays complete", true, transfer.ServerComplete},
		{"a download is still demoted", false, transfer.ServerFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client, serverSide := net.Pipe()
			defer client.Close()
			defer serverSide.Close()

			run := &run{
				sessionID: testSession,
				ctx:       context.Background(),
				cancel:    func() {},
				listener:  &onceCloseListener{Listener: noopListener{}},
				lane:      newEventLane(),
				conns:     map[net.Conn]struct{}{},
			}
			run.connsGone = sync.NewCond(&run.mu)
			if testCase.receiving {
				run.destination = &spyDestination{}
			}
			complete := completeEvent(testSession, transfer.ProgressSnapshot{BytesSent: 10, TotalBytes: 10, TotalKnown: true})
			conn := &finalizingConn{Conn: serverSide, writeErr: errors.New("test: the phone went away"), terminal: &complete}

			run.trackConnection(conn, http.StateClosed)

			select {
			case event := <-run.lane.channel():
				if event.Kind != testCase.want {
					t.Fatalf("terminal event = %s, want %s", event.Kind, testCase.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no terminal event was published")
			}
		})
	}
}

type noopListener struct{}

func (noopListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (noopListener) Close() error              { return nil }
func (noopListener) Addr() net.Addr            { return &net.TCPAddr{} }

// --- Start validation and production defaults ---------------------------------------

func TestStartRefusesAReceiveRequestThatAlsoNamesAnItem(t *testing.T) {
	dest := &spyDestination{}
	server := newTestServer(t, nil)
	_, err := server.Start(context.Background(), transfer.ServerStartRequest{
		SessionID: testSession, Token: testToken, Item: startRequest().Item, Destination: dest,
	}, &stubAuthorizer{})
	if transfer.ErrorCodeOf(err) != transfer.ErrServerStartFailed {
		t.Fatalf("Start = %v, want server_start_failed", err)
	}
}

func TestStartingAReceiveRunNeedsNoPayloadPortAndABareSendStartStillDoes(t *testing.T) {
	dir := fixtureDir(t)
	_ = dir
	server := newTestServer(t, nil)
	if _, err := server.Start(context.Background(), startRequest(), &stubAuthorizer{}); transfer.ErrorCodeOf(err) != transfer.ErrServerStartFailed {
		t.Fatalf("a send Start without a payload port = %v, want server_start_failed", err)
	}
	rig := startReceiveRig(t)
	if rig.handle.Port == 0 {
		t.Fatal("a receive Start without a payload port did not bind")
	}
}

// The seam tests above shrink the deadlines; this drives the values the shipped
// server runs on.
func TestTheProductionUploadInactivityBoundIsThirtySeconds(t *testing.T) {
	if got := defaultTimeouts().uploadIdle; got != 30*time.Second {
		t.Fatalf("defaultTimeouts().uploadIdle = %v, want 30s", got)
	}
	if got := (serverTimeouts{}).uploadIdleOrDefault(); got != 30*time.Second {
		t.Fatalf("an unset uploadIdle resolves to %v, want the 30s default", got)
	}
	if got := (serverTimeouts{uploadIdle: time.Second}).uploadIdleOrDefault(); got != time.Second {
		t.Fatalf("an explicit uploadIdle resolves to %v, want it kept", got)
	}
	if New(nil).timeouts.uploadIdle != 30*time.Second {
		t.Fatal("New() does not ship the 30s upload inactivity bound")
	}
}
