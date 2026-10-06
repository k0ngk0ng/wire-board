package game

import (
	"errors"
	"slices"
)

// Official E&P 2025 variable setup (mission guide p9). Independent of the
// printed Land Ho opening. Private until full lair state/recipe acceptance.
type catanExplorerSetup struct {
	Start       int   `json:"start"`
	Step        int   `json:"step"`
	Harbors     []int `json:"harbors"`
	Settlements []int `json:"settlements"`
}
type catanExplorerSetupStep struct {
	Player int    `json:"player"`
	Owner  int    `json:"owner"`
	Kind   string `json:"kind"`
}

func catanExplorerSetupPlan(players, start int) []catanExplorerSetupStep {
	result := []catanExplorerSetupStep{}
	for i := 0; i < players; i++ {
		p := (start + i) % players
		result = append(result, catanExplorerSetupStep{p, p, "harbor"})
	}
	if players == 2 {
		for i := 0; i < 2; i++ {
			result = append(result, catanExplorerSetupStep{(start + i) % 2, -2 - i, "harbor"})
		}
	}
	for i := players - 1; i >= 0; i-- {
		p := (start + i) % players
		result = append(result, catanExplorerSetupStep{p, p, "settlement"})
	}
	if players == 2 {
		for i := 1; i >= 0; i-- {
			result = append(result, catanExplorerSetupStep{(start + i) % 2, -2 - i, "settlement"})
		}
	}
	for i := 0; i < players; i++ {
		p := (start + i) % players
		result = append(result, catanExplorerSetupStep{p, p, "road"}, catanExplorerSetupStep{p, p, "ship"})
	}
	return result
}
func catanExplorerSetupIndex(players, owner int) int {
	if owner >= 0 {
		return owner
	}
	return players - owner - 2
}
func newCatanExplorerLairsSetup(players int, layout string, start int) (*Catan, *catanExplorerBoard, *catanExplorerSailing, *catanExplorerCargo, *catanExplorerEconomy, *catanExplorerSetup, error) {
	return newCatanExplorerMissionSetup(players, "pirate-lairs", layout, start)
}
func newCatanExplorerMissionSetup(players int, scenario, layout string, start int) (*Catan, *catanExplorerBoard, *catanExplorerSailing, *catanExplorerCargo, *catanExplorerEconomy, *catanExplorerSetup, error) {
	if players < 2 || players > 4 || start < 0 || start >= players {
		return nil, nil, nil, nil, nil, nil, errors.New("巢穴开局人数或先手无效")
	}
	g, b, err := newCatanExplorerBoard(players, scenario, layout)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	f, _ := newCatanExplorerSailing(players)
	g.Bank = []int{19, 19, 19, 19, 19}
	g.StartPlayer = start
	g.SetupVertex = -1
	g.Dice = []int{0, 0}
	g.DiscardDue = make([]int, players)
	g.Victims = []int{}
	g.DevDeck = []int{}
	g.DevDiscard = []int{}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
		g.Players[p].Dev = make([]int, 5)
		g.Players[p].NewDev = make([]int, 5)
	}
	c, err := newCatanExplorerCargo(g, f, scenario)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	e, err := newCatanExplorerEconomy(g, f, c)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	count := players
	if players == 2 {
		count += 2
	}
	setup := &catanExplorerSetup{Start: start, Harbors: make([]int, count), Settlements: make([]int, count)}
	for i := range setup.Harbors {
		setup.Harbors[i], setup.Settlements[i] = -1, -1
	}
	return g, b, f, c, e, setup, setup.validate(g, b, f, c, e)
}
func (s catanExplorerSetup) current(players int) *catanExplorerSetupStep {
	plan := catanExplorerSetupPlan(players, s.Start)
	if s.Step < 0 || s.Step >= len(plan) {
		return nil
	}
	return &plan[s.Step]
}
func (s catanExplorerSetup) validate(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) error {
	if g == nil || b == nil || f == nil || c == nil || e == nil || !catanExplorerMissionScenario(b.Scenario) || c.Scenario != b.Scenario || s.Start < 0 || s.Start >= len(g.Players) || g.StartPlayer != s.Start {
		return errors.New("巢穴开局组件或先手无效")
	}
	for _, loc := range c.Fish {
		if loc != (catanExplorerCargoLocation{"supply", -1}) {
			return errors.New("开局鱼群必须全部在公共供应区")
		}
	}
	if err := b.validate(g); err != nil {
		return err
	}
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	n := len(g.Players)
	count := n
	if n == 2 {
		count += 2
	}
	plan := catanExplorerSetupPlan(n, s.Start)
	if len(s.Harbors) != count || len(s.Settlements) != count || s.Step < 0 || s.Step > len(plan) {
		return errors.New("开局记录长度或步骤无效")
	}
	// During setup every piece must be accounted for by a completed plan step.
	harbors, settlements, roads, ships := map[int]bool{}, map[int]bool{}, map[int]bool{}, map[int]bool{}
	for _, step := range plan[:s.Step] {
		switch step.Kind {
		case "harbor":
			harbors[step.Owner] = true
		case "settlement":
			settlements[step.Owner] = true
		case "road":
			roads[step.Owner] = true
		case "ship":
			ships[step.Owner] = true
		}
	}
	vertexCount := 0
	for ownerIndex := 0; ownerIndex < count; ownerIndex++ {
		owner := ownerIndex
		if ownerIndex >= n {
			owner = -2 - (ownerIndex - n)
		}
		h, v := s.Harbors[ownerIndex], s.Settlements[ownerIndex]
		if (h >= 0) != harbors[owner] || (v >= 0) != settlements[owner] || h < -1 || v < -1 {
			return errors.New("开局建筑与放置顺序不一致")
		}
		if h >= 0 {
			if h >= len(g.Vertices) || !slices.Contains(b.HarborStarts, h) || g.Vertices[h].Owner != owner || g.Vertices[h].Level != 2 {
				return errors.New("起始港口必须在印刷绿点")
			}
			vertexCount++
		}
		if v >= 0 {
			if v >= len(g.Vertices) || !catanExplorerSetupStartingVertex(g, b, v) || g.Vertices[v].Owner != owner || g.Vertices[v].Level != 1 {
				return errors.New("起始村庄必须在起始岛")
			}
			vertexCount++
		}
	}
	actual := 0
	for _, v := range g.Vertices {
		if v.Level > 0 {
			actual++
		}
	}
	if actual != vertexCount {
		return errors.New("出现未经放置的起始建筑")
	}
	for owner := -3; owner < n; owner++ {
		roadCount := 0
		for _, edge := range g.Edges {
			if edge.Owner == owner && owner != -1 {
				roadCount++
				if owner < 0 || edge.A != s.Settlements[owner] && edge.B != s.Settlements[owner] {
					return errors.New("起始道路须接自己的起始村庄；中立方无起始道路")
				}
			}
		}
		want := 0
		if roads[owner] {
			want = 1
		}
		if roadCount != want {
			return errors.New("起始道路数量不符")
		}
	}
	for ship, edge := range f.Positions {
		owner := ship / 3
		if ships[owner] && ship%3 == 0 {
			h := s.Harbors[owner]
			if edge < 0 || g.Edges[edge].A != h && g.Edges[edge].B != h {
				return errors.New("起始船必须停在自己港口旁")
			}
		} else if edge != -1 {
			return errors.New("起始船数量不符")
		}
	}
	for id, loc := range c.Units {
		expected := catanExplorerCargoLocation{"supply", -1}
		if id%11 == 0 && ships[id/11] {
			expected = catanExplorerCargoLocation{"ship", id / 11 * 3}
		}
		if loc != expected {
			return errors.New("起始每人只有一艘载移民船，无中立船")
		}
	}
	done := s.Step == len(plan)
	if done {
		if g.SetupStep != 2*n || g.TurnSerial != 1 || e.Turn == nil || e.Turn.Player != s.Start || e.Turn.Sequence != 1 || e.Turn.Phase != "roll" {
			return errors.New("完成开局后必须从先手第一次生产开始")
		}
	} else if g.SetupStep != 0 || g.TurnSerial != 0 || e.Turn != nil {
		return errors.New("开局未完不能开始生产")
	}
	for p, player := range g.Players {
		score := 0
		if harbors[p] {
			score += 2
		}
		if settlements[p] {
			score++
		}
		if player.Eliminated || player.Score != score || !catanBundle(player.Dev) || !catanBundle(player.NewDev) || sum(player.Dev)+sum(player.NewDev) != 0 {
			return errors.New("起始玩家得分或状态无效")
		}
		want := make([]int, 5)
		if done {
			for _, tile := range g.Tiles {
				if tile.Resource < 5 && slices.Contains(tile.Vertices, s.Settlements[p]) {
					want[tile.Resource]++
				}
			}
		}
		if !slices.Equal(player.Resources, want) || e.Gold[p] != 2 {
			return errors.New("起始资源只能来自村庄，每人2金币")
		}
	}
	return nil
}
func catanExplorerSetupStartingVertex(g *Catan, b *catanExplorerBoard, vertex int) bool {
	if !catanExplorerLandVertex(g, vertex) {
		return false
	}
	for _, tile := range b.Starting {
		if slices.Contains(g.Tiles[tile].Vertices, vertex) {
			return true
		}
	}
	return false
}
func (s catanExplorerSetup) choices(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing) []int {
	result := []int{}
	step := s.current(len(g.Players))
	if step == nil {
		return result
	}
	switch step.Kind {
	case "harbor", "settlement":
		for _, v := range g.Vertices {
			if v.Level != 0 || step.Kind == "harbor" && !slices.Contains(b.HarborStarts, v.ID) || step.Kind == "settlement" && !catanExplorerSetupStartingVertex(g, b, v.ID) {
				continue
			}
			spaced := true
			for _, edge := range g.Edges {
				if edge.A == v.ID && g.Vertices[edge.B].Level > 0 || edge.B == v.ID && g.Vertices[edge.A].Level > 0 {
					spaced = false
					break
				}
			}
			if spaced {
				result = append(result, v.ID)
			}
		}
	case "road":
		v := s.Settlements[step.Owner]
		for _, edge := range g.Edges {
			if edge.Owner == -1 && (edge.A == v || edge.B == v) && catanExplorerLandEdge(g, edge.ID) {
				result = append(result, edge.ID)
			}
		}
	case "ship":
		v := s.Harbors[step.Owner]
		for _, edge := range g.Edges {
			if (edge.A == v || edge.B == v) && catanExplorerSeaEdge(g, edge.ID) && !slices.Contains(f.Positions, edge.ID) {
				result = append(result, edge.ID)
			}
		}
	}
	return result
}
func (s *catanExplorerSetup) place(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player, prompt int, kind string, target int) error {
	if err := s.validate(g, b, f, c, e); err != nil {
		return err
	}
	step := s.current(len(g.Players))
	if step == nil || player != step.Player || prompt != s.Step+1 || kind != step.Kind || !slices.Contains(s.choices(g, b, f), target) {
		return errors.New("不是当前开局玩家、步骤或合法放置位置")
	}
	base := *g
	base.Explorer = nil
	ng, nf, nc, ne, ns := clone(base), clone(*f), clone(*c), clone(*e), clone(*s)
	owner := step.Owner
	index := catanExplorerSetupIndex(len(g.Players), owner)
	switch kind {
	case "harbor":
		ng.Vertices[target].Owner, ng.Vertices[target].Level = owner, 2
		ns.Harbors[index] = target
		if owner >= 0 {
			ng.Players[owner].Score += 2
		}
	case "settlement":
		ng.Vertices[target].Owner, ng.Vertices[target].Level = owner, 1
		ns.Settlements[index] = target
		if owner >= 0 {
			ng.Players[owner].Score++
		}
	case "road":
		ng.Edges[target].Owner = owner
	case "ship":
		nf.Positions[owner*3] = target
		nc.Units[owner*11] = catanExplorerCargoLocation{"ship", owner * 3}
	}
	ns.Step++
	if ns.current(len(g.Players)) == nil {
		for p := range ng.Players {
			for _, tile := range ng.Tiles {
				if tile.Resource < 5 && slices.Contains(tile.Vertices, ns.Settlements[p]) {
					if ng.Bank[tile.Resource] == 0 {
						return errors.New("起始资源库存不足")
					}
					ng.Bank[tile.Resource]--
					ng.Players[p].Resources[tile.Resource]++
				}
			}
		}
		ng.SetupStep, ng.TurnSerial = 2*len(g.Players), 1
		if err := ne.beginProduction(&ng, &nf, &nc, ns.Start, 1); err != nil {
			return err
		}
	}
	if err := ns.validate(&ng, b, &nf, &nc, &ne); err != nil {
		return err
	}
	ng.Explorer = g.Explorer
	*g, *f, *c, *e, *s = ng, nf, nc, ne, ns
	return nil
}
