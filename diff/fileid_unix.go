//go:build !windows

package diff

import "golang.org/x/sys/unix"

type fileIdentity struct {
	dev uint64
	ino uint64
}

func fileIdentityFromPath(path string) (fileIdentity, bool) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return fileIdentity{}, false
	}

	return fileIdentity{dev: uint64(stat.Dev), ino: stat.Ino}, true
}
