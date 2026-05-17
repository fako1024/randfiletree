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
)
