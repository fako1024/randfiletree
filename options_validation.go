package randfiletree

import (
	"fmt"
	"math"
)

func validateProbability(name string, probability float64) error {
	if math.IsNaN(probability) {
		return fmt.Errorf("%s must not be NaN", name)
	}
	if math.IsInf(probability, 0) {
		return fmt.Errorf("%s must be finite", name)
	}
	if probability < 0 || probability > 1 {
		return fmt.Errorf("%s must be within [0, 1], got %v", name, probability)
	}

	return nil
}

func validateIntRange(name string, min, max int) error {
	if min < 0 {
		return fmt.Errorf("%s minimum must be >= 0, got %d", name, min)
	}
	if max <= min {
		return fmt.Errorf("%s maximum must be > minimum, got min=%d max=%d", name, min, max)
	}

	return nil
}

func validateFileNameGenerator(name string, gen FileNameGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateNumberGenerator(name string, gen NumberGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateFileModeGenerator(name string, gen FileModeGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateDataGenerator(name string, gen DataGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateBooleanGenerator(name string, gen BooleanGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}
