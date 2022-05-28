package diff

import (
	"os"
	"path/filepath"
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

func clearTree(path string) error {
	return os.RemoveAll(path)
}
