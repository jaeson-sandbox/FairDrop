package transfer

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

const (
	// testReceiveFolder is a full absolute path with a space, searched for by
	// every disclosure assertion: a leak anywhere is detected, not argued about.
	testReceiveFolder = `/Users/receiver/Phone Photos`
	testReceiveName   = "Phone Photos"
	testSubfolderPath = `/Users/receiver/Phone Photos/FairDrop 2026-10-10 14.05`

	// testUploadURL is spelled out rather than built from uploadPathPrefix, for
	// the reason testURL is: deriving it from the constant under test would let a
	// URL pointing at a route the server answers with 404 ship green.
	testUploadURL = "http://" + testAddress + ":45678" + "/upload/" + string(testToken)
)

// fakeSink is the SinkPort every coordinator receive test runs against. Every
// call passes through the harness gate, so a call made while the coordinator
// holds its state mutex fails the test by name.
type fakeSink struct {
	h    *harness
	open func(ctx context.Context, absolutePath string) (ReceiveDestination, error)

	mu    sync.Mutex
	paths []string
	dests []*fakeDestination
}

func (f *fakeSink) OpenDestination(ctx context.Context, absolutePath string) (ReceiveDestination, error) {
	f.h.enter("sink.Open")
	f.mu.Lock()
	f.paths = append(f.paths, absolutePath)
	f.mu.Unlock()
	if f.open != nil {
		return f.open(ctx, absolutePath)
	}
	return f.newDestination(), nil
}

func (f *fakeSink) newDestination() *fakeDestination {
	dest := &fakeDestination{h: f.h, name: testReceiveName, folder: testSubfolderPath, closeSeen: make(chan struct{}, 8)}
	f.mu.Lock()
	f.dests = append(f.dests, dest)
	f.mu.Unlock()
	return dest
}

func (f *fakeSink) opened() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

// destination returns the n-th destination the sink handed out.
func (f *fakeSink) destination(n int) *fakeDestination {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n >= len(f.dests) {
		f.h.t.Fatalf("the sink handed out %d destinations, want at least %d", len(f.dests), n+1)
	}
	return f.dests[n]
}

// fakeDestination reports whatever result a test sets, and records how it was
// used.
type fakeDestination struct {
	h      *harness
	name   string
	folder string

	mu           sync.Mutex
	current      ReceiveResult // what Snapshot reports
	closeResult  ReceiveResult // what Close reports
	closeErr     error
	closes       int
	closeSeen    chan struct{}
	closeBlocked chan struct{} // when non-nil, Close waits for it to be closed
	folderGone   bool
}

func (f *fakeDestination) Name() string { return f.name }

func (f *fakeDestination) CheckSpace(int64) error {
	f.h.enter("destination.CheckSpace")
	return nil
}

func (f *fakeDestination) SaveFile(context.Context, string, io.Reader) (int64, error) {
	f.h.enter("destination.SaveFile")
	f.h.t.Error("the coordinator wrote a file; only the server writes")
	return 0, nil
}

func (f *fakeDestination) Snapshot() ReceiveResult {
	f.h.enter("destination.Snapshot")
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func (f *fakeDestination) Folder() (string, bool) {
	f.h.enter("destination.Folder")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.folderGone {
		return "", false
	}
	return f.folder, true
}

func (f *fakeDestination) Close() (ReceiveResult, error) {
	f.h.enter("destination.Close")
	f.mu.Lock()
	f.closes++
	blocked, result, err := f.closeBlocked, f.closeResult, f.closeErr
	f.mu.Unlock()
	select {
	case f.closeSeen <- struct{}{}:
	default:
	}
	if blocked != nil {
		<-blocked
	}
	return result, err
}

func (f *fakeDestination) closeCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}

// finalResult sets what the destination reports once closed.
func (f *fakeDestination) finalResult(result ReceiveResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current, f.closeResult = result, result
}

// newReceiveHarness is newHarness with a fake sink wired into the coordinator.
// The coordinator is built without one in newHarness so every existing test
// keeps running against exactly the dependencies it always had.
func newReceiveHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.sink = &fakeSink{h: h}
	h.coordinator.sink = h.sink
	return h
}

// receiveWaiting starts a receive session and fails the test unless it commits.
func (h *harness) receiveWaiting() ReceiveMetadata {
	h.t.Helper()
	metadata, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
	if err != nil {
		h.t.Fatalf("StartReceive returned %v, want a committed session", err)
	}
	if live := h.liveSession(); live != nil {
		h.track(live)
	}
	return metadata
}

// receiveTransferring starts a receive session and claims it, leaving the
// coordinator in TRANSFERRING with exactly the started event published.
func (h *harness) receiveTransferring() ReceiveMetadata {
	h.t.Helper()
	metadata := h.receiveWaiting()
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		h.t.Fatalf("AuthorizeClaim returned %v, want a committed transfer", err)
	}
	return metadata
}

func receiveProgressSnapshot(sent, total int64, percent float64) ProgressSnapshot {
	return ProgressSnapshot{BytesSent: sent, TotalBytes: total, TotalKnown: true, Percent: percent, SpeedBytesPerSec: 2048}
}

func receiveProgressEvent(id SessionID, snapshot ProgressSnapshot, filesSaved int) ServerEvent {
	event := progressEvent(id, snapshot)
	event.Receive = &ReceiveStatus{FilesSaved: filesSaved}
	return event
}

// eventually polls until check holds or the probe timeout passes.
func (h *harness) eventually(what string, check func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(mutexProbeTimeout)
	for !check() {
		if time.Now().After(deadline) {
			h.t.Fatalf("%s never happened", what)
		}
		time.Sleep(200 * time.Microsecond)
	}
}
