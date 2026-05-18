//go:build !linux

package randfiletree

import "fmt"

func createPlannedSpecialFile(path string, fileType SpecialFileType, mode uint32, major, minor int) error {
	if err := validateSpecialFileType(fileType); err != nil {
		return err
	}

	_ = mode
	_ = major
	_ = minor

	return fmt.Errorf("%w for `%s` (%s)", ErrSpecialFileLinuxOnly, path, fileType)
}
