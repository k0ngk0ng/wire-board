package tests

// Failure cases established before implementation: immediate damage/terrain/result;
// next actor while projectile flies; lost or replayed stages on restore/reclaim;
// early landing HP/death; overwritten range targets; duplicate timers; stale legacy
// shots replayed; delayed timer consuming the next actor's actionable time.
import (
	"encoding/json"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

type resolutionWire struct {
	Stage                  string `json:"stage"`
	ShotID                 int64  `json:"shotId"`
	At, ImpactAt, SettleAt int64
	Falls                  []struct {
		Seat                   int
		FromX, FromY, ToX, ToY float64
		At, EndAt              int64
	}
}
type timelineWire struct {
	Resolution *resolutionWire
	LastShot   *struct {
		ID, At, ImpactAt, SettleAt int64
		Arcs                       []struct {
			Path        []fb.Point
			PathTimes   []float64
			ImpactAt    int64
			OutOfBounds bool
		}
		Blasts []struct {
			Hits []struct{ Victim, Damage int }
		}
	}
}

func timeline(t *testing.T, m gameapi.Match) (fb.State, timelineWire) {
	t.Helper()
	var s fb.State
	var w timelineWire
	if e := json.Unmarshal(m.Data, &s); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(m.Data, &w); e != nil {
		t.Fatal(e)
	}
	return s, w
}
func liveFixture(t *testing.T, players int, lethal bool) (gameapi.Module, gameapi.Match, time.Time) {
	t.Helper()
	mod := fb.Module()
	now := time.Unix(1800000000, 0)
	m, e := mod.Start(gameapi.Context{Players: players, Now: now}, json.RawMessage(`{"map":"stage-16","turnSeconds":20}`))
	if e != nil {
		t.Fatal(e)
	}
	s, _ := timeline(t, m)
	for i := 0; i < players; i++ {
		a, _ := json.Marshal(fb.Action{Type: "select", Epoch: s.Epoch, Tank: 1})
		m, e = mod.Apply(gameapi.Context{Players: players, Actor: i, Now: now}, m, a)
		if e != nil {
			t.Fatal(e)
		}
	}
	s, _ = timeline(t, m)
	s.Wind = 0
	s.Tanks[0].X = 300
	s.Tanks[0].Y = 808
	for i := 1; i < players/2; i++ {
		s.Tanks[i].X = 1000 + float64(i)*30
		s.Tanks[i].Y = 780
	}
	for i := players / 2; i < players; i++ {
		s.Tanks[i].X = 430 + float64(i-players/2)*25
		s.Tanks[i].Y = 816
		if lethal {
			s.Tanks[i].HP = 1
		}
	}
	m.Data, _ = json.Marshal(s)
	return mod, m, now.Add(time.Second)
}
func fireFixture(t *testing.T, mod gameapi.Module, m gameapi.Match, now time.Time) gameapi.Match {
	t.Helper()
	s, _ := timeline(t, m)
	a, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 0, Power: 30, Facing: 1, Weapon: "primary"})
	n, e := mod.Apply(gameapi.Context{Players: len(s.Tanks), Actor: s.Current, Now: now}, m, a)
	if e != nil {
		t.Fatal(e)
	}
	return n
}
func timerAt(t *testing.T, mod gameapi.Module, m gameapi.Match, at int64) gameapi.Match {
	t.Helper()
	s, _ := timeline(t, m)
	n, e := mod.OnTimer(gameapi.Context{Players: len(s.Tanks), Actor: -1, Now: time.UnixMilli(at)}, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = n.Validate(len(s.Tanks)); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestShotTimelinePublicModule(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8} {
		t.Run(string(rune('0'+players)), func(t *testing.T) {
			mod, m, now := liveFixture(t, players, false)
			before, _ := timeline(t, m)
			n := fireFixture(t, mod, m, now)
			s, w := timeline(t, n)
			if !reflect.DeepEqual(before.Craters, s.Craters) || before.Turn != s.Turn || n.Result != nil {
				t.Fatal("fire resolved terrain/turn/result before visual impact")
			}
			for i := range s.Tanks {
				if s.Tanks[i].HP != before.Tanks[i].HP || s.Tanks[i].Y != before.Tanks[i].Y {
					t.Fatalf("fire changed target %d before impact", i)
				}
			}
			if w.Resolution == nil || w.Resolution.Stage != "flying" || len(n.Actors) != 0 || n.Deadline != 0 || w.Resolution.ImpactAt <= now.UnixMilli() {
				t.Fatalf("missing non-actionable flight: %+v actors=%v", w.Resolution, n.Actors)
			}
			if w.LastShot == nil || w.LastShot.ImpactAt != w.Resolution.ImpactAt || len(w.LastShot.Arcs[0].PathTimes) != len(w.LastShot.Arcs[0].Path) {
				t.Fatal("shot lacks server timeline")
			}
			impact := w.Resolution.ImpactAt
			for _, seat := range []int{0, players / 2} {
				a, _ := json.Marshal(fb.Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: 1})
				if _, e := mod.Apply(gameapi.Context{Players: players, Actor: seat, Now: now}, n, a); e == nil {
					t.Fatal("accepted action during flight")
				}
				a, e := mod.BotAction(gameapi.Context{Players: players, Actor: seat, Now: now}, n)
				if e != nil || a != nil {
					t.Fatalf("bot acted during flight: %s %v", a, e)
				}
			}
			for _, fn := range []func(gameapi.Context, gameapi.Match) (gameapi.Match, error){mod.Restore, mod.Reclaim} {
				restored, e := fn(gameapi.Context{Players: players, Actor: 0, Now: time.UnixMilli(impact - 1)}, n)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(n.Data, restored.Data) || n.SystemDeadline != restored.SystemDeadline {
					t.Fatal("restore/reclaim changed in-flight timeline")
				}
			}
			early := timerAt(t, mod, n, now.UnixMilli()+333)
			if !reflect.DeepEqual(n.Data, early.Data) {
				t.Fatal("early timer advanced flight")
			}
			n = timerAt(t, mod, n, impact)
			s, w = timeline(t, n)
			if w.Resolution == nil || w.Resolution.Stage != "settling" || len(n.Actors) != 0 {
				t.Fatal("impact omitted locked settling stage")
			}
			if s.Tanks[players/2].HP >= before.Tanks[players/2].HP || s.Tanks[players/2].Y != before.Tanks[players/2].Y {
				t.Fatal("impact damage/position wrong")
			}
			if len(s.Craters) == 0 {
				t.Fatal("impact did not carve")
			}
			if s.Tanks[players/2].Damage != 0 {
				t.Fatal("victim received inflicted-damage credit")
			}
			hitSeats := map[int]bool{}
			for _, ev := range s.Events {
				if ev.Type == "hit" && ev.At == impact {
					hitSeats[ev.Target] = true
				}
			}
			for _, b := range w.LastShot.Blasts {
				for _, h := range b.Hits {
					if !hitSeats[h.Victim] {
						t.Fatal("blast missing per-victim event")
					}
				}
			}
			for _, f := range w.Resolution.Falls {
				if f.EndAt <= f.At || f.FromY == f.ToY {
					t.Fatal("invalid public falling path")
				}
			}
			for _, fn := range []func(gameapi.Context, gameapi.Match) (gameapi.Match, error){mod.Restore, mod.Reclaim} {
				r, e := fn(gameapi.Context{Players: players, Actor: 0, Now: time.UnixMilli(impact + 1)}, n)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(n.Data, r.Data) {
					t.Fatal("restore changed settling")
				}
			}
			duplicate := timerAt(t, mod, n, impact)
			if !reflect.DeepEqual(n.Data, duplicate.Data) {
				t.Fatal("duplicate timer damaged twice")
			}
			late := w.Resolution.SettleAt + 1500
			n = timerAt(t, mod, n, late)
			s, w = timeline(t, n)
			if w.Resolution != nil || s.Turn != before.Turn+1 || s.Deadline != late+20000 || len(n.Actors) != 1 {
				t.Fatalf("next actor did not receive full actionable time: turn=%d deadline=%d late=%d", s.Turn, s.Deadline, late)
			}
			t.Logf("players=%d shot=%d impact=%d completed=%d nextDeadline=%d", players, s.LastShot.ID, impact, late, s.Deadline)
		})
	}
}
func TestLethalShotDoesNotFinishBeforeLanding(t *testing.T) {
	mod, m, now := liveFixture(t, 2, true)
	n := fireFixture(t, mod, m, now)
	_, w := timeline(t, n)
	if n.Result != nil || w.Resolution == nil {
		t.Fatal("lethal shot finished synchronously")
	}
	n = timerAt(t, mod, n, w.Resolution.ImpactAt)
	_, w = timeline(t, n)
	if n.Result != nil || w.Resolution == nil {
		t.Fatal("lethal impact skipped settling")
	}
	n = timerAt(t, mod, n, w.Resolution.SettleAt)
	if n.Result == nil || !n.Result.Team {
		t.Fatal("resolved lethal shot missing team result")
	}
}
func TestLegacyShotRestoreDoesNotReplayOrExtendDeadline(t *testing.T) {
	mod, m, now := liveFixture(t, 2, false)
	s, _ := timeline(t, m)
	s.LastShot = &fb.Shot{ID: 9, At: now.Add(-time.Minute).UnixMilli(), Arcs: []fb.Arc{{Path: []fb.Point{{X: 100, Y: 100}}, Impact: fb.Point{X: 100, Y: 100}, Hit: -1}}}
	m.Data, _ = json.Marshal(s)
	for _, fn := range []func(gameapi.Context, gameapi.Match) (gameapi.Match, error){mod.Restore, mod.Reclaim} {
		n, e := fn(gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(s.Deadline + 100)}, m)
		if e != nil {
			t.Fatal(e)
		}
		got, w := timeline(t, n)
		if w.Resolution != nil || got.Deadline != s.Deadline || got.LastShot.ID != 9 {
			t.Fatal("legacy restore/reclaim replayed shot or extended deadline")
		}
	}
}

// Additional end-to-end cases exercise the same predeclared falling/restore
// scenarios through the public module, including a surviving teammate.
func TestFallingMovePublishesDamageAtLandingAndAdvancesDeadActor(t *testing.T) {
	mod, m, now := liveFixture(t, 4, false)
	s, _ := timeline(t, m)
	s.Tanks[0].X, s.Tanks[0].Y, s.Tanks[0].HP = 300, 700, 1
	m.Data, _ = json.Marshal(s)
	action, _ := json.Marshal(fb.Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: 1})
	next, err := mod.Apply(gameapi.Context{Players: 4, Actor: 0, Now: now}, m, action)
	if err != nil {
		t.Fatal(err)
	}
	current, w := timeline(t, next)
	if w.Resolution == nil || w.Resolution.Stage != "settling" || current.Tanks[0].Y != 700 || current.Tanks[0].HP != 1 || len(next.Actors) != 0 {
		t.Fatal("move leaked landing result")
	}
	var landing int64
	for _, f := range w.Resolution.Falls {
		if f.Seat == 0 {
			landing = f.EndAt
		}
	}
	if landing == 0 {
		t.Fatal("missing actor fall")
	}
	next = timerAt(t, mod, next, landing-1)
	current, _ = timeline(t, next)
	if current.Tanks[0].Y != 700 || current.Tanks[0].HP != 1 || !current.Tanks[0].Alive {
		t.Fatal("damage before landing")
	}
	next = timerAt(t, mod, next, landing)
	current, _ = timeline(t, next)
	if current.Tanks[0].Alive || current.Tanks[0].HP != 0 || current.Tanks[0].Y <= 700 {
		t.Fatal("landing did not publish position and death together")
	}
	for i := 0; i < 12; i++ {
		_, w = timeline(t, next)
		if w.Resolution == nil {
			break
		}
		next = timerAt(t, mod, next, next.SystemDeadline)
	}
	current, w = timeline(t, next)
	if w.Resolution != nil || current.Current == 0 || len(next.Actors) != 1 || next.Actors[0] == 0 {
		t.Fatal("dead moving actor retained turn")
	}
	if _, err = mod.Restore(gameapi.Context{Players: 4, Actor: -1, Now: time.UnixMilli(current.Deadline - 1)}, next); err != nil {
		t.Fatal(err)
	}
}
