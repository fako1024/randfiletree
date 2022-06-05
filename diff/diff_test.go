package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fako1024/randfiletree"
	"github.com/stretchr/testify/assert"
)

const (
	baseDir = "randfiletree_diff_test"
)

var (
	testBasePath = "/dev/shm"
)

func TestRandomTree(t *testing.T) {

	path := filepath.Join(testBasePath, baseDir)
	assert.Nil(t, clearTree(path))

	g := randfiletree.New(path)
	assert.Nil(t, g.Run())

	if err := Paths(path, path); err != nil {
		t.Fatal(err)
	}

}

func TestMain(m *testing.M) {

	// Ascertain that the base path exists
	if _, err := os.Stat(testBasePath); err != nil {
		fmt.Printf("cannot ascertain existence of base path `%s` for testing, falling back: %s\n", testBasePath, err)
		testBasePath = strings.TrimSuffix(os.TempDir(), "/")
	}
	os.Exit(m.Run())
}

func clearTree(path string) error {
	return os.RemoveAll(path)
}
