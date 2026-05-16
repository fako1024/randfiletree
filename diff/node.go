package diff

import "io/fs"

// Node denotes an element / node in a file tree.
type Node struct {
	Path       string
	LinkTarget string

	Size    int64
	Mode    fs.FileMode
	ModTime int64

	ModTimeNsec int64

	Hash []byte

	UID          uint32
	GID          uint32
	HasOwnership bool
}

type collectedPaths struct {
	nodes          []Node
	hardlinkGroups [][]string
}
