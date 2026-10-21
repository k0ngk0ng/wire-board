package fortressblue

// Failure inventory first: fixed inventory mistaken for drops; selection/flight
// spawns early; airborne crates award early; enemy/dead/full inventory pickup;
// duplicate timer/reload dupes; unsupported terrain leaves invisible rewards;
// stale/invalid saves; bots bypass public movement or ignore reachable supplies.
import (
	"encoding/json"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"testing"
	"time"
)

func supplyState(t *testing.T) *State {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{1, 1})
	return s
}
func TestSupplySpawnAndTimerPersistence(t *testing.T) {
	s := supplyState(t)
	at := time.UnixMilli(s.StartAt + 1000)
	s.NextSupplyTurn = s.Turn + 1
	scheduleSupply(s, at.UnixMilli())
	if len(s.Supplies) != 0 {
		t.Fatal("supply before scheduled turn")
	}
	advanceTurn(s, at)
	if len(s.Supplies) != 1 {
		t.Fatal("no supply on due turn")
	}
	d := s.Supplies[0]
	if d.DropAt <= d.At || d.LandAt <= d.DropAt || d.Landed {
		t.Fatal(d)
	}
	before := len(s.Tanks[0].Items)
	m, _ := s.envelope()
	restored, err := restore(ctx(2, 0, at), m)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = decode(restored)
	if s.Supplies[0] != d {
		t.Fatal("restore changed supply")
	}
	processSupplies(s, d.LandAt-1)
	if s.Supplies[0].Landed || len(s.Tanks[0].Items) != before {
		t.Fatal("awarded airborne")
	}
	s.Tanks[0].X = d.X
	s.Tanks[0].Y = d.Y
	s.Tanks[0].Items = nil
	processSupplies(s, d.LandAt)
	if len(s.Tanks[0].Items) != 0 {
		t.Fatal("pickup preceded movement/contact display")
	}
	processSupplies(s, d.LandAt+180)
	if len(s.Supplies) != 0 || len(s.Tanks[0].Items) != 1 || s.Tanks[0].Items[0] != d.ItemID {
		t.Fatal("landing pickup not applied once")
	}
	processSupplies(s, d.LandAt+181)
	if len(s.Tanks[0].Items) != 1 {
		t.Fatal("duplicate pickup")
	}
}
func TestSupplyPickupBothTeamsCapacityAndTerrain(t *testing.T) {
	for _, seat := range []int{0, 1} {
		s := supplyState(t)
		s.Supplies = []Supply{{ID: 1, ItemID: 0, X: s.Tanks[seat].X, Y: s.Tanks[seat].Y, FromX: s.Tanks[seat].X, At: s.StartAt, DropAt: s.StartAt + 1, LandAt: s.StartAt + 2, Landed: true}}
		s.Tanks[seat].Items = []int{0, 1, 2, 3}
		processSupplies(s, s.StartAt+3)
		if len(s.Supplies) != 1 {
			t.Fatal("full inventory consumed supply")
		}
		s.Tanks[seat].Items = nil
		s.Tanks[seat].Alive = false
		processSupplies(s, s.StartAt+4)
		if len(s.Supplies) != 1 {
			t.Fatal("dead tank got supply")
		}
		s.Tanks[seat].Alive = true
		hp := s.Tanks[seat].HP
		processSupplies(s, s.StartAt+5)
		processSupplies(s, s.StartAt+185)
		if len(s.Tanks[seat].Items) != 1 || s.Tanks[seat].HP != hp {
			t.Fatal("pickup bypasses item usage")
		}
	}
	s := supplyState(t)
	s.Supplies = []Supply{{ID: 1, ItemID: 0, X: s.Tanks[0].X, Y: s.Tanks[0].Y, FromX: s.Tanks[0].X, At: s.StartAt, DropAt: s.StartAt + 1, LandAt: s.StartAt + 2, Landed: true}}
	s.Craters = append(s.Craters, Crater{X: s.Supplies[0].X, Y: s.Supplies[0].Y, R: 80})
	processSupplies(s, s.StartAt+3)
	if len(s.Supplies) != 0 {
		t.Fatal("unsupported crate remains")
	}
}
func TestSupplyBotUsesPublicMoveAndRejectsCorruptSave(t *testing.T) {
	s := supplyState(t)
	s.Tanks[0].Items = nil
	s.Supplies = []Supply{{ID: 1, ItemID: 0, X: s.Tanks[0].X + 35, Y: s.Tanks[0].Y, FromX: s.Tanks[0].X + 35, At: s.StartAt, DropAt: s.StartAt + 1, LandAt: s.StartAt + 2, Landed: true}}
	at := time.UnixMilli(s.StartAt + 1000)
	dir := botSupplyMove(s, 0, at.UnixMilli())
	if dir != 1 {
		t.Fatalf("bot doesn't seek reachable crate: %d", dir)
	}
	m, _ := s.envelope()
	raw, _ := json.Marshal(Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: dir})
	next, err := apply(ctx(2, 0, at), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := decode(next)
	if got.Tanks[0].X <= s.Tanks[0].X {
		t.Fatal("bot movement bypassed public rules")
	}
	s.Supplies[0].ItemID = 999999
	m, _ = s.envelope()
	if _, err = decode(m); err == nil {
		t.Fatal("unknown crate content accepted")
	}
}

func TestSupplyBotWaitsForVisibleContactBeforeFiring(t *testing.T) {
	s := supplyState(t)
	// Arrange a genuinely level native surface; a box at a different elevation
	// is correctly removed as unsupported, so cannot test contact timing.
	stage, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(stage, s.Craters)
	for x := 20.0; x < float64(stage.Width)-70; x += 4 {
		y := groundY(stage, mask, x, 0)
		flat := y < float64(stage.Height)
		for dx := 0.0; dx <= 40; dx += 4 {
			if groundY(stage, mask, x+dx, 0) != y {
				flat = false
				break
			}
		}
		if flat {
			s.Tanks[0].X = x
			s.Tanks[0].Y = y
			break
		}
	}

	s.Tanks[0].Items = nil
	s.Tanks[1].Items = nil
	at := s.StartAt + 1000
	s.Supplies = []Supply{{ID: 1, ItemID: 0, X: s.Tanks[0].X + 35, Y: s.Tanks[0].Y, FromX: s.Tanks[0].X + 35, At: s.StartAt, DropAt: s.StartAt + 1, LandAt: s.StartAt + 2, Landed: true}}
	m, _ := s.envelope()
	for step := 0; step < 20; step++ {
		now := time.UnixMilli(at + int64(step*65))
		var err error
		if m.SystemDeadline > 0 && m.SystemDeadline <= now.UnixMilli() {
			m, err = onTimer(ctx(2, 0, now), m)
			if err != nil {
				t.Fatal(err)
			}
		}
		current, _ := decode(m)
		if len(current.Tanks[0].Items) == 1 {
			return
		}
		action, err := botAction(ctx(2, 0, now), m)
		if err != nil {
			t.Fatal(err)
		}
		if action == nil {
			continue
		}
		var a Action
		_ = json.Unmarshal(action, &a)
		if a.Type == "fire" {
			t.Fatal("bot fired instead of allowing visible pickup to finish")
		}
		m, err = apply(ctx(2, 0, now), m, action)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("bot never picked up reachable supply")
}

func TestSupplyDeliveryLocksTurnAndLateTimerGrantsFullDeadline(t *testing.T) {
	s := supplyState(t)
	s.NextSupplyTurn = s.Turn + 1
	advanceTurn(s, time.UnixMilli(s.StartAt+1000))
	m, _ := s.envelope()
	if len(m.Actors) != 0 || m.Deadline != 0 || s.Deadline != 0 {
		t.Fatal("transport consumes next player's action time")
	}
	if s.canAct(s.Current) {
		t.Fatal("player can act during delivery")
	}
	late := time.UnixMilli(s.Supplies[0].LandAt + 10000)
	next, err := onTimer(ctx(2, 0, late), m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decode(next)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deadline != late.Add(time.Duration(got.Options.TurnSeconds)*time.Second).UnixMilli() || len(next.Actors) != 1 {
		t.Fatal("late delivery timer shortened next turn")
	}
}

func TestSupplyDeadlineCrossesSuddenAndMatchEndWithoutPastTimer(t *testing.T) {
	for _, ending := range []bool{false, true} {
		s := supplyState(t)
		at := s.StartAt + 1000
		s.NextSupplyTurn = s.Turn + 1
		if ending {
			s.MatchEndsAt = at + 100
		} else {
			s.SuddenAt = at + 100
		}
		advanceTurn(s, time.UnixMilli(at))
		m, _ := s.envelope()
		land := s.Supplies[0].LandAt
		if m.SystemDeadline != land {
			t.Fatalf("delivery must finish its timeline before global transition: %d != %d", m.SystemDeadline, land)
		}
		next, err := onTimer(ctx(2, 0, time.UnixMilli(land+100)), m)
		if err != nil {
			t.Fatal(err)
		}
		if next.SystemDeadline > 0 && next.SystemDeadline <= land+100 {
			t.Fatal("past timer loop")
		}
		got, err := decode(next)
		if err != nil {
			t.Fatal(err)
		}
		if ending && !got.Finished {
			t.Fatal("match end not applied after landing")
		}
		if !ending && !got.Sudden {
			t.Fatal("sudden transition lost")
		}
	}
}
