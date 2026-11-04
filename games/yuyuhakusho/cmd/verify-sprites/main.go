// Command verify-sprites compares an independent sprite/DMA composer with
// explicitly isolated original-CPU outputs. It never executes a ROM.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type spriteFrame struct {
	GraphicsSet  uint32 `json:"graphics_set"`
	Parameter    uint16 `json:"parameter"`
	TilesBase    uint32 `json:"tiles_base"`
	CharacterID  int    `json:"character_id"`
	Payload      string `json:"payload"`
	SourceSHA256 string `json:"source_sha256"`
}

type spriteSnapshot struct {
	Actor        string `json:"actor"`
	DMA          string `json:"dma"`
	Sprites      string `json:"sprites"`
	Layers       string `json:"layers"`
	DMACursor    uint16 `json:"dma_cursor"`
	Count        uint16 `json:"count"`
	SpriteCursor uint16 `json:"sprite_cursor"`
	Flags        uint8  `json:"flags"`
}

type spriteCall struct {
	GraphicsSet uint32 `json:"graphics_set"`
	Parameter   uint16 `json:"parameter"`
	Variant     int    `json:"variant"`
	Before      spriteSnapshot
	After       spriteSnapshot
	Return      string
}

type spriteFixture struct {
	Schema  int
	ROMSHA  string `json:"rom_sha256"`
	Library []spriteFrame
	Calls   []spriteCall
}

type spriteReport struct {
	Pass   bool   `json:"pass"`
	Frames int    `json:"frames"`
	Calls  int    `json:"original_cpu_calls"`
	Scope  string `json:"scope"`
}

func spriteBytes(s string, size int) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != size {
		return nil, errors.New("invalid original sprite snapshot byte count")
	}
	return b, nil
}

func composeSpriteSnapshot(before spriteSnapshot, frame spriteFrame) (spriteSnapshot, error) {
	payload, err := hex.DecodeString(frame.Payload)
	if err != nil || len(payload)&3 != 0 {
		return before, errors.New("invalid sprite payload")
	}
	var transfers, pieces []uint32
	position := 0
	for i := 0; i < 4; i++ {
		if position+4 > len(payload) {
			return before, errors.New("truncated sprite transfer list")
		}
		v := binary.BigEndian.Uint32(payload[position:])
		position += 4
		transfers = append(transfers, v&0x7fffffff)
		if v&0x80000000 != 0 {
			break
		}
	}
	terminated := false
	for i := 0; i < 80; i++ {
		if position+4 > len(payload) {
			break
		}
		v := binary.BigEndian.Uint32(payload[position:])
		position += 4
		pieces = append(pieces, v)
		if v&0x80000000 != 0 {
			terminated = true
			break
		}
	}
	if !terminated || position != len(payload) {
		return before, errors.New("invalid sprite piece termination")
	}
	actor, err := spriteBytes(before.Actor, 256)
	if err != nil {
		return before, err
	}
	dma, err := spriteBytes(before.DMA, 0xc00)
	if err != nil {
		return before, err
	}
	sprites, err := spriteBytes(before.Sprites, 0x1000)
	if err != nil {
		return before, err
	}
	layers, err := spriteBytes(before.Layers, 256)
	if err != nil {
		return before, err
	}
	word := func(i int) uint16 { return binary.BigEndian.Uint16(actor[i:]) }
	if before.DMACursor&1 != 0 || before.DMACursor < 0xd400 || before.DMACursor > 0xe000 || before.SpriteCursor&1 != 0 || before.SpriteCursor < 0xe005 || before.SpriteCursor >= 0xf000 {
		return before, errors.New("invalid original sprite output cursor")
	}
	if binary.BigEndian.Uint32(actor[0x1c:]) != frame.GraphicsSet || word(0x3c) != frame.Parameter || binary.BigEndian.Uint32(actor[0x20:]) != frame.TilesBase {
		return before, errors.New("sprite frame key differs from actor")
	}
	x, y := word(8)+word(0x28), word(12)+word(0x2a)
	if int8(before.Flags) >= 0 || x-0x40 == 0 || x-0x40 >= 0x1c0 || y-0x40 == 0 || y-0x40 >= 0x160 {
		return before, nil
	}
	after := before
	attributes := word(0x24)
	if actor[0x27] == 0 {
		for i, transfer := range transfers {
			if binary.BigEndian.Uint32(actor[0x2c+i*4:]) == transfer {
				continue
			}
			binary.BigEndian.PutUint32(actor[0x2c+i*4:], transfer)
			source := frame.TilesBase + (transfer>>16)*32
			length := uint16(uint8(transfer)) << 4
			destination := uint16((transfer&0xff00)>>3) + (attributes&0x7ff)*32
			command := uint32(destination&0x3fff)<<16 | uint32(destination>>14) | 0x40000080
			words := [...]uint16{0x9300 | length&255, 0x9400 | length>>8, 0x9500 | uint16(source>>1)&255,
				0x9600 | uint16(source>>9)&255, 0x9700 | uint16(source>>17)&127, 0x8f02, uint16(command >> 16), uint16(command)}
			index := int(after.DMACursor) - 0xd400
			if index+16 > len(dma) {
				return before, errors.New("sprite DMA queue overflow")
			}
			for j, value := range words {
				binary.BigEndian.PutUint16(dma[index+j*2:], value)
			}
			after.DMACursor += 16
		}
		for i := len(transfers); i < 4; i++ {
			binary.BigEndian.PutUint32(actor[0x2c+i*4:], 0)
		}
	}
	layer := int(actor[0x26]&0xfc) + 2
	tail := binary.BigEndian.Uint16(layers[layer:])
	previousLink := int(tail) - 5 - 0xe000
	if previousLink < 0 || previousLink >= len(sprites) {
		return before, errors.New("invalid sprite layer tail")
	}
	sprites[previousLink] = uint8(after.Count)
	for _, piece := range pieces {
		index := int(after.SpriteCursor) - 0xe000
		if index+8 > len(sprites) {
			return before, errors.New("sprite output overflow")
		}
		low, high := uint16(piece), uint16(piece>>16)
		size := uint8(low>>8) & 15
		dx, dy := int16((high>>8)&127)-64, int16(high&127)-64
		if attributes&0x0800 != 0 {
			dx = -dx
		}
		if attributes&0x1000 != 0 {
			dy = -dy
		}
		scale := int32(int16(word(0x3e)))
		dx = int16(int32(dx) * scale >> 8)
		dy = int16(int32(dy) * scale >> 8)
		sx := x + uint16(dx) - uint16((size>>2)+1)*4
		sy := y + uint16(dy) - uint16((size&3)+1)*4
		after.Count = after.Count&0xff00 | uint16(uint8(after.Count+1))
		flags := (low & 0xf000) >> 1
		attrs := attributes + (low & 255)
		if flags&0x6000 != 0 {
			attrs &^= 0x6000
		}
		attrs ^= flags
		values := [...]uint16{sy, low&0xff00 | uint16(uint8(after.Count)), attrs, sx}
		for i, value := range values {
			binary.BigEndian.PutUint16(sprites[index+i*2:], value)
		}
		after.SpriteCursor += 8
		if piece&0x80000000 != 0 {
			break
		}
		if after.Count >= 80 {
			after.Flags = 1
			break
		}
	}
	binary.BigEndian.PutUint16(layers[layer:], after.SpriteCursor)
	after.Actor, after.DMA, after.Sprites, after.Layers = hex.EncodeToString(actor), hex.EncodeToString(dma), hex.EncodeToString(sprites), hex.EncodeToString(layers)
	return after, nil
}

func verifySpriteFixture(fixture spriteFixture) (spriteReport, error) {
	report := spriteReport{Frames: len(fixture.Library), Calls: len(fixture.Calls), Scope: "Original sprite composition and DMA requests only; not full pixel rendering"}
	if fixture.Schema != 1 || fixture.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(fixture.Library) == 0 {
		return report, errors.New("unbound sprite fixture")
	}
	type key struct {
		set       uint32
		parameter uint16
	}
	library := make(map[key]spriteFrame, len(fixture.Library))
	seen := make(map[key]map[int]bool, len(fixture.Library))
	characters := make(map[int]bool)
	for _, frame := range fixture.Library {
		k := key{frame.GraphicsSet, frame.Parameter}
		if _, ok := library[k]; ok || frame.CharacterID < 0 || frame.CharacterID >= 11 {
			return report, errors.New("duplicate sprite key or unavailable character")
		}
		raw, err := hex.DecodeString(frame.Payload)
		digest := sha256.Sum256(raw)
		if err != nil || hex.EncodeToString(digest[:]) != frame.SourceSHA256 {
			return report, errors.New("unbound sprite source bytes")
		}
		library[k], seen[k], characters[frame.CharacterID] = frame, make(map[int]bool), true
	}
	if len(characters) != 11 {
		return report, errors.New("incomplete source-character coverage")
	}
	for i, call := range fixture.Calls {
		k := key{call.GraphicsSet, call.Parameter}
		frame, ok := library[k]
		if !ok || call.Return != "0x3d8" || seen[k][call.Variant] || call.Variant < 0 || call.Variant >= 22 {
			return report, fmt.Errorf("sprite call %d has invalid key, scenario or return", i)
		}
		seen[k][call.Variant] = true
		got, err := composeSpriteSnapshot(call.Before, frame)
		if err != nil || got != call.After {
			return report, fmt.Errorf("sprite call %d key=%x/%x variant=%d differs: %w", i, k.set, k.parameter, call.Variant, err)
		}
	}
	for k := range library {
		expected := 8
		if k.parameter == 2 {
			expected = 22
		}
		if len(seen[k]) != expected {
			return report, errors.New("incomplete per-frame sprite scenarios")
		}
		for variant := 0; variant < expected; variant++ {
			if !seen[k][variant] {
				return report, errors.New("missing sprite scenario")
			}
		}
	}
	report.Pass = true
	return report, nil
}

func main() {
	path := flag.String("oracle", "games/yuyuhakusho/data/sprite-oracle.json.gz", "Original sprite fixture")
	flag.Parse()
	compressed, err := os.ReadFile(*path)
	if err != nil {
		panic(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		panic(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		panic(err)
	}
	var fixture spriteFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		panic(err)
	}
	report, err := verifySpriteFixture(fixture)
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		panic(err)
	}
}
