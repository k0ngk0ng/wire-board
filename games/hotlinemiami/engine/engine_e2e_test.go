package engine_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

func arena() engine.Map {
	u := engine.Unit
	return engine.Map{ID: "fixture", Name: "测试地图", Width: 640 * u, Height: 480 * u,
		Walls:  []engine.Rect{{X: 0, Y: 0, W: 640 * u, H: 8 * u}, {X: 0, Y: 472 * u, W: 640 * u, H: 8 * u}, {X: 0, Y: 0, W: 8 * u, H: 480 * u}, {X: 632 * u, Y: 0, W: 8 * u, H: 480 * u}},
		Spawns: []engine.Point{{X: 80 * u, Y: 80 * u}, {X: 560 * u, Y: 400 * u}, {X: 560 * u, Y: 80 * u}, {X: 80 * u, Y: 400 * u}},
	}
}

func match(t *testing.T, count int) *engine.State {
	t.Helper()
	s, err := engine.NewWithMap(engine.Options{Map: "fixture", RoundSeconds: 180}, count, 19, arena())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func input(t *testing.T, s *engine.State, seat int, in engine.Input) {
	t.Helper()
	in.Seq = s.Players[seat].ReceivedSeq + 1
	in.Life = s.Players[seat].Life
	if err := engine.QueueInput(s, seat, in); err != nil {
		t.Fatal(err)
	}
}

func TestInputOrderAndShortEdges(t *testing.T) {
	s := match(t, 2)
	p := &s.Players[0]
	p.ProtectedUntil = 0
	before := p.Ammo
	input(t, s, 0, engine.Input{Fire: true})
	input(t, s, 0, engine.Input{})
	engine.Step(s)
	if s.Players[0].Ammo != before-1 || s.Players[0].AppliedSeq != 2 || len(s.Bullets) != 1 {
		t.Fatalf("short edge lost: %+v bullets=%d", s.Players[0], len(s.Bullets))
	}
	old := engine.Input{Seq: 1, Life: p.Life, MoveX: 1000}
	if engine.QueueInput(s, 0, old) == nil {
		t.Fatal("accepted stale sequence")
	}
	old.Seq = 1000000
	if engine.QueueInput(s, 0, old) == nil {
		t.Fatal("accepted unbounded sequence jump")
	}
	released := match(t, 2)
	input(t, released, 0, engine.Input{Fire: true})
	engine.Advance(released, 15)
	ammo := released.Players[0].Ammo
	input(t, released, 0, engine.Input{})
	engine.Step(released)
	if released.Players[0].Ammo != ammo {
		t.Fatal("release generated a shot from stale held input")
	}
	bounded := match(t, 2)
	bounded.Players[0].ReceivedSeq = engine.MaxInputSeq - 1
	if err := engine.QueueInput(bounded, 0, engine.Input{Seq: engine.MaxInputSeq, Life: bounded.Players[0].Life}); err != nil {
		t.Fatal(err)
	}
	if engine.QueueInput(bounded, 0, engine.Input{Seq: engine.MaxInputSeq + 1, Life: bounded.Players[0].Life}) == nil {
		t.Fatal("accepted input beyond maximum sequence")
	}
}

func TestComputerUsesSameVisibleCamera(t *testing.T) {
	u := engine.Unit
	setup := func(t *testing.T) *engine.State {
		world := engine.Map{ID: "camera-fixture", Name: "镜头验证", Width: 1600 * u, Height: 1200 * u, Spawns: []engine.Point{{X: 800 * u, Y: 600 * u}, {X: 1000 * u, Y: 600 * u}, {X: 800 * u, Y: 800 * u}, {X: 1000 * u, Y: 800 * u}}}
		s, err := engine.NewWithMap(engine.Options{Map: world.ID}, 2, 19, world)
		if err != nil {
			t.Fatal(err)
		}
		s.Players[0].X, s.Players[0].Y = 800*u, 600*u
		s.Players[0].ProtectedUntil, s.Players[1].ProtectedUntil = 0, 0
		return s
	}
	cases := []struct {
		Name string
		X    int
		Y    int
		Want bool
	}{
		{"right-inside", 1088*u - 1, 600 * u, true}, {"right-edge", 1088 * u, 600 * u, true}, {"right-outside", 1088*u + 1, 600 * u, false},
		{"left-outside", 512*u - 1, 600 * u, false}, {"left-edge", 512 * u, 600 * u, true}, {"left-inside", 512*u + 1, 600 * u, true},
		{"bottom-inside", 800 * u, 780*u - 1, true}, {"bottom-edge", 800 * u, 780 * u, true}, {"bottom-outside", 800 * u, 780*u + 1, false},
		{"top-outside", 800 * u, 420*u - 1, false}, {"top-edge", 800 * u, 420 * u, true}, {"top-inside", 800 * u, 420*u + 1, true},
	}
	for _, scenario := range cases {
		t.Run(scenario.Name, func(t *testing.T) {
			s := setup(t)
			s.Players[1].X, s.Players[1].Y = scenario.X, scenario.Y
			if got := engine.BotInput(s, 0).Fire; got != scenario.Want {
				t.Fatalf("shared 576x360 viewport target=%d,%d fire=%v want=%v", scenario.X, scenario.Y, got, scenario.Want)
			}
		})
	}
	s := setup(t)
	s.Players[0].X, s.Players[0].Y = 80*u, 80*u
	s.Players[1].X, s.Players[1].Y = 560*u, 80*u
	if !engine.BotInput(s, 0).Fire {
		t.Fatal("expanded camera did not clamp to left map edge")
	}
	s.Players[0].X, s.Players[0].Y = 9*u, 9*u
	s.Players[1].X, s.Players[1].Y = 576*u, 360*u
	if !engine.BotInput(s, 0).Fire {
		t.Fatal("an obsolete distance cap hid a target inside the clamped viewport and pistol range")
	}
	s.Players[0].X, s.Players[0].Y = 1520*u, 1120*u
	s.Players[1].X, s.Players[1].Y = 1024*u-1, 1120*u
	if engine.BotInput(s, 0).Fire {
		t.Fatal("camera included the integer beyond its right-edge clamp")
	}
	s.Players[1].X = 1024 * u
	if !engine.BotInput(s, 0).Fire {
		t.Fatal("camera excluded its right-edge clamp boundary")
	}
	s.Players[1].X, s.Players[1].Y = 1520*u, 840*u-1
	if engine.BotInput(s, 0).Fire {
		t.Fatal("camera included the integer beyond its bottom-edge clamp")
	}
	s.Players[1].Y = 840 * u
	if !engine.BotInput(s, 0).Fire {
		t.Fatal("camera excluded its bottom-edge clamp boundary")
	}
	blocked := setup(t)
	blocked.Players[1].X, blocked.Players[1].Y = 950*u, 600*u
	blocked.World.Walls = []engine.Rect{{X: 900 * u, Y: 550 * u, W: 8 * u, H: 100 * u}}
	if engine.BotInput(blocked, 0).Fire {
		t.Fatal("expanded view bypassed wall line of sight")
	}
	blocked.World.Walls = nil
	blocked.World.Doors = []engine.DoorDef{{ID: 10, X: 900 * u, Y: 550 * u, W: 8 * u, H: 100 * u}}
	blocked.Doors = []engine.Door{{ID: 10}}
	if engine.BotInput(blocked, 0).Fire {
		t.Fatal("expanded view bypassed a closed door")
	}
	blocked.Doors[0].Open = true
	if !engine.BotInput(blocked, 0).Fire {
		t.Fatal("open-door visible target was ignored")
	}
}

func TestSharedViewportPixelConstants(t *testing.T) {
	if engine.ViewportWidthPixels != 576 || engine.ViewportHeightPixels != 360 || engine.ViewportWidthPixels*5 != engine.ViewportHeightPixels*8 {
		t.Fatal("shared viewport must be 576x360 raw integer pixels at 8:5")
	}
	if engine.Unit != 256 || engine.FPS != 60 || engine.MoveSpeed != 3*engine.Unit || engine.Radius != 9*engine.Unit {
		t.Fatal("viewport metadata changed physical or time units")
	}
}

func TestMovementCollisionAndDiagonal(t *testing.T) {
	s := match(t, 2)
	p := s.Players[0]
	a := engine.PredictPlayer(arena(), p, engine.Input{MoveX: 1000}, 9)
	b := engine.PredictPlayer(arena(), p, engine.Input{MoveX: 1000, MoveY: 1000}, 9)
	ax := int64(a.X - p.X)
	bx, by := int64(b.X-p.X), int64(b.Y-p.Y)
	if bx*bx+by*by > ax*ax || ax <= 0 {
		t.Fatalf("diagonal acceleration: straight=%d diagonal=%d,%d", ax, bx, by)
	}
	input(t, s, 0, engine.Input{MoveX: -1000})
	engine.Advance(s, 300)
	if s.Players[0].X < 8*engine.Unit+engine.Radius {
		t.Fatal("walked through wall")
	}
	engine.ClearInput(s, 0)
	x := s.Players[0].X
	engine.Advance(s, s.Frame+5)
	if s.Players[0].X != x {
		t.Fatal("clear did not release")
	}
}

func TestShotsWallsDeathAndRespawn(t *testing.T) {
	s := match(t, 2)
	u := engine.Unit
	s.Players[0].X, s.Players[0].Y = 80*u, 200*u
	s.Players[1].X, s.Players[1].Y = 180*u, 200*u
	s.Players[0].ProtectedUntil, s.Players[1].ProtectedUntil = 0, 0
	input(t, s, 0, engine.Input{Fire: true})
	input(t, s, 0, engine.Input{})
	engine.Advance(s, 12)
	if s.Players[1].Alive || s.Players[0].Score != 1 || s.Players[1].Deaths != 1 {
		t.Fatalf("shot not settled: %+v", s.Players)
	}
	life := s.Players[1].Life
	engine.Advance(s, s.Players[1].RespawnFrame)
	if !s.Players[1].Alive || s.Players[1].Life != life+1 || s.Players[1].ProtectedUntil <= s.Frame {
		t.Fatal("respawn did not create protected new life")
	}
	if engine.QueueInput(s, 1, engine.Input{Seq: s.Players[1].ReceivedSeq + 1, Life: life, Fire: true}) == nil {
		t.Fatal("accepted stale life")
	}
	blocked := match(t, 2)
	blocked.World.Walls = append(blocked.World.Walls, engine.Rect{X: 130 * u, Y: 100 * u, W: u, H: 200 * u})
	blocked.Players[0].X, blocked.Players[0].Y = 80*u, 200*u
	blocked.Players[1].X, blocked.Players[1].Y = 180*u, 200*u
	blocked.Players[0].ProtectedUntil, blocked.Players[1].ProtectedUntil = 0, 0
	input(t, blocked, 0, engine.Input{Fire: true})
	engine.Advance(blocked, 15)
	if !blocked.Players[1].Alive {
		t.Fatal("projectile tunneled through thin wall")
	}
}

func TestWeaponsThrowPickupAndFinish(t *testing.T) {
	for _, w := range engine.Weapons() {
		s := match(t, 2)
		s.Players[0].Weapon, s.Players[0].Ammo = w.ID, w.Ammo
		s.Players[0].ProtectedUntil = 0
		input(t, s, 0, engine.Input{Throw: true})
		engine.Step(s)
		if s.Players[0].Weapon != "" || len(s.Bullets) != 1 {
			t.Fatalf("throw %s did not release held weapon", w.ID)
		}
	}
	s := match(t, 2)
	p := &s.Players[0]
	s.Drops = []engine.Drop{{ID: 900, X: p.X, Y: p.Y, Weapon: "uzi", Ammo: 3}}
	input(t, s, 0, engine.Input{Pickup: true})
	engine.Step(s)
	if s.Players[0].Weapon != "uzi" || s.Players[0].Ammo != 3 {
		t.Fatal("pickup refilled ammo")
	}
	s.Players[0].Ammo = 0
	input(t, s, 0, engine.Input{Fire: true})
	engine.Step(s)
	if len(s.Bullets) != 0 {
		t.Fatal("empty weapon fired")
	}
	s.Players[1].X, s.Players[1].Y = s.Players[0].X+20*engine.Unit, s.Players[0].Y
	s.Players[1].ProtectedUntil = 0
	s.Players[1].DownUntil = s.Frame + 120
	x, y := s.Players[0].X, s.Players[0].Y
	input(t, s, 0, engine.Input{Finish: true, MoveY: 1000})
	engine.Step(s)
	if s.Players[1].Alive || s.Players[0].Score != 1 {
		t.Fatal("finish did not settle")
	}
	if s.Players[0].X != x || s.Players[0].Y != y {
		t.Fatal("execution did not hold the attacker in an exposed animation")
	}
}

func TestDoorsGlassAndMutualKills(t *testing.T) {
	s := match(t, 2)
	u := engine.Unit
	s.World.Doors = []engine.DoorDef{{ID: 1, X: 115 * u, Y: 170 * u, W: 4 * u, H: 60 * u}}
	s.Doors = []engine.Door{{ID: 1}}
	s.Players[0].X, s.Players[0].Y = 90*u, 200*u
	input(t, s, 0, engine.Input{Interact: true})
	engine.Step(s)
	if !s.Doors[0].Open {
		t.Fatal("nearby door did not open")
	}
	m := match(t, 2)
	m.Players[0].X, m.Players[0].Y = 100*u, 200*u
	m.Players[1].X, m.Players[1].Y = 200*u, 200*u
	m.Players[0].ProtectedUntil, m.Players[1].ProtectedUntil = 0, 0
	input(t, m, 0, engine.Input{Aim: 0, Fire: true})
	input(t, m, 1, engine.Input{Aim: 512, Fire: true})
	engine.Advance(m, 12)
	if m.Players[0].Alive || m.Players[1].Alive || m.Players[0].Score != 1 || m.Players[1].Score != 1 {
		t.Fatalf("mutual attacks not preserved: %+v", m.Players)
	}
}

func TestFullBotMatchDeterminismAndBudgets(t *testing.T) {
	for _, count := range []int{2, 3, 4} {
		a, b := match(t, count), match(t, count)
		for !a.Finished {
			for seat := 0; seat < count; seat++ {
				input(t, a, seat, engine.BotInput(a, seat))
				input(t, b, seat, engine.BotInput(b, seat))
			}
			engine.Step(a)
			engine.Step(b)
			if len(a.Events) > engine.MaxEvents || len(a.Drops) > engine.MaxDrops || len(a.Bullets) > engine.MaxBullets {
				t.Fatal("unbounded state")
			}
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatal("same seed diverged")
		}
		kills := 0
		for _, p := range a.Players {
			kills += p.Kills
		}
		if kills == 0 {
			t.Fatal("bots completed without combat")
		}
		encoded, err := json.Marshal(a)
		if err != nil || len(encoded) > 150000 {
			t.Fatalf("unbounded serialized state bytes=%d err=%v", len(encoded), err)
		}
		before := a.Frame
		engine.Step(a)
		if a.Frame != before {
			t.Fatal("finished match advanced")
		}
	}
}

func TestBoundedProjectileHistory(t *testing.T) {
	setup := func(t *testing.T) *engine.State {
		s := match(t, 2)
		u := engine.Unit
		s.Players[0].X, s.Players[0].Y = 100*u, 200*u
		s.Players[1].X, s.Players[1].Y = 140*u, 198*u
		s.Players[0].ProtectedUntil, s.Players[1].ProtectedUntil = 0, 0
		engine.Advance(s, 4)
		input(t, s, 1, engine.Input{MoveY: 1000})
		engine.Advance(s, 10)
		return s
	}
	compensated := setup(t)
	input(t, compensated, 0, engine.Input{Fire: true, ViewFrame: 4})
	engine.Step(compensated)
	if compensated.Players[1].Alive || compensated.Players[0].Score != 1 || compensated.Players[0].Ammo != 7 {
		t.Fatal("historical projectile segment was not settled exactly once")
	}
	engine.Advance(compensated, 16)
	if compensated.Players[0].Score != 1 {
		t.Fatal("compensated projectile was advanced twice")
	}
	normal := setup(t)
	input(t, normal, 0, engine.Input{Fire: true})
	engine.Advance(normal, 16)
	if !normal.Players[1].Alive {
		t.Fatal("current projectile used historical target without a validated view frame")
	}
	if engine.QueueInput(normal, 0, engine.Input{Seq: normal.Players[0].ReceivedSeq + 1, Life: normal.Players[0].Life, Fire: true, ViewFrame: 17}) == nil {
		t.Fatal("accepted a future historical frame")
	}
	normal.Players[0].NextAttackFrame = 0
	input(t, normal, 0, engine.Input{Fire: true, ViewFrame: 1})
	engine.Advance(normal, 22)
	if !normal.Players[1].Alive {
		t.Fatal("stale view frame rewound beyond the history window")
	}
	oldLife := setup(t)
	oldLife.Players[1].Life++
	input(t, oldLife, 0, engine.Input{Fire: true, ViewFrame: 4})
	engine.Step(oldLife)
	if !oldLife.Players[1].Alive || oldLife.Players[0].Score != 0 {
		t.Fatal("historical hit damaged a new life")
	}
	closed := match(t, 2)
	u := engine.Unit
	closed.World.Doors = []engine.DoorDef{{ID: 1, X: 125 * u, Y: 100 * u, W: u, H: 200 * u}}
	closed.Doors = []engine.Door{{ID: 1}}
	closed.Players[0].X, closed.Players[0].Y = 100*u, 200*u
	closed.Players[1].X, closed.Players[1].Y = 140*u, 200*u
	closed.Players[0].ProtectedUntil, closed.Players[1].ProtectedUntil = 0, 0
	engine.Advance(closed, 8)
	closed.Doors[0].Open = true
	engine.Advance(closed, 10)
	input(t, closed, 0, engine.Input{Fire: true, ViewFrame: 4})
	engine.Step(closed)
	if !closed.Players[1].Alive || !closed.Doors[0].Open {
		t.Fatal("compensation ignored a historically closed door or rolled back current door state")
	}
}
