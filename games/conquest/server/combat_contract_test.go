package conquest

// Original-JAR probe expectations were recorded before implementing resolution.
// The test fixture enters combat through the same planning/submission/timer
// contract as a real match; it never invokes a private damage function.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestConquestOriginalCombatProbeE2E(t *testing.T) {
	cases := []struct {
		name                      string
		sent, defender            int
		fort, town, king, travels bool
		capture                   bool
		left                      int
	}{
		{"plain", 1000, 500, false, false, false, false, true, 506},
		{"equal", 1000, 1000, false, false, false, false, false, 0},
		{"fort", 1000, 1000, true, false, false, false, false, 450},
		{"combined", 4000, 1000, true, true, true, false, true, 1421},
		{"king_failed", 1000, 2000, false, false, false, true, false, 710},
		{"empty_fort", 1000, 0, true, false, false, false, true, 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0)}
			mod := Module()
			options, _ := mod.Configure(2, raw(Options{MapID: "europe", Difficulty: "normal", TurnSeconds: 120}))
			m, e := mod.Start(c, options)
			if e != nil {
				t.Fatal(e)
			}
			s, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			g, _ := mapFor(s.MapID)
			from := s.Players[0].King
			source := &s.Provinces[g.index[from]]
			var to string
			for _, p := range g.data.Provinces {
				if len(p.Neighbors) > 0 {
					from = p.ID
					to = p.Neighbors[0]
					break
				}
			}
			source = &s.Provinces[g.index[from]]
			source.Owner = 0
			source.Troops = tc.sent + 1
			source.Mobile = source.Troops
			s.Players[0].King = from
			s.Players[0].Gold = 100000
			target := &s.Provinces[g.index[to]]
			target.Owner = -1
			target.Troops = tc.defender
			target.Mobile = tc.defender
			target.Fort = tc.fort
			target.Town = tc.town
			if tc.king {
				target.Owner = 1
				s.Players[1].King = to
			}
			s.refresh()
			m = s.envelope()
			action := map[string]any{"type": "move", "session": s.Session, "epoch": s.Epoch, "from": from, "to": to, "amount": tc.sent, "king": tc.travels}
			m, e = mod.Apply(c, m, raw(action))
			if e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				c.Actor = i
				m, e = mod.Apply(c, m, raw(map[string]any{"type": "submit", "session": s.Session, "epoch": s.Epoch}))
				if e != nil {
					t.Fatal(e)
				}
			}
			before, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			if before.Provinces[g.index[to]].Troops != tc.defender {
				t.Fatal("damage happened before visual arrival")
			}
			if m.SystemDeadline <= c.Now.UnixMilli() {
				t.Fatal("missing flight interval")
			}
			early, e := mod.OnTimer(c, m)
			if e != nil {
				t.Fatal(e)
			}
			if string(early.Data) != string(m.Data) {
				t.Fatal("early timer changed combat")
			}
			for i := 0; i < 20 && m.SystemDeadline > 0; i++ {
				c.Now = time.UnixMilli(m.SystemDeadline)
				m, e = mod.OnTimer(c, m)
				if e != nil {
					t.Fatal(e)
				}
			}
			var end State
			if e = json.Unmarshal(m.Data, &end); e != nil {
				t.Fatal(e)
			}
			got := end.Provinces[g.index[to]]
			if got.Troops != tc.left || ((got.Owner == 0) != tc.capture) {
				t.Fatalf("got owner=%d troops=%d want capture=%v troops=%d", got.Owner, got.Troops, tc.capture, tc.left)
			}
			if tc.capture && tc.defender > 0 && got.Fort {
				t.Fatal("occupied defended fort survived")
			}
			if tc.capture && tc.defender == 0 && tc.fort && !got.Fort {
				t.Fatal("empty fort incorrectly demolished")
			}
			if tc.travels && !tc.capture && end.Players[0].King != from {
				t.Fatal("failed offensive moved ruler")
			}
		})
	}
}
func TestConquestKingCaptureRetreatE2E(t *testing.T) {
	c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0)}
	m, e := Module().Start(c, raw(Options{MapID: "europe", Difficulty: "normal", TurnSeconds: 120}))
	if e != nil {
		t.Fatal(e)
	}
	s, _ := decode(m)
	g, _ := mapFor(s.MapID)
	var from, to string
	for _, p := range g.data.Provinces {
		if len(p.Neighbors) > 0 {
			from = p.ID
			to = p.Neighbors[0]
			break
		}
	}
	for i := range s.Provinces {
		s.Provinces[i].Owner = -1
	}
	s.Provinces[g.index[from]].Owner = 0
	s.Provinces[g.index[from]].Troops = 10000
	s.Players[0].King = from
	s.Provinces[g.index[to]].Owner = 1
	s.Provinces[g.index[to]].Troops = 100
	s.Players[1].King = to
	assigned := 0
	for i := range s.Provinces {
		p := &s.Provinces[i]
		if p.ID != from && p.ID != to && assigned < 6 {
			p.Owner = 1
			p.Troops = 1000 + assigned
			assigned++
		}
	}
	s.refresh()
	m = s.envelope()
	m, e = Module().Apply(c, m, raw(Action{Type: "move", Session: s.Session, Epoch: s.Epoch, From: from, To: to, Amount: 9000}))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		c.Actor = i
		m, e = Module().Apply(c, m, raw(Action{Type: "submit", Session: s.Session, Epoch: s.Epoch}))
		if e != nil {
			t.Fatal(e)
		}
	}
	for m.SystemDeadline > 0 {
		c.Now = time.UnixMilli(m.SystemDeadline)
		m, e = Module().OnTimer(c, m)
		if e != nil {
			t.Fatal(e)
		}
	}
	end, _ := decode(m)
	if end.Players[1].Eliminated || end.Players[1].Territories != 1 || end.Players[1].King == to {
		t.Fatal("ruler succession did not retain strongest 18%", end.Players[1])
	}
}
