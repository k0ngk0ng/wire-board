package fortressblue

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func chooseOptions(t *testing.T, players int, options Options) gameapi.Match {
	t.Helper()
	raw, _ := json.Marshal(options)
	configured, err := configure(players, raw)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	m, err := Module().Start(gameapi.Context{Players: players, Now: now}, configured)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(players); err != nil {
		t.Fatal(err)
	}
	return m
}

func stateOf(t *testing.T, m gameapi.Match) *State {
	t.Helper()
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func applyAction(t *testing.T, m gameapi.Match, actor int, action Action, now time.Time) gameapi.Match {
	t.Helper()
	raw, _ := json.Marshal(action)
	next, err := Module().Apply(gameapi.Context{Players: len(stateOf(t, m).Tanks), Actor: actor, Now: now, Version: 1}, m, raw)
	if err != nil {
		t.Fatalf("apply %s seat %d: %v", action.Type, actor, err)
	}
	if err := next.Validate(len(stateOf(t, m).Tanks)); err != nil {
		t.Fatal(err)
	}
	return next
}

func rejectAction(t *testing.T, m gameapi.Match, actor int, action Action, now time.Time) {
	t.Helper()
	raw, _ := json.Marshal(action)
	if _, err := Module().Apply(gameapi.Context{Players: len(stateOf(t, m).Tanks), Actor: actor, Now: now, Version: 1}, m, raw); err == nil {
		t.Fatalf("action %s seat %d was accepted", action.Type, actor)
	}
}

func startPlaying(t *testing.T, players int, options Options) (*State, gameapi.Match, time.Time) {
	t.Helper()
	m := chooseOptions(t, players, options)
	s := stateOf(t, m)
	now := time.Unix(1_800_000_000, 0).Add(SelectMS * time.Millisecond)
	for i := range s.Tanks {
		class := classOrder[i%len(classOrder)]
		m = applyAction(t, m, i, Action{Type: "select", Epoch: s.Epoch, Class: class}, now)
	}
	s = stateOf(t, m)
	if s.Phase != PhasePlaying {
		t.Fatalf("phase after draft = %s", s.Phase)
	}
	return s, m, now
}

func TestConfigureTeamsAndMaps(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8, 10} {
		for _, id := range mapOrder {
			m := chooseOptions(t, players, Options{Map: id, TurnSeconds: 25})
			s := stateOf(t, m)
			if len(s.Tanks) != players || len(s.Order) != players || teamSize(players) != players/2 {
				t.Fatalf("players %d state %+v", players, s)
			}
			seen := map[int]bool{}
			for _, seat := range s.Order {
				if seen[seat] || seat < 0 || seat >= players {
					t.Fatalf("bad turn order %v", s.Order)
				}
				seen[seat] = true
			}
			for i, seat := range s.Order {
				wantTeam := TeamRed
				if i%2 == 1 {
					wantTeam = TeamBlue
				}
				if s.Tanks[seat].Team != wantTeam {
					t.Fatalf("order/team mismatch: %v", s.Order)
				}
			}
		}
	}
	for _, players := range []int{1, 3, 5, 7, 9, 11, 12} {
		raw, _ := json.Marshal(Options{Map: "hills", TurnSeconds: 25})
		if _, err := configure(players, raw); err == nil {
			t.Fatalf("players %d accepted", players)
		}
	}
	for _, options := range []Options{{Map: "bad", TurnSeconds: 25}, {Map: "hills", TurnSeconds: 5}, {Map: "hills", TurnSeconds: 60}} {
		raw, _ := json.Marshal(options)
		if _, err := configure(10, raw); err == nil {
			t.Fatalf("options %+v accepted", options)
		}
	}
}

func TestMapsSymmetricAndSpawnLedges(t *testing.T) {
	for _, id := range mapOrder {
		s, _, _ := startPlaying(t, 10, Options{Map: id, TurnSeconds: 20})
		if len(s.Heights) != Columns || s.MapID != id {
			t.Fatalf("map %s heights %d id %s", id, len(s.Heights), s.MapID)
		}
		for i := 0; i < Columns; i++ {
			if s.Heights[i] != s.Heights[Columns-1-i] {
				t.Fatalf("map %s column %d not symmetric", id, i)
			}
			if s.Heights[i] < 0 || s.Heights[i] > FieldHeight {
				t.Fatalf("map %s height out of range", id)
			}
		}
		for i, tank := range s.Tanks {
			if !tank.Selected || !tank.Alive || tank.MaxHP <= 0 {
				t.Fatalf("tank %d not initialized: %+v", i, tank)
			}
			if tank.X <= 0 || tank.X >= FieldWidth || tank.Y <= 0 || tank.Y >= FieldHeight {
				t.Fatalf("spawn out of field: %+v", tank)
			}
			if tank.Y < groundAt(s.Heights, tank.X)-0.01 {
				t.Fatalf("spawn inside terrain: %+v", tank)
			}
		}
	}
}

func TestSelectTimeoutStartsWithDeterministicPicks(t *testing.T) {
	m := chooseOptions(t, 4, Options{Map: "hills", TurnSeconds: 20})
	s := stateOf(t, m)
	now := time.Unix(1_800_000_000, 0)
	m = applyAction(t, m, 0, Action{Type: "select", Epoch: s.Epoch, Class: "heavy"}, now)
	if stateOf(t, m).Phase != PhaseSelect {
		t.Fatal("started before everyone picked")
	}
	late := time.Unix(1_800_000_000, 0).Add(SelectMS*time.Millisecond + 1)
	next, err := Module().OnTimer(gameapi.Context{Players: 4, Now: late}, m)
	if err != nil {
		t.Fatal(err)
	}
	s = stateOf(t, next)
	if s.Phase != PhasePlaying || s.Current != s.Order[0] {
		t.Fatalf("timeout did not start: %+v", s)
	}
	for _, tank := range s.Tanks {
		if !tank.Selected || !tank.Alive {
			t.Fatalf("timeout left unselected tank: %+v", tank)
		}
	}
	if next.SystemDeadline == 0 || next.SystemDeadline <= late.UnixMilli() {
		t.Fatalf("bad system deadline %d", next.SystemDeadline)
	}
}

func TestMoveAndFireAdvanceTurn(t *testing.T) {
	s, m, now := startPlaying(t, 4, Options{Map: "hills", TurnSeconds: 25})
	current := s.Current
	tank := s.Tanks[current]
	moveDir := 1
	if tank.Facing < 0 {
		moveDir = -1
	}
	m = applyAction(t, m, current, Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: moveDir}, now)
	s = stateOf(t, m)
	if s.Tanks[current].Fuel >= tank.Fuel {
		t.Fatalf("fuel did not decrease: %v -> %v", tank.Fuel, s.Tanks[current].Fuel)
	}
	// Move again with a stale turn is rejected.
	rejectAction(t, m, s.Current, Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn - 1, Dir: 1}, now)
	// Fire from the wrong seat is rejected.
	rejectAction(t, m, (s.Current+1)%len(s.Tanks), Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 60, Facing: 1, Weapon: "primary"}, now)
	m = applyAction(t, m, s.Current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 60, Facing: 1, Weapon: "primary"}, now)
	next := stateOf(t, m)
	if next.Turn != s.Turn+1 || next.Current == s.Current || next.LastShot == nil || len(next.LastShot.Arcs) != 1 {
		t.Fatalf("fire did not advance: %+v", next)
	}
	if next.Wind != windFor(next.Seed, next.Turn, next.Sudden) {
		t.Fatalf("wind not deterministic: %d", next.Wind)
	}
	// Stale fire for the previous turn is rejected.
	rejectAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 60, Facing: 1, Weapon: "primary"}, now)
}

func TestShotDamagesTargetAndCarvesTerrain(t *testing.T) {
	s, _, now := startPlaying(t, 2, Options{Map: "hills", TurnSeconds: 25})
	shooter, target := 0, 1
	st := &s.Tanks[shooter]
	tt := &s.Tanks[target]
	// Put the target on the same plateau, 30 px to the right.
	tt.X = st.X + 30
	tt.Y = groundAt(s.Heights, tt.X)
	beforeHP := tt.HP
	beforeHeight := append([]float64(nil), s.Heights...)
	// Apply a primary blast directly at the target. The fire/advance pipeline is
	// covered by TestMoveAndFireAdvanceTurn; this isolates damage/terrain rules.
	weapon := classes["balanced"].Primary
	blasts := []Blast{}
	total := 0
	applyBlast(s, shooter, weapon, Point{X: tt.X, Y: tt.Y - 7}, &blasts, now, &total)
	settleTanks(s, nil, now.UnixMilli())
	if s.Tanks[target].HP >= beforeHP && s.Tanks[target].Alive {
		t.Fatalf("target not damaged: %+v", s.Tanks[target])
	}
	if len(blasts) != 1 || total <= 0 {
		t.Fatalf("bad blast record: %+v", blasts)
	}
	changed := false
	for i := range s.Heights {
		if s.Heights[i] != beforeHeight[i] {
			changed = true
		}
	}
	if !changed {
		t.Fatal("terrain unchanged after explosion")
	}
}

func TestSpecialAmmoAndValidation(t *testing.T) {
	s, m, now := startPlaying(t, 2, Options{Map: "hills", TurnSeconds: 25})
	current := s.Current
	tank := s.Tanks[current]
	spec := classes[tank.Class]
	if spec.SpecialAmmo == 0 {
		t.Fatal("special missing")
	}
	rejectAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 4, Power: 50, Facing: 1, Weapon: "primary"}, now)
	rejectAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 50, Power: 120, Facing: 1, Weapon: "primary"}, now)
	rejectAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 50, Power: 50, Facing: 2, Weapon: "primary"}, now)
	rejectAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 50, Power: 50, Facing: 1, Weapon: "primary", Class: "heavy"}, now)
	m = applyAction(t, m, current, Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 50, Power: 50, Facing: 1, Weapon: "special"}, now)
	s = stateOf(t, m)
	used := s.Tanks[current]
	if used.Special != tank.Special-1 {
		t.Fatalf("special ammo %d -> %d", tank.Special, used.Special)
	}
	if len(s.LastShot.Arcs) != spec.Special.Shots {
		t.Fatalf("special arcs %d want %d", len(s.LastShot.Arcs), spec.Special.Shots)
	}
}

func TestFallDamageAndWater(t *testing.T) {
	s, _, _ := startPlaying(t, 2, Options{Map: "islands", TurnSeconds: 25})
	seat := s.Current
	tank := &s.Tanks[seat]
	// Carve a deep hole under the tank and settle it.
	col := int(tank.X / ColumnWidth)
	if col < 2 {
		col = 2
	}
	s.Heights[col] = 4
	s.Heights[col-1] = 4
	s.Heights[col+1] = 4
	hp := tank.HP
	settleTanks(s, nil, 1_800_000_000_000)
	if s.Tanks[seat].Alive && s.Tanks[seat].HP >= hp {
		t.Fatalf("falling into deep terrain did not damage: %+v", s.Tanks[seat])
	}
}

func TestTeamOutcomeAndTimeoutResult(t *testing.T) {
	s, m, now := startPlaying(t, 4, Options{Map: "hills", TurnSeconds: 25})
	for i := range s.Tanks {
		if s.Tanks[i].Team == TeamBlue {
			s.Tanks[i].Alive = false
			s.Tanks[i].HP = 0
		}
	}
	env, _ := s.envelope()
	m = env
	if !s.checkTeamOutcome(now.UnixMilli()) {
		t.Fatal("team wipeout not detected")
	}
	env, _ = s.envelope()
	m = env
	if m.Result == nil || !m.Result.Team || len(m.Result.Winners) != 2 {
		t.Fatalf("bad team result: %+v", m.Result)
	}
	for _, w := range m.Result.Winners {
		if s.Tanks[w].Team != TeamRed {
			t.Fatal("wrong team winner")
		}
	}

	s, m, _ = startPlaying(t, 2, Options{Map: "hills", TurnSeconds: 25})
	s.Tanks[0].HP, s.Tanks[1].HP = 20, 20
	s.MatchEndsAt = 1_800_000_000_000
	env, _ = s.envelope()
	next, err := Module().OnTimer(gameapi.Context{Players: 2, Now: time.Unix(1_800_000_000, 0)}, env)
	if err != nil {
		t.Fatal(err)
	}
	if next.Result == nil || !next.Result.Draw {
		t.Fatalf("tied HP should draw: %+v", next.Result)
	}
}

func TestSuddenDeathAndViewControls(t *testing.T) {
	s, m, now := startPlaying(t, 4, Options{Map: "hills", TurnSeconds: 25})
	view, err := Module().View(m, s.Current)
	if err != nil {
		t.Fatal(err)
	}
	v := view.(View)
	if !v.CanFire || !v.CanMove {
		t.Fatalf("current player cannot act: %+v", v)
	}
	other := (s.Current + 1) % len(s.Tanks)
	view, _ = Module().View(m, other)
	if view.(View).CanFire {
		t.Fatal("non-current player can fire")
	}
	view, _ = Module().View(m, -1)
	if view.(View).CanFire || view.(View).CanMove || view.(View).CanSelect {
		t.Fatal("spectator controls leaked")
	}
	late := time.UnixMilli(s.SuddenAt + 1)
	next, err := Module().OnTimer(gameapi.Context{Players: 4, Now: late}, m)
	if err != nil {
		t.Fatal(err)
	}
	s = stateOf(t, next)
	if !s.Sudden || s.Wind < -20 || s.Wind > 20 {
		t.Fatalf("sudden death not enabled: %+v", s)
	}
	if next.SystemDeadline == 0 || next.SystemDeadline <= late.UnixMilli() {
		t.Fatalf("sudden deadline %d", next.SystemDeadline)
	}
	_ = now
}

func TestDecodeRejectsMalformedState(t *testing.T) {
	m := chooseOptions(t, 2, Options{Map: "hills", TurnSeconds: 25})
	var raw map[string]any
	_ = json.Unmarshal(m.Data, &raw)
	cases := []map[string]any{}
	clone := func() map[string]any {
		b, _ := json.Marshal(raw)
		var v map[string]any
		_ = json.Unmarshal(b, &v)
		return v
	}
	bad := clone()
	bad["heights"] = []float64{1, 2}
	cases = append(cases, bad)
	bad = clone()
	bad["phase"] = "bogus"
	cases = append(cases, bad)
	bad = clone()
	bad["tanks"] = []any{}
	cases = append(cases, bad)
	bad = clone()
	bad["epoch"] = ""
	cases = append(cases, bad)
	for i, value := range cases {
		rawData, _ := json.Marshal(value)
		badMatch := gameapi.Match{Schema: 1, Data: rawData}
		if _, err := decode(badMatch); err == nil {
			t.Fatalf("malformed state %d accepted", i)
		}
	}
	badMatch := gameapi.Match{Schema: 2, Data: m.Data}
	if _, err := decode(badMatch); err == nil {
		t.Fatal("wrong schema accepted")
	}
	if !strings.Contains(strings.ToLower(m.Log[0]), "疯狂坦克") {
		t.Fatalf("missing intro log: %v", m.Log)
	}
}

func TestSimultaneousWipeoutIsExplicitDraw(t *testing.T) {
	s, _, _ := startPlaying(t, 2, Options{Map: "hills", TurnSeconds: 20})
	for i := range s.Tanks {
		s.Tanks[i].Alive, s.Tanks[i].HP = false, 0
	}
	if !s.checkTeamOutcome(1_800_000_020_000) {
		t.Fatal("wipeout not detected")
	}
	m, err := s.envelope()
	if err != nil {
		t.Fatal(err)
	}
	if m.Result == nil || !m.Result.Draw || len(m.Result.Winners) != 0 {
		t.Fatalf("simultaneous wipeout not a draw: %+v", m.Result)
	}
	for _, rank := range m.Result.Ranks {
		if rank != 1 {
			t.Fatal("draw rank")
		}
	}
}
