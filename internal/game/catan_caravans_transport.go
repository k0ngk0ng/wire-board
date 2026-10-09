package game

import (
	"errors"
	"math"
	"slices"
)

const CatanCaravansTransportRules = "catan-caravans-transport-2025"

func (g *Catan) caravansTransport() bool {
	return g.Caravans != nil && g.Caravans.Transport == CatanCaravansTransportRules && g.Transport != nil && g.Transport.Map != nil && g.Transport.Map.Caravans == CatanCaravansTransportRules
}
func caravansTransportRecipe(n int) (holes []int, resources [5]int) {
	if n > 4 {
		return []int{5, 31}, [5]int{6, 5, 6, 6, 5}
	}
	return []int{9}, [5]int{3, 2, 3, 4, 3}
}
func caravansTransportNumbers(g *Catan) []int {
	holes, _ := caravansTransportRecipe(len(g.Players))
	_, order, faces := catanCaravanRecipe(false)
	if len(g.Players) > 4 {
		// Site recipe: retain all seven depots and replace the two deserts.
		// Read the enlarged board clockwise in stable outer-to-inner rings.
		order = make([]int, len(g.Tiles))
		cx, cy := 0.0, 0.0
		for i, t := range g.Tiles {
			order[i] = i
			cx += t.X
			cy += t.Y
		}
		cx /= float64(len(g.Tiles))
		cy /= float64(len(g.Tiles))
		slices.SortFunc(order, func(a, b int) int {
			x, y := g.Tiles[a], g.Tiles[b]
			ra := int(math.Round(math.Hypot(x.X-cx, x.Y-cy) / (math.Sqrt(3) * g.HexSize)))
			rb := int(math.Round(math.Hypot(y.X-cx, y.Y-cy) / (math.Sqrt(3) * g.HexSize)))
			if ra != rb {
				return rb - ra
			}
			aa, bb := math.Atan2(x.Y-cy, x.X-cx), math.Atan2(y.Y-cy, y.X-cx)
			if aa < bb {
				return -1
			}
			if aa > bb {
				return 1
			}
			return a - b
		})
		faces = append(slices.Clone(faces), faces[:17]...)
	}
	numbers := make([]int, len(g.Tiles))
	at := 0
	for _, id := range order {
		if slices.Contains(holes, id) {
			continue
		}
		numbers[id] = faces[at]
		at++
	}
	return numbers
}
func caravansTransportMap(g *Catan) (*catanCaravanMap, error) {
	holes, _ := caravansTransportRecipe(len(g.Players))
	starts, err := caravanStarts(g, holes)
	if err != nil {
		return nil, err
	}
	supply := 22
	if len(g.Players) > 4 {
		supply = 33
	}
	return &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: supply}, nil
}
func NewCatanCaravansTransport(n int, knights bool) (*State, error) {
	s, err := NewCatanTransport(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	holes, counts := caravansTransportRecipe(n)
	pool := []int{}
	for color, count := range counts {
		for range count {
			pool = append(pool, color)
		}
	}
	shuffle(pool)
	numbers := caravansTransportNumbers(g)
	at := 0
	for id := range g.Tiles {
		if slices.Contains(holes, id) {
			g.Tiles[id].Resource = catanWateringHole
		} else if g.Tiles[id].Resource != catanTransportTerrain {
			g.Tiles[id].Resource = pool[at]
			at++
		}
		g.Tiles[id].Number = numbers[id]
	}
	g.Transport.Map.Caravans = CatanCaravansTransportRules
	m, err := caravansTransportMap(g)
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Rules: CatanCaravansRules, Transport: CatanCaravansTransportRules, Map: m, Wagons: []catanCaravanWagon{}}
	s.Log = []string{"商队＋运输：商品地块正常生产，水源不生产；不使用强盗与最长道路，15分获胜", "本站回合顺序：先完成运输马车移动与装卸，再为本回合建设投票放商队马车；商队不走货物站内部路径或禁建边，蛮族不阻挡商队"}
	if n > 4 {
		s.Log = append(s.Log, catanTransportDeckNotice, "本站五六人商队运输：保留37格地图与七个货物站，两处沙漠改为水源、六个商队出口、33辆商队马车；固定环形数字配方，使用配对回合")
	}
	if n == 2 {
		s.Log = append(s.Log, "本站双人商队运输：保留双次生产、中立建设与运输过路费；中立建设不触发投票，商队每轮尽量放满、最多两辆")
	}
	if knights {
		s.enableCitiesKnights()
		g.Transport.Knights = CatanTransportKnightsRules
		g.Transport.DeckRecipe = ""
		g.Caravans.Knights = CatanCaravansKnightsRules
		for i := range g.Tiles {
			if g.Tiles[i].Resource == 0 {
				g.Tiles[i].Resource = 3
				break
			}
		}
		if n == 2 {
			g.Two.Knights = CatanTwoKnightsRules
		}
		s.Log = append(s.Log, "本站商队运输城市骑士：15分获胜；商队以木材或砖块出价，使用进步牌，海上蛮族与道路蛮族分别结算；多保留一块麦田、少一块森林")
	}
	s.catanScores()
	return s, s.validateCatanTransport()
}
func (g *Catan) validateCaravansTransportMap() error {
	if !g.caravansTransport() {
		return errors.New("商队运输组合标记无效")
	}
	c := g.Caravans
	if c.Map == nil || c.Rivers != "" || c.Attack != "" || len(c.ExtraNumbers) != 0 || c.Map.NumberRecipe != "" || (c.Knights != "") != g.transportKnights() || !slices.Equal(c.Map.NumberSwaps, g.Transport.Map.NumberSwaps) {
		return errors.New("商队运输组件或数字记录无效")
	}
	if err := g.Transport.Map.validate(g); err != nil {
		return err
	}
	want, err := caravansTransportMap(g)
	if err != nil {
		return err
	}
	if c.Map.Supply != want.Supply || !slices.Equal(c.Map.WateringHoles, want.WateringHoles) || !slices.Equal(c.Map.Starts, want.Starts) {
		return errors.New("商队运输水源、出口或供应无效")
	}
	return nil
}

// A completed delivery precedes bidding; there is no live travel response while
// the other players vote. The main controller synchronizes after the final vote.
func (s *State) catanAfterTransportTravel() error {
	if s.Catan.caravansTransport() {
		s.Catan.Transport.Travel = nil
		s.Catan.Transport.ArrivalResolved = false
		if s.catanBeginCaravanVote() {
			return nil
		}
	}
	s.catanNext()
	return s.catanTransportSyncTurn()
}

// Merchant trains remain on hex borders; depot spokes belong to cargo wagons.
func (g *Catan) caravanEdgeAllowed(edge int) bool {
	if g.Transport == nil {
		return true
	}
	m := g.Transport.Map
	if !m.canBuildRoad(g, edge) {
		return false
	}
	for _, site := range m.Sites {
		if slices.Contains(site.Paths, edge) {
			return false
		}
	}
	return true
}
