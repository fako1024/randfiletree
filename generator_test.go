package randfiletree

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

	path := filepath.Join(testBasePath, baseDir)

	g := New(path)
	assert.Nil(t, g.RemoveAll())
	assert.Nil(t, g.Run())
	n1 := 0
	assert.Nil(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n1++
		return nil
	}))

	assert.Nil(t, g.Run())
	n2 := 0
	assert.Nil(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n2++
		return nil
	}))

	assert.Greater(t, n2, n1)
}

func TestMain(m *testing.M) {

	// Ascertain that the base path exists
	if _, err := os.Stat(testBasePath); err != nil {
		fmt.Printf("cannot ascertain existence of base path `%s` for testing, falling back: %s\n", testBasePath, err)
		testBasePath = strings.TrimSuffix(os.TempDir(), "/")
	}
	os.Exit(m.Run())
}
