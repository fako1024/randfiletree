//go:build !linux

package randfiletree

import (
	"fmt"
	"os"
)

// applyLinkMetadata is a no-op on non-Linux platforms: symlink metadata
// controls are only honored by the Linux helpers, and the existing
// non-Linux paths return explicit unsupported errors when a metadata
// payload is requested.
func applyLinkMetadata(path string, metadata metadataConfig) error {
	if metadata.hasOwnership {
		return fmt.Errorf("%w for `%s`", ErrOwnershipMetadataUnsupported, path)
	}
	if metadata.hasTimestamps {
		return fmt.Errorf("%w for `%s`", ErrTimestampMetadataUnsupported, path)
	}
	if metadata.hasXAttrs {
		return fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
	}
	if metadata.hasACL {
		return fmt.Errorf("%w for `%s`", ErrACLMetadataUnsupported, path)
	}
	return nil
}

func applyMetadata(path string, mode uint32, metadata metadataConfig) error {
	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to chmod `%s` to %#o: %w", path, mode, err)
	}

	if metadata.hasOwnership {
		return fmt.Errorf("%w for `%s`", ErrOwnershipMetadataUnsupported, path)
	}

	if metadata.hasTimestamps {
		return fmt.Errorf("%w for `%s`", ErrTimestampMetadataUnsupported, path)
	}

	if metadata.hasXAttrs {
		return fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
	}

	if metadata.hasACL {
		return fmt.Errorf("%w for `%s`", ErrACLMetadataUnsupported, path)
	}

	return nil
}
