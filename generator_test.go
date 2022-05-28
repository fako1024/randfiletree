package randfiletree

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	baseDir = "randfiletree_test"
)

var (
	testBasePath = "/dev/shm"
)

func TestDefaultOptions(t *testing.T) {

	path := filepath.Join(testBasePath, "randfiletree_test")
	assert.Nil(t, clearTree(path))

	g := New(path)
	assert.Nil(t, g.Run())
	n := 0
	assert.Nil(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n++
		return nil
	}))
	fmt.Println("Number of tree elements after first run:", n)

	assert.Nil(t, g.Run())
	n = 0
	assert.Nil(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n++
		return nil
	}))
	fmt.Println("Number of tree elements after second run:", n)
}

func TestMain(m *testing.M) {

	// Ascertain that the base path exists
	if _, err := os.Stat(testBasePath); err != nil {
		fmt.Printf("cannot ascertain existence of base path `%s` for testing: %s", testBasePath, err)
		os.Exit(-1)
	}

	os.Exit(m.Run())
}

func clearTree(path string) error {
	return os.RemoveAll(path)
}