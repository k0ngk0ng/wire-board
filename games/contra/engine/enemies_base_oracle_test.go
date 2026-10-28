package contra

// Failure scenarios before base implementation: wrong bank/table selection;
// advancing a room before its delayed opening finishes; losing shared counters;
// generating with a full pool or on the wrong parity; draw refusal incorrectly
// advancing animation; wrong tile/supertile coordinate adapter; mutation of
// neighboring enemies, queue bytes or map cells; ignored byte wrapping.
import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type baseOracleSnapshot struct {
	InheritedThrowAttribute      *uint8 `json:"scratch0A"`
	Pool                         [][]uint8
	World                        []uint8
	Players                      [2]EnemyTarget
	QueueUsed                    uint8
	Queue, Supertiles, Collision string
}
type baseOracleRecord struct {
	ID, Frame, Slot, Type int
	Name                  string
	Before, After         baseOracleSnapshot
	Audio                 []struct {
		Kind string
		Code uint8
	}
	Requests []struct {
		Kind             string
		X, Y, Code, Used uint8
	}
}

func verifyBaseOriginal(t *testing.T, kinds []int, step func(*EnemySystem, int, EnemyStepContext) error) {
	t.Helper()
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	inputFile, err := os.Open("../../../docs/research/nes-contra/evidence/enemies-base-throw-inputs.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer inputFile.Close()
	inputZip, err := gzip.NewReader(inputFile)
	if err != nil {
		t.Fatal(err)
	}
	defer inputZip.Close()
	var observations struct {
		Original bool `json:"source_is_original_execution"`
		Fixtures []struct {
			Fixture, FixtureSHA256 string
			Records                []struct {
				ID, Slot int
				Before   uint8
			}
		}
	}
	if err = json.NewDecoder(inputZip).Decode(&observations); err != nil || !observations.Original {
		t.Fatal(err)
	}
	type observedInput struct {
		Slot  int
		Value uint8
	}
	throwInputs := map[string]map[int]observedInput{}
	throwSources := map[string]string{}
	for _, fixture := range observations.Fixtures {
		throwSources[fixture.Fixture] = fixture.FixtureSHA256
		values := map[int]observedInput{}
		for _, input := range fixture.Records {
			if _, exists := values[input.ID]; exists {
				t.Fatal("duplicate throw-input observation")
			}
			values[input.ID] = observedInput{input.Slot, input.Before}
		}
		throwInputs[fixture.Fixture] = values
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	paths := []string{"../../../docs/research/nes-contra/evidence/enemies-base-controlled.json.gz"}
	paths = append(paths, "../../../docs/research/nes-contra/evidence/enemies-base-waves-controlled.json.gz")
	paths = append(paths, "../../../docs/research/nes-contra/evidence/enemies-base-pickup-controlled.json.gz", "../../../docs/research/nes-contra/evidence/enemies-base-pickup-natural.json.gz")
	for _, stage := range []int{2, 4} {
		for _, mode := range []string{"room", "boss"} {
			paths = append(paths, fmt.Sprintf("../../../docs/research/nes-contra/evidence/enemies-base-%d-%s.json.gz", stage, mode))
		}
	}
	for _, path := range paths {
		fixtureRaw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fixtureHash := sha256.Sum256(fixtureRaw)
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var data struct {
			Original bool `json:"source_is_original_execution"`
			Records  []baseOracleRecord
		}
		err = json.NewDecoder(z).Decode(&data)
		z.Close()
		f.Close()
		if err != nil || !data.Original {
			t.Fatal(path, err)
		}
		for _, r := range data.Records {
			if r.Name != "step" || !slices.Contains(kinds, r.Type) {
				continue
			}
			if r.Type == 0 && len(r.Before.World) <= 24 {
				continue
			}
			count++
			b := r.Before
			inherited := b.InheritedThrowAttribute
			if inherited == nil {
				name := filepath.Base(path)
				if values, exists := throwInputs[name]; exists {
					if throwSources[name] != hex.EncodeToString(fixtureHash[:]) {
						t.Fatal("inherited-input source changed", path)
					}
					input, ok := values[r.ID]
					if !ok || input.Slot != r.Slot {
						t.Fatal("inherited-input actor missing/mismatched")
					}
					value := input.Value
					inherited = &value
				}
			}
			w := b.World
			wantw := r.After.World
			s := EnemySystem{Slots: enemyPoolFromOracle(b.Pool), Base: BaseEnemyState{w[12], w[13], w[14], w[15], w[16]}}
			cleared := w[9]
			random := RandomState{Value: w[4]}
			mapsRaw, _ := hex.DecodeString(b.Supertiles)
			maps := [128]uint8(mapsRaw)
			collisionRaw, _ := hex.DecodeString(b.Collision)
			collision := CollisionWindow(collisionRaw)
			c := EnemyStepContext{Level: w[2], Frame: w[0], WeaponStrength: w[1], GameLoop: w[3], Random: w[4], SharedRandom: &random, SceneType: w[6], Vertical: w[7] != 0, Screen: w[8], ScreenCleared: &cleared, FrameScroll: w[10], AttackEnabled: w[17] != 0, Players: b.Players, Terrain: &collision, Scroll: CollisionScroll{Y: w[21], X: w[22], NameTable: w[23]}}
			if len(w) > 24 {
				c.ScreenScroll = w[24]
			}
			variant := RegularMap
			if c.SceneType == 0x80 {
				variant = BaseBossMap
			}
			tiles, err := resources.Graphics(int(c.Level), variant)
			if err != nil {
				t.Fatal(err)
			}
			// Controlled dispatch changes SceneType but does not perform the
			// room loader: update definitions follow SceneType, while the
			// loaded source attribute page remains Base1 regular. Natural
			// Boss captures verify the normal matched Boss graphics context.
			if path == paths[0] && variant == BaseBossMap {
				source, err := resources.Graphics(1, RegularMap)
				if err != nil {
					t.Fatal(err)
				}
				tiles.BaseAttributes = source.BaseAttributes
			}
			animations, err := OriginalTileAnimations(c.Level)
			if err != nil {
				t.Fatal(err)
			}
			q := GraphicsQueue{Used: b.QueueUsed}
			requests := 0
			request := func(kind string, req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
				if requests >= len(r.Requests) {
					t.Fatalf("%s id%d unexpected %s %+v", path, r.ID, kind, req)
				}
				want := r.Requests[requests]
				requests++
				if want.Kind != kind || want.X != req.X || want.Y != req.Y || want.Code != req.Code || want.Used != q.Used {
					t.Fatalf("%s id%d request%d got%s %+v used%d want%+v", path, r.ID, requests, kind, req, q.Used, want)
				}
				gc := GraphicsContext{Scroll: c.Scroll, Supertiles: &maps, Tileset: tiles}
				var result GraphicsRequestResult
				var e error
				if kind == "tiles" {
					result, e = q.RequestTileAnimation(req.X, req.Y, req.Code, gc, animations)
				} else {
					result, e = q.RequestSupertile(req.X, req.Y, req.Code, gc)
				}
				return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, e
			}
			c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return request("supertile", req) }
			c.ChangeTiles = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return request("tiles", req) }
			c.InheritedThrowAttribute = inherited
			if err = step(&s, r.Slot, c); err != nil {
				t.Fatalf("%s id%d kind%x phase%d: %v", path, r.ID, r.Type, b.Pool[r.Slot][4], err)
			}
			want := enemyPoolFromOracle(r.After.Pool)
			if s.Slots != want {
				for i, e := range s.Slots {
					if e != want[i] {
						t.Fatalf("%s id%d kind%x phase%d slot%d\ngot %+v\nwant%+v\nbefore%+v", path, r.ID, r.Type, b.Pool[r.Slot][4], i, e, want[i], enemyFromOracle(b.Pool[i]))
					}
				}
			}
			if s.Base != (BaseEnemyState{wantw[12], wantw[13], wantw[14], wantw[15], wantw[16]}) || cleared != wantw[9] || random.Value != wantw[4] {
				t.Fatalf("%s id%d shared state differs: %+v cleared%d random%d", path, r.ID, s.Base, cleared, random.Value)
			}
			var sounds []uint8
			for _, a := range r.Audio {
				if a.Kind != "play" {
					t.Fatalf("unhandled audio %s", a.Kind)
				}
				sounds = append(sounds, a.Code)
			}
			if !slices.Equal(s.PendingSounds, sounds) || requests != len(r.Requests) {
				t.Fatalf("%s id%d sounds/requests got%v/%d want%v/%d", path, r.ID, s.PendingSounds, requests, sounds, len(r.Requests))
			}
			queue, _ := hex.DecodeString(b.Queue)
			at := int(b.QueueUsed)
			for _, p := range q.Packets {
				encoded, e := p.Encode()
				if e != nil {
					t.Fatal(e)
				}
				copy(queue[at:], encoded)
				at += len(encoded)
			}
			wantQueue, _ := hex.DecodeString(r.After.Queue)
			wantMap, _ := hex.DecodeString(r.After.Supertiles)
			wantCollision, _ := hex.DecodeString(r.After.Collision)
			if q.Used != r.After.QueueUsed || !bytes.Equal(queue, wantQueue) || !bytes.Equal(maps[:], wantMap) || !bytes.Equal(collision[:], wantCollision) {
				t.Fatalf("%s id%d graphics differs used%d/%d queue%t map%t collision%t\ngot %x\nwant%x", path, r.ID, q.Used, r.After.QueueUsed, bytes.Equal(queue, wantQueue), bytes.Equal(maps[:], wantMap), bytes.Equal(collision[:], wantCollision), queue, wantQueue)
			}
		}
	}
	if count == 0 {
		t.Fatal("no original calls selected")
	}
	t.Logf("verified %d original base calls", count)
}

func TestOriginalBaseWalls(t *testing.T) {
	verifyBaseOriginal(t, []int{0x13, 0x14}, func(s *EnemySystem, slot int, c EnemyStepContext) error {
		if s.Slots[slot].Kind == 0x13 {
			return s.StepBaseWallTurret(slot, c)
		}
		return s.StepBaseWallCore(slot, c)
	})
}
func TestOriginalBaseGenerators(t *testing.T) {
	verifyBaseOriginal(t, []int{0x19, 0x1a, 0x20}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepBaseGenerator(slot, c) })
}

func TestOriginalBaseIndoorSoldiers(t *testing.T) {
	verifyBaseOriginal(t, []int{0x15, 0x16, 0x17, 0x18}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepBaseIndoorSoldier(slot, c) })
}
func TestOriginalBaseThrownWeapons(t *testing.T) {
	verifyBaseOriginal(t, []int{0x11, 0x12}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepBaseThrownWeapon(slot, c) })
}
func TestOriginalBaseRedBlueSoldiers(t *testing.T) {
	verifyBaseOriginal(t, []int{0x1e, 0x1f}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepBaseRedBlueSoldier(slot, c) })
}

func TestOriginalBaseCommonWalls(t *testing.T) {
	verifyBaseOriginal(t, []int{8, 10}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepBaseCommonWall(slot, c) })
}
