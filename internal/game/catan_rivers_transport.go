package game

import (
	"errors"
	"slices"
)

func (g *Catan) riversTransport() bool {
	return g.Rivers != nil && g.Rivers.Transport == CatanRiversTransportRules && g.Transport != nil && g.Transport.Map != nil && g.Transport.Map.Rivers == CatanRiversTransportRules
}
func (g *Catan) riverBridgeReward() int {
	if g.riversTransport() {
		return 2
	}
	return 3
}

// Commodity terrain retains its artwork and cargo behavior while producing
// the resource specified by the official Rivers/Traders combination sheet.
func (g *Catan) productionResource(tile CatanTile) int {
	if g.riversTransport() || g.caravansTransport() {
		for _, site := range g.Transport.Map.Sites {
			if site.Tile == tile.ID {
				switch site.Kind {
				case "quarry":
					return 1
				case "glassworks":
					return 0
				case "castle":
					return 2
				}
			}
		}
	}
	return tile.Resource
}

// Internal constructor: the public room gate stays closed until HTTP and UI
// acceptance have passed for the complete combination.
func NewCatanRiversTransport(n int, knights bool) (*State, error) {
	s, err := NewCatanTransport(n)
	if err != nil {
		return nil, err
	}
	board, m, r, err := newCatanRiversTransportBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize
	g.Transport = makeCatanTransportPieces(g, m)
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Transport: CatanRiversTransportRules, Map: r}
	for p := range g.Players {
		g.Transport.Gold[p] = 3
	}
	g.Transport.GoldBank = m.Gold - 3*n
	if n > 4 {
		g.Transport.DeckRecipe = CatanTransportExtendedDeck
	}
	if n == 2 {
		if err := g.prepareTwoNeutrals(); err != nil {
			return nil, err
		}
	}
	s.Log = []string{"河流＋运输：每人起始3金币，河岸起始村庄和城市及道路各领1金币，建桥领2金币；贫穷不扣分，13分获胜", "河流＋运输：无桥跨河需3移动点，己方桥梁需1点，对手桥梁需1点并付2金币；商品地块生产资源，2与12正常生产"}
	if n == 2 {
		s.Log = append(s.Log, "本站双人补充：中立桥梁的2金币通行费，银行和对手各得1金币")
	}
	if n > 4 {
		s.Log = append(s.Log, catanTransportDeckNotice, "本站五六人河流运输：37格地图、三条河流、七个商品地块，固定数字配置，使用配对回合")
	}
	if knights {
		logs := s.Log
		s.enableCitiesKnights()
		g.Rivers.Knights = CatanRiversKnightsRules
		g.Transport.Knights = CatanTransportKnightsRules
		g.Transport.DeckRecipe = ""
		fixed := riversTransportFixed(n > 4)
		for i := range g.Tiles {
			if _, ok := fixed[i]; !ok && g.Tiles[i].Resource == 0 {
				g.Tiles[i].Resource = 3
				break
			}
		}
		if n == 2 {
			g.Two.Knights = CatanTwoKnightsRules
		}
		s.Log = append(logs[1:], "本站河流运输城市骑士组合：15分获胜，使用进步牌；多保留一块麦田、少一块森林；共用金币，可付5金币免除城市劫掠")
	}
	s.catanScores()
	if err := s.validateCatanTwo(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTransport()
}

func (g *Catan) validateRiversTransportMap() error {
	if !g.riversTransport() {
		return errors.New("河流运输组合标记无效")
	}
	r := g.Rivers
	if r.Map == nil || len(r.Gold) != 0 || r.Bank != 0 || r.GoldIssued != 0 || r.Bought != 0 || (r.Knights != "") != g.transportKnights() {
		return errors.New("河流运输须共用单一金币账本")
	}
	if err := g.Transport.Map.validate(g); err != nil {
		return err
	}
	want, err := riversTransportChannels(g)
	if err != nil {
		return err
	}
	m := r.Map
	if m.DoubleNumberTile != want.DoubleNumberTile || m.NumberRecipe != "" || !slices.Equal(m.NumberSwaps, g.Transport.Map.NumberSwaps) || !slices.Equal(m.Swamps, want.Swamps) || !slices.Equal(m.Bridges, want.Bridges) || len(m.Channels) != len(want.Channels) {
		return errors.New("河流运输桥位或数字无效")
	}
	for i, c := range m.Channels {
		if c.Outlet != want.Channels[i].Outlet || !slices.Equal(c.Tiles, want.Channels[i].Tiles) {
			return errors.New("河流运输路径无效")
		}
	}
	return nil
}

func (g *Catan) transportSetupGold(p int) int {
	if !g.riversTransport() {
		return 5
	}
	amount := 3
	for _, v := range g.Vertices {
		if v.Owner == p && v.Level > 0 && g.riverVertex(v.ID) {
			amount++
		}
	}
	for _, e := range g.Edges {
		if e.Owner == p && g.riverEdge(e.ID) {
			amount++
		}
	}
	return amount
}
