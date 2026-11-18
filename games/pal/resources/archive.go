// Package resources reads the original PAL 98 data format without executing
// the original binary or replacing any of its rule tables with invented data.
package resources

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Archive struct {
	data    []byte
	offsets []uint32
}

func OpenArchive(data []byte) (*Archive, error) {
	if len(data) < 4 {
		return nil, errors.New("truncated MKF header")
	}
	header := binary.LittleEndian.Uint32(data)
	if header < 4 || header%4 != 0 || uint64(header) > uint64(len(data)) {
		return nil, errors.New("invalid MKF index size")
	}
	offsets := make([]uint32, int(header/4))
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(data[i*4:])
		if uint64(offsets[i]) > uint64(len(data)) || offsets[i] < header || i > 0 && offsets[i] < offsets[i-1] {
			return nil, fmt.Errorf("invalid MKF offset %d", i)
		}
	}
	if uint64(offsets[len(offsets)-1]) != uint64(len(data)) {
		return nil, errors.New("MKF trailing or missing data")
	}
	return &Archive{data: append([]byte(nil), data...), offsets: offsets}, nil
}

func (a *Archive) Count() int { return len(a.offsets) - 1 }

func (a *Archive) Chunk(index int) ([]byte, error) {
	if index < 0 || index >= a.Count() {
		return nil, errors.New("MKF chunk index outside archive")
	}
	return append([]byte{}, a.data[a.offsets[index]:a.offsets[index+1]]...), nil
}
