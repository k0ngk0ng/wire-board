package game

import (
	"errors"
	"slices"
)

// One identity owns both its hex and its blocking edge. Edge=-1 means the
// piece is at the hex center; Tile=-1 means it is in the supply or captured.
type catanAttackTransportBarbarian struct {
	Tile   int `json:"tile"`
	Edge   int `json:"edge"`
	Captor int `json:"captor"` // -1 supply/board, player index or -2 neutral.
}
type catanAttackTransportPieces struct {
	Barbarians []catanAttackTransportBarbarian `json:"barbarians"`
}

func newCatanAttackTransportPieces(g *Catan, b *catanAttackTransportBoard) (*catanAttackTransportPieces, error) {
	if err := b.validate(g); err != nil {
		return nil, err
	}
	p := &catanAttackTransportPieces{Barbarians: make([]catanAttackTransportBarbarian, b.Attack.Barbarians)}
	for i := range p.Barbarians {
		p.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
	}
	at := 0
	for _, site := range b.Transport.Sites {
		if b.productiveCommodity(site.Tile) && slices.Contains(b.Attack.Coast, site.Tile) && (g.Tiles[site.Tile].Number == 2 || g.Tiles[site.Tile].Number == 12) {
			p.Barbarians[at] = catanAttackTransportBarbarian{site.Tile, -1, -1}
			at++
		}
	}
	return p, p.validate(g, b)
}
func (p catanAttackTransportPieces) counts(g *Catan) []int {
	counts := make([]int, len(g.Tiles))
	for _, piece := range p.Barbarians {
		if piece.Tile >= 0 && piece.Tile < len(counts) {
			counts[piece.Tile]++
		}
	}
	return counts
}
func (p catanAttackTransportPieces) blocking(edge int) int {
	if edge < 0 {
		return -1
	}
	for id, piece := range p.Barbarians {
		if piece.Tile >= 0 && piece.Edge == edge {
			return id
		}
	}
	return -1
}
func (p catanAttackTransportPieces) edges(g *Catan, tile, except int) []int {
	out := []int{}
	if tile < 0 || tile >= len(g.Tiles) {
		return out
	}
	for _, e := range g.Edges {
		if !slices.Contains(e.Tiles, tile) {
			continue
		}
		owner := p.blocking(e.ID)
		if owner < 0 || owner == except {
			out = append(out, e.ID)
		}
	}
	return out
}
func (p catanAttackTransportPieces) validate(g *Catan, b *catanAttackTransportBoard) error {
	issued := 0
	if g != nil && g.attackTransportKnights() {
		issued = g.Attack.City.Issued
	}
	if g == nil || b == nil || issued < 0 || issued > catanGoldLedgerLimit || len(p.Barbarians) != b.Attack.Barbarians+issued {
		return errors.New("蛮族运输棋子数量无效")
	}
	edges := map[int]bool{}
	for _, piece := range p.Barbarians {
		if piece.Tile < -1 || piece.Tile >= len(g.Tiles) || piece.Captor < -2 || piece.Captor >= len(g.Players) || piece.Captor == -2 && (len(g.Players) != 2 || g.attackKnights()) {
			return errors.New("蛮族运输棋子归属无效")
		}
		if piece.Tile < 0 {
			if piece.Edge != -1 {
				return errors.New("供应或俘虏不能占边")
			}
			continue
		}
		if piece.Captor != -1 || g.Tiles[piece.Tile].Resource == catanCastle || piece.Edge < -1 {
			return errors.New("蛮族运输在场棋子无效")
		}
		if piece.Edge >= 0 {
			if piece.Edge >= len(g.Edges) || !slices.Contains(g.Edges[piece.Edge].Tiles, piece.Tile) || edges[piece.Edge] {
				return errors.New("蛮族运输地块与阻挡边不一致")
			}
			edges[piece.Edge] = true
		}
	}
	for _, count := range p.counts(g) {
		if count > 3 {
			return errors.New("同一地块蛮族超过三个")
		}
	}
	return nil
}

// Automatic landing uses the first free edge in stable board order. If all
// incident borders/spokes are occupied, the official rule permits the center.
func (p *catanAttackTransportPieces) land(g *Catan, b *catanAttackTransportBoard, tile int) (int, error) {
	if err := p.validate(g, b); err != nil {
		return -1, err
	}
	if !slices.Contains(b.Attack.Coast, tile) || p.counts(g)[tile] >= 3 {
		return -1, errors.New("不能登陆该地块")
	}
	id := -1
	for i, piece := range p.Barbarians {
		if piece.Tile < 0 && piece.Captor == -1 {
			id = i
			break
		}
	}
	if id < 0 {
		return -1, errors.New("蛮族供应已用完")
	}
	edge := -1
	if edges := p.edges(g, tile, -1); len(edges) > 0 {
		edge = edges[0]
	}
	p.Barbarians[id] = catanAttackTransportBarbarian{tile, edge, -1}
	return id, nil
}

// Treason and wagon drive-off may move inland. They must select both a hex
// and an unoccupied incident edge, avoiding ambiguity on shared borders.
func (p *catanAttackTransportPieces) relocate(g *Catan, b *catanAttackTransportBoard, id, tile, edge int) error {
	if err := p.validate(g, b); err != nil {
		return err
	}
	if id < 0 || id >= len(p.Barbarians) || p.Barbarians[id].Tile < 0 || tile < 0 || tile >= len(g.Tiles) || g.Tiles[tile].Resource == catanCastle || p.counts(g)[tile] >= 3 || tile == p.Barbarians[id].Tile && edge == p.Barbarians[id].Edge || !slices.Contains(p.edges(g, tile, id), edge) {
		return errors.New("请选择未征服地块及另一空边或内部路径")
	}
	p.Barbarians[id] = catanAttackTransportBarbarian{tile, edge, -1}
	return nil
}
func (p *catanAttackTransportPieces) capture(g *Catan, b *catanAttackTransportBoard, id, player int) error {
	if err := p.validate(g, b); err != nil {
		return err
	}
	if id < 0 || id >= len(p.Barbarians) || p.Barbarians[id].Tile < 0 || player < 0 && !(player == -2 && len(g.Players) == 2) || player >= len(g.Players) {
		return errors.New("俘获蛮族目标或玩家无效")
	}
	p.Barbarians[id] = catanAttackTransportBarbarian{-1, -1, player}
	return nil
}

// Battles may occur inland too (explicit in the official older combination
// rules). Stable map order also makes automatic resolution reproducible.
func (p catanAttackTransportPieces) battleTiles(g *Catan) []int {
	out := []int{}
	for tile, count := range p.counts(g) {
		if count > 0 {
			out = append(out, tile)
		}
	}
	return out
}

// Apply an already resolved prisoner distribution atomically. Rewards and
// knight casualties remain the battle controller's responsibility. Capturing
// IDs instead of clearing a count also frees each corresponding wagon route.
func (p *catanAttackTransportPieces) captureBattle(g *Catan, b *catanAttackTransportBoard, tile int, prisoners []int) error {
	if err := p.validate(g, b); err != nil {
		return err
	}
	seats := len(g.Players)
	if seats == 2 && !g.attackKnights() {
		seats++ // Combined neutral faction, as in ordinary two-player Attack.
	}
	if tile < 0 || tile >= len(g.Tiles) || len(prisoners) != seats {
		return errors.New("战斗地块或俘虏分配玩家无效")
	}
	ids := []int{}
	for id, piece := range p.Barbarians {
		if piece.Tile == tile {
			ids = append(ids, id)
		}
	}
	total := 0
	for _, count := range prisoners {
		if count < 0 || count > len(ids) {
			return errors.New("战斗俘虏数量无效")
		}
		total += count
	}
	if len(ids) == 0 || total != len(ids) {
		return errors.New("战斗俘虏分配与地块蛮族不符")
	}
	at := 0
	for player, count := range prisoners {
		captor := player
		if len(g.Players) == 2 && player == 2 {
			captor = -2
		}
		for range count {
			p.Barbarians[ids[at]] = catanAttackTransportBarbarian{-1, -1, captor}
			at++
		}
	}
	return nil
}

func (p catanAttackTransportPieces) supply() int {
	count := 0
	for _, piece := range p.Barbarians {
		if piece.Tile == -1 && piece.Captor == -1 {
			count++
		}
	}
	return count
}

// A production 2/12 or one invasion roll may match several extended-board
// coastal hexes. Sample without replacement when supply is short; this
// generalizes the already approved last-piece random allocation rule.
func (p *catanAttackTransportPieces) landNumber(g *Catan, b *catanAttackTransportBoard, number int, choose func(int) int) ([]int, bool, error) {
	if err := p.validate(g, b); err != nil {
		return nil, false, err
	}
	if number < 2 || number > 12 {
		return nil, false, errors.New("蛮族登陆点数无效")
	}
	if number == 7 {
		return []int{}, false, nil
	}
	counts := p.counts(g)
	targets := []int{}
	for _, tile := range b.Attack.Coast {
		if g.Tiles[tile].Number == number && counts[tile] < 3 {
			targets = append(targets, tile)
		}
	}
	remaining := p.supply()
	shortage := len(targets) > remaining
	if shortage {
		pool := slices.Clone(targets)
		targets = []int{}
		for len(targets) < remaining {
			if choose == nil {
				return nil, false, errors.New("蛮族供应不足，需要服务器随机分配")
			}
			at := choose(len(pool))
			if at < 0 || at >= len(pool) {
				return nil, false, errors.New("蛮族登陆随机目标无效")
			}
			targets = append(targets, pool[at])
			pool = slices.Delete(pool, at, at+1)
		}
	}
	next := clone(*p)
	for _, tile := range targets {
		if _, err := next.land(g, b, tile); err != nil {
			return nil, false, err
		}
	}
	*p = next
	return targets, shortage, nil
}
