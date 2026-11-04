package main

import (
	"reflect"
	"strings"
	"testing"
)

// Original complete palette calls/private original-frame pixels and these
// tests precede the independent indexed-asset implementation.
func characterAssetInputs(t *testing.T) (characterAssets, characterAssetOracle, sourceSpriteFixture) {
	t.Helper()
	a, o, s, err := loadCharacterInputs("../../data")
	if err != nil {
		t.Fatal(err)
	}
	return a, o, s
}

func TestCharacterAssetsAgainstOriginal(t *testing.T) {
	a, o, s := characterAssetInputs(t)
	report, err := verifyCharacterAssets(a, o, s)
	if err != nil || !report.Pass || report.Characters != 11 || report.Frames != 1133 || report.Tiles != 32567 || report.PaletteCalls != 528 || report.PixelCases != 132 || report.Pixels < 100000 || report.Mismatches != 0 || report.PriorDependentFrames != 7 {
		t.Fatalf("indexed asset/source/CPU/pixel comparison failed: %+v err=%v", report, err)
	}
	t.Logf("complete palette calls=%d original-frame pixels=%d cases=%d source characters=%d", report.PaletteCalls, report.Pixels, report.PixelCases, report.Characters)
}

func TestCharacterAssetPriorSlotState(t *testing.T) {
	a, _, _ := characterAssetInputs(t)
	var frame assetFrame
	found := false
	for _, f := range a.Frames {
		if f.CharacterID == 1 && f.Parameter == 184 {
			frame, found = f, true
		}
	}
	if !found || !reflect.DeepEqual(frame.RequiredPriorSlots, []uint16{83}) {
		t.Fatal("known original prior-tile dependency missing")
	}
	if _, err := resolveFrameSlots(frame, nil, len(a.TileSources)); err == nil {
		t.Fatal("uninitialized prior tile was silently replaced by transparency")
	}
	prior := map[uint16]uint32{83: 0, 511: 3}
	got, err := resolveFrameSlots(frame, prior, len(a.TileSources))
	if err != nil || got[83] != 0 || got[511] != 3 || !reflect.DeepEqual(prior, map[uint16]uint32{83: 0, 511: 3}) {
		t.Fatal("prior mapping was lost or caller mapping mutated", err)
	}
	got[83] = 1
	if prior[83] != 0 {
		t.Fatal("result aliases caller tile mapping")
	}
	bad := frame
	bad.Assignments = append([]assetAssignment(nil), frame.Assignments...)
	bad.Assignments[0].Tile = uint32(len(a.TileSources))
	if _, err := resolveFrameSlots(bad, prior, len(a.TileSources)); err == nil || prior[83] != 0 {
		t.Fatal("bad tile accepted or storage changed before error")
	}
}

func TestCharacterAssetsRejectCorruption(t *testing.T) {
	a, o, s := characterAssetInputs(t)
	for _, corruption := range []string{"tile_payload", "tile_source", "piece_geometry", "palette", "palette_binding", "pixel_timing", "pixel_video", "prior_dependency", "asset_binding"} {
		t.Run(corruption, func(t *testing.T) {
			x, y := a, o
			switch corruption {
			case "tile_payload":
				x.TileBytes = "ff" + x.TileBytes[2:]
			case "tile_source":
				x.TileSources = append([]assetTileSource(nil), a.TileSources...)
				x.TileSources[0].Address++
			case "piece_geometry":
				x.Frames = append([]assetFrame(nil), a.Frames...)
				x.Frames[0].Pieces = append([]assetPiece(nil), a.Frames[0].Pieces...)
				x.Frames[0].Pieces[0].CenterX++
			case "palette":
				x.Palettes = append([]assetPalette(nil), a.Palettes...)
				x.Palettes[0].Variants[0][0] ^= 2
			case "palette_binding":
				y.Bindings = append([]assetPaletteCall(nil), o.Bindings...)
				y.Bindings[0].SourcePointer++
			case "pixel_timing":
				y.Pixels = append([]assetPixelCase(nil), o.Pixels...)
				y.Pixels[0].VideoFrame = y.Pixels[0].StateFrame
			case "pixel_video":
				y.Pixels = append([]assetPixelCase(nil), o.Pixels...)
				y.Pixels[0].Video = strings.Repeat("00", len(y.Pixels[0].Video)/2)
			case "prior_dependency":
				x.Frames = append([]assetFrame(nil), a.Frames...)
				for i := range x.Frames {
					if len(x.Frames[i].RequiredPriorSlots) != 0 {
						x.Frames[i].RequiredPriorSlots = nil
						break
					}
				}
			case "asset_binding":
				y.AssetSHA = "unbound"
			}
			if _, err := verifyCharacterAssets(x, y, s); err == nil {
				t.Fatal("corrupted independent asset or original evidence accepted")
			}
		})
	}
}

func TestCharacterAssetsRequireCompleteCoverage(t *testing.T) {
	a, o, s := characterAssetInputs(t)
	for _, scope := range []string{"palette_calls", "pixel_cases", "frames", "characters", "duplicate_palette", "duplicate_pixel"} {
		t.Run(scope, func(t *testing.T) {
			x, y := a, o
			switch scope {
			case "palette_calls":
				y.Bindings = o.Bindings[1:]
			case "pixel_cases":
				y.Pixels = o.Pixels[1:]
			case "frames":
				x.Frames = a.Frames[1:]
			case "characters":
				x.Palettes = a.Palettes[:10]
			case "duplicate_palette":
				y.Bindings = append([]assetPaletteCall(nil), o.Bindings...)
				y.Bindings[0] = y.Bindings[1]
			case "duplicate_pixel":
				y.Pixels = append([]assetPixelCase(nil), o.Pixels...)
				y.Pixels[0] = y.Pixels[1]
			}
			if _, err := verifyCharacterAssets(x, y, s); err == nil {
				t.Fatal("incomplete or repeated original coverage accepted")
			}
		})
	}
}
