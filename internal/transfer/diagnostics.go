package transfer

import "sync"

// This file holds the bounded diagnostics ring buffer: an internal cleanup
// record the coordinator writes to and a test (or the injected Diagnose seam)
// reads from. It never carries adapter text, only a stable code and a message
// this package chose (AD-9).

const (
	// maxDiagnostics bounds the internal cleanup record. A session produces a
	// handful at most, and the sink exists to be inspected, not to grow.
	maxDiagnostics = 32
)

// diagnostic is one internal cleanup note. It carries a stable code and a
// message this package chose, never adapter text: adapter text is exactly
// where absolute paths and capability tokens live.
type diagnostic struct {
	code    ErrorCode
	message string
}

type diagnosticSink struct {
	mu         sync.Mutex
	entries    []diagnostic
	overflowed bool
}

// diagnosticOverflow replaces whatever entry would have been silently
// dropped once the sink is full. A truncated sink is indistinguishable from
// a complete one to anything that reads it, and this package's whole reason
// for keeping the sink is to be inspected (D-031) -- so the last slot is
// reserved for saying so, once, rather than left to keep dropping silently
// forever after.
var diagnosticOverflow = diagnostic{
	code:    ErrTransferFailed,
	message: "further diagnostics were dropped: the sink reached its limit",
}

func (s *diagnosticSink) record(entry diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overflowed {
		return
	}
	if len(s.entries) >= maxDiagnostics-1 {
		s.entries = append(s.entries, diagnosticOverflow)
		s.overflowed = true
		return
	}
	s.entries = append(s.entries, entry)
}

func (s *diagnosticSink) snapshot() []diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]diagnostic, len(s.entries))
	copy(out, s.entries)
	return out
}
