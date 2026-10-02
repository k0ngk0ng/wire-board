package game

import (
	"reflect"
	"strings"
	"testing"
)

func requireSplendorLog(t *testing.T, s *State, parts ...string) {
	t.Helper()
	if len(s.Log) == 0 {
		t.Fatal("missing action log")
	}
	line := s.Log[len(s.Log)-1]
	for _, part := range parts {
		if !strings.Contains(line, part) {
			t.Fatalf("log %q missing %q", line, part)
		}
	}
}

func TestSplendorLogsGemColorsAndReturns(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	apply(t, s, Action{Type: "take", Tokens: []int{1, 0, 1, 0, 1, 0}})
	requireSplendorLog(t, s, "玩家 1", "祖母绿（绿）×1", "蓝宝石（蓝）×1", "红宝石（红）×1", "共 3 枚")
	apply(t, s, Action{Type: "take", Tokens: []int{0, 2, 0, 0, 0, 0}})
	requireSplendorLog(t, s, "玩家 2", "钻石（白）×2", "共 2 枚")
	s.Phase = "discard"
	s.Splendor.Players[s.Turn].Tokens = []int{2, 2, 2, 3, 2, 1}
	apply(t, s, Action{Type: "discard", Tokens: []int{0, 0, 0, 1, 0, 1}})
	requireSplendorLog(t, s, "玩家 1", "归还", "缟玛瑙（黑）×1", "黄金×1", "10 / 10")
}

func TestSplendorPurchaseLogsActualPayment(t *testing.T) {
	for _, spec := range []struct {
		name           string
		reserved, free bool
		payment        []int
		want           []string
	}{
		{name: "automatic gold", want: []string{"从市场购买", "支付 祖母绿（绿）×1、钻石（白）×1、黄金×1"}},
		{name: "chosen gold from reservation", reserved: true, payment: []int{0, 0, 0, 0, 0, 3}, want: []string{"从预留区购买", "支付 黄金×3"}},
		{name: "fully discounted", free: true, want: []string{"无需支付宝石"}},
	} {
		t.Run(spec.name, func(t *testing.T) {
			s := mustGame(t, "splendor", 2)
			p := &s.Splendor.Players[0]
			card := Card{ID: 999, Tier: 2, Color: 2, Points: 1, Cost: []int{2, 2, 0, 0, 0}}
			s.Splendor.Nobles = nil
			if spec.reserved {
				p.Reserved = []Card{card}
			} else {
				s.Splendor.Market[1][0] = card
			}
			p.Tokens = []int{1, 1, 0, 0, 0, 3}
			p.Bonus = []int{1, 0, 0, 0, 0}
			if spec.free {
				p.Bonus = []int{2, 2, 0, 0, 0}
			}
			before := append([]int(nil), p.Tokens...)
			apply(t, s, Action{Type: "buy", Card: 999, Tokens: spec.payment})
			requireSplendorLog(t, s, append(spec.want, "玩家 1", "蓝宝石（蓝）发展卡", "2 级，1 分，#999", "永久蓝宝石（蓝） +1", "当前 1 分")...)
			if spec.free && !reflect.DeepEqual(before, p.Tokens) {
				t.Fatal("free purchase spent tokens")
			}
			if spec.reserved && (p.Tokens[0] != 1 || p.Tokens[1] != 1 || p.Tokens[5] != 0) {
				t.Fatal("chosen payment not applied")
			}
		})
	}
}

func TestSplendorReservationLogsProtectBlindCards(t *testing.T) {
	for _, gold := range []int{0, 1} {
		s := mustGame(t, "splendor", 2)
		s.Splendor.Bank[5] = gold
		other := clone(*s)
		// Changing every secret attribute must not change the public log.
		other.Splendor.Decks[1][0] = Card{ID: 999, Tier: 2, Color: 4, Points: 9, Cost: []int{9, 8, 7, 6, 5}}
		for _, g := range []*State{s, &other} {
			apply(t, g, Action{Type: "reserve", Tier: 2})
		}
		if !reflect.DeepEqual(s.Log, other.Log) {
			t.Fatal("blind reservation exposed secret card", s.Log, other.Log)
		}
		requireSplendorLog(t, s, "从 2 级牌堆盲预留了 1 张发展卡")
		if gold > 0 {
			requireSplendorLog(t, s, "获得黄金×1")
		} else {
			requireSplendorLog(t, s, "黄金已空，未获得黄金")
		}
		for _, viewer := range []int{1, -1} {
			logs := s.View(viewer)["log"].([]any)
			if strings.Contains(logs[len(logs)-1].(string), "#") {
				t.Fatal("secret identity exposed to viewer")
			}
		}
	}
	s := mustGame(t, "splendor", 2)
	s.Splendor.Market[0][0] = Card{ID: 998, Tier: 1, Color: 3, Points: 0, Cost: []int{0, 2, 1, 0, 0}}
	apply(t, s, Action{Type: "reserve", Card: 998})
	requireSplendorLog(t, s, "从市场预留", "缟玛瑙（黑）发展卡", "1 级，0 分，#998", "获得黄金×1")
}

func TestSplendorNobleLogsFollowActionAndUseActingPlayer(t *testing.T) {
	for _, choose := range []bool{false, true} {
		s := mustGame(t, "splendor", 2)
		s.Splendor.Nobles = []Noble{{ID: 7, Cost: []int{0, 0, 0, 0, 0}}}
		if choose {
			s.Splendor.Nobles = append(s.Splendor.Nobles, Noble{ID: 8, Cost: []int{0, 0, 0, 0, 0}})
		}
		apply(t, s, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}})
		if choose {
			apply(t, s, Action{Type: "noble", Noble: 7})
		}
		requireSplendorLog(t, s, "玩家 1", "贵族 #7", "+3 分", "当前 3 分")
		if len(s.Log) != 2 || !strings.Contains(s.Log[0], "拿取") || s.Turn != 1 {
			t.Fatal("noble log missing, duplicated or out of order", s.Log)
		}
	}
}

func TestSplendorRejectedActionDoesNotLog(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	before := append([]string{}, s.Log...)
	if err := s.Apply(0, Action{Type: "buy", Card: 999}); err == nil {
		t.Fatal("invalid purchase accepted")
	}
	if !reflect.DeepEqual(before, s.Log) {
		t.Fatal("rejected action appended a log")
	}
}
