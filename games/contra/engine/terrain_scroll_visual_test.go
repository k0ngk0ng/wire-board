package contra

// Integration gate written before World wires the scroll effects into visual
// output: real PaletteOutput.Load must produce original palette bytes, request
// count, fade timer and generation compatibility; CHR presets affect desired
// pages only. No recorded after-buffer is supplied to a handler.
import "testing"

func TestOriginalTerrainScrollWithPaletteAndCHR(t *testing.T) {
	resources := terrainScrollResources(t, "levels.json")
	checked := 0
	for _, scope := range []string{"natural", "boundaries", "counters"} {
		d := terrainScrollOriginal(t, scope)
		for _, capture := range d.Captures {
			for _, row := range capture.Records {
				s, w, c, h, clear, external := terrainScrollState(t, d, row.Before)
				_, _, _, _, _, wantExternal := terrainScrollState(t, d, row.After)
				palette := PaletteOutput{FadeTimer: external["paletteTimer"], RequestedColors: external["paletteCount"]}
				copy(palette.Desired[:], scrollBytes(t, row.Before.Palette, 32))
				// Applied is a canary: DB81 must not cross the display boundary.
				for i := range palette.Applied {
					palette.Applied[i] = 0x2a
				}
				applied := palette.Applied
				patterns := PatternPages{Desired: [8]uint8(scrollBytes(t, row.Before.CHR, 8)), Applied: [8]uint8{7, 6, 5, 4, 3, 2, 1, 0}}
				appliedPatterns := patterns.Applied
				queue := GraphicsQueue{Used: uint8(len(scrollBytes(t, row.Before.Queue, -1)))}
				result, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &queue, Resources: resources})
				if err != nil {
					t.Fatal(capture.Name, row.Frame, err)
				}
				bias := external["generationBias"]
				for _, effect := range result.Effects {
					switch effect.Kind {
					case ScrollPaletteTimer:
						palette.FadeTimer = effect.Value
					case ScrollPaletteLoad:
						loaded, err := palette.Load(effect.Value, w.SceneType, h, resources)
						if err != nil {
							t.Fatal(capture.Name, row.Frame, err)
						}
						if loaded.GenerationBiasChanged {
							bias = loaded.GenerationBiasValue
						}
					case ScrollCHRPreset:
						pages, err := resources.CHRPreset(int(effect.Value))
						if err != nil {
							t.Fatal(err)
						}
						patterns.Desired = pages
					case ScrollAudioReset, ScrollSound:
						// Audio requests are compared by the core scroll oracle. They
						// do not authorize changes to these visual state owners.
					default:
						t.Fatal("unrecognized terrain effect", effect)
					}
				}
				wantPalette := [32]uint8(scrollBytes(t, row.After.Palette, 32))
				wantPages := [8]uint8(scrollBytes(t, row.After.CHR, 8))
				if palette.Desired != wantPalette || palette.RequestedColors != wantExternal["paletteCount"] || palette.FadeTimer != wantExternal["paletteTimer"] || bias != wantExternal["generationBias"] || patterns.Desired != wantPages {
					t.Fatalf("%s frame%d visual effect composition differs: palette=%x want=%x requested=%d/%d fade=%d/%d bias=%d/%d chr=%x/%x", capture.Name, row.Frame, palette.Desired, wantPalette, palette.RequestedColors, wantExternal["paletteCount"], palette.FadeTimer, wantExternal["paletteTimer"], bias, wantExternal["generationBias"], patterns.Desired, wantPages)
				}
				if palette.Applied != applied || patterns.Applied != appliedPatterns {
					t.Fatal("scroll presented effects before the display boundary")
				}
				checked++
			}
		}
	}
	if checked < 8070 {
		t.Fatal("incomplete joint scroll/visual oracle", checked)
	}
	t.Logf("%d original complete DB81 visual outputs match real Palette and CHR composition", checked)
}
