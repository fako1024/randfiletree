//go:build linux

package diff

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPathsWithOptionsDeviceIDMismatch(t *testing.T) {
	scenario := requireDiffCrossDeviceScenario(t)

	leftFile := filepath.Join(scenario.leftPath, "file.txt")
	rightFile := filepath.Join(scenario.rightPath, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(leftFile, ts, ts))
	require.NoError(t, os.Chtimes(rightFile, ts, ts))

	opts := DefaultOptions()
	opts.CompareDeviceIDs = true

	err := PathsWithOptions(scenario.leftPath, scenario.rightPath, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsDeviceIDToggle(t *testing.T) {
	scenario := requireDiffCrossDeviceScenario(t)

	leftFile := filepath.Join(scenario.leftPath, "file.txt")
	rightFile := filepath.Join(scenario.rightPath, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(leftFile, ts, ts))
	require.NoError(t, os.Chtimes(rightFile, ts, ts))

	optsNoDevice := DefaultOptions()
	optsNoDevice.CompareDeviceIDs = false
	require.NoError(t, PathsWithOptions(scenario.leftPath, scenario.rightPath, optsNoDevice))

	optsDevice := DefaultOptions()
	optsDevice.CompareDeviceIDs = true
	err := PathsWithOptions(scenario.leftPath, scenario.rightPath, optsDevice)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

type diffCrossDeviceScenario struct {
	leftPath  string
	rightPath string

	rightMounted bool
	bindSource   string
}

func (s *diffCrossDeviceScenario) close() error {
	if s == nil {
		return nil
	}

	var errs []error
	if s.rightMounted {
		if err := unix.Unmount(s.rightPath, 0); err != nil {
			errs = append(errs, fmt.Errorf("failed to unmount `%s`: %w", s.rightPath, err))
		}
		s.rightMounted = false
	}

	if s.bindSource != "" {
		if err := os.RemoveAll(s.bindSource); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove bind source `%s`: %w", s.bindSource, err))
		}
		s.bindSource = ""
	}

	if len(errs) > 0 {
		errMsg := errs[0].Error()
		for i := 1; i < len(errs); i++ {
			errMsg += "; " + errs[i].Error()
		}

		return errors.New(errMsg)
	}

	return nil
}

func requireDiffCrossDeviceScenario(t *testing.T) *diffCrossDeviceScenario {
	t.Helper()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftDev, err := diffPathDeviceID(left)
	require.NoError(t, err)
	rightDev, err := diffPathDeviceID(right)
	require.NoError(t, err)

	scenario := &diffCrossDeviceScenario{leftPath: left, rightPath: right}

	if leftDev == rightDev {
		mountErr := unix.Mount("tmpfs", right, "tmpfs", 0, "size=16777216")
		if mountErr != nil {
			bindErr := tryDiffBindFallback(leftDev, right, scenario)
			if bindErr != nil {
				t.Skipf("cross-device diff scenario unavailable: tmpfs mount failed (%v); bind fallback failed (%v)", mountErr, bindErr)
			}
		} else {
			scenario.rightMounted = true
		}
	}

	leftDev, err = diffPathDeviceID(left)
	require.NoError(t, err)
	rightDev, err = diffPathDeviceID(right)
	require.NoError(t, err)
	if leftDev == rightDev {
		require.NoError(t, scenario.close())
		t.Skip("cross-device diff scenario did not produce distinct device IDs")
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.close())
	})

	return scenario
}

func tryDiffBindFallback(leftDev uint64, target string, scenario *diffCrossDeviceScenario) error {
	candidates := []string{"/dev/shm", "/run/shm"}
	for _, candidateBase := range candidates {
		info, err := os.Stat(candidateBase)
		if err != nil || !info.IsDir() {
			continue
		}

		candidatePath := filepath.Join(candidateBase, fmt.Sprintf("randfiletree-diff-crossdev-%d", time.Now().UnixNano()))
		if err := os.MkdirAll(candidatePath, 0o700); err != nil {
			continue
		}

		candidateDev, err := diffPathDeviceID(candidatePath)
		if err != nil {
			_ = os.RemoveAll(candidatePath)
			continue
		}
		if candidateDev == leftDev {
			_ = os.RemoveAll(candidatePath)
			continue
		}

		if err := unix.Mount(candidatePath, target, "", uintptr(unix.MS_BIND), ""); err != nil {
			_ = os.RemoveAll(candidatePath)
			continue
		}

		scenario.rightMounted = true
		scenario.bindSource = candidatePath
		return nil
	}

	return fmt.Errorf("no suitable bind source found")
}

func diffPathDeviceID(path string) (uint64, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return 0, fmt.Errorf("failed to stat `%s`: %w", path, err)
	}

	return uint64(stat.Dev), nil
}
