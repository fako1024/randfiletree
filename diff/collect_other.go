//go:build !linux

package diff

import "fmt"

func collectPlatformMetadata(path string, node *Node, opts Options) error {
	if opts.CompareSparseness {
		return fmt.Errorf("%w for `%s`: %w", ErrSparsenessCollectionUnsupported, path, ErrSparsenessMetadataUnavailable)
	}

	if opts.CompareXAttrs {
		return fmt.Errorf("%w for `%s`: %w", ErrXAttrCollectionUnsupported, path, ErrXAttrMetadataUnavailable)
	}

	if opts.CompareACLs {
		return fmt.Errorf("%w for `%s`: %w", ErrACLCollectionUnsupported, path, ErrACLMetadataUnavailable)
	}

	node.HasXAttrs = false
	node.HasACL = false
	node.HasAllocatedBlocks = false
	node.HasDevID = false

	return nil
}
