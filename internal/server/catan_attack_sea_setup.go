package server

// Attack sea room scenarios map to the engine's printed recipe names.
func publicCatanAttackSeaSetup(scenario string) (string, bool) {
	switch scenario {
	case "attack-shores":
		return "shores", true
	case "attack-desert":
		return "desert", true
	case "attack-tribe":
		return "tribe", true
	case "attack-wonders":
		return "wonders", true
	case "attack-pirates":
		return "pirates", true
	}
	return "", false
}
