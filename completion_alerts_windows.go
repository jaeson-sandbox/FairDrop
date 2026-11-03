//go:build windows

package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var alertEpoch atomic.Uint64
var alertUser32 = syscall.NewLazyDLL("user32.dll")
var alertEnumWindows = alertUser32.NewProc("EnumWindows")
var alertGetClassName = alertUser32.NewProc("GetClassNameW")
var alertGetWindowPID = alertUser32.NewProc("GetWindowThreadProcessId")
var alertGetForeground = alertUser32.NewProc("GetForegroundWindow")
var alertGetWindow = alertUser32.NewProc("GetWindow")
var alertIsWindowVisible = alertUser32.NewProc("IsWindowVisible")
var alertFlash = alertUser32.NewProc("FlashWindowEx")
var alertGetCurrentPID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcessId")
var alertSetWinEventHook = alertUser32.NewProc("SetWinEventHook")
var alertGetMessage = alertUser32.NewProc("GetMessageW")
var alertPeekMessage = alertUser32.NewProc("PeekMessageW")
var alertDispatchMessage = alertUser32.NewProc("DispatchMessageW")
var alertPostThreadMessage = alertUser32.NewProc("PostThreadMessageW")
var alertUnhookWinEvent = alertUser32.NewProc("UnhookWinEvent")
var alertGetCurrentThreadID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
var alertObserverOnce sync.Once
var alertObserverStarted atomic.Bool
var alertObserverDone = make(chan struct{})
var alertFocus atomic.Pointer[windowsFocus]
var alertThreadID atomic.Uint32
var alertObserverStopped atomic.Bool
var alertPostQuit = func(thread uint32) { alertPostThreadMessage.Call(uintptr(thread), 0x12, 0, 0) }

type windowsObserverPump struct {
	hook       func() uintptr
	unhook     func(uintptr)
	peek       func()
	threadID   func() uint32
	foreground func() uintptr
	getMessage func() int32
	dispatch   func()
}

type windowsFocus struct {
	hwnd       uintptr
	generation uint64
	known      bool
	background bool
}

var alertFocusCallback = syscall.NewCallback(func(_, _, hwnd, _, _, _, _ uintptr) uintptr {
	alertSample(hwnd)
	return 0
})

func alertSample(hwnd uintptr) {
	foregroundPID := alertWindowPIDFunc(hwnd)
	own := alertCurrentPIDFunc()
	known := hwnd != 0 && foregroundPID != 0 && own != 0
	background := known && foregroundPID != own
	previous := alertFocus.Load()
	generation := uint64(1)
	if previous != nil {
		generation = previous.generation
		if previous.known != known || previous.background != background {
			generation++
		}
	}
	alertFocus.Store(&windowsFocus{hwnd: hwnd, generation: generation, known: known, background: background})
}

var alertCandidate atomic.Uintptr
var alertEnumCallback = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
	pid, _, _ := alertGetCurrentPID.Call()
	if pid == 0 {
		return 1
	}
	var windowPID uint32
	alertGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
	if uintptr(windowPID) != pid {
		return 1
	}
	owner, _, _ := alertGetWindow.Call(hwnd, 4) // GW_OWNER: top-level window only.
	if owner != 0 {
		return 1
	}
	var class [64]uint16
	length, _, _ := alertGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	if length != 0 && syscall.UTF16ToString(class[:]) == "wailsWindow" {
		alertCandidate.Store(hwnd)
		return 0
	}
	return 1
})

// The immediate native API seam is replaceable in Windows tests. Production
// still resolves the HWND through EnumWindows and rechecks its identity just
// before FlashWindowEx.
var alertCurrentPIDFunc = func() uintptr { pid, _, _ := alertGetCurrentPID.Call(); return pid }
var alertOwnWindowFunc = func() uintptr {
	alertCandidate.Store(0)
	alertEnumWindows.Call(alertEnumCallback, 0)
	return alertCandidate.Load()
}
var alertWindowPIDFunc = func(hwnd uintptr) uintptr {
	var pid uint32
	if hwnd != 0 {
		alertGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	}
	return uintptr(pid)
}
var alertWindowClassFunc = func(hwnd uintptr) string {
	var class [64]uint16
	length, _, _ := alertGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(class[:])
}
var alertWindowOwnerFunc = func(hwnd uintptr) uintptr { owner, _, _ := alertGetWindow.Call(hwnd, 4); return owner }
var alertVisibleFunc = func(hwnd uintptr) bool { visible, _, _ := alertIsWindowVisible.Call(hwnd); return visible != 0 }
var alertForegroundFunc = func() uintptr { hwnd, _, _ := alertGetForeground.Call(); return hwnd }
var alertFlashFunc = func(info *flashInfo) { alertFlash.Call(uintptr(unsafe.Pointer(info))) }

const flashTray = 0x00000002

type flashInfo struct {
	size    uint32
	window  uintptr
	flags   uint32
	count   uint32
	timeout uint32
}

func nativeCompletionInvalidate(epoch uint64) { alertEpoch.Store(epoch) }
func nativeCompletionStart()                  {}
func nativeCompletionEnable() {
	alertObserverOnce.Do(func() {
		alertObserverStarted.Store(true)
		go func() { defer close(alertObserverDone); alertObserveForeground() }()
	})
}
func nativeCompletionStop() {
	alertObserverStopped.Store(true)
	alertFocus.Store(nil)
	if thread := alertThreadID.Load(); thread != 0 {
		alertPostQuit(thread)
	}
}
func alertObserveForeground() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var message [64]byte
	pump := windowsObserverPump{
		hook: func() uintptr {
			hook, _, _ := alertSetWinEventHook.Call(3, 3, 0, alertFocusCallback, 0, 0, 0)
			return hook
		},
		unhook:     func(hook uintptr) { alertUnhookWinEvent.Call(hook) },
		peek:       func() { alertPeekMessage.Call(uintptr(unsafe.Pointer(&message[0])), 0, 0, 0, 0) },
		threadID:   func() uint32 { thread, _, _ := alertGetCurrentThreadID.Call(); return uint32(thread) },
		foreground: func() uintptr { foreground, _, _ := alertGetForeground.Call(); return foreground },
		getMessage: func() int32 {
			got, _, _ := alertGetMessage.Call(uintptr(unsafe.Pointer(&message[0])), 0, 0, 0)
			return int32(got)
		},
		dispatch: func() { alertDispatchMessage.Call(uintptr(unsafe.Pointer(&message[0]))) },
	}
	runWindowsForegroundObserver(pump)
}

func runWindowsForegroundObserver(pump windowsObserverPump) {
	// A failed registration, WM_QUIT or unexpected GetMessage error must all
	// revoke the last observed background state before this thread exits.
	defer alertFocus.Store(nil)
	if alertObserverStopped.Load() {
		return
	}
	// WinEvent callbacks run on the thread that installed the hook. No polling
	// occurs during a default-off run or while the foreground is stationary.
	hook := pump.hook()
	if hook == 0 {
		return
	}
	defer pump.unhook(hook)
	pump.peek()
	alertThreadID.Store(pump.threadID())
	defer alertThreadID.Store(0)
	if alertObserverStopped.Load() {
		return
	}
	alertSample(pump.foreground())
	for {
		got := pump.getMessage()
		if got == 0 || got == -1 {
			return
		}
		pump.dispatch()
	}
}
func nativeCompletionSnapshot() focusSnapshot {
	state := alertFocus.Load()
	if state == nil || alertObserverStopped.Load() {
		return focusSnapshot{}
	}
	return focusSnapshot{generation: state.generation, background: state.background}
}

func nativeCompletionCue(request alertRequest) {
	pid := alertCurrentPIDFunc()
	if pid == 0 {
		return
	}
	ownWindow := alertOwnWindowFunc()
	if ownWindow == 0 || alertEpoch.Load() != request.epoch {
		return
	}
	if alertWindowPIDFunc(ownWindow) != pid || alertWindowClassFunc(ownWindow) != "wailsWindow" ||
		alertWindowOwnerFunc(ownWindow) != 0 || !alertVisibleFunc(ownWindow) {
		return
	}
	foreground := alertForegroundFunc()
	if foreground == ownWindow {
		return
	}
	foregroundPID := alertWindowPIDFunc(foreground)
	focus := nativeCompletionSnapshot()
	if foreground == 0 || foregroundPID == 0 || foregroundPID == pid ||
		!focus.background || focus.generation != request.focus || alertEpoch.Load() != request.epoch {
		return
	}
	info := flashInfo{size: uint32(unsafe.Sizeof(flashInfo{})), window: ownWindow, flags: flashTray, count: 3}
	// The return reports prior caption activity, not success or failure.
	alertFlashFunc(&info)
}
