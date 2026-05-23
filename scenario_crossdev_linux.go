//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

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

	secondaryMounted bool
	bindSourcePath   string
}

// SetupCrossDeviceScenario initializes a deterministic two-root cross-device scenario.
func SetupCrossDeviceScenario(basePath string) (*CrossDeviceScenario, error) {
	if strings.TrimSpace(basePath) == "" {
		return nil, ErrBasePathEmpty
	}

	if err := ensureNoSymlinkPathComponents(basePath); err != nil {
		return nil, err
	}

	scenario := &CrossDeviceScenario{
		BasePath: filepath.Clean(basePath),
	}

	if err := os.MkdirAll(scenario.BasePath, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cross-device scenario base `%s`: %w", scenario.BasePath, err)
	}

	if err := ensureScenarioDirectory(scenario.BasePath, "base path"); err != nil {
		return nil, err
	}

	scenario.Primary.Path = filepath.Join(scenario.BasePath, "left")
	scenario.Secondary.Path = filepath.Join(scenario.BasePath, "right")

	if err := os.MkdirAll(scenario.Primary.Path, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cross-device scenario primary root `%s`: %w", scenario.Primary.Path, err)
	}
	if err := os.MkdirAll(scenario.Secondary.Path, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cross-device scenario secondary root `%s`: %w", scenario.Secondary.Path, err)
	}

	if err := ensureScenarioDirectory(scenario.Primary.Path, "primary root"); err != nil {
		return nil, err
	}
	if err := ensureScenarioDirectory(scenario.Secondary.Path, "secondary root"); err != nil {
		return nil, err
	}

	primaryDev, err := pathDeviceID(scenario.Primary.Path)
	if err != nil {
		return nil, err
	}
	secondaryDev, err := pathDeviceID(scenario.Secondary.Path)
	if err != nil {
		return nil, err
	}

	scenario.Primary.DeviceID = primaryDev
	scenario.Secondary.DeviceID = secondaryDev

	if scenario.IsCrossDevice() {
		return scenario, nil
	}

	attempts := make([]string, 0, 2)

	err = MountTmpfs(scenario.Secondary.Path, defaultTmpfsMountSizeBytes)
	if err == nil {
		scenario.secondaryMounted = true
		if err := scenario.refreshDeviceIDs(); err != nil {
			_ = scenario.Close()
			return nil, err
		}

		if scenario.IsCrossDevice() {
			return scenario, nil
		}

		attempts = append(attempts, "tmpfs mount did not produce a distinct device")
		if err := scenario.Close(); err != nil {
			return nil, err
		}
	}

	if err != nil {
		attempts = append(attempts, fmt.Sprintf("tmpfs mount failed: %v", err))
		if !errors.Is(err, ErrMountPermissionDenied) && !errors.Is(err, ErrMountUnsupported) {
			return nil, err
		}
	}

	err = scenario.tryBindMountFallback()
	if err == nil {
		if err := scenario.refreshDeviceIDs(); err != nil {
			_ = scenario.Close()
			return nil, err
		}

		if scenario.IsCrossDevice() {
			return scenario, nil
		}

		attempts = append(attempts, "bind mount fallback did not produce a distinct device")
		if err := scenario.Close(); err != nil {
			return nil, err
		}
	}

	if err != nil {
		attempts = append(attempts, fmt.Sprintf("bind mount fallback failed: %v", err))
		if !errors.Is(err, ErrMountPermissionDenied) && !errors.Is(err, ErrMountUnsupported) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("%w for base `%s`: %s", ErrCrossDeviceScenarioUnavailable, scenario.BasePath, strings.Join(attempts, "; "))
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
	if s == nil {
		return nil
	}

	var errs []error

	if s.secondaryMounted {
		if err := Unmount(s.Secondary.Path); err != nil {
			errs = append(errs, err)
		} else {
			s.secondaryMounted = false
		}
	}

	if s.bindSourcePath != "" {
		if err := os.RemoveAll(s.bindSourcePath); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove cross-device bind source `%s`: %w", s.bindSourcePath, err))
		} else {
			s.bindSourcePath = ""
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (s *CrossDeviceScenario) refreshDeviceIDs() error {
	primaryDev, err := pathDeviceID(s.Primary.Path)
	if err != nil {
		return err
	}
	secondaryDev, err := pathDeviceID(s.Secondary.Path)
	if err != nil {
		return err
	}

	s.Primary.DeviceID = primaryDev
	s.Secondary.DeviceID = secondaryDev

	return nil
}

func (s *CrossDeviceScenario) tryBindMountFallback() error {
	candidates := []string{"/dev/shm", "/run/shm"}
	for _, candidateBase := range candidates {
		candidateBase = filepath.Clean(candidateBase)
		info, err := os.Stat(candidateBase)
		if err != nil || !info.IsDir() {
			continue
		}

		candidatePath := filepath.Join(candidateBase, fmt.Sprintf("randfiletree-crossdev-%d-%d", os.Getpid(), time.Now().UnixNano()))
		if err := os.MkdirAll(candidatePath, 0o700); err != nil {
			continue
		}

		candidateDev, err := pathDeviceID(candidatePath)
		if err != nil {
			_ = os.RemoveAll(candidatePath)
			continue
		}
		if candidateDev == s.Primary.DeviceID {
			_ = os.RemoveAll(candidatePath)
			continue
		}

		if err := MountBind(candidatePath, s.Secondary.Path); err != nil {
			_ = os.RemoveAll(candidatePath)
			if errors.Is(err, ErrMountPermissionDenied) || errors.Is(err, ErrMountUnsupported) {
				continue
			}
			return err
		}

		s.secondaryMounted = true
		s.bindSourcePath = candidatePath

		return nil
	}

	return fmt.Errorf("%w: no suitable bind source detected", ErrCrossDeviceScenarioUnavailable)
}

func pathDeviceID(path string) (uint64, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to stat `%s` for device ID: %w", path, err)
	}

	return uint64(stat.Dev), nil
}

func ensureNoSymlinkPathComponents(path string) error {
	cleanPath := filepath.Clean(path)
	absPath := cleanPath
	if !filepath.IsAbs(absPath) {
		resolvedPath, err := filepath.Abs(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve base path `%s`: %w", path, err)
		}

		absPath = resolvedPath
	}

	currentPath := string(filepath.Separator)
	components := strings.Split(strings.TrimPrefix(absPath, string(filepath.Separator)), string(filepath.Separator))
	for _, component := range components {
		if component == "" || component == "." {
			continue
		}

		currentPath = filepath.Join(currentPath, component)

		info, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return fmt.Errorf("failed to inspect base path component `%s`: %w", currentPath, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("base path component `%s`: %w", currentPath, ErrBasePathSymlink)
		}

		if !info.IsDir() {
			return fmt.Errorf("base path component `%s` is not a directory", currentPath)
		}
	}

	return nil
}

func ensureScenarioDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("failed to inspect cross-device scenario %s `%s`: %w", label, path, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cross-device scenario %s `%s`: %w", label, path, ErrBasePathSymlink)
	}

	if !info.IsDir() {
		return fmt.Errorf("cross-device scenario %s `%s` is not a directory", label, path)
	}

	return nil
}
