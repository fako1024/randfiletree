//go:build linux

package diff

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func collectPlatformMetadata(path string, node *Node) error {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return fmt.Errorf("failed to stat path `%s`: %w", path, err)
	}

	node.UID = stat.Uid
	node.GID = stat.Gid
	node.HasOwnership = true
	node.ModTimeNsec = unix.TimespecToNsec(stat.Mtim)

	return nil
}
