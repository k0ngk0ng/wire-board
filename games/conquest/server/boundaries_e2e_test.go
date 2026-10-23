package conquest

// Regression expectations discovered in original-JAR probes and contract
// review; written before fixing each corresponding execution boundary.
import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestConquestFinalViewAndExpiredWindow(t *testing.T) {
	c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0)}
	mod := Module()
	m, e := mod.Start(c, raw(Options{MapID: "europe", Difficulty: "normal", TurnSeconds: 120}))
	if e != nil {
		t.Fatal(e)
	}
	s, _ := decode(m)
	c.Now = time.UnixMilli(m.Deadline)
	if _, e = mod.Apply(c, m, raw(Action{Type: "submit", Session: s.Session, Epoch: s.Epoch})); e == nil {
		t.Fatal("expired human submitted")
	}
	c.Automated = []bool{true, false}
	if _, e = mod.Apply(c, m, raw(Action{Type: "submit", Session: s.Session, Epoch: s.Epoch})); e != nil {
		t.Fatal("bot cannot take expired window", e)
	}
	c.Now = time.Unix(1700000000, 0)
	c.Automated = nil
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
	m = s.envelope()
	m, e = mod.Apply(c, m, raw(Action{Type: "move", Session: s.Session, Epoch: s.Epoch, From: from, To: to, Amount: 9000}))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		c.Actor = i
		m, e = mod.Apply(c, m, raw(Action{Type: "submit", Session: s.Session, Epoch: s.Epoch}))
		if e != nil {
			t.Fatal(e)
		}
	}
	for m.SystemDeadline > 0 {
		c.Now = time.UnixMilli(m.SystemDeadline)
		m, e = mod.OnTimer(c, m)
		if e != nil {
			t.Fatal(e)
		}
	}
	if m.Result == nil {
		t.Fatal("missing conquest victory")
	}
	if _, e = mod.View(m, 0); e != nil {
		t.Fatal("final board unreadable", e)
	}
	if _, e = mod.Restore(c, m); e != nil {
		t.Fatal("final save unreadable", e)
	}
}
func TestConquestBankruptcyOriginalOrder(t *testing.T) {
	c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0)}
	m, e := Module().Start(c, raw(Options{MapID: "europe", Difficulty: "normal", TurnSeconds: 120}))
	if e != nil {
		t.Fatal(e)
	}
	s, _ := decode(m)
	s.Players[0].Gold = -10000 // Income must use pre-bankruptcy morale 90, not 89.
	m = s.envelope()
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
	end, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	p := end.Players[0]
	production := int64(0)
	for _, v := range end.Provinces {
		if v.Owner == 0 {
			production += int64(v.Economy) * int64(v.Population) / 50000
		}
	}
	p.Morale = 90
	copyState := *end
	copyState.Players = append([]Player(nil), end.Players...)
	copyState.Players[0] = p
	earned := income(&copyState, 0)
	want := -int64(10000) + earned - earned/10 - int64(p.Troops/5)
	if end.Players[0].Gold != want || end.Players[0].Morale != 89 {
		t.Fatalf("wrong bankruptcy order gold=%d want=%d morale=%d production=%d", end.Players[0].Gold, want, end.Players[0].Morale, production)
	}
}
