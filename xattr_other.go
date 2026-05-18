//go:build !linux

package randfiletree

import "fmt"

func setPathXAttr(path, name string, value []byte) error {
	if _, err := validateXAttrName(name); err != nil {
		return err
	}

	_ = value

	return fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
}

func removePathXAttr(path, name string) error {
	if _, err := validateXAttrName(name); err != nil {
		return err
	}

	return fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
}

func listPathXAttrNames(path string) ([]string, error) {
	return nil, fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
}

func getPathXAttr(path, name string) ([]byte, error) {
	if _, err := validateXAttrName(name); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("%w for `%s`", ErrXAttrMetadataUnsupported, path)
}

func scanPathXAttrSet(path string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
