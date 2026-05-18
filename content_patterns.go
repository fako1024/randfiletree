package randfiletree

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
)

const (
	defaultContentWriteChunkSize   = 64 * 1024
	defaultPartialOverwriteCount   = 2
	defaultPatternSegmentMaxLength = 1 << 20
)

// ContentPattern defines how file content is generated and written.
type ContentPattern uint8

const (
	ContentPatternDenseRandom ContentPattern = iota + 1
	ContentPatternSparseHoles
	ContentPatternRepeatedBlocks
	ContentPatternPartialRangeOverwrite
)

func (p ContentPattern) String() string {
	switch p {
	case ContentPatternDenseRandom:
		return "dense-random"
	case ContentPatternSparseHoles:
		return "sparse-holes"
	case ContentPatternRepeatedBlocks:
		return "repeated-blocks"
	case ContentPatternPartialRangeOverwrite:
		return "partial-range-overwrite"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

func validateContentPattern(pattern ContentPattern) error {
	switch pattern {
	case ContentPatternDenseRandom,
		ContentPatternSparseHoles,
		ContentPatternRepeatedBlocks,
		ContentPatternPartialRangeOverwrite:
		return nil
	default:
		return fmt.Errorf("invalid content pattern %d", pattern)
	}
}

// ContentPatternGenerator generates a content pattern.
type ContentPatternGenerator func(r *rand.Rand) ContentPattern

type contentPatternProbability struct {
	pattern    ContentPattern
	cumulative float64
}

// ContentPatternGeneratorProbabilityWeighted picks a content pattern based on weighted probabilities.
func ContentPatternGeneratorProbabilityWeighted(probabilities map[ContentPattern]float64) ContentPatternGenerator {
	patterns := make([]ContentPattern, 0, len(probabilities))
	for pattern, probability := range probabilities {
		if probability <= 0 {
			continue
		}

		patterns = append(patterns, pattern)
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i] < patterns[j]
	})

	total := 0.0
	for _, pattern := range patterns {
		total += probabilities[pattern]
	}

	weighted := make([]contentPatternProbability, 0, len(patterns))
	cumulative := 0.0
	for _, pattern := range patterns {
		cumulative += probabilities[pattern] / total
		weighted = append(weighted, contentPatternProbability{
			pattern:    pattern,
			cumulative: cumulative,
		})
	}

	return func(r *rand.Rand) ContentPattern {
		v := r.Float64()
		for _, patternProbability := range weighted {
			if v <= patternProbability.cumulative {
				return patternProbability.pattern
			}
		}

		return weighted[len(weighted)-1].pattern
	}
}

type plannedFileContent struct {
	pattern ContentPattern

	logicalSize int64

	expectedWrittenBytes int64
	expectedSparse       bool

	seed int64

	repeatedBlock []byte

	sparseExtents    []plannedContentRange
	overwriteExtents []plannedContentRange
}

type plannedContentRange struct {
	offset int64
	length int64
	seed   int64
}

func (g *Generator) planFileContentPattern(r *rand.Rand) (plannedFileContent, error) {
	pattern := g.contentPatternGen(r)
	if err := validateContentPattern(pattern); err != nil {
		return plannedFileContent{}, err
	}

	logicalSize := g.contentLogicalSizeGen(r)
	if logicalSize < 0 {
		return plannedFileContent{}, fmt.Errorf("content logical size must be >= 0, got %d", logicalSize)
	}

	content := plannedFileContent{
		pattern:     pattern,
		logicalSize: int64(logicalSize),
	}

	switch pattern {
	case ContentPatternDenseRandom:
		content.seed = r.Int63()
		content.expectedWrittenBytes = content.logicalSize
		content.expectedSparse = false

	case ContentPatternSparseHoles:
		content.expectedSparse = false
		if content.logicalSize == 0 {
			break
		}

		length := randomizedSegmentLength(r, content.logicalSize)
		if content.logicalSize > 1 && length >= content.logicalSize {
			length = content.logicalSize - 1
		}

		offset, err := randomizedOffset(r, content.logicalSize, length)
		if err != nil {
			return plannedFileContent{}, err
		}

		content.sparseExtents = []plannedContentRange{{
			offset: offset,
			length: length,
			seed:   r.Int63(),
		}}
		content.expectedWrittenBytes = length
		content.expectedSparse = content.logicalSize > content.expectedWrittenBytes

	case ContentPatternRepeatedBlocks:
		block, err := g.resolveRepeatedBlock(r, content.logicalSize)
		if err != nil {
			return plannedFileContent{}, err
		}

		content.repeatedBlock = block
		content.expectedWrittenBytes = content.logicalSize
		content.expectedSparse = false

	case ContentPatternPartialRangeOverwrite:
		content.seed = r.Int63()
		content.expectedWrittenBytes = content.logicalSize
		content.expectedSparse = false

		if content.logicalSize == 0 {
			break
		}

		updates := make([]plannedContentRange, 0, defaultPartialOverwriteCount)
		for i := 0; i < defaultPartialOverwriteCount; i++ {
			length := randomizedSegmentLength(r, content.logicalSize)
			offset, err := randomizedOffset(r, content.logicalSize, length)
			if err != nil {
				return plannedFileContent{}, err
			}

			updates = append(updates, plannedContentRange{
				offset: offset,
				length: length,
				seed:   r.Int63(),
			})
			content.expectedWrittenBytes += length
		}

		sort.Slice(updates, func(i, j int) bool {
			if updates[i].offset == updates[j].offset {
				return updates[i].length < updates[j].length
			}

			return updates[i].offset < updates[j].offset
		})

		content.overwriteExtents = updates

	default:
		return plannedFileContent{}, fmt.Errorf("unsupported content pattern %d", pattern)
	}

	return content, nil
}

func (g *Generator) resolveRepeatedBlock(r *rand.Rand, logicalSize int64) ([]byte, error) {
	var block []byte

	if g.dataGen != nil {
		generated, err := g.dataGen(r)
		if err != nil {
			return nil, fmt.Errorf("failed to generate repeated block data: %w", err)
		}

		block = append([]byte(nil), generated...)
	} else {
		blockLen := defaultContentWriteChunkSize
		if logicalSize > 0 && logicalSize < int64(blockLen) {
			blockLen = int(logicalSize)
		}
		if blockLen == 0 {
			blockLen = 1
		}

		block = make([]byte, blockLen)
		if _, err := r.Read(block); err != nil {
			return nil, fmt.Errorf("failed to generate repeated block data: %w", err)
		}
	}

	if len(block) == 0 {
		return nil, ErrContentPatternRepeatedBlockEmpty
	}

	if len(block) > defaultContentWriteChunkSize {
		block = append([]byte(nil), block[:defaultContentWriteChunkSize]...)
	}

	return block, nil
}

func randomizedSegmentLength(r *rand.Rand, logicalSize int64) int64 {
	if logicalSize <= 0 {
		return 0
	}

	maxLength := logicalSize
	if maxLength > defaultPatternSegmentMaxLength {
		maxLength = defaultPatternSegmentMaxLength
	}

	return int64(r.Intn(int(maxLength))) + 1
}

func randomizedOffset(r *rand.Rand, logicalSize, length int64) (int64, error) {
	if logicalSize < 0 {
		return 0, fmt.Errorf("logical size must be >= 0, got %d", logicalSize)
	}
	if length < 0 {
		return 0, fmt.Errorf("segment length must be >= 0, got %d", length)
	}
	if length > logicalSize {
		return 0, fmt.Errorf("segment length %d exceeds logical size %d", length, logicalSize)
	}

	remaining := logicalSize - length
	if remaining == 0 {
		return 0, nil
	}
	if remaining == 1 {
		return 0, nil
	}

	return r.Int63n(remaining-1) + 1, nil
}

func writePlannedFileContent(path string, mode uint32, content plannedFileContent) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(mode&0o777))
	if err != nil {
		return fmt.Errorf("failed to create planned file `%s`: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	switch content.pattern {
	case ContentPatternDenseRandom:
		if err := writeRandomContentAt(f, 0, content.logicalSize, content.seed); err != nil {
			return err
		}

	case ContentPatternSparseHoles:
		for _, sparseExtent := range content.sparseExtents {
			if err := writeRandomContentAt(f, sparseExtent.offset, sparseExtent.length, sparseExtent.seed); err != nil {
				return err
			}
		}

	case ContentPatternRepeatedBlocks:
		if err := writeRepeatedContent(f, content.logicalSize, content.repeatedBlock); err != nil {
			return err
		}

	case ContentPatternPartialRangeOverwrite:
		if err := writeRandomContentAt(f, 0, content.logicalSize, content.seed); err != nil {
			return err
		}

		for _, overwriteExtent := range content.overwriteExtents {
			if err := writeRandomContentAt(f, overwriteExtent.offset, overwriteExtent.length, overwriteExtent.seed); err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("unsupported content pattern %d", content.pattern)
	}

	if err := f.Truncate(content.logicalSize); err != nil {
		return fmt.Errorf("failed to truncate planned file `%s` to %d bytes: %w", path, content.logicalSize, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to finalize planned file `%s`: %w", path, err)
	}

	return nil
}

func writeRandomContentAt(f *os.File, offset, length, seed int64) error {
	if length == 0 {
		return nil
	}

	/* #nosec G404 */
	r := rand.New(rand.NewSource(seed))
	buf := make([]byte, int(minInt64(length, defaultContentWriteChunkSize)))

	position := int64(0)
	for position < length {
		chunkLen := int(minInt64(length-position, int64(len(buf))))
		if _, err := r.Read(buf[:chunkLen]); err != nil {
			return fmt.Errorf("failed to generate deterministic content chunk: %w", err)
		}

		nWritten, err := f.WriteAt(buf[:chunkLen], offset+position)
		if err != nil {
			return fmt.Errorf("failed to write deterministic content chunk at offset %d: %w", offset+position, err)
		}
		if nWritten != chunkLen {
			return fmt.Errorf("failed to write deterministic content chunk at offset %d: wrote %d bytes, expected %d", offset+position, nWritten, chunkLen)
		}

		position += int64(chunkLen)
	}

	return nil
}

func writeRepeatedContent(f *os.File, logicalSize int64, block []byte) error {
	if logicalSize == 0 {
		return nil
	}

	if len(block) == 0 {
		return ErrContentPatternRepeatedBlockEmpty
	}

	chunk := make([]byte, defaultContentWriteChunkSize)
	for i := range chunk {
		chunk[i] = block[i%len(block)]
	}

	offset := int64(0)
	for offset < logicalSize {
		chunkLen := int(minInt64(logicalSize-offset, int64(len(chunk))))

		nWritten, err := f.WriteAt(chunk[:chunkLen], offset)
		if err != nil {
			return fmt.Errorf("failed to write repeated-block chunk at offset %d: %w", offset, err)
		}
		if nWritten != chunkLen {
			return fmt.Errorf("failed to write repeated-block chunk at offset %d: wrote %d bytes, expected %d", offset, nWritten, chunkLen)
		}

		offset += int64(chunkLen)
	}

	return nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}

	return b
}
