package doudizhu

// Detailed failure inventory is frozen in docs/research/huanle-sandayi.md.
// These cases precede the v2 rule implementation; legacy engines stay unchanged.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestHuanleOnlyFourPlayersAndFixedPreset(t *testing.T) {
	mod := Module()
	if mod.MinPlayers != 4 || mod.MaxPlayers != 4 {
		t.Fatal("new room is not fixed at four players")
	}
	if _, e := mod.Configure(3, nil); e == nil {
		t.Fatal("created classic three-player room")
	}
	o, e := mod.Configure(4, nil)
	if e != nil {
		t.Fatal(e)
	}
	var opt Options
	_ = json.Unmarshal(o, &opt)
	if opt.Mode != "shanghai4" || !opt.AllowStraight || opt.PersonalDoubling {
		t.Fatal(opt)
	}
}
func TestHuanleSequenceAndPlaneRules(t *testing.T) {
	for _, tc := range []struct {
		spec, kind string
		high       int
	}{
		{"A 2 3 4 5", "straight", 5}, {"2 3 4 5 6", "straight", 6}, {"10 J Q K A", "straight", 14}, {"J Q K A 2", "", 0},
		{"A A 2 2 3 3", "pairs", 3}, {"2 2 3 3 4 4", "pairs", 4}, {"K K A A 2 2", "", 0},
		{"3 3 3 4 4 4 8 8 9 9", "plane_pair", 4}, {"3 3 3 4 4 4 8 8 10 10", "", 0},
		{"3 3 3 4", "", 0}, {"3 3 3 4 4 4 5 6", "", 0}, {"3 3 3 3 4 5", "", 0},
		{"BJ BJ RJ RJ", "rocket", 17}, {"2 2 2 2 2 2 2 2", "bomb", 15},
	} {
		c, e := classifyHuanle(cards(tc.spec))
		if tc.kind == "" {
			if e == nil {
				t.Fatal("accepted invalid", tc.spec, c)
			}
		} else if e != nil || c.Kind != tc.kind || c.High != tc.high {
			t.Fatal(tc.spec, c, e)
		}
	}
}
func TestHuanleBombQuotasAndScoreTable(t *testing.T) {
	s := State{Players: 4, Landlord: 0, Bids: []int{3, 0, 1, 2}, BidDone: []bool{true, true, true, true}, BombCounts: []int{99, 0, 0, 0}, Ruleset: huanleRuleset}
	for p, want := range []int{-1, 1, 1, 2} {
		if got := s.bombLimit(p); got != want {
			t.Fatal(p, got, want)
		}
	}
	s.BidDone[2] = false
	if s.bombLimit(2) != -1 {
		t.Fatal("unasked farmer limited")
	}
	for _, tc := range []struct {
		win, head    bool
		mult, report int
		play         bool
		want         int
	}{
		{true, false, 1, 0, false, 3}, {true, true, 1, 0, false, 12}, {false, true, 1, 0, false, -3},
		{true, true, 8, 0, false, 96}, {true, true, 8, 1, false, 12}, {true, true, 8, 2, false, 24},
		{true, true, 8, 1, true, 108}, {false, true, 8, 1, true, -21}, {false, true, 1, 2, true, 3},
	} {
		s.Bid = 3
		s.Multiplier = tc.mult
		s.TouLiao = tc.head
		s.ReportPlay = tc.play
		s.Reports = make([][]int, tc.report)
		if got := s.huanleTransfer(tc.win); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
func TestHuanleMarkerPrivateBottomAndHuangFan(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 4, Now: time.Unix(1700000000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	if s.Ruleset != huanleRuleset || !slices.Contains(s.Hands[s.BidStart], s.Marker) || s.BidStart != s.MarkerSeat {
		t.Fatal("marker/start mismatch")
	}
	for i := 0; i < 4; i++ {
		ctx.Actor = m.Actors[0]
		s, _ = decode(m)
		raw, _ := json.Marshal(Action{Type: "bid", Window: s.Window, Bid: 0})
		m, e = mod.Apply(ctx, m, raw)
		if e != nil {
			t.Fatal(e)
		}
	}
	s, _ = decode(m)
	if !s.HuangFan || s.DealSerial != 2 || s.Round != 1 {
		t.Fatal("all pass did not produce huangfan")
	}
	ctx.Actor = m.Actors[0]
	raw, _ := json.Marshal(Action{Type: "bid", Window: s.Window, Bid: 3})
	m, e = mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	s, _ = decode(m)
	announced := false
	for _, event := range s.Events {
		if event.Type == "bid" && event.Seat == s.Landlord && event.Value == 3 {
			announced = true
		}
	}
	if !announced {
		t.Fatal("called tier absent from public sound event")
	}
	if !s.TouLiao || s.Phase != "landlord_choice" || len(s.Hands[s.Landlord]) != 33 {
		t.Fatal("first three-tier choice")
	}
	for _, p := range []int{-1, (s.Landlord + 1) % 4} {
		v, e := mod.View(m, p)
		if e != nil {
			t.Fatal(e)
		}
		d := v.(map[string]any)
		if len(d["bottom"].([]int)) != 0 || len(d["landlordHand"].([]int)) != 0 {
			t.Fatal("private bottom/hand leaked")
		}
		for _, ev := range d["events"].([]Event) {
			if ev.Type == "landlord" && len(ev.Cards) > 0 {
				t.Fatal("bottom leaked through event")
			}
		}
	}
	before := append([]byte{}, m.Data...)
	ctx.Actor = s.Landlord
	raw, _ = json.Marshal(Action{Type: "landlord", Window: s.Window, OpenHand: true, Continue: true})
	next, e := mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, []byte(m.Data)) {
		t.Fatal("mutated snapshot")
	}
	v, _ := mod.View(next, -1)
	if len(v.(map[string]any)["landlordHand"].([]int)) != 33 {
		t.Fatal("tan-da did not reveal landlord hand")
	}
}
