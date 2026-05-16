//go:build windows

package diff

type fileIdentity struct{}

func fileIdentityFromPath(path string) (fileIdentity, bool) {
	return fileIdentity{}, false
}
