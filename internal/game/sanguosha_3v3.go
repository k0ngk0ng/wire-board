package game

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

type SGThreeV3 struct {
	Rules      string     `json:"rules"`
	Camps      []string   `json:"camps"`      // per player: cold/warm
	Leaders    []int      `json:"leaders"`    // [cold, warm] player index
	Pool       []string   `json:"pool"`       // generals still on the table
	Picked     [][]string `json:"picked"`     // per camp, generals taken in the draft
	PickFirst  int        `json:"pickFirst"`  // camp that picks first
	PickCamp   int        `json:"pickCamp"`   // camp to pick now
	PickIndex  int        `json:"pickIndex"`  // index into sgThreePickCounts
	PickLeft   int        `json:"pickLeft"`   // generals still owed by the picking camp
	AssignCamp int        `json:"assignCamp"` // camp assigning away its three generals
	AssignAt   int        `json:"assignAt"`   // 0 leader, 1 vanguard A, 2 vanguard B
	Assigned   [][]string `json:"assigned"`   // per camp, three generals in seat order
	Stage      string     `json:"stage"`      // pickFirst/pick/assign/firstSide/play
	FirstSide  int        `json:"firstSide"`  // camp that holds the first action right
	Side       int        `json:"side"`       // camp holding the action right
	Acted      []bool     `json:"acted"`      // per player, acted in the current right
	RightsHeld int        `json:"rightsHeld"` // rights completed, two per round
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
		g.Deck = append(g.Deck, c.ID)
	}
	if options.Deck == "military" {
		for _, c := range sgMilitaryCards {
			g.Deck = append(g.Deck, c.ID)
		}
	}
	shuffle(g.Deck)
	ids := []string{}
	for _, x := range g.generalCatalog() {
		ids = append(ids, x.ID)
	}
	shuffle(ids)
	if len(ids) < 16 {
		ids = append(ids, ids...)
	}
	cold := sgThreeCold
	if catanRandom(2) == 1 {
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
		Rules: SGThreeV3Rules, Camps: camps, Leaders: leaders, Pool: clone(ids[:16]),
		Picked: [][]string{{}, {}}, Assigned: [][]string{{}, {}}, Stage: "pickFirst",
		Acted: make([]bool, n), PickFirst: sgThreeWarm, PickCamp: sgThreeWarm,
	}
	s.Turn, s.Phase, g.InPlay = leaders[sgThreeWarm], "sg_select", false
	s.sgLog("三国杀3v3：冷色方主帅%s、前锋%s；暖色方主帅%s、前锋%s",
		s.sgPlayerLabel(leaders[sgThreeCold]), s.sgSeatList(sgThreeCold),
		s.sgPlayerLabel(leaders[sgThreeWarm]), s.sgSeatList(sgThreeWarm))
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
		t.Acted = make([]bool, len(s.Sanguosha.Players))
		next := t.Leaders[camp]
		if a.Choice == "vanguards" {
			vanguards := []int{}
			for _, member := range s.sgThreeAliveMembers(camp) {
				if member != t.Leaders[camp] {
					vanguards = append(vanguards, member)
				}
			}
			if len(vanguards) == 0 {
				return true, errors.New("本方已无前锋可以行动")
			}
			next = vanguards[0]
		}
		s.sgLog("%s 选择%s行动", s.sgName(i), map[string]string{"leader": "主帅", "vanguards": "前锋"}[a.Choice])
		s.sgPush(SGEvent{Type: "begin", Actor: next})
		return true, nil
	}
	return false, nil
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
		for _, id := range t.Picked[camp] {
			_ = id // Reserve generals stay recorded for the scoreboard.
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
	if t.RightsHeld%2 == 0 {
		s.Round++
	}
	t.Acted = make([]bool, len(s.Sanguosha.Players))
	s.sgThreeBeginRight(next, false)
}

func (s *State) sgThreeBeginRight(side int, first bool) {
	t := s.Sanguosha.ThreeV3
	t.Side = side
	if len(s.sgThreeAliveMembers(side)) == 0 {
		s.sgThreePassRight(t.Leaders[1-side])
		return
	}
	if leader := t.Leaders[side]; s.sgAlive(leader) {
		s.sgAsk(leader, "sg_3v3_side", "选择由主帅行动或两名前锋连续行动", SGEvent{})
		return
	}
	// A camp without its leader keeps playing with its vanguards.
	for _, member := range s.sgThreeAliveMembers(side) {
		s.sgPush(SGEvent{Type: "begin", Actor: member})
		return
	}
}

// sgThreeDie settles deaths: a fallen leader ends the game, a vanguard death
// pays the killer two cards.
func (s *State) sgThreeDie(i, killer int) {
	g := s.Sanguosha
	t := g.ThreeV3
	p := &g.Players[i]
	p.Dead = true
	if i == s.Turn {
		s.sgClearSilence()
	}
	s.sgLog("%s 阵亡（%s）", s.sgName(i), sgCampLabel(s.sgThreeCamp(i)))
	s.sgDeathClear(i)
	camp := s.sgThreeCamp(i)
	if i != t.Leaders[camp] {
		if killer >= 0 && killer != i && s.sgAlive(killer) {
			s.sgPush(SGEvent{Type: "draw", Actor: killer, Amount: 2})
		}
		if s.sgThreeIsRightOver(camp) {
			s.sgThreePassRight(t.Leaders[camp])
		}
		return
	}
	other := 1 - camp
	if g.Players[t.Leaders[other]].Dead {
		if winner, ok := s.sgThreeTiedLeaders(); ok {
			s.sgThreeFinish(winner)
			return
		}
		s.Finished, s.Winners = true, []int{}
		s.sgLog("双方主帅同时阵亡且无法分出胜负，本局平局")
		return
	}
	s.sgThreeFinish(other)
}

func (s *State) sgThreeIsRightOver(camp int) bool {
	if camp != s.Sanguosha.ThreeV3.Side {
		return false
	}
	for _, member := range s.sgThreeAliveMembers(camp) {
		if !s.Sanguosha.ThreeV3.Acted[member] {
			return false
		}
	}
	return true
}

// sgThreeTiedLeaders settles both-leaders-down by living members, then by the
// hit points both sides have lost.
func (s *State) sgThreeTiedLeaders() (int, bool) {
	bestCamp, bestAlive, bestLost := -1, -1, -1
	tied := false
	for camp := sgThreeCold; camp <= sgThreeWarm; camp++ {
		alive, lost := 0, 0
		for _, i := range s.sgThreeMembers(camp) {
			if s.sgAlive(i) {
				alive++
				continue
			}
			lost += s.Sanguosha.Players[i].MaxHP
		}
		if alive > bestAlive || alive == bestAlive && lost < bestLost {
			bestCamp, bestAlive, bestLost, tied = camp, alive, lost, false
			continue
		}
		if alive == bestAlive && lost == bestLost {
			tied = true
		}
	}
	if tied || bestCamp < 0 {
		return -1, false
	}
	return bestCamp, true
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
		alive := s.sgThreeAliveMembers(s.sgThreeCamp(i))
		if len(alive) > 1 {
			return Action{Choice: "vanguards", Prompt: q.ID}, true
		}
		return Action{Choice: "leader", Prompt: q.ID}, true
	}
	return Action{}, false
}
