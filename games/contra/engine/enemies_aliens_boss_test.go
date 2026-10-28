package contra

// Source fixtures and failure cases preceded these Boss implementations:
// byte-wrapped HP scaling, partial pair drawing, stale draw-address collision
// removal, full/self-reused pools, cleanup including inactive kinds but not slot
// zero, Japanese sound/palette state and no premature final clear.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type alienBossSnapshot struct {
	Enemies   [][]uint8
	World     []uint8
	Collision string
	Graphics  struct {
		Used, Mode        uint8
		Bytes, Supertiles string
	}
}
type alienBossOracle struct {
	Original bool     `json:"source_is_original_execution"`
	Fields   [][2]any `json:"world_fields"`
	Records  []struct {
		ID, Frame, Actor int
		Before, After    alienBossSnapshot
		Sounds           []uint8
		AudioResets      int
		Requests         []struct{ X, Y, Code, Used uint8 }
	}
}

func TestOriginalAlienGuardianAndHeart(t *testing.T) {
	f, err := os.Open("testdata/original-enemies-aliens-boss.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data alienBossOracle
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original || len(data.Records) != 617 {
		t.Fatal("original Boss coverage")
	}
	world := func(v []uint8) map[string]uint8 {
		m := map[string]uint8{}
		for i, p := range data.Fields {
			m[p[0].(string)] = v[i]
		}
		return m
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
	for _, r := range data.Records {
		before := enemyPoolFromOracle(r.Before.Enemies)
		s := EnemySystem{Slots: before}
		w := world(r.Before.World)
		after := world(r.After.World)
		mapBytes, _ := hex.DecodeString(r.Before.Collision)
		terrain := CollisionWindow(mapBytes)
		mappedBytes, _ := hex.DecodeString(r.Before.Graphics.Supertiles)
		mapped := [128]uint8(mappedBytes)
		q := GraphicsQueue{Used: r.Before.Graphics.Used}
		requests, resets := 0, 0
		header := PaletteState{}
		header.CycleIndices[1] = w["cycle1"]
		header.CycleIndices[3] = w["cycle3"]
		address := uint16(w["addressHigh"])<<8 | uint16(w["addressLow"])
		autoScroll, pace := w["autoScrollB"], w["presentationPace"]
		b := AlienBossContext{AutoScroll: &autoScroll, PresentationPace: &pace, CollisionAddress: &address}
		c := EnemyStepContext{Level: 7, Frame: w["frame"], FrameScroll: w["frameScroll"], GameLoop: w["loop"], WeaponStrength: w["weaponStrength"], AttackEnabled: w["soldierAllowed"] != 0, Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}, Scene: EnemySceneContext{Header: &header}}
		c.OnBossControl = func(control BossControl) error {
			switch control.Kind {
			case BossAudioReset:
				resets++
			case BossBeginDefeat:
				w["bossDefeated"] = 1
				w["delayLow"] = 255
			case BossExitReady:
				w["delayLow"] = control.Delay
				w["delayHigh"] = 0
			default:
				t.Fatal("unknown Boss control")
			}
			return nil
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("case %d extra request %+v", r.ID, req)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("case %d request %+v/%d want %+v", r.ID, req, q.Used, want)
			}
			v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
		}
		if before[r.Actor].Kind == 16 {
			err = s.StepAlienGuardian(r.Actor, c, b)
		} else {
			err = s.StepAlienHeart(r.Actor, c, b)
		}
		if err != nil {
			t.Fatalf("case %d kind %d routine %d: %v", r.ID, before[r.Actor].Kind, before[r.Actor].Routine, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("case %d slot %d\ngot %+v\nwant %+v\nbefore %+v", r.ID, i, e, want[i], before[i])
				}
			}
		}
		bias := w["generationBias"]
		if s.GenerationBiasChanged {
			bias = s.GenerationBiasValue
		}
		if bias != after["generationBias"] || address != uint16(after["addressHigh"])<<8|uint16(after["addressLow"]) {
			t.Fatalf("case %d address/bias got %04x/%02x want %02x%02x/%02x", r.ID, address, bias, after["addressHigh"], after["addressLow"], after["generationBias"])
		}
		if autoScroll != after["autoScrollB"] || pace != after["presentationPace"] || header.CycleIndices[1] != after["cycle1"] || header.CycleIndices[3] != after["cycle3"] || w["bossDefeated"] != after["bossDefeated"] || w["delayLow"] != after["delayLow"] || w["delayHigh"] != after["delayHigh"] || resets != r.AudioResets {
			t.Fatalf("case %d shared Boss state differs", r.ID)
		}
		if !reflect.DeepEqual(append([]uint8{}, s.PendingSounds...), append([]uint8{}, r.Sounds...)) {
			t.Fatalf("case %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
		}
		encoded, _ := hex.DecodeString(r.Before.Graphics.Bytes)
		at := int(r.Before.Graphics.Used)
		for _, packet := range q.Packets {
			raw, e := packet.Encode()
			if e != nil {
				t.Fatal(e)
			}
			copy(encoded[at:], raw)
			at += len(raw)
		}
		expected, _ := hex.DecodeString(r.After.Graphics.Bytes)
		expectedMap, _ := hex.DecodeString(r.After.Collision)
		if q.Used != r.After.Graphics.Used || requests != len(r.Requests) || !bytes.Equal(encoded, expected) || !bytes.Equal(terrain[:], expectedMap) {
			t.Fatalf("case %d graphics/collision differ", r.ID)
		}
	}
	t.Log("617 original Guardian/Heart dispatches matched")
}
