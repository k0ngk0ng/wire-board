package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Isolated failure scenarios, specified before the policy/death changes:
//   - a wounded ally is attacked instead of an in-range enemy;
//   - a dying ally is not rescued, or an enemy is rescued;
//   - counters protect the wrong camp or ignore counter-counter parity;
//   - an AOE exposes a one-HP friendly leader, or medicine heals the enemy;
//   - changing hidden opponent cards changes our otherwise identical decision;
//   - death discards inheritance, skips Duanchang/Wuhun, loses its continuation
//     on save/restore, grants the wrong reward, or continues after leader death.
//
// Fixed positions make rare nested death effects deterministic. HTTP natural
// match acceptance remains in internal/server/sanguosha_3v3_test.go.
func sgThreeBattleState(t *testing.T) *State {
	t.Helper()
	s, err := NewSanguosha(6, SGOptions{Mode: "3v3", Packs: []string{"standard", "wind", "fire", "thicket", "mountain", "god"}})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Sanguosha
	g.Selecting, g.Selected, g.Pending, g.Queue = false, 6, nil, nil
	g.InPlay, g.ActivePhase = true, "play"
	g.ThreeV3.Camps = []string{"cold", "cold", "cold", "warm", "warm", "warm"}
	g.ThreeV3.Leaders = []int{1, 4}
	g.ThreeV3.Stage, g.ThreeV3.Side = "play", 0
	s.Turn, s.Phase = 0, "sg_play"
	for i := range g.Players {
		g.Players[i] = SGPlayer{General: "zhangfei", HP: 4, MaxHP: 4, Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}}
	}
	slices.Sort(g.Deck)
	return s
}

func TestSanguoshaThreeV3BotCampActions(t *testing.T) {
	t.Run("attack enemy and ignore hidden card identities", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[1].HP = 1
		slash := sgGive(t, s, 0, "slash")
		enemyCard := sgGive(t, s, 5, "peach")
		a, err := s.BotAction(0)
		if err != nil || a.Type != "sg_play" || !slices.Equal(a.Cards, []int{slash}) || !slices.Equal(a.Targets, []int{5}) {
			t.Fatalf("expected slash at enemy 5, got %+v %v", a, err)
		}
		copy := clone(*s)
		copy.Sanguosha.Players[5].Hand[0], copy.Sanguosha.Deck[0] = copy.Sanguosha.Deck[0], enemyCard
		slices.Reverse(copy.Sanguosha.Deck)
		b, err := copy.BotAction(0)
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("hidden cards affected decision", a, b, err)
		}
		sgDo(t, s, 0, a)
	})
	for _, target := range []int{1, 4} {
		t.Run("rescue camp "+string(rune('0'+target)), func(t *testing.T) {
			s := sgThreeBattleState(t)
			s.Sanguosha.Players[target].HP = 1
			peach := sgGive(t, s, 0, "peach")
			s.sgPush(SGEvent{Type: "damage", Actor: 5, Target: target, Amount: 1})
			s.sgRun()
			if q := s.Sanguosha.Pending; q == nil || q.Kind != "peach" || q.Player != 0 {
				t.Fatal("fixture did not ask for rescue", q)
			}
			a, err := s.BotAction(0)
			if err != nil {
				t.Fatal(err)
			}
			if target == 1 && !slices.Equal(a.Cards, []int{peach}) || target == 4 && a.Choice != "pass" {
				t.Fatal("wrong camp rescue", target, a)
			}
			sgDo(t, s, 0, a)
			if target == 1 && s.Sanguosha.Players[target].HP != 1 {
				t.Fatal("ally not rescued")
			}
		})
	}
	for _, tc := range []struct {
		name               string
		target             int
		kind               string
		cancelled, counter bool
	}{
		{"protect ally", 1, "indulgence", false, true},
		{"already protected ally", 1, "indulgence", true, false},
		{"restore enemy harm", 4, "indulgence", true, true},
		{"let enemy harm resolve", 4, "indulgence", false, false},
		{"deny enemy benefit", 4, "ex_nihilo", false, true},
		{"restore ally benefit", 1, "ex_nihilo", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sgThreeBattleState(t)
			card := sgGive(t, s, 0, "nullification")
			s.sgAsk(-1, "nullification", "无懈可击", SGEvent{Type: "trick", Actor: 5, Target: tc.target, Kind: tc.kind, Flag: tc.cancelled})
			a, err := s.BotAction(0)
			if err != nil || slices.Contains(a.Cards, card) != tc.counter {
				t.Fatal("counter chose the wrong camp/parity", tc, a, err)
			}
			sgDo(t, s, 0, a)
		})
	}
	t.Run("avoid lethal friendly AOE", func(t *testing.T) {
		s := sgThreeBattleState(t)
		for _, i := range []int{1, 3, 4, 5} {
			s.Sanguosha.Players[i].HP = 1
		}
		sgGive(t, s, 0, "savage_assault")
		a, err := s.BotAction(0)
		if err != nil || a.Type != "sg_end" {
			t.Fatal("bot exposes its own leader", a, err)
		}
	})
	t.Run("heal wounded teammate", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[0].General = "huatuo"
		s.Sanguosha.Players[1].HP = 1
		sgGive(t, s, 0, "jink")
		a, err := s.BotAction(0)
		if err != nil || a.Skill != "qingnang" || !slices.Equal(a.Targets, []int{1}) {
			t.Fatal("medicine did not help teammate", a, err)
		}
		sgDo(t, s, 0, a)
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("medicine failed")
		}
	})
}

func sgThreePassRescue(t *testing.T, s *State) {
	t.Helper()
	for steps := 0; steps < 20 && s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "peach"; steps++ {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
}

func TestSanguoshaThreeV3DeathContinuations(t *testing.T) {
	t.Run("inheritance then three-card reward survives restore", func(t *testing.T) {
		s := sgThreeBattleState(t)
		g := s.Sanguosha
		g.Players[1].General, g.Players[3].HP = "caopi", 1
		hand := sgGive(t, s, 3, "jink")
		equip := sgGive(t, s, 3, "crossbow")
		delay := sgGive(t, s, 3, "lightning")
		g.Players[3].Hand = []int{hand}
		g.Players[3].Equip = []int{equip}
		g.Players[3].Judgment = []SGDelayed{{Card: delay, Kind: "lightning"}}
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 3, Amount: 1})
		s.sgRun()
		sgThreePassRescue(t, s)
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "xingshang" || q.Player != 1 {
			t.Fatal("inheritance was skipped", q)
		}
		if len(s.Sanguosha.Players[0].Hand) != 0 {
			t.Fatal("reward ran before inheritance")
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var restored State
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		s = &restored
		sgDo(t, s, 1, Action{Choice: "yes"})
		g = s.Sanguosha
		if !slices.Contains(g.Players[1].Hand, hand) || !slices.Contains(g.Players[1].Hand, equip) || !slices.Contains(g.Discard, delay) || len(g.Players[0].Hand) != 3 {
			t.Fatal("inheritance zones or reward wrong", g.Players[1].Hand, g.Players[0].Hand)
		}
		if g.Players[0].TurnKills != 1 {
			t.Fatal("kill not recorded for death skills")
		}
		sgConservation(t, s)
	})
	t.Run("duanchang removes killer skills", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[3].General, s.Sanguosha.Players[3].HP = "caiwenji", 1
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 3, Amount: 1})
		s.sgRun()
		sgThreePassRescue(t, s)
		if !s.Sanguosha.Players[0].SkillsLost || s.sgHas(0, "paoxiao") {
			t.Fatal("duanchang did not remove killer skills")
		}
		sgConservation(t, s)
	})
	t.Run("dead vanguard wuhun can defeat enemy leader", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[0].General, s.Sanguosha.Players[0].HP = "shenguanyu", 1
		sgTop(t, s, func(c SGCard) bool { return c.Kind != "peach" && c.Kind != "god_salvation" })
		s.sgPush(SGEvent{Type: "damage", Actor: 4, Target: 0, Amount: 1})
		s.sgRun()
		sgThreePassRescue(t, s)
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "wuhun_target" || q.Player != 0 {
			t.Fatal("wuhun lost", q)
		}
		a, err := s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		sgDo(t, s, 0, a)
		if !s.Finished || !slices.Equal(s.Winners, []int{0, 1, 2}) || !s.Sanguosha.Players[4].Dead || s.Sanguosha.Pending != nil || len(s.Sanguosha.Queue) != 0 {
			t.Fatal("nested leader death did not end match", s.Winners, s.Sanguosha.Pending)
		}
		sgConservation(t, s)
	})
	t.Run("leader death stops wuhun and later damage", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[1].General, s.Sanguosha.Players[1].HP = "shenguanyu", 1
		s.sgPush(SGEvent{Type: "damage", Actor: 4, Target: 1, Amount: 1}, SGEvent{Type: "damage", Actor: 1, Target: 4, Amount: 10})
		s.sgRun()
		sgThreePassRescue(t, s)
		if !s.Finished || !slices.Equal(s.Winners, []int{3, 4, 5}) || s.Sanguosha.Players[4].Dead || s.Sanguosha.Pending != nil || len(s.Sanguosha.Queue) != 0 {
			t.Fatal("terminal leader death resumed effects", s.Winners)
		}
		sgConservation(t, s)
	})
}

// Scenarios fixed before the direction/reward code: a 3v3 group trick must
// resolve clockwise by default and counter-clockwise on request; a direction
// outside 3v3, on a non-group card, or with an unknown value must be rejected
// without touching state; and lord skills must stay locked in 3v3 even for the
// seat that holds the identity-mode lord role.
func TestSanguoshaThreeV3GroupTrickDirection(t *testing.T) {
	generals := []string{"zhangfei", "guanyu", "zhaoyun", "machao", "lvbu", "diaochan"}
	for _, tc := range []struct {
		name      string
		direction string
		reverse   bool
	}{
		{"clockwise default", "", false},
		{"clockwise explicit", "cw", false},
		{"counter-clockwise", "ccw", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sgThreeBattleState(t)
			for i, id := range generals {
				s.Sanguosha.Players[i].General = id
			}
			order := []int{1, 2, 3, 4, 5}
			if tc.reverse {
				slices.Reverse(order)
			}
			parts := []string{}
			for _, seat := range order {
				parts = append(parts, s.sgName(seat))
			}
			want := s.sgName(0) + " 使用「万箭齐发」→ " + strings.Join(parts, " ")
			card := sgGive(t, s, 0, "archery_attack")
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}, Targets: []int{1, 2, 3, 4, 5}, Direction: tc.direction})
			found := ""
			for _, line := range s.Log {
				if strings.Contains(line, "使用「万箭齐发」") {
					found = line
				}
			}
			if found != want {
				t.Fatalf("resolution order %q, want %q", found, want)
			}
			sgConservation(t, s)
		})
	}
	t.Run("direction rejected outside 3v3", func(t *testing.T) {
		s, err := NewSanguosha(6, SGOptions{Packs: []string{"standard"}})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.sgUseDirected(0, "archery_attack", nil, nil, true, "ccw"); err == nil {
			t.Fatal("identity mode accepted a resolution direction")
		}
	})
	t.Run("direction rejected on a single-target card", func(t *testing.T) {
		s := sgThreeBattleState(t)
		card := sgGive(t, s, 0, "slash")
		if err := s.sgUseDirected(0, "slash", []int{card}, []int{1}, false, "ccw"); err == nil {
			t.Fatal("3v3 accepted a direction for 杀")
		}
	})
	t.Run("unknown direction rejected", func(t *testing.T) {
		s := sgThreeBattleState(t)
		card := sgGive(t, s, 0, "archery_attack")
		if err := s.sgUseDirected(0, "archery_attack", []int{card}, []int{1, 2}, false, "sideways"); err == nil {
			t.Fatal("3v3 accepted an unknown direction")
		}
	})
}

func TestSanguoshaThreeV3LordSkillsStayLocked(t *testing.T) {
	identity, err := NewSanguosha(6, SGOptions{Packs: []string{"standard"}})
	if err != nil {
		t.Fatal(err)
	}
	identity.Sanguosha.Players[0].General, identity.Sanguosha.Players[0].Role = "caocao", "lord"
	if !identity.sgHas(0, "hujia") {
		t.Fatal("identity lord lost 护驾")
	}
	s := sgThreeBattleState(t)
	s.Sanguosha.Players[0].General, s.Sanguosha.Players[0].Role = "caocao", "lord"
	if s.sgHas(0, "hujia") || s.sgHas(0, "jianxiong") == false {
		t.Fatal("3v3 unlocked a lord skill or locked a normal skill")
	}
}
