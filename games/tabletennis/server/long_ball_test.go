package tabletennis

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestLongBallCanBeReturnedInVisibleNearCourt(t *testing.T) {
	for _, seat := range []int{0, 1} {
		dir := sign(seat)
		from, to := 1-seat, seat
		for _, landing := range []float64{.45, 1.3} {
			p := Vec{0, 1.05, -dir * 1.1}
			duration := .4
			vz := dir * (landing + 1.1) / (duration - .06*duration*duration)
			a := acceleration(Vec{0, 0, vz}, .9)
			vy := (.78 - p.Y - .5*a.Y*duration*duration) / duration
			flight := traceFlight(p, Vec{0, vy, vz}, .9, 1000, from, to, false, false)
			if flight.ReceiveAt == 0 {
				t.Fatal("fixture did not land legally", flight.Reason)
			}
			t.Logf("seat%d landing%.2f first bounce%d window%dms", seat, landing, flight.ReceiveAt, flight.ContactEnd-flight.ContactStart)
			at := flight.ReceiveAt
			for at < flight.EndAt {
				q, _ := flight.at(at)
				if q.Z*dir >= 2.05 {
					break
				}
				at++
			}
			point, _ := flight.at(at)
			if at >= flight.EndAt {
				t.Fatal("near-court fixture already ended")
			}
			s := fixture(t, 2)
			s.Phase = "rally"
			s.ServerAt = at
			s.Expected = seat
			s.Receiver = seat
			s.Flight = &flight
			m, _ := s.match()
			height := point.Y
			action := Action{Type: "swing", Epoch: s.Epoch, Rally: s.Rally, Shot: s.Shot, Seq: 1, Kind: "block", Power: .35, RacketX: point.X * dir / .9, RacketY: &height, RacketZ: point.Z * dir, At: at}
			raw, _ := json.Marshal(action)
			next, e := Module().Apply(gameapi.Context{Players: 2, Actor: seat, Now: time.UnixMilli(at)}, m, raw)
			if e != nil {
				t.Errorf("visible near-court ball rejected at z%.3f: %v", point.Z*dir, e)
				continue
			}
			after, _ := decode(next)
			if after.Shot != s.Shot+1 {
				t.Fatal("near-court return did not hit")
			}
		}
	}
}

func TestBotPreparationStaysInsideLongBallReach(t *testing.T) {
	s := fixture(t, 2)
	base := s.ServerAt
	s.Phase = "rally"
	s.Expected = 0
	s.Options.Difficulty = "easy"
	s.Flight = &Flight{StartAt: base, ReceiveAt: base + 800, ContactStart: base + 801, ContactEnd: base + 825, EndAt: base + 2000, Segments: []Segment{{At: base + 800, Until: base + 2000, P: Vec{0, .9, 2}, V: Vec{0, 0, 10}}}}
	m, _ := s.match()
	for i := int64(100); i < 800; i += 100 {
		ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(base + i)}
		raw, e := Module().BotAction(ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		if len(raw) == 0 {
			continue
		}
		var a Action
		json.Unmarshal(raw, &a)
		if a.Type == "move" && a.RacketZ > ReachDepth {
			t.Fatalf("bot chased the ball beyond the reachable corridor: %.3f", a.RacketZ)
		}
		m, e = Module().Apply(ctx, m, raw)
		if e != nil {
			t.Fatal(e)
		}
	}
}
