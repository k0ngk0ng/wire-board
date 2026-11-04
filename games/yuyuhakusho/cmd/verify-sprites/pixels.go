package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

type spriteROMSource struct {
	Owner       uint16
	Slot        int
	Source      uint32
	Destination uint16
	Bytes       string
}

type spritePixelFrame struct {
	Frame          int
	RAM            string
	VRAM           string
	ROMSources     []spriteROMSource `json:"rom_sources"`
	CRAMCompact    []uint16          `json:"cram_compact"`
	VDPRegisters   []uint8           `json:"vdp_registers"`
	PixelFormat    int               `json:"pixel_format"`
	Width, Height  int
	Video          string
	VideoNext      string `json:"video_next"`
	VideoNextFrame int    `json:"video_next_frame"`
}

type spritePixelReport struct {
	Pass               bool    `json:"pass"`
	Frames             int     `json:"frames"`
	ComparedPixels     int     `json:"high_priority_sprite_pixels"`
	CharacterPixels    int     `json:"character_pixels"`
	Mismatches         int     `json:"mismatches"`
	ROMTransfers       int     `json:"rom_transfers"`
	ROMBytes           int     `json:"rom_bytes"`
	SourceCharacterIDs []uint8 `json:"source_character_ids"`
	Scope              string  `json:"scope"`
}

func spriteColor565(compact uint16, limited bool) uint16 {
	if limited {
		compact &= 0x49
	}
	r, g, b := (compact&7)*2, ((compact>>3)&7)*2, ((compact>>6)&7)*2
	return r<<12 | (r>>3)<<11 | g<<7 | (g>>2)<<5 | b<<1 | b>>3
}

// verifyNaturalSpritePixels independently decodes MD 4bpp tiles, column-major
// sprite pieces and normal-mode palette colors. It compares every opaque
// high-priority sprite pixel in these references. Backgrounds, low-priority
// layer merging, scanline overflow and shadow/highlight mode are not claimed.
func verifyNaturalSpritePixels(frames []spritePixelFrame) (spritePixelReport, error) {
	report := spritePixelReport{Frames: len(frames), Scope: "Natural normal-mode high-priority sprites only; VRAM state f versus video f+1"}
	characters := make(map[uint8]bool)
	seenFrames := make(map[int]bool)
	for _, frame := range frames {
		if frame.Frame <= 0 || seenFrames[frame.Frame] || frame.VideoNextFrame != frame.Frame+1 || frame.PixelFormat != 2 || frame.Width != 320 || frame.Height != 224 || len(frame.CRAMCompact) != 64 || len(frame.VDPRegisters) != 32 || frame.VDPRegisters[12]&0x0e != 0 {
			return report, errors.New("unbound pixel timing, dimensions or unsupported VDP mode")
		}
		seenFrames[frame.Frame] = true
		ram, err := spriteBytes(frame.RAM, 65536)
		if err != nil {
			return report, err
		}
		vram, err := spriteBytes(frame.VRAM, 65536)
		if err != nil {
			return report, err
		}
		video, err := spriteBytes(frame.VideoNext, frame.Width*frame.Height*2)
		if err != nil {
			return report, err
		}
		var characterTiles [2048]bool
		for _, source := range frame.ROMSources {
			if source.Slot < 0 || source.Slot >= 4 || int(source.Owner)+256 > len(ram) {
				return report, errors.New("invalid original tile source owner")
			}
			a := ram[int(source.Owner):]
			descriptor := binary.BigEndian.Uint32(a[0x2c+source.Slot*4:])
			length := int(descriptor&255) * 32
			expectedSource := binary.BigEndian.Uint32(a[0x20:]) + (descriptor>>16)*32
			expectedDestination := uint16((descriptor&0xff00)>>3) + (binary.BigEndian.Uint16(a[0x24:])&0x7ff)*32
			payload, err := hex.DecodeString(source.Bytes)
			if err != nil || length == 0 || len(payload) != length || source.Source != expectedSource || source.Destination != expectedDestination {
				return report, errors.New("ROM tile source does not match actor transfer descriptor")
			}
			for i, value := range payload {
				if vram[(int(source.Destination)+i)&65535] != value {
					return report, fmt.Errorf("frame %d original ROM/VRAM tile mismatch", frame.Frame)
				}
			}
			for i := 0; i < length; i += 32 {
				characterTiles[((int(source.Destination)+i)&65535)/32] = true
			}
			characters[a[0x58]] = true
			report.ROMTransfers++
			report.ROMBytes += length
		}
		if len(frame.ROMSources) == 0 {
			return report, errors.New("no independently bound character tiles")
		}
		pixels := frame.Width * frame.Height
		occupied := make([]bool, pixels)
		eligible := make([]bool, pixels)
		character := make([]bool, pixels)
		colors := make([]uint16, pixels)
		satBase := int(frame.VDPRegisters[5]&0x7e) << 9
		var seen [80]bool
		index, ended := 0, false
		for parts := 0; parts < 80; parts++ {
			if index >= len(seen) || seen[index] {
				return report, errors.New("invalid original sprite chain")
			}
			seen[index] = true
			start := (satBase + index*8) & 65535
			if start+8 > len(vram) {
				return report, errors.New("sprite table wraps outside supported reference")
			}
			entry := vram[start:]
			sx := int(binary.BigEndian.Uint16(entry[6:])&511) - 128
			sy := int(binary.BigEndian.Uint16(entry)&1023) - 128
			attributes := binary.BigEndian.Uint16(entry[4:])
			size, link := entry[2]&15, entry[3]&127
			width, height := int(size>>2)+1, int(size&3)+1
			for py := 0; py < height*8; py++ {
				y := sy + py
				if y < 0 || y >= frame.Height {
					continue
				}
				for px := 0; px < width*8; px++ {
					x := sx + px
					if x < 0 || x >= frame.Width || occupied[y*frame.Width+x] {
						continue
					}
					u, v := px, py
					if attributes&0x0800 != 0 {
						u = width*8 - 1 - u
					}
					if attributes&0x1000 != 0 {
						v = height*8 - 1 - v
					}
					tile := (int(attributes&0x7ff) + (u/8)*height + v/8) & 2047
					value := vram[(tile*32+(v%8)*4+(u%8)/2)&65535]
					value >>= 4 * uint(1-u&1)
					value &= 15
					if value == 0 {
						continue
					}
					position := y*frame.Width + x
					palette := int((attributes>>9)&0x30) | int(value)
					if frame.CRAMCompact[palette] > 511 {
						return report, errors.New("invalid compact MD palette value")
					}
					occupied[position], eligible[position] = true, attributes&0x8000 != 0
					character[position] = characterTiles[tile]
					colors[position] = spriteColor565(frame.CRAMCompact[palette], frame.VDPRegisters[0]&4 == 0)
				}
			}
			if link == 0 {
				ended = true
				break
			}
			index = int(link)
		}
		if !ended {
			return report, errors.New("unterminated original sprite chain")
		}
		compared, characterPixels := 0, 0
		for i, compare := range eligible {
			if !compare {
				continue
			}
			compared++
			if character[i] {
				characterPixels++
			}
			if colors[i] != binary.LittleEndian.Uint16(video[i*2:]) {
				report.Mismatches++
			}
		}
		if compared == 0 || characterPixels == 0 {
			return report, errors.New("empty sprite pixel coverage")
		}
		report.ComparedPixels += compared
		report.CharacterPixels += characterPixels
	}
	if report.Mismatches != 0 || len(frames) == 0 {
		return report, errors.New("original high-priority sprite pixel mismatch")
	}
	for id := uint8(0); id < 11; id++ {
		if characters[id] {
			report.SourceCharacterIDs = append(report.SourceCharacterIDs, id)
		}
	}
	report.Pass = true
	return report, nil
}
