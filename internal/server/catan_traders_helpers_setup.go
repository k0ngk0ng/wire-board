package server

import "github.com/k0ngk0ng/wire-board/internal/game"

func publicCatanTradersHelpers(s string) bool {
	return s == "rivers" || s == "caravans" || s == "barbarian-attack" || s == "transport" || publicCatanTradersCombination(s) || publicCatanRiversSea(s) || publicCatanCaravanSea(s) || publicCatanTransportSea(s) || s == "attack-shores" || s == "attack-desert" || s == "attack-tribe" || s == "attack-wonders" || s == "attack-pirates"
}
func (r *Room) catanTradersHelpersAvailable() bool {
	scene := r.CatanScenario
	if r.CatanTwoRules != "" {
		scene = r.CatanTwoScenario
	}
	return r.Kind == "catan" && publicCatanTradersHelpers(scene)
}
func validCatanTradersRoomOptions(o game.CatanOptions) bool {
	return validCatanExplorerOptions(o)
}
