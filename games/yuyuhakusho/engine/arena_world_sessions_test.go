package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

var arenaWorldIDsTest = []uint16{0, 2, 4, 6, 8, 10, 12, 14}

func arenaWorldReadTest(t *testing.T, path string, target any) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= 2 && raw[0] == 31 && raw[1] == 139 {
		z, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		raw, err = io.ReadAll(z)
		if err != nil {
			t.Fatal(err)
		}
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(path, err)
	}
	return raw
}

// Typed static rules and resource metadata are the only additions. No captured
// world state, original RAM or media is supplied to DecodeChuWorldRules.
func arenaWorldRuleJSONTest(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var rules map[string]json.RawMessage
	arenaWorldReadTest(t, "../data/chu-world-rules.json.gz", &rules)
	var linear, four map[string]json.RawMessage
	arenaWorldReadTest(t, "../data/arena-scene-linear-rows.json.gz", &linear)
	arenaWorldReadTest(t, "../data/arena-scene-4-tables.json.gz", &four)
	scenes := map[string]json.RawMessage{"linear_rows": linear["linear_rows"], "arena4_rows": four["arena4_rows"], "arena4_colors": four["arena4_colors"]}
	rules["arena_scenes"], _ = json.Marshal(scenes)
	var resources []map[string]any
	for _, id := range arenaWorldIDsTest {
		label := fmt.Sprintf("%x", id)
		if id == 0 {
			label = "default"
		}
		var asset struct {
			Palette [16]uint16 `json:"initial_scene_palette_0"`
			Binding struct {
				Source uint32 `json:"resource_list"`
			} `json:"resource_binding"`
		}
		arenaWorldReadTest(t, "../data/arena-assets-"+label+".json.gz", &asset)
		var palette [16]uint16
		for i, compact := range asset.Palette {
			palette[i] = (compact&7)<<1 | ((compact>>3)&7)<<5 | ((compact>>6)&7)<<9
		}
		resources = append(resources, map[string]any{"arena": id, "source": asset.Binding.Source, "palette_0": palette})
	}
	rules["arena_resources"], _ = json.Marshal(resources)
	return rules
}

func arenaWorldLibraryTest(t *testing.T) ChuWorldLibrary {
	t.Helper()
	raw, _ := json.Marshal(arenaWorldRuleJSONTest(t))
	library, err := DecodeChuWorldRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	return library
}

type arenaWorldOriginalPassTest struct {
	chuWorldOriginalPass
	RawArenaSprites []ArenaASprite `json:"raw_arena_sprites"`
}

func arenaWorldPassesTest(t *testing.T, arena uint16) []arenaWorldOriginalPassTest {
	t.Helper()
	name := fmt.Sprintf("testdata/arena-world-%x", arena)
	compressed, err := os.ReadFile(name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Arena                                   uint16
		Records, FramesChecked, IncompleteCalls int
		FramesMatch                             bool   `json:"frames_match_original_control"`
		FixtureSHA256                           string `json:"fixture_sha256"`
		GzipSHA256                              string `json:"gzip_sha256"`
		Identity                                struct {
			ROMSHA256  string `json:"rom_sha256"`
			CoreSHA256 string `json:"core_sha256"`
		}
		Pass bool
	}
	// Explicit wire tags retain the actual producer's underscored field names.
	var metadata map[string]json.RawMessage
	arenaWorldReadTest(t, name+"-manifest.json", &metadata)
	rawManifest, _ := json.Marshal(metadata)
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(metadata["frames_checked"], &manifest.FramesChecked)
	_ = json.Unmarshal(metadata["incomplete_calls"], &manifest.IncompleteCalls)
	sum := sha256.Sum256(compressed)
	if !manifest.Pass || !manifest.FramesMatch || manifest.Arena != arena || manifest.FramesChecked != 3602 || manifest.IncompleteCalls != 0 || hex.EncodeToString(sum[:]) != manifest.GzipSHA256 ||
		manifest.Identity.ROMSHA256 != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" ||
		manifest.Identity.CoreSHA256 != "e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306" {
		t.Fatal("unverified whole-arena original control capture")
	}
	var rows []arenaWorldOriginalPassTest
	raw := arenaWorldReadTest(t, name+".json.gz", &rows)
	sum = sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != manifest.FixtureSHA256 || len(rows) != manifest.Records || rows[0].Frame != 2700 || rows[len(rows)-1].Frame != 3600 {
		t.Fatal("whole-arena original fixture is incomplete or changed")
	}
	return rows
}

func compareArenaWorldSpritesTest(t *testing.T, row arenaWorldOriginalPassTest, events []ChuWorldEvent) {
	t.Helper()
	var got []ArenaASprite
	for _, event := range events {
		if event.Kind != "arena-sprite" {
			continue
		}
		var fields struct{ RawArenaSprite *ArenaASprite }
		raw, _ := json.Marshal(event)
		if err := json.Unmarshal(raw, &fields); err != nil || fields.RawArenaSprite == nil || event.ObjectID != 0x9e00 {
			t.Fatal("raw arena sprite event lost its immediate descriptor", err)
		}
		got = append(got, *fields.RawArenaSprite)
	}
	if len(got) != len(row.RawArenaSprites) || len(got) != 0 && !reflect.DeepEqual(got, row.RawArenaSprites) {
		t.Fatalf("world frame=%d arena sprite order differs: got=%+v want=%+v", row.Frame, got, row.RawArenaSprites)
	}
}

func TestArenaWorldOriginalWholePasses(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range arenaWorldIDsTest[1:] {
		t.Run(fmt.Sprintf("%x", arena), func(t *testing.T) {
			rows := arenaWorldPassesTest(t, arena)
			calls := 0
			for _, row := range rows {
				state := decodeChuWorldTest(t, row.Before)
				copy := decodeChuWorldTest(t, row.Before)
				got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
				if err != nil {
					t.Fatal(row.Frame, err)
				}
				if !reflect.DeepEqual(state, copy) {
					t.Fatal("whole-pass callback changed caller-owned state", row.Frame)
				}
				compareChuWorldOrderTest(t, row.chuWorldOriginalPass, events)
				compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
				compareChuWorldBytesTest(t, got, row.chuWorldOriginalPass, events)
				compareArenaWorldSpritesTest(t, row, events)
				calls += len(row.Calls)
			}
			t.Logf("arena=%x original whole-passes=%d callbacks=%d", arena, len(rows), calls)
		})
	}
}

func TestArenaWorldSingleInitialContinuous(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range arenaWorldIDsTest[1:] {
		t.Run(fmt.Sprintf("%x", arena), func(t *testing.T) {
			rows := arenaWorldPassesTest(t, arena)
			state := decodeChuWorldTest(t, rows[0].Before)
			for i, row := range rows {
				if i != 0 {
					begin, out, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
					if err != nil || out.ResetRequested {
						t.Fatal("continuous frame begin", row.Frame, err)
					}
					state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
				}
				got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
				if err != nil {
					t.Fatal(row.Frame, err)
				}
				compareChuWorldOrderTest(t, row.chuWorldOriginalPass, events)
				compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
				compareChuWorldBytesTest(t, got, row.chuWorldOriginalPass, events)
				compareArenaWorldSpritesTest(t, row, events)
				state = got
				state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
				if err != nil {
					t.Fatal(err)
				}
				if i == len(rows)/2 {
					raw, _ := json.Marshal(state)
					var restored ChuWorldState
					if err = json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(restored, state) {
						t.Fatal("whole-arena session save/restore changed owned state", err)
					}
					state = restored
				}
			}
			t.Logf("arena=%x single-initial whole-passes=%d final-frame=%d", arena, len(rows), rows[len(rows)-1].Frame)
		})
	}
}
