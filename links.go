package randfiletree

import (
	"fmt"
	"math/rand"
	"sort"
)

// SymlinkStrategy defines how a symlink target is constructed.
type SymlinkStrategy uint8

const (
	SymlinkStrategyAbsolute SymlinkStrategy = iota + 1
	SymlinkStrategyRelative
	SymlinkStrategyDangling
	SymlinkStrategySelfReferential
	SymlinkStrategyChained
	SymlinkStrategyCycle
)

func (s SymlinkStrategy) String() string {
	switch s {
	case SymlinkStrategyAbsolute:
		return "absolute"
	case SymlinkStrategyRelative:
		return "relative"
	case SymlinkStrategyDangling:
		return "dangling"
	case SymlinkStrategySelfReferential:
		return "self-referential"
	case SymlinkStrategyChained:
		return "chained"
	case SymlinkStrategyCycle:
		return "cycle"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

func validateSymlinkStrategy(strategy SymlinkStrategy) error {
	switch strategy {
	case SymlinkStrategyAbsolute,
		SymlinkStrategyRelative,
		SymlinkStrategyDangling,
		SymlinkStrategySelfReferential,
		SymlinkStrategyChained,
		SymlinkStrategyCycle:
		return nil
	default:
		return fmt.Errorf("invalid symlink strategy %d", strategy)
	}
}

// SymlinkStrategyGenerator generates a symlink strategy.
type SymlinkStrategyGenerator func(r *rand.Rand) SymlinkStrategy

type symlinkStrategyProbability struct {
	strategy   SymlinkStrategy
	cumulative float64
}

// SymlinkStrategyGeneratorProbabilityWeighted picks a symlink strategy based on weighted probabilities.
func SymlinkStrategyGeneratorProbabilityWeighted(probabilities map[SymlinkStrategy]float64) SymlinkStrategyGenerator {
	strategies := make([]SymlinkStrategy, 0, len(probabilities))
	for strategy, probability := range probabilities {
		if probability <= 0 {
			continue
		}

		strategies = append(strategies, strategy)
	}

	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i] < strategies[j]
	})

	total := 0.0
	for _, strategy := range strategies {
		total += probabilities[strategy]
	}

	weighted := make([]symlinkStrategyProbability, 0, len(strategies))
	cumulative := 0.0
	for _, strategy := range strategies {
		cumulative += probabilities[strategy] / total
		weighted = append(weighted, symlinkStrategyProbability{
			strategy:   strategy,
			cumulative: cumulative,
		})
	}

	return func(r *rand.Rand) SymlinkStrategy {
		v := r.Float64()
		for _, strategyProbability := range weighted {
			if v <= strategyProbability.cumulative {
				return strategyProbability.strategy
			}
		}

		return weighted[len(weighted)-1].strategy
	}
}
