package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/qr"
	"fairdrop/internal/server"
	"fairdrop/internal/source"
	"fairdrop/internal/stream"
	"fairdrop/internal/transfer"
)

func TestCompletionAlertsRequireOptInAndOneBackgroundSuccess(t *testing.T) {
	background := false
	called := make(chan uint64, 4)
	alerts := newCompletionAlerts(func(request alertRequest) { called <- request.epoch }, func(uint64) {}, func() focusSnapshot { return focusSnapshot{generation: 1, background: background} })
	defer alerts.close()
	if alerts.complete("disabled") {
		t.Fatal("disabled completion requested a cue")
	}
	if alerts.lastSession != "" {
		t.Fatal("disabled completion entered alert dispatcher")
	}
	if !alerts.set(true) {
		t.Fatal("opt-in was not enabled")
	}
	if alerts.complete("foreground") {
		t.Fatal("foreground completion requested a cue")
	}
	background = true
	if !alerts.complete("background") {
		t.Fatal("background completion was not queued")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("background success did not request a cue")
	}
	if alerts.complete("background") {
		t.Fatal("duplicate completion requested a cue")
	}
	alerts.set(false)
	if alerts.complete("disabled-again") {
		t.Fatal("disabled completion requested a cue")
	}
	if alerts.lastSession != "background" {
		t.Fatal("disabled completion entered alert dispatcher")
	}
	alerts.set(true)
}

func TestNewAppUsesNativeCompletionAdapters(t *testing.T) {
	app := NewApp()
	for _, seam := range []struct {
		name      string
		got, want any
	}{
		{"cue", app.alerts.cue, nativeCompletionCue},
		{"invalidation", app.alerts.invalidate, nativeCompletionInvalidate},
		{"foreground snapshot", app.alerts.focus, nativeCompletionSnapshot},
	} {
		if reflect.ValueOf(seam.got).Pointer() != reflect.ValueOf(seam.want).Pointer() {
			t.Errorf("NewApp did not wire the native %s adapter", seam.name)
		}
	}
}

func TestCompletionAlertsDiscardQueuedWorkAndDoNotBlockPublisher(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	called := make(chan uint64, 2)
	alerts := newCompletionAlerts(func(request alertRequest) {
		if request.epoch == 1 {
			close(entered)
			<-release
		}
		called <- request.epoch
	}, func(uint64) {}, func() focusSnapshot { return focusSnapshot{generation: 1, background: true} })
	alerts.set(true)
	alerts.complete("first")
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first native cue never entered")
	}
	finished := make(chan struct{})
	go func() {
		alerts.complete("second")
		if len(alerts.pending) != 1 {
			t.Error("second completion was not queued")
		}
		alerts.set(false)
		if len(alerts.pending) != 0 {
			t.Error("disabled alert left stale work queued")
		}
		alerts.set(true)
		alerts.complete("third")
		if len(alerts.pending) != 1 {
			t.Error("fresh completion was not queued")
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("slow cue blocked lifecycle or toggle")
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case <-called:
		case <-time.After(time.Second):
			t.Fatal("fresh completion did not pass a stalled cue")
		}
	}
	if len(alerts.pending) != 0 {
		t.Fatal("stale queued completion survived the worker")
	}
	alerts.close()
	alerts.complete("after-exit")
	if alerts.set(true) {
		t.Fatal("alerts re-enabled after exit")
	}
}

func TestUnavailableCueDoesNotStopLaterTransfers(t *testing.T) {
	entered := make(chan struct{})
	delivered := make(chan struct{})
	calls := 0
	alerts := newCompletionAlerts(func(alertRequest) {
		calls++
		if calls == 1 {
			close(entered)
			panic("OS cue unavailable")
		}
		close(delivered)
	}, func(uint64) {}, func() focusSnapshot { return focusSnapshot{generation: 1, background: true} })
	defer alerts.close()
	alerts.set(true)
	if !alerts.complete("first") {
		t.Fatal("first completion was not admitted")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("unavailable cue never entered")
	}
	if !alerts.complete("second") {
		t.Fatal("second completion was blocked by unavailable cue")
	}
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("worker did not recover for later transfer")
	}
}

func TestForegroundReturnFencesDelayedCueAndKeepsNewestSuccess(t *testing.T) {
	var focus atomic.Pointer[focusSnapshot]
	focus.Store(&focusSnapshot{generation: 1, background: true})
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan alertRequest, 2)
	alerts := newCompletionAlerts(func(request alertRequest) {
		if request.focus == 1 {
			close(entered)
			<-release
		}
		if current := focus.Load(); current.background && current.generation == request.focus {
			delivered <- request
		}
	}, func(uint64) {}, func() focusSnapshot { return *focus.Load() })
	defer alerts.close()
	alerts.set(true)
	if !alerts.complete("first") {
		t.Fatal("initial success was not queued")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("native cue did not enter")
	}
	focus.Store(&focusSnapshot{generation: 2, background: false})
	focus.Store(&focusSnapshot{generation: 3, background: true})
	if !alerts.complete("second") {
		t.Fatal("new background success was dropped")
	}
	focus.Store(&focusSnapshot{generation: 4, background: false})
	focus.Store(&focusSnapshot{generation: 5, background: true})
	if !alerts.complete("third") {
		t.Fatal("new background successes were dropped")
	}
	close(release)
	select {
	case request := <-delivered:
		if request.focus != 5 {
			t.Fatalf("delivered obsolete request %+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("newest eligible success was swallowed")
	}
}

func TestRapidCompletionsReplacePendingWithoutBlocking(t *testing.T) {
	var focus atomic.Pointer[focusSnapshot]
	focus.Store(&focusSnapshot{generation: 1, background: true})
	entered, release := make(chan struct{}), make(chan struct{})
	delivered := make(chan uint64, 1)
	alerts := newCompletionAlerts(func(request alertRequest) {
		if request.focus == 1 {
			close(entered)
			<-release
		}
		if current := focus.Load(); current.generation == request.focus {
			delivered <- request.focus
		}
	}, func(uint64) {}, func() focusSnapshot { return *focus.Load() })
	defer alerts.close()
	alerts.set(true)
	alerts.complete("first")
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("native cue never entered")
	}
	finished := make(chan struct{})
	go func() {
		for generation := uint64(2); generation <= 1001; generation++ {
			focus.Store(&focusSnapshot{generation: generation, background: true})
			alerts.complete(fmt.Sprintf("session-%d", generation))
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("publication blocked behind full slot")
	}
	close(release)
	select {
	case generation := <-delivered:
		if generation != 1001 {
			t.Fatalf("delivered generation %d, want newest 1001", generation)
		}
	case <-time.After(time.Second):
		t.Fatal("newest pending success was swallowed")
	}
}

func TestAppCompletionCueUsesOnlyAuthoritativeCompleteEvent(t *testing.T) {
	called := make(chan alertRequest, 3)
	var invalidations []uint64
	app := NewApp()
	app.emit = func(context.Context, string, ...interface{}) {}
	app.logf = func(string, ...any) {}
	app.alerts = newCompletionAlerts(func(request alertRequest) { called <- request }, func(epoch uint64) { invalidations = append(invalidations, epoch) }, func() focusSnapshot { return focusSnapshot{generation: 1, background: true} })
	app.startup(context.Background())
	defer app.alerts.close()
	if state, err := app.GetCompletionAlerts(); err != nil || state {
		t.Fatalf("fresh launch state = %v, %v", state, err)
	}
	if enabled, err := app.SetCompletionAlerts(true); err != nil || !enabled {
		t.Fatalf("opt-in = %v, %v", enabled, err)
	}
	if state, err := app.GetCompletionAlerts(); err != nil || !state {
		t.Fatalf("remount state = %v, %v", state, err)
	}
	for _, kind := range []transfer.EventKind{transfer.TransferStarted, transfer.TransferProgress, transfer.TransferError, transfer.TransferReset, transfer.TransferNotice} {
		app.publish(transfer.Event{Kind: kind, SessionID: "failed"})
	}
	if app.alerts.lastSession != "" {
		t.Fatal("non-success event entered the completion dispatcher")
	}
	app.publish(transfer.Event{Kind: transfer.TransferComplete, SessionID: "successful"})
	select {
	case request := <-called:
		if request.epoch != 1 || request.focus != 1 {
			t.Fatalf("send cue = %+v, want epoch and focus 1", request)
		}
	case <-time.After(time.Second):
		t.Fatal("authoritative complete did not request a cue")
	}
	appObserver{app: app}.Publish(transfer.Event{Kind: transfer.TransferComplete, SessionID: "receive-success",
		Progress: &transfer.ProgressSnapshot{BytesSent: 10, TotalBytes: 10, TotalKnown: true, Percent: 100},
		Receive:  &transfer.ReceiveStatus{FilesSaved: 1, Result: transfer.ReceiveComplete, SubfolderExists: true}})
	select {
	case request := <-called:
		if request.epoch != 1 || request.focus != 1 {
			t.Fatalf("receive cue = %+v, want epoch and focus 1", request)
		}
	case <-time.After(time.Second):
		t.Fatal("production-shaped receive completion did not request a cue")
	}
	if enabled, err := app.SetCompletionAlerts(false); err != nil || enabled {
		t.Fatalf("opt-out = %v, %v", enabled, err)
	}
	if enabled, err := app.SetCompletionAlerts(true); err != nil || !enabled {
		t.Fatalf("re-opt-in = %v, %v", enabled, err)
	}
	appObserver{app: app}.Publish(transfer.Event{Kind: transfer.TransferComplete, SessionID: "fresh-after-opt-in"})
	select {
	case request := <-called:
		if request.epoch != 3 {
			t.Fatalf("fresh cue epoch = %d, want 3", request.epoch)
		}
	case <-time.After(time.Second):
		t.Fatal("fresh completion after re-opt-in was lost")
	}
	app.shutdown(context.Background())
	if !slices.Equal(invalidations, []uint64{0, 1, 2, 3, 4}) {
		t.Fatalf("native invalidation epochs = %v", invalidations)
	}
}

func TestAppOptOutAndShutdownInvalidateInFlightNativeCue(t *testing.T) {
	for _, scenario := range []struct {
		name string
		end  func(*App)
	}{
		{"opt-out", func(app *App) { _, _ = app.SetCompletionAlerts(false) }},
		{"shutdown", func(app *App) { app.shutdown(context.Background()) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			delivered := make(chan alertRequest, 1)
			var nativeEpoch atomic.Uint64
			app := NewApp()
			app.emit = func(context.Context, string, ...interface{}) {}
			app.logf = func(string, ...any) {}
			app.alerts = newCompletionAlerts(func(request alertRequest) {
				close(entered)
				<-release
				if nativeEpoch.Load() == request.epoch {
					delivered <- request
				}
				close(finished)
			}, nativeEpoch.Store, func() focusSnapshot { return focusSnapshot{generation: 1, background: true} })
			app.startup(context.Background())
			t.Cleanup(func() { app.shutdown(context.Background()) })
			if enabled, err := app.SetCompletionAlerts(true); err != nil || !enabled {
				t.Fatalf("opt-in = %v, %v", enabled, err)
			}
			appObserver{app: app}.Publish(transfer.Event{Kind: transfer.TransferComplete, SessionID: "pending-native"})
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("native cue did not enter")
			}
			scenario.end(app)
			if got := nativeEpoch.Load(); got != 2 {
				t.Fatalf("native epoch after %s = %d, want 2", scenario.name, got)
			}
			close(release)
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("native cue did not finish")
			}
			select {
			case request := <-delivered:
				t.Fatalf("obsolete native cue delivered: %+v", request)
			default:
			}
		})
	}
}

func TestRealFileHTTPCompletionRequestsOneBackgroundCue(t *testing.T) {
	path := filepath.Join(nativeFixture(t), "payload.txt")
	if err := os.WriteFile(path, []byte("completed over real HTTP"), 0600); err != nil {
		t.Fatal(err)
	}
	called := make(chan alertRequest, 2)
	var cueCount atomic.Int32
	app := NewApp()
	app.emit = func(context.Context, string, ...interface{}) {}
	app.logf = func(string, ...any) {}
	app.alerts = newCompletionAlerts(func(request alertRequest) { cueCount.Add(1); called <- request }, func(uint64) {}, func() focusSnapshot { return focusSnapshot{generation: 1, background: true} })
	app.startup(context.Background())
	inspector := source.New()
	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source: newSelectionSource(inspector), Network: recoveryLoopbackNetwork{},
		Server: server.New(stream.New(inspector)), QR: qr.New(), Observer: appObserver{app: app},
	})
	app.useCoordinator(coordinator)
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if enabled, err := app.SetCompletionAlerts(true); err != nil || !enabled {
		t.Fatalf("opt-in = %v, %v", enabled, err)
	}
	metadata, err := app.StageTransfer(path)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(metadata.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "completed over real HTTP" {
		t.Fatalf("HTTP transfer = status %d, body %q, error %v", response.StatusCode, body, readErr)
	}
	select {
	case request := <-called:
		if request.epoch != 1 || request.focus != 1 {
			t.Fatalf("HTTP cue = %+v, want epoch and focus 1", request)
		}
	case <-time.After(time.Second):
		t.Fatal("real HTTP completion did not reach the native cue adapter")
	}
	app.shutdown(context.Background()) // coordinator quiescence: no later success can enqueue.
	if got := cueCount.Load(); got != 1 {
		t.Fatalf("real HTTP cue count = %d, want exactly one", got)
	}
}
