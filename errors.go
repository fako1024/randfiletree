package randfiletree

import "errors"

var (
	ErrNilGenerator       = errors.New("nil generator")
	ErrEmptySymlinkTarget = errors.New("empty symlink target")

	ErrSymlinkStrategyProbabilitiesEmpty       = errors.New("symlink strategy probabilities must not be empty")
	ErrSymlinkStrategyProbabilitiesNonPositive = errors.New("sum of symlink strategy probabilities must be > 0")

	ErrSpecialFileTypeProbabilitiesEmpty       = errors.New("special file type probabilities must not be empty")
	ErrSpecialFileTypeProbabilitiesNonPositive = errors.New("sum of special file type probabilities must be > 0")
	ErrSpecialDeviceConfigurationIncomplete    = errors.New("special device configuration requires both major and minor generators")
	ErrSpecialDeviceNumbersRequired            = errors.New("special device generation requires major and minor number generators")

	ErrContentPatternProbabilitiesEmpty       = errors.New("content pattern probabilities must not be empty")
	ErrContentPatternProbabilitiesNonPositive = errors.New("sum of content pattern probabilities must be > 0")
	ErrContentPatternConfigurationIncomplete  = errors.New("content pattern configuration requires both pattern and logical size generators")

	ErrSpecialFilePermissionDenied = errors.New("insufficient privileges to create requested special file")
	ErrSpecialFileUnsupported      = errors.New("special file generation is unsupported on this filesystem or platform")
	ErrSpecialFileLinuxOnly        = errors.New("special file generation is only supported on linux")

	ErrMountPermissionDenied          = errors.New("insufficient privileges to perform mount operation")
	ErrMountUnsupported               = errors.New("mount operation is unsupported on this filesystem or platform")
	ErrMountLinuxOnly                 = errors.New("mount operations are only supported on linux")
	ErrCrossDeviceScenarioLinuxOnly   = errors.New("cross-device scenarios are only supported on linux")
	ErrCrossDeviceScenarioUnavailable = errors.New("cross-device scenario is unavailable in this environment")

	ErrFilesystemFeatureScenarioLinuxOnly   = errors.New("filesystem feature scenarios are only supported on linux")
	ErrFilesystemFeatureSelectionEmpty      = errors.New("filesystem feature scenario requires at least one opt-in feature")
	ErrFilesystemFeaturePermissionDenied    = errors.New("insufficient privileges to apply requested filesystem feature")
	ErrFilesystemFeatureUnsupported         = errors.New("filesystem feature is unsupported on this filesystem or platform")
	ErrFilesystemFeatureScenarioUnavailable = errors.New("filesystem feature scenario is unavailable in this environment")

	ErrContentPatternRepeatedBlockEmpty = errors.New("repeated-block content pattern requires non-empty data block")

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

	ErrACLMetadataUnsupported = errors.New("ACL metadata controls are only supported on linux")
	ErrACLPermissionDenied    = errors.New("insufficient privileges to set requested ACL")
	ErrACLUnsupported         = errors.New("ACL controls are unsupported on this filesystem or platform")
	ErrACLInvalidEntry        = errors.New("ACL entry is invalid")
	ErrACLEntryContainsNUL    = errors.New("ACL entry must not contain NUL bytes")
	ErrACLEntryEmpty          = errors.New("ACL entry must not be empty")
	ErrACLEntryContainsComma  = errors.New("ACL entry must not contain comma")

	ErrByteNameAlphabetInvalid = errors.New("byte name alphabet must not contain NUL or '/' bytes")

	ErrFaultInjected                 = errors.New("fault injected")
	ErrOperationStartIndexNegative   = errors.New("operation start index must be >= 0")
	ErrOperationStartIndexOutOfRange = errors.New("operation start index exceeds operation count")
)

func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	errMsg := errs[0].Error()
	for i := 1; i < len(errs); i++ {
		errMsg += "; " + errs[i].Error()
	}

	return errors.New(errMsg)
}
