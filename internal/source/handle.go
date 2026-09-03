package source

import "io/fs"

// pathPlan is a parsed native absolute path. Components retain the caller's
// spelling; dot and dot-dot are interpreted by the handle stack, never by a
// path-cleaning function.
type pathPlan struct {
	anchor         string
	components     []string
	rootLabel      string
	hadTrailingSep bool
}

// metadataHandle owns a no-follow handle that can read metadata but cannot
// read file contents or enumerate a directory.
type metadataHandle interface {
	Stat() (fs.FileInfo, error)
	OpenChildMetadata(name string) (metadataHandle, error)
	OpenSearch() (metadataHandle, error)
	OpenEnumeration() (directoryHandle, error)
	Close() error
}

// directoryHandle adds fixed-batch enumeration to the metadata operations.
// Child opens are relative to this already-open directory.
type directoryHandle interface {
	metadataHandle
	ReadDir(n int) ([]fs.DirEntry, error)
}

type handleFactory interface {
	Parse(string) (pathPlan, error)
	OpenAnchor(pathPlan) (metadataHandle, error)
}

func productionHandleFactory() handleFactory { return nativeHandleFactory{} }
