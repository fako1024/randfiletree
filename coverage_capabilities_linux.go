//go:build linux

package randfiletree

import (
	"errors"
	"os"
	"path/filepath"
)

func coverageProbeXAttrSupport(probeBasePath string) bool {
	if err := os.MkdirAll(probeBasePath, 0o755); err != nil {
		return false
	}

	target := filepath.Join(probeBasePath, ".randfiletree-coverage-xattr-probe")
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	defer func() {
		_ = os.Remove(target)
	}()

	if err := setPathXAttr(target, "user.randfiletree-probe", []byte("v")); err != nil {
		if errors.Is(err, ErrXAttrUnsupported) || errors.Is(err, ErrXAttrPermissionDenied) {
			return false
		}
		return false
	}
	_ = removePathXAttr(target, "user.randfiletree-probe")

	return true
}

func coverageProbeACLSupport(probeBasePath string) bool {
	if err := os.MkdirAll(probeBasePath, 0o755); err != nil {
		return false
	}

	target := filepath.Join(probeBasePath, ".randfiletree-coverage-acl-probe")
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	defer func() {
		_ = os.Remove(target)
	}()

	if err := applyACL(target, 0o600, []string{"u::rw-", "g::---", "o::---"}); err != nil {
		if errors.Is(err, ErrACLUnsupported) || errors.Is(err, ErrACLPermissionDenied) {
			return false
		}
		return false
	}

	return true
}
