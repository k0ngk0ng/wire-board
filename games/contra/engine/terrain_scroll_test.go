package contra

// Failure inventory written before implementation: wrong active map variant;
// lost 240px vertical wrap; premature whole-map collision overwrite; wrong
// attribute nibble mixing; stale alternate trigger/header; palette/CHR request
// reordering; queue-prefix loss or wrong byte grouping; invalid resources or
// queue overflow committing only part of a terrain phase. Expected maps,
// collision and queue bytes come from original DB81 calls on actual ROM data.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type terrainScrollSnapshot struct {
	State                                                  []uint8
	Variant                                                MapVariant
	Header, Pointers, Maps, Collision, Queue, Palette, CHR string
}
type terrainScrollRecord struct {
	Frame         int
	Before, After terrainScrollSnapshot
	Effects       []TerrainScrollEffect
	PPUUnchanged  bool `json:"ppuUnchanged"`
}
type terrainScrollOracle struct {
	Fields   []struct{ Name, Address string }
	Captures []struct {
		Name    string
		Stage   int
		Records []terrainScrollRecord
	}
}

func terrainScrollOriginal(t *testing.T, scope string) terrainScrollOracle {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/terrain-scroll-" + scope + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d terrainScrollOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if len(d.Fields) != 31 || len(d.Captures) == 0 || scope != "counters" && len(d.Captures) != 8 {
		t.Fatalf("incomplete %s scrolling oracle", scope)
	}
	return d
}
func terrainScrollResources(t *testing.T, name string) *LevelResources {
	t.Helper()
	raw, err := os.ReadFile("leveldata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func scrollBytes(t *testing.T, s string, n int) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || (n >= 0 && len(b) != n) {
		t.Fatal("invalid original scroll bytes", err, len(b), n)
	}
	return b
}
func terrainScrollState(t *testing.T, d terrainScrollOracle, s terrainScrollSnapshot) (TerrainScrollState, OutdoorWorld, CollisionWindow, PaletteState, uint8, map[string]uint8) {
	t.Helper()
	v := map[string]uint8{}
	for i, f := range d.Fields {
		v[f.Name] = s.State[i]
	}
	state := TerrainScrollState{AlternateGraphicsScreen: v["alternate"], Axis: v["axis"], TransitionRemaining: v["remaining"],
		TileAddress: uint16(v["tileLo"]) | uint16(v["tileHi"])<<8, AttributeAddress: uint16(v["attrLo"]) | uint16(v["attrHi"])<<8,
		MapOffset: v["mapOffset"], TankScroll: v["tankScroll"], Variant: s.Variant, Supertiles: [128]uint8(scrollBytes(t, s.Maps, 128))}
	w := OutdoorWorld{Frame: v["frame"], Level: v["level"], SceneType: v["sceneType"], ScrollType: v["scrollType"], StopScroll: v["stopScroll"], Screen: v["screen"], ScreenScroll: v["screenScroll"],
		FrameScroll: v["frameScroll"], AutoScroll: [2]uint8{v["autoA"], v["autoB"]}, IndoorScroll: v["indoorScroll"], BossScrollComplete: v["bossScrollComplete"],
		Scroll: CollisionScroll{X: v["scrollX"], Y: v["scrollY"], NameTable: v["ppuControl"]}}
	raw := scrollBytes(t, s.Header, 15)
	h := PaletteState{CollisionLimits: [3]uint8(raw[:3]), CycleIndices: [4]uint8(raw[3:7]), PaletteIndices: [8]uint8(raw[7:])}
	return state, w, CollisionWindow(scrollBytes(t, s.Collision, 128)), h, v["screenCleared"], v
}

func TestOriginalTerrainScrollAllStages(t *testing.T) {
	stageResources := map[uint8]*LevelResources{}
	count := 0
	levels := map[uint8]int{}
	effects := map[TerrainScrollEffectKind]int{}
	for _, scope := range []string{"natural", "boundaries", "counters"} {
		d := terrainScrollOriginal(t, scope)
		for _, capture := range d.Captures {
			for _, row := range capture.Records {
				s, w, c, h, clear, external := terrainScrollState(t, d, row.Before)
				resources := stageResources[w.Level]
				if resources == nil {
					resources = terrainScrollResources(t, fmt.Sprintf("stage-%d.json", w.Level+1))
					stageResources[w.Level] = resources
				}
				wantS, wantW, wantC, wantH, wantClear, wantExternal := terrainScrollState(t, d, row.After)
				prefix := scrollBytes(t, row.Before.Queue, -1)
				q := GraphicsQueue{Used: uint8(len(prefix))}
				result, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &q, Resources: resources})
				if err != nil {
					t.Fatalf("%s %s frame%d: %v", scope, capture.Name, row.Frame, err)
				}
				if s != wantS || w != wantW || c != wantC || h != wantH || clear != wantClear {
					t.Fatalf("%s %s frame%d phase differs\nscroll=%+v\nwant=%+v\nworld=%+v\nwant=%+v\nheader=%+v want=%+v clear=%d want=%d\ncollision=%x want=%x", scope, capture.Name, row.Frame, s, wantS, w, wantW, h, wantH, clear, wantClear, c, wantC)
				}
				encoded := append([]uint8{}, prefix...)
				for _, packet := range q.Packets {
					raw, err := packet.Encode()
					if err != nil {
						t.Fatal(err)
					}
					encoded = append(encoded, raw...)
				}
				wantQueue := scrollBytes(t, row.After.Queue, -1)
				if !reflect.DeepEqual(encoded, wantQueue) || int(q.Used) != len(wantQueue) {
					t.Fatalf("%s frame%d queue differs got=%x/%d want=%x/%d", capture.Name, row.Frame, encoded, q.Used, wantQueue, len(wantQueue))
				}
				if !reflect.DeepEqual(append([]TerrainScrollEffect{}, result.Effects...), append([]TerrainScrollEffect{}, row.Effects...)) {
					t.Fatalf("%s frame%d effects %+v want %+v", capture.Name, row.Frame, result.Effects, row.Effects)
				}
				if result.ResetEnemyReadOffset {
					external["enemyOffset"] = 0
				}
				if external["enemyOffset"] != wantExternal["enemyOffset"] || external["force0"]+result.ForceJump[0] != wantExternal["force0"] || external["force1"]+result.ForceJump[1] != wantExternal["force1"] {
					t.Fatalf("%s frame%d cross-phase signals differ", capture.Name, row.Frame)
				}
				if !row.PPUUnchanged {
					t.Fatal("source changed display within queued phase")
				}
				for _, effect := range result.Effects {
					effects[effect.Kind]++
				}
				levels[w.Level]++
				count++
			}
		}
	}
	if count < 8000 || len(levels) != 8 {
		t.Fatalf("missing actual stage coverage count%d levels%v", count, levels)
	}
	t.Logf("%d original terrain phases match eight-stage state/maps/collision/queue and ordered effects; levels=%v effects=%v", count, levels, effects)
}

func TestTerrainScrollFailureLeavesWholePhaseUnchanged(t *testing.T) {
	resources := terrainScrollResources(t, "levels.json")
	d := terrainScrollOriginal(t, "boundaries")
	var row terrainScrollRecord
	found := false
	for _, capture := range d.Captures {
		if capture.Stage != 1 {
			continue
		}
		for _, r := range capture.Records {
			if r.Before.Header != r.After.Header && r.Before.Queue != r.After.Queue {
				row = r
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("missing real alternate+draw boundary")
	}
	for _, kind := range []string{"queue-overflow", "missing-stage"} {
		t.Run(kind, func(t *testing.T) {
			s, w, c, h, clear, _ := terrainScrollState(t, d, row.Before)
			q := GraphicsQueue{Used: 160, Packets: []GraphicsPacket{{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{0x5a}}}}}}
			beforeS, beforeW, beforeC, beforeH, beforeClear, beforeQ := s, w, c, h, clear, q
			r := resources
			if kind == "missing-stage" {
				r = terrainScrollResources(t, "stage-2.json")
			}
			result, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &q, Resources: r})
			if err == nil {
				t.Fatal("invalid phase unexpectedly committed")
			}
			if s != beforeS || w != beforeW || c != beforeC || h != beforeH || clear != beforeClear || !reflect.DeepEqual(q, beforeQ) || len(result.Effects) != 0 {
				t.Fatal("failed scroll left partial state or pending effects")
			}
		})
	}
}

func TestTerrainScrollAndEnemyShareOriginalQueueBudget(t *testing.T) {
	d := terrainScrollOriginal(t, "natural")
	r := terrainScrollResources(t, "stage-1.json")
	for _, capture := range d.Captures {
		if capture.Stage != 1 {
			continue
		}
		for _, row := range capture.Records {
			s, w, c, h, clear, _ := terrainScrollState(t, d, row.Before)
			if len(row.Before.Queue) != 0 || w.FrameScroll != 1 || w.ScreenScroll&15 != 2 {
				continue
			}
			var q GraphicsQueue
			if _, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &q, Resources: r}); err != nil {
				t.Fatal(err)
			}
			if q.Used != 24 {
				t.Fatalf("attribute packet grouping changed: %d", q.Used)
			}
			set, err := r.Graphics(0, s.Variant)
			if err != nil {
				t.Fatal(err)
			}
			ctx := GraphicsContext{Scroll: w.Scroll, Supertiles: &s.Supertiles, Tileset: set}
			for i := 0; i < 2; i++ {
				v, err := q.RequestSupertile(32, 48, 0, ctx)
				if err != nil || !v.Accepted {
					t.Fatal("premature shared budget rejection", i, v, err)
				}
			}
			used := q.Used
			v, err := q.RequestSupertile(32, 48, 0, ctx)
			if err != nil || v.Accepted || q.Used != used {
				t.Fatal("enemy request bypassed shared admission", v, err)
			}
			return
		}
	}
	t.Fatal("missing real empty-prefix attribute phase")
}

func TestTerrainScrollLateResourceFailureIsAtomic(t *testing.T) {
	d := terrainScrollOriginal(t, "boundaries")
	for _, kind := range []string{"missing-lookahead-map", "bad-late-supertile"} {
		t.Run(kind, func(t *testing.T) {
			resources := terrainScrollResources(t, "levels.json")
			var chosen terrainScrollRecord
			found := false
			for _, capture := range d.Captures {
				if capture.Stage != 1 {
					continue
				}
				for _, row := range capture.Records {
					s, w, _, _, _, _ := terrainScrollState(t, d, row.Before)
					if kind == "missing-lookahead-map" && row.Before.Maps != row.After.Maps && s.Axis == 31 {
						chosen = row
						found = true
						break
					}
					if kind == "bad-late-supertile" && w.ScreenScroll&7 == 7 && uint8(s.TileAddress)&1 == 0 && row.Before.Maps == row.After.Maps {
						chosen = row
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatal("missing original late-failure phase")
			}
			s, w, c, h, clear, _ := terrainScrollState(t, d, chosen.Before)
			if kind == "missing-lookahead-map" {
				stage := resources.stages[0]
				next := int(w.Screen + 2)
				filtered := stage.Maps.Entries[:0:0]
				for _, entry := range stage.Maps.Entries {
					if entry.Index != next {
						filtered = append(filtered, entry)
					}
				}
				stage.Maps.Entries = filtered
				resources.stages[0] = stage
			} else {
				s.Supertiles[int(s.MapOffset)+48+int(s.Axis/4)] = 255
			}
			q := GraphicsQueue{Used: uint8(len(scrollBytes(t, chosen.Before.Queue, -1)))}
			beforeS, beforeW, beforeC, beforeH, beforeClear, beforeQ := s, w, c, h, clear, q
			result, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &q, Resources: resources})
			if err == nil {
				t.Fatal("missing resource unexpectedly accepted")
			}
			if s != beforeS || w != beforeW || c != beforeC || h != beforeH || clear != beforeClear || !reflect.DeepEqual(q, beforeQ) || len(result.Effects) != 0 {
				t.Fatal("late resource error committed a partial scroll")
			}
		})
	}
}
