package stream

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

// Payload memory must stay O(buffer), so streaming a file an order of magnitude
// larger must not allocate an order of magnitude more. Deliberately sequential:
// ReadMemStats is process-wide.
func TestWriteToAllocationsDoNotGrowWithPayloadSize(t *testing.T) {
	// Both payloads sit well ABOVE the bound asserted below, which is what makes
	// each arm able to discriminate on its own: an implementation that read the
	// file into memory would exceed the bound at 2 MiB already, rather than
	// slipping under it and leaving only the large arm to catch the fault.
	const (
		smallPayload = 2 << 20
		largePayload = 32 << 20
	)

	smallAllocated := measureWriteToAllocations(t, smallPayload)
	largeAllocated := measureWriteToAllocations(t, largePayload)

	// Headroom around the single reusable buffer, but still far below the
	// smaller payload, so reading either file into memory fails this bound.
	bound := uint64(2 * defaultBufferSize)
	if smallAllocated > bound {
		t.Fatalf("streaming %d bytes allocated %d, want at most %d", smallPayload, smallAllocated, bound)
	}
	if largeAllocated > bound {
		t.Fatalf("streaming %d bytes allocated %d, want at most %d", largePayload, largeAllocated, bound)
	}

	growth := uint64(4 * defaultBufferSize)
	if largeAllocated > smallAllocated+growth {
		t.Fatalf(
			"streaming %d bytes allocated %d against %d for %d bytes: payload memory grew with the file",
			largePayload, largeAllocated, smallAllocated, smallPayload,
		)
	}
}

func measureWriteToAllocations(t *testing.T, size int) uint64 {
	t.Helper()

	path := filepath.Join(t.TempDir(), "bounded.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte("m"), size), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, err := source.New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("staging fixture failed: %v", err)
	}
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	// io.Discard's Write allocates nothing, so the delta is the payload path's
	// own allocation and nothing else.
	if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	runtime.ReadMemStats(&after)

	written, _ := prepared.Size()
	if written != int64(size) {
		t.Fatalf("Size() = %d, want the %d bytes on disk", written, size)
	}
	return after.TotalAlloc - before.TotalAlloc
}

// BenchmarkWriteToBufferSizes is the measurement behind defaultBufferSize. The
// buffer is a throughput choice only: every size here costs the same memory
// regardless of payload size, so the smallest size that stops improving wins.
func BenchmarkWriteToBufferSizes(b *testing.B) {
	const payloadSize = 32 << 20

	path := filepath.Join(b.TempDir(), "benchmark.bin")
	if err := os.WriteFile(path, bytes.Repeat([]byte("b"), payloadSize), 0o600); err != nil {
		b.Fatal(err)
	}
	staged, err := source.New().Inspect(context.Background(), path)
	if err != nil {
		b.Fatalf("staging fixture failed: %v", err)
	}

	sizes := map[string]int{
		"032KiB": 32 << 10,
		"064KiB": 64 << 10,
		"128KiB": 128 << 10,
		"256KiB": 256 << 10,
		"512KiB": 512 << 10,
		"1MiB":   1 << 20,
	}
	for name, size := range sizes {
		b.Run(name, func(b *testing.B) {
			adapter := &Payloads{source: source.New(), bufferSize: size}
			b.SetBytes(payloadSize)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				// Prepare walks every syntactic ancestor and opens the file, and
				// WriteTo is once-only, so each iteration needs a fresh payload.
				// Both it and Close stay outside the timed region: this benchmark
				// exists to compare buffer sizes, and per-iteration open/validate
				// overhead would compress exactly the differences being measured.
				b.StopTimer()
				prepared, err := adapter.Prepare(context.Background(), staged)
				if err != nil {
					b.Fatalf("Prepare() error = %v (code %q)", err, transfer.ErrorCodeOf(err))
				}
				b.StartTimer()

				if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
					b.Fatalf("WriteTo() error = %v", err)
				}

				b.StopTimer()
				if err := prepared.Close(); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
				b.StartTimer()
			}
		})
	}
}
