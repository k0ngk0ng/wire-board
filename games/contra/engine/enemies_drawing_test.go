package contra

// Before implementation: boxes can wait on a full graphics queue; opening HP
// changes only after accepted updates; destruction creates a separate explosion
// and transforms its own slot without resetting position/fractions. This uses
// real queue packets instead of guessing acceptance from the expected output.
import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestOriginalWeaponBoxes(t *testing.T) {
	rows, maps := loadEnemyFixture(t, "testdata/original-enemy-drawing.json.gz")
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 2 {
			continue
		}
		w := r.Before.World
		window := maps[r.Before.Map]
		rawMap, err := hex.DecodeString(r.Before.Graphics.Supertiles)
		if err != nil || len(rawMap) != 128 {
			t.Fatal("invalid supertiles", err)
		}
		supertiles := ([128]uint8)(rawMap)
		queue := GraphicsQueue{Used: r.Before.Graphics.Used}
		c := EnemyStepContext{Level: w[3], SceneType: w[7], Frame: w[0], Vertical: w[8] != 0, FrameScroll: w[11], Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			result, err := queue.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &supertiles, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, err
		}
		s := EnemySystem{Slots: before}
		if err := s.StepWeaponBox(r.Actor, c); err != nil {
			t.Fatalf("box original %d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("box original %d frame%d slot%d got %+v want %+v before %+v", r.ID, r.Frame, i, e, want[i], before[i])
				}
			}
		}
		if len(s.PendingSounds) != len(r.Sounds) {
			t.Fatalf("box sound %d got %v want %v", r.ID, s.PendingSounds, r.Sounds)
		}
		for i, sound := range s.PendingSounds {
			if sound != r.Sounds[i] {
				t.Fatal("box sound", sound, r.Sounds)
			}
		}
		beforeBytes, _ := hex.DecodeString(r.Before.Graphics.Bytes)
		afterBytes, _ := hex.DecodeString(r.After.Graphics.Bytes)
		actual := append([]uint8(nil), beforeBytes...)
		var appended []uint8
		for _, packet := range queue.Packets {
			part, err := packet.Encode()
			if err != nil {
				t.Fatal(err)
			}
			appended = append(appended, part...)
		}
		copy(actual[int(r.Before.Graphics.Used):], appended)
		// $0700..$07BF is the actual graphics buffer. The remainder contains
		// palette/score/bank bookkeeping; EB1B's bank restore changes $07EC,
		// which an independent game intentionally does not emulate.
		if queue.Used != r.After.Graphics.Used || !bytes.Equal(actual[:192], afterBytes[:192]) || r.Before.Graphics.Mode != r.After.Graphics.Mode {
			t.Fatalf("box graphics original %d used%d want%d", r.ID, queue.Used, r.After.Graphics.Used)
		}
		count++
	}
	if count != 868 {
		t.Fatalf("box natural coverage %d want868", count)
	}
}

func TestOriginalWeaponPickups(t *testing.T) {
	// Original M-item creation and flight after a real gun-box kill. Keep the
	// hidden Y byte/fraction, landing snap, horizontal bounce and sprite timing;
	// collection is a later collision phase and must not be inferred here.
	rows, maps := loadEnemyFixture(t, "testdata/original-enemy-drawing.json.gz")
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 0 {
			continue
		}
		w := r.Before.World
		window := maps[r.Before.Map]
		s := EnemySystem{Slots: before}
		if err := s.StepPickup(r.Actor, EnemyStepContext{Level: w[3], SceneType: w[7], Vertical: w[8] != 0, FrameScroll: w[11], Frame: w[0], Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}); err != nil {
			t.Fatal(err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("pickup original %d frame%d slot%d got%+v want%+v before%+v", r.ID, r.Frame, i, e, want[i], before[i])
				}
			}
		}
		count++
	}
	if count != 121 {
		t.Fatalf("natural pickup coverage %d want121", count)
	}
}

func TestOriginalExplodingBridges(t *testing.T) {
	// Trigger proximity, admitted halves, collision deletion, three secondary
	// explosions, own explosion and progression to the next segment retain the
	// original ordering. Real queue admission is part of this comparison.
	rows, maps := loadEnemyFixture(t, "testdata/original-enemy-drawing.json.gz")
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 18 {
			continue
		}
		w := r.Before.World
		window := maps[r.Before.Map]
		raw, _ := hex.DecodeString(r.Before.Graphics.Supertiles)
		supertiles := ([128]uint8)(raw)
		queue := GraphicsQueue{Used: r.Before.Graphics.Used}
		c := EnemyStepContext{Level: w[3], SceneType: w[7], Frame: w[0], Vertical: w[8] != 0, FrameScroll: w[11], Terrain: &window, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}
		for i, p := range r.Before.Players {
			c.Players[i] = EnemyTarget{State: p.State, X: p.X, Y: p.Y}
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			v, err := queue.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &supertiles, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, err
		}
		s := EnemySystem{Slots: before}
		if err := s.StepBridge(r.Actor, c); err != nil {
			t.Fatalf("bridge original %d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("bridge original %d frame%d slot%d got%+v want%+v before%+v", r.ID, r.Frame, i, e, want[i], before[i])
				}
			}
		}
		if window != maps[r.After.Map] {
			t.Fatalf("bridge original %d collision differs", r.ID)
		}
		if len(s.PendingSounds) != len(r.Sounds) {
			t.Fatalf("bridge original %d sounds got%v want%v", r.ID, s.PendingSounds, r.Sounds)
		}
		for i, v := range s.PendingSounds {
			if v != r.Sounds[i] {
				t.Fatal("bridge sound", v, r.Sounds)
			}
		}
		old, _ := hex.DecodeString(r.Before.Graphics.Bytes)
		after, _ := hex.DecodeString(r.After.Graphics.Bytes)
		actual := append([]uint8(nil), old...)
		var appended []uint8
		for _, p := range queue.Packets {
			data, err := p.Encode()
			if err != nil {
				t.Fatal(err)
			}
			appended = append(appended, data...)
		}
		copy(actual[int(r.Before.Graphics.Used):], appended)
		if queue.Used != r.After.Graphics.Used || !bytes.Equal(actual[:192], after[:192]) {
			t.Fatalf("bridge original %d queue differs", r.ID)
		}
		count++
	}
	if count != 161 {
		t.Fatalf("bridge natural coverage%d want161", count)
	}
}
