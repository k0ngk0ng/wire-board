package bubbleparty

// Failure scenarios are inventoried before implementation in
// docs/research/bubbleparty/mounts-original-evidence.md. In particular: riding
// must not overwrite walking upgrades, invalidate restored saves, or absorb
// more than one independent water hit.
import (
	"encoding/json"
	"testing"
)

func TestMountSpeedDoesNotOverwriteWalkingUpgrade(t *testing.T) {
	for _, tc := range []struct {
		kind  string
		speed float64
	}{{"turtle", 600}, {"turtle-fast", 1200.0 / 9}, {"owl", 240}, {"airship", 120}, {"airship-broken", 600}, {"octopus", 600}, {"octopus-fast", 1200.0 / 9}, {"railcar", 600}} {
		p := Player{Alive: true, SpeedMS: 120, Mount: tc.kind}
		if got := effectiveMoveMS(&p); got != tc.speed {
			t.Fatalf("%s speed=%g want=%g", tc.kind, got, tc.speed)
		}
		if p.SpeedMS != 120 {
			t.Fatal("mount overwrote walking speed")
		}
		p.Mount = ""
		if effectiveMoveMS(&p) != 120 {
			t.Fatal("dismount lost walking upgrade")
		}
	}
}

func TestMountPickupAndWaterLoss(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 41)
	beginPlaying(t, s)
	p := &s.Players[0]
	p.SpeedMS = 120
	s.Items = []Item{{ID: 100, X: p.X, Y: p.Y, Type: "turtle"}}
	s.pickupItems(s.ServerAt)
	if p.Mount != "turtle" || len(s.Items) != 0 {
		t.Fatalf("mount pickup failed: %q", p.Mount)
	}
	s.Items = []Item{{ID: 101, X: p.X, Y: p.Y, Type: "owl"}}
	s.pickupItems(s.ServerAt)
	if p.Mount != "turtle" || len(s.Items) != 1 {
		t.Fatal("rider consumed a second mount")
	}
	s.hitWater(p, s.ServerAt+1, 1)
	if p.Mount != "" || p.TrappedUntil != 0 || p.SpeedMS != 120 {
		t.Fatal("mount did not absorb one hit and preserve upgrade")
	}
	s.hitWater(p, s.ServerAt+2, 1)
	if p.TrappedUntil != 0 {
		t.Fatal("same water immediately trapped dismounted rider")
	}
	s.hitWater(p, p.InvulnerableUntil+1, 1)
	if p.TrappedUntil == 0 {
		t.Fatal("dismount made rider permanently invulnerable")
	}
}

func TestMountRestoreAndValidation(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 42)
	s.Players[0].Mount = "owl"
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Players[0].Mount != "owl" || effectiveMoveMS(&restored.Players[0]) != 240 {
		t.Fatal("restore lost mount")
	}
	if !validMount("") || !validMount("turtle") || validMount("fake") || !validMount("airship") {
		t.Fatal("unverified mount accepted")
	}
	if mountKind("speed") != "" || mountKind("turtle-fast") != "turtle-fast" {
		t.Fatal("wrong item classification")
	}
}

func TestMountCannotProtectEliminatedOrTrappedPlayer(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 43)
	p := &s.Players[0]
	p.Mount = "owl"
	p.Alive = false
	if s.absorbMountHit(p, s.ServerAt, 1) {
		t.Fatal("dead player consumed mount")
	}
	p.Alive = true
	p.TrappedUntil = s.ServerAt + 1000
	if s.absorbMountHit(p, s.ServerAt, 1) {
		t.Fatal("trapped player consumed mount")
	}
	if s.absorbMountHit(nil, s.ServerAt, 1) {
		t.Fatal("nil player accepted")
	}
}

func TestMountNativeSpeedAndAbilities(t *testing.T) {
	p := Player{SpeedMS: 150, NativeSpeed: 7}
	if effectiveMoveMS(&p) != 1200.0/7 {
		t.Fatal("native walking speed rounded or ignored")
	}
	for _, kind := range []string{"airship", "airship-broken"} {
		if !mountPassesBoxes(kind) || mountCanPickup(kind) {
			t.Fatal("airship abilities differ from ItemAttr effects5,9")
		}
	}
	for _, kind := range []string{"", "turtle", "turtle-fast", "owl", "octopus", "octopus-fast", "railcar"} {
		if mountPassesBoxes(kind) || !mountCanPickup(kind) {
			t.Fatal("ground mount received flying abilities")
		}
	}
}
