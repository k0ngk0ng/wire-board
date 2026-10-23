package conquest

// These regressions were specified before resuming implementation fixes.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func regressionStart(t *testing.T) (gameapi.Match, gameapi.Context, *State, *geography) {
	t.Helper()
	c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0), Automated: []bool{false, false}}
	m, e := Module().Start(c, raw(Options{MapID: "europe", Difficulty: "normal", TurnSeconds: 120}))
	if e != nil {
		t.Fatal(e)
	}
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	g, _ := mapFor(s.MapID)
	return m, c, s, g
}
func regressionAct(t *testing.T, m gameapi.Match, c gameapi.Context, a Action) gameapi.Match {
	t.Helper()
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	a.Session = s.Session
	a.Epoch = s.Epoch
	n, e := Module().Apply(c, m, raw(a))
	if e != nil {
		t.Fatal(e)
	}
	return n
}
func TestConquestArrivalCannotMoveTwiceE2E(t *testing.T) {
	_, c, s, g := regressionStart(t)
	var a, b, d string
	for _, p := range g.data.Provinces {
		if len(p.Neighbors) >= 2 {
			a = p.Neighbors[0]
			b = p.ID
			d = p.Neighbors[1]
			break
		}
	}
	for _, id := range []string{a, b, d} {
		i := g.index[id]
		s.Provinces[i].Owner = 0
		s.Provinces[i].Troops = 0
	}
	s.Provinces[g.index[a]].Troops = 500
	s.Players[0].King = a
	s.Players[0].MP10 = 100
	m := s.envelope()
	m = regressionAct(t, m, c, Action{Type: "move", From: a, To: b, Amount: 300})
	v, _ := Module().View(m, 0)
	p := v.(View).Projected[g.index[b]]
	if p.Mobile != 0 {
		t.Fatalf("arriving soldiers are usable twice: %d", p.Mobile)
	}
	s, _ = decode(m)
	if _, e := Module().Apply(c, m, raw(Action{Type: "move", Session: s.Session, Epoch: s.Epoch, From: b, To: d, Amount: 300})); e == nil {
		t.Fatal("second hop used unarrived soldiers")
	}
	_, c, s, g = regressionStart(t)
	for _, id := range []string{a, b, d} {
		i := g.index[id]
		s.Provinces[i].Owner = 0
		s.Provinces[i].Troops = 500
	}
	s.Players[0].King = a
	s.Players[0].MP10 = 100
	m = s.envelope()
	m = regressionAct(t, m, c, Action{Type: "move", From: a, To: b, Amount: 300, King: true})
	s, _ = decode(m)
	if _, e := Module().Apply(c, m, raw(Action{Type: "move", Session: s.Session, Epoch: s.Epoch, From: b, To: d, Amount: 100, King: true})); e == nil {
		t.Fatal("ruler travelled twice in one plan")
	}
}
func resolvingFixture(t *testing.T) (gameapi.Match, gameapi.Context, *geography) {
	t.Helper()
	_, c, s, g := regressionStart(t)
	var from, to string
	for _, p := range g.data.Provinces {
		if len(p.Neighbors) > 0 {
			from = p.ID
			to = p.Neighbors[0]
			break
		}
	}
	s.Provinces[g.index[from]].Owner = 0
	s.Provinces[g.index[from]].Troops = 1000
	s.Players[0].King = from
	s.Players[0].MP10 = 100
	s.Provinces[g.index[to]].Owner = -1
	m := s.envelope()
	m = regressionAct(t, m, c, Action{Type: "recruit", Province: from, Amount: 100})
	m = regressionAct(t, m, c, Action{Type: "move", From: from, To: to, Amount: 100})
	m = regressionAct(t, m, c, Action{Type: "submit"})
	c.Actor = 1
	m = regressionAct(t, m, c, Action{Type: "submit"})
	if m.SystemDeadline == 0 {
		t.Fatal("fixture not resolving")
	}
	return m, c, g
}
func TestConquestResolutionDoesNotDoubleProjectE2E(t *testing.T) {
	m, _, _ := resolvingFixture(t)
	s, _ := decode(m)
	v, e := Module().View(m, 0)
	if e != nil {
		t.Fatal(e)
	}
	got := v.(View)
	if got.AvailableGold != s.Players[0].Gold || got.AvailableMovement != float64(s.Players[0].MP10)/10 || !reflect.DeepEqual(got.Projected, s.Provinces) {
		t.Fatal("resolving view reapplies already executed plans")
	}
	if len(got.Orders) != 0 {
		t.Fatal("executing instructions presented as editable plan")
	}
}
func TestConquestCorruptPendingRejectedE2E(t *testing.T) {
	valid, c, _ := resolvingFixture(t)
	cases := map[string]func(*State, *gameapi.Match){
		"negative-seat":   func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Seat = -1 },
		"large-seat":      func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Seat = 20 },
		"unknown-type":    func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Order.Type = "cheat" },
		"unknown-from":    func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Order.From = "missing" },
		"negative-amount": func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Order.Amount = -50 },
		"extra-fields":    func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Order.Building = "fort" },
		"invalid-id":      func(s *State, m *gameapi.Match) { s.Pending[s.Cursor].Order.ID = 0 },
		"finished-cursor": func(s *State, m *gameapi.Match) { s.Cursor = len(s.Pending) },
		"wrong-envelope":  func(s *State, m *gameapi.Match) { m.Paused = true },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			var s State
			if e := json.Unmarshal(valid.Data, &s); e != nil {
				t.Fatal(e)
			}
			m := valid
			change(&s, &m)
			m.Data = raw(s)
			if _, e := Module().Restore(c, m); e == nil {
				t.Fatal("corrupt pending snapshot restored")
			}
			if _, e := Module().View(m, 0); e == nil {
				t.Fatal("corrupt pending snapshot exposed")
			}
		})
	}
}
func TestConquestSubmittedPauseReopenKeepsDeadline(t *testing.T) {
	for submitted := 0; submitted < 2; submitted++ {
		t.Run(string(rune('0'+submitted)), func(t *testing.T) {
			m, c, _, _ := regressionStart(t)
			c.Actor = submitted
			m = regressionAct(t, m, c, Action{Type: "submit"})
			c.Now = c.Now.Add(17 * time.Second)
			yes := true
			for i := 0; i < 2; i++ {
				c.Actor = i
				m = regressionAct(t, m, c, Action{Type: "pause", Enabled: &yes})
			}
			if !m.Paused {
				t.Fatal("not paused")
			}
			c.Now = c.Now.Add(48 * time.Hour)
			for i := 0; i < 2; i++ {
				c.Actor = i
				m = regressionAct(t, m, c, Action{Type: "resume", Enabled: &yes})
			}
			c.Actor = submitted
			m = regressionAct(t, m, c, Action{Type: "unsubmit"})
			if e := m.Validate(2); e != nil {
				t.Fatal(e)
			}
			s, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			want := c.Now.Add(103 * time.Second).UnixMilli()
			if s.Players[submitted].Deadline != want || m.Deadline != want || len(m.TimeoutActors) != 2 {
				t.Fatalf("reopened submitted seat lost deadline: players=%v deadline=%d timeoutActors=%v", s.Players, m.Deadline, m.TimeoutActors)
			}
		})
	}
}

func TestConquestPlanSerialAndCorruptRestoredPlan(t *testing.T) {
	_, c, s, g := regressionStart(t)
	s.NextOrder[0] = 100000
	m := s.envelope()
	tax := 20
	if _, err := Module().Apply(c, m, raw(Action{Type: "tax", Session: s.Session, Epoch: s.Epoch, Value: &tax})); err == nil {
		t.Fatal("legal serial boundary produced unreadable next snapshot")
	}
	if _, err := Module().Restore(c, m); err != nil {
		t.Fatal("boundary snapshot itself should remain readable", err)
	}
	_, c, s, g = regressionStart(t)
	from := s.Players[0].King
	to := g.data.Provinces[g.index[from]].Neighbors[0]
	s.NextOrder[0] = 1
	s.Plans[0] = []Order{{ID: 1, Type: "move", From: from, To: to, Amount: 999999}}
	m = s.envelope()
	if _, err := Module().Restore(c, m); err == nil {
		t.Fatal("restore accepted plan with unavailable army")
	}
}
