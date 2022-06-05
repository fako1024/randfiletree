package randfiletree

import (
	"fmt"
	"math/rand"
)

const ()

var (
	// FileNameAlphabetBasic represents a "safe" alphabet restricted to lowercase/uppercase characters and numbers
	FileNameAlphabetBasic = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890")

	// FileNameAlphabetLinux represents an alphabet compatible with common linux systems
	FileNameAlphabetLinux = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890!@#$%^&*()-_+= ;.,")
)

// StringGenerator denotes a generic generator function for strings (e.g. for filenames)
type StringGenerator func(r *rand.Rand, length int) string

// FileNameGenerator is basically just a string generator
type FileNameGenerator = StringGenerator

// StringGeneratorAlphabet generates a string of requested length based on a provided alphabet
func StringGeneratorAlphabet(alphabet []rune) StringGenerator {
	return func(r *rand.Rand, length int) string {
		b := make([]rune, length)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return string(b)
	}
}

// NumberGenerator denotes a generic generator function for integers (e.g. for length of strings or data)
type NumberGenerator func(r *rand.Rand) int

// FileNameLenGenerator is basically just an integer number generator
type FileNameLenGenerator = NumberGenerator

// NumberGeneratorConstant generates a constant number
func NumberGeneratorConstant(val int) NumberGenerator {
	return func(r *rand.Rand) int {
		return val
	}
}

// NumberGeneratorRandomFlat generates a random number out of a range (equal probabilities)
func NumberGeneratorRandomFlat(min, max int) NumberGenerator {
	return func(r *rand.Rand) int {
		return r.Intn(max-min) + min
	}
}

// FileModeGenerator denotes a generic generator function for file modes (i.e. uint32)
type FileModeGenerator func(r *rand.Rand) uint32

// FileModeGeneratorConstant returns a fixed file mode
func FileModeGeneratorConstant(mode uint32) FileModeGenerator {
	return func(r *rand.Rand) uint32 {
		return mode
	}
}

// DataGenerator denotes a generic data generator
type DataGenerator func(r *rand.Rand) ([]byte, error)

// DataGeneratorFixed returns a fixed set of bytes
func DataGeneratorFixed(data []byte) DataGenerator {
	return func(r *rand.Rand) ([]byte, error) {
		return data, nil
	}
}

// DataGeneratorFixedString returns a fixed set of bytes based on a string
func DataGeneratorFixedString(str string) DataGenerator {
	return DataGeneratorFixed([]byte(str))
}

// DataGeneratorRandomFixedLen returns a random set of bytes of requested length
func DataGeneratorRandomFixedLen(length int) DataGenerator {
	return func(r *rand.Rand) ([]byte, error) {
		data := make([]byte, length)
		nRead, err := r.Read(data)
		if err != nil || nRead != length {
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}
		return data, nil
	}
}

// DataGeneratorRandom returns a random set of bytes of randomized length
func DataGeneratorRandom(lengthGen NumberGenerator) DataGenerator {
	return func(r *rand.Rand) ([]byte, error) {
		data := make([]byte, lengthGen(r))
		nRead, err := r.Read(data)
		if err != nil || nRead != len(data) {
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}
		return data, nil
	}
}

// BooleanGenerator generates a true / false value
type BooleanGenerator func(r *rand.Rand) bool

// BooleanGeneratorProbabilityFlat returns a random boolean with a given
// probablity of being true
func BooleanGeneratorProbabilityFlat(prob float64) BooleanGenerator {
	return func(r *rand.Rand) bool {
		return prob > r.Float64()
	}
}
