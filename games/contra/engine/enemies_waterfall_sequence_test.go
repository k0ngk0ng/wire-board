package contra

// Continuous integration acceptance: maintain the independently executed whole
// pool across 2,400 original E54F phases. Only the declared initial state and
// three external destruction inputs are applied; native after states are never
// fed back into the independent system. Original player collision is disabled
// by the fixture's spawn protection and empty projectile pool.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestWaterfallContinuousOriginalBoss(t *testing.T) {
	type state struct {
		Enemies, Players                [][]uint8
		World                           []uint8
		Collision, Graphics, Supertiles string
	}
	var data struct {
		Original bool     `json:"source_is_original_execution"`
		Fields   [][2]any `json:"world_fields"`
		Records  []struct {
			ID            int
			Before, After state
			Sounds        []uint8
			AudioResets   int
			Slots         []struct {
				Slot          int
				Kind, Routine uint8
			}
			Requests []struct{ X, Y, Code, Used uint8 }
		}
	}
	f, err := os.Open("testdata/original-enemies-waterfall-sequence.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original || len(data.Records) != 2400 {
		t.Fatal("continuous source coverage")
	}
	world := func(values []uint8) map[string]uint8 {
		r := map[string]uint8{}
		for i, field := range data.Fields {
			r[field[0].(string)] = values[i]
		}
		return r
	}
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
	s := EnemySystem{Slots: enemyPoolFromOracle(data.Records[0].Before.Enemies)}
	initial := world(data.Records[0].Before.World)
	arms, defeated, low, high, bias := initial["destroyedArms"], initial["bossDefeated"], initial["delayLow"], initial["delayHigh"], initial["generationBias"]
	count, rejected, resetsTotal := 0, 0, 0
	for _, r := range data.Records {
		if r.ID == 1000 || r.ID == 1600 {
			root := 15
			if r.ID == 1600 {
				root = 14
			}
			hand := int(uint8(s.Slots[root].VelocityX))
			if hand >= 16 {
				t.Fatal("invalid live hand")
			}
			s.Slots[hand].Routine = 5
		}
		if r.ID == 2000 {
			s.Slots[13].Routine = 6
		}
		w := world(r.Before.World)
		after := world(r.After.World)
		if s.Slots != enemyPoolFromOracle(r.Before.Enemies) || arms != w["destroyedArms"] || defeated != w["bossDefeated"] || low != w["delayLow"] || high != w["delayHigh"] || bias != w["generationBias"] {
			t.Fatalf("tick %d independent state drifted before original phase", r.ID)
		}
		terrainBytes, _ := hex.DecodeString(r.Before.Collision)
		terrain := CollisionWindow(terrainBytes)
		mappedBytes, _ := hex.DecodeString(r.Before.Supertiles)
		mapped := [128]uint8(mappedBytes)
		q := GraphicsQueue{Used: w["queueUsed"]}
		requests, called, resets := 0, 0, 0
		c := EnemyStepContext{Level: 2, Frame: w["frame"], Vertical: true, FrameScroll: w["frameScroll"], Random: w["random"], AttackEnabled: w["soldierAllowed"] != 0, Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["ppuControl"]}}
		for i, p := range r.Before.Players {
			c.Players[i] = EnemyTarget{State: p[0], X: p[1], Y: p[3], GameOver: p[41]}
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("tick %d extra request", r.ID)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("tick %d graphic request %+v/%d want %+v", r.ID, req, q.Used, want)
			}
			v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			if !v.Accepted {
				rejected++
			}
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
		}
		b := WaterfallBossContext{ScrollComplete: w["bossScrollComplete"], PaletteTimer: w["paletteTimer"], DestroyedArms: &arms, OnDefeated: func() error { defeated = 1; low = 255; resets++; return nil }, OnClearDelay: func(delay uint8) error { low = delay; high = 0; return nil }}
		s.PendingSounds = nil
		s.GenerationBiasChanged = false
		for slot := 15; slot >= 0; slot-- {
			e := s.Slots[slot]
			if e.Routine == 0 {
				continue
			}
			if called >= len(r.Slots) || r.Slots[called].Slot != slot || r.Slots[called].Kind != e.Kind || r.Slots[called].Routine != e.Routine {
				t.Fatalf("tick %d live dispatch %d differs from original", r.ID, slot)
			}
			called++
			count++
			switch e.Kind {
			case 1:
				err = s.StepBullet(slot, c)
			case 2:
				err = s.StepWeaponBox(slot, c)
			case 20:
				err = s.StepDragonMouth(slot, c, b)
			case 21:
				err = s.StepDragonArm(slot, c, b)
			default:
				t.Fatalf("unexpected sequence kind %d", e.Kind)
			}
			if err != nil {
				t.Fatalf("tick %d slot %d: %v", r.ID, slot, err)
			}
			if s.GenerationBiasChanged {
				bias = s.GenerationBiasValue
				s.GenerationBiasChanged = false
			}
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for slot, e := range s.Slots {
				if e != want[slot] {
					t.Fatalf("tick %d slot %d\ngot %+v\nwant %+v", r.ID, slot, e, want[slot])
				}
			}
		}
		if arms != after["destroyedArms"] || defeated != after["bossDefeated"] || low != after["delayLow"] || high != after["delayHigh"] || bias != after["generationBias"] || resets != r.AudioResets {
			t.Fatalf("tick %d shared state got arms%d boss%d delay%02x/%02x bias%02x reset%d want arms%d boss%d delay%02x/%02x bias%02x reset%d", r.ID, arms, defeated, low, high, bias, resets, after["destroyedArms"], after["bossDefeated"], after["delayLow"], after["delayHigh"], after["generationBias"], r.AudioResets)
		}
		resetsTotal += resets
		if !reflect.DeepEqual(append([]uint8{}, s.PendingSounds...), append([]uint8{}, r.Sounds...)) {
			t.Fatalf("tick %d sounds %v want %v", r.ID, s.PendingSounds, r.Sounds)
		}
		encoded, _ := hex.DecodeString(r.Before.Graphics)
		at := int(w["queueUsed"])
		for _, packet := range q.Packets {
			raw, e := packet.Encode()
			if e != nil {
				t.Fatal(e)
			}
			copy(encoded[at:], raw)
			at += len(raw)
		}
		expected, _ := hex.DecodeString(r.After.Graphics)
		expectedTerrain, _ := hex.DecodeString(r.After.Collision)
		if q.Used != after["queueUsed"] || called != len(r.Slots) || requests != len(r.Requests) || !bytes.Equal(encoded, expected) || !bytes.Equal(terrain[:], expectedTerrain) {
			t.Fatalf("tick %d graphics/terrain/calls differ", r.ID)
		}
	}
	if count != 19511 || rejected == 0 || resetsTotal != 1 || arms != 2 || defeated != 1 || low != 0x60 || high != 0 || s.Slots[13].Routine != 0 {
		t.Fatalf("incomplete original sequence: calls%d reject%d reset%d arms%d boss%d delay%d/%d", count, rejected, resetsTotal, arms, defeated, low, high)
	}
	t.Logf("2400 continuous original phases, %d live entity calls, %d graphics refusals matched", count, rejected)
}
