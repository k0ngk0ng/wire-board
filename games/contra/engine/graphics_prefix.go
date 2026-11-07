package contra

import "errors"

// ClearPrefix represents Guardian's bounded collision-overrun AND operation.
// Only the five adjacent graphics-header bytes can be affected. Used and packet
// allocation stay unchanged: a cleared first Kind stops drawing, not admission.
func (q *GraphicsQueue) ClearPrefix(offset uint8, andMask uint8) error {
	if q == nil || q.Mode != GraphicsRectangular || offset >= 5 {
		return errors.New("invalid Guardian graphics-prefix effect")
	}
	raw, err := q.Encode()
	if err != nil {
		return err
	}
	if int(offset) >= len(raw) {
		return errors.New("graphics-prefix effect outside owned command bytes")
	}
	q.HeadClear[offset] |= ^andMask
	return nil
}

// DecodedWrites interprets the existing graphics data after a finite header
// effect. It follows CBC0's data format, not CPU instructions. Zero lengths or
// counts mean 256 in the original loops; if that would read outside this owned
// queue, report an unsupported stream instead of inventing neighboring memory.
func (q *GraphicsQueue) DecodedWrites() ([]BackgroundWrite, error) {
	if err := q.validateMode(); err != nil {
		return nil, err
	}
	if q.HeadClear == [5]uint8{} {
		return q.plainWrites(), nil
	}
	raw, err := q.Encode()
	if err != nil {
		return nil, err
	}
	var writes []BackgroundWrite
	for at := 0; at < len(raw); {
		kind := raw[at]
		if kind == 0 {
			break
		}
		if kind != 1 && kind != 2 || at+3 > len(raw) {
			return nil, errors.New("invalid modified graphics command")
		}
		length, count := int(raw[at+1]), int(raw[at+2])
		if length == 0 {
			length = 256
		}
		if count == 0 {
			count = 256
		}
		at += 3
		if count*(length+2) > len(raw)-at {
			return nil, errors.New("modified graphics reads outside owned queue")
		}
		step := uint16(1)
		if kind == 2 {
			step = 32
		}
		for segment := 0; segment < count; segment++ {
			address := (uint16(raw[at])<<8 | uint16(raw[at+1])) & 0x3fff
			at += 2
			values := append([]uint8(nil), raw[at:at+length]...)
			at += length
			end := uint32(address) + uint32(length-1)*uint32(step)
			if address >= 0x2000 && end <= 0x3fff {
				writes = append(writes, BackgroundWrite{Address: address, Step: step, Values: values})
			} else {
				// This cartridge has immutable CHR ROM: redirected pattern
				// writes do not modify pixels or any gameplay state.
				for i, value := range values {
					where := (uint32(address) + uint32(i)*uint32(step)) & 0x3fff
					if where >= 0x2000 {
						writes = append(writes, BackgroundWrite{Address: uint16(where), Step: 1, Values: []uint8{value}})
					}
				}
			}
		}
	}
	return writes, nil
}
