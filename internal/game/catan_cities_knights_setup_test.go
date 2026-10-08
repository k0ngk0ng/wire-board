package game

import "testing"

func TestCatanCitiesKnightsConfigurationIdentityAndValidation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		setup, err := NormalizeCatanCitiesKnightsSetup(n, CatanCitiesKnightsSetup{})
		if err != nil || setup.Layout != "variable" || setup.Rules != catanCitiesKnightsRules(n) {
			t.Fatal(n, setup, err)
		}
		state, err := NewCatanCitiesKnightsConfigured(n, CatanOptions{FiveSix: n > 4}, setup)
		if err != nil {
			t.Fatal(err)
		}
		if state.Catan.CitiesKnightsSetup() != setup || state.Catan.CitiesKnights.Rules != setup.Rules || state.Catan.SetupStep != 0 {
			t.Fatal("configuration not frozen in game")
		}
		old := clone(*state)
		old.Catan.CitiesKnights.Layout = ""
		old.Catan.CitiesKnights.Rules = CatanCitiesKnightsRules
		if old.Catan.CitiesKnightsSetup() != setup || old.Catan.CitiesKnights.Layout != "" {
			t.Fatal("legacy metadata fallback mutated or misidentified state")
		}
		for _, bad := range []CatanCitiesKnightsSetup{{Layout: "fixed"}, {Layout: "prepared"}, {Rules: "unknown"}, {Rules: "catan-base-2025"}} {
			if _, err := NewCatanCitiesKnightsConfigured(n, CatanOptions{FiveSix: n > 4}, bad); err == nil {
				t.Fatal("invalid rules accepted", bad)
			}
		}
		if _, err := NewCatanCitiesKnightsConfigured(n, CatanOptions{FiveSix: n <= 4}, setup); err == nil {
			t.Fatal("wrong player extension accepted")
		}
		if helper, err := NewCatanCitiesKnightsConfigured(n, CatanOptions{FiveSix: n > 4, Helpers: true}, setup); err != nil || !helper.Catan.cityHelpers() {
			t.Fatal("missing versioned Helpers adaptation", err)
		}
	}
	for _, n := range []int{2, 7} {
		if _, err := NormalizeCatanCitiesKnightsSetup(n, CatanCitiesKnightsSetup{}); err == nil {
			t.Fatal("unsupported number", n)
		}
	}
	if _, err := NormalizeCatanCitiesKnightsSetup(6, CatanCitiesKnightsSetup{Rules: CatanCitiesKnightsRules}); err == nil {
		t.Fatal("wrong rules version accepted")
	}
	if _, err := NormalizeCatanCitiesKnightsSetup(3, CatanCitiesKnightsSetup{Rules: CatanCitiesKnightsFiveSixRules}); err == nil {
		t.Fatal("wrong rules version accepted")
	}
}
