[1mdiff --git a/generator.go b/generator.go[m
[1mindex fcd707a..786b450 100644[m
[1m--- a/generator.go[m
[1m+++ b/generator.go[m
[36m@@ -62,6 +62,11 @@[m [mfunc (g *Generator) Run() error {[m
 	return g.writeDir(g.basePath, 0)[m
 }[m
 [m
[32m+[m[32m// RemoveAll removes (and recreates) the directory[m
[32m+[m[32mfunc (g *Generator) RemoveAll() error {[m
[32m+[m	[32mreturn os.RemoveAll(g.basePath)[m
[32m+[m[32m}[m
[32m+[m
 // Walk performs a recursive walk through the provided directory (wrapping filepath.Walk())[m
 func (g *Generator) Walk(fn filepath.WalkFunc) error {[m
 	return filepath.Walk(g.basePath, fn)[m
[1mdiff --git a/generator_test.go b/generator_test.go[m
[1mindex 4829af3..8ed9096 100644[m
[1m--- a/generator_test.go[m
[1m+++ b/generator_test.go[m
[36m@@ -22,9 +22,9 @@[m [mvar ([m
 func TestDefaultOptions(t *testing.T) {[m
 [m
 	path := filepath.Join(testBasePath, baseDir)[m
[31m-	assert.Nil(t, clearTree(path))[m
 [m
 	g := New(path)[m
[32m+[m	[32massert.Nil(t, g.RemoveAll())[m
 	assert.Nil(t, g.Run())[m
 	n := 0[m
 	assert.Nil(t, g.Walk(func(path string, info fs.FileInfo, err error) error {[m
[36m@@ -51,7 +51,3 @@[m [mfunc TestMain(m *testing.M) {[m
 	}[m
 	os.Exit(m.Run())[m
 }[m
[31m-[m
[31m-func clearTree(path string) error {[m
[31m-	return os.RemoveAll(path)[m
[31m-}[m
[1mdiff --git a/options.go b/options.go[m
[1mindex 443ba0f..7e16797 100644[m
[1m--- a/options.go[m
[1m+++ b/options.go[m
[36m@@ -4,8 +4,6 @@[m [mimport "math/rand"[m
 [m
 // Seed sets a new seed (and a new random source, for that matter)[m
 func (g *Generator) Seed(seed int64) *Generator {[m
[31m-[m
[31m-	/* #nosec G404 */[m
[31m-	g.rndSrc = rand.New(rand.NewSource(seed))[m
[32m+[m	[32mg.rndSrc = rand.New(rand.NewSource(seed)) // #nosec G404[m
 	return g[m
 }[m
