package contra

// Failure scenarios before implementation: angle quadrant/nibble boundaries;
// integer speed rounding and clamp drift; attack-enabled exception lost; wrong
// descending free-slot choice or mutation when pool full; new shots moving a
// frame early; sniper aim sampled after scroll, recoil counters or pose drift.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestOriginalEnemyShotAndAimBoundaries(t *testing.T) {
	path := "testdata/original-enemy-shooters-boundaries.json.gz"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != "48d18a1b921dc4c3cf4a3e9cb5660683195f55bb9c2666458d76955e45a6fa77" {
		t.Fatal("original controlled oracle identity changed")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct {
			Angle, Speed, X, Y, Enabled, SX, SY, TX, TY, Precision uint8
			Fill, Hole                                             int
		}
		Records []struct {
			Kind            string
			Case            int
			Success         bool
			Before, After   [][]uint8
			Value, Quadrant uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Cases) != 2786 {
		t.Fatal("original coverage changed")
	}
	counts := map[string]int{}
	for _, r := range oracle.Records {
		c := oracle.Cases[r.Case-1]
		counts[r.Kind]++
		switch r.Kind {
		case "quadrant":
			value, quadrant, err := EnemyAimQuadrant(c.SX, c.SY, c.TX, c.TY, c.Precision)
			if err != nil || value != r.Value || quadrant != r.Quadrant {
				t.Fatalf("original quadrant case%d: %d/%d,%v want%d/%d", r.Case, value, quadrant, err, r.Value, r.Quadrant)
			}
		case "angle":
			value, err := EnemyAimAngle(c.SX, c.SY, c.TX, c.TY, c.Precision)
			if err != nil || value != r.Value {
				t.Fatalf("original angle case%d: %d,%v want%d", r.Case, value, err, r.Value)
			}
		case "shot":
			s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
			created, err := s.CreateEnemyShot(EnemyStepContext{Level: 0, AttackEnabled: c.Enabled != 0}, c.Angle, c.Speed, c.X, c.Y)
			if err != nil || created != r.Success {
				t.Fatalf("original shot case%d: %t,%v want%t", r.Case, created, err, r.Success)
			}
			want := enemyPoolFromOracle(r.After)
			if s.Slots != want {
				for i, v := range s.Slots {
					if v != want[i] {
						t.Fatalf("original shot case%d slot%d got%+v want%+v", r.Case, i, v, want[i])
					}
				}
			}
		default:
			t.Fatal("unrecognized oracle kind", r.Kind)
		}
	}
	if counts["shot"] != 2786 || counts["angle"] != 2786 || counts["quadrant"] != 2786 {
		t.Fatal("missing observations", counts)
	}
}

func TestOriginalTurretStatesWithGraphicsQueue(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-turrets.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct {
			Kind, Routine, Vertical, Scroll, FrameCounter, Enabled, Strength, Loop, Random, Used, AliasX, AliasY uint8
			Players                                                                                              [2]EnemyTarget
		}
		Records []struct {
			Case              int
			Before, After     [][]uint8
			Sounds            []uint8
			Requests          []struct{ X, Y, Code, Used uint8 }
			Targets           []struct{ Index, X, Y uint8 }
			Supertiles, Queue string
			Used              uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 352 {
		t.Fatal("turret original coverage changed")
	}
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	aliases := 0
	for _, r := range oracle.Records {
		input := oracle.Cases[r.Case-1]
		rawMap, err := hex.DecodeString(r.Supertiles)
		if err != nil || len(rawMap) != 128 {
			t.Fatal("invalid supertile fixture")
		}
		mapped := ([128]uint8)(rawMap)
		c := EnemyStepContext{Level: 0, Frame: input.FrameCounter, FrameScroll: input.Scroll, Vertical: input.Vertical != 0, AttackEnabled: input.Enabled != 0, Players: input.Players, WeaponStrength: input.Strength, GameLoop: input.Loop, Random: input.Random, Scroll: CollisionScroll{Y: 224, NameTable: 0xb0}}
		q := GraphicsQueue{Used: input.Used}
		requests := 0
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("case%d unexpected request%+v", r.Case, req)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("case%d request got%+v used%d want%+v", r.Case, req, q.Used, want)
			}
			result, err := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, err
		}
		var weapons WeaponSystem
		weapons.Projectiles[2].Y = input.AliasX
		weapons.Projectiles[8].YFraction = input.AliasY
		c.ShooterTargetAt = func(index uint8) (EnemyTarget, error) {
			aliases++
			return JungleShooterTarget(index, c.Players, &weapons)
		}
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		switch input.Kind {
		case 4:
			err = s.StepTurret(15, c)
		case 7:
			err = s.StepRedTurret(15, c)
		case 16:
			err = s.StepBossCannon(15, c)
		default:
			t.Fatal("invalid type")
		}
		if err != nil {
			t.Fatalf("original turret%d: %v", r.Case, err)
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("original turret%d kind%d phase%d slot%d got%+v want%+v before%+v", r.Case, input.Kind, input.Routine, i, v, want[i], enemyFromOracle(r.Before[i]))
				}
			}
		}
		if !slices.Equal(s.PendingSounds, r.Sounds) || requests != len(r.Requests) {
			t.Fatalf("original turret%d sounds/requests differ", r.Case)
		}
		queue := make([]uint8, 128)
		at := int(input.Used)
		for _, packet := range q.Packets {
			raw, err := packet.Encode()
			if err != nil {
				t.Fatal(err)
			}
			copy(queue[at:], raw)
			at += len(raw)
		}
		wantQueue, _ := hex.DecodeString(r.Queue)
		if q.Used != r.Used || !bytes.Equal(queue, wantQueue) {
			t.Fatalf("original turret%d queue differs used%d want%d", r.Case, q.Used, r.Used)
		}
	}
	if aliases == 0 {
		t.Fatal("red-turret non-player alias was not exercised")
	}
}

func TestOriginalControlledShooterStates(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-states.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct {
			Kind, Routine, SceneType, Vertical, Scroll, FrameCounter, Enabled uint8
			Players                                                           [2]EnemyTarget
		}
		Records []struct {
			Case          int
			Before, After [][]uint8
			Sounds        []uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 198 {
		t.Fatal("controlled shooter coverage changed")
	}
	for _, r := range oracle.Records {
		input := oracle.Cases[r.Case-1]
		c := EnemyStepContext{Level: 0, SceneType: input.SceneType, Frame: input.FrameCounter, FrameScroll: input.Scroll, Vertical: input.Vertical != 0, AttackEnabled: input.Enabled != 0, Players: input.Players, Scroll: CollisionScroll{Y: 224, NameTable: 0xb0}}
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		if input.Kind == 6 {
			err = s.StepSniper(15, c)
		} else {
			err = s.StepRunnerFire(15, c)
		}
		if err != nil {
			t.Fatalf("controlled shooter%d: %v", r.Case, err)
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("controlled shooter%d kind%d phase%d slot%d got%+v want%+v before%+v", r.Case, input.Kind, input.Routine, i, v, want[i], enemyFromOracle(r.Before[i]))
				}
			}
		}
		if !slices.Equal(s.PendingSounds, r.Sounds) {
			t.Fatalf("controlled shooter%d sounds got%v want%v", r.Case, s.PendingSounds, r.Sounds)
		}
	}
}

func TestOriginalSniperSteps(t *testing.T) {
	rows, _ := loadEnemySteps(t)
	count := 0
	for _, r := range rows {
		before := enemyPoolFromOracle(r.Before.Enemies)
		if before[r.Actor].Kind != 6 {
			continue
		}
		w := r.Before.World
		c := EnemyStepContext{Level: w[3], SceneType: w[7], Frame: w[0], FrameScroll: w[11], Random: w[5], AttackEnabled: w[22] != 0, Vertical: w[8] != 0, Scroll: CollisionScroll{Y: w[24], X: w[25], NameTable: w[26]}}
		for i, p := range r.Before.Players {
			c.Players[i] = EnemyTarget{State: p.State, X: p.X, Y: p.Y}
		}
		s := EnemySystem{Slots: before}
		if err := s.StepSniper(r.Actor, c); err != nil {
			t.Fatalf("original sniper%d: %v", r.ID, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("original sniper%d frame%d slot%d got%+v want%+v before%+v", r.ID, r.Frame, i, v, want[i], before[i])
				}
			}
		}
		count++
	}
	if count == 0 {
		t.Fatal("no natural sniper observations")
	}
	t.Logf("matched %d natural sniper calls", count)
}

func TestContinuousNaturalShooterStates(t *testing.T) {
	// Acceptance against continuous controller-only original play: this includes
	// queue contention from actual scrolling/enemies, not isolated accept mocks.
	f, err := os.Open("testdata/original-enemy-shooters-natural.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Records  []struct {
			Frame, Slot                         int
			Kind                                uint8
			Before, After                       [][]uint8
			Sounds                              []uint8
			Supertiles, QueueBefore, QueueAfter string
			UsedBefore, UsedAfter               uint8
			Requests                            []struct{ X, Y, Code, Used uint8 }
			World                               struct {
				Level, SceneType, Frame, Scroll, Vertical, Enabled, Strength, Loop, Random, ScrollX, ScrollY, Control, AliasX, AliasY uint8
				Players                                                                                                               [2]EnemyTarget
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 2621 {
		t.Fatal("continuous original coverage changed")
	}
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[uint8]int{}
	for _, r := range oracle.Records {
		w := r.World
		if w.Level != 0 || w.SceneType != 0 {
			t.Fatal("natural fixture moved outside first-stage scope")
		}
		mappedBytes, err := hex.DecodeString(r.Supertiles)
		if err != nil || len(mappedBytes) != 128 {
			t.Fatal("invalid natural map")
		}
		mapped := ([128]uint8)(mappedBytes)
		c := EnemyStepContext{Level: w.Level, SceneType: w.SceneType, Frame: w.Frame, FrameScroll: w.Scroll, Vertical: w.Vertical != 0, AttackEnabled: w.Enabled != 0, Players: w.Players, WeaponStrength: w.Strength, GameLoop: w.Loop, Random: w.Random, Scroll: CollisionScroll{X: w.ScrollX, Y: w.ScrollY, NameTable: w.Control}}
		q := GraphicsQueue{Used: r.UsedBefore}
		requests := 0
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("frame%d slot%d unexpected natural graphic%+v", r.Frame, r.Slot, req)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("frame%d natural request got%+v used%d want%+v", r.Frame, req, q.Used, want)
			}
			result, err := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, err
		}
		var weapons WeaponSystem
		weapons.Projectiles[2].Y = w.AliasX
		weapons.Projectiles[8].YFraction = w.AliasY
		c.ShooterTargetAt = func(index uint8) (EnemyTarget, error) { return JungleShooterTarget(index, c.Players, &weapons) }
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		switch r.Kind {
		case 4:
			err = s.StepTurret(r.Slot, c)
		case 6:
			err = s.StepSniper(r.Slot, c)
		case 7:
			err = s.StepRedTurret(r.Slot, c)
		case 16:
			err = s.StepBossCannon(r.Slot, c)
		default:
			t.Fatal("unexpected shooter kind")
		}
		if err != nil {
			t.Fatalf("natural frame%d kind%d: %v", r.Frame, r.Kind, err)
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("natural frame%d kind%d slot%d got%+v want%+v before%+v", r.Frame, r.Kind, i, v, want[i], enemyFromOracle(r.Before[i]))
				}
			}
		}
		if !slices.Equal(s.PendingSounds, r.Sounds) || requests != len(r.Requests) {
			t.Fatalf("natural frame%d kind%d sounds/requests differ", r.Frame, r.Kind)
		}
		queue, err := hex.DecodeString(r.QueueBefore)
		if err != nil {
			t.Fatal(err)
		}
		at := int(r.UsedBefore)
		for _, packet := range q.Packets {
			raw, err := packet.Encode()
			if err != nil {
				t.Fatal(err)
			}
			copy(queue[at:], raw)
			at += len(raw)
		}
		wantQueue, _ := hex.DecodeString(r.QueueAfter)
		if q.Used != r.UsedAfter || !bytes.Equal(queue, wantQueue) {
			t.Fatalf("natural frame%d queue differs used%d want%d", r.Frame, q.Used, r.UsedAfter)
		}
		counts[r.Kind]++
	}
	if counts[4] != 806 || counts[6] != 1051 || counts[7] != 764 {
		t.Fatal("natural kind counts differ", counts)
	}
	t.Logf("matched continuous original shooter calls: %v", counts)
}
