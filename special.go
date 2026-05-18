package randfiletree

import (
	"fmt"
	"math/rand"
	"sort"
)

// SpecialFileType defines non-regular inode types that can be generated.
type SpecialFileType uint8

const (
	SpecialFileTypeFIFO SpecialFileType = iota + 1
	SpecialFileTypeSocket
	SpecialFileTypeCharDevice
	SpecialFileTypeBlockDevice
)

func (s SpecialFileType) String() string {
	switch s {
	case SpecialFileTypeFIFO:
		return "fifo"
	case SpecialFileTypeSocket:
		return "socket"
	case SpecialFileTypeCharDevice:
		return "char-device"
	case SpecialFileTypeBlockDevice:
		return "block-device"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

func validateSpecialFileType(fileType SpecialFileType) error {
	switch fileType {
	case SpecialFileTypeFIFO,
		SpecialFileTypeSocket,
		SpecialFileTypeCharDevice,
		SpecialFileTypeBlockDevice:
		return nil
	default:
		return fmt.Errorf("invalid special file type %d", fileType)
	}
}

func isSpecialDeviceType(fileType SpecialFileType) bool {
	return fileType == SpecialFileTypeCharDevice || fileType == SpecialFileTypeBlockDevice
}

// SpecialFileTypeGenerator generates a special file type.
type SpecialFileTypeGenerator func(r *rand.Rand) SpecialFileType

type specialFileTypeProbability struct {
	fileType   SpecialFileType
	cumulative float64
}

// SpecialFileTypeGeneratorProbabilityWeighted picks a special file type based on weighted probabilities.
func SpecialFileTypeGeneratorProbabilityWeighted(probabilities map[SpecialFileType]float64) SpecialFileTypeGenerator {
	fileTypes := make([]SpecialFileType, 0, len(probabilities))
	for fileType, probability := range probabilities {
		if probability <= 0 {
			continue
		}

		fileTypes = append(fileTypes, fileType)
	}

	sort.Slice(fileTypes, func(i, j int) bool {
		return fileTypes[i] < fileTypes[j]
	})

	total := 0.0
	for _, fileType := range fileTypes {
		total += probabilities[fileType]
	}

	weighted := make([]specialFileTypeProbability, 0, len(fileTypes))
	cumulative := 0.0
	for _, fileType := range fileTypes {
		cumulative += probabilities[fileType] / total
		weighted = append(weighted, specialFileTypeProbability{
			fileType:   fileType,
			cumulative: cumulative,
		})
	}

	return func(r *rand.Rand) SpecialFileType {
		v := r.Float64()
		for _, fileTypeProbability := range weighted {
			if v <= fileTypeProbability.cumulative {
				return fileTypeProbability.fileType
			}
		}

		return weighted[len(weighted)-1].fileType
	}
}
