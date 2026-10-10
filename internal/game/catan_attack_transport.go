package game

import (
	"errors"
	"slices"
)

// The shared invasion is authoritative. Attack's per-hex counts and prisoners
// are checked projections for existing scoring/building/knight helpers.
// Transport alone owns the gold ledger and cargo inventory.
type catanAttackTransport struct {
	Rules     string                     `json:"rules"`
	Pieces    catanAttackTransportPieces `json:"pieces"`
	Attempted []bool                     `json:"attempted,omitempty"`
}

func (g *Catan) attackTransport() bool {
	return g != nil && g.AttackTransport != nil && g.AttackTransport.Rules == CatanAttackTransportRules && g.Attack != nil && g.Transport != nil && g.Attack.Map != nil && g.Transport.Map != nil && g.Attack.Map.Transport == CatanAttackTransportRules && g.Transport.Map.Attack == CatanAttackTransportRules
}
func (g *Catan) attackTransportBoard() *catanAttackTransportBoard {
	return &catanAttackTransportBoard{Rules: CatanAttackTransportRules, Attack: *g.Attack.Map, Transport: *g.Transport.Map}
}
func (g *Catan) syncAttackTransportCounts() {
	a, p := g.Attack, &g.AttackTransport.Pieces
	a.Barbarians = p.counts(g)
	a.Prisoners = make([]int, len(g.Players))
	a.NeutralPrisoners = 0
	for _, piece := range p.Barbarians {
		if piece.Tile < 0 {
			if piece.Captor >= 0 {
				a.Prisoners[piece.Captor]++
			} else if piece.Captor == -2 {
				a.NeutralPrisoners++
			}
		}
	}
	g.clearAttackConqueredMerchant()
}
func (g *Catan) validateAttackTransportPieces() error {
	if !g.attackTransport() {
		return errors.New("蛮族运输组合组件缺失")
	}
	if err := g.AttackTransport.Pieces.validate(g, g.attackTransportBoard()); err != nil {
		return err
	}
	a := g.Attack
	if len(a.Gold) != 0 || a.GoldBank != 0 || a.GoldIssued != 0 || a.Bought != 0 {
		return errors.New("蛮族运输必须共用运输金币账本")
	}
	if g.Transport.Barbarians != [3]int{-1, -1, -1} {
		return errors.New("蛮族运输不能保留独立运输蛮族")
	}
	counts := g.AttackTransport.Pieces.counts(g)
	prisoners := make([]int, len(g.Players))
	neutral := 0
	for _, piece := range g.AttackTransport.Pieces.Barbarians {
		if piece.Tile < 0 {
			if piece.Captor >= 0 {
				prisoners[piece.Captor]++
			} else if piece.Captor == -2 {
				neutral++
			}
		}
	}
	if !slices.Equal(a.Barbarians, counts) || !slices.Equal(a.Prisoners, prisoners) || a.NeutralPrisoners != neutral {
		return errors.New("蛮族运输数量与共享棋子不一致")
	}
	if q := g.Transport.Travel; q != nil {
		shared := catanAttackTransportTravel{Travel: *q, Attempted: g.AttackTransport.Attempted}
		if err := shared.validate(g, g.attackTransportBoard(), &g.AttackTransport.Pieces, g.Transport.Gold); err != nil {
			return err
		}
	} else if len(g.AttackTransport.Attempted) != 0 {
		return errors.New("没有马车移动但存在驱赶记录")
	}
	return nil
}

// NewCatanAttackTransport creates the shared invasion/cargo recipe.
func NewCatanAttackTransport(n int, knights bool) (*State, error) {
	if knights {
		return newCatanAttackTransportKnights(n)
	}
	return newCatanAttackTransportState(n)
}

// Shared ordinary foundation; the cities recipe replaces its knight controller.
func newCatanAttackTransportState(n int) (*State, error) {
	s, err := NewCatanTransport(n)
	if err != nil {
		return nil, err
	}
	board, b, err := newCatanAttackTransportBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize
	g.Transport = makeCatanTransportPieces(g, &b.Transport)
	g.Transport.Barbarians = [3]int{-1, -1, -1}
	g.Attack = &catanAttack{Rules: catanAttackRules, Map: &b.Attack, Knights: []catanAttackKnight{}, Discard: []string{}}
	for _, card := range []string{"capture", "knighthood", "swift_knight", "treason"} {
		for range catanAttackCardCounts()[card] {
			g.Attack.Deck = append(g.Attack.Deck, card)
		}
	}
	shuffle(g.Attack.Deck)
	pieces, err := newCatanAttackTransportPieces(g, b)
	if err != nil {
		return nil, err
	}
	g.AttackTransport = &catanAttackTransport{Rules: CatanAttackTransportRules, Pieces: *pieces}
	g.syncAttackTransportCounts()
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	if n == 2 {
		g.Attack.TwoRules = CatanTwoAttackRules
		if err = g.prepareTwoNeutrals(); err != nil {
			return nil, err
		}
	}
	s.Log = []string{"蛮族进攻＋运输：共用蛮族和金币，使用蛮族进攻发展牌；不使用强盗、最长道路和最大骑士军队，14分获胜", "本站回合顺序：先移动骑士并结算全部战斗，再移动运输马车和装卸；登陆自动关联第一条空边"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人蛮族运输：37格、七个货物站与两座骑士城堡；玻璃工坊为12／12／2，内陆货物城堡为6，采石场无数字；使用配对回合")
	}
	s.catanScores()
	if err = s.validateCatanAttack(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTransport()
}

func (t catanTransport) validateTravel(g *Catan) error {
	if !g.attackTransport() {
		return t.Travel.validate(g, t.Map, t.Barbarians, t.Gold)
	}
	q := catanAttackTransportTravel{Travel: *t.Travel, Attempted: g.AttackTransport.Attempted}
	return q.validate(g, g.attackTransportBoard(), &g.AttackTransport.Pieces, t.Gold)
}

// Temporary call-local adapter; no pointer alias or duplicate state is persisted.
func (t *catanTransport) sharedTravel(g *Catan) *catanAttackTransportTravel {
	return &catanAttackTransportTravel{Travel: *t.Travel, Attempted: slices.Clone(g.AttackTransport.Attempted)}
}
func (t *catanTransport) saveSharedTravel(g *Catan, q *catanAttackTransportTravel) {
	*t.Travel = q.Travel
	g.AttackTransport.Attempted = q.Attempted
}

func (g *Catan) attackBattleTiles() []int {
	if !g.attackTransport() {
		return append(slices.Clone(g.Attack.Map.Coast), g.Attack.Map.Reserves...)
	}
	// Preserve official coastal order, then resolve inland in board order.
	out := slices.Clone(g.Attack.Map.Coast)
	for _, t := range g.Tiles {
		if t.Resource != catanCastle && !slices.Contains(out, t.ID) {
			out = append(out, t.ID)
		}
	}
	return out
}
func (g *Catan) attackGoldReady(amount int) bool {
	if g.attackTransport() {
		_, err := catanGoldShortfall(g.Transport.GoldBank, g.Transport.GoldIssued, amount)
		return err == nil
	}
	_, err := catanGoldShortfall(g.Attack.GoldBank, g.Attack.GoldIssued, amount)
	return err == nil
}
func (g *Catan) attackReward(rewards []int) error {
	if len(rewards) != len(g.Players) {
		return errors.New("战斗奖励人数无效")
	}
	total := 0
	for _, n := range rewards {
		if n < 0 || n > catanGoldLedgerLimit {
			return errors.New("战斗奖励无效")
		}
		total += n
	}
	if g.attackTransport() {
		t := g.Transport
		if err := t.ensureGold(total); err != nil {
			return err
		}
		for p, n := range rewards {
			t.Gold[p] += n
			t.GoldBank -= n
		}
		return nil
	}
	a := g.Attack
	if err := a.ensureGold(total); err != nil {
		return err
	}
	for p, n := range rewards {
		a.Gold[p] += n
		a.GoldBank -= n
	}
	return nil
}
func (s *State) attackTransportLanding(roll func() [2]int, choose func(int) int) error {
	g := s.Catan
	a := g.Attack
	if g.setup() || s.Phase != "catan_turn" || s.Finished || roll == nil {
		return errors.New("当前不能蛮族登陆")
	}
	pieces := clone(g.AttackTransport.Pieces)
	event := &catanAttackLandingRecord{ID: a.Sequence + 1, Player: s.Turn, Rolls: []catanAttackLandingRoll{}}
	used := map[int]bool{}
	for attempts := 0; len(event.Rolls) < 3 && pieces.supply() > 0; attempts++ {
		if attempts >= 10000 {
			return errors.New("登陆骰未产生有效不同点数")
		}
		dice := roll()
		n := dice[0] + dice[1]
		if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
			return errors.New("登陆骰无效")
		}
		if n == 7 || used[n] {
			continue
		}
		used[n] = true
		targets, short, err := pieces.landNumber(g, g.attackTransportBoard(), n, choose)
		if err != nil {
			return err
		}
		event.Rolls = append(event.Rolls, catanAttackLandingRoll{Dice: dice, Tiles: targets, Shortage: short})
	}
	g.AttackTransport.Pieces = pieces
	g.syncAttackTransportCounts()
	a.Sequence = event.ID
	a.Landing = event
	for _, r := range event.Rolls {
		for _, tile := range r.Tiles {
			s.catanLog(s.Turn, "蛮族登陆：地块 #%d 增加1个（%d/3），同步阻挡道路", tile+1, a.Barbarians[tile])
		}
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
func (s *State) attackTransportProductionLanding(total int) error {
	g := s.Catan
	if g.attackKnights() {
		return nil
	} // City production already resolves one invasion per roll.
	if total != 2 && total != 12 {
		return nil
	}
	tiles, _, err := g.AttackTransport.Pieces.landNumber(g, g.attackTransportBoard(), total, catanRandom)
	if err != nil {
		return err
	}
	g.syncAttackTransportCounts()
	for _, tile := range tiles {
		s.catanLog(s.Turn, "掷出%d：地块 #%d 蛮族登陆（%d/3）", total, tile+1, g.Attack.Barbarians[tile])
	}
	s.catanScores()
	return nil
}
func (g *Catan) attackCapture(tile, player int) error {
	if g.attackTransport() {
		for id, p := range g.AttackTransport.Pieces.Barbarians {
			if p.Tile == tile {
				if err := g.AttackTransport.Pieces.capture(g, g.attackTransportBoard(), id, player); err != nil {
					return err
				}
				g.syncAttackTransportCounts()
				return nil
			}
		}
		return errors.New("该地块没有可俘获的蛮族")
	}
	g.Attack.Barbarians[tile]--
	g.Attack.Prisoners[player]++
	return nil
}

func (g *Catan) attackBattleKnightLimit(tile int) int {
	if !g.attackTransport() {
		return 6
	}
	n := 0
	for _, e := range g.Edges {
		if slices.Contains(e.Tiles, tile) {
			n++
		}
	}
	return n
}

func (g *Catan) attackCaptureTargets() []int {
	if !g.attackTransport() {
		return g.Attack.captureTargets()
	}
	return g.AttackTransport.Pieces.battleTiles(g)
}
func (g *Catan) attackTreasonPlans() []catanAttackTreasonPlan {
	if !g.attackTransport() {
		return g.Attack.treasonPlans()
	}
	// Existing two-distinct-source rule and supply priority are retained. Inland
	// destinations are added; only complete edge assignments are advertised.
	a := *g.Attack
	m := *a.Map
	m.Coast = g.attackBattleTiles()
	a.Map = &m
	plans := a.treasonPlans()
	out := []catanAttackTreasonPlan{}
	for _, plan := range plans {
		for i, to := range plan.Destinations {
			if len(plan.Sources) == 1 {
				if _, err := g.attackTreasonPieces(plan.Sources, []int{to}); err == nil {
					out = append(out, catanAttackTreasonPlan{Sources: plan.Sources, Destinations: []int{to}})
				}
				continue
			}
			if len(plan.Sources) != 2 {
				continue
			}
			for _, other := range plan.Destinations[i+1:] {
				destinations := []int{to, other}
				if _, err := g.attackTreasonPieces(plan.Sources, destinations); err == nil {
					out = append(out, catanAttackTreasonPlan{Sources: plan.Sources, Destinations: destinations})
				}
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	// If two cannot be assigned, search the one-move supplement explicitly.
	for _, from := range append(g.attackCaptureTargets(), -1) {
		if from == -1 && len(g.attackCaptureTargets()) > 0 {
			continue
		}
		for _, to := range g.attackBattleTiles() {
			if _, err := g.attackTreasonPieces([]int{from}, []int{to}); err == nil {
				out = append(out, catanAttackTreasonPlan{Sources: []int{from}, Destinations: []int{to}})
			}
		}
	}
	if len(out) == 0 {
		return []catanAttackTreasonPlan{{Sources: []int{}, Destinations: []int{}}}
	}
	return out
}
func (g *Catan) attackTreasonPieces(from, to []int) (*catanAttackTransportPieces, error) {
	if len(from) != len(to) || len(from) > 2 {
		return nil, errors.New("叛变移动数量无效")
	}
	p := catanAttackTransportPieces{Barbarians: slices.Clone(g.AttackTransport.Pieces.Barbarians)}
	ids := []int{}
	counts := p.counts(g)
	for i, tile := range from {
		if tile < -1 || tile >= len(g.Tiles) || tile >= 0 && slices.Contains(from[:i], tile) {
			return nil, errors.New("叛变来源无效")
		}
		id := -1
		for j, piece := range p.Barbarians {
			if piece.Tile == tile && piece.Captor == -1 {
				id = j
				break
			}
		}
		if id < 0 {
			return nil, errors.New("叛变来源没有蛮族")
		}
		ids = append(ids, id)
		p.Barbarians[id] = catanAttackTransportBarbarian{-1, -1, 0} // temporarily exclude this ID from source selection
	}
	for i, tile := range to {
		if tile < 0 || tile >= len(g.Tiles) || g.Tiles[tile].Resource == catanCastle || counts[tile] >= 3 || slices.Contains(from, tile) || slices.Contains(to[:i], tile) {
			return nil, errors.New("叛变目的地无效")
		}
	}
	var assign func(int) bool
	assign = func(at int) bool {
		if at == len(ids) {
			return true
		}
		for _, edge := range p.edges(g, to[at], -1) {
			p.Barbarians[ids[at]] = catanAttackTransportBarbarian{to[at], edge, -1}
			if assign(at + 1) {
				return true
			}
			p.Barbarians[ids[at]] = catanAttackTransportBarbarian{-1, -1, 0}
		}
		return false
	}
	if !assign(0) {
		return nil, errors.New("叛变目的地没有可关联的空边")
	}
	return &p, p.validate(g, g.attackTransportBoard())
}
func (t catanTransport) quoteTravel(g *Catan, q catanTransportTravel, edge int) (catanTransportStep, error) {
	if !g.attackTransport() {
		return q.quote(g, t.Map, t.Barbarians, t.Gold, edge)
	}
	shared := catanAttackTransportTravel{Travel: q, Attempted: g.AttackTransport.Attempted}
	return shared.quote(g, g.attackTransportBoard(), &g.AttackTransport.Pieces, t.Gold, edge)
}
