package game

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func railLogGame(t *testing.T) *State {
	t.Helper()
	s := mustGame(t, "rail", 2)
	s.AutoChooseRailSetup()
	s.Log = nil
	s.Rail.Face = []int{0, 1, 2, 3, 4}
	s.Rail.Deck = []int{5, 6, 7, 0, 1, 2, 3, 4}
	s.Rail.Discard = nil
	return s
}

func TestRailPublicDrawAndBlindLogPrivacy(t *testing.T) {
	s := railLogGame(t)
	apply(t, s, Action{Type: "draw", Slot: 2})
	if got := s.Log[0]; !strings.Contains(got, "玩家 1 拿取公开蓝色列车牌×1（市场第 3 格，第 1 次摸牌）") {
		t.Fatal(got)
	}
	apply(t, s, Action{Type: "draw", Slot: 1})
	if got := s.Log[1]; !strings.Contains(got, "玩家 1") || !strings.Contains(got, "白色列车牌×1") || !strings.Contains(got, "第 2 次摸牌") || s.Turn != 1 {
		t.Fatal(s.Log, s.Turn)
	}
	var logs []string
	for color := 0; color < 9; color++ {
		s := railLogGame(t)
		s.Rail.Deck[0] = color
		apply(t, s, Action{Type: "draw", Slot: -1})
		if color == 0 {
			logs = s.Log
		} else if !reflect.DeepEqual(logs, s.Log) {
			t.Fatal("blind color leaked", s.Log)
		}
	}
	s = railLogGame(t)
	s.Rail.Face[0] = 8
	apply(t, s, Action{Type: "draw", Slot: 0})
	if !strings.Contains(s.Log[0], "万能牌×1") || !strings.Contains(s.Log[0], "本回合摸牌结束") || s.Turn != 1 {
		t.Fatal(s.Log)
	}
}

func TestRailClaimLogActualPaymentAndLastRound(t *testing.T) {
	for _, allWild := range []bool{false, true} {
		s := railLogGame(t)
		var route Route
		for _, r := range MapData().Routes {
			if r.Length == 3 {
				route = r
				break
			}
		}
		color := max(route.Color, 0)
		wild := 1
		if allWild {
			wild = route.Length
		}
		p := &s.Rail.Players[0]
		p.Hand = make([]int, 9)
		p.Hand[color] = route.Length - wild
		p.Hand[8] = wild
		p.Trains = 5
		apply(t, s, Action{Type: "claim", Route: route.ID, Color: color, Wild: wild})
		got := s.Log[0]
		for _, want := range []string{"玩家 1", MapData().Cities[route.A].Name, MapData().Cities[route.B].Name, "3 节", "获得 4 分", "剩余 2 节车厢", fmt.Sprintf("万能牌×%d", wild)} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %s: %s", want, got)
			}
		}
		if allWild && strings.Contains(got, "列车牌") {
			t.Fatal("phantom colored payment", got)
		}
		if !allWild && !strings.Contains(got, railCardName(color)+"×2") {
			t.Fatal(got)
		}
		if !strings.Contains(s.Log[len(s.Log)-1], "触发最后一轮") {
			t.Fatal(s.Log)
		}
		before := append([]string{}, s.Log...)
		if err := s.Apply(1, Action{Type: "claim", Route: route.ID, Color: color}); err == nil || !reflect.DeepEqual(before, s.Log) {
			t.Fatal("rejected move logged")
		}
	}
}

func TestRailMarketResetLogOrdering(t *testing.T) {
	s := railLogGame(t)
	s.Rail.Face = []int{8, 8, 0, 1, 2}
	s.Rail.Deck = []int{8, 3, 4, 5, 6, 7, 0, 1}
	s.Rail.Discard = nil
	apply(t, s, Action{Type: "draw", Slot: 2})
	if len(s.Log) != 2 || !strings.Contains(s.Log[0], "紫色列车牌×1") || !strings.Contains(s.Log[1], "已重置市场（1 次）") {
		t.Fatal(s.Log)
	}
}

func TestRailTicketLogDoesNotRevealDestinations(t *testing.T) {
	var first []string
	for _, offset := range []int{0, 6} {
		s := railLogGame(t)
		s.Rail.TicketDeck = MapData().Tickets[offset : offset+3]
		apply(t, s, Action{Type: "tickets"})
		apply(t, s, Action{Type: "keep", Keep: []int{s.Rail.Pending[0].ID}})
		if offset == 0 {
			first = s.Log
		} else if !reflect.DeepEqual(first, s.Log) {
			t.Fatal("ticket identity leaked", s.Log)
		}
		if !strings.Contains(s.Log[0], "3 张目的地任务") || !strings.Contains(s.Log[1], "保留了 1 张目的地任务，放回 2 张") {
			t.Fatal(s.Log)
		}
	}
}

func TestRailHiddenDrawEventsArePublicWithoutCardIdentity(t *testing.T) {
	var events []HiddenDrawEvent
	for color := 0; color < 9; color++ {
		s := railLogGame(t)
		s.Rail.Deck[0] = color
		s.Rail.Deck[1] = color
		apply(t, s, Action{Type: "draw", Slot: -1})
		apply(t, s, Action{Type: "draw", Slot: -1})
		if color == 0 {
			events = append([]HiddenDrawEvent{}, s.Rail.HiddenDrawEvents...)
		} else if !reflect.DeepEqual(events, s.Rail.HiddenDrawEvents) {
			t.Fatal("color leaked in animation events")
		}
		if s.Rail.HiddenDrawID != 2 || len(events) != 2 || events[0].Player != 0 || events[1].Player != 0 || s.Turn != 1 {
			t.Fatal("wrong draw actor or sequence", events)
		}
		for _, viewer := range []int{0, 1, -1} {
			if len(s.View(viewer)["rail"].(map[string]any)["hiddenDrawEvents"].([]any)) != 2 {
				t.Fatal("missing public animation")
			}
		}
		apply(t, s, Action{Type: "draw", Slot: 0})
		if s.Rail.HiddenDrawID != 2 {
			t.Fatal("public draw created hidden animation")
		}
	}
	s := railLogGame(t)
	for i := 0; i < 14; i++ {
		s.Rail.Deck = []int{0, 1, 2}
		apply(t, s, Action{Type: "draw", Slot: -1})
	}
	if len(s.Rail.HiddenDrawEvents) != 10 || s.Rail.HiddenDrawEvents[0].ID != 5 || s.Rail.HiddenDrawID != 14 {
		t.Fatal("event retention unbounded")
	}
}
