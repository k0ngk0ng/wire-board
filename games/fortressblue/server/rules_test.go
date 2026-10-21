package fortressblue

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func ctx(players, actor int, now time.Time) gameapi.Context {
	return gameapi.Context{Actor: actor, Players: players, Version: 1, RequestedVersion: 1, Now: now}
}

func startAt(t *testing.T, players int, mapID string) (*State, gameapi.Match) {
	t.Helper()
	now := time.Unix(1700000000, 0)
	raw, _ := json.Marshal(Options{Map: mapID, TurnSeconds: 30, BotDifficulty: "hard"})
	m, err := start(ctx(players, -1, now), raw)
	if err != nil {
		t.Fatal(err)
	}
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	return s, m
}

func selectAll(t *testing.T, s *State, m gameapi.Match, ids []int) (gameapi.Match, *State) {
	t.Helper()
	now := time.Unix(1700000001, 0)
	for seat, id := range ids {
		raw, _ := json.Marshal(Action{Type: "select", Epoch: s.Epoch, Tank: id})
		var err error
		m, err = apply(ctx(len(ids), seat, now), m, raw)
		if err != nil {
			t.Fatalf("select seat %d: %v", seat, err)
		}
	}
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, s
}

func TestOriginalCatalog(t *testing.T) {
	c, err := fbmaps.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Maps) != 17 {
		t.Fatalf("maps = %d, want 17", len(c.Maps))
	}
	for _, m := range c.Maps {
		if m.Width < 1200 || m.Height != 960 || len(m.Spawns) != 8 || m.Mask == "" {
			t.Fatalf("bad map %+v", m)
		}
		mask := fbmaps.BaseMask(m)
		if len(mask) != (m.MaskCols*m.MaskRows+7)/8 {
			t.Fatalf("mask size %s", m.ID)
		}
	}
	tanks, err := fbdata.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(tanks.Tanks) != 14 || tanks.Explosion.Count != 28 || tanks.Explosion.CellW <= 0 {
		t.Fatalf("tank catalog invalid: %d tanks, explosion %+v", len(tanks.Tanks), tanks.Explosion)
	}
	for id, tank := range tanks.Tanks {
		if tank.HP <= 0 || tank.Primary <= 0 || tank.Sprites["idle"].URL == "" || tank.Bullet == nil {
			t.Fatalf("tank %s incomplete: %+v", id, tank)
		}
	}
}

func TestStartTeamSpawns(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8} {
		s, _ := startAt(t, players, "stage-16")
		if len(s.Tanks) != players || s.Phase != PhaseSelect {
			t.Fatalf("%d players state: %d tanks phase %s", players, len(s.Tanks), s.Phase)
		}
		for i, tank := range s.Tanks {
			if tank.Team != teamOf(i, players) || !tank.Alive || tank.X < 0 || tank.X > 1536 {
				t.Fatalf("%d players tank %d invalid: %+v", players, i, tank)
			}
		}
		if s.Tanks[0].X >= s.Tanks[players-1].X {
			t.Fatalf("%d players spawns not separated: %+v", players, s.Tanks)
		}
	}
}

func TestProjectileTraversesOpenAir(t *testing.T) {
	m := fbmaps.Map{Width: 2048, Height: 960, MaskCols: 512, MaskRows: 240}
	mask := make([]byte, (m.MaskCols*m.MaskRows+7)/8)
	arc := simulateArc(m, mask, []Tank{{Seat: 0, Alive: true, X: 200, Y: 700}}, 0, 200, 700, 45, 100, 1, 0.9, 0, MaxPath)
	if len(arc.Path) < 10 || arc.Impact.X <= 200 || !arc.OutOfBounds {
		t.Fatalf("open-air projectile failed to travel to a world boundary: %+v", arc.Impact)
	}
}

func TestTerrainMaskAndProjectile(t *testing.T) {
	m, ok := fbmaps.ByID("stage-16")
	if !ok {
		t.Fatal("map missing")
	}
	mask := fbmaps.BaseMask(m)
	// The foreground layer has solid terrain near the bottom and air above.
	if !solid(m, mask, 54, 800) {
		t.Fatal("expected solid ground near left spawn")
	}
	if solid(m, mask, 54, 40) {
		t.Fatal("expected air at the top")
	}
	arc := simulateArc(m, mask, []Tank{{Seat: 0, Alive: true, X: 300, Y: 700}}, 0, 300, 700, 45, 45, 1, 1, 0, MaxPath)
	if len(arc.Path) < 10 || arc.Impact.X <= 300 {
		t.Fatalf("arc did not travel: %+v", arc)
	}
	if !solid(m, mask, arc.Impact.X, arc.Impact.Y) {
		t.Fatalf("arc did not stop on terrain: %+v", arc.Impact)
	}
}

func TestBlastCarvesAndDamages(t *testing.T) {
	s, match := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, match, []int{1, 2})
	m, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(m, s.Craters)
	target := s.Tanks[1]
	before := target.HP
	s.Tanks[0].Damage = 0
	blast := applyBlast(s, 0, Weapon{Name: "主炮", Damage: 300, Blast: 20}, Point{X: target.X, Y: target.Y - 18}, "", 1, 1)
	if blast.R < 30 || len(s.Craters) != 1 {
		t.Fatalf("blast/crater missing: %+v craters=%d", blast, len(s.Craters))
	}
	if s.Tanks[1].HP >= before || s.Tanks[0].Damage <= 0 {
		t.Fatalf("damage not applied: hp %d -> %d, damage %d", before, s.Tanks[1].HP, s.Tanks[0].Damage)
	}
	after := maskFor(m, s.Craters)
	if len(after) != len(mask) || string(after) == string(mask) {
		t.Fatal("terrain mask did not change")
	}
}

func TestSelectAndTurnAdvance(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	m, s = selectAll(t, s, m, []int{1, 2})
	if s.Phase != PhasePlaying || s.Current != 0 || s.Turn != 1 {
		t.Fatalf("playing not started: phase=%s current=%d turn=%d", s.Phase, s.Current, s.Turn)
	}
	raw, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 70, Facing: 1, Weapon: "primary"})
	now := time.Unix(1700000002, 0)
	m2, err := apply(ctx(2, 0, now), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	m2 = finishTestResolution(t, m2)
	s2, err := decode(m2)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Turn != 2 || s2.Current != 1 || s2.LastShot == nil || len(s2.LastShot.Arcs) != 1 {
		t.Fatalf("turn did not advance: turn=%d current=%d shot=%+v", s2.Turn, s2.Current, s2.LastShot)
	}
}

func TestSpecialFireCreatesGroundHazard(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	m, s = selectAll(t, s, m, []int{1, 1})
	if s.Tanks[0].Special != -1 {
		t.Fatalf("special ammo = %d", s.Tanks[0].Special)
	}
	// Put the target directly in front to guarantee a direct hit.
	s.Tanks[1].X = s.Tanks[0].X + 40
	s.Tanks[1].Y = s.Tanks[0].Y
	m = mustMatch(t, s)
	raw, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 0, Power: 30, Facing: 1, Weapon: "special"})
	m2, err := apply(ctx(2, 0, time.Unix(1700000003, 0)), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	m2 = finishTestResolution(t, m2)
	s2, _ := decode(m2)
	if s2.Tanks[0].Special != -1 {
		t.Fatalf("special ammo not consumed: %d", s2.Tanks[0].Special)
	}
	var hazardView struct{ Hazards []struct{ Kind string } }
	_ = json.Unmarshal(m2.Data, &hazardView)
	if len(hazardView.Hazards) == 0 {
		t.Fatal("fire special did not create ground hazard")
	}
}

func TestTimeoutAdvancesTurn(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	m, s = selectAll(t, s, m, []int{1, 2})
	now := time.Unix(1700000100, 0)
	s.Deadline = now.Add(-time.Second).UnixMilli()
	m = mustMatch(t, s)
	raw, _ := json.Marshal(s.Options)
	_ = raw
	m2, err := onTimer(ctx(2, -1, now), m)
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := decode(m2)
	if s2.Turn != 2 || s2.Current != 1 {
		t.Fatalf("timeout did not advance: turn=%d current=%d", s2.Turn, s2.Current)
	}
}

func TestDrawAndWinner(t *testing.T) {
	s, _ := startAt(t, 2, "stage-16")
	for i := range s.Tanks {
		s.Tanks[i].Alive = false
	}
	if !s.checkTeamOutcome(1) || !s.Draw || s.Phase != PhaseFinished {
		t.Fatalf("simultaneous death not draw: %+v", s)
	}
	s, _ = startAt(t, 2, "stage-16")
	s.Tanks[0].Alive = false
	if !s.checkTeamOutcome(1) || s.Draw || s.WinnerTeam != TeamBlue {
		t.Fatalf("blue win not detected: %+v", s)
	}
}

func TestBadStateRejected(t *testing.T) {
	s, _ := startAt(t, 2, "stage-16")
	s.Craters = append(s.Craters, Crater{X: -100, Y: 10, R: 20})
	if _, err := decode(mustMatch(t, s)); err == nil {
		t.Fatal("negative crater accepted")
	}
	s, _ = startAt(t, 2, "stage-16")
	s.Tanks[0].Team = 1
	if _, err := decode(mustMatch(t, s)); err == nil {
		t.Fatal("wrong team accepted")
	}
}

func TestBotReturnsLegalActions(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	now := time.Unix(1700000000, 0)
	for seat := 0; seat < 2; seat++ {
		raw, err := botAction(ctx(2, seat, now), m)
		if err != nil || raw == nil {
			t.Fatalf("bot select seat %d: %v %s", seat, err, raw)
		}
		m, err = apply(ctx(2, seat, now), m, raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, _ = decode(m)
	if s.Phase != PhasePlaying {
		t.Fatalf("bots did not start match: %s", s.Phase)
	}
	raw, err := botAction(ctx(2, s.Current, now), m)
	if err != nil || raw == nil {
		t.Fatalf("bot fire: %v %s", err, raw)
	}
	var a Action
	_ = json.Unmarshal(raw, &a)
	if a.Type == "item" {
		m, err = apply(ctx(2, s.Current, now), m, raw)
		if err != nil {
			t.Fatal(err)
		}
		raw, err = botAction(ctx(2, s.Current, now), m)
		if err != nil || raw == nil {
			t.Fatalf("bot fire after item: %v %s", err, raw)
		}
		_ = json.Unmarshal(raw, &a)
	}
	if a.Type != "fire" && a.Type != "move" || a.Type == "fire" && (a.Angle < 0 || a.Angle > 90 || a.Power < 0 || a.Power > 100) {
		t.Fatalf("illegal bot action: %s %+v", raw, a)
	}
}

func mustMatch(t *testing.T, s *State) gameapi.Match {
	t.Helper()
	m, err := s.envelope()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDamageFormula(t *testing.T) {
	s, match := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, match, []int{1, 2})
	def := tankDefense(s.Tanks[1])
	if def <= 0 {
		t.Fatal("defense not loaded")
	}
	// Direct hit should remove a meaningful fraction of 950–1150 HP.
	before := s.Tanks[1].HP
	applyBlast(s, 0, Weapon{Damage: 300, Blast: 20}, Point{X: s.Tanks[1].X, Y: s.Tanks[1].Y - 18}, "", 1, 1)
	lost := before - s.Tanks[1].HP
	if lost < 80 || lost > 400 {
		t.Fatalf("unexpected direct damage %d (defense %d)", lost, def)
	}
}

func TestAimBounds(t *testing.T) {
	if clamp(100, 0, 90) != 90 || math.Abs(clamp(-3, 0, 90)) != 0 {
		t.Fatal("clamp broken")
	}
}

func TestSelectTimeoutStartsMatch(t *testing.T) {
	s, m := startAt(t, 4, "stage-7")
	now := time.Unix(1700000200, 0)
	s.SelectUntil = now.Add(-time.Second).UnixMilli()
	m = mustMatch(t, s)
	m2, err := onTimer(ctx(4, -1, now), m)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := decode(m2)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhasePlaying || s2.Current != 0 || s2.Turn != 1 {
		t.Fatalf("select timeout did not start: %+v", s2)
	}
	for _, tank := range s2.Tanks {
		if !tank.Selected || tank.MaxHP <= 0 || tank.Alive == false {
			t.Fatalf("timeout tank incomplete: %+v", tank)
		}
	}
}

func TestAllSpecialSkillKinds(t *testing.T) {
	cases := []struct {
		tank   int
		status string
	}{
		{1, "burn"},
		{2, "poison"},
		{9, ""},
	}
	for _, tc := range cases {
		s, match := startAt(t, 2, "stage-16")
		_, s = selectAll(t, s, match, []int{tc.tank, 1})
		s.Tanks[1].X = s.Tanks[0].X + 42
		s.Tanks[1].Y = s.Tanks[0].Y
		match = mustMatch(t, s)
		spec, _ := tankSpec(tc.tank)
		raw, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: float64(spec.MinAngle), Power: 30, Facing: 1, Weapon: "special"})
		next, err := apply(ctx(2, 0, time.Unix(1700000300, 0)), match, raw)
		if err != nil {
			t.Fatalf("tank %d special: %v", tc.tank, err)
		}
		next = finishTestResolution(t, next)
		s2, err := decode(next)
		if err != nil {
			t.Fatal(err)
		}
		if s2.Tanks[0].Special != -1 || s2.LastShot == nil || len(s2.LastShot.Blasts) != 1 {
			t.Fatalf("tank %d special state: %+v", tc.tank, s2.Tanks[0])
		}
		if tc.status != "" {
			var hazardView struct{ Hazards []struct{ Kind string } }
			_ = json.Unmarshal(next.Data, &hazardView)
			if len(hazardView.Hazards) == 0 {
				t.Fatalf("tank %d missing hazard", tc.tank)
			}
		}
		if tc.tank == 9 && s2.LastShot.Blasts[0].R <= 50 {
			t.Fatalf("explosion special radius = %v", s2.LastShot.Blasts[0].R)
		}
	}
}

func TestClientCatalogMatchesServer(t *testing.T) {
	all, err := fbmaps.All()
	if err != nil {
		t.Fatal(err)
	}
	client, err := fbmaps.Client()
	if err != nil {
		t.Fatal(err)
	}
	if len(client.Maps) != len(all.Maps) {
		t.Fatalf("client maps %d server %d", len(client.Maps), len(all.Maps))
	}
	byID := map[string]fbmaps.Map{}
	for _, m := range all.Maps {
		byID[m.ID] = m
	}
	for _, c := range client.Maps {
		m, ok := byID[c.ID]
		if !ok || m.Name != c.Name || m.Width != c.Width || m.OffY != c.OffY || c.FG == "" || c.Thumb == "" {
			t.Fatalf("client map mismatch %+v", c)
		}
	}
}

func TestCraterPersistsAcrossSnapshot(t *testing.T) {
	s, _ := startAt(t, 2, "stage-16")
	m, _ := fbmaps.ByID(s.MapID)
	s.Craters = append(s.Craters, Crater{X: 500, Y: 700, R: 40, Seat: 0})
	before := maskFor(m, s.Craters)
	match := mustMatch(t, s)
	s2, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	after := maskFor(m, s2.Craters)
	if len(before) != len(after) || string(before) != string(after) {
		t.Fatal("crater mask changed across snapshot")
	}
}

// Existing rule cases assert resolved outcomes; exercise the public timer path
// instead of making Apply resolve a shot synchronously.
func finishTestResolution(t *testing.T, m gameapi.Match) gameapi.Match {
	t.Helper()
	for i := 0; i < 32; i++ {
		var w struct{ Resolution *struct{ Stage string } }
		if err := json.Unmarshal(m.Data, &w); err != nil {
			t.Fatal(err)
		}
		if w.Resolution == nil {
			return m
		}
		s, err := decode(m)
		if err != nil {
			t.Fatal(err)
		}
		m, err = onTimer(ctx(len(s.Tanks), -1, time.UnixMilli(m.SystemDeadline)), m)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("resolution did not complete")
	return m
}
