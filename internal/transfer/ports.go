package transfer

import "context"

// SourcePort validates and describes a selected source path without opening
// its contents.
type SourcePort interface {
	Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
}
