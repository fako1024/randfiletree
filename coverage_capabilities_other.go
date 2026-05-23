//go:build !linux

package randfiletree

func coverageProbeXAttrSupport(probeBasePath string) bool {
	_ = probeBasePath
	return false
}

func coverageProbeACLSupport(probeBasePath string) bool {
	_ = probeBasePath
	return false
}
