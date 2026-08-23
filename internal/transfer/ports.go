package transfer

import "context"

// SourcePort turns a selected path into a validated StagedItem.
//
// It is consumer-owned: the coordinator declares it here and internal/source
// implements it, so there is exactly one source interface in the module. The
// other Epic 1 ports (NetworkPort, ServerPort, QRPort, Observer) land with the
// stories that need them.
type SourcePort interface {
	// Inspect validates one absolute path and describes it without opening it,
	// following any link, or walking any directory.
	//
	// It returns the zero StagedItem and a coded error on every failure:
	// ErrInvalidSelection for an empty or non-absolute path, ErrPathNotFound
	// when the path no longer exists, ErrPathUnsupported for a directory,
	// symlink, junction/reparse point, special file, or otherwise unusable
	// path, and ErrCancelled when ctx is already done. Implementations honor
	// ctx before touching the filesystem.
	Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
}
