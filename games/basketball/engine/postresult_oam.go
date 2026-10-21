package engine

import "fmt"

type OriginalPostResultSpriteContext struct {
	SurfaceCodes [13]uint8 `json:"surfaceCodes"`
}
type OriginalPostResultOAM struct {
	OAM    [256]uint8 `json:"oam"`
	Pieces uint8      `json:"pieces"`
}
type postResultSpritePiece struct {
	Tile, Attributes uint8
	X, Y             int8
}

// EmitOriginalPostResultOAM restores complete CE09 for source result/reward/
// ending actor kinds17..19. It preserves prior unused-slot bytes and culled
// fragments' partial writes. The eight-sprites-per-line limit belongs to the
// rasterizer; CE09 itself stops only when all64 hardware slots have been used.
// It does not clear the caller's draw list or mutate actors/projection state.
func EmitOriginalPostResultOAM(previous [256]uint8, actors OpeningState, context OriginalPostResultSpriteContext) (OriginalPostResultOAM, error) {
	initial := OriginalPostResultOAM{OAM: previous}
	result := initial
	t := actors.Transition
	if t.Scene != 1 && t.Scene != 7 {
		return initial, fmt.Errorf("unrestored post-result sprite scene%d", t.Scene)
	}
	if t.Scene == 1 && t.Match.Phase != 6 && t.Match.Phase != 7 {
		return initial, fmt.Errorf("post-result sprites require result/reward phase")
	}
	for _, owner := range actors.Projection.DrawOrder[:12] {
		if owner&128 != 0 {
			continue
		}
		if owner >= 4 {
			return initial, fmt.Errorf("unrestored post-result sprite owner%d", owner)
		}
		object := t.Objects[owner]
		kind := t.Kinds[owner]
		packed := displayPose(object.Pose)
		if kind < 17 || kind > 19 || int(object.Pose.Frame) >= len(originalPostResultSpriteProfiles[kind-17]) {
			return initial, fmt.Errorf("unrestored post-result kind%d pose%d", kind, object.Pose.Frame)
		}
		tileOffset, paletteOffset := uint8(0), uint8(0)
		if t.Scene == 1 {
			if owner >= 2 {
				tileOffset = 64
				paletteOffset = 2
			}
		} else if kind == 18 && packed&127 == 2 || kind == 17 && (packed == 31 || packed&126 == 22 || (packed&127 >= 111 && packed&127 < 114)) {
			paletteOffset = 32
		}
		projected := actors.Projection.Projected[owner]
		emit := func(piece postResultSpritePiece, shadow bool) (bool, error) {
			slot := int(result.Pieces) * 4
			y := int32(projected.Y) + int32(piece.Y)
			if shadow {
				y += int32(int16(uint16(object.Body.Position.Height >> 8)))
			}
			sy := uint16(y)
			result.OAM[slot] = uint8(sy)
			if sy>>8 != 0 || ((t.Scene == 1 || t.Scene == 7 && t.Match.Phase != 11) && sy >= 185) {
				return false, nil
			}
			tile := piece.Tile
			if t.Scene == 1 && tile < 4 {
				index := int(t.Characters[owner])
				if owner >= 2 {
					index += 5
				}
				if index >= len(originalPostResultHeadOffsets[tile]) {
					return false, fmt.Errorf("unrestored post-result head index%d", index)
				}
				tile += originalPostResultHeadOffsets[tile][index]
				if t.Match.Phase == 7 && t.Characters[owner] >= 5 {
					tile += 2
				}
			} else if tile&192 == 64 {
				tile += tileOffset
			}
			result.OAM[slot+1] = tile
			attr := piece.Attributes
			if attr&3 < 2 {
				attr += paletteOffset
			}
			dx := uint8(piece.X)
			if !object.Pose.Mirrored {
				dx = ^(dx + 8) + 1
				attr ^= 64
			}
			result.OAM[slot+2] = attr
			sx := uint16(int32(projected.X) + int32(int8(dx)))
			result.OAM[slot+3] = uint8(sx)
			if sx>>8 != 0 {
				return false, nil
			}
			result.Pieces++
			return result.Pieces == 64, nil
		}
		for _, piece := range originalPostResultSpriteProfiles[kind-17][object.Pose.Frame] {
			full, err := emit(piece, false)
			if err != nil {
				return initial, err
			}
			if full {
				return result, nil
			}
		}
		if t.Scene == 1 && object.Airborne && context.SurfaceCodes[owner] == 0 {
			full, err := emit(postResultSpritePiece{Tile: 47, X: -5, Y: 16}, true)
			if err != nil {
				return initial, err
			}
			if full {
				return result, nil
			}
		}
	}
	for slot := int(result.Pieces) * 4; slot < 256; slot += 4 {
		result.OAM[slot] = 248
	}
	return result, nil
}

// ReverseOriginalPostResultOAM is original F002: reverse all64 four-byte slots,
// including hidden ones, after DMA. It is not a reversal of visible pieces only.
func ReverseOriginalPostResultOAM(previous [256]uint8) [256]uint8 {
	var result [256]uint8
	for slot := 0; slot < 64; slot++ {
		copy(result[slot*4:slot*4+4], previous[(63-slot)*4:(64-slot)*4])
	}
	return result
}
