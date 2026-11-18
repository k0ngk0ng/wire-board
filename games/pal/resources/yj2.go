// SPDX-License-Identifier: GPL-3.0-only
// YJ2 format/algorithm reference: SDLPAL yj1.c.
// Copyright (c) 2009-2011 Wei Mingzhi; (c) 2011-2026 SDLPAL development team.
// Portions based on PalLibrary, copyright (c) 2006-2007 Lou Yihua.
// This Go implementation adds bounded input/output and back-reference checks.

package resources

import (
	"encoding/binary"
	"errors"
	"io"
)

var yj2DistanceHigh = [256]byte{
	0x3f, 0x0b, 0x17, 0x03, 0x2f, 0x0a, 0x16, 0x00, 0x2e, 0x09, 0x15, 0x02, 0x2d, 0x01, 0x08, 0x00,
	0x3e, 0x07, 0x14, 0x03, 0x2c, 0x06, 0x13, 0x00, 0x2b, 0x05, 0x12, 0x02, 0x2a, 0x01, 0x04, 0x00,
	0x3d, 0x0b, 0x11, 0x03, 0x29, 0x0a, 0x10, 0x00, 0x28, 0x09, 0x0f, 0x02, 0x27, 0x01, 0x08, 0x00,
	0x3c, 0x07, 0x0e, 0x03, 0x26, 0x06, 0x0d, 0x00, 0x25, 0x05, 0x0c, 0x02, 0x24, 0x01, 0x04, 0x00,
	0x3b, 0x0b, 0x17, 0x03, 0x23, 0x0a, 0x16, 0x00, 0x22, 0x09, 0x15, 0x02, 0x21, 0x01, 0x08, 0x00,
	0x3a, 0x07, 0x14, 0x03, 0x20, 0x06, 0x13, 0x00, 0x1f, 0x05, 0x12, 0x02, 0x1e, 0x01, 0x04, 0x00,
	0x39, 0x0b, 0x11, 0x03, 0x1d, 0x0a, 0x10, 0x00, 0x1c, 0x09, 0x0f, 0x02, 0x1b, 0x01, 0x08, 0x00,
	0x38, 0x07, 0x0e, 0x03, 0x1a, 0x06, 0x0d, 0x00, 0x19, 0x05, 0x0c, 0x02, 0x18, 0x01, 0x04, 0x00,
	0x37, 0x0b, 0x17, 0x03, 0x2f, 0x0a, 0x16, 0x00, 0x2e, 0x09, 0x15, 0x02, 0x2d, 0x01, 0x08, 0x00,
	0x36, 0x07, 0x14, 0x03, 0x2c, 0x06, 0x13, 0x00, 0x2b, 0x05, 0x12, 0x02, 0x2a, 0x01, 0x04, 0x00,
	0x35, 0x0b, 0x11, 0x03, 0x29, 0x0a, 0x10, 0x00, 0x28, 0x09, 0x0f, 0x02, 0x27, 0x01, 0x08, 0x00,
	0x34, 0x07, 0x0e, 0x03, 0x26, 0x06, 0x0d, 0x00, 0x25, 0x05, 0x0c, 0x02, 0x24, 0x01, 0x04, 0x00,
	0x33, 0x0b, 0x17, 0x03, 0x23, 0x0a, 0x16, 0x00, 0x22, 0x09, 0x15, 0x02, 0x21, 0x01, 0x08, 0x00,
	0x32, 0x07, 0x14, 0x03, 0x20, 0x06, 0x13, 0x00, 0x1f, 0x05, 0x12, 0x02, 0x1e, 0x01, 0x04, 0x00,
	0x31, 0x0b, 0x11, 0x03, 0x1d, 0x0a, 0x10, 0x00, 0x1c, 0x09, 0x0f, 0x02, 0x1b, 0x01, 0x08, 0x00,
	0x30, 0x07, 0x0e, 0x03, 0x1a, 0x06, 0x0d, 0x00, 0x19, 0x05, 0x0c, 0x02, 0x18, 0x01, 0x04, 0x00,
}
var yj2DistanceBits = [16]int{8, 5, 6, 4, 7, 5, 6, 3, 7, 5, 6, 4, 7, 4, 5, 3}

type yj2Node struct{ weight, value, parent, left, right int }
type yj2Tree struct {
	nodes  [641]yj2Node
	leaves [321]int
}

func newYJ2Tree() *yj2Tree {
	t := &yj2Tree{}
	for i := range t.nodes {
		t.nodes[i].value, t.nodes[i].weight = i, 1
	}
	for i := range t.leaves {
		t.leaves[i] = i
	}
	t.nodes[640].parent = 640
	for child, parent := 0, 321; parent <= 640; child, parent = child+2, parent+1 {
		t.nodes[parent].left, t.nodes[parent].right = child, child+1
		t.nodes[child].parent, t.nodes[child+1].parent = parent, parent
		t.nodes[parent].weight = t.nodes[child].weight + t.nodes[child+1].weight
	}
	return t
}

func (t *yj2Tree) adjust(value int) {
	i := t.leaves[value]
	for t.nodes[i].value != 640 {
		j := i + 1
		for j < len(t.nodes) && t.nodes[j].weight == t.nodes[i].weight {
			j++
		}
		j--
		if j != i {
			t.nodes[i].parent, t.nodes[j].parent = t.nodes[j].parent, t.nodes[i].parent
			if t.nodes[i].value > 320 {
				t.nodes[t.nodes[i].left].parent, t.nodes[t.nodes[i].right].parent = j, j
			} else {
				t.leaves[t.nodes[i].value] = j
			}
			if t.nodes[j].value > 320 {
				t.nodes[t.nodes[j].left].parent, t.nodes[t.nodes[j].right].parent = i, i
			} else {
				t.leaves[t.nodes[j].value] = i
			}
			t.nodes[i], t.nodes[j] = t.nodes[j], t.nodes[i]
			i = j
		}
		t.nodes[i].weight++
		i = t.nodes[i].parent
	}
	t.nodes[i].weight++
}

type yj2Bits struct {
	data     []byte
	position int
}

func (b *yj2Bits) bit() (int, error) {
	if b.position/8 >= len(b.data) {
		return 0, io.ErrUnexpectedEOF
	}
	value := int(b.data[b.position/8] >> (b.position % 8) & 1)
	b.position++
	return value, nil
}

// DecompressYJ2 decodes a Windows PAL resource. The caller supplies a bound for
// decoded output; corrupt lengths, back-references and truncated streams fail
// before they can access memory outside the resource or produce partial data.
func DecompressYJ2(source []byte, limit int) ([]byte, error) {
	if len(source) < 4 {
		return nil, io.ErrUnexpectedEOF
	}
	length := uint64(binary.LittleEndian.Uint32(source))
	if limit <= 0 || length == 0 || length > uint64(limit) {
		return nil, errors.New("YJ2 output exceeds resource limit")
	}
	out := make([]byte, 0, int(length))
	tree := newYJ2Tree()
	bits := yj2Bits{data: source[4:]}
	for {
		i := 640
		for tree.nodes[i].value > 320 {
			bit, err := bits.bit()
			if err != nil {
				return nil, err
			}
			if bit == 0 {
				i = tree.nodes[i].left
			} else {
				i = tree.nodes[i].right
			}
		}
		value := tree.nodes[i].value
		if tree.nodes[640].weight == 0x8000 {
			for j := range tree.leaves {
				if tree.nodes[tree.leaves[j]].weight&1 != 0 {
					tree.adjust(j)
				}
			}
			for j := range tree.nodes {
				tree.nodes[j].weight >>= 1
			}
		}
		tree.adjust(value)
		if value <= 255 {
			if len(out) >= int(length) {
				return nil, errors.New("YJ2 literal exceeds declared output")
			}
			out = append(out, byte(value))
			continue
		}
		code := 0
		for j := 0; j < 8; j++ {
			bit, err := bits.bit()
			if err != nil {
				return nil, err
			}
			code |= bit << j
		}
		prefix := code & 255
		shift := yj2DistanceBits[prefix&15]
		for j := 8; j < shift+6; j++ {
			bit, err := bits.bit()
			if err != nil {
				return nil, err
			}
			code |= bit << j
		}
		distance := (code>>shift)&63 | int(yj2DistanceHigh[prefix])<<6
		if distance == 4095 {
			if len(out) != int(length) {
				return nil, errors.New("YJ2 output does not match declared length")
			}
			return out, nil
		}
		start, count := len(out)-distance-1, value-253
		if start < 0 || count > int(length)-len(out) {
			return nil, errors.New("invalid YJ2 back reference")
		}
		for j := 0; j < count; j++ {
			out = append(out, out[start+j])
		}
	}
}
