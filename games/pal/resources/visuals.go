package resources

import (
	"encoding/binary"
	"errors"
)

type IndexedBitmap struct {
	Width  int
	Height int
	Pixels []byte
	Alpha  []byte
}

// DecodeRLE preserves palette indices and separate transparency. Index zero is
// a valid opaque pixel. As in the original clipped blitter, a final run may
// cross the image boundary; no bytes outside the visible image are read.
func DecodeRLE(data []byte) (IndexedBitmap, error) {
	var result IndexedBitmap
	if len(data) < 4 {
		return result, errors.New("truncated RLE dimensions")
	}
	offset := 0
	if binary.LittleEndian.Uint32(data) == 2 {
		offset = 4
	}
	if len(data) < offset+4 {
		return result, errors.New("truncated extended RLE header")
	}
	result.Width, result.Height = int(binary.LittleEndian.Uint16(data[offset:])), int(binary.LittleEndian.Uint16(data[offset+2:]))
	if result.Width > 4096 || result.Height > 4096 || result.Width*result.Height > 4*1024*1024 {
		return IndexedBitmap{}, errors.New("RLE dimensions exceed limit")
	}
	count := result.Width * result.Height
	result.Pixels = make([]byte, count)
	result.Alpha = make([]byte, count)
	offset += 4
	for pixel := 0; pixel < count; {
		if offset >= len(data) {
			return IndexedBitmap{}, errors.New("truncated RLE run")
		}
		run := int(data[offset])
		offset++
		if run&128 != 0 && run <= 128+result.Width {
			pixel += run - 128
			continue
		}
		visible := min(run, count-pixel)
		if visible > len(data)-offset {
			return IndexedBitmap{}, errors.New("truncated RLE pixels")
		}
		copy(result.Pixels[pixel:pixel+visible], data[offset:offset+visible])
		for i := pixel; i < pixel+visible; i++ {
			result.Alpha[i] = 255
		}
		pixel += visible
		offset += run
	}
	return result, nil
}
