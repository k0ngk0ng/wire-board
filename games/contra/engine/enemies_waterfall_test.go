package contra

// Failure cases recorded before implementation: exact flip/reversal and byte
// wrap; offscreen removal followed by caller writes; full/self-reused slots;
// raw terrain carry including Y+8 overflow; scuba shot/timeout precedence;
// preservation of all other slots and original sound ordering.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type waterfallOracleState struct {
	Enemies   [][]uint8
	World     []uint8
	Players   []EnemyTarget
	Collision string
	Graphics  struct {
		Used, Mode        uint8
		Bytes, Supertiles string
	}
}

// These tests preceded both Boss implementations. Expected outcomes are from
// real Japanese dispatch execution; the graphics callback uses the real queue
// and stage-three tileset, including admission refusal and partial two-part draw.
func TestWaterfallOriginalBoss(t *testing.T) {
	data := readWaterfallOracle(t, "boss")
	raw, err := os.ReadFile("leveldata/stage-3.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	tileset, err := resources.Graphics(2, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range data.Records {
		before := enemyPoolFromOracle(r.Before.Enemies)
		s := EnemySystem{Slots: before}
		w := waterfallWorld(data, r.Before)
		after := waterfallWorld(data, r.After)
		mapBytes, _ := hex.DecodeString(r.Before.Collision)
		terrain := CollisionWindow(mapBytes)
		mappedBytes, _ := hex.DecodeString(r.Before.Graphics.Supertiles)
		mapped := [128]uint8(mappedBytes)
		q := GraphicsQueue{Used: r.Before.Graphics.Used}
		requests := 0
		resets := 0
		c := EnemyStepContext{Level: w["level"], Frame: w["frame"], FrameScroll: w["frameScroll"], Vertical: w["scrollType"] != 0, SceneType: w["sceneType"], Random: w["random"], AttackEnabled: w["soldierAllowed"] != 0, Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}}
		copy(c.Players[:], r.Before.Players)
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("case %d extra graphic request %+v", r.ID, req)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("case %d request %+v used %d want %+v", r.ID, req, q.Used, want)
			}
			v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
		}
		arms := w["destroyedArms"]
		boss := WaterfallBossContext{ScrollComplete: w["bossScrollComplete"], PaletteTimer: w["paletteTimer"], DestroyedArms: &arms,
			OnDefeated:   func() error { w["bossDefeated"] = 1; w["delayLow"] = 255; resets++; return nil },
			OnClearDelay: func(delay uint8) error { w["delayLow"] = delay; w["delayHigh"] = 0; return nil }}
		if before[r.Actor].Kind == 20 {
			err = s.StepDragonMouth(r.Actor, c, boss)
		} else {
			err = s.StepDragonArm(r.Actor, c, boss)
		}
		if err != nil {
			t.Fatalf("case %d kind %d state %d: %v", r.ID, before[r.Actor].Kind, before[r.Actor].Routine, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for slot, e := range s.Slots {
				if e != want[slot] {
					t.Fatalf("case %d kind %d state %d slot %d\ngot %+v\nwant %+v\nbefore %+v", r.ID, before[r.Actor].Kind, before[r.Actor].Routine, slot, e, want[slot], before[slot])
				}
			}
		}
		if arms != after["destroyedArms"] || w["bossDefeated"] != after["bossDefeated"] || w["delayLow"] != after["delayLow"] || w["delayHigh"] != after["delayHigh"] || resets != r.AudioResets {
			t.Fatalf("case %d Boss shared state differs", r.ID)
		}
		if !reflect.DeepEqual(append([]uint8{}, s.PendingSounds...), append([]uint8{}, r.Sounds...)) {
			t.Fatalf("case %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
		}
		bias := w["generationBias"]
		if s.GenerationBiasChanged {
			bias = s.GenerationBiasValue
		}
		if bias != after["generationBias"] {
			t.Fatalf("case %d bias %02x want %02x writes %+v", r.ID, bias, after["generationBias"], r.BiasWrites)
		}
		encoded, _ := hex.DecodeString(r.Before.Graphics.Bytes)
		at := int(r.Before.Graphics.Used)
		for _, packet := range q.Packets {
			b, e := packet.Encode()
			if e != nil {
				t.Fatal(e)
			}
			copy(encoded[at:], b)
			at += len(b)
		}
		expected, _ := hex.DecodeString(r.After.Graphics.Bytes)
		expectedTerrain, _ := hex.DecodeString(r.After.Collision)
		if q.Used != r.After.Graphics.Used || requests != len(r.Requests) || !bytes.Equal(encoded, expected) || !bytes.Equal(terrain[:], expectedTerrain) {
			t.Fatalf("case %d graphics/terrain differ", r.ID)
		}
	}
	t.Logf("%d original Dragon mouth and arm calls matched", len(data.Records))
}

type waterfallOracleRow struct {
	ID, Frame, Actor int
	Before, After    waterfallOracleState
	Sounds           []uint8
	AudioResets      int
	Requests         []struct{ X, Y, Code, Used uint8 }
	BiasWrites       []struct {
		PC    int
		Value uint8
	}
}
type waterfallOracle struct {
	Original    bool `json:"source_is_original_execution"`
	Records     []waterfallOracleRow
	WorldFields [][2]any `json:"world_fields"`
}

func readWaterfallOracle(t *testing.T, name string) waterfallOracle {
	t.Helper()
	f, err := os.Open("testdata/original-enemies-waterfall-" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data waterfallOracle
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original || len(data.Records) == 0 {
		t.Fatal("missing original waterfall execution")
	}
	return data
}
func waterfallWorld(data waterfallOracle, state waterfallOracleState) map[string]uint8 {
	w := map[string]uint8{}
	for i, field := range data.WorldFields {
		w[field[0].(string)] = state.World[i]
	}
	return w
}
func TestWaterfallOriginalHazards(t *testing.T) {
	for _, spec := range []struct {
		name  string
		count int
	}{{"hazards", 234}, {"natural", 5563}} {
		t.Run(spec.name, func(t *testing.T) {
			data := readWaterfallOracle(t, spec.name)
			if len(data.Records) != spec.count {
				t.Fatal("original hazard coverage", len(data.Records))
			}
			for _, r := range data.Records {
				before := enemyPoolFromOracle(r.Before.Enemies)
				s := EnemySystem{Slots: before}
				w := waterfallWorld(data, r.Before)
				bytes, err := hex.DecodeString(r.Before.Collision)
				if err != nil || len(bytes) != 128 {
					t.Fatal("invalid map")
				}
				terrain := CollisionWindow(bytes)
				c := EnemyStepContext{Level: w["level"], Frame: w["frame"], FrameScroll: w["frameScroll"], Vertical: w["scrollType"] != 0, SceneType: w["sceneType"], Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}}
				if before[r.Actor].Kind == 12 {
					err = s.StepScuba(r.Actor, c)
				} else {
					err = s.StepWaterfallHazard(r.Actor, c)
				}
				if err != nil {
					t.Errorf("case %d kind %d routine %d: %v", r.ID, before[r.Actor].Kind, before[r.Actor].Routine, err)
					continue
				}
				want := enemyPoolFromOracle(r.After.Enemies)
				if s.Slots != want {
					for slot, e := range s.Slots {
						if e != want[slot] {
							t.Fatalf("case %d slot %d\ngot %+v\nwant %+v\nbefore %+v", r.ID, slot, e, want[slot], before[slot])
						}
					}
				}
				if !reflect.DeepEqual(append([]uint8{}, s.PendingSounds...), append([]uint8{}, r.Sounds...)) {
					t.Fatalf("case %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
				}
			}
			if !t.Failed() {
				t.Logf("%d original Waterfall hazard/scuba calls matched", len(data.Records))
			}
		})
	}
}
