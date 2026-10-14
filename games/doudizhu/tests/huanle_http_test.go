package doudizhu_test

import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
)

// These deliberately labelled milestones force rare report/quota branches.
// Everything after Start goes through real sessions, HTTP, Apply and SQLite.
func huanleFixture(kind string) gameapi.Module {
	mod := game.Module()
	start := mod.Start
	mod.Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		m, e := start(ctx, o)
		if e != nil {
			return m, e
		}
		var s game.State
		_ = json.Unmarshal(m.Data, &s)
		s.Hands = make([][]int, 4)
		used := map[int]bool{}
		if kind == "report" {
			s.Hands[0] = []int{52, 53, 106, 107, 6, 19, 32, 45, 60, 73, 86}
		} else {
			s.Hands[1] = []int{1, 14, 27, 40, 2, 15, 28, 41}
		}
		for _, h := range s.Hands {
			for _, id := range h {
				used[id] = true
			}
		}
		rest := []int{}
		for id := 0; id < 108; id++ {
			if !used[id] {
				rest = append(rest, id)
			}
		}
		for p := 0; p < 4; p++ {
			size := 25
			if p == 0 {
				size = 33
			}
			count := size - len(s.Hands[p])
			s.Hands[p] = append(s.Hands[p], rest[:count]...)
			rest = rest[count:]
		}
		s.Bottom = append([]int{}, s.Hands[0][25:]...)
		s.Landlord = 0
		s.Bid = 3
		s.Multiplier = 1
		s.Played = []int{}
		s.LastPlay = nil
		s.Table = []game.Play{}
		s.TouLiao = kind == "report"
		s.OpenHand = false
		s.Reports = [][]int{}
		s.ReportPlay = false
		s.BombCounts = make([]int, 4)
		s.Bids = make([]int, 4)
		s.BidDone = make([]bool, 4)
		s.Bids[0] = 3
		s.BidDone[0] = true
		s.BidSteps = 1
		s.BidStart = 0
		s.MarkerSeat = 0
		s.Marker = 0
		s.Turn = 0
		s.Phase = "landlord_choice"
		s.Due = make([]int64, 4)
		if kind == "quota" {
			s.TouLiao = false
			s.Bids = []int{3, 0, 1, 2}
			s.BidDone = []bool{true, true, true, true}
			s.BidSteps = 4
			s.BidStart = 1
			s.MarkerSeat = 1
			s.Marker = 1
			s.Turn = 1
			s.Phase = "play"
		}
		s.Due[s.Turn] = ctx.Now.UnixMilli() + 30000
		s.Logs = append(s.Logs, "HTTP 定点夹具："+kind+"，不代表自然发到此牌型")
		m.Data, _ = json.Marshal(s)
		m.Actors = []int{s.Turn}
		m.TimeoutActors = []int{s.Turn}
		m.Attention = []int{s.Turn}
		m.Deadline = s.Due[s.Turn]
		return m, nil
	}
	return mod
}
func TestHuanleReportPrivacyAndAtomicSettlementHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, huanleFixture("report"), "doudizhu")
	h.AssertUnpublished()
	h.Start(map[string]any{"rounds": 1})
	d := gametest.Data(h.People[0].Room())
	c := d["controls"].(map[string]any)
	if len(c["reports"].([]any)) != 2 {
		t.Fatal("missing royal/seven-card reports")
	}
	for _, p := range append(h.People[1:], h.Spectator) {
		v := gametest.Data(p.Room())
		if len(v["bottom"].([]any)) > 0 || len(v["landlordHand"].([]any)) > 0 || len(v["controls"].(map[string]any)["reports"].([]any)) > 0 {
			t.Fatal("landlord secret leaked")
		}
	}
	before := gametest.Data(h.People[0].Room())
	h.People[0].Action(map[string]any{"type": "landlord", "window": d["window"], "reports": []int{0, 0}, "continue": false}, 400)
	if !reflect.DeepEqual(before, gametest.Data(h.People[0].Room())) {
		t.Fatal("invalid report mutated state")
	}
	request := h.People[0].ActionRequest(map[string]any{"type": "landlord", "window": d["window"], "reports": []int{0, 1}, "continue": false})
	id := h.People[0].Room()["id"].(string)
	h.People[0].Request("POST", "/api/rooms/"+id, request, 200)
	room := h.People[0].Room()
	if room["status"] != "finished" {
		t.Fatal("report-no-play stalled")
	}
	result := gametest.Data(room)["roundResults"].([]any)[0].(map[string]any)
	if result["reason"] != "report_no_play" || result["firstOut"].(float64) != -1 {
		t.Fatal("report was presented as having emptied the hand")
	}
	scores := result["scores"].([]any)
	for p, want := range []float64{72, -24, -24, -24} {
		if scores[p].(float64) != want {
			t.Fatal("head/double-report settlement", scores)
		}
	}
	h.Record("double report without play; private bottom retained")
	h.People[0].Request("POST", "/api/rooms/"+id, request, 200)
	if !reflect.DeepEqual(gametest.Data(room), gametest.Data(h.People[0].Room())) {
		t.Fatal("replayed report settled twice")
	}
	h.Restart()
	if !reflect.DeepEqual(gametest.Data(room), gametest.Data(h.People[0].Room())) {
		t.Fatal("report result did not restore")
	}
}
func TestHuanleBombQuotaSortedCardsHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, huanleFixture("quota"), "doudizhu")
	h.Start(map[string]any{"rounds": 1})
	d := gametest.Data(h.People[1].Room())
	h.People[1].Action(map[string]any{"type": "play", "window": d["window"], "cards": []int{40, 1, 27, 14}}, 200)
	d = gametest.Data(h.People[1].Room())
	if d["multiplier"].(float64) != 1 {
		t.Fatal("bomb incorrectly doubled Shanghai score")
	}
	cs := d["lastPlay"].(map[string]any)["cards"].([]any)
	for i, want := range []float64{1, 14, 27, 40} {
		if cs[i].(float64) != want {
			t.Fatal("click order leaked into table cards", cs)
		}
	}
	for _, p := range []int{2, 3, 0} {
		v := gametest.Data(h.People[p].Room())
		h.People[p].Action(map[string]any{"type": "pass", "window": v["window"]}, 200)
	}
	before := gametest.Data(h.People[1].Room())
	if before["controls"].(map[string]any)["bombRemaining"].(float64) != 0 {
		t.Fatal("pass bidder did not consume its quota")
	}
	for _, v := range before["controls"].(map[string]any)["moves"].([]any) {
		k := v.(map[string]any)["kind"]
		if k == "bomb" || k == "rocket" {
			t.Fatal("hint bypassed quota")
		}
	}
	h.People[1].Action(map[string]any{"type": "play", "window": before["window"], "cards": []int{2, 15, 28, 41}}, 400)
	if !reflect.DeepEqual(before, gametest.Data(h.People[1].Room())) {
		t.Fatal("rejected second bomb changed cards or deadline")
	}
	h.Record("sorted bomb; repeat bomb rejected without state changes")
}
func TestHuanleNaturalCompleteMatchesHTTP(t *testing.T) {
	for _, reveal := range []bool{false, true} {
		t.Run(fmt.Sprint(reveal), func(t *testing.T) {
			h := gametest.NewHTTP(t, 4, game.Module(), "doudizhu")
			h.Start(map[string]any{"rounds": 2})
			steps := 0
			for ; steps < 1200; steps++ {
				room := h.People[0].Room()
				if room["status"] == "finished" {
					break
				}
				p := int(room["game"].(map[string]any)["actors"].([]any)[0].(float64))
				d := gametest.Data(h.People[p].Room())
				ctrl := d["controls"].(map[string]any)
				a := map[string]any{"window": d["window"]}
				switch d["phase"] {
				case "bid":
					a["type"] = "bid"
					a["bid"] = 3
				case "landlord_choice":
					a["type"] = "landlord"
					a["openHand"] = reveal && ctrl["canOpen"].(bool)
					a["continue"] = true
				case "play":
					moves := ctrl["moves"].([]any)
					if len(moves) == 0 {
						a["type"] = "pass"
					} else {
						best := moves[0].(map[string]any)
						for _, v := range moves {
							m := v.(map[string]any)
							if len(m["cards"].([]any)) > len(best["cards"].([]any)) {
								best = m
							}
						}
						a["type"] = "play"
						a["cards"] = best["cards"]
					}
				case "round_end":
					a["type"] = "ready"
				default:
					t.Fatal("unexpected phase", d["phase"])
				}
				h.People[p].Action(a, 200)
				spectator := gametest.Data(h.Spectator.Room())
				if len(spectator["bottom"].([]any)) > 0 || len(spectator["hand"].([]any)) > 0 {
					t.Fatal("private cards leaked")
				}
				if spectator["openHand"].(bool) {
					lord := int(spectator["landlord"].(float64))
					count := int(spectator["players"].([]any)[lord].(map[string]any)["count"].(float64))
					if len(spectator["landlordHand"].([]any)) != count {
						t.Fatal("public landlord hand drifted")
					}
				}
			}
			if steps == 1200 {
				t.Fatal("natural match stalled")
			}
			d := gametest.Data(h.People[0].Room())
			if len(d["roundResults"].([]any)) != 2 {
				t.Fatal("not all rounds completed")
			}
			for _, v := range d["roundResults"].([]any) {
				rr := v.(map[string]any)
				sum := 0.
				for _, score := range rr["scores"].([]any) {
					sum += score.(float64)
				}
				if sum != 0 || rr["spring"] != "" {
					t.Fatal("wrong Shanghai settlement")
				}
			}
			h.Record("two natural Shanghai hands completed")
			h.Restart()
			if !reflect.DeepEqual(d, gametest.Data(h.People[0].Room())) {
				t.Fatal("natural result restore")
			}
		})
	}
}
