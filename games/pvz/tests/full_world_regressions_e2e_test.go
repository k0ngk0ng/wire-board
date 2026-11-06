package pvz_test

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	"testing"
)

func fullWorldReject(t *testing.T, s *engine.State, kind string, row, col int) {
	t.Helper()
	s.Players[0].Deck = []string{kind}
	s.Players[0].Cooldowns[kind] = 0
	before := engine.Digest(*s)
	if e := engine.Apply(s, 0, engine.Action{Type: "plant", Kind: kind, Row: row, Col: col, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1}); e == nil || engine.Digest(*s) != before {
		t.Fatalf("illegal %s placement was accepted or changed state", kind)
	}
}
func TestPvZFullWorldOriginalTerrainLayersE2E(t *testing.T) {
	for _, kind := range []string{"potatomine", "seashroom", "tanglekelp"} {
		t.Run("water-support-"+kind, func(t *testing.T) {
			s := fullWorldControlled(t, "pool")
			fullWorldPlace(t, &s, "lilypad", 2, 0)
			fullWorldReject(t, &s, kind, 2, 0)
		})
	}
	for _, kind := range []string{"spikeweed", "spikerock"} {
		t.Run("ground-support-"+kind, func(t *testing.T) {
			s := fullWorldControlled(t, "day")
			fullWorldPlace(t, &s, "flowerpot", 0, 0)
			if kind == "spikerock" {
				s.Plants = append(s.Plants, engine.Plant{ID: s.NextID + 1, Owner: 0, Kind: "spikeweed", Row: 0, Col: 0, HP: 300, MaxHP: 300, Layer: "body", Flags: map[string]int64{}})
				s.NextID++
			}
			fullWorldReject(t, &s, kind, 0, 0)
		})
	}
	t.Run("cattail-pumpkin", func(t *testing.T) {
		s := fullWorldControlled(t, "pool")
		fullWorldPlace(t, &s, "lilypad", 2, 0)
		fullWorldPlace(t, &s, "cattail", 2, 0)
		fullWorldPlace(t, &s, "pumpkin", 2, 0)
		if e := engine.Validate(s); e != nil {
			t.Fatal(e)
		}
	})
	for _, q := range []struct {
		scene, base string
		row         int
	}{{"pool", "lilypad", 2}, {"roof", "flowerpot", 0}} {
		t.Run("pending-base-"+q.base, func(t *testing.T) {
			s := fullWorldControlled(t, q.scene)
			s.Players[0].ImitaterKind = q.base
			fullWorldPlace(t, &s, "imitater", q.row, 0)
			fullWorldReject(t, &s, "peashooter", q.row, 0)
		})
	}
	t.Run("pot-under-shell", func(t *testing.T) {
		s := fullWorldControlled(t, "day")
		fullWorldPlace(t, &s, "pumpkin", 0, 0)
		fullWorldReject(t, &s, "flowerpot", 0, 0)
	})
	t.Run("cannon-mismatched-pots", func(t *testing.T) {
		s := fullWorldControlled(t, "day")
		fullWorldPlace(t, &s, "flowerpot", 0, 0)
		fullWorldPlace(t, &s, "kernelpult", 0, 0)
		fullWorldPlace(t, &s, "kernelpult", 0, 1)
		fullWorldReject(t, &s, "cobcannon", 0, 0)
	})
}
func TestPvZFullWorldCannonRightSupportAndImitaterHPE2E(t *testing.T) {
	t.Run("right-support-removes-whole-cannon", func(t *testing.T) {
		s := fullWorldControlled(t, "roof")
		fullWorldPlace(t, &s, "flowerpot", 0, 0)
		fullWorldPlace(t, &s, "flowerpot", 0, 1)
		right := s.Plants[1].ID
		fullWorldPlace(t, &s, "kernelpult", 0, 0)
		fullWorldPlace(t, &s, "kernelpult", 0, 1)
		fullWorldPlace(t, &s, "cobcannon", 0, 0)
		if e := engine.Apply(&s, 0, engine.Action{Type: "shovel", PlantID: right, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1}); e != nil {
			t.Fatal(e)
		}
		for _, p := range s.Plants {
			if p.Kind == "cobcannon" {
				t.Fatal("cannon survived unsupported right half")
			}
		}
		if e := engine.Validate(s); e != nil {
			t.Fatal(e)
		}
		fullWorldArtifact(t, s)
	})
	t.Run("imitater-has-own-300-hp-until-morph", func(t *testing.T) {
		s := fullWorldControlled(t, "day")
		s.Players[0].ImitaterKind = "tallnut"
		fullWorldPlace(t, &s, "imitater", 0, 0)
		if s.Plants[0].HP != 300 || s.Plants[0].MaxHP != 300 {
			t.Fatal("unmorphed imitater inherited target durability")
		}
		if e := engine.Step(&s, 40); e != nil {
			t.Fatal(e)
		}
		if s.Plants[0].State != "imitater-explode" {
			t.Fatal("imitater did not enter its morph clip after two seconds")
		}
		if e := engine.Step(&s, 24); e != nil {
			t.Fatal(e)
		}
		if s.Plants[0].Kind != "tallnut" || s.Plants[0].MaxHP != 8000 {
			t.Fatal("morph did not initialize target durability")
		}
		fullWorldArtifact(t, s)
	})
}
func TestPvZFullWorldFogTargetSidecarsE2E(t *testing.T) {
	s := fullWorldControlled(t, "fog")
	fullWorldPlace(t, &s, "magnetshroom", 0, 0)
	s.Players[1].Deck = []string{"zombie"}
	s.Players[1].Money = 10000
	if e := engine.Step(&s, 400); e != nil {
		t.Fatal(e)
	}
	if e := engine.Apply(&s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0, Epoch: s.Epoch, Seq: 1}); e != nil {
		t.Fatal(e)
	}
	zid := s.Zombies[0].ID
	s.Zombies[0].X = 7500
	s.Plants[0].TargetID = zid
	s.Plants[0].Flags["targetX"] = 7500
	s.NextID++
	s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 0, Row: 0, X: 1500, Y: 500, Kind: "cabbage", Damage: 40, TargetID: zid, TargetX: 7500, TargetY: 500, LandAt: s.Tick + 24, Payload: "lob", BornAt: s.Tick})
	s.EventSerial++
	s.Events = append(s.Events, engine.Event{ID: s.EventSerial, Type: "magnet", Owner: 0, Row: 0, X: 7500, Target: zid, Tick: s.Tick})
	s.EventSerial++
	s.Events = append(s.Events, engine.Event{ID: s.EventSerial, Type: "hit", Owner: 0, Row: 0, X: 7500, Target: zid, Tick: s.Tick})
	p := engine.ProjectState(s, 0)
	if len(p.Zombies) != 0 {
		t.Fatal("hidden enemy survived projection")
	}
	if p.Plants[0].TargetID != 0 || p.Plants[0].Flags["targetX"] != 0 {
		t.Fatal("plant target sidecar leaked fog coordinates")
	}
	if p.Projectiles[0].TargetID != 0 || p.Projectiles[0].TargetX != 0 || p.Projectiles[0].TargetY != 0 {
		t.Fatal("projectile target sidecar leaked fog coordinates")
	}
	for _, e := range p.Events {
		if e.Type == "magnet" || e.Type == "hit" {
			t.Fatal("owned target event leaked hidden enemy")
		}
	}
	raw, _ := json.Marshal(p)
	fullWorldArtifact(t, map[string]any{"publicProjection": json.RawMessage(raw)})
}

// Controlled mechanism fixtures use explicit money/cooldown setup; natural
// complete-match resource acceptance remains in full_bots and HTTP matrices.
func fullWorldControlled(t *testing.T, scene string) engine.State {
	t.Helper()
	s, e := engine.New(engine.Options{Scene: scene}, 2, "full-world-regressions")
	if e != nil {
		t.Fatal(e)
	}
	s.Players[0].Money = 100000
	return s
}
func fullWorldPlace(t *testing.T, s *engine.State, kind string, row, col int) {
	t.Helper()
	s.Players[0].Deck = []string{kind}
	s.Players[0].Cooldowns[kind] = 0
	a := engine.Action{Type: "plant", Kind: kind, Row: row, Col: col, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1}
	if e := engine.Apply(s, 0, a); e != nil {
		t.Fatal(kind, e)
	}
}
func TestPvZFullWorldUpgradeLayersRegressionE2E(t *testing.T) {
	t.Run("cattail-does-not-overwrite-an-existing-body", func(t *testing.T) {
		s := fullWorldControlled(t, "pool")
		fullWorldPlace(t, &s, "lilypad", 2, 0)
		fullWorldPlace(t, &s, "peashooter", 2, 0)
		s.Players[0].Deck = []string{"cattail"}
		before := engine.Digest(s)
		e := engine.Apply(&s, 0, engine.Action{Type: "plant", Kind: "cattail", Row: 2, Col: 0, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1})
		if e == nil || engine.Digest(s) != before {
			t.Fatal("cattail upgrade bypassed occupied body or changed rejected purchase")
		}
		fullWorldArtifact(t, s)
	})
	t.Run("cob-does-not-keep-a-pumpkin", func(t *testing.T) {
		s := fullWorldControlled(t, "day")
		fullWorldPlace(t, &s, "kernelpult", 0, 0)
		fullWorldPlace(t, &s, "kernelpult", 0, 1)
		fullWorldPlace(t, &s, "pumpkin", 0, 0)
		s.Players[0].Deck = []string{"cobcannon"}
		before := engine.Digest(s)
		e := engine.Apply(&s, 0, engine.Action{Type: "plant", Kind: "cobcannon", Row: 0, Col: 0, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1})
		if e == nil || engine.Digest(s) != before {
			t.Fatal("cob upgrade retained forbidden shell or changed rejected purchase")
		}
		fullWorldArtifact(t, s)
	})
}
func TestPvZFullWorldImitaterCoffeeDelayE2E(t *testing.T) {
	s := fullWorldControlled(t, "day")
	fullWorldPlace(t, &s, "puffshroom", 0, 0)
	if !s.Plants[0].Asleep {
		t.Fatal("day mushroom did not sleep")
	}
	s.Players[0].Deck = []string{"imitater"}
	s.Players[0].ImitaterKind = "coffeebean"
	if e := engine.Apply(&s, 0, engine.Action{Type: "plant", Kind: "imitater", Row: 0, Col: 0, ImitateKind: "coffeebean", Epoch: s.Epoch, Seq: s.Players[0].Seq + 1}); e != nil {
		t.Fatal(e)
	}
	if e := engine.Step(&s, 63); e != nil {
		t.Fatal(e)
	}
	if !s.Plants[0].Asleep {
		t.Fatal("imitated coffee bypassed transformation delay")
	}
	if e := engine.Step(&s, 21); e != nil {
		t.Fatal(e)
	}
	if s.Plants[0].Asleep {
		t.Fatal("imitated coffee never woke target")
	}
	for _, p := range s.Plants {
		if p.Kind == "imitater" || p.Kind == "coffeebean" {
			t.Fatal("instant coffee remained a permanent layer")
		}
	}
	if e := engine.Validate(s); e != nil {
		t.Fatal(e)
	}
	fullWorldArtifact(t, s)
}
