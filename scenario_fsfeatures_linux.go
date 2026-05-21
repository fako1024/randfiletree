//go:build linux

package randfiletree

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	fsImmutableFlag = 0x00000010
	fsAppendFlag    = 0x00000020

	filesystemFeatureProbePayload = "probe"
	reflinkProbePayload           = "reflink-probe-payload"
	immutableFeaturePayload       = "immutable-feature-payload"
	appendOnlyFeaturePayload      = "append-only-feature-payload"
	reflinkFeaturePayload         = "reflink-feature-payload"
)

func probeFilesystemFeatures(basePath string, features []FilesystemFeature) ([]FilesystemFeatureStatus, error) {
	statuses := make([]FilesystemFeatureStatus, 0, len(features))

	for _, feature := range features {
		err := probeFilesystemFeature(basePath, feature)
		if err != nil {
			statuses = append(statuses, statusFromFilesystemFeatureError(feature, err))
			continue
		}

		statuses = append(statuses, FilesystemFeatureStatus{
			Feature:      feature,
			Availability: FilesystemFeatureAvailabilityAvailable,
			Diagnostic:   "feature probe succeeded",
		})
	}

	return statuses, nil
}

func setupFilesystemFeatureScenario(basePath string, features []FilesystemFeature) (_ *FilesystemFeatureScenario, err error) {
	statuses, err := probeFilesystemFeatures(basePath, features)
	if err != nil {
		return nil, err
	}

	if availabilityErr := ensureFilesystemFeatureAvailability(basePath, statuses); availabilityErr != nil {
		return nil, availabilityErr
	}

	scenario := &FilesystemFeatureScenario{
		BasePath:      basePath,
		FeatureStatus: append([]FilesystemFeatureStatus(nil), statuses...),
	}

	defer func() {
		if err == nil {
			return
		}

		_ = scenario.Close()
	}()

	for _, feature := range features {
		switch feature {
		case FilesystemFeatureImmutable:
			if err = setupImmutableFeature(scenario); err != nil {
				return nil, err
			}
		case FilesystemFeatureAppendOnly:
			if err = setupAppendOnlyFeature(scenario); err != nil {
				return nil, err
			}
		case FilesystemFeatureReflink:
			if err = setupReflinkFeature(scenario); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported filesystem feature %d", feature)
		}
	}

	return scenario, nil
}

func restoreFilesystemFeatureFlags(restore filesystemFlagRestore) error {
	if restore.path == "" {
		return nil
	}

	if err := setPathFilesystemFlags(restore.path, restore.flags); err != nil {
		return fmt.Errorf(
			"failed to restore filesystem feature flags for `%s` (%s): %w",
			restore.path,
			restore.feature,
			err,
		)
	}

	return nil
}

func probeFilesystemFeature(basePath string, feature FilesystemFeature) error {
	switch feature {
	case FilesystemFeatureImmutable:
		return probeFilesystemFlagFeature(basePath, feature, fsImmutableFlag)
	case FilesystemFeatureAppendOnly:
		return probeFilesystemFlagFeature(basePath, feature, fsAppendFlag)
	case FilesystemFeatureReflink:
		return probeReflinkFeature(basePath)
	default:
		return fmt.Errorf("unsupported filesystem feature %d", feature)
	}
}

func probeFilesystemFlagFeature(basePath string, feature FilesystemFeature, flag int) error {
	probePath, cleanup, err := createFilesystemFeatureProbeFile(basePath, feature)
	if err != nil {
		return err
	}
	defer cleanup()

	originalFlags, err := getPathFilesystemFlags(probePath)
	if err != nil {
		return err
	}

	updatedFlags := originalFlags | flag
	if err := setPathFilesystemFlags(probePath, updatedFlags); err != nil {
		return err
	}

	activeFlags, err := getPathFilesystemFlags(probePath)
	if err != nil {
		return err
	}

	if activeFlags&flag == 0 {
		return fmt.Errorf(
			"failed to enable %s flag for `%s`: %w",
			feature,
			probePath,
			ErrFilesystemFeatureUnsupported,
		)
	}

	if err := setPathFilesystemFlags(probePath, originalFlags); err != nil {
		return err
	}

	return nil
}

func probeReflinkFeature(basePath string) error {
	sourcePath, err := writeFilesystemFeatureProbePath(basePath, "reflink-source", reflinkProbePayload)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(sourcePath)
	}()

	clonePath, err := writeFilesystemFeatureProbePath(basePath, "reflink-clone", "")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(clonePath)
	}()

	if err := cloneFileReflink(sourcePath, clonePath); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read reflink probe source `%s`: %w", sourcePath, err)
	}

	cloneData, err := os.ReadFile(clonePath)
	if err != nil {
		return fmt.Errorf("failed to read reflink probe clone `%s`: %w", clonePath, err)
	}

	if !bytes.Equal(sourceData, cloneData) {
		return fmt.Errorf("reflink probe content mismatch for `%s` and `%s`", sourcePath, clonePath)
	}

	return nil
}

func setupImmutableFeature(scenario *FilesystemFeatureScenario) error {
	path := filepath.Join(scenario.BasePath, "immutable.txt")
	if err := os.WriteFile(path, []byte(immutableFeaturePayload), 0o640); err != nil {
		return fmt.Errorf("failed to create immutable feature file `%s`: %w", path, err)
	}

	restore, err := enableFilesystemFlagFeature(path, FilesystemFeatureImmutable, fsImmutableFlag)
	if err != nil {
		return err
	}

	scenario.ImmutablePath = path
	scenario.flagRestores = append(scenario.flagRestores, restore)

	return nil
}

func setupAppendOnlyFeature(scenario *FilesystemFeatureScenario) error {
	path := filepath.Join(scenario.BasePath, "append-only.txt")
	if err := os.WriteFile(path, []byte(appendOnlyFeaturePayload), 0o640); err != nil {
		return fmt.Errorf("failed to create append-only feature file `%s`: %w", path, err)
	}

	restore, err := enableFilesystemFlagFeature(path, FilesystemFeatureAppendOnly, fsAppendFlag)
	if err != nil {
		return err
	}

	scenario.AppendOnlyPath = path
	scenario.flagRestores = append(scenario.flagRestores, restore)

	return nil
}

func setupReflinkFeature(scenario *FilesystemFeatureScenario) error {
	sourcePath := filepath.Join(scenario.BasePath, "reflink-source.bin")
	clonePath := filepath.Join(scenario.BasePath, "reflink-clone.bin")

	if err := os.WriteFile(sourcePath, []byte(reflinkFeaturePayload), 0o640); err != nil {
		return fmt.Errorf("failed to create reflink source file `%s`: %w", sourcePath, err)
	}

	if err := os.WriteFile(clonePath, nil, 0o640); err != nil {
		return fmt.Errorf("failed to prepare reflink clone file `%s`: %w", clonePath, err)
	}

	if err := cloneFileReflink(sourcePath, clonePath); err != nil {
		return err
	}

	scenario.ReflinkSourcePath = sourcePath
	scenario.ReflinkClonePath = clonePath

	return nil
}

func createFilesystemFeatureProbeFile(basePath string, feature FilesystemFeature) (string, func(), error) {
	file, err := os.CreateTemp(basePath, fmt.Sprintf(".randfiletree-%s-probe-", feature))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create %s probe file in `%s`: %w", feature, basePath, err)
	}

	path := file.Name()
	if _, err := file.WriteString(filesystemFeatureProbePayload); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("failed to initialize %s probe file `%s`: %w", feature, path, err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("failed to close %s probe file `%s`: %w", feature, path, err)
	}

	cleanup := func() {
		_ = os.Remove(path)
	}

	return path, cleanup, nil
}

func writeFilesystemFeatureProbePath(basePath, namePrefix, payload string) (string, error) {
	file, err := os.CreateTemp(basePath, ".randfiletree-"+namePrefix+"-")
	if err != nil {
		return "", fmt.Errorf("failed to create %s probe file in `%s`: %w", namePrefix, basePath, err)
	}

	path := file.Name()
	if payload != "" {
		if _, err := file.WriteString(payload); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", fmt.Errorf("failed to initialize %s probe file `%s`: %w", namePrefix, path, err)
		}
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to close %s probe file `%s`: %w", namePrefix, path, err)
	}

	return path, nil
}

func enableFilesystemFlagFeature(path string, feature FilesystemFeature, flag int) (filesystemFlagRestore, error) {
	currentFlags, err := getPathFilesystemFlags(path)
	if err != nil {
		return filesystemFlagRestore{}, err
	}

	nextFlags := currentFlags | flag
	if err := setPathFilesystemFlags(path, nextFlags); err != nil {
		return filesystemFlagRestore{}, err
	}

	effectiveFlags, err := getPathFilesystemFlags(path)
	if err != nil {
		return filesystemFlagRestore{}, err
	}

	if effectiveFlags&flag == 0 {
		return filesystemFlagRestore{}, fmt.Errorf(
			"failed to enable %s flag for `%s`: %w",
			feature,
			path,
			ErrFilesystemFeatureUnsupported,
		)
	}

	return filesystemFlagRestore{
		feature: feature,
		path:    path,
		flags:   currentFlags,
	}, nil
}

func getPathFilesystemFlags(path string) (int, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open `%s` for flag inspection: %w", path, mapFilesystemFeatureError(err))
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	flags, err := unix.IoctlGetInt(fd, unix.FS_IOC_GETFLAGS)
	if err != nil {
		return 0, fmt.Errorf("failed to read filesystem flags for `%s`: %w", path, mapFilesystemFeatureError(err))
	}

	return flags, nil
}

func setPathFilesystemFlags(path string, flags int) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("failed to open `%s` for flag update: %w", path, mapFilesystemFeatureError(err))
	}
	defer func() {
		_ = unix.Close(fd)
	}()

	if err := unix.IoctlSetInt(fd, unix.FS_IOC_SETFLAGS, flags); err != nil {
		return fmt.Errorf("failed to update filesystem flags for `%s`: %w", path, mapFilesystemFeatureError(err))
	}

	return nil
}

func cloneFileReflink(sourcePath, clonePath string) error {
	sourceFile, err := os.OpenFile(sourcePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open reflink source `%s`: %w", sourcePath, mapFilesystemFeatureError(err))
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	cloneFile, err := os.OpenFile(clonePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open reflink clone `%s`: %w", clonePath, mapFilesystemFeatureError(err))
	}
	defer func() {
		_ = cloneFile.Close()
	}()

	if err := unix.IoctlFileClone(int(cloneFile.Fd()), int(sourceFile.Fd())); err != nil {
		return fmt.Errorf("failed to create reflink `%s` -> `%s`: %w", sourcePath, clonePath, mapFilesystemFeatureError(err))
	}

	return nil
}

func mapFilesystemFeatureError(err error) error {
	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return fmt.Errorf("%v; %w", err, ErrFilesystemFeaturePermissionDenied)
	case errors.Is(err, unix.ENOTSUP),
		errors.Is(err, unix.EOPNOTSUPP),
		errors.Is(err, unix.ENOTTY),
		errors.Is(err, unix.EINVAL),
		errors.Is(err, unix.ENODEV),
		errors.Is(err, unix.ENOSYS),
		errors.Is(err, unix.EXDEV):
		return fmt.Errorf("%v; %w", err, ErrFilesystemFeatureUnsupported)
	default:
		return err
	}
}
