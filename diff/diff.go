package diff

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/minio/highwayhash"
)

var hashKey []byte

// Node denotes an element / node in a file tree
type Node struct {
	Path       string
	LinkTarget string

	Size    int64
	Mode    fs.FileMode
	ModTime int64

	Hash []byte
}

type collectedPaths struct {
	nodes          []Node
	hardlinkGroups [][]string
}

func init() {
	var err error
	hashKey, err = hex.DecodeString("000102030405060708090A0B0C0D0E0FF0E0D0C0B0A090807060504030201000")
	if err != nil {
		panic(err)
	}
}

// Paths performs a recursive diff of two file trees / paths
// Any deviation will result in an error denoting the respective diff
func Paths(a, b string) error {

	pathsA, err := buildPaths(a)
	if err != nil {
		return err
	}

	pathsB, err := buildPaths(b)
	if err != nil {
		return err
	}

	if diff := cmp.Diff(pathsA.nodes, pathsB.nodes, cmpopts.IgnoreUnexported()); diff != "" {
		return fmt.Errorf("mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(pathsA.hardlinkGroups, pathsB.hardlinkGroups); diff != "" {
		return fmt.Errorf("hardlink topology mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func buildPaths(basePath string) (result collectedPaths, err error) {
	hardlinkPathsByKey := map[fileIdentity][]string{}

	err = filepath.Walk(basePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to accecss path `%s`: %w", path, err)
		}

		node := Node{
			Path:    strings.TrimPrefix(path, basePath),
			Mode:    info.Mode(),
			ModTime: info.ModTime().Unix(),
		}
		if node.Path == "" {
			return nil
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			node.LinkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink target `%s`: %w", path, err)
			}
		}
		if info.Mode().IsRegular() {
			node.Size = info.Size()
			node.Hash, err = hashFile(path, hashKey)
			if err != nil {
				return err
			}

			identity, identityOK := fileIdentityFromPath(path)
			if identityOK {
				hardlinkPathsByKey[identity] = append(hardlinkPathsByKey[identity], node.Path)
			}
		}
		result.nodes = append(result.nodes, node)

		return nil
	})
	if err != nil {
		return
	}

	sort.Slice(result.nodes, func(i, j int) bool {
		return result.nodes[i].Path < result.nodes[j].Path
	})

	result.hardlinkGroups = buildHardlinkGroups(hardlinkPathsByKey)

	return
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
