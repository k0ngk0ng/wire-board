package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Kept separate from identity roles and from the version of standard generals.
type SGHegemony struct {
	RevealRewards []SGEvent     `json:"revealRewards,omitempty"`
	Version       int           `json:"version"`
	First         int           `json:"first"`
	FirstClaimed  bool          `json:"firstClaimed,omitempty"`
	AwaitEffects  map[int][]int `json:"awaitEffects,omitempty"`
}

type SGHegemonyPlayer struct {
	Luoshen          []int             `json:"luoshen,omitempty"` // References cards still in the shared processing area.
	KnownGenerals    map[int][2]string `json:"knownGenerals,omitempty"`
	Deputy           string            `json:"deputy"`
	Shown            [2]bool           `json:"shown"`
	Lost             [2]bool           `json:"lost"`
	CompanionClaimed bool              `json:"companionClaimed,omitempty"`
	HalfClaimed      bool              `json:"halfClaimed,omitempty"`
}

func (s *State) sgHegemony() bool { return s.Sanguosha != nil && s.Sanguosha.Hegemony != nil }

func (s *State) initSGHegemony(n int) {
	seats := make([]int, n)
	for i := range seats {
		seats[i] = i
	}
	shuffle(seats)
	first := seats[0]
	g := &Sanguosha{
		RulesVersion: 1, ActivePhase: "setup", Selecting: true, Lord: first,
		Hegemony: &SGHegemony{Version: 1, First: first}, Players: make([]SGPlayer, n),
		Discard: []int{}, Table: []int{}, Queue: []SGEvent{},
		Options: SGOptions{Mode: "hegemony", Deck: "hegemony", Packs: []string{"hegemony"}},
	}
	s.Sanguosha = g
	s.Turn, s.Phase = first, "sg_select"
	ids := []string{}
	for _, general := range sgHegemonyGenerals {
		ids = append(ids, general.ID)
	}
	shuffle(ids)
	for i := range g.Players {
		// Seven distinct candidates guarantee a same-kingdom pair among four
		// kingdoms; all eight seats receive disjoint pools from the 60 generals.
		g.Players[i] = SGPlayer{Choices: clone(ids[i*7 : i*7+7]), Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}, Hegemony: &SGHegemonyPlayer{}}
	}
	for _, c := range sgHegemonyCards {
		g.Deck = append(g.Deck, c.ID)
	}
	shuffle(g.Deck)
	s.sgAsk(first, "heg_generals", "先选主将，再选一名同势力副将", SGEvent{})
}

func (s *State) sgHegGeneralIDs(i int) [2]string {
	p := s.Sanguosha.Players[i]
	return [2]string{p.General, p.Hegemony.Deputy}
}

func (s *State) sgHegShown(i int) bool {
	if i < 0 || i >= len(s.Sanguosha.Players) || !s.sgHegemony() {
		return false
	}
	p := s.Sanguosha.Players[i].Hegemony
	return p != nil && (p.Shown[0] || p.Shown[1])
}

func (s *State) sgHegSkills(i int, shownOnly bool) []string {
	p := s.Sanguosha.Players[i]
	if p.Hegemony == nil || p.SkillsLost {
		return []string{}
	}
	out := []string{}
	for slot, id := range s.sgHegGeneralIDs(i) {
		if p.Hegemony.Lost[slot] || shownOnly && !p.Hegemony.Shown[slot] {
			continue
		}
		for _, skill := range sgGeneral(id).Skills {
			if !sgLordSkill(skill) && !slices.Contains(out, skill) {
				out = append(out, skill)
			}
		}
	}
	return out
}

// Friendship requires a public faction. A hidden seat is not yet an ally,
// even if the server privately knows it chose the same kingdom. Careerists
// are each their own faction, never allied to one another.
func (s *State) sgHegFriend(a, b int) bool {
	if a < 0 || b < 0 || a >= len(s.Sanguosha.Players) || b >= len(s.Sanguosha.Players) {
		return false
	}
	if a == b {
		return true
	}
	pa, pb := s.Sanguosha.Players[a], s.Sanguosha.Players[b]
	return s.sgHegShown(a) && s.sgHegShown(b) && pa.Role != "careerist" && pb.Role != "careerist" && pa.Role == pb.Role
}

func sgHegCompanions(a, b string) bool {
	for _, pair := range sgHegemonyCompanions {
		if pair == [2]string{a, b} || pair == [2]string{b, a} {
			return true
		}
	}
	return false
}

func sgHegFactionName(role string) string {
	return map[string]string{"wei": "魏", "shu": "蜀", "wu": "吴", "qun": "群", "careerist": "野心家", "": "未知势力"}[role]
}

// Counts revealed seats including the dead. The first floor(N/2) members keep
// their kingdom; later revealers become careerists. Once declared, the role
// does not change merely because somebody dies.
func (s *State) sgHegAssignFaction(i int) {
	p := &s.Sanguosha.Players[i]
	kingdom := sgGeneral(p.General).Kingdom
	n := 1
	for j, other := range s.Sanguosha.Players {
		if j != i && s.sgHegShown(j) && other.Role == kingdom {
			n++
		}
	}
	p.Role = kingdom
	if n > len(s.Sanguosha.Players)/2 {
		p.Role = "careerist"
	}
}

// The caller controls timing and whether reveal rewards are queued. Death and
// terminal reveals publish both generals without producing fresh rewards.
func (s *State) sgHegShow(i int, slots []int, rewards bool) {
	g := s.Sanguosha
	p := &g.Players[i]
	wasShown := s.sgHegShown(i)
	names := []string{}
	for _, slot := range slots {
		if !p.Hegemony.Shown[slot] {
			p.Hegemony.Shown[slot] = true
			names = append(names, sgGeneral(s.sgHegGeneralIDs(i)[slot]).Name)
		}
	}
	if len(names) == 0 {
		return
	}
	if !wasShown {
		s.sgHegAssignFaction(i)
	}
	s.sgLog("玩家 %d 明置 %s，势力为%s", i+1, strings.Join(names, "、"), sgHegFactionName(p.Role))
	if !rewards || !s.sgAlive(i) {
		return
	}
	// Determine terminal state before offering any rewards, as in GeneralShown.
	if s.sgHegCheckVictory() {
		return
	}
	es := []SGEvent{}
	if !g.Hegemony.FirstClaimed {
		g.Hegemony.FirstClaimed = true
		es = append(es, SGEvent{Type: "heg_reward", Actor: i, Kind: "first"})
	}
	if p.Hegemony.Shown[0] && p.Hegemony.Shown[1] {
		if !p.Hegemony.CompanionClaimed && sgHegCompanions(p.General, p.Hegemony.Deputy) {
			p.Hegemony.CompanionClaimed = true
			es = append(es, SGEvent{Type: "heg_reward", Actor: i, Kind: "companion"})
		}
		if !p.Hegemony.HalfClaimed && (sgGeneral(p.General).HP+sgGeneral(p.Hegemony.Deputy).HP)%2 == 1 {
			p.Hegemony.HalfClaimed = true
			es = append(es, SGEvent{Type: "heg_reward", Actor: i, Kind: "half"})
		}
	}
	g.Hegemony.RevealRewards = append(g.Hegemony.RevealRewards, es...)
}

func (s *State) sgHegCheckVictory() bool {
	g := s.Sanguosha
	if !s.sgHegemony() || g.Selecting || s.Finished {
		return s.Finished
	}
	alive := s.sgOrder(0)
	if len(alive) == 0 {
		return false
	}
	if len(alive) > 1 {
		kingdom := sgGeneral(g.Players[alive[0]].General).Kingdom
		members, hidden := 0, 0
		for _, i := range alive {
			if sgGeneral(g.Players[i].General).Kingdom != kingdom || g.Players[i].Role == "careerist" {
				return false
			}
			if !s.sgHegShown(i) {
				hidden++
			}
		}
		for i, p := range g.Players {
			if s.sgHegShown(i) && p.Role == kingdom {
				members++
			}
		}
		// Do not reveal private generals to settle a game if any would become
		// a separate careerist faction when revealed.
		if hidden > 0 && members+hidden > len(g.Players)/2 {
			return false
		}
	}
	for _, i := range alive {
		s.sgHegShow(i, []int{0, 1}, false)
	}
	s.Winners = nil
	for i := range g.Players {
		if s.sgHegFriend(alive[0], i) {
			s.Winners = append(s.Winners, i)
		}
	}
	s.Finished = true
	s.sgLog("国战结束，%s势力成员共同获胜", sgHegFactionName(g.Players[alive[0]].Role))
	return true
}

func (s *State) sgHegRespond(i int, a Action, q SGPrompt) (bool, error) {
	if !s.sgHegemony() {
		return false, nil
	}
	if q.Kind == "draw_phase" && a.Choice == "yingzi+haoshi" {
		for _, skill := range []string{"yingzi", "haoshi"} {
			if err := s.sgHegRevealSkill(i, skill); err != nil {
				return true, err
			}
		}
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: 5}, SGEvent{Type: "haoshi_give", Actor: i})
		return true, nil
	}
	if skill := sgHegResponseSkill(q, a); skill != "" {
		if err := s.sgHegRevealSkill(i, skill); err != nil {
			return true, err
		}
	}
	if handled, err := s.sgHegLockedRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgHegCardRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgHegLuoshenRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgHegTriggerRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgHegRulesRespond(i, a, q); handled {
		return true, err
	}
	if handled, err := s.sgHegRevealRespond(i, a, q); handled {
		return true, err
	}
	g := s.Sanguosha
	p := &g.Players[i]
	switch q.Kind {
	case "heg_generals":
		ids := strings.Split(a.Choice, "+")
		if len(ids) != 2 || ids[0] == ids[1] || !slices.Contains(p.Choices, ids[0]) || !slices.Contains(p.Choices, ids[1]) || sgGeneral(ids[0]).Kingdom != sgGeneral(ids[1]).Kingdom {
			return true, errors.New("请选择候选中两名不同的同势力武将，依次作为主将和副将")
		}
		p.General, p.Hegemony.Deputy = ids[0], ids[1]
		p.MaxHP = (sgGeneral(ids[0]).HP + sgGeneral(ids[1]).HP) / 2
		p.HP, p.Choices = p.MaxHP, nil
		g.Selected++
		if g.Selected < len(g.Players) {
			s.sgAsk(s.sgNext(i), "heg_generals", "先选主将，再选一名同势力副将", SGEvent{})
		} else {
			g.Selecting = false
			for _, who := range s.sgOrder(g.Hegemony.First) {
				s.sgDraw(who, 4)
			}
			s.sgPush(SGEvent{Type: "begin", Actor: g.Hegemony.First})
		}
		return true, nil
	case "heg_reward":
		if !slices.Contains(q.Choices, a.Choice) {
			return true, errors.New("请选择此亮将奖励的合法选项")
		}
		switch a.Choice {
		case "draw":
			n := 2
			if q.Event.Kind == "half" {
				n = 1
			}
			s.sgDraw(i, n)
		case "heal":
			s.sgHeal(i, 1)
		}
		return true, nil
	}
	return false, nil
}

func (s *State) sgHegEvent(e SGEvent) bool {
	if !s.sgHegemony() {
		return false
	}
	if s.sgHegLockedEvent(e) {
		return true
	}
	if s.sgHegSavageEvent(e) {
		return true
	}
	if s.sgHegCardEvent(e) {
		return true
	}
	if s.sgHegLuoshenEvent(e) {
		return true
	}
	if s.sgHegTriggerEvent(e) {
		return true
	}
	if s.sgHegRulesEvent(e) {
		return true
	}
	if s.sgHegRevealEvent(e) {
		return true
	}
	if e.Type == "heg_xiongyi_heal" {
		if s.sgAlive(e.Actor) && s.sgHegSmallestFaction(e.Actor) {
			s.sgHeal(e.Actor, 1)
		}
		return true
	}
	if e.Type == "heg_reward" {
		if s.sgAlive(e.Actor) {
			label := map[string]string{"first": "首亮：可摸两张牌", "companion": "珠联璧合：可摸两张牌或回复一点体力", "half": "阴阳鱼：可摸一张牌"}[e.Kind]
			s.sgAsk(e.Actor, "heg_reward", label, e)
			s.Sanguosha.Pending.Choices = []string{"draw", "pass"}
			p := s.Sanguosha.Players[e.Actor]
			if e.Kind == "companion" && p.HP < p.MaxHP {
				s.Sanguosha.Pending.Choices = append(s.Sanguosha.Pending.Choices, "heal")
			}
		}
		return true
	}
	return false
}

func (s *State) sgHegName(i int) string {
	names := []string{}
	for slot, id := range s.sgHegGeneralIDs(i) {
		if s.Sanguosha.Players[i].Hegemony.Shown[slot] {
			names = append(names, sgGeneral(id).Name)
		}
	}
	if len(names) == 0 {
		return fmt.Sprintf("玩家 %d（暗将）", i+1)
	}
	return fmt.Sprintf("玩家 %d（%s）", i+1, strings.Join(names, "／"))
}

func (s *State) sgHegDie(i, killer int) {
	g := s.Sanguosha
	p := &g.Players[i]
	p.Dead = true
	if i == s.Turn {
		s.sgClearSilence()
	}
	s.sgHegShow(i, []int{0, 1}, false)
	s.sgLog("%s 阵亡，势力为%s", s.sgName(i), sgHegFactionName(p.Role))
	if s.sgHegCheckVictory() {
		s.sgDeathClear(i)
		return
	}
	es := []SGEvent{}
	for _, who := range s.sgOrder(s.Turn) {
		es = append(es, SGEvent{Type: "heg_suishi", Actor: who, Target: i, Kind: "death"})
	}
	if slices.Contains(s.sgHegSkills(i, true), "heg_duanchang") && killer >= 0 && killer < len(g.Players) {
		es = append(es, SGEvent{Type: "heg_duanchang", Actor: i, Target: killer})
	}
	es = append(es, SGEvent{Type: "death_loot", Actor: killer, Target: i})
	s.sgPush(es...)
}

func (s *State) sgHegDeathReward(victim, killer int) {
	if !s.sgAlive(killer) || !s.sgHegShown(killer) {
		return
	}
	g := s.Sanguosha
	if s.sgHegFriend(victim, killer) {
		ids := append(clone(g.Players[killer].Hand), g.Players[killer].Equip...)
		s.sgDiscard(killer, ids)
		s.sgLog("%s 击杀同势力角色，弃置所有手牌和装备", s.sgName(killer))
		return
	}
	n := 1
	for _, who := range s.sgOrder(0) {
		if who != victim && s.sgHegFriend(victim, who) {
			n++
		}
	}
	s.sgDraw(killer, n)
	s.sgLog("%s 获得击杀奖励：%d 张牌", s.sgName(killer), n)
}
