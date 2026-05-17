package randfiletree

import "errors"

var (
	ErrNilGenerator       = errors.New("nil generator")
	ErrEmptySymlinkTarget = errors.New("empty symlink target")

	ErrSymlinkStrategyProbabilitiesEmpty       = errors.New("symlink strategy probabilities must not be empty")
	ErrSymlinkStrategyProbabilitiesNonPositive = errors.New("sum of symlink strategy probabilities must be > 0")
)
