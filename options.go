package randfiletree

import (
	"fmt"
	"math/rand"
)

// Option denotes a configuration option for a Generator.
type Option func(*Generator) error

// NewWithOptions instantiates a new generator and applies the provided options.
func NewWithOptions(basePath string, opts ...Option) (*Generator, error) {
	g := New(basePath)
	if err := g.Configure(opts...); err != nil {
		return nil, err
	}

	return g, nil
}

// Configure applies the provided options atomically.
func (g *Generator) Configure(opts ...Option) error {
	if g == nil {
		return fmt.Errorf("nil generator")
	}

	next := *g
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("option at index %d is nil", i)
		}
		if err := opt(&next); err != nil {
			return fmt.Errorf("failed to apply option at index %d: %w", i, err)
		}
	}

	*g = next

	return nil
}

// Seed sets a new seed (and a new random source, for that matter)
func (g *Generator) Seed(seed int64) *Generator {
	g.rndSrc = rand.New(rand.NewSource(seed)) // #nosec G404
	return g
}

// WithSeed sets a new seed (and a new random source, for that matter).
func WithSeed(seed int64) Option {
	return func(g *Generator) error {
		g.rndSrc = rand.New(rand.NewSource(seed)) // #nosec G404
		return nil
	}
}
