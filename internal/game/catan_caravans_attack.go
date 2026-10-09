package game

import (
	"errors"
	"slices"
)

const CatanCaravansAttackRules = "catan-caravans-attack-2025"

func (g *Catan) caravansAttack() bool {
	return g.Caravans != nil && g.Caravans.Attack == CatanCaravansAttackRules && g.Attack != nil && g.Attack.Map != nil && g.Attack.Map.Caravans == CatanCaravansAttackRules
}

// The coastal watering hole retains the printed orientation. A hut pointing
// outside the island has no outgoing edge and cannot start a merchant train.
func caravanAttackStarts(g *Catan, holes []int) ([]catanCaravanWagon, error) {
	starts := []catanCaravanWagon{}
	for _, id := range holes {
		if id < 0 || id >= len(g.Tiles) || len(g.Tiles[id].Vertices) != 6 {
			return nil, errors.New("商队蛮族水源无效")
		}
		for _, corner := range []int{1, 3, 5} {
			from := g.Tiles[id].Vertices[corner]
			found := -1
			for _, e := range g.Edges {
				if (e.A == from || e.B == from) && !slices.Contains(e.Tiles, id) {
					if found >= 0 {
						return nil, errors.New("水源出口不唯一")
					}
					found = e.ID
				}
			}
			if found >= 0 {
				starts = append(starts, catanCaravanWagon{Edge: found, From: from})
			}
		}
	}
	return starts, nil
}
func caravanAttackMap(g *Catan) (*catanCaravanMap, error) {
	holes := slices.Clone(catanAttackBoardRecipe(len(g.Players) > 4).deserts)
	starts, err := caravanAttackStarts(g, holes)
	if err != nil {
		return nil, err
	}
	supply := 22
	if len(g.Players) > 4 {
		supply = 33
	}
	return &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: supply}, nil
}

// NewCatanCaravansAttack builds the combined board for two to six players.
func NewCatanCaravansAttack(n int, knights bool) (*State, error) {
	s, err := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := caravanAttackMap(g)
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Rules: CatanCaravansRules, Attack: CatanCaravansAttackRules, Map: m, Wagons: []catanCaravanWagon{}}
	g.Attack.Map.Caravans = CatanCaravansAttackRules
	for _, id := range m.WateringHoles {
		g.Tiles[id].Resource = catanWateringHole
	}
	s.Log = []string{"商队＋蛮族进攻：沿海水源替换沙漠，小地图保留两个商队出口；蛮族不阻挡马车，12分获胜", "本站回合顺序：先完成骑士移动与战斗，再为本回合建设投票放马车；全部结算后切换玩家"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人商队蛮族配方：两处沿海水源替换沙漠，共五个有效出口、33辆马车，沿用蛮族地图数字和配对回合")
	}
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：沿用双人蛮族中立骑士与两次生产；商队每轮尽量放满、最多两辆，中立建筑不触发商队")
	}
	if knights {
		logs := s.Log
		s.enableCitiesKnights()
		g.Caravans.Knights = CatanCaravansKnightsRules
		g.Attack.City = &catanAttackCity{Rules: CatanAttackKnightsRules, Knights: []catanAttackCityKnight{}}
		g.Attack.Deck, g.Attack.Discard = []string{}, []string{}
		if n == 2 {
			g.Two.Knights = CatanTwoKnightsRules
			g.Attack.TwoRules = CatanTwoAttackKnightsRules
		}
		s.Log = append(logs[1:], "本站商队蛮族城市骑士：15分获胜，商队用木材或砖块出价；使用道路骑士和进步牌，不使用海上蛮族或强盗")
	}
	s.catanScores()
	if err := s.validateCaravans(); err != nil {
		return nil, err
	}
	return s, s.validateCatanAttack()
}

func (g *Catan) validateCaravansAttackMap() error {
	if !g.caravansAttack() {
		return errors.New("商队蛮族组合标记无效")
	}
	c := g.Caravans
	if c.Map == nil || c.Rivers != "" || len(c.ExtraNumbers) != 0 || c.Map.NumberRecipe != "" || len(c.Map.NumberSwaps) != 0 || (c.Knights != "") != g.attackKnights() {
		return errors.New("商队蛮族地图组件无效")
	}
	board := g
	if g.attackKnights() {
		if err := g.Attack.City.validateNumbers(g); err != nil {
			return err
		}
		copy := clone(*g)
		for id, number := range g.attackPrintedNumbers() {
			copy.Tiles[id].Number = number
		}
		board = &copy
	}
	if err := g.Attack.Map.validate(board); err != nil {
		return err
	}
	want, err := caravanAttackMap(g)
	if err != nil {
		return err
	}
	if c.Map.Supply != want.Supply || !slices.Equal(c.Map.WateringHoles, want.WateringHoles) || !slices.Equal(c.Map.Starts, want.Starts) {
		return errors.New("商队蛮族水源、出口或供应无效")
	}
	return nil
}

// Both end-of-turn controllers finish battles before exposing any bid choices.
func (s *State) catanAfterAttackBattles() {
	if s.Catan.attackPirates() && !s.Finished && s.Catan.pirateFortressReady(s.Turn) {
		s.catanAttackFortress(s.Turn, catanRandom(6)+1)
	}
	if s.Finished {
		return
	}
	if s.Catan.caravansAttack() && s.catanBeginCaravanVote() {
		return
	}
	s.catanNext()
	s.catanVictory()
}
func (g *Catan) addCaravanEscrow(total []int) {
	if g.Caravans == nil || g.Caravans.Pending == nil {
		return
	}
	for _, bid := range g.Caravans.Pending.Bids {
		for color, amount := range bid {
			if color < len(total) {
				total[color] += amount
			}
		}
	}
}
