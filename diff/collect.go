package diff

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/minio/highwayhash"
)

var hashKey = []byte{
	0x00, 0x01, 0x02, 0x03,
	0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b,
	0x0c, 0x0d, 0x0e, 0x0f,
	0xf0, 0xe0, 0xd0, 0xc0,
	0xb0, 0xa0, 0x90, 0x80,
	0x70, 0x60, 0x50, 0x40,
	0x30, 0x20, 0x10, 0x00,
}

func collectPaths(basePath string, opts Options) (result collectedPaths, err error) {
	hardlinkPathsByKey := map[fileIdentity][]string{}

	err = filepath.Walk(basePath, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to access path `%s`: %w", path, walkErr)
		}

		nodePath, err := canonicalNodePath(basePath, path)
		if err != nil {
			return err
		}

		node := Node{
			Path:        nodePath,
			Mode:        info.Mode(),
			ModTime:     info.ModTime().Unix(),
			ModTimeNsec: info.ModTime().UnixNano(),
			InodeType:   inodeTypeFromFileMode(info.Mode()),
		}

		if node.Path == "" {
			return nil
		}

		if err := collectPlatformMetadata(path, &node, opts); err != nil {
			return err
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			node.LinkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink target `%s`: %w", path, err)
			}
		}

		if info.Mode().IsRegular() {
			node.Size = info.Size()
			if opts.CompareContentHash {
				node.Hash, err = hashFile(path, hashKey)
				if err != nil {
					return err
				}
			}

			if opts.CompareHardlinkTopology {
				identity, identityOK := fileIdentityFromPath(path)
				if identityOK {
					hardlinkPathsByKey[identity] = append(hardlinkPathsByKey[identity], node.Path)
				}
			}
		}

		result.nodes = append(result.nodes, node)

		return nil
	})
	if err != nil {
		return result, err
	}

	sort.Slice(result.nodes, func(i, j int) bool {
		return result.nodes[i].Path < result.nodes[j].Path
	})

	if opts.CompareHardlinkTopology {
		result.hardlinkGroups = buildHardlinkGroups(hardlinkPathsByKey)
	}

	return result, nil
}

func canonicalNodePath(basePath, path string) (string, error) {
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return "", fmt.Errorf("failed to determine relative path for `%s`: %w", path, err)
	}

	if relPath == "." {
		return "", nil
	}

	return "/" + filepath.ToSlash(relPath), nil
}

func buildHardlinkGroups(pathsByIdentity map[fileIdentity][]string) [][]string {
	groups := make([][]string, 0, len(pathsByIdentity))
	for _, paths := range pathsByIdentity {
		if len(paths) < 2 {
			continue
		}

		group := append([]string(nil), paths...)
		sort.Strings(group)
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i]) == 0 || len(groups[j]) == 0 {
			return len(groups[i]) < len(groups[j])
		}

		return groups[i][0] < groups[j][0]
	})

	return groups
}

func hashFile(file string, hashKey []byte) ([]byte, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	hash, err := highwayhash.New(hashKey)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(hash, f)
	return hash.Sum(nil), err
}

func inodeTypeFromFileMode(mode fs.FileMode) InodeType {
	switch {
	case mode.IsRegular():
		return InodeTypeRegular
	case mode.IsDir():
		return InodeTypeDirectory
	case mode&fs.ModeSymlink != 0:
		return InodeTypeSymlink
	case mode&fs.ModeNamedPipe != 0:
		return InodeTypeFIFO
	case mode&fs.ModeSocket != 0:
		return InodeTypeSocket
	case mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice != 0:
		return InodeTypeCharDevice
	case mode&fs.ModeDevice != 0:
		return InodeTypeBlockDevice
	default:
		return InodeTypeOther
	}
}
