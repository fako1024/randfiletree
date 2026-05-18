package randfiletree

import "errors"

var (
	ErrNilGenerator       = errors.New("nil generator")
	ErrEmptySymlinkTarget = errors.New("empty symlink target")

	ErrSymlinkStrategyProbabilitiesEmpty       = errors.New("symlink strategy probabilities must not be empty")
	ErrSymlinkStrategyProbabilitiesNonPositive = errors.New("sum of symlink strategy probabilities must be > 0")

	ErrOwnershipMetadataConfigurationIncomplete = errors.New("ownership metadata configuration requires both uid and gid generators")
	ErrTimestampMetadataConfigurationIncomplete = errors.New("timestamp metadata configuration requires both atime and mtime generators")

	ErrOwnershipMetadataPermissionDenied = errors.New("insufficient privileges to set requested ownership metadata")
	ErrOwnershipMetadataUnsupported      = errors.New("ownership metadata controls are only supported on linux")
	ErrTimestampMetadataUnsupported      = errors.New("nanosecond timestamp metadata controls are only supported on linux")

	ErrXAttrNameEmpty            = errors.New("xattr name must not be empty")
	ErrXAttrNameContainsNUL      = errors.New("xattr name must not contain NUL bytes")
	ErrXAttrNameMissingNamespace = errors.New("xattr name must include namespace prefix")
	ErrXAttrNamespaceUnsupported = errors.New("xattr namespace is unsupported")
	ErrXAttrNamespaceNotAllowed  = errors.New("xattr namespace requires explicit opt-in")
	ErrXAttrPermissionDenied     = errors.New("insufficient privileges to set requested xattr")
	ErrXAttrUnsupported          = errors.New("xattr controls are unsupported on this filesystem or platform")
	ErrXAttrMetadataUnsupported  = errors.New("xattr metadata controls are only supported on linux")

	ErrACLMetadataUnsupported     = errors.New("ACL metadata controls are only supported on linux")
	ErrACLCommandBackendDisabled  = errors.New("ACL command backend must be explicitly enabled")
	ErrACLToolingUnavailable      = errors.New("ACL command tooling is unavailable")
	ErrACLPermissionDenied        = errors.New("insufficient privileges to set requested ACL")
	ErrACLUnsupported             = errors.New("ACL controls are unsupported on this filesystem or platform")
	ErrACLInvalidEntry            = errors.New("ACL entry is invalid")
	ErrACLEntryContainsNUL        = errors.New("ACL entry must not contain NUL bytes")
	ErrACLEntryEmpty              = errors.New("ACL entry must not be empty")
	ErrACLEntryContainsComma      = errors.New("ACL entry must not contain comma")
	ErrACLConfigurationIncomplete = errors.New("ACL metadata configuration requires command backend opt-in")
)
