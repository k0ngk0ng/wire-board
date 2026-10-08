package game

import (
	"errors"
	"math"
	"slices"
)

const CatanFishingTransportRules = "catan-fishing-transport-2025"

func (g *Catan) fishingTransport() bool {
	return g.Transport != nil && g.Fishing != nil && g.Fishing.Transport == CatanFishingTransportRules
}

// Transport's 37-hex extension has its own frame. This deterministic site
// recipe distributes grounds around the coast, avoiding ports and crossed-out
// commodity edges. Never reuse the ordinary 30-hex fishing frame coordinates.
func transportFishingGrounds(g *Catan) ([]catanFishingGround, error) {
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		blocked[p.Edge] = true
	}
	for _, site := range g.Transport.Map.Sites {
		for _, id := range site.Blocked {
			blocked[id] = true
		}
	}
	coast := []int{}
	cx, cy := 0.0, 0.0
	for _, tile := range g.Tiles {
		cx += tile.X
		cy += tile.Y
	}
	cx /= float64(len(g.Tiles))
	cy /= float64(len(g.Tiles))
	for _, e := range g.Edges {
		if len(e.Tiles) != 1 {
			continue
		}
		center := false
		for _, site := range g.Transport.Map.Sites {
			center = center || e.A == site.Center || e.B == site.Center
		}
		if !center {
			coast = append(coast, e.ID)
		}
	}
	angle := func(id int) float64 {
		e := g.Edges[id]
		a, b := g.Vertices[e.A], g.Vertices[e.B]
		return math.Atan2((a.Y+b.Y)/2-cy, (a.X+b.X)/2-cx)
	}
	slices.SortFunc(coast, func(a, b int) int {
		if angle(a) < angle(b) {
			return -1
		}
		if angle(a) > angle(b) {
			return 1
		}
		return a - b
	})
	candidates := []catanFishingGround{}
	for i, id := range coast {
		next := coast[(i+1)%len(coast)]
		if blocked[id] || blocked[next] {
			continue
		}
		a, b := g.Edges[id], g.Edges[next]
		joint, first, last := a.A, a.B, b.A
		if joint != b.A && joint != b.B {
			joint, first = a.B, a.A
		}
		if joint != b.A && joint != b.B {
			continue
		}
		if last == joint {
			last = b.B
		}
		candidates = append(candidates, catanFishingGround{Edges: [2]int{id, next}, Vertices: [3]int{first, joint, last}})
	}
	count := 6
	if len(g.Players) > 4 {
		count = 8
	}
	// Choose a full nonoverlapping set; prefer roughly equal angular spacing.
	var choose func([]catanFishingGround, map[int]bool) ([]catanFishingGround, bool)
	choose = func(out []catanFishingGround, used map[int]bool) ([]catanFishingGround, bool) {
		if len(out) == count {
			return out, true
		}
		target := -math.Pi + 2*math.Pi*(float64(len(out))+.5)/float64(count)
		order := slices.Clone(candidates)
		distance := func(c catanFishingGround) float64 {
			d := math.Abs(angle(c.Edges[0]) - target)
			return math.Min(d, 2*math.Pi-d)
		}
		slices.SortStableFunc(order, func(a, b catanFishingGround) int {
			if distance(a) < distance(b) {
				return -1
			}
			if distance(a) > distance(b) {
				return 1
			}
			return 0
		})
		for _, c := range order {
			if used[c.Edges[0]] || used[c.Edges[1]] {
				continue
			}
			used[c.Edges[0]], used[c.Edges[1]] = true, true
			if result, ok := choose(append(out, c), used); ok {
				return result, true
			}
			delete(used, c.Edges[0])
			delete(used, c.Edges[1])
		}
		return nil, false
	}
	out, ok := choose(nil, map[int]bool{})
	if !ok {
		return nil, errors.New("运输海岸没有足够的无重叠渔场")
	}
	return out, nil
}
func NewCatanFishingTransport(n int, knights bool) (*State, error) {
	var s *State
	var err error
	if knights {
		s, err = NewCatanTransportCitiesKnights(n)
	} else {
		s, err = NewCatanTransport(n)
	}
	if err != nil {
		return nil, err
	}
	g := s.Catan
	tokens, err := newCatanFishingTokens(n)
	if n == 2 {
		tokens, err = newTwoCatanFishTokens()
	}
	if err != nil {
		return nil, err
	}
	f := &CatanFishing{Transport: CatanFishingTransportRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	f.Map.Lakes = []catanFishingLake{}
	f.Map.Grounds, err = transportFishingGrounds(g)
	if err != nil {
		return nil, err
	}
	numbers := []int{4, 5, 6, 8, 9, 10}
	if n > 4 {
		numbers = append(numbers, 5, 9)
	}
	shuffle(numbers)
	for i := range f.Map.Grounds {
		f.Map.Grounds[i].Number = numbers[i]
	}
	if n == 2 {
		f.Two = CatanTwoFishingRules
		g.Two.Tokens, g.Two.Bank = []int{0, 0}, 0
		if knights {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	g.Fishing = f
	if len(s.Log) > 1 {
		s.Log[1] = "运输任务：不使用强盗与最长道路；胜利门槛按渔夫组合规则计算"
	}
	s.Log = append(s.Log, "渔夫＋运输：无湖泊，海岸渔场使用本站固定位置，避开港口和商品地块禁行边；2鱼替代1粮使马车增加2移动点，每回合二选一且最多一次，快速旅程不重置次数")
	if knights {
		s.Log = append(s.Log, "本站三模块适配：沿用运输骑士15分目标，旧靴需16分，7鱼领取进步牌")
	} else {
		s.Log = append(s.Log, "按官方渔夫运输组合页：12分获胜，旧靴需13分；不改变普通运输的13分目标")
	}
	if n == 2 {
		s.Log = append(s.Log, "本站双人渔夫运输：每人起始五枚鱼筹码，停用贸易筹码及骑士换筹码，公开分数落后者鱼行动少付1鱼；保留中立建设与两次生产")
	}
	s.catanScores()
	return s, s.validateCatanTransport()
}
func (f catanFishingMap) validateTransport(g *Catan) error {
	if !g.fishingTransport() || g.Explorer != nil || g.Attack != nil || g.Caravans != nil || g.Rivers != nil || g.Seafarers != nil || g.Fishing.Attack != "" || g.Fishing.Rivers != "" || g.Fishing.Caravans != "" || len(f.Lakes) != 0 || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 || f.SeaRecipe != "" || f.NumberRecipe != "" {
		return errors.New("渔夫运输组合或无湖配置无效")
	}
	if err := g.Transport.Map.validate(g); err != nil {
		return err
	}
	want, err := transportFishingGrounds(g)
	if err != nil {
		return err
	}
	if len(want) != len(f.Grounds) {
		return errors.New("运输渔场数量无效")
	}
	numbers := []int{}
	for i, ground := range f.Grounds {
		if ground.Edges != want[i].Edges || ground.Vertices != want[i].Vertices || ground.SeaTile != nil {
			return errors.New("运输渔场不符合海岸配方")
		}
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	expected := []int{4, 5, 6, 8, 9, 10}
	if len(g.Players) > 4 {
		expected = []int{4, 5, 5, 6, 8, 9, 9, 10}
	}
	if !slices.Equal(numbers, expected) {
		return errors.New("运输渔场数字无效")
	}
	return nil
}
func (s *State) catanTransportFish(player int, a Action) error {
	g, t := s.Catan, s.Catan.Transport
	if !g.fishingTransport() || a.Skill != "" || len(a.Take)+len(a.Give)+len(a.Cards) > 0 {
		return errors.New("当前不能用鱼增加马车步数")
	}
	if err := t.travelAllowed(g, player, t.Sequence); err != nil {
		return err
	}
	q := t.Travel
	if q.Ended || q.Pending != -1 || q.WheatUsed {
		return errors.New("本回合最多用鱼或粮食增加步数一次，且需先完成当前回应")
	}
	if err := g.Fishing.Tokens.spend(player, a.Tokens, g.fishActionCost(player, "catan_transport_fish")); err != nil {
		return err
	}
	q.WheatUsed = true
	q.FishUsed = true
	q.Points += 2
	paid := 0
	for _, id := range a.Tokens {
		paid += catanFishValue(id)
	}
	s.catanLog(player, "支付 %d 鱼，马车增加2移动点，本回合不能再用鱼或粮食增加步数", paid)
	return nil
}
