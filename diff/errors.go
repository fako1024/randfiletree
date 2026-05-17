package diff

import "errors"

var (
	ErrXAttrComparatorNil = errors.New("xattr comparison enabled but xattr comparator is nil")
	ErrACLComparatorNil   = errors.New("ACL comparison enabled but ACL comparator is nil")
)
