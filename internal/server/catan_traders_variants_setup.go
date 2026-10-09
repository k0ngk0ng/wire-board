package server

func publicCatanTradersVariants(s string) bool { return publicCatanTradersHelpers(s) }
func (r *Room) catanTradersVariantsAvailable() bool {
	scene := r.CatanScenario
	if r.CatanTwoRules != "" {
		scene = r.CatanTwoScenario
	}
	return r.Kind == "catan" && r.Capacity >= 2 && r.Capacity <= 6 && publicCatanTradersVariants(scene)
}
