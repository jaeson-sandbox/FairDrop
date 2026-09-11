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

// statHandle describes an object through a metadata snapshot or an opened
// descriptor. Identity and kind checks compare the snapshot with later opens.
type statHandle interface {
	Stat() (fs.FileInfo, error)
}

// metadataHandle owns a no-follow metadata view that cannot read contents or
// enumerate a directory. Linux/Windows pin a descriptor; Darwin snapshots stat
// metadata relative to an owned ancestor descriptor and does not pin the leaf.
// Later search/enumeration/content opens must pass an identity check. No view
// exposes Read; Close releases its resources without closing borrowed parents.
type metadataHandle interface {
	statHandle
	OpenChildMetadata(name string) (metadataHandle, error)
	OpenSearch() (metadataHandle, error)
	OpenEnumeration() (directoryHandle, error)
	Close() error
}

// directoryHandle adds fixed-batch enumeration and the one content open in the
// package to the metadata operations. Child opens are relative to this
// already-open directory, so only a directory this traversal has already
// validated and opened can produce a readable child.
type directoryHandle interface {
	metadataHandle
	ReadDir(n int) ([]fs.DirEntry, error)
	OpenChildContent(name string) (contentHandle, error)
}

// contentHandle is the only handle in this package that can read bytes. It is
// opened parent-relative and no-follow, it refuses anything that is not a
// regular file before it can block or read, and its owner closes it before the
// traversal moves on.
type contentHandle interface {
	statHandle
	Read(p []byte) (int, error)
	Close() error
}

type handleFactory interface {
	Parse(string) (pathPlan, error)
	OpenAnchor(pathPlan) (metadataHandle, error)
}

func productionHandleFactory() handleFactory { return nativeHandleFactory{} }
