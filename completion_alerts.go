package main

import "sync"

type alertRequest struct{ epoch, focus uint64 }
type focusSnapshot struct {
	generation uint64
	background bool
}

// completionAlerts owns run-local eligibility. A single worker and one pending
// slot keep a stalled OS call away from the coordinator's event lane.
type completionAlerts struct {
	mu            sync.Mutex
	enabled       bool
	closed        bool
	workerStarted bool
	epoch         uint64
	lastSession   string
	pending       chan alertRequest
	cue           func(alertRequest)
	invalidate    func(uint64)
	focus         func() focusSnapshot
}

func newCompletionAlerts(cue func(alertRequest), invalidate func(uint64), focus func() focusSnapshot) *completionAlerts {
	a := &completionAlerts{pending: make(chan alertRequest, 1), cue: cue, invalidate: invalidate, focus: focus}
	invalidate(0)
	return a
}

func (a *completionAlerts) valid(request alertRequest) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || !a.enabled || a.epoch != request.epoch {
		return false
	}
	focus := a.focus()
	return focus.background && focus.generation == request.focus
}

func (a *completionAlerts) set(enabled bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return false
	}
	if a.enabled != enabled {
		a.enabled = enabled
		a.epoch++
		a.invalidate(a.epoch)
		select {
		case <-a.pending:
		default:
		}
	}
	if enabled && !a.workerStarted {
		a.workerStarted = true
		go func() {
			for request := range a.pending {
				if a.valid(request) {
					a.deliver(request)
				}
			}
		}()
	}
	return a.enabled
}

func (a *completionAlerts) get() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enabled && !a.closed
}

func (a *completionAlerts) complete(session string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || !a.enabled || session == "" || session == a.lastSession {
		return false
	}
	a.lastSession = session
	focus := a.focus()
	if !focus.background {
		return false
	}
	request := alertRequest{epoch: a.epoch, focus: focus.generation}
	// The only concurrent consumer removes. Every producer holds mu, so after
	// this nonblocking drain the capacity-one channel has room for one send.
	select {
	case <-a.pending:
	default:
	}
	a.pending <- request
	return true
}

func (a *completionAlerts) deliver(request alertRequest) {
	defer func() { _ = recover() }()
	a.cue(request)
}

func (a *completionAlerts) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	a.closed = true
	a.enabled = false
	a.epoch++
	a.invalidate(a.epoch)
	close(a.pending)
}
