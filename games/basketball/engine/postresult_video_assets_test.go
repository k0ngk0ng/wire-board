package engine

import (
	"reflect"
	"testing"
)

func TestOriginalPostResultVideoPaletteAssets(t *testing.T) {
	var rows struct {
		Palettes []struct {
			Background    uint8
			Sprites       [2]uint8
			Flags, Level  uint8
			Before, After EndingUpload
		}
	}
	readPropAttackFixture(t, "testdata/postresult-video-assets.json.gz", &rows)
	if len(rows.Palettes) != 1900 {
		t.Fatal("palette source matrix", len(rows.Palettes))
	}
	for i, r := range rows.Palettes {
		got, e := BuildOriginalPresentationPalette(r.Before, r.Background, r.Sprites, r.Flags, r.Level)
		if e != nil || got != r.After {
			t.Fatal(i, e, got, r.After)
		}
	}
}
func TestOriginalPostResultVideoBackgroundAssets(t *testing.T) {
	var rows struct {
		Backgrounds []struct {
			Preset, Palette_Preset, Compressed_Bank uint8
			Target                                  uint16
			Field_Banks                             [2]uint8
			Data                                    []uint8
		}
		Work []struct {
			Preset uint8
			Cycles uint32
		}
	}
	readPropAttackFixture(t, "testdata/postresult-video-assets.json.gz", &rows)
	for i, r := range rows.Backgrounds {
		got, e := OriginalPresentationBackground(r.Preset)
		if e != nil || got.Target != r.Target || got.PaletteID != r.Palette_Preset || got.CompressedCHR != r.Compressed_Bank || got.FieldCHR != r.Field_Banks || !reflect.DeepEqual(got.Data, r.Data) || got.WorkCycles != rows.Work[i].Cycles {
			t.Fatal(i, "background", e)
		}
	}
}
func TestOriginalPostResultVideoAssetsRejectUnknown(t *testing.T) {
	if _, e := OriginalPresentationBackground(255); e == nil {
		t.Fatal("unknown background accepted")
	}
	original := EndingUpload{Length: 5}
	got, e := BuildOriginalPresentationPalette(original, 255, [2]uint8{2, 3}, 0, 16)
	if e == nil || got != original {
		t.Fatal("unknown palette partially changed")
	}
}
