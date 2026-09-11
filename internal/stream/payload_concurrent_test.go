package stream

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

type heldDestination struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	body    bytes.Buffer
}

type countedPayloadFile struct {
	payloadFile
	reads atomic.Int64
}

func (f *countedPayloadFile) Read(p []byte) (int, error) {
	f.reads.Add(1)
	return f.payloadFile.Read(p)
}

func (w *heldDestination) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return w.body.Write(p)
}

func TestWriteToConcurrentCallersStreamExactlyOnce(t *testing.T) {
	for _, folder := range []bool{false, true} {
		name := "file"
		if folder {
			name = "folder"
		}
		t.Run(name, func(t *testing.T) {
			selected := writeFile(t, "concurrent.bin", []byte("single winner"))
			if folder {
				selected = fixtureDir(t)
				writeTree(t, selected, map[string]string{"content.txt": "single winner"})
			}
			prepared, err := New(source.New()).Prepare(context.Background(), stage(t, selected))
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			var counted *countedPayloadFile
			if !folder {
				filePayload := prepared.(*payload)
				counted = &countedPayloadFile{payloadFile: filePayload.file}
				filePayload.file = counted
			}
			destination := &heldDestination{entered: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(destination.release) }) }
			defer release()
			first := make(chan error, 1)
			go func() { first <- prepared.WriteTo(context.Background(), destination) }()
			select {
			case <-destination.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("first WriteTo never reached destination")
			}
			// The first caller is definitely still streaming, not merely done
			// before the second begins. The loser must return without writing.
			var refused bytes.Buffer
			var before int64
			if counted != nil {
				before = counted.reads.Load()
			}
			second := make(chan error, 1)
			go func() { second <- prepared.WriteTo(context.Background(), &refused) }()
			select {
			case err := <-second:
				// A second read at EOF also returns transfer_failed with zero
				// output. That is not an ownership refusal: it consumed the
				// descriptor and would steal bytes from a larger first stream.
				if counted != nil && counted.reads.Load() != before {
					t.Fatal("concurrent second WriteTo read the file instead of refusing ownership")
				}
				if transfer.ErrorCodeOf(err) != transfer.ErrTransferFailed || refused.Len() != 0 {
					t.Fatal("concurrent second WriteTo was not refused before writing")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("second WriteTo blocked behind first caller")
			}
			release()
			if err := <-first; err != nil {
				t.Fatalf("winning WriteTo failed: %v", err)
			}
			if folder {
				assertEntryContents(t, openArchive(t, destination.body.Bytes()), prepared.DownloadName()[:len(prepared.DownloadName())-4]+"/content.txt", "single winner")
			} else if destination.body.String() != "single winner" {
				t.Fatal("winning payload differs")
			}
		})
	}
}
