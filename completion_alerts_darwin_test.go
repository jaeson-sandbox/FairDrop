//go:build darwin && cgo

package main

import "testing"

func TestDarwinCompletionCueChecksFocusEpochAndCoalescesLatestMainQueueRequest(t *testing.T) {
	nativeAlertTestBegin()
	t.Cleanup(nativeAlertTestEnd)
	nativeCompletionInvalidate(1)
	first := nativeCompletionSnapshot()
	if !first.background || first.generation != 1 {
		t.Fatalf("initial focus snapshot = %+v", first)
	}
	nativeCompletionCue(alertRequest{epoch: 1, focus: first.generation})
	if nativeAlertTestCount() != 1 {
		t.Fatal("informational attention seam was not invoked")
	}
	nativeAlertTestFocus(true)
	foreground := nativeCompletionSnapshot()
	if foreground.background {
		t.Fatal("active FairDrop was reported as background")
	}
	nativeCompletionCue(alertRequest{epoch: 1, focus: foreground.generation})
	if nativeAlertTestCount() != 1 {
		t.Fatal("foreground FairDrop requested attention")
	}
	nativeAlertTestFocus(false)
	returned := nativeCompletionSnapshot()
	nativeCompletionCue(alertRequest{epoch: 1, focus: first.generation})
	if nativeAlertTestCount() != 1 {
		t.Fatal("old success replayed after return to foreground")
	}
	nativeCompletionCue(alertRequest{epoch: 1, focus: returned.generation})
	if nativeAlertTestCount() != 2 {
		t.Fatal("fresh background success was not delivered")
	}
	nativeAlertTestHold(true)
	nativeCompletionCue(alertRequest{epoch: 1, focus: returned.generation})
	nativeAlertTestFocus(true)
	nativeAlertTestFocus(false)
	latest := nativeCompletionSnapshot()
	nativeCompletionCue(alertRequest{epoch: 1, focus: latest.generation})
	nativeAlertTestFlush()
	if nativeAlertTestCount() != 3 {
		t.Fatal("the delayed main-queue slot swallowed a fresh success")
	}
	nativeCompletionCue(alertRequest{epoch: 1, focus: latest.generation})
	nativeCompletionInvalidate(2) // opt-out fences a request already in AppKit's queue.
	nativeAlertTestFlush()
	if nativeAlertTestCount() != 3 {
		t.Fatal("opt-out failed to fence delayed native work")
	}
	nativeCompletionInvalidate(3)
	nativeAlertTestHold(false)
	nativeCompletionCue(alertRequest{epoch: 3, focus: latest.generation})
	if nativeAlertTestCount() != 4 {
		t.Fatal("re-enabled fresh success was lost")
	}
}
