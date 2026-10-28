package contra

import "errors"

// GraphicsTextPacket is CB37's zero-$23 string format. Kinds 1 and 2 advance
// addresses by 1 and 32. It is distinct from CBC0's counted rectangle packets.
// A trailing FF terminates the packet only when the next byte is a command
// (0..5); otherwise it represents a literal tile FF and reading continues.
type GraphicsTextPacket struct {
	Kind    uint8
	Address uint16
	Values  []uint8
}

func (p GraphicsTextPacket) EncodedSize() int { return 4 + len(p.Values) }
func (p GraphicsTextPacket) Encode() ([]uint8, error) {
	if p.Kind != 1 && p.Kind != 2 || p.Address > 0x3fff || p.EncodedSize() > 255 {
		return nil, errors.New("invalid text graphics packet")
	}
	for i, v := range p.Values {
		if v == 255 && i+1 < len(p.Values) && p.Values[i+1] < 6 {
			return nil, errors.New("text tile FF before command byte is ambiguous")
		}
	}
	out := make([]uint8, 0, p.EncodedSize())
	out = append(out, p.Kind, uint8(p.Address>>8), uint8(p.Address))
	out = append(out, p.Values...)
	out = append(out, 255)
	return out, nil
}
