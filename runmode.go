package randfiletree

import "fmt"

// RunMode denotes how a planned run is applied to an existing base path.
type RunMode uint8

const (
	// RunModeAppend adds missing entries while allowing existing matching paths.
	RunModeAppend RunMode = iota

	// RunModeStrict fails when any planned path already exists.
	RunModeStrict

	// RunModeReplace clears the base path before applying the plan.
	RunModeReplace
)

func (m RunMode) String() string {
	switch m {
	case RunModeAppend:
		return "append"
	case RunModeStrict:
		return "strict"
	case RunModeReplace:
		return "replace"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

func validateRunMode(mode RunMode) error {
	switch mode {
	case RunModeAppend, RunModeStrict, RunModeReplace:
		return nil
	default:
		return fmt.Errorf("run mode must be one of append, strict, replace, got %d", mode)
	}
}
