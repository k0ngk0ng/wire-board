package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCatanHarborsConfigurationMapsAndValidation(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, info := range CatanSeafarersScenarios(n) {
			for _, ck := range []bool{false, true} {
				if ck && !CatanCitiesKnightsSeafarersSupported(info.ID) {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/CK=%v", n, info.ID, ck), func(t *testing.T) {
					var world *CatanNewWorldMap
					var err error
					if info.ID == "new_world" {
						world, err = GenerateCatanNewWorldMap(n)
						if err != nil {
							t.Fatal(err)
						}
					}
					var s *State
					if ck {
						s, err = NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: info.ID}, world)
					} else {
						s, err = NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: info.ID}, world)
					}
					if err != nil {
						t.Fatal(err)
					}
					target := s.Catan.victoryTarget()
					if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					if s.Catan.Harbors == nil || s.Catan.victoryTarget() != target+1 {
						t.Fatal("configuration not applied")
					}
					before, _ := json.Marshal(s)
					if err = s.ConfigureCatanHarbors(CatanHarborsSetup{}); err == nil {
						t.Fatal("existing award removed")
					}
					after, _ := json.Marshal(s)
					if string(before) != string(after) {
						t.Fatal("rejected configuration changed state")
					}
				})
			}
		}
	}
	for _, enabled := range []bool{false, true} {
		s, err := NewCatanConfigured(5, CatanOptions{FiveSix: true}, CatanBaseConfiguration{Layout: "fixed"})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: enabled}); err != nil {
			t.Fatal(err)
		}
		if (s.Catan.Harbors != nil) != enabled {
			t.Fatal("fixed board variant")
		}
	}
	for _, setup := range []CatanHarborsSetup{{Enabled: true}, {Enabled: true, Rules: "wrong"}} {
		s, err := NewCatan(3, CatanOptions{Helpers: true})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s)
		if err = s.ConfigureCatanHarbors(setup); err == nil {
			t.Fatal("invalid Helpers/version combination")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("failed configuration changed game")
		}
	}
	s := catanGame(t, 3)
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, s.Turn, a)
	if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err == nil {
		t.Fatal("mid-setup configuration accepted")
	}
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Phase = "catan_turn"
	if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err == nil {
		t.Fatal("started game configuration accepted")
	}
}
