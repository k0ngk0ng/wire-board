package game

import (
	"errors"
	"slices"
)

type catanAttackKnight struct {
	Player int `json:"player"`
	Edge   int `json:"edge"`
}

// Scenario state uses its own development deck; ordinary development/robber
// actions cannot drive these pieces. Public configuration remains disabled.
type catanAttack struct {
	WonderLanding    *catanAttackWonderLanding `json:"wonderLanding,omitempty"`
	TribeRoute       *CatanRouteCompletion     `json:"tribeRoute,omitempty"`
	City             *catanAttackCity          `json:"city,omitempty"`
	TwoRules         string                    `json:"twoRules,omitempty"`
	TwoLanding       bool                      `json:"twoLanding,omitempty"`
	NeutralPrisoners int                       `json:"neutralPrisoners,omitempty"`
	GoldIssued       int                       `json:"goldIssued,omitempty"`
	EndPlan          *catanAttackEndPlan       `json:"endPlan,omitempty"`
	EndSequence      int                       `json:"endSequence,omitempty"`
	End              *catanAttackEndRecord     `json:"end,omitempty"`
	CardSequence     int                       `json:"cardSequence"`
	Pending          *catanAttackCardPending   `json:"pending,omitempty"`
	Bought           int                       `json:"bought"`
	Sequence         int                       `json:"sequence"`
	Landing          *catanAttackLandingRecord `json:"landing,omitempty"`
	Rules            string                    `json:"rules"`
	Map              *catanAttackMap           `json:"map"`
	Barbarians       []int                     `json:"barbarians"`
	Knights          []catanAttackKnight       `json:"knights"`
	Prisoners        []int                     `json:"prisoners"`
	Gold             []int                     `json:"gold"`
	GoldBank         int                       `json:"goldBank"`
	Deck             []string                  `json:"deck"`
	Discard          []string                  `json:"discard"`
}

func catanAttackCardCounts() map[string]int {
	return map[string]int{"capture": 4, "knighthood": 14, "swift_knight": 4, "treason": 4}
}

func newCatanAttackPieces(g *Catan, m *catanAttackMap) (*catanAttack, error) {
	if m == nil {
		return nil, errors.New("蛮族进攻地图缺失")
	}
	if err := m.validate(g); err != nil {
		return nil, err
	}
	n := len(g.Players)
	a := &catanAttack{Rules: catanAttackRules, Map: m, Barbarians: make([]int, len(g.Tiles)), Knights: []catanAttackKnight{}, Prisoners: make([]int, n), Gold: make([]int, n), GoldBank: m.Gold, Discard: []string{}}
	for _, id := range m.Reserves {
		a.Barbarians[id] = 12
	}
	for _, id := range m.Coast {
		if g.pirateIslands() == nil && len(m.Reserves) == 0 && (g.Tiles[id].Number == 2 || g.Tiles[id].Number == 12 || m.Rivers == CatanRiversAttackRules && n <= 4 && id == 11) {
			a.Barbarians[id] = 1
		}
	}
	for _, card := range []string{"capture", "knighthood", "swift_knight", "treason"} {
		for range catanAttackCardCounts()[card] {
			a.Deck = append(a.Deck, card)
		}
	}
	shuffle(a.Deck)
	return a, a.validate(g)
}

func (a catanAttack) validate(g *Catan) error {
	if a.Rules != catanAttackRules || a.Map == nil || a.City != nil {
		return errors.New("蛮族进攻规则版本或地图缺失")
	}
	if err := a.Map.validate(g); err != nil {
		return err
	}
	n := len(g.Players)
	if g.attackTransport() {
		if err := g.validateAttackTransportPieces(); err != nil {
			return err
		}
	} else {
		if len(a.Barbarians) != len(g.Tiles) || len(a.Gold) != n || len(a.Prisoners) != n || a.GoldIssued < 0 || a.GoldIssued > catanGoldLedgerLimit || a.GoldBank < 0 || a.GoldBank > a.Map.Gold+a.GoldIssued || g.setup() && a.GoldIssued != 0 {
			return errors.New("蛮族进攻组件状态无效")
		}
		total, gold := a.NeutralPrisoners, int64(a.GoldBank)
		for id, count := range a.Barbarians {
			if count < 0 || slices.Contains(a.Map.Reserves, id) && count > 12 || !slices.Contains(a.Map.Reserves, id) && (count > 3 || count > 0 && !slices.Contains(a.Map.Coast, id)) {
				return errors.New("蛮族只能在可生产的沿海地块，每格至多3个")
			}
			total += count
		}
		for p := range n {
			if a.Gold[p] < 0 || a.Gold[p] > a.Map.Gold+a.GoldIssued || a.Prisoners[p] < 0 {
				return errors.New("金币或俘虏数量无效")
			}
			gold += int64(a.Gold[p])
			total += a.Prisoners[p]
		}
		if total > a.Map.Barbarians || len(a.Map.Reserves) > 0 && total != a.Map.Barbarians || gold != int64(a.Map.Gold)+int64(a.GoldIssued) {
			return errors.New("蛮族或金币库存不守恒")
		}
	}
	used := map[int]bool{}
	knights := map[int]int{}
	for _, k := range a.Knights {
		if (k.Player < 0 || k.Player >= n) && !(g.twoAttack() && k.Player == catanAttackNeutral) || k.Edge < 0 || k.Edge >= len(g.Edges) || used[k.Edge] || !g.attackSeaKnightEdge(k.Edge) {
			return errors.New("骑士玩家或位置无效")
		}
		used[k.Edge] = true
		knights[k.Player]++
		if knights[k.Player] > 6 {
			return errors.New("每位玩家至多6名骑士")
		}
	}
	counts, err := g.attackTribeReservedCards()
	if err != nil {
		return err
	}
	if q := g.HelperPending; q != nil && q.Kind == "attack_development" {
		for _, card := range q.Cards {
			if card < 0 || card >= 4 {
				return errors.New("助手蛮族牌无效")
			}
			counts[catanTradersAttackCards()[card]]++
		}
	}
	if g.tradersHelpers() && g.TradersHelpers.AttackCard != "" {
		counts[g.TradersHelpers.AttackCard]++
	}
	if a.Pending != nil {
		counts[a.Pending.Card]++
	}
	for _, cards := range [][]string{a.Deck, a.Discard} {
		for _, card := range cards {
			counts[card]++
		}
	}
	want := catanAttackCardCounts()
	if len(counts) != len(want) {
		return errors.New("蛮族进攻发展卡种类无效")
	}
	for card, count := range want {
		if counts[card] != count {
			return errors.New("蛮族进攻发展卡库存不符")
		}
	}
	return nil
}

func (a catanAttack) supply() int {
	return a.Map.Barbarians - sum(a.Barbarians) - sum(a.Prisoners) - a.NeutralPrisoners
}
func (a catanAttack) conquered(tile int) bool {
	return tile >= 0 && tile < len(a.Barbarians) && a.Barbarians[tile] >= 3
}
func (a catanAttack) conqueredBuilding(g *Catan, vertex int) bool {
	if vertex < 0 || vertex >= len(g.Vertices) {
		return false
	}
	touches := false
	for _, tile := range g.Tiles {
		if g.Seafarers != nil && (tile.Resource == CatanSea || tile.Resource == CatanFog) {
			continue
		}
		if !slices.Contains(tile.Vertices, vertex) {
			continue
		}
		touches = true
		if !a.conquered(tile.ID) {
			return false
		}
	}
	return touches
}
func (a catanAttack) buildBlocked(g *Catan, edge, vertex int) bool {
	// Existing pieces remain. Only NEW construction on any affected side or
	// corner is prohibited; conquest of a building requires ALL touching hexes.
	for _, tile := range g.Tiles {
		if !a.conquered(tile.ID) {
			continue
		}
		if vertex >= 0 && slices.Contains(tile.Vertices, vertex) {
			return true
		}
		if edge >= 0 && edge < len(g.Edges) && slices.Contains(g.Edges[edge].Tiles, tile.ID) {
			return true
		}
	}
	return false
}
func (a catanAttack) castleEdge(g *Catan, edge int) bool {
	if edge < 0 || edge >= len(g.Edges) {
		return false
	}
	for _, tile := range g.Edges[edge].Tiles {
		if slices.Contains(a.Map.Castles, tile) {
			return true
		}
	}
	return false
}

// Knights move on the edge graph, ignoring road owners, buildings and other
// knights while passing. Only their final edge must be empty and non-castle.
// A normal move allows 3 steps; paid wheat allows 5. Zero-step stays are dealt
// with by the end-phase controller, which forces castle knights out.
func (a catanAttack) knightDestinations(g *Catan, index, steps int) map[int]int {
	result := map[int]int{}
	if index < 0 || index >= len(a.Knights) || steps != 3 && steps != 5 {
		return result
	}
	start := a.Knights[index].Edge
	distances := map[int]int{start: 0}
	queue := []int{start}
	occupied := map[int]bool{}
	for _, k := range a.Knights {
		occupied[k.Edge] = true
	}
	for len(queue) > 0 {
		edge := queue[0]
		queue = queue[1:]
		d := distances[edge]
		if d == steps {
			continue
		}
		e := g.Edges[edge]
		for _, next := range g.Edges {
			if next.A != e.A && next.A != e.B && next.B != e.A && next.B != e.B {
				continue
			}
			if !g.attackSeaKnightEdge(next.ID) {
				continue
			}
			if _, seen := distances[next.ID]; seen {
				continue
			}
			distances[next.ID] = d + 1
			queue = append(queue, next.ID)
			if !occupied[next.ID] && !a.castleEdge(g, next.ID) {
				result[next.ID] = d + 1
			}
		}
	}
	return result
}
