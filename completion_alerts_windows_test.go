//go:build windows

package main

import (
	"testing"
	"time"
)

func awaitWindowsObserverExit(t *testing.T) {
	t.Helper()
	if !alertObserverStarted.Load() {
		return
	}
	select {
	case <-alertObserverDone:
	case <-time.After(time.Second):
		t.Fatal("Windows foreground observer did not release its hook")
	}
}

func TestWindowsRegisteredObserverClearsBackgroundOnEveryExit(t *testing.T) {
	awaitWindowsObserverExit(t)
	oldPID, oldWindowPID, oldFocus := alertCurrentPIDFunc, alertWindowPIDFunc, alertFocus.Load()
	oldStopped, oldThread, oldPost := alertObserverStopped.Load(), alertThreadID.Load(), alertPostQuit
	t.Cleanup(func() {
		alertCurrentPIDFunc, alertWindowPIDFunc = oldPID, oldWindowPID
		alertFocus.Store(oldFocus)
		alertObserverStopped.Store(oldStopped)
		alertThreadID.Store(oldThread)
		alertPostQuit = oldPost
	})
	alertCurrentPIDFunc = func() uintptr { return 10 }
	alertWindowPIDFunc = func(uintptr) uintptr { return 20 }
	for _, scenario := range []struct {
		name    string
		hook    uintptr
		message int32
		stop    bool
	}{
		{"registration failure", 0, 0, false},
		{"unexpected GetMessage error", 88, -1, false},
		{"normal WM_QUIT", 88, 0, false},
		{"shutdown posts quit", 88, 0, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			alertObserverStopped.Store(false)
			alertFocus.Store(&windowsFocus{known: true, background: true, generation: 7})
			unhooked, posted := uintptr(0), uint32(0)
			alertPostQuit = func(thread uint32) { posted = thread }
			pump := windowsObserverPump{
				hook:   func() uintptr { return scenario.hook },
				unhook: func(hook uintptr) { unhooked = hook },
				peek:   func() {}, threadID: func() uint32 { return 99 },
				foreground: func() uintptr { return 22 },
				getMessage: func() int32 {
					if scenario.stop {
						nativeCompletionStop()
					}
					return scenario.message
				},
				dispatch: func() { t.Fatal("terminal message was dispatched") },
			}
			runWindowsForegroundObserver(pump)
			if alertFocus.Load() != nil || nativeCompletionSnapshot().background {
				t.Fatal("observer exit retained obsolete background eligibility")
			}
			if alertThreadID.Load() != 0 {
				t.Fatal("observer exit retained thread ID")
			}
			if scenario.hook != 0 && unhooked != scenario.hook {
				t.Fatal("registered WinEvent hook was not released")
			}
			if scenario.stop && posted != 99 {
				t.Fatalf("shutdown posted quit to thread %d, want 99", posted)
			}
		})
	}
}

func TestWindowsObserverPreservesBackgroundAcrossForeignAppSwitch(t *testing.T) {
	awaitWindowsObserverExit(t)
	oldPID, oldWindowPID, oldFocus, oldStopped := alertCurrentPIDFunc, alertWindowPIDFunc, alertFocus.Load(), alertObserverStopped.Load()
	t.Cleanup(func() {
		alertCurrentPIDFunc, alertWindowPIDFunc = oldPID, oldWindowPID
		alertFocus.Store(oldFocus)
		alertObserverStopped.Store(oldStopped)
	})
	alertObserverStopped.Store(false)
	alertFocus.Store(nil)
	alertCurrentPIDFunc = func() uintptr { return 10 }
	alertWindowPIDFunc = func(hwnd uintptr) uintptr {
		if hwnd == 1 || hwnd == 2 {
			return 20
		}
		if hwnd == 3 {
			return 10
		}
		return 0
	}
	alertSample(1)
	first := nativeCompletionSnapshot()
	if !first.background || first.generation == 0 {
		t.Fatalf("first foreign foreground = %+v", first)
	}
	alertSample(2)
	second := nativeCompletionSnapshot()
	if second.generation != first.generation || !second.background {
		t.Fatalf("foreign-to-foreign switch invalidated eligibility: %+v", second)
	}
	alertSample(3)
	active := nativeCompletionSnapshot()
	if active.background || active.generation == second.generation {
		t.Fatalf("FairDrop activation did not invalidate: %+v", active)
	}
	alertSample(1)
	away := nativeCompletionSnapshot()
	if !away.background || away.generation == active.generation {
		t.Fatalf("return to background did not get a fresh generation: %+v", away)
	}
	alertSample(0)
	unknown := nativeCompletionSnapshot()
	if unknown.background || unknown.generation == away.generation {
		t.Fatalf("unknown foreground did not fail closed: %+v", unknown)
	}
}

func TestWindowsCompletionCueUsesOwnVisibleBackgroundWindowAndFiniteTrayFlash(t *testing.T) {
	awaitWindowsObserverExit(t)
	oldPID, oldOwn, oldWindowPID := alertCurrentPIDFunc, alertOwnWindowFunc, alertWindowPIDFunc
	oldClass, oldOwner, oldVisible := alertWindowClassFunc, alertWindowOwnerFunc, alertVisibleFunc
	oldForeground, oldFlash := alertForegroundFunc, alertFlashFunc
	oldFocus, oldEpoch, oldStopped := alertFocus.Load(), alertEpoch.Load(), alertObserverStopped.Load()
	t.Cleanup(func() {
		alertCurrentPIDFunc, alertOwnWindowFunc, alertWindowPIDFunc = oldPID, oldOwn, oldWindowPID
		alertWindowClassFunc, alertWindowOwnerFunc, alertVisibleFunc = oldClass, oldOwner, oldVisible
		alertForegroundFunc, alertFlashFunc = oldForeground, oldFlash
		alertFocus.Store(oldFocus)
		alertEpoch.Store(oldEpoch)
		alertObserverStopped.Store(oldStopped)
	})
	const ownWindow, foreignWindow, ownPID, foreignPID = uintptr(101), uintptr(202), uintptr(303), uintptr(404)
	windowPID := ownPID
	windowClass := "wailsWindow"
	var owner uintptr
	visible := true
	foreground := foreignWindow
	foregroundPID := foreignPID
	var flashes []flashInfo
	alertCurrentPIDFunc = func() uintptr { return ownPID }
	alertOwnWindowFunc = func() uintptr { return ownWindow }
	alertWindowPIDFunc = func(hwnd uintptr) uintptr {
		if hwnd == ownWindow {
			return windowPID
		}
		if hwnd == foreignWindow {
			return foregroundPID
		}
		return 0
	}
	alertWindowClassFunc = func(uintptr) string { return windowClass }
	alertWindowOwnerFunc = func(uintptr) uintptr { return owner }
	alertVisibleFunc = func(uintptr) bool { return visible }
	alertForegroundFunc = func() uintptr { return foreground }
	alertFlashFunc = func(info *flashInfo) { flashes = append(flashes, *info) }
	alertObserverStopped.Store(false)
	alertFocus.Store(&windowsFocus{hwnd: foreignWindow, generation: 7, background: true})
	alertEpoch.Store(2)
	request := alertRequest{epoch: 2, focus: 7}
	nativeCompletionCue(request)
	if len(flashes) != 1 || flashes[0].window != ownWindow || flashes[0].flags != 2 || flashes[0].count != 3 || flashes[0].timeout != 0 {
		t.Fatalf("native flash = %+v, want one finite taskbar-only flash of FairDrop's own window", flashes)
	}
	checkNoFlash := func(name string) {
		t.Helper()
		before := len(flashes)
		nativeCompletionCue(request)
		if len(flashes) != before {
			t.Errorf("%s flashed the taskbar", name)
		}
	}
	foreground = ownWindow
	checkNoFlash("foreground FairDrop")
	foreground = foreignWindow
	foregroundPID = 0
	checkNoFlash("unknown foreground owner")
	foregroundPID = foreignPID
	windowPID = foreignPID
	checkNoFlash("foreign candidate")
	windowPID = ownPID
	windowClass = "otherWindow"
	checkNoFlash("wrong window class")
	windowClass = "wailsWindow"
	owner = foreignWindow
	checkNoFlash("owned popup")
	owner = 0
	visible = false
	checkNoFlash("hidden window")
	visible = true
	alertFocus.Store(&windowsFocus{hwnd: foreignWindow, generation: 8, background: true})
	checkNoFlash("stale activation")
	alertFocus.Store(&windowsFocus{hwnd: foreignWindow, generation: 7, background: true})
	nativeCompletionInvalidate(3)
	checkNoFlash("disabled epoch")
	nativeCompletionInvalidate(2)
	alertFocus.Store(nil)
	checkNoFlash("observer unavailable")
}
