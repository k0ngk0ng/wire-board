package game

import (
	"errors"
	"slices"
)

type catanTransportToken struct {
	ID     int    `json:"id"`
	Origin string `json:"origin"`
	Cargo  string `json:"cargo"`
}
type catanTransportWagon struct {
	Position  int   `json:"position"` // -1 until placed with the starting city.
	Level     int   `json:"level"`
	Cargo     int   `json:"cargo"` // 0 means empty; physical tokens have stable positive IDs.
	Delivered []int `json:"delivered"`
}

type catanTransportArrivalResult struct {
	Sequence  uint64 `json:"sequence"`
	Player    int    `json:"player"`
	Site      int    `json:"site"`
	Delivered int    `json:"delivered"`
	Loaded    int    `json:"loaded"`
	Gold      int    `json:"gold"`
}

// Persisted scenario state, including the extended online deck version.
type catanTransport struct {
	Knights           string                       `json:"knights,omitempty"`
	DeckRecipe        string                       `json:"deckRecipe,omitempty"`
	GameTurn          uint64                       `json:"gameTurn"`
	Swift             bool                         `json:"swift"`
	Moves             int                          `json:"moves"`
	BarbarianSequence uint64                       `json:"barbarianSequence"`
	BarbarianPending  bool                         `json:"barbarianPending"`
	Map               *catanTransportMap           `json:"map"`
	Wagons            []catanTransportWagon        `json:"wagons"`
	Stacks            [3][]int                     `json:"stacks"` // quarry, glassworks, castle; top at index 0.
	Gold              []int                        `json:"gold"`
	GoldBank          int                          `json:"goldBank"`
	GoldIssued        int                          `json:"goldIssued,omitempty"`
	Barbarians        [3]int                       `json:"barbarians"`
	Active            int                          `json:"active"`
	TurnSerial        uint64                       `json:"turnSerial"`
	Bought            int                          `json:"bought"`
	Sequence          uint64                       `json:"sequence"`
	Travel            *catanTransportTravel        `json:"travel,omitempty"`
	ArrivalResolved   bool                         `json:"arrivalResolved"`
	LastArrival       *catanTransportArrivalResult `json:"lastArrival,omitempty"`
}

func catanTransportOrigins() [3]string { return [3]string{"quarry", "glassworks", "castle"} }

// Official component inventories: base 2025 p3 explicitly says 6 of each
// back, extension p3 adds 3 of each back. Never replace drawn tokens or
// reshuffle delivered tokens when a stack empties. Expanded IDs 37–54 leave
// all base physical IDs unchanged.
func catanTransportTokens(extended bool) []catanTransportToken {
	out := []catanTransportToken{}
	batches := []int{6}
	if extended {
		batches = append(batches, 3)
	}
	goods := [3][2]string{{"marble", "sand"}, {"glass", "tools"}, {"sand", "tools"}}
	for _, count := range batches {
		for origin, name := range catanTransportOrigins() {
			for _, cargo := range goods[origin] {
				for range count {
					out = append(out, catanTransportToken{len(out) + 1, name, cargo})
				}
			}
		}
	}
	return out
}
func catanTransportOriginIndex(origin string) int {
	origins := catanTransportOrigins()
	return slices.Index(origins[:], origin)
}
func (t catanTransport) token(id int) (catanTransportToken, bool) {
	all := catanTransportTokens(len(t.Wagons) > 4)
	if id < 1 || id > len(all) {
		return catanTransportToken{}, false
	}
	return all[id-1], true
}
func newCatanTransportPieces(g *Catan, m *catanTransportMap) (*catanTransport, error) {
	if !catanTransportPlayersValid(g) || m == nil {
		return nil, errors.New("运输组件人数或双人控制器无效")
	}
	if err := m.validate(g); err != nil {
		return nil, err
	}
	t := &catanTransport{Map: m, Active: -1, Gold: make([]int, len(g.Players)), GoldBank: m.Gold - 5*len(g.Players), Barbarians: m.Barbarians}
	for p := range g.Players {
		t.Gold[p] = 5
		t.Wagons = append(t.Wagons, catanTransportWagon{Position: -1, Delivered: []int{}})
	}
	for _, token := range catanTransportTokens(len(g.Players) > 4) {
		i := catanTransportOriginIndex(token.Origin)
		t.Stacks[i] = append(t.Stacks[i], token.ID)
	}
	for i := range t.Stacks {
		shuffle(t.Stacks[i])
	}
	return t, t.validate(g)
}

func (t catanTransport) validate(g *Catan) error {
	if !catanTransportPlayersValid(g) || t.Map == nil || len(t.Wagons) != len(g.Players) || len(t.Gold) != len(g.Players) {
		return errors.New("运输组件或人数无效")
	}
	if g.Attack != nil || g.Caravans != nil || g.Rivers != nil || g.Fishing != nil && !g.fishingTransport() || g.Seafarers != nil || g.CitiesKnights != nil && !g.transportKnights() || g.Options.Helpers || g.Options.AllHelpers {
		return errors.New("运输与其他扩展的组合尚未接入")
	}
	if err := t.Map.validate(g); err != nil {
		return err
	}
	if t.GoldIssued < 0 || t.GoldIssued > catanGoldLedgerLimit || t.TurnSerial == 0 && t.GoldIssued != 0 {
		return errors.New("运输金币记账无效")
	}
	supply := t.Map.Gold + t.GoldIssued
	total := int64(t.GoldBank)
	for _, amount := range t.Gold {
		total += int64(amount)
	}
	if t.GoldBank < 0 || t.GoldBank > supply || total != int64(supply) || t.Bought < 0 || t.Bought > 2 {
		return errors.New("运输金币银行、总库存或购买次数无效")
	}
	all := catanTransportTokens(len(g.Players) > 4)
	seen := make([]bool, len(all))
	claim := func(id int, origin string) error {
		if id < 1 || id > len(all) || seen[id-1] || origin != "" && all[id-1].Origin != origin {
			return errors.New("运输货物重复、来源错误或编号无效")
		}
		seen[id-1] = true
		return nil
	}
	for i, origin := range catanTransportOrigins() {
		for _, id := range t.Stacks[i] {
			if err := claim(id, origin); err != nil {
				return err
			}
		}
	}
	for p, wagon := range t.Wagons {
		if wagon.Position < -1 || wagon.Position >= len(g.Vertices) || wagon.Level < 0 || wagon.Level > 4 || t.Gold[p] < 0 || t.Gold[p] > supply || wagon.Position == -1 && (wagon.Level != 0 || wagon.Cargo != 0 || len(wagon.Delivered) != 0) {
			return errors.New("运输马车位置、等级、载货或金币无效")
		}
		if t.TurnSerial == 0 && (wagon.Level != 0 || wagon.Cargo != 0 || len(wagon.Delivered) != 0 || t.Gold[p] != 5) {
			return errors.New("运输开局等级、载货或金币无效")
		}
		if t.TurnSerial == 0 && wagon.Position >= 0 && (g.Vertices[wagon.Position].Owner != p || g.Vertices[wagon.Position].Level != 2) {
			return errors.New("起始马车必须在自己的城市")
		}
		if wagon.Cargo != 0 {
			if err := claim(wagon.Cargo, ""); err != nil {
				return err
			}
		}
		for _, id := range wagon.Delivered {
			if err := claim(id, ""); err != nil {
				return err
			}
		}
	}
	if slices.Contains(seen, false) {
		return errors.New("运输货物丢失")
	}
	for i, edge := range t.Barbarians {
		if edge < 0 || edge >= len(g.Edges) || slices.Contains(t.Barbarians[:i], edge) {
			return errors.New("运输蛮族位置无效")
		}
	}
	if t.Active < -1 || t.Active >= len(g.Players) || (t.Active == -1) != (t.TurnSerial == 0) || t.TurnSerial == 0 && (t.Bought != 0 || t.Sequence != 0 || t.Travel != nil) {
		return errors.New("运输行动玩家或回合序号无效")
	}
	if t.TurnSerial > 0 {
		for _, w := range t.Wagons {
			if w.Position == -1 {
				return errors.New("起始马车尚未全部放置")
			}
		}
	}
	if t.Travel != nil {
		q := t.Travel
		if q.Player != t.Active || t.Sequence == 0 || t.Wagons[q.Player].Position != q.Position || t.Wagons[q.Player].Level != q.Level {
			return errors.New("马车移动记录与所属玩家不一致")
		}
		if err := q.validate(g, t.Map, t.Barbarians, t.Gold); err != nil {
			return err
		}
		currentRecord := t.LastArrival != nil && t.LastArrival.Sequence == t.Sequence
		if t.ArrivalResolved != currentRecord || t.ArrivalResolved && (q.Arrived < 0 || t.LastArrival.Player != q.Player || t.LastArrival.Site != q.Arrived || t.LastArrival.Loaded != 0 && t.LastArrival.Loaded != t.Wagons[q.Player].Cargo) {
			return errors.New("到达结算标志与本次交货记录不一致")
		}
	} else if t.ArrivalResolved {
		return errors.New("缺少货物到达移动记录")
	}
	if last := t.LastArrival; last != nil {
		if last.Sequence == 0 || last.Sequence > t.Sequence || last.Player < 0 || last.Player >= len(g.Players) || last.Site < 0 || last.Site >= len(t.Map.Sites) || last.Gold < 0 || last.Gold > 5 || (last.Delivered == 0) != (last.Gold == 0) {
			return errors.New("运输交付记录无效")
		}
		if last.Delivered != 0 {
			token, ok := t.token(last.Delivered)
			if !ok || !t.Map.accepts(last.Site, token.Cargo) || !slices.Contains(t.Wagons[last.Player].Delivered, last.Delivered) {
				return errors.New("运输交付记录与已交货物不符")
			}
		}
		if last.Loaded != 0 {
			token, ok := t.token(last.Loaded)
			if !ok || token.Origin != t.Map.Sites[last.Site].Kind {
				return errors.New("运输装货记录与来源不符")
			}
		}
	}
	stock := 19
	if len(g.Players) > 4 {
		stock = 24
	}
	cards := 5
	if g.transportKnights() {
		cards = 8
	}
	if len(g.Bank) != cards || !g.cardBundle(g.Bank) {
		return errors.New("运输资源银行无效")
	}
	totals := slices.Clone(g.Bank)
	for _, p := range g.Players {
		if len(p.Resources) != cards || !g.cardBundle(p.Resources) {
			return errors.New("运输玩家资源无效")
		}
		for c, count := range p.Resources {
			totals[c] += count
		}
	}
	for c, count := range totals {
		want := stock
		if c >= 5 {
			want = 12
			if len(g.Players) > 4 {
				want = 18
			}
		}
		if count != want {
			return errors.New("运输资源库存不守恒")
		}
	}
	return nil
}

func (t catanTransport) extraPoints(player int) int {
	if player < 0 || player >= len(t.Wagons) {
		return 0
	}
	w := t.Wagons[player]
	points := len(w.Delivered)
	if w.Level == 4 {
		points++
	}
	return points
}

// Safe projection, usable by every seat and observers. Commodity being carried
// is face up; completed tokens and supply order/IDs are never serialized here.
type catanTransportWagonView struct {
	Position  int                  `json:"position"`
	Level     int                  `json:"level"`
	Cargo     *catanTransportToken `json:"cargo,omitempty"`
	Delivered int                  `json:"delivered"`
	Points    int                  `json:"points"`
	Gold      int                  `json:"gold"`
}
type catanTransportPublicView struct {
	Wagons          []catanTransportWagonView `json:"wagons"`
	Supply          [3]int                    `json:"supply"`
	GoldBank        int                       `json:"goldBank"`
	GoldIssued      int                       `json:"goldIssued,omitempty"`
	GoldRule        string                    `json:"goldRule"`
	Barbarians      [3]int                    `json:"barbarians"`
	Active          int                       `json:"active"`
	Sequence        uint64                    `json:"sequence"`
	Travel          *catanTransportTravel     `json:"travel,omitempty"`
	ArrivalResolved bool                      `json:"arrivalResolved"`
}

func (t catanTransport) publicView() catanTransportPublicView {
	view := catanTransportPublicView{GoldBank: t.GoldBank, GoldIssued: t.GoldIssued, GoldRule: "ledger", Barbarians: t.Barbarians, Active: t.Active, Sequence: t.Sequence, ArrivalResolved: t.ArrivalResolved}
	if t.Travel != nil {
		q := *t.Travel
		view.Travel = &q
	}
	for i, stack := range t.Stacks {
		view.Supply[i] = len(stack)
	}
	for p, w := range t.Wagons {
		v := catanTransportWagonView{Position: w.Position, Level: w.Level, Delivered: len(w.Delivered), Points: t.extraPoints(p), Gold: t.Gold[p]}
		if token, ok := t.token(w.Cargo); ok {
			v.Cargo = &token
		}
		view.Wagons = append(view.Wagons, v)
	}
	return view
}
