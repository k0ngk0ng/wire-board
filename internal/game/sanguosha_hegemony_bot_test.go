package game

import (
	"reflect"
	"testing"
)

func sgHegBotTeamState() *State {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_caocao", "heg_xiahoudun"})
	sgHegSetPair(s, 1, [2]string{"heg_guojia", "heg_zhenji"})
	s.sgHegShow(0, []int{0, 1}, false)
	s.sgHegShow(1, []int{0}, false)
	s.sgHegShow(3, []int{0}, false) // Public opposition anchors an ongoing game.
	s.Sanguosha.Hegemony.FirstClaimed = true
	return s
}

func TestSanguoshaHegemonyBotTargetsPublicEnemies(t *testing.T) {
	s := sgHegBotTeamState()
	s.Sanguosha.Players[1].HP = 1
	sgGive(t, s, 0, "slash")
	a, err := s.sgBot(0)
	if err != nil || a.Type != "sg_play" || !reflect.DeepEqual(a.Targets, []int{5}) {
		t.Fatal("bot selected the vulnerable adjacent ally", a, err)
	}
	if err := s.Apply(0, a); err != nil {
		t.Fatal(err)
	}
}

func TestSanguoshaHegemonyBotRescueAndCounterspell(t *testing.T) {
	s := sgHegBotTeamState()
	peach := sgGive(t, s, 0, "peach")
	s.Sanguosha.Players[1].HP = 0
	s.sgAsk(0, "peach", "rescue", SGEvent{Actor: 3, Target: 1, Step: 0})
	a, err := s.sgBot(0)
	if err != nil || !reflect.DeepEqual(a.Cards, []int{peach}) {
		t.Fatal("bot did not rescue a public teammate", a, err)
	}
	if err := s.Apply(0, a); err != nil {
		t.Fatal(err)
	}
	if s.Sanguosha.Players[1].HP != 1 {
		t.Fatal("teammate rescue failed")
	}
	s = sgHegBotTeamState()
	counter := sgGive(t, s, 0, "heg_nullification")
	s.sgAsk(-1, "nullification", "counter", SGEvent{Actor: 3, Target: 1, Kind: "duel", Aux: -1, Step: 0})
	a, err = s.sgBot(0)
	if err != nil || !reflect.DeepEqual(a.Cards, []int{counter}) || a.Choice != "faction" {
		t.Fatal("bot did not protect the teammate's faction", a, err)
	}
	s.Sanguosha.Pending.Event.Flag = true
	a, err = s.sgBot(0)
	if err != nil || a.Choice != "pass" {
		t.Fatal("bot restored harmful effect on a teammate", a, err)
	}
}

func TestSanguoshaHegemonyBotDoesNotReadOtherSecrets(t *testing.T) {
	s := sgHegBotTeamState()
	sgGive(t, s, 0, "slash")
	x := sgGive(t, s, 2, "jink")
	y := sgGive(t, s, 4, "peach")
	before := clone(*s)
	a, err := s.sgBot(0)
	if err != nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("planning modified authoritative state", err)
	}
	// Only hidden information changes. Public HP, hand counts, skills,
	// equipment and factions remain identical.
	s.Sanguosha.Players[2].General, s.Sanguosha.Players[2].Hegemony.Deputy = "heg_sunquan", "heg_ganning"
	s.Sanguosha.Players[4].General, s.Sanguosha.Players[4].Hegemony.Deputy = "heg_jiaxu", "heg_jiling"
	s.Sanguosha.Players[2].Hand, s.Sanguosha.Players[4].Hand = []int{y}, []int{x}
	b, err := s.sgBot(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("bot choice depended on another seat's private generals/cards", a, b, err)
	}
}

func TestSanguoshaHegemonyBotHiddenRescueConversion(t *testing.T) {
	s := sgHegBotTeamState()
	sgHegSetPair(s, 2, [2]string{"heg_huatuo", "heg_panfeng"})
	sgHegSetPair(s, 3, [2]string{"heg_lvbu", "heg_jiling"})
	red := sgFindGive(t, s, 2, func(c SGCard) bool { return c.Kind != "peach" && c.Suit == 1 })
	s.Sanguosha.Players[3].HP = 0
	s.sgAsk(2, "peach", "rescue", SGEvent{Actor: 0, Target: 3})
	a, err := s.sgBot(2)
	if err != nil || a.Skill != "jijiu" || !reflect.DeepEqual(a.Cards, []int{red}) || s.sgHegShown(2) {
		t.Fatal("own hidden rescue conversion not planned without revealing", a, err)
	}
	if err := s.Apply(2, a); err != nil {
		t.Fatal(err)
	}
	if !s.sgHas(2, "jijiu") || s.Sanguosha.Players[3].HP != 1 {
		t.Fatal("committed emergency rescue did not reveal and heal")
	}
}

func TestSanguoshaHegemonyBotNationalActiveSkills(t *testing.T) {
	t.Run("Rende supports allies", func(t *testing.T) {
		s := sgHegState(6)
		sgHegSetPair(s, 0, [2]string{"heg_liubei", "heg_guanyu"})
		s.sgHegShow(0, []int{0}, false)
		s.sgHegShow(2, []int{0}, false)
		s.Sanguosha.Players[0].HP = 2
		for range 3 {
			sgGive(t, s, 0, "slash")
		}
		a, err := s.sgBot(0)
		if err != nil || a.Skill != "heg_rende" || !reflect.DeepEqual(a.Targets, []int{2}) || len(a.Cards) != 3 {
			t.Fatal("Rende action", a, err)
		}
	})
	t.Run("mandatory target still completes when all choices are allies", func(t *testing.T) {
		s := sgHegBotTeamState()
		s.sgAsk(0, "heg_shuangren_slash", "mandatory", SGEvent{Actor: 0, Target: 1})
		s.Sanguosha.Pending.Targets = []int{1}
		a, err := s.sgBot(0)
		if err != nil || !reflect.DeepEqual(a.Targets, []int{1}) {
			t.Fatal("bot stuck after taking over a committed mandatory effect", a, err)
		}
	})
}
