package diff

import "io/fs"

// Node denotes an element / node in a file tree.
type Node struct {
	Path       string
	LinkTarget string

	Size    int64
	Mode    fs.FileMode
	ModTime int64
	Atime   int64

	ModTimeNsec int64
	AtimeNsec   int64

	Hash []byte

	UID           uint32
	GID           uint32
	HasOwnership  bool
	HasAccessTime bool

	XAttrs    []XAttr
	HasXAttrs bool

	ACLEntries []string
	HasACL     bool
}

// XAttr denotes a single extended attribute name/value pair.
type XAttr struct {
	Name  string
	Value []byte
}

type collectedPaths struct {
	nodes          []Node
	hardlinkGroups [][]string
}
