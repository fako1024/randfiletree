package randfiletree

import (
	"fmt"
	"math/rand"
)

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	FileNameAlphabetBasic = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890")

	FileNameAlphabetLinux = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890!@#$%^&*()-_+= ;.,")

	ByteNameAlphabetEdgeCase = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890!@#$%^&*()-_+= ;.,")
)

type StringGenerator func(r *rand.Rand, length int) string

type FileNameGenerator = StringGenerator

type ByteNameGenerator func(r *rand.Rand, byteLen int) string

func StringGeneratorAlphabet(alphabet []rune) StringGenerator {
	return func(r *rand.Rand, length int) string {
		b := make([]rune, length)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return string(b)
	}
}

func ByteNameGeneratorAlphabet(alphabet []byte) ByteNameGenerator {
	return func(r *rand.Rand, byteLen int) string {
		b := make([]byte, byteLen)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return string(b)
	}
}

func ByteNamePresetLeadingSpaces(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	nSpaces := 1 + r.Intn(minInt(byteLen-1, 5))
	name := make([]byte, byteLen)
	for i := 0; i < nSpaces; i++ {
		name[i] = ' '
	}
	for i := nSpaces; i < byteLen; i++ {
		name[i] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
	}
	return string(name)
}

func ByteNamePresetTrailingSpaces(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	nSpaces := 1 + r.Intn(minInt(byteLen-1, 5))
	name := make([]byte, byteLen)
	for i := 0; i < byteLen-nSpaces; i++ {
		name[i] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
	}
	for i := byteLen - nSpaces; i < byteLen; i++ {
		name[i] = ' '
	}
	return string(name)
}

func ByteNamePresetLeadingDots(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	nDots := 1 + r.Intn(minInt(byteLen-1, 3))
	name := make([]byte, byteLen)
	for i := 0; i < nDots; i++ {
		name[i] = '.'
	}
	for i := nDots; i < byteLen; i++ {
		name[i] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
	}
	return string(name)
}

func ByteNamePresetNewlineTab(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	name := make([]byte, byteLen)
	nSpecial := 1 + r.Intn(minInt(byteLen-1, 3))
	specialChars := []byte{'\n', '\r', '\t'}
	for i := 0; i < nSpecial; i++ {
		name[i] = specialChars[r.Intn(len(specialChars))]
	}
	for i := nSpecial; i < byteLen; i++ {
		name[i] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
	}
	return string(name)
}

func ByteNamePresetControlChars(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	name := make([]byte, byteLen)
	nControl := 1 + r.Intn(minInt(byteLen-1, 4))
	for i := 0; i < nControl; i++ {
		name[i] = byte(1 + r.Intn(31))
	}
	for i := nControl; i < byteLen; i++ {
		name[i] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
	}
	return string(name)
}

func ByteNamePresetInvalidUTF8(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	name := make([]byte, byteLen)
	nInvalid := 1 + r.Intn(minInt(byteLen-1, 4))
	invalidSequences := [][]byte{
		{0x80}, {0xBF}, {0xC0}, {0xC1},
		{0xF5}, {0xFF},
		{0xC2, 0x00}, {0xE0, 0x00},
	}
	pos := 0
	for i := 0; i < nInvalid && pos < byteLen; i++ {
		seq := invalidSequences[r.Intn(len(invalidSequences))]
		for _, b := range seq {
			if pos >= byteLen {
				break
			}
			if b == 0 {
				b = 0x80
			}
			name[pos] = b
			pos++
		}
	}
	for pos < byteLen {
		name[pos] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
		pos++
	}
	return string(name)
}

var combiningBytes = []byte{0xCC, 0x81, 0xCC, 0x82, 0xCC, 0x83, 0xCC, 0x84}

func ByteNamePresetUnicodeNormalization(r *rand.Rand, byteLen int) string {
	if byteLen < 2 {
		byteLen = 2
	}
	name := make([]byte, byteLen)
	nCombining := 1 + r.Intn(minInt(byteLen-1, 3))
	pos := 0
	name[pos] = 'e'
	pos++
	for i := 0; i < nCombining && pos+1 < byteLen; i++ {
		name[pos] = combiningBytes[i*2]
		pos++
		if pos < byteLen {
			name[pos] = combiningBytes[i*2+1]
			pos++
		}
	}
	for pos < byteLen {
		name[pos] = ByteNameAlphabetEdgeCase[r.Intn(len(ByteNameAlphabetEdgeCase))]
		pos++
	}
	return string(name)
}

func ByteNameGeneratorPreset(preset func(r *rand.Rand, byteLen int) string) ByteNameGenerator {
	return preset
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

// NumberGeneratorRandomFlat generates a random number out of a range (equal probabilities).
//
// Precondition: max > min. The Option helpers (WithDirNameLengthRange,
// WithFileNameLengthRange, WithFilesPerDirectoryRange, WithDirectoriesPerDirectoryRange,
// WithDataLengthRange, WithContentLogicalSizeRange) validate this via
// validateIntRange/validateContentLogicalSizeRange and surface a proper
// configuration error before this constructor is invoked. Direct callers
// that bypass those helpers are responsible for honoring the precondition;
// rand.Intn enforces it on first use.
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
