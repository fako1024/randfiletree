//go:build !linux

package diff

func collectPlatformMetadata(_ string, _ *Node) error {
	return nil
}
