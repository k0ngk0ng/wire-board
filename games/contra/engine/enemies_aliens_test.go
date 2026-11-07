package contra

// Failures considered before implementation: wrong stage dispatch, dropped
// original carry (including deliberate subtract-three), repeated aim timer
// index, wrong turn direction at half-circle, missing dead-player fallback,
// byte counter underflow, and initializing velocity a frame too late.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type alienOracleRow struct {
	ID, Actor     int
	Before, After struct {
		Enemies   [][]uint8
		World     []uint8
		Players   [2]EnemyTarget
		Collision string
		Graphics  struct {
			Used, Mode        uint8
			Bytes, Supertiles string
		}
	}
	Sounds     []uint8
	Requests   []struct{ X, Y, Code, Used uint8 }
	BiasWrites []struct {
		PC    int
		Value uint8
	}
}

func loadAlienOracle(t *testing.T, name string) ([]alienOracleRow, []string) {
	t.Helper()
	f, err := os.Open("testdata/original-enemies-aliens-" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Original    bool `json:"source_is_original_execution"`
		Records     []alienOracleRow
		WorldFields [][]json.RawMessage `json:"world_fields"`
	}
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original {
		t.Fatal("expected original execution")
	}
	names := make([]string, len(data.WorldFields))
	for i, w := range data.WorldFields {
		if err = json.Unmarshal(w[0], &names[i]); err != nil {
			t.Fatal(err)
		}
	}
	return data.Records, names
}

func TestOriginalAlienFetus(t *testing.T) {
	checked := 0
	for _, name := range []string{"natural", "fetus"} {
		rows, names := loadAlienOracle(t, name)
		for _, r := range rows {
			before := enemyPoolFromOracle(r.Before.Enemies)
			if before[r.Actor].Kind != 17 {
				continue
			}
			w, after := map[string]uint8{}, map[string]uint8{}
			for i, n := range names {
				w[n] = r.Before.World[i]
				after[n] = r.After.World[i]
			}
			b, err := hex.DecodeString(r.Before.Collision)
			if err != nil || len(b) != 128 {
				t.Fatal("original collision map", err)
			}
			terrain := CollisionWindow(b)
			s := EnemySystem{Slots: before, AlienAimIndex: w["alienAimIndex"]}
			c := EnemyStepContext{Level: w["level"], GameLoop: w["loop"], Frame: w["frame"], FrameScroll: w["frameScroll"], Random: w["random"], SceneType: w["sceneType"], Players: r.Before.Players,
				Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}}
			if err := s.StepAlienFetus(r.Actor, c); err != nil {
				t.Fatalf("%s id%d: %v", name, r.ID, err)
			}
			want := enemyPoolFromOracle(r.After.Enemies)
			if s.Slots != want {
				for i, e := range s.Slots {
					if e != want[i] {
						t.Fatalf("%s id%d slot%d got%+v want%+v before%+v world%v", name, r.ID, i, e, want[i], before[i], w)
					}
				}
			}
			if s.AlienAimIndex != after["alienAimIndex"] || !reflect.DeepEqual(s.PendingSounds, r.Sounds) && len(s.PendingSounds)+len(r.Sounds) != 0 {
				t.Fatalf("%s id%d aim%d/%d sounds%v/%v", name, r.ID, s.AlienAimIndex, after["alienAimIndex"], s.PendingSounds, r.Sounds)
			}
			if s.GenerationBiasChanged != (len(r.BiasWrites) > 0) {
				t.Fatalf("%s id%d bias write mismatch", name, r.ID)
			}
			if len(r.BiasWrites) > 0 && s.GenerationBiasValue != r.BiasWrites[len(r.BiasWrites)-1].Value {
				t.Fatalf("%s id%d bias %d/%v", name, r.ID, s.GenerationBiasValue, r.BiasWrites)
			}
			checked++
		}
	}
	if checked != 816 {
		t.Fatalf("fetus coverage %d want816", checked)
	}
}

// These cases were recorded before the four handlers were written. Full pool
// mutation, original queue admission and bytes, same-slot scroll/removal, and
// every observed $06 write are compared, not just final sprite positions.
func TestOriginalAlienSmallEntities(t *testing.T) {
	rows, names := loadAlienOracle(t, "small")
	if len(rows) != 1006 {
		t.Fatal("small alien coverage", len(rows))
	}
	raw, err := os.ReadFile("leveldata/stage-8.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	tileset, err := resources.Graphics(7, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []uint8{18, 19, 20, 21} {
		t.Run(map[uint8]string{18: "mouth", 19: "blob", 20: "spider", 21: "spawner"}[kind], func(t *testing.T) {
			for _, r := range rows {
				before := enemyPoolFromOracle(r.Before.Enemies)
				if before[r.Actor].Kind != kind {
					continue
				}
				w := map[string]uint8{}
				for i, n := range names {
					w[n] = r.Before.World[i]
				}
				b, _ := hex.DecodeString(r.Before.Collision)
				terrain := CollisionWindow(b)
				b, _ = hex.DecodeString(r.Before.Graphics.Supertiles)
				mapped := [128]uint8(b)
				q := GraphicsQueue{Used: r.Before.Graphics.Used}
				requests := 0
				s := EnemySystem{Slots: before}
				c := EnemyStepContext{Level: 7, GameLoop: w["loop"], WeaponStrength: w["weaponStrength"], Frame: w["frame"], FrameScroll: w["frameScroll"], Random: w["random"], Players: r.Before.Players, PlayerWeapons: [2]uint8{w["p0weapon"], w["p1weapon"]}, AttackEnabled: w["soldierAllowed"] != 0, Terrain: &terrain,
					Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}}
				c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
					if requests >= len(r.Requests) {
						t.Fatalf("id%d extra request %+v", r.ID, req)
					}
					want := r.Requests[requests]
					requests++
					if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
						t.Fatalf("id%d request%+v used%d want%+v", r.ID, req, q.Used, want)
					}
					v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
					return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
				}
				switch kind {
				case 18:
					err = s.StepAlienMouth(r.Actor, c)
				case 19:
					err = s.StepAlienBlob(r.Actor, c)
				case 20:
					err = s.StepAlienSpider(r.Actor, c)
				case 21:
					err = s.StepAlienSpawner(r.Actor, c)
				}
				if err != nil {
					t.Fatalf("id%d: %v", r.ID, err)
				}
				want := enemyPoolFromOracle(r.After.Enemies)
				if s.Slots != want {
					for i, e := range s.Slots {
						if e != want[i] {
							t.Fatalf("id%d slot%d\ngot%+v\nwant%+v\nbefore%+v world%v", r.ID, i, e, want[i], before[i], w)
						}
					}
				}
				if !bytes.Equal(s.PendingSounds, r.Sounds) {
					t.Fatalf("id%d sounds%v/%v", r.ID, s.PendingSounds, r.Sounds)
				}
				if s.GenerationBiasChanged != (len(r.BiasWrites) > 0) {
					t.Fatalf("id%d bias changed%v writes%+v", r.ID, s.GenerationBiasChanged, r.BiasWrites)
				}
				if len(r.BiasWrites) > 0 && s.GenerationBiasValue != r.BiasWrites[len(r.BiasWrites)-1].Value {
					t.Fatalf("id%d bias %d/%v", r.ID, s.GenerationBiasValue, r.BiasWrites)
				}
				encoded, _ := hex.DecodeString(r.Before.Graphics.Bytes)
				at := int(r.Before.Graphics.Used)
				for _, p := range q.Packets {
					b, e := p.Encode()
					if e != nil {
						t.Fatal(e)
					}
					copy(encoded[at:], b)
					at += len(b)
				}
				expected, _ := hex.DecodeString(r.After.Graphics.Bytes)
				expectedTerrain, _ := hex.DecodeString(r.After.Collision)
				if q.Used != r.After.Graphics.Used || requests != len(r.Requests) || !bytes.Equal(encoded, expected) || !bytes.Equal(terrain[:], expectedTerrain) {
					t.Fatalf("id%d graphics/terrain differ", r.ID)
				}
			}
		})
	}
}
