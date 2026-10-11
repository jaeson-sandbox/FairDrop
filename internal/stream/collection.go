package stream

import (
	"context"
	"errors"
	"sync"
	"time"

	"fairdrop/internal/transfer"
)

// openMember is the resources of one collection member while it is the
// current one: a verified top-level file descriptor or one prepared directory
// pin, never both and never more than one member's worth at a time.
type openMember struct {
	kind    transfer.ItemKind
	name    string
	modTime time.Time
	file    *payload
	dir     transfer.PreparedDirectory
}

func (m *openMember) close() error {
	if m.file != nil {
		return m.file.Close()
	}
	if m.dir != nil {
		return m.dir.Close()
	}
	return nil
}

// lazyCollection prepares a collection's members one at a time, in archive
// order, and owns whichever single member is current.
//
// The invariant that matters is that at most one member holds a file
// descriptor or a directory pin at any moment, so the number of selected
// items never consumes the source's 64-handle budget. It is enforced
// structurally: install refuses to hold a second member, and the stream
// releases a member before asking for the next.
//
// Ownership: the current member is released either by the streaming worker
// (release) after it has been written, or by shutdown when the archive is
// closed first. Both paths are safe to run concurrently with each other and
// with repeated calls, because member close is idempotent. No lock is held
// across a native call.
type lazyCollection struct {
	payloads *Payloads
	members  []transfer.StagedItem
	width    int

	mu       sync.Mutex
	current  *openMember
	closed   bool
	closeErr error
}

// openFirst prepares member 0. It runs inside Payloads.Prepare, before any
// response header, so a failure is a clean pre-stream refusal with nothing
// held.
func (l *lazyCollection) openFirst(ctx context.Context) error {
	member, err := l.prepare(ctx, 0)
	if err != nil {
		return err
	}
	return l.install(member)
}

// next returns member index, preparing it now unless it is the first, which
// Prepare opened earlier. A preparation failure here happens after headers
// are on the wire, so the caller aborts the ZIP.
func (l *lazyCollection) next(ctx context.Context, index int) (*openMember, error) {
	if index == 0 {
		l.mu.Lock()
		member := l.current
		l.mu.Unlock()
		if member == nil {
			return nil, releasedBeforeStreaming()
		}
		return member, nil
	}
	member, err := l.prepare(ctx, index)
	if err != nil {
		// A deadline or cancellation is the stream's own interruption, not a
		// pre-header setup failure.
		if ctx.Err() != nil {
			return nil, contextError(ctx)
		}
		return nil, wrapUncodedSourceError(err)
	}
	if err := contextError(ctx); err != nil {
		l.discard(member)
		return nil, err
	}
	if err := l.install(member); err != nil {
		return nil, err
	}
	return member, nil
}

// prepare acquires member index's resources without installing them. It
// revalidates exactly as the singleton path does: a file's opened descriptor
// must match its Lstat and its staged kind, size and modification time; a
// directory takes a fresh no-follow identity pin that Walk later compares.
func (l *lazyCollection) prepare(ctx context.Context, index int) (*openMember, error) {
	item := l.members[index]
	name := collectionMemberName(item, index, l.width)
	switch item.Kind {
	case transfer.ItemFile:
		file, err := l.payloads.prepareFile(ctx, item)
		if err != nil {
			return nil, err
		}
		return &openMember{kind: item.Kind, name: name, modTime: item.ModTime, file: file}, nil
	case transfer.ItemDirectory:
		pin, err := l.payloads.source.PrepareDirectory(ctx, item.Path)
		if err != nil {
			return nil, wrapUncodedSourceError(err)
		}
		if pin == nil {
			return nil, transfer.NewError(transfer.ErrSetupFailed, "prepared directory is unavailable")
		}
		if err := prepareContextError(ctx); err != nil {
			_ = pin.Close()
			return nil, err
		}
		return &openMember{kind: item.Kind, name: name, modTime: item.ModTime, dir: pin}, nil
	default:
		return nil, transfer.NewError(transfer.ErrSetupFailed, "collection member is invalid")
	}
}

// install makes member the one current member. If the collection was closed
// while it was being prepared, or a previous member is somehow still held, the
// new member is closed again rather than leaked.
func (l *lazyCollection) install(member *openMember) error {
	l.mu.Lock()
	closed, held := l.closed, l.current != nil
	if !closed && !held {
		l.current = member
	}
	l.mu.Unlock()
	switch {
	case closed:
		return errors.Join(releasedBeforeStreaming(), member.close())
	case held:
		return errors.Join(
			transfer.NewError(transfer.ErrTransferFailed, "a previous collection member is still held"),
			member.close(),
		)
	}
	return nil
}

// discard closes a member that was prepared but will not be used, recording a
// close failure for Close to report.
func (l *lazyCollection) discard(member *openMember) {
	if err := member.close(); err != nil {
		l.recordCloseError(err)
	}
}

// release closes the current member once the stream is done with it, before
// the next member is prepared. A close failure is recorded and surfaces from
// the archive's Close rather than aborting a member that was already written.
func (l *lazyCollection) release(member *openMember) {
	l.mu.Lock()
	if l.current == member {
		l.current = nil
	}
	l.mu.Unlock()
	l.discard(member)
}

func (l *lazyCollection) recordCloseError(err error) {
	l.mu.Lock()
	l.closeErr = errors.Join(l.closeErr, err)
	l.mu.Unlock()
}

// shutdown releases whatever member is current, if any, and reports every
// close failure seen so far. It is the archive's Close, and is safe before
// the stream starts, during it, and after it.
func (l *lazyCollection) shutdown() error {
	l.mu.Lock()
	l.closed = true
	current := l.current
	l.current = nil
	prior := l.closeErr
	l.closeErr = nil
	l.mu.Unlock()
	var err error
	if current != nil {
		err = current.close()
	}
	return errors.Join(prior, err)
}

func releasedBeforeStreaming() error {
	return transfer.NewError(transfer.ErrTransferFailed, "payload was released before streaming")
}
