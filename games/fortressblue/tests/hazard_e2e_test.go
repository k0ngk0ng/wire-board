package tests

// Failures declared before hazard implementation: carrying fire/poison as a
// personal three-turn status; per-seat acceleration; damage after leaving or
// across stacked floors; floating fire after support is destroyed; a flying
// shot overwriting hazard HP/death; restore resetting tick/expiry; dead actor
// still offered input; missing release-to-launch preparation phase.
import (
	"encoding/json"
	"fmt"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

type hazardWire struct {
	ID                    int64
	Kind                  string
	X, Y, R               float64
	At, EndAt, NextTickAt int64
	Source                int
}

func hazards(t *testing.T, m gameapi.Match) []hazardWire {
	t.Helper()
	var w struct{ Hazards []hazardWire }
	if e := json.Unmarshal(m.Data, &w); e != nil {
		t.Fatal(e)
	}
	return w.Hazards
}
func withHazards(t *testing.T, m gameapi.Match, h []hazardWire) gameapi.Match {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(m.Data, &v); e != nil {
		t.Fatal(e)
	}
	v["hazards"] = h
	m.Data, _ = json.Marshal(v)
	return m
}
func hazardAt(now time.Time, kind string, source int, x, y, r float64) hazardWire {
	return hazardWire{ID: 1, Kind: kind, X: x, Y: y, R: r, At: now.UnixMilli(), EndAt: now.Add(4 * time.Second).UnixMilli(), NextTickAt: now.Add(time.Second).UnixMilli(), Source: source}
}
func TestGroundHazardsHaveAbsoluteTicksAndPersist(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8} {
		t.Run(string(rune('0'+players)), func(t *testing.T) {
			mod, m, now := liveFixture(t, players, false)
			s, _ := timeline(t, m)
			before := s.Tanks[0].HP
			h := hazardAt(now, "fire", players/2, 300, 808, 40)
			m = withHazards(t, m, []hazardWire{h})
			n := timerAt(t, mod, m, h.NextTickAt-1)
			got := hazards(t, n)
			if len(got) != 1 || got[0].NextTickAt != h.NextTickAt {
				t.Fatal("hazard vanished or tick reset")
			}
			n = timerAt(t, mod, n, h.NextTickAt)
			s, _ = timeline(t, n)
			if s.Tanks[0].HP != before-18 {
				t.Fatalf("%d players ticked wrong amount: %d", players, s.Tanks[0].HP)
			}
			got = hazards(t, n)
			if got[0].NextTickAt != h.NextTickAt+1000 || got[0].EndAt != h.EndAt {
				t.Fatal("tick/expiry changed with player count")
			}
			restored, e := mod.Restore(gameapi.Context{Players: players, Actor: 0, Now: time.UnixMilli(h.NextTickAt + 500)}, n)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(hazards(t, restored), got) {
				t.Fatal("restore restarted hazard")
			}
			duplicate := timerAt(t, mod, n, h.NextTickAt)
			ds, _ := timeline(t, duplicate)
			if ds.Tanks[0].HP != s.Tanks[0].HP {
				t.Fatal("duplicate hazard tick")
			}
			n = timerAt(t, mod, n, h.EndAt)
			if len(hazards(t, n)) != 0 {
				t.Fatal("expired hazard remained")
			}
		})
	}
}
func TestLeavingHazardOrDifferentFloorStopsDamage(t *testing.T) {
	mod, m, now := liveFixture(t, 4, false)
	s, _ := timeline(t, m)
	s.Tanks[1].X, s.Tanks[1].Y = 300, 600
	before := s
	m.Data, _ = json.Marshal(s)
	h := hazardAt(now, "poison", 2, 300, 808, 6)
	m = withHazards(t, m, []hazardWire{h})
	raw, _ := json.Marshal(fb.Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: 1})
	n, e := mod.Apply(gameapi.Context{Players: 4, Actor: 0, Now: now}, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = timeline(t, n)
	if s.Tanks[0].X <= 306 {
		t.Fatal("fixture did not leave cloud")
	}
	n = timerAt(t, mod, n, h.NextTickAt)
	s, _ = timeline(t, n)
	if len(hazards(t, n)) != 1 {
		t.Fatal("hazard disappeared instead of remaining at its origin")
	}
	if s.Tanks[0].HP != before.Tanks[0].HP || s.Tanks[1].HP != before.Tanks[1].HP {
		t.Fatal("hazard followed a tank or damaged a different floor")
	}
}
func TestDestroyedSupportExtinguishesGroundHazard(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.MapID = "stage-4"
	s.Options.Map = "stage-4"
	s.Tanks[0].X, s.Tanks[0].Y = 520, 272
	s.Craters = []fb.Crater{{X: 520, Y: 272, R: 80, Seat: 1}}
	m.Data, _ = json.Marshal(s)
	h := hazardAt(now, "fire", 1, 520, 272, 40)
	other := hazardAt(now, "fire", 1, 1000, 272, 40)
	other.ID = 2
	m = withHazards(t, m, []hazardWire{h, other})
	n := timerAt(t, mod, m, h.NextTickAt)
	if len(hazards(t, n)) != 1 || hazards(t, n)[0].ID != 2 {
		t.Fatal("fire floated after support was excavated")
	}
	got, _ := timeline(t, n)
	if got.Tanks[0].HP != s.Tanks[0].HP {
		t.Fatal("unsupported fire damaged tank")
	}
}
func TestHazardDamageDuringFlightIsNotOverwritten(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	h := hazardAt(now, "fire", 1, 300, 808, 40)
	m = withHazards(t, m, []hazardWire{h})
	raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 90, Power: 100, Facing: 1, Weapon: "primary"})
	n, e := mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	_, w := timeline(t, n)
	if w.Resolution == nil || w.Resolution.ImpactAt <= h.NextTickAt {
		t.Fatal("fixture does not overlap flight/tick")
	}
	n = timerAt(t, mod, n, h.NextTickAt)
	during, _ := timeline(t, n)
	if during.Tanks[0].HP != s.Tanks[0].HP-18 {
		t.Fatal("hazard paused during flight")
	}
	for i := 0; i < 30; i++ {
		_, w = timeline(t, n)
		if w.Resolution == nil {
			break
		}
		n = timerAt(t, mod, n, n.SystemDeadline)
	}
	after, _ := timeline(t, n)
	if after.Tanks[0].HP > during.Tanks[0].HP {
		t.Fatal("shot overwrote hazard damage")
	}
}
func TestHazardDeathNeverLeavesADeadActor(t *testing.T) {
	mod, m, now := liveFixture(t, 4, false)
	s, _ := timeline(t, m)
	s.Tanks[0].HP = 10
	m.Data, _ = json.Marshal(s)
	h := hazardAt(now, "poison", 2, 300, 808, 40)
	m = withHazards(t, m, []hazardWire{h})
	n := timerAt(t, mod, m, h.NextTickAt)
	s, _ = timeline(t, n)
	if s.Tanks[0].Alive || s.Current == 0 || len(n.Actors) != 1 || n.Actors[0] == 0 {
		t.Fatal("hazard-killed actor retained turn")
	}
}
func TestSpecialFireCreatesTerrainHazardNotCarriedStatus(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 0, Power: 30, Facing: 1, Weapon: "special"})
	n, e := mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	_, w := timeline(t, n)
	n = timerAt(t, mod, n, w.Resolution.ImpactAt)
	got := hazards(t, n)
	if len(got) != 1 || got[0].Kind != "fire" || got[0].NextTickAt != got[0].At+1000 {
		t.Fatal("fire shell omitted grounded hazard")
	}
	after, _ := timeline(t, n)
	for _, status := range after.Tanks[1].Statuses {
		if status.Kind == "burn" {
			t.Fatal("fire attached a carried three-turn status")
		}
	}
}
func TestProjectileWaitsForSourceLaunchFrame(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	n := fireFixture(t, mod, m, now)
	s, _ := timeline(t, n)
	var wire struct {
		LastShot struct {
			At, LaunchAt, ImpactAt int64
			Arcs                   []struct{ PathTimes []float64 }
		}
	}
	_ = json.Unmarshal(n.Data, &wire)
	shot := wire.LastShot
	if shot.LaunchAt != shot.At+334 || shot.ImpactAt <= shot.LaunchAt || len(shot.Arcs) == 0 || shot.Arcs[0].PathTimes[0] != 334 {
		t.Fatal("server omitted source-frame windup")
	}
	for _, ev := range s.Events {
		if ev.Type == "shot" {
			t.Fatal("projectile/audio event before launch frame")
		}
	}
	early := timerAt(t, mod, n, shot.LaunchAt-1)
	if !reflect.DeepEqual(n.Data, early.Data) {
		t.Fatal("pre-launch timer changed state")
	}
	launched := timerAt(t, mod, n, shot.LaunchAt)
	s, _ = timeline(t, launched)
	count := 0
	for _, ev := range s.Events {
		if ev.Type == "shot" && ev.At == shot.LaunchAt {
			count++
		}
	}
	if count != 1 || len(launched.Actors) != 0 {
		t.Fatal("launch not published once while actors locked")
	}
	again := timerAt(t, mod, launched, shot.LaunchAt)
	s, _ = timeline(t, again)
	count = 0
	for _, ev := range s.Events {
		if ev.Type == "shot" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("duplicate launch")
	}
}

func TestHazardUsesInFlightFallPositionAndPublishesDeathThere(t *testing.T) {
	for _, lethal := range []bool{false, true} {
		mod, m, now := liveFixture(t, 4, false)
		s, _ := timeline(t, m)
		endY := 1008.0
		tickAt := now.Add(time.Second).UnixMilli()
		h := hazardAt(now, "fire", 2, 300, 808, 40)
		if lethal {
			endY = 848
			s.Tanks[0].HP = 10
			s.Craters = []fb.Crater{{X: 300, Y: 808, R: 40, Seat: 2}}
			h = hazardAt(now.Add(500*time.Millisecond), "fire", 2, 300, 848, 40)
			tickAt = h.NextTickAt
		}
		s.Resolution = &fb.Resolution{Stage: "settling", ShotID: 0, At: now.UnixMilli(), ImpactAt: now.UnixMilli(), SettleAt: now.Add(2 * time.Second).UnixMilli(), Falls: []fb.Fall{{Seat: 0, FromX: 300, FromY: 808, ToX: 300, ToY: endY, At: now.UnixMilli(), EndAt: now.Add(2 * time.Second).UnixMilli()}}}
		s.Deadline = 0
		s.MoveRemainingMS = 10000
		m.Actors = nil
		m.TimeoutActors = nil
		m.Attention = nil
		m.Deadline = 0
		m.Data, _ = json.Marshal(s)
		m = withHazards(t, m, []hazardWire{h})
		n := timerAt(t, mod, m, tickAt)
		got, _ := timeline(t, n)
		if !lethal {
			if got.Tanks[0].HP != s.Tanks[0].HP {
				t.Fatal("hazard damaged old fall origin after visual departure")
			}
		} else {
			want := 808 + (848-808)*.75*.75
			if got.Tanks[0].Alive || got.Tanks[0].HP != 0 || got.Tanks[0].Y != want {
				t.Fatalf("fall death not published at quadratic timeline position: %+v wantY=%v", got.Tanks[0], want)
			}
		}
	}
}

func TestBotLeavesGroundFireThroughLegalMoves(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.Tanks[0].Items = nil
	m.Data, _ = json.Marshal(s)
	h := hazardAt(now, "fire", 1, 300, 808, 40)
	m = withHazards(t, m, []hazardWire{h})
	for step := 0; step < 6; step++ {
		s, _ = timeline(t, m)
		if s.Tanks[0].X < h.X-h.R || s.Tanks[0].X > h.X+h.R {
			return
		}
		raw, e := mod.BotAction(gameapi.Context{Players: 2, Actor: 0, Now: now}, m)
		if e != nil {
			t.Fatal(e)
		}
		var action fb.Action
		_ = json.Unmarshal(raw, &action)
		if action.Type != "move" {
			t.Fatalf("bot remained in damaging ground fire: %s", raw)
		}
		m, e = mod.Apply(gameapi.Context{Players: 2, Actor: 0, Now: now}, m, raw)
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("bot exhausted movement without leaving fire")
}

func TestSimultaneousLandingAndHazardDeathsDraw(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.Tanks[0].HP, s.Tanks[1].HP = 1, 1
	end := now.Add(2 * time.Second).UnixMilli()
	s.Resolution = &fb.Resolution{Stage: "settling", ShotID: 0, At: now.UnixMilli(), ImpactAt: now.UnixMilli(), SettleAt: end, Falls: []fb.Fall{{Seat: 0, FromX: 300, FromY: 808, ToX: 300, ToY: 900, At: now.UnixMilli(), EndAt: end}}}
	s.Deadline = 0
	s.MoveRemainingMS = 10000
	m.Actors = nil
	m.Attention = nil
	m.TimeoutActors = nil
	m.Deadline = 0
	m.Data, _ = json.Marshal(s)
	h := hazardAt(now.Add(time.Second), "fire", 0, 430, 816, 40)
	m = withHazards(t, m, []hazardWire{h})
	n := timerAt(t, mod, m, end)
	if n.Result == nil || !n.Result.Draw || len(n.Result.Winners) != 0 {
		t.Fatalf("same-timestamp mutual elimination declared an early winner: %+v", n.Result)
	}
}

// Preparation is interruptible by death. The instant of launch belongs to the
// launched branch, because the shared scheduler processes launch before a
// hazard at the same timestamp. Already airborne shells continue after death.
func TestDeathCancelsOnlyUnlaunchedShot(t *testing.T) {
	for _, offset := range []int64{100, 334, 400} {
		t.Run(fmt.Sprint(offset), func(t *testing.T) {
			mod, m, now := liveFixture(t, 4, false)
			s, _ := timeline(t, m)
			s.Tanks[0].HP = 10
			m.Data, _ = json.Marshal(s)
			h := hazardAt(now.Add(time.Duration(offset-1000)*time.Millisecond), "fire", 2, 300, 808, 40)
			m = withHazards(t, m, []hazardWire{h})
			raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 45, Power: 70, Facing: 1, Weapon: "primary"})
			n, e := mod.Apply(gameapi.Context{Players: 4, Actor: 0, Now: now}, m, raw)
			if e != nil {
				t.Fatal(e)
			}
			before, w := timeline(t, n)
			impact := w.Resolution.ImpactAt
			n = timerAt(t, mod, n, h.NextTickAt)
			after, w := timeline(t, n)
			if after.Tanks[0].Alive {
				t.Fatal("fixture shooter did not die")
			}
			var wire struct{ LastShot struct{ CancelledAt int64 } }
			_ = json.Unmarshal(n.Data, &wire)
			if offset < 334 {
				if wire.LastShot.CancelledAt != h.NextTickAt || w.Resolution != nil || after.Current == 0 {
					t.Fatal("pre-launch death did not cancel and release dead actor")
				}
				for _, event := range after.Events {
					if event.Type == "shot" || event.Type == "explode" {
						t.Fatal("unlaunched dead shooter produced a projectile")
					}
				}
				n = timerAt(t, mod, n, impact)
				after, _ = timeline(t, n)
				if len(after.Craters) != len(before.Craters) {
					t.Fatal("cancelled shot excavated terrain later")
				}
			} else {
				if wire.LastShot.CancelledAt != 0 || w.Resolution == nil {
					t.Fatal("already launched shot was cancelled")
				}
				found := false
				for _, event := range after.Events {
					if event.Type == "shot" {
						found = true
					}
				}
				if !found {
					t.Fatal("launched projectile event missing")
				}
				n = timerAt(t, mod, n, impact)
				after, _ = timeline(t, n)
				if len(after.LastShot.Blasts) == 0 && !after.LastShot.Arcs[0].OutOfBounds {
					t.Fatal("airborne shot disappeared after shooter death")
				}
			}
		})
	}
}
