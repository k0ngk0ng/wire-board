package contra

import "errors"

// CB30 chooses one parser per submission through original $23. The Go zero
// value is rectangular (the ordinary battle mode); GraphicsText corresponds to
// original $23==0. The enum is semantic state, not a copy of that source byte.
type GraphicsQueueMode uint8

const (
	GraphicsRectangular GraphicsQueueMode = iota
	GraphicsText
)

func (q *GraphicsQueue) validateMode() error {
	if q == nil {
		return errors.New("missing graphics queue")
	}
	switch q.Mode {
	case GraphicsRectangular:
		if len(q.TextPackets) != 0 {
			return errors.New("mixed graphics parsers")
		}
	case GraphicsText:
		if len(q.Packets) != 0 {
			return errors.New("mixed graphics parsers")
		}
		if q.HeadClear != [5]uint8{} {
			return errors.New("text queue cannot have Guardian header effects")
		}
	default:
		return errors.New("unknown graphics parser")
	}
	return nil
}

func (q *GraphicsQueue) AppendText(packet GraphicsTextPacket) error {
	if err := q.validateMode(); err != nil {
		return err
	}
	if q.Mode != GraphicsText {
		return errors.New("text packet requires text graphics mode")
	}
	raw, err := packet.Encode()
	if err != nil {
		return err
	}
	if int(q.Used)+len(raw) > GraphicsBufferCapacity {
		return errors.New("text packet exceeds the frame buffer")
	}
	packet.Values = append([]uint8(nil), packet.Values...)
	q.TextPackets = append(q.TextPackets, packet)
	q.Used += uint8(len(raw))
	return nil
}

// Clone preserves the parser and byte budget, with owned mutable payloads.
func (q GraphicsQueue) Clone() GraphicsQueue {
	next := q
	if q.Packets != nil {
		next.Packets = make([]GraphicsPacket, len(q.Packets))
		for i, p := range q.Packets {
			next.Packets[i] = p
			next.Packets[i].Segments = make([]GraphicsSegment, len(p.Segments))
			for j, s := range p.Segments {
				next.Packets[i].Segments[j] = GraphicsSegment{Address: s.Address, Values: append([]uint8(nil), s.Values...)}
			}
		}
	}
	if q.TextPackets != nil {
		next.TextPackets = make([]GraphicsTextPacket, len(q.TextPackets))
		for i, p := range q.TextPackets {
			next.TextPackets[i] = p
			next.TextPackets[i].Values = append([]uint8(nil), p.Values...)
		}
	}
	return next
}

// Encode retains the native parser's exact cost, without the frame terminator.
func (q GraphicsQueue) Encode() ([]uint8, error) {
	if err := q.validateMode(); err != nil {
		return nil, err
	}
	var raw []uint8
	for _, p := range q.Packets {
		b, e := p.Encode()
		if e != nil {
			return nil, e
		}
		raw = append(raw, b...)
	}
	for _, p := range q.TextPackets {
		b, e := p.Encode()
		if e != nil {
			return nil, e
		}
		raw = append(raw, b...)
	}
	if len(raw) != int(q.Used) || len(raw) > GraphicsBufferCapacity {
		return nil, errors.New("inconsistent graphics byte budget")
	}
	for i, bits := range q.HeadClear {
		if bits == 0 {
			continue
		}
		if i >= len(raw) {
			return nil, errors.New("graphics head effect has no owned byte")
		}
		raw[i] &= ^bits
	}
	return raw, nil
}
