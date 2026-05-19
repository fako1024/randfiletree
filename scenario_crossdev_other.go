//go:build !linux

package randfiletree

import "fmt"

// DeviceRoot describes a scenario root and its backing filesystem device ID.
type DeviceRoot struct {
	Path     string
	DeviceID uint64
}

// CrossDeviceScenario describes two roots on different devices under a shared base path.
type CrossDeviceScenario struct {
	BasePath  string
	Primary   DeviceRoot
	Secondary DeviceRoot
}

// SetupCrossDeviceScenario initializes a deterministic two-root cross-device scenario.
func SetupCrossDeviceScenario(basePath string) (*CrossDeviceScenario, error) {
	return nil, fmt.Errorf("failed to setup cross-device scenario for base `%s`: %w", basePath, ErrCrossDeviceScenarioLinuxOnly)
}

// IsCrossDevice indicates whether primary and secondary roots are on different devices.
func (s *CrossDeviceScenario) IsCrossDevice() bool {
	if s == nil {
		return false
	}

	return s.Primary.DeviceID != s.Secondary.DeviceID
}

// Close tears down mounted resources created for the scenario.
func (s *CrossDeviceScenario) Close() error {
	return nil
}
