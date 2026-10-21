package bubbleparty

import (
	"encoding/json"
	"math"
	"testing"
)

// Failure inventory: docs/research/bubble-classic-maps/PUSH-SCENARIOS.md.
func pushFixture() *State {
	s := &State{Schema: 1, RulesVersion: 3, Phase: PhasePlaying, Cells: make([]byte, Cols*Rows),
		NativeObjects: make([]int, Cols*Rows), Epoch: "push-test", MapID: "community10", TeamSize: 1,
		Options: Options{Map: "community10", RoundSeconds: 300}, ServerAt: 1000, StartAt: 1000, MatchEndsAt: 301000}
	for i := range s.NativeObjects {
		s.NativeObjects[i] = -1
	}
	s.Players = []Player{
		{Seat: 0, Team: 0, Alive: true, X: 4, Y: 5, OffsetX: .5 - PlayerRadius, SpeedMS: BaseMoveMS, MaxBombs: 1, Range: 2},
		{Seat: 1, Team: 1, Alive: true, X: 12, Y: 10, SpeedMS: BaseMoveMS, MaxBombs: 1, Range: 2},
	}
	s.setCell(5, 5, CellPushCrate)
	s.NativeObjects[5*Cols+5] = 66
	return s
}

func setPushPosition(p *Player, x, y float64) {
	p.X, p.Y = int(math.Floor(x+.5)), int(math.Floor(y+.5))
	p.OffsetX, p.OffsetY = x-float64(p.X), y-float64(p.Y)
}

func TestPushBoxFourDirectionsFromContinuousContact(t *testing.T) {
	for _, tc := range []struct {
		dx, dy int
		dir    string
	}{{1, 0, "right"}, {-1, 0, "left"}, {0, 1, "down"}, {0, -1, "up"}} {
		t.Run(tc.dir, func(t *testing.T) {
			s := pushFixture()
			p := &s.Players[0]
			setPushPosition(p, 5-float64(tc.dx)*(.5+PlayerRadius), 5-float64(tc.dy)*(.5+PlayerRadius))
			beforeX, beforeY := p.X, p.Y
			if !s.canPushBox(p, 5, 5, tc.dx, tc.dy) || !s.tryPushBox(p, tc.dx, tc.dy, 1234) {
				t.Fatal("aligned contact did not push", p)
			}
			x, y := 5+tc.dx, 5+tc.dy
			if s.cell(5, 5) != CellEmpty || s.cell(x, y) != CellPushCrate || s.NativeObjects[5*Cols+5] != -1 || s.NativeObjects[y*Cols+x] != 66 {
				t.Fatal("box kind or native identity did not move")
			}
			if p.X != beforeX || p.Y != beforeY || len(s.Events) != 1 {
				t.Fatal("push mutated player position or emitted duplicate events")
			}
			e := s.Events[0]
			if e.Type != "push" || e.Seat != 0 || e.X != x || e.Y != y || e.Kind != tc.dir || e.Value != 5*Cols+5 || e.At != 1234 {
				t.Fatal("incorrect authoritative push event", e)
			}
			if s.tryPushBox(p, tc.dx, tc.dy, 1234) || len(s.Events) != 1 {
				t.Fatal("same contact point pushed twice")
			}
		})
	}
}

func TestPushBoxRejectsAllOccupiedTargets(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(*State)
	}{
		{"native-object", func(s *State) { s.NativeObjects[5*Cols+6] = 70 }},
		{"fixed-brick", func(s *State) { s.setCell(6, 5, CellCrate) }},
		{"pushable-box-no-chain", func(s *State) { s.setCell(6, 5, CellPushCrate) }},
		{"tree-or-wall", func(s *State) { s.setCell(6, 5, CellWall) }},
		{"bomb-cell", func(s *State) { s.setCell(6, 5, CellBomb) }},
		{"bomb-entity", func(s *State) { s.Bombs = []Bomb{{ID: 1, X: 6, Y: 5}} }},
		{"item", func(s *State) { s.Items = []Item{{ID: 1, X: 6, Y: 5, Type: ItemSpeed}} }},
		{"opponent", func(s *State) { s.Players[1].X, s.Players[1].Y = 6, 5 }},
		{"teammate", func(s *State) { s.Players[1].X, s.Players[1].Y, s.Players[1].Team = 6, 5, 0 }},
		{"trapped-opponent", func(s *State) { s.Players[1].X, s.Players[1].Y, s.Players[1].TrappedUntil = 6, 5, 9999 }},
		{"neighbor-body", func(s *State) { setPushPosition(&s.Players[1], 6.5+PlayerRadius-.01, 5) }},
		{"neighbor-body-tangent", func(s *State) { setPushPosition(&s.Players[1], 6.5+PlayerRadius, 5) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := pushFixture()
			tc.arrange(s)
			before, _ := json.Marshal(s)
			if s.canPushBox(&s.Players[0], 5, 5, 1, 0) || s.tryPushBox(&s.Players[0], 1, 0, 1234) {
				t.Fatal("occupied target accepted")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejected push mutated state")
			}
		})
	}
}

func TestPushBoxRejectsNonContactAndInvalidActors(t *testing.T) {
	cases := []struct {
		name    string
		dx, dy  int
		arrange func(*State)
	}{
		{"not-yet-contact", 1, 0, func(s *State) { s.Players[0].OffsetX = 0 }},
		{"corner-not-row-aligned", 1, 0, func(s *State) { s.Players[0].OffsetY = .1 }},
		{"overlap-not-face-contact", 1, 0, func(s *State) { s.Players[0].OffsetX += .02 }},
		{"diagonal", 1, 1, func(*State) {}},
		{"zero-direction", 0, 0, func(*State) {}},
		{"jump-direction", 2, 0, func(*State) {}},
		{"fixed-brick-source", 1, 0, func(s *State) { s.setCell(5, 5, CellCrate) }},
		{"wall-source", 1, 0, func(s *State) { s.setCell(5, 5, CellWall) }},
		{"legacy-rules", 1, 0, func(s *State) { s.RulesVersion = 2 }},
		{"dead", 1, 0, func(s *State) { s.Players[0].Alive = false }},
		{"trapped", 1, 0, func(s *State) { s.Players[0].TrappedUntil = 9999 }},
		{"airship", 1, 0, func(s *State) { s.Players[0].Mount = "airship" }},
		{"broken-airship", 1, 0, func(s *State) { s.Players[0].Mount = "airship-broken" }},
		{"countdown", 1, 0, func(s *State) { s.Phase = PhaseCountdown }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := pushFixture()
			tc.arrange(s)
			if s.tryPushBox(&s.Players[0], tc.dx, tc.dy, 1234) {
				t.Fatal("invalid contact/actor pushed")
			}
			if len(s.Events) != 0 {
				t.Fatal("invalid push emitted event")
			}
		})
	}
	s := pushFixture()
	if s.tryPushBox(nil, 1, 0, 1234) {
		t.Fatal("nil actor pushed")
	}
}

func TestPushBoxMapEdgesAndDeadBody(t *testing.T) {
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		s := pushFixture()
		x, y := 5, 5
		if d[0] > 0 {
			x = Cols - 1
		}
		if d[0] < 0 {
			x = 0
		}
		if d[1] > 0 {
			y = Rows - 1
		}
		if d[1] < 0 {
			y = 0
		}
		s.setCell(5, 5, CellEmpty)
		s.setCell(x, y, CellPushCrate)
		setPushPosition(&s.Players[0], float64(x)-float64(d[0])*(.5+PlayerRadius), float64(y)-float64(d[1])*(.5+PlayerRadius))
		if s.tryPushBox(&s.Players[0], d[0], d[1], 1234) {
			t.Fatal("pushed outside map", d)
		}
	}
	s := pushFixture()
	s.Players[1].Alive = false
	s.Players[1].X, s.Players[1].Y = 6, 5
	if !s.tryPushBox(&s.Players[0], 1, 0, 1234) {
		t.Fatal("dead player blocked push")
	}
}

func TestPushBoxCompetitionIsAtomicAndNativeMetadataOptional(t *testing.T) {
	s := pushFixture()
	s.setCell(6, 4, CellPushCrate)
	s.NativeObjects[4*Cols+6] = 215
	setPushPosition(&s.Players[1], 6, 4-(.5+PlayerRadius))
	if !s.tryPushBox(&s.Players[0], 1, 0, 1234) {
		t.Fatal("first push failed")
	}
	if s.tryPushBox(&s.Players[1], 0, 1, 1234) {
		t.Fatal("second push overwrote contested target")
	}
	if s.cell(6, 4) != CellPushCrate || s.NativeObjects[4*Cols+6] != 215 || s.NativeObjects[5*Cols+6] != 66 || len(s.Events) != 1 {
		t.Fatal("competition lost a box or identity")
	}
	for _, n := range []int{0, 1} {
		s := pushFixture()
		s.NativeObjects = make([]int, n)
		if !s.tryPushBox(&s.Players[0], 1, 0, 1234) {
			t.Fatal("legacy/native-free map could not push")
		}
	}
}

func TestPushBoxPersistsAndRemainsDestructibleAfterRestore(t *testing.T) {
	s := pushFixture()
	if !s.tryPushBox(&s.Players[0], 1, 0, 1234) {
		t.Fatal("push failed")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.cell(6, 5) != CellPushCrate || restored.NativeObjects[5*Cols+6] != 66 || restored.NativeObjects[5*Cols+5] != -1 {
		t.Fatal("restore lost moved crate")
	}
	restored.Bombs = []Bomb{{ID: 1, Seat: 0, X: 4, Y: 5, Range: 4, ExplodeAt: 2000}}
	restored.setCell(4, 5, CellBomb)
	restored.explodeBombs(2000)
	if restored.cell(6, 5) != CellEmpty || restored.NativeObjects[5*Cols+6] != -1 || restored.Players[0].Crates != 1 {
		t.Fatal("moved box was not destroyed with its native identity")
	}
	for _, b := range restored.Blasts {
		if b.Y == 5 && b.X > 6 {
			t.Fatal("water passed through moved box", b)
		}
	}
}

func TestPushBoxMotionPersistsUntilExactDeadline(t *testing.T) {
	s := pushFixture()
	if !s.tryPushBox(&s.Players[0], 1, 0, 1000) {
		t.Fatal("push failed")
	}
	if len(s.BoxMotions) != 1 {
		t.Fatal("missing movement envelope")
	}
	m := s.BoxMotions[0]
	if m.FromX != 5 || m.FromY != 5 || m.X != 6 || m.Y != 5 || m.At != 1000 || m.EndAt != 1300 || m.ObjectID != 66 {
		t.Fatal("incorrect server movement envelope", m)
	}
	setPushPosition(&s.Players[0], 6-(.5+PlayerRadius), 5)
	s.ServerAt = 1299
	if s.canPushBox(&s.Players[0], 6, 5, 1, 0) || s.tryPushBox(&s.Players[0], 1, 0, 1299) {
		t.Fatal("moving box pushed before landing")
	}
	// ServerAt intentionally still points to the previous step; try uses at.
	if !s.tryPushBox(&s.Players[0], 1, 0, 1300) {
		t.Fatal("landed box still locked at deadline")
	}
	if len(s.BoxMotions) != 1 || s.BoxMotions[0].X != 7 || s.BoxMotions[0].At != 1300 {
		t.Fatal("expired envelope not replaced", s.BoxMotions)
	}
}

func TestPushBoxRejectsMovingBoxSourceAndDestination(t *testing.T) {
	for _, tc := range []struct {
		name   string
		motion BoxMotion
	}{
		{"source", BoxMotion{FromX: 6, FromY: 5, X: 6, Y: 6, At: 1000, EndAt: 1300}},
		{"target", BoxMotion{FromX: 6, FromY: 4, X: 6, Y: 5, At: 1000, EndAt: 1300}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pushFixture()
			s.BoxMotions = []BoxMotion{tc.motion}
			if s.tryPushBox(&s.Players[0], 1, 0, 1299) {
				t.Fatal("entered moving box envelope")
			}
			if !s.tryPushBox(&s.Players[0], 1, 0, 1300) {
				t.Fatal("expired moving box envelope still blocks")
			}
		})
	}
}

func TestPushBoxMotionBoundAndNativeFreeIdentity(t *testing.T) {
	s := pushFixture()
	for i := 0; i < 32; i++ {
		s.BoxMotions = append(s.BoxMotions, BoxMotion{FromX: 1, FromY: 1, X: 2, Y: 1, At: 1000, EndAt: 1300})
	}
	if s.tryPushBox(&s.Players[0], 1, 0, 1299) {
		t.Fatal("exceeded active motion limit")
	}
	if !s.tryPushBox(&s.Players[0], 1, 0, 1300) || len(s.BoxMotions) != 1 {
		t.Fatal("expired motion limit not reclaimed")
	}
	s = pushFixture()
	s.NativeObjects = nil
	if !s.tryPushBox(&s.Players[0], 1, 0, 1000) || s.BoxMotions[0].ObjectID != -1 {
		t.Fatal("native-free box invented object identity")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.BoxMotions) != 1 || restored.BoxMotions[0] != s.BoxMotions[0] {
		t.Fatal("motion lost in JSON restore")
	}
}
