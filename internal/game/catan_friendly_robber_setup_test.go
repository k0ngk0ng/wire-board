package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCatanFriendlyRobberConfiguration(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"variable"}
		if n >= 5 {
			layouts = append(layouts, "fixed")
		}
		for _, layout := range layouts {
			for _, harbors := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/harbors=%v", n, layout, harbors), func(t *testing.T) {
					s, err := NewCatanConfigured(n, CatanOptions{FiveSix: n > 4}, CatanBaseConfiguration{Layout: layout})
					if err != nil {
						t.Fatal(err)
					}
					if harbors {
						if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
					}
					target := s.Catan.victoryTarget()
					if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
						t.Fatal(err)
					}
					if s.Catan.FriendlyRobber == nil || s.Catan.victoryTarget() != target {
						t.Fatal("variant/target")
					}
					before, _ := json.Marshal(s)
					if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{}); err == nil {
						t.Fatal("reconfiguration")
					}
					after, _ := json.Marshal(s)
					if string(before) != string(after) {
						t.Fatal("rejected edit changed state")
					}
				})
			}
		}
	}
	for _, kind := range []string{"sea", "ck", "version", "setup", "playing", "finished", "wrong-kind", "nil"} {
		t.Run(kind, func(t *testing.T) {
			s := catanGame(t, 3)
			request := CatanFriendlyRobberSetup{Enabled: true}
			switch kind {
			case "sea":
				s.Catan.Seafarers = &CatanSeafarers{}
			case "ck":
				s.Catan.CitiesKnights = &CatanCitiesKnights{}
			case "version":
				request.Rules = "future"
			case "setup":
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, s.Turn, a)
			case "playing":
				s.Phase = "catan_turn"
				s.Catan.SetupStep = s.Catan.SetupLimit()
			case "finished":
				s.Finished = true
			case "wrong-kind":
				s.Kind = "splendor"
			case "nil":
				s = nil
			}
			before, _ := json.Marshal(s)
			if err := s.ConfigureCatanFriendlyRobber(request); err == nil {
				t.Fatal("invalid configuration accepted")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejection changed state")
			}
		})
	}
	s := catanGame(t, 3)
	if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{}); err != nil || s.Catan.FriendlyRobber != nil {
		t.Fatal("disabled variant", err)
	}
}
