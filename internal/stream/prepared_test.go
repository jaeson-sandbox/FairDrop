package stream

import (
	"context"
	"io"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

type testPreparedDirectory struct {
	walk  func(context.Context, transfer.SourceVisitor) error
	close func() error
}

func (p testPreparedDirectory) Walk(ctx context.Context, visit transfer.SourceVisitor) error {
	return p.walk(ctx, visit)
}
func (p testPreparedDirectory) Close() error {
	if p.close != nil {
		return p.close()
	}
	return nil
}

func (s *scriptedSource) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	if s.prepare != nil {
		return s.prepare(ctx, path)
	}
	return testPreparedDirectory{walk: func(ctx context.Context, visit transfer.SourceVisitor) error { return s.Walk(ctx, path, visit) }}, nil
}

func (f sourceFunc) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	return source.New().PrepareDirectory(ctx, path)
}

func (c *countingSource) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	p, err := c.inner.PrepareDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	return testPreparedDirectory{close: p.Close, walk: func(ctx context.Context, visit transfer.SourceVisitor) error {
		c.mu.Lock()
		c.walks++
		c.mu.Unlock()
		return p.Walk(ctx, visit)
	}}, nil
}

func (b *borrowTracker) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	p, err := b.inner.PrepareDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	return testPreparedDirectory{close: p.Close, walk: func(ctx context.Context, visit transfer.SourceVisitor) error {
		return p.Walk(ctx, func(entry transfer.SourceEntry, content io.Reader) error {
			if content != nil {
				b.lent.Add(1)
				b.mu.Lock()
				b.kept = append(b.kept, content)
				b.mu.Unlock()
			}
			return visit(entry, content)
		})
	}}, nil
}

func (s *countedWalkSource) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	p, err := s.SourcePort.PrepareDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	return testPreparedDirectory{close: p.Close, walk: func(ctx context.Context, visit transfer.SourceVisitor) error {
		s.walks.Add(1)
		return p.Walk(ctx, func(entry transfer.SourceEntry, content io.Reader) error {
			if content != nil {
				content = countedWalkReader{Reader: content, bytes: &s.bytes}
			}
			return visit(entry, content)
		})
	}}, nil
}
