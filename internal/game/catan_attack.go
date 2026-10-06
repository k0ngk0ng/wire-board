package game

import (
	"errors"
	"fmt"
	"slices"
)

type catanAttackLandingRoll struct {
	Dice  [2]int `json:"dice"`
	Tiles []int  `json:"tiles"`
}
type catanAttackLandingRecord struct {
	ID     int                      `json:"id"`
	Player int                      `json:"player"`
	Rolls  []catanAttackLandingRoll `json:"rolls"`
}

// Internal constructor until special-card and end-turn battle acceptance.
// No public waiting-room recipe may select the incomplete scenario.
func newCatanAttackState(n int, options CatanOptions) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("蛮族进攻助手组合尚未核对")
	}
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	board, m, err := newCatanAttackBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	s.Log = []string{"蛮族进攻：起始先建村庄，再逆序建城市；城市只领每相邻地块1张起始资源", "蛮族进攻：不使用强盗或最大骑士军队；自己回合达到12分获胜"}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func (s *State) validateCatanAttack() error {
	g := s.Catan
	if g == nil || g.Attack == nil {
		return nil
	}
	a := g.Attack
	n := len(g.Players)
	if n < 3 || n > 6 || g.Two != nil || g.Caravans != nil || g.Rivers != nil || g.Fishing != nil || g.Seafarers != nil || g.CitiesKnights != nil || g.BaseSetup != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.CardEvent != nil || g.RevealedEvent != nil || g.Options.Helpers || g.Options.AllHelpers || (n > 4) != g.Options.FiveSix || (n > 4) != (g.Paired != nil) {
		return errors.New("蛮族进攻人数或尚未接入的组合无效")
	}
	if err := a.validate(g); err != nil {
		return err
	}
	if g.Robber != -1 || g.ArmyOwner != -1 || len(g.DevDeck) != 0 || len(g.DevDiscard) != 0 || a.Bought < 0 || a.Bought > 2 || a.Sequence < 0 || (a.Landing == nil) != (a.Sequence == 0) {
		return errors.New("蛮族进攻基础牌堆、强盗或登陆状态无效")
	}
	if g.setup() && (a.Bought != 0 || a.Sequence != 0 || len(a.Knights) > 0) {
		return errors.New("起始建设不能触发登陆或骑士行动")
	}
	if !slices.Contains([]string{"catan_setup_settlement", "catan_setup_city", "catan_setup_road", "catan_roll", "catan_turn", "catan_discard", "catan_steal", "finished"}, s.Phase) {
		return errors.New("蛮族进攻阶段无效")
	}
	supply := 19
	if n > 4 {
		supply = 24
	}
	if !catanBundle(g.Bank) {
		return errors.New("资源银行无效")
	}
	total := slices.Clone(g.Bank)
	for _, p := range g.Players {
		if !catanBundle(p.Resources) || !catanBundle(p.Dev) || !catanBundle(p.NewDev) || sum(p.Dev) != 0 || sum(p.NewDev) != 0 || p.Knights != 0 {
			return errors.New("蛮族进攻不能持有基础发展卡或军队计数")
		}
		for c, count := range p.Resources {
			total[c] += count
		}
	}
	for _, count := range total {
		if count != supply {
			return errors.New("资源库存不守恒")
		}
	}
	if q := a.Landing; q != nil {
		if q.ID != a.Sequence || q.Player < 0 || q.Player >= n || len(q.Rolls) > 3 {
			return errors.New("登陆记录无效")
		}
		seen := map[int]bool{}
		for _, roll := range q.Rolls {
			total := roll.Dice[0] + roll.Dice[1]
			if roll.Dice[0] < 1 || roll.Dice[0] > 6 || roll.Dice[1] < 1 || roll.Dice[1] > 6 || total == 7 || seen[total] || len(roll.Tiles) > 2 {
				return errors.New("登陆点数记录无效")
			}
			seen[total] = true
			for i, id := range roll.Tiles {
				if !slices.Contains(a.Map.Coast, id) || g.Tiles[id].Number != total || slices.Contains(roll.Tiles[:i], id) {
					return errors.New("登陆地块记录无效")
				}
			}
		}
	}
	return nil
}

// Resolve every required landing immediately. Dice only select coastal
// numbers; they do not advance production RollID or overwrite its dice.
// Compute off to the side so a supply/rule error cannot partly place pieces.
func (s *State) catanAttackLanding(roll func() [2]int) error {
	g := s.Catan
	a := g.Attack
	if a == nil || g.setup() || s.Phase != "catan_turn" || s.Finished {
		return errors.New("当前不能进行蛮族登陆")
	}
	counts := slices.Clone(a.Barbarians)
	remaining := a.supply()
	event := &catanAttackLandingRecord{ID: a.Sequence + 1, Player: s.Turn, Rolls: []catanAttackLandingRoll{}}
	used := map[int]bool{}
	for len(event.Rolls) < 3 && remaining > 0 {
		dice := roll()
		total := dice[0] + dice[1]
		if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
			return errors.New("登陆骰子无效")
		}
		if total == 7 || used[total] {
			continue
		}
		used[total] = true
		targets := []int{}
		for _, id := range a.Map.Coast {
			if g.Tiles[id].Number == total && counts[id] < 3 {
				targets = append(targets, id)
			}
		}
		if len(targets) > remaining {
			// 5–6 has two matching coastal 5s/9s. No invented priority for a last
			// single piece: this unresolved rule boundary remains a release gate.
			return errors.New("扩充同点数双地块的蛮族供应不足分配尚待核对")
		}
		for _, id := range targets {
			counts[id]++
			remaining--
		}
		event.Rolls = append(event.Rolls, catanAttackLandingRoll{dice, targets})
	}
	a.Barbarians = counts
	a.Sequence = event.ID
	a.Landing = event
	for _, r := range event.Rolls {
		if len(r.Tiles) == 0 {
			s.catanLog(s.Turn, "蛮族登陆掷出 %d+%d：对应沿海地块已被征服", r.Dice[0], r.Dice[1])
			continue
		}
		for _, id := range r.Tiles {
			s.catanLog(s.Turn, "蛮族登陆掷出 %d+%d：地块 #%d 增加1个蛮族（%d/3）", r.Dice[0], r.Dice[1], id+1, counts[id])
			if counts[id] == 3 {
				s.Log = append(s.Log, fmt.Sprintf("地块 #%d 被征服：停止生产并禁止在相邻交点或边新建", id+1))
			}
		}
	}
	if len(event.Rolls) == 0 {
		s.Log = append(s.Log, "蛮族供应已空，本次建设没有新增登陆")
	}
	s.catanScores()
	s.catanVictory()
	return nil
}

func (s *State) catanAttackView(v map[string]any) {
	g := s.Catan
	a := g.Attack
	if a == nil {
		return
	}
	public := v["attack"].(map[string]any)
	delete(public, "deck")
	public["devRemaining"] = len(a.Deck)
	public["supply"] = a.supply()
	conquered, buildings := []int{}, []int{}
	for _, t := range g.Tiles {
		if a.conquered(t.ID) {
			conquered = append(conquered, t.ID)
		}
	}
	for _, point := range g.Vertices {
		if point.Level > 0 && a.conqueredBuilding(g, point.ID) {
			buildings = append(buildings, point.ID)
		}
	}
	public["conquered"] = conquered
	public["conqueredBuildings"] = buildings
	left := make([]int, len(g.Players))
	for p := range left {
		left[p] = 6
	}
	for _, k := range a.Knights {
		left[k.Player]--
	}
	public["knightsLeft"] = left
}
