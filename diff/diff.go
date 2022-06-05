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
	Path string

	Size    int64
	Mode    fs.FileMode
	ModTime int64

	Hash []byte
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

	if diff := cmp.Diff(pathsA, pathsB, cmpopts.IgnoreUnexported()); diff != "" {
		return fmt.Errorf("mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func buildPaths(basePath string) (nodes []Node, err error) {
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
		if info.Mode().IsRegular() {
			node.Size = info.Size()
			node.Hash, err = hashFile(path, hashKey)
			if err != nil {
				return err
			}
		}
		nodes = append(nodes, node)

		return nil
	})
	if err != nil {
		return
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Path < nodes[j].Path
	})

	return
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
