package server

import (
	"errors"
	"math"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// TestPercentIsAlwaysFiniteAndClamped covers the rule that keeps a snapshot
// serializable: NaN and infinity do not merely look wrong at the UI boundary,
// they fail JSON marshalling and lose the event entirely.
func TestPercentIsAlwaysFiniteAndClamped(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		sent  int64
		total int64
		known bool
		want  float64
	}{
		"half of a known total":      {sent: 50, total: 100, known: true, want: 50},
		"complete known total":       {sent: 100, total: 100, known: true, want: 100},
		"known empty total":          {sent: 0, total: 0, known: true, want: 0},
		"unknown total":              {sent: 4096, total: 0, known: false, want: 0},
		"unknown total with a size":  {sent: 4096, total: 8192, known: false, want: 0},
		"source grew past its bound": {sent: 200, total: 100, known: true, want: 100},
		"nothing sent yet":           {sent: 0, total: 100, known: true, want: 0},
		"negative total":             {sent: 10, total: -1, known: true, want: 0},
		"negative sent":              {sent: -10, total: 100, known: true, want: 0},
		"huge but finite":            {sent: math.MaxInt64, total: 1, known: true, want: 100},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := percentOf(test.sent, test.total, test.known)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("percentOf(%d, %d, %t) = %v, want a finite value", test.sent, test.total, test.known, got)
			}
			if got < 0 || got > 100 {
				t.Fatalf("percentOf(%d, %d, %t) = %v, want it clamped to [0,100]", test.sent, test.total, test.known, got)
			}
			if got != test.want {
				t.Fatalf("percentOf(%d, %d, %t) = %v, want %v", test.sent, test.total, test.known, got, test.want)
			}
		})
	}
}

// TestSpeedIsFiniteAndNonNegative keeps the same guarantee on the throughput
// field, including the zero-elapsed case a fast local transfer produces.
func TestSpeedIsFiniteAndNonNegative(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		sent    int64
		elapsed time.Duration
		want    float64
	}{
		"steady rate":     {sent: 1000, elapsed: time.Second, want: 1000},
		"half a second":   {sent: 500, elapsed: 500 * time.Millisecond, want: 1000},
		"no time elapsed": {sent: 1000, elapsed: 0, want: 0},
		"negative clock":  {sent: 1000, elapsed: -time.Second, want: 0},
		"nothing sent":    {sent: 0, elapsed: time.Second, want: 0},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := speedOf(test.sent, test.elapsed)
			if math.IsNaN(got) || math.IsInf(got, 0) || got < 0 {
				t.Fatalf("speedOf(%d, %v) = %v, want a finite non-negative rate", test.sent, test.elapsed, got)
			}
			if got != test.want {
				t.Fatalf("speedOf(%d, %v) = %v, want %v", test.sent, test.elapsed, got, test.want)
			}
		})
	}
}

// TestCountingWriterCountsOnlyAcceptedBytes is the wire-accuracy rule at its
// source: a destination that accepts a prefix, or fails outright, must not
// advance progress past what it took.
func TestCountingWriterCountsOnlyAcceptedBytes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		writer writerFunc
		want   int64
	}{
		"full write":  {writer: func(p []byte) (int, error) { return len(p), nil }, want: 16},
		"short write": {writer: func(p []byte) (int, error) { return len(p) / 2, nil }, want: 8},
		"failed write with a partial prefix": {
			writer: func(p []byte) (int, error) { return 4, errors.New("connection reset") },
			want:   4,
		},
		"failed write with nothing accepted": {
			writer: func([]byte) (int, error) { return 0, errors.New("connection reset") },
			want:   0,
		},
		"negative count from a broken writer": {
			writer: func([]byte) (int, error) { return -1, errors.New("broken") },
			want:   0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			meter := newMeter(16, true, newTestClock(0).now, nil)
			counting := &countingWriter{dst: test.writer, meter: meter}
			_, _ = counting.Write(make([]byte, 16))

			if got := meter.snapshot().BytesSent; got != test.want {
				t.Fatalf("BytesSent = %d, want %d", got, test.want)
			}
		})
	}
}

// writerFunc adapts a function to io.Writer so a test can hand the meter a
// destination that accepts less than it was offered.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// TestMeterCoalescesToFourPerSecond checks the cadence in isolation: the
// counter keeps counting between snapshots, so a dropped one loses nothing.
func TestMeterCoalescesToFourPerSecond(t *testing.T) {
	t.Parallel()

	var published []transfer.ProgressSnapshot
	clock := newTestClock(100 * time.Millisecond)
	meter := newMeter(1000, true, clock.now, func(snapshot transfer.ProgressSnapshot) {
		published = append(published, snapshot)
	})

	for range 9 {
		meter.record(100)
	}

	if len(published) != 3 {
		t.Fatalf("published %d snapshots across 900ms, want 3 under a 4 Hz cap", len(published))
	}
	for index, snapshot := range published {
		wantBytes := int64((index + 1) * 300)
		if snapshot.BytesSent != wantBytes {
			t.Fatalf("snapshot[%d].BytesSent = %d, want %d", index, snapshot.BytesSent, wantBytes)
		}
	}
	final := meter.snapshot()
	// The only assertion in the package on a speed the meter itself produced:
	// speedOf is unit-tested in isolation, so a snapshot that stopped
	// populating the field, or populated it from the wrong interval, would go
	// unnoticed everywhere else.
	if final.SpeedBytesPerSec <= 0 {
		t.Fatalf("SpeedBytesPerSec = %v, want a positive rate after %d bytes over a known interval",
			final.SpeedBytesPerSec, final.BytesSent)
	}
	if math.IsInf(final.SpeedBytesPerSec, 0) || math.IsNaN(final.SpeedBytesPerSec) {
		t.Fatalf("SpeedBytesPerSec = %v, want a finite rate", final.SpeedBytesPerSec)
	}
	if final.BytesSent != 900 || final.Percent != 90 {
		t.Fatalf("final snapshot = %+v, want 900 bytes at 90%%", final)
	}
}

// TestMeterNormalizesAnUnusableLength keeps a payload that reports a negative
// or unknown length from producing a total the UI would divide by.
func TestMeterNormalizesAnUnusableLength(t *testing.T) {
	t.Parallel()

	for name, total := range map[string]int64{"negative": -1, "unknown": 4096} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			known := name == "negative"
			meter := newMeter(total, known, newTestClock(0).now, nil)
			meter.record(128)

			snapshot := meter.snapshot()
			if snapshot.TotalKnown || snapshot.TotalBytes != 0 || snapshot.Percent != 0 {
				t.Fatalf("snapshot = %+v, want an unknown total with zeroed total and percent", snapshot)
			}
			if snapshot.BytesSent != 128 {
				t.Fatalf("BytesSent = %d, want 128", snapshot.BytesSent)
			}
		})
	}
}
