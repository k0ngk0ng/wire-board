package sanguosha

import (
	"errors"
	"slices"
	"strconv"
)

// 3v3 legacy competitive rules: six players split into cold and warm camps of
// one leader and two vanguards, a shared sixteen-general draft, an action right
// the camp leader assigns, and a game that ends the moment a leader dies.
// Rule sources and the site's reading are recorded in
// docs/research/sanguosha-3v3-rules.md.
const SGThreeV3Rules = "sanguosha-3v3-legacy"

const (
	sgThreeCold = 0
	sgThreeWarm = 1
)

// Printed seat order: cold vanguard A, cold leader, cold vanguard B, warm
// vanguard B, warm leader, warm vanguard A.
var sgThreeSeatRoles = [6]struct {
	camp     int
	vanguard bool
}{{sgThreeCold, true}, {sgThreeCold, false}, {sgThreeCold, true}, {sgThreeWarm, true}, {sgThreeWarm, false}, {sgThreeWarm, true}}

var sgThreePickCounts = []int{1, 2, 2, 2, 2, 2, 2, 2, 1}

// The printed draft splits the 32 general cards into four face-down piles of
// eight; each leader takes one pile and those sixteen are laid out.
const (
	sgThreePileCount = 4
	sgThreePileSize  = 8
)

type SGThreeV3 struct {
	Rules      string     `json:"rules"`
	Variant    string     `json:"variant,omitempty"`   // official / extended
	Camps      []string   `json:"camps"`               // per player: cold/warm
	Leaders    []int      `json:"leaders"`             // [cold, warm] player index
	Pool       []string   `json:"pool"`                // generals still on the table
	Picked     [][]string `json:"picked"`              // per camp, generals taken in the draft
	PickFirst  int        `json:"pickFirst"`           // camp that picks first
	PickCamp   int        `json:"pickCamp"`            // camp to pick now
	PickIndex  int        `json:"pickIndex"`           // index into sgThreePickCounts
	PickLeft   int        `json:"pickLeft"`            // generals still owed by the picking camp
	AssignCamp int        `json:"assignCamp"`          // camp assigning away its three generals
	AssignAt   int        `json:"assignAt"`            // 0 leader, 1 vanguard A, 2 vanguard B
	Assigned   [][]string `json:"assigned"`            // per camp, three generals in seat order
	Stage      string     `json:"stage"`               // pickFirst/pick/assign/firstSide/play
	FirstSide  int        `json:"firstSide"`           // camp that holds the first action right
	Side       int        `json:"side"`                // camp holding the action right
	Acted      []bool     `json:"acted"`               // per player, acted in the current round
	RightsHeld int        `json:"rightsHeld"`          // action units completed over the game
	Piles      [][]string `json:"piles,omitempty"`     // face-down piles, public once both are taken
	PilePicks  []int      `json:"pilePicks,omitempty"` // per camp, chosen pile index or -1
	PileTurn   int        `json:"pileTurn,omitempty"`  // camp choosing a pile now
}

// sgThreeOfficialGenerals is the printed competitive general range: the 25
// standard generals plus the seven wind generals, without Yuji. The god pack
// stays out even though identity rooms may enable it.
func sgThreeOfficialGenerals() []string {
	ids := []string{}
	for _, general := range SGGenerals {
		ids = append(ids, general.ID)
	}
	for _, general := range sgWindGenerals {
		if general.ID != "yuji" {
			ids = append(ids, general.ID)
		}
	}
	return ids
}

// sgThreeOpenPiles lists the piles a leader may still take, as prompt choices.
func sgThreeOpenPiles(t *SGThreeV3) []string {
	out := []string{}
	for i := range t.Piles {
		if slices.Contains(t.PilePicks, i) {
			continue
		}
		out = append(out, strconv.Itoa(i+1))
	}
	return out
}

func (s *State) sgThreeV3() bool {
	return s.Sanguosha != nil && s.Sanguosha.ThreeV3 != nil
}

func (s *State) sgThreeCamp(i int) int {
	g := s.Sanguosha
	if !s.sgThreeV3() || i < 0 || i >= len(g.ThreeV3.Camps) {
		return -1
	}
	if g.ThreeV3.Camps[i] == "cold" {
		return sgThreeCold
	}
	return sgThreeWarm
}

func (s *State) sgThreeLeader(camp int) int {
	if !s.sgThreeV3() || camp < 0 || camp > 1 {
		return -1
	}
	return s.Sanguosha.ThreeV3.Leaders[camp]
}

func (s *State) sgThreeMembers(camp int) []int {
	out := []int{}
	for i := range s.Sanguosha.Players {
		if s.sgThreeCamp(i) == camp {
			out = append(out, i)
		}
	}
	return out
}

func (s *State) sgThreeAliveMembers(camp int) []int {
	out := []int{}
	for _, i := range s.sgThreeMembers(camp) {
		if s.sgAlive(i) {
			out = append(out, i)
		}
	}
	return out
}

// sgThreeIsLeader reports whether the seat holds its camp's leader card.
func (s *State) sgThreeIsLeader(i int) bool {
	camp := s.sgThreeCamp(i)
	return camp >= 0 && s.sgThreeLeader(camp) == i
}

func (s *State) initSGThreeV3(n int, options SGOptions) {
	if n != 6 {
		n = 6
	}
	g := &Sanguosha{
		RulesVersion: 1, ActivePhase: "setup", Selecting: true,
		Players: make([]SGPlayer, n), Discard: []int{}, Table: []int{}, Queue: []SGEvent{},
		Options: options,
	}
	for i := range g.Players {
		g.Players[i] = SGPlayer{Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}}
	}
	s.Sanguosha = g
	for _, c := range sgCards {
		// The printed 3v3 deck is the standard 108 without Lightning.
		if options.ThreeVariant != SGThreeExtended && c.Kind == "lightning" {
			continue
		}
		g.Deck = append(g.Deck, c.ID)
	}
	if options.Deck == "military" && options.ThreeVariant == SGThreeExtended {
		for _, c := range sgMilitaryCards {
			g.Deck = append(g.Deck, c.ID)
		}
	}
	shuffle(g.Deck)
	ids := []string{}
	if options.ThreeVariant == SGThreeExtended {
		for _, x := range g.generalCatalog() {
			ids = append(ids, x.ID)
		}
		shuffle(ids)
		if len(ids) < 16 {
			ids = append(ids, ids...)
		}
	} else {
		ids = sgThreeOfficialGenerals()
		shuffle(ids)
	}
	cold := sgThreeCold
	if sgRandom(2) == 1 {
		cold = sgThreeWarm
	}
	camps := make([]string, n)
	leaders := make([]int, 2)
	for seat, role := range sgThreeSeatRoles {
		camp := role.camp
		if cold == sgThreeWarm {
			camp = 1 - camp
		}
		if camp == sgThreeCold {
			camps[seat] = "cold"
		} else {
			camps[seat] = "warm"
		}
		if !role.vanguard {
			leaders[camp] = seat
		}
	}
	g.ThreeV3 = &SGThreeV3{
		Rules: SGThreeV3Rules, Variant: options.ThreeVariant, Camps: camps, Leaders: leaders,
		Picked: [][]string{{}, {}}, Assigned: [][]string{{}, {}}, Stage: "pickFirst",
		Acted: make([]bool, n), PickFirst: sgThreeWarm, PickCamp: sgThreeWarm,
	}
	if options.ThreeVariant == SGThreeExtended {
		g.ThreeV3.Pool = clone(ids[:16])
	} else {
		piles := [][]string{}
		for p := 0; p < sgThreePileCount; p++ {
			piles = append(piles, clone(ids[p*sgThreePileSize:(p+1)*sgThreePileSize]))
		}
		g.ThreeV3.Piles = piles
		g.ThreeV3.PilePicks = []int{-1, -1}
		g.ThreeV3.PileTurn = sgThreeWarm
		g.ThreeV3.Stage = "piles"
	}
	s.Turn, s.Phase, g.InPlay = leaders[sgThreeWarm], "sg_select", false
	s.sgLog("三国杀3v3：冷色方主帅%s、前锋%s；暖色方主帅%s、前锋%s",
		s.sgPlayerLabel(leaders[sgThreeCold]), s.sgSeatList(sgThreeCold),
		s.sgPlayerLabel(leaders[sgThreeWarm]), s.sgSeatList(sgThreeWarm))
	if g.ThreeV3.Stage == "piles" {
		s.sgAsk(leaders[sgThreeWarm], "sg_3v3_pile", "选择本方要取走的武将堆（暗置，剩余4堆各8名）", SGEvent{})
		return
	}
	s.sgAsk(leaders[sgThreeWarm], "sg_3v3_first", "选择本局先选将的一方", SGEvent{})
}

func (s *State) sgPlayerLabel(i int) string {
	return "玩家" + string(rune('1'+i))
}

func (s *State) sgSeatList(camp int) string {
	out := ""
	for _, i := range s.sgThreeMembers(camp) {
		if s.sgThreeIsLeader(i) {
			continue
		}
		if out != "" {
			out += "、"
		}
		out += s.sgPlayerLabel(i)
	}
	return out
}

// sgThreeRespond handles every draft and action-right prompt of the mode.
func (s *State) sgThreeRespond(i int, a Action, q SGPrompt) (bool, error) {
	if !s.sgThreeV3() {
		return false, nil
	}
	t := s.Sanguosha.ThreeV3
	switch q.Kind {
	case "sg_3v3_pile":
		if i != t.Leaders[t.PileTurn] {
			return true, errors.New("只有轮到的主帅可以选择武将堆")
		}
		index, convErr := strconv.Atoi(a.Choice)
		if convErr != nil || index < 1 || index > len(t.Piles) {
			return true, errors.New("请选择有效的武将堆")
		}
		pick := index - 1
		if slices.Contains(t.PilePicks, pick) {
			return true, errors.New("该武将堆已被取走")
		}
		t.PilePicks[t.PileTurn] = pick
		if t.PileTurn == sgThreeWarm {
			t.PileTurn = sgThreeCold
			s.sgThreeAskPile()
			return true, nil
		}
		// Both piles are taken: lay their sixteen generals out for the draft.
		t.Pool = append(clone(t.Piles[t.PilePicks[sgThreeWarm]]), t.Piles[t.PilePicks[sgThreeCold]]...)
		t.Stage = "pickFirst"
		s.sgLog("分堆选将：%s取第%d堆、%s取第%d堆，共16名武将平摊待选",
			s.sgCampName(sgThreeWarm), t.PilePicks[sgThreeWarm]+1,
			s.sgCampName(sgThreeCold), t.PilePicks[sgThreeCold]+1)
		s.sgThreeAskFirst()
		return true, nil
	case "sg_3v3_first":
		if a.Choice != "us" && a.Choice != "them" {
			return true, errors.New("请选择我方先选或对方先选")
		}
		first := sgThreeWarm
		if a.Choice == "them" {
			first = sgThreeCold
		}
		t.PickFirst, t.PickCamp, t.PickIndex, t.Stage = first, first, 0, "pick"
		t.PickLeft = sgThreePickCounts[0]
		s.sgLog("%s 选择先选将", s.sgCampName(first))
		s.sgThreeAskPick()
		return true, nil
	case "sg_3v3_pick":
		if !slices.Contains(t.Pool, a.Choice) {
			return true, errors.New("该武将不在本局选将池中")
		}
		at := slices.Index(t.Pool, a.Choice)
		t.Pool = slices.Delete(t.Pool, at, at+1)
		t.Picked[t.PickCamp] = append(t.Picked[t.PickCamp], a.Choice)
		t.PickLeft--
		if t.PickLeft > 0 {
			s.sgThreeAskPick()
			return true, nil
		}
		t.PickIndex++
		if t.PickIndex >= len(sgThreePickCounts) {
			t.Stage, t.AssignCamp, t.AssignAt = "assign", sgThreeCold, 0
			s.sgLog("选将结束：冷色方 %d 名、暖色方 %d 名", len(t.Picked[sgThreeCold]), len(t.Picked[sgThreeWarm]))
			s.sgThreeAskAssign()
			return true, nil
		}
		t.PickCamp = 1 - t.PickCamp
		t.PickLeft = sgThreePickCounts[t.PickIndex]
		s.sgThreeAskPick()
		return true, nil
	case "sg_3v3_assign":
		camp := min(t.AssignCamp, sgThreeWarm)
		if i != t.Leaders[camp] {
			return true, errors.New("只有本方主帅可以分配武将")
		}
		if !slices.Contains(t.Picked[camp], a.Choice) {
			return true, errors.New("只能分配本方已选到的武将")
		}
		for _, taken := range t.Assigned[camp] {
			if taken == a.Choice {
				return true, errors.New("同一名武将不能分配给两名玩家")
			}
		}
		t.Assigned[camp] = append(t.Assigned[camp], a.Choice)
		t.AssignAt++
		if t.AssignAt < 3 {
			s.sgThreeAskAssign()
			return true, nil
		}
		if camp == sgThreeWarm {
			s.sgThreeStartPlay()
			return true, nil
		}
		t.AssignCamp, t.AssignAt = sgThreeWarm, 0
		s.sgThreeAskAssign()
		return true, nil
	case "sg_3v3_first_side":
		if i != t.Leaders[sgThreeCold] {
			return true, errors.New("由冷色方主帅决定先手")
		}
		if a.Choice != "cold" && a.Choice != "warm" {
			return true, errors.New("请选择先手阵营")
		}
		side := sgThreeCold
		if a.Choice == "warm" {
			side = sgThreeWarm
		}
		t.FirstSide, t.Stage = side, "play"
		s.sgLog("%s 先手", s.sgCampName(side))
		s.sgThreeBeginRight(side, true)
		return true, nil
	case "sg_3v3_side":
		camp := s.sgThreeCamp(i)
		if camp != t.Side || i != t.Leaders[camp] {
			return true, errors.New("只有当前行动权阵营的主帅可以选择行动顺序")
		}
		if a.Choice != "leader" && a.Choice != "vanguards" {
			return true, errors.New("请选择主帅行动或前锋行动")
		}
		next := t.Leaders[camp]
		if !slices.Contains(s.sgThreeRightChoices(camp), a.Choice) {
			return true, errors.New("该行动单元本轮已完成，请选择尚未行动的角色")
		}
		if a.Choice == "vanguards" {
			if !slices.Contains(s.sgThreeReadyVanguards(camp), a.Target) {
				return true, errors.New("请选择本方尚未行动的前锋先行")
			}
			next = a.Target
		}
		s.sgLog("%s 选择%s行动", s.sgName(i), map[string]string{"leader": "主帅", "vanguards": "前锋"}[a.Choice])
		s.sgPush(SGEvent{Type: "begin", Actor: next})
		return true, nil
	}
	return false, nil
}

// sgThreeAskFirst asks the warm leader who drafts first, the printed warm
// initiative of the selection phase.
func (s *State) sgThreeAskFirst() {
	s.sgAsk(s.Sanguosha.ThreeV3.Leaders[sgThreeWarm], "sg_3v3_first", "选择本局先选将的一方", SGEvent{})
}

// sgThreeAskPile asks the leader whose turn it is to take one of the remaining
// face-down piles.
func (s *State) sgThreeAskPile() {
	t := s.Sanguosha.ThreeV3
	s.sgAsk(t.Leaders[t.PileTurn], "sg_3v3_pile",
		"选择本方要取走的武将堆（暗置，剩余"+strconv.Itoa(len(sgThreeOpenPiles(t)))+"堆）", SGEvent{})
}

func (s *State) sgThreeAskPick() {
	t := s.Sanguosha.ThreeV3
	s.sgAsk(t.Leaders[t.PickCamp], "sg_3v3_pick", "为本方选择武将（本次还需 "+strconv.Itoa(t.PickLeft)+" 名）", SGEvent{})
}

// sgThreeAskAssign walks the leader through leader, vanguard A, vanguard B.
func (s *State) sgThreeAskAssign() {
	t := s.Sanguosha.ThreeV3
	slot := []string{"主帅", "前锋A", "前锋B"}[min(t.AssignAt, 2)]
	s.sgAsk(t.Leaders[t.AssignCamp], "sg_3v3_assign", "为本方"+slot+"选择出场武将", SGEvent{})
}

func (s *State) sgThreeStartPlay() {
	t := s.Sanguosha.ThreeV3
	g := s.Sanguosha
	for camp := sgThreeCold; camp <= sgThreeWarm; camp++ {
		members := s.sgThreeMembers(camp)
		vanguards := []int{}
		for _, member := range members {
			if member != t.Leaders[camp] {
				vanguards = append(vanguards, member)
			}
		}
		if len(vanguards) != 2 || len(t.Assigned[camp]) != 3 {
			return
		}
		order := []int{t.Leaders[camp]}
		slices.Sort(vanguards)
		order = append(order, vanguards...)
		for at, id := range t.Assigned[camp] {
			p := &g.Players[order[at]]
			p.General = id
			p.MaxHP = sgGeneral(id).HP
			if at == 0 {
				p.MaxHP++ // The leader card adds one maximum hit point.
			}
			p.HP = p.MaxHP
		}
	}
	g.Selecting = false
	t.Stage = "firstSide"
	s.sgLog("选将完成，进入对局")
	s.sgAsk(s.sgThreeLeader(sgThreeCold), "sg_3v3_first_side", "选择先手阵营", SGEvent{})
}

// sgThreeContinueRight hands the current right to the camp's second vanguard.
func (s *State) sgThreeContinueRight(actor int) bool {
	t := s.Sanguosha.ThreeV3
	camp := s.sgThreeCamp(actor)
	if camp < 0 {
		return false
	}
	t.Acted[actor] = true
	if actor == t.Leaders[camp] {
		return false
	}
	for _, member := range s.sgThreeAliveMembers(camp) {
		if member != t.Leaders[camp] && !t.Acted[member] {
			s.sgPush(SGEvent{Type: "begin", Actor: member})
			return true
		}
	}
	return false
}

// sgThreePassRight moves the right to the other camp and asks its leader.
func (s *State) sgThreePassRight(actor int) {
	t := s.Sanguosha.ThreeV3
	camp := s.sgThreeCamp(actor)
	if camp < 0 {
		camp = t.Side
	}
	next := 1 - camp
	t.Side = next
	t.RightsHeld++
	if s.sgThreeIsRightOver(sgThreeCold) && s.sgThreeIsRightOver(sgThreeWarm) {
		s.Round++
		clear(t.Acted)
	} else if s.sgThreeIsRightOver(next) {
		next = camp
	}
	s.sgThreeBeginRight(next, false)
}

func (s *State) sgThreeBeginRight(side int, first bool) {
	t := s.Sanguosha.ThreeV3
	t.Side = side
	if first {
		s.sgGameStartFrom(t.Leaders[side])
		return
	}
	if leader := t.Leaders[side]; s.sgAlive(leader) {
		s.sgAsk(leader, "sg_3v3_side", "选择本轮尚未行动的主帅或前锋；选择前锋时指定先行者", SGEvent{})
		s.Sanguosha.Pending.Choices = s.sgThreeRightChoices(side)
		s.Sanguosha.Pending.Targets = s.sgThreeReadyVanguards(side)
		return
	}
	// A leader death ends the match; it never grants a vanguard-only turn.
	s.sgThreeFinish(1 - side)
}

func (s *State) sgThreeReadyVanguards(camp int) []int {
	t := s.Sanguosha.ThreeV3
	ready := []int{}
	for _, member := range s.sgThreeAliveMembers(camp) {
		if member != t.Leaders[camp] && !t.Acted[member] {
			ready = append(ready, member)
		}
	}
	return ready
}

func (s *State) sgThreeRightChoices(camp int) []string {
	t := s.Sanguosha.ThreeV3
	choices := []string{}
	if leader := t.Leaders[camp]; s.sgAlive(leader) && !t.Acted[leader] {
		choices = append(choices, "leader")
	}
	if len(s.sgThreeReadyVanguards(camp)) > 0 {
		choices = append(choices, "vanguards")
	}
	return choices
}

// sgThreeDie settles deaths: a fallen leader ends the game, a vanguard death
// pays the killer two cards.
func (s *State) sgThreeDie(i, killer int) {
	g := s.Sanguosha
	t := g.ThreeV3
	p := &g.Players[i]
	duanchang, wuhun := s.sgHas(i, "duanchang"), s.sgHas(i, "wuhun")
	p.Dead = true
	if i == s.Turn {
		s.sgClearSilence()
	}
	s.sgGodDeath(i, killer)
	s.sgLog("%s 阵亡（%s）", s.sgName(i), sgCampLabel(s.sgThreeCamp(i)))
	camp := s.sgThreeCamp(i)
	if i == t.Leaders[camp] {
		s.sgDeathClear(i)
		s.sgThreeFinish(1 - camp)
		return
	}
	// Keep the common death ordering: Wuhun before inheritance, then zone
	// cleanup and the mode-specific reward. A nested leader death stops it all.
	s.sgPush(SGEvent{Type: "death_loot", Actor: killer, Target: i})
	if wuhun {
		s.sgPush(SGEvent{Type: "wuhun", Actor: i})
	}
	if duanchang {
		s.sgLoseSkills(killer)
	}
}

// sgThreeAllyFirstDirection returns the group-trick direction that reaches the
// user's own camp before the enemy camp, or "" for the clockwise default. Only
// 五谷丰登 makes use of it: its revealed cards are taken in resolution order, so
// the earlier seats choose first. Both rings start at the user, so the camp
// whose first opponent sits later wins the comparison.
func (s *State) sgThreeAllyFirstDirection(i int) string {
	if !s.sgThreeV3() {
		return ""
	}
	camp := s.sgThreeCamp(i)
	order := s.sgOrder(i)
	firstOpponent := func(seats []int) int {
		for pos, seat := range seats {
			if pos > 0 && s.sgThreeCamp(seat) != camp {
				return pos
			}
		}
		return len(seats)
	}
	back := slices.Clone(order)
	slices.Reverse(back)
	if firstOpponent(back) > firstOpponent(order) {
		return "ccw"
	}
	return ""
}

// sgThreeDirectedTrick reports the group tricks whose 3v3 version lets the
// user choose the resolution direction.
func sgThreeDirectedTrick(kind string) bool {
	return slices.Contains([]string{"amazing_grace", "god_salvation", "savage_assault", "archery_attack"}, kind)
}

func (s *State) sgThreeIsRightOver(camp int) bool {
	for _, member := range s.sgThreeAliveMembers(camp) {
		if !s.Sanguosha.ThreeV3.Acted[member] {
			return false
		}
	}
	return true
}

func (s *State) sgThreeFinish(winner int) {
	s.Finished = true
	s.Winners = s.sgThreeMembers(winner)
	s.sgLog("%s获胜", s.sgCampName(winner))
}

func (s *State) sgCampName(camp int) string {
	return sgCampLabel(camp)
}

func sgCampLabel(camp int) string {
	if camp == sgThreeCold {
		return "冷色方"
	}
	return "暖色方"
}

// sgThreeBotAction answers the draft and action-right prompts. The choices stay
// public-information only: table order for picks, camp order for assignments,
// vanguards while both are alive and the leader otherwise.
func (s *State) sgThreeBotAction(i int) (Action, bool) {
	g := s.Sanguosha
	t := g.ThreeV3
	q := g.Pending
	if q == nil || q.Player != i {
		return Action{}, false
	}
	switch q.Kind {
	case "sg_3v3_pile":
		if open := sgThreeOpenPiles(t); len(open) > 0 {
			return Action{Choice: open[0], Prompt: q.ID}, true
		}
		return Action{}, false
	case "sg_3v3_first":
		return Action{Choice: "us", Prompt: q.ID}, true
	case "sg_3v3_pick":
		best, bestHP := "", -1
		for _, id := range t.Pool {
			if hp := sgGeneral(id).HP; hp > bestHP {
				best, bestHP = id, hp
			}
		}
		if best == "" {
			return Action{}, false
		}
		return Action{Choice: best, Prompt: q.ID}, true
	case "sg_3v3_assign":
		camp := min(t.AssignCamp, sgThreeWarm)
		for _, id := range t.Picked[camp] {
			if !slices.Contains(t.Assigned[camp], id) {
				return Action{Choice: id, Prompt: q.ID}, true
			}
		}
		return Action{}, false
	case "sg_3v3_first_side":
		return Action{Choice: "cold", Prompt: q.ID}, true
	case "sg_3v3_side":
		if ready := s.sgThreeReadyVanguards(s.sgThreeCamp(i)); len(ready) > 0 {
			return Action{Choice: "vanguards", Target: ready[0], Prompt: q.ID}, true
		}
		return Action{Choice: "leader", Prompt: q.ID}, true
	}
	return Action{}, false
}
