package diff

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/minio/highwayhash"
)

var hashKey []byte

func init() {
	var err error
	hashKey, err = hex.DecodeString("000102030405060708090A0B0C0D0E0FF0E0D0C0B0A090807060504030201000")
	if err != nil {
		panic(err)
	}
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
		}

		if node.Path == "" {
			return nil
		}

		if err := collectPlatformMetadata(path, &node); err != nil {
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
