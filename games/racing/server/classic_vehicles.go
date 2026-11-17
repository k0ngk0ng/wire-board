package racing

import "encoding/json"

var classicExtraVehicles = loadClassicExtraVehicles()

func loadClassicExtraVehicles() map[string]ClassicVehicleParams {
	data, err := classicRuleFiles.ReadFile("data/extra-vehicles.json")
	if err != nil {
		panic(err)
	}
	var catalog struct {
		Version  int                             `json:"version"`
		Vehicles map[string]ClassicVehicleParams `json:"vehicles"`
	}
	if json.Unmarshal(data, &catalog) != nil || catalog.Version != 1 || len(catalog.Vehicles) == 0 {
		panic("invalid extra classic vehicle catalog")
	}
	for id, params := range catalog.Vehicles {
		if params.ID != id || !validClassicVehicle(params) {
			panic("invalid extra classic vehicle parameters")
		}
		switch id {
		case "p911", "fm50", "ldia", "mcf1", "ccor":
			panic("extra catalog cannot replace an existing vehicle")
		}
	}
	return catalog.Vehicles
}
