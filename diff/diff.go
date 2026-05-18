package diff

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
)

// Paths performs a recursive diff of two file trees / paths
// Any deviation will result in an error denoting the respective diff
func Paths(a, b string) error {
	return PathsWithOptions(a, b, DefaultOptions())
}

// PathsWithOptions performs a recursive diff of two file trees / paths
// using explicit comparison options.
func PathsWithOptions(a, b string, opts Options) error {
	if err := opts.validate(); err != nil {
		return fmt.Errorf("invalid diff options: %w", err)
	}

	pathsA, err := collectPaths(a, opts)
	if err != nil {
		return err
	}

	pathsB, err := collectPaths(b, opts)
	if err != nil {
		return err
	}

	if err := ensureOwnershipMetadata(pathsA.nodes, pathsB.nodes, opts); err != nil {
		return err
	}

	if err := ensureAccessTimeMetadata(pathsA.nodes, pathsB.nodes, opts); err != nil {
		return err
	}

	if err := ensureXAttrMetadata(pathsA.nodes, pathsB.nodes, opts); err != nil {
		return err
	}

	if err := ensureACLMetadata(pathsA.nodes, pathsB.nodes, opts); err != nil {
		return err
	}

	if diff := cmp.Diff(projectNodes(pathsA.nodes, opts), projectNodes(pathsB.nodes, opts)); diff != "" {
		return fmt.Errorf("mismatch (-want +got):\n%s", diff)
	}

	if opts.CompareHardlinkTopology {
		if diff := cmp.Diff(pathsA.hardlinkGroups, pathsB.hardlinkGroups); diff != "" {
			return fmt.Errorf("hardlink topology mismatch (-want +got):\n%s", diff)
		}
	}

	if err := runMetadataHooks(pathsA.nodes, pathsB.nodes, opts); err != nil {
		return err
	}

	return nil
}

func ensureOwnershipMetadata(nodesA, nodesB []Node, opts Options) error {
	if !opts.CompareOwnership {
		return nil
	}

	for _, node := range nodesA {
		if !node.HasOwnership {
			return fmt.Errorf("ownership comparison requested but metadata unavailable for left path `%s`", node.Path)
		}
	}

	for _, node := range nodesB {
		if !node.HasOwnership {
			return fmt.Errorf("ownership comparison requested but metadata unavailable for right path `%s`", node.Path)
		}
	}

	return nil
}

type projectedNode struct {
	Path       string
	LinkTarget string

	Size    int64
	Mode    uint32
	ModTime int64
	Atime   int64

	Hash []byte

	UID uint32
	GID uint32
}

func projectNodes(nodes []Node, opts Options) []projectedNode {
	projected := make([]projectedNode, len(nodes))

	for i, node := range nodes {
		projectedNode := projectedNode{
			Path:       node.Path,
			LinkTarget: node.LinkTarget,
			Size:       node.Size,
			Mode:       uint32(node.Mode),
			ModTime:    node.ModTime,
			Atime:      node.Atime,
		}

		if opts.TimestampPrecision == TimestampPrecisionNanoseconds {
			projectedNode.ModTime = node.ModTimeNsec
			projectedNode.Atime = node.AtimeNsec
		}

		if opts.CompareContentHash {
			projectedNode.Hash = node.Hash
		}

		if opts.CompareOwnership {
			projectedNode.UID = node.UID
			projectedNode.GID = node.GID
		}

		if !opts.CompareAccessTime {
			projectedNode.Atime = 0
		}

		projected[i] = projectedNode
	}

	return projected
}

func runMetadataHooks(nodesA, nodesB []Node, opts Options) error {
	if !opts.CompareXAttrs && !opts.CompareACLs {
		return nil
	}

	for i := range nodesA {
		if opts.CompareXAttrs {
			if diff := cmp.Diff(nodesA[i].XAttrs, nodesB[i].XAttrs); diff != "" {
				return fmt.Errorf("xattr mismatch for path `%s` (-want +got):\n%s", nodesA[i].Path, diff)
			}

			if opts.XAttrComparator != nil {
				if err := opts.XAttrComparator(nodesA[i].Path, nodesA[i], nodesB[i]); err != nil {
					return fmt.Errorf("xattr mismatch for path `%s`: %w", nodesA[i].Path, err)
				}
			}
		}

		if opts.CompareACLs {
			if diff := cmp.Diff(nodesA[i].ACLEntries, nodesB[i].ACLEntries); diff != "" {
				return fmt.Errorf("ACL mismatch for path `%s` (-want +got):\n%s", nodesA[i].Path, diff)
			}

			if opts.ACLComparator != nil {
				if err := opts.ACLComparator(nodesA[i].Path, nodesA[i], nodesB[i]); err != nil {
					return fmt.Errorf("ACL mismatch for path `%s`: %w", nodesA[i].Path, err)
				}
			}
		}
	}

	return nil
}

func ensureAccessTimeMetadata(nodesA, nodesB []Node, opts Options) error {
	if !opts.CompareAccessTime {
		return nil
	}

	for _, node := range nodesA {
		if !node.HasAccessTime {
			return fmt.Errorf("access-time comparison requested but metadata unavailable for left path `%s`", node.Path)
		}
	}

	for _, node := range nodesB {
		if !node.HasAccessTime {
			return fmt.Errorf("access-time comparison requested but metadata unavailable for right path `%s`", node.Path)
		}
	}

	return nil
}

func ensureXAttrMetadata(nodesA, nodesB []Node, opts Options) error {
	if !opts.CompareXAttrs {
		return nil
	}

	for _, node := range nodesA {
		if !node.HasXAttrs {
			return fmt.Errorf("%w for left path `%s`", ErrXAttrMetadataUnavailable, node.Path)
		}
	}

	for _, node := range nodesB {
		if !node.HasXAttrs {
			return fmt.Errorf("%w for right path `%s`", ErrXAttrMetadataUnavailable, node.Path)
		}
	}

	return nil
}

func ensureACLMetadata(nodesA, nodesB []Node, opts Options) error {
	if !opts.CompareACLs {
		return nil
	}

	for _, node := range nodesA {
		if !node.HasACL {
			return fmt.Errorf("%w for left path `%s`", ErrACLMetadataUnavailable, node.Path)
		}
	}

	for _, node := range nodesB {
		if !node.HasACL {
			return fmt.Errorf("%w for right path `%s`", ErrACLMetadataUnavailable, node.Path)
		}
	}

	return nil
}
