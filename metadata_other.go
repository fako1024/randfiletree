//go:build !linux

package randfiletree

import (
	"fmt"
	"os"
)

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

	return nil
}
