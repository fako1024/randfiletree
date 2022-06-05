package randfiletree

import "math/rand"

// Seed sets a new seed (and a new random source, for that matter)
func (g *Generator) Seed(seed int64) *Generator {

	/* #nosec G404 */
	g.rndSrc = rand.New(rand.NewSource(seed))
	return g
}
