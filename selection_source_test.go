package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

func TestSelectionResolutionHonoursAdmissionAndCancellation(t *testing.T) {
	for _, ending := range []string{"caller", "cancel", "shutdown"} {
		t.Run(ending, func(t *testing.T) {
			app, inspector := nativeMatrixApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			app.startup(ctx)
			entered, release := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			enteredOnce := sync.OnceFunc(func() { close(entered) })
			inspector.selection.resolve = func(path string) string {
				enteredOnce()
				<-release
				return path
			}
			staged := make(chan error, 1)
			go func() { _, err := app.StageTransfer("private selection"); staged <- err }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("resolver was not entered")
			}
			if _, err := app.StageTransfer("another selection"); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
				t.Fatal("busy admission reached filesystem resolution")
			}
			switch ending {
			case "caller":
				cancel()
			case "cancel":
				if err := app.CancelTransfer(); err != nil {
					t.Fatal("Cancel did not release blocked selection")
				}
			case "shutdown":
				if err := app.transfers.Shutdown(context.Background()); err != nil {
					t.Fatal("Shutdown did not release blocked selection")
				}
			}
			select {
			case err := <-staged:
				if err == nil {
					t.Fatal("abandoned selection committed")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("selection waited for the blocked filesystem call")
			}
			app.startup(context.Background())
			want := transfer.ErrBusy
			if ending == "shutdown" {
				want = transfer.ErrShuttingDown
			}
			for range 3 {
				retry := make(chan error, 1)
				go func() { _, err := app.StageTransfer("retry"); retry <- err }()
				select {
				case err := <-retry:
					if transfer.ErrorCodeOf(err) != want {
						t.Fatal("retry admitted additional unresolved filesystem work")
					}
				case <-time.After(time.Second):
					t.Fatal("retry admitted additional unresolved filesystem work")
				}
			}
			if inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
				t.Fatal("abandoned resolver reached Inspect or network")
			}
			unblock()
			deadline := time.After(5 * time.Second)
			for inspector.selection.resolving.Load() {
				select {
				case <-deadline:
					t.Fatal("returned resolver retained busy ownership")
				default:
					runtime.Gosched()
				}
			}
			if inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
				t.Fatal("late resolver performed Inspect or network work")
			}
		})
	}
}

func TestSelectionResolutionCancelledBeforeEntryDoesNoFilesystemWork(t *testing.T) {
	source := newSelectionSource(nil)
	source.resolve = func(string) string { t.Error("cancelled context reached resolver"); return "" }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.Inspect(ctx, "private selection"); transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatal("pre-cancelled resolution was not refused")
	}
}

func TestSelectionResolutionRetryAfterFilesystemReturns(t *testing.T) {
	app, inspector := nativeMatrixApp(t)
	file := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(file, []byte("native matrix payload"), 0o600); err != nil {
		t.Fatal("fixture write failed")
	}
	// The production default must canonicalize before Inspect and preserve the
	// raw inspector for later streaming; native matrix tests exercise that path.
	if _, err := app.StageTransfer(file); err != nil {
		t.Fatal("healthy resolution failed")
	}
	if inspector.calls.Load() != 1 || inspector.networkCalls.Load() != 1 {
		t.Fatal("healthy selection did not inspect before network")
	}
}
