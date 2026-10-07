package game

import "errors"

// Only an immediate lack of legal placements is a deadlock. A bounded bot
// search failing to find a completion is deliberately not sufficient evidence.
func (s *State) CatanExplorerSetupBlocked() bool {
	if s == nil || s.Finished || s.Phase != "catan_explorer_setup" || s.Catan == nil || s.Catan.Explorer == nil {
		return false
	}
	x := s.Catan.Explorer
	return x.Board != nil && x.Fleet != nil && x.Setup != nil && x.Setup.current(len(s.Catan.Players)) != nil && len(x.Setup.choices(s.Catan, x.Board, x.Fleet)) == 0
}

// Room-host authorization belongs to the server. This is not a player action,
// so State.Apply cannot be used to bypass that permission.
func (s *State) ResetCatanExplorerSetup() error {
	if s == nil || s.Catan == nil || s.Catan.Explorer == nil {
		return errors.New("只有开局无合法放置位置时才能重新布置")
	}
	if err := s.validateCatanExplorer(); err != nil {
		return err
	}
	if !s.CatanExplorerSetupBlocked() {
		return errors.New("只有开局无合法放置位置时才能重新布置")
	}
	next := clone(*s)
	g, x := next.Catan, next.Catan.Explorer
	setup := x.Setup
	setup.PromptBase += len(setup.plan(len(g.Players))) + 1
	setup.Step = 0
	for p := range setup.Harbors {
		setup.Harbors[p], setup.Settlements[p] = -1, -1
	}
	for v := range g.Vertices {
		g.Vertices[v].Owner, g.Vertices[v].Level, g.Vertices[v].Harbor = -1, 0, false
	}
	for e := range g.Edges {
		g.Edges[e].Owner = -1
	}
	for ship := range x.Fleet.Positions {
		x.Fleet.Positions[ship] = -1
	}
	for unit := range x.Cargo.Units {
		x.Cargo.Units[unit] = catanExplorerCargoLocation{"supply", -1}
	}
	for p := range g.Players {
		g.Players[p].Score = 0
	}
	next.Turn = setup.Start
	x.Motion = nil
	x.ActionID++
	next.Log = append(next.Log, "开局已无合法放置位置，房主重新布置起始棋子；保留地图、先手和牌堆")
	if err := next.validateCatanExplorer(); err != nil {
		return err
	}
	*s = next
	return nil
}
