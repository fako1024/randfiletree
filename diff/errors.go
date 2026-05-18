package diff

import "errors"

var (
	ErrXAttrComparatorNil = errors.New("xattr comparison enabled but xattr comparator is nil")
	ErrACLComparatorNil   = errors.New("ACL comparison enabled but ACL comparator is nil")

	ErrXAttrMetadataUnavailable      = errors.New("xattr comparison requested but metadata unavailable")
	ErrACLMetadataUnavailable        = errors.New("ACL comparison requested but metadata unavailable")
	ErrSparsenessMetadataUnavailable = errors.New("sparseness comparison requested but metadata unavailable")

	ErrXAttrCollectionUnsupported      = errors.New("xattr metadata collection unsupported")
	ErrACLCollectionUnsupported        = errors.New("ACL metadata collection unsupported")
	ErrSparsenessCollectionUnsupported = errors.New("sparseness metadata collection unsupported")
)
