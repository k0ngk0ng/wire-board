package game

// Land Ho has no printed 5–6 opening or city opening. These configurations
// use an explicit site recipe; the original 2–4 printed game is unchanged.
const CatanExplorerIntroRules = "wire-board-land-ho-variants-v1"

func catanExplorerIntroVariant(players int, scenario string, cities bool) bool {
	return scenario == "land-ho" && (players > 4 || cities)
}

func newCatanExplorerIntro(players int, cities bool) (*State, error) {
	if cities {
		return newCatanExplorerCityState(players, "land-ho", nil)
	}
	g, b, f, c, e, setup, err := newCatanExplorerMissionSetup(players, "land-ho", "variable", catanRandom(players))
	if err != nil {
		return nil, err
	}
	g.Explorer = &catanExplorer{Board: b, Fleet: f, Cargo: c, Economy: e, Setup: setup}
	s := &State{Kind: "catan", Catan: g, Turn: setup.Start, Round: 1, Phase: "catan_explorer_setup", Log: []string{
		"本站五六人初航：扩大地图、自由开局、配对回合，8分获胜；无海盗、巢穴、鱼群和香料任务",
	}}
	return s, s.validateCatanExplorer()
}
