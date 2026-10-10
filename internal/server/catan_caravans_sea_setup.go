package server

// Caravan sea room scenarios map to the engine's printed recipe names.
func publicCatanCaravansSeaSetup(scenario string) (string, bool) {
	switch scenario {
	case "caravans-shores":
		return "shores", true
	case "caravans-islands":
		return "islands", true
	case "caravans-desert":
		return "desert", true
	case "caravans-tribe":
		return "tribe", true
	case "caravans-new-world":
		return "new_world", true
	}
	return "", false
}
