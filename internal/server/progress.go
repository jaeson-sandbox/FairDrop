package server

import (
	"io"
	"math"
	"time"

	"fairdrop/internal/transfer"
)

// progressInterval is the 4 Hz cap expressed as a minimum spacing. Snapshots
// are coalesced, never queued: a byte counter's intermediate values carry no
// information the next snapshot does not already contain.
const progressInterval = 250 * time.Millisecond

// clock is the time seam. Cadence is behavior, so tests drive it with a
// deterministic clock instead of sleeping.
type clock func() time.Time

// meter is the wire-accurate byte counter behind every progress snapshot.
//
// It is owned by one serving goroutine and is not safe for concurrent use;
// that is deliberate, because the only thing allowed to advance it is a write
// the receiver's connection accepted.
type meter struct {
	total      int64
	totalKnown bool
	now        clock
	publish    func(transfer.ProgressSnapshot)

	started  time.Time
	lastEmit time.Time
	sent     int64
}

// newMeter normalizes the payload's advertised length and starts the clock.
// A length that is unknown or negative collapses to the contract's unknown
// form -- TotalKnown false, TotalBytes 0 -- so no downstream arithmetic ever
// sees a total it cannot divide by.
func newMeter(total int64, totalKnown bool, now clock, publish func(transfer.ProgressSnapshot)) *meter {
	if !totalKnown || total < 0 {
		total, totalKnown = 0, false
	}
	startedAt := now()
	return &meter{
		total:      total,
		totalKnown: totalKnown,
		now:        now,
		publish:    publish,
		started:    startedAt,
		lastEmit:   startedAt,
	}
}

// record accounts for bytes the destination accepted and emits a snapshot when
// the cadence allows one. Publishing is non-blocking, so the throttle bounds
// event volume rather than protecting the handler from a slow consumer.
func (m *meter) record(accepted int) {
	if accepted <= 0 {
		return
	}
	m.sent += int64(accepted)

	at := m.now()
	if at.Sub(m.lastEmit) < progressInterval {
		return
	}
	m.lastEmit = at
	if m.publish != nil {
		m.publish(m.snapshotAt(at))
	}
}

// snapshot is the authoritative terminal view: the byte count that actually
// reached the receiver, whether the transfer finished or failed.
func (m *meter) snapshot() transfer.ProgressSnapshot {
	return m.snapshotAt(m.now())
}

func (m *meter) snapshotAt(at time.Time) transfer.ProgressSnapshot {
	return transfer.ProgressSnapshot{
		BytesSent:        m.sent,
		TotalBytes:       m.total,
		TotalKnown:       m.totalKnown,
		Percent:          percentOf(m.sent, m.total, m.totalKnown),
		SpeedBytesPerSec: speedOf(m.sent, at.Sub(m.started)),
	}
}

// percentOf implements the contract's percentage rule. An unknown total and a
// known empty total both report zero -- there is no fraction of nothing -- and
// every other result is finite and clamped, because NaN and infinity would
// fail JSON marshalling at the UI boundary rather than merely look wrong.
func percentOf(sent, total int64, totalKnown bool) float64 {
	if !totalKnown || total <= 0 || sent <= 0 {
		return 0
	}
	percent := 100 * float64(sent) / float64(total)
	switch {
	case math.IsNaN(percent), percent < 0:
		return 0
	case math.IsInf(percent, 0), percent > 100:
		return 100
	}
	return percent
}

// speedOf is the average rate over the whole stream so far. It is presentation
// only -- the UI shows throughput visually and never speaks it -- so a stable
// average is preferred to a twitchy instantaneous rate. A zero-length or
// not-yet-elapsed stream reports zero rather than dividing by it.
func speedOf(sent int64, elapsed time.Duration) float64 {
	if sent <= 0 || elapsed <= 0 {
		return 0
	}
	speed := float64(sent) / elapsed.Seconds()
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 0 {
		return 0
	}
	return speed
}

// countingWriter is the seam between the payload and the receiver's
// connection. It counts what the destination accepted, never what the payload
// offered: a short write, or a write that fails partway, advances the meter by
// the accepted prefix only, so progress can never claim bytes the receiver
// did not get.
type countingWriter struct {
	dst   io.Writer
	meter *meter
}

func (c *countingWriter) Write(p []byte) (int, error) {
	accepted, err := c.dst.Write(p)
	if accepted > 0 {
		c.meter.record(accepted)
	}
	return accepted, err
}
