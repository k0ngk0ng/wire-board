package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func explorerHelpersNew(t *testing.T, n int, scenario string, cities, fishing, events, all bool) *State {
	t.Helper()
	var s *State
	var err error
	if fishing {
		s, err = NewCatanExplorerFishing(n, scenario, cities, true)
	} else if cities {
		s, err = NewCatanExplorerCitiesKnights(n, scenario)
	} else if scenario == "land-ho" {
		s, err = NewCatanExplorerLandHo(n)
	} else if scenario == "spices-for-catan" {
		s, err = NewCatanExplorerSpices(n)
	} else {
		s, err = NewCatanExplorerMission(n, scenario)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err = s.enableCatanExplorerHelpers(all); err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func explorerHelpersStep(t *testing.T, s *State) Action {
	t.Helper()
	if s.CatanExplorerSetupBlocked() {
		if err := s.ResetCatanExplorerSetup(); err != nil {
			t.Fatal(err)
		}
	}
	p := explorerIntroActor(s)
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(s.Phase, p, err)
	}
	explorerHelpersTrace = append(explorerHelpersTrace, fmt.Sprintf("p%d %s c=%s vtx=%d phase=%s", p, a.Type, a.Choice, a.Vertex, s.Phase))
	if len(explorerHelpersTrace) > 8 {
		explorerHelpersTrace = explorerHelpersTrace[len(explorerHelpersTrace)-8:]
	}
	if err = s.Apply(p, a); err != nil {
		explorerHelpersFail(t, s, a, err)
		detail := fmt.Sprintf("trace=%v phase=%s turn=%d/%d roll=%d", explorerHelpersTrace, s.Phase, s.Turn, s.CatanPendingActor(), s.Catan.RollID)
		if x := s.Catan.Explorer; x != nil && x.Economy.Turn != nil {
			detail += fmt.Sprintf(" turnPhase=%s", x.Economy.Turn.Phase)
		}
		if f := s.Catan.Fishing; f != nil {
			detail += fmt.Sprintf(" fishPending=%+v fishLastRoll=%d fishClaims=%d", f.Pending, f.LastRollID, len(f.Tokens.Pending))
		}
		if k := s.Catan.CitiesKnights; k != nil {
			detail += fmt.Sprintf(" cityPending=%+v cityEvent=%v", k.Pending, k.Event != nil)
		}
		detail += fmt.Sprintf(" helper=%+v", s.Catan.HelperPending)
		t.Fatal(s.Phase, p, a, err, detail)
	}
	if x := s.Catan.Explorer; s.Catan.CitiesKnights != nil && x != nil && x.Economy != nil && x.Economy.Turn != nil && s.Catan.Fishing != nil {
		if err := s.validateExplorerCityProduction(); err != nil {
			explorerHelpersFail(t, s, a, err)
		}
	}
	return a
}

// Temporary diagnostic ring buffer for the rare explorer helper stall.
var explorerHelpersTrace []string

func TestCatanExplorerHelpersRecipesAndTurns(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"land-ho", "pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			for _, city := range []bool{false, true} {
				if city && n == 2 {
					continue
				}
				for _, all := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/%s/city%t/all%t", n, scenario, city, all), func(t *testing.T) {
						s := explorerHelpersNew(t, n, scenario, city, all, true, all)
						for step := 0; step < 600 && s.Catan.TurnSerial < 5; step++ {
							explorerHelpersStep(t, s)
							explorerFishingRestore(t, s)
						}
						if s.Catan.TurnSerial < 5 {
							t.Fatal("opening/turn did not progress")
						}
					})
				}
			}
		}
	}
}

// Explicit component fixture: use legal opening, then set a conserved hand
// and helper ownership to exercise each ability without relying on shuffle.
func explorerHelpersFixture(t *testing.T, city bool, id int) *State {
	t.Helper()
	s := explorerHelpersNew(t, 3, "land-ho", city, false, false, true)
	for step := 0; step < 100 && s.Phase != "catan_turn"; step++ {
		explorerHelpersStep(t, s)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("fixture did not reach construction")
	}
	explorerHelpersAssign(t, s, s.Turn, id)
	g := s.Catan
	for r, n := range g.Players[s.Turn].Resources {
		g.Bank[r] += n
		g.Players[s.Turn].Resources[r] = 0
	}
	for r := range 5 {
		g.Bank[r] -= 4
		g.Players[s.Turn].Resources[r] = 4
	}
	explorerFishingRestore(t, s)
	return s
}

func explorerHelpersAssign(t *testing.T, s *State, player, id int) {
	t.Helper()
	g := s.Catan
	old := g.Players[player].Helper.ID
	if old != id {
		at := slices.Index(g.HelperDisplay, id)
		if at >= 0 {
			g.HelperDisplay[at] = old
		} else {
			found := false
			for p := range g.Players {
				if p != player && g.Players[p].Helper != nil && g.Players[p].Helper.ID == id {
					g.Players[p].Helper = &CatanHelperSeat{ID: old}
					found = true
				}
			}
			if !found {
				t.Fatal("helper missing", id)
			}
		}
	}
	g.Players[player].Helper = &CatanHelperSeat{ID: id}
}

func explorerHelpersAct(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.Apply(p, a); err != nil {
		t.Fatal(a, err)
	}
	explorerFishingRestore(t, s)
}

func explorerHelpersReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before := clone(*s)
	if err := s.Apply(p, a); err == nil {
		t.Fatal("illegal action accepted", a)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected helper changed state")
	}
}

func TestCatanExplorerHelpersActions(t *testing.T) {
	for _, city := range []bool{false, true} {
		for _, id := range []int{1, 2, 4, 6, 8, 9, 10, 11, 12} {
			t.Run(fmt.Sprintf("city%t/id%d", city, id), func(t *testing.T) {
				s := explorerHelpersFixture(t, city, id)
				g, x, p := s.Catan, s.Catan.Explorer, s.Turn
				if id == 8 {
					// Explicit geometry fixture: extend legal roads until a village site exists.
					for step := 0; step < 5 && len(s.catanExplorerHelperChoices(p)) == 0; step++ {
						edge := catanExplorerBotRoad(g, p)
						if edge < 0 {
							break
						}
						if err := x.Cargo.buildRoadCost(g, x.Fleet, p, g.TurnSerial, edge, true); err != nil {
							t.Fatal(err)
						}
					}
				}
				choices := s.catanExplorerHelperChoices(p)
				if len(choices) == 0 {
					t.Fatal("no helper action")
				}
				var a Action
				encoded, err := json.Marshal(catanExplorerChoiceView(choices[:1])[0])
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(encoded, &a); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(a, choices[0]) {
					t.Fatal("helper preview lost action fields")
				}
				before := clone(*s)
				explorerHelpersReject(t, s, p, func() Action { b := a; b.Prompt++; return b }())
				explorerHelpersReject(t, s, (p+1)%len(g.Players), a)
				explorerHelpersAct(t, s, p, a)
				if s.Phase != "catan_helper" || s.Catan.HelperPending.Kind != "exchange" {
					t.Fatal("missing flip/exchange")
				}
				if id == 2 || id == 6 {
					for r, n := range a.Tokens {
						if s.Catan.Players[p].Resources[r] != before.Catan.Players[p].Resources[r]-n {
							t.Fatal("substitution payment")
						}
					}
				}
				explorerHelpersAct(t, s, p, Action{Type: "catan_helper_choice", Choice: "flip"})
				if s.Phase != "catan_turn" || !s.Catan.Players[p].Helper.Moon || s.Catan.helperReady(p, id) {
					t.Fatal("helper cooldown")
				}
				explorerHelpersReject(t, s, p, a)
			})
		}
	}
}

func TestCatanExplorerHelpersNaturalMatches(t *testing.T) {
	for _, tc := range []struct {
		n                  int
		scenario           string
		city, fish, events bool
	}{
		{2, "land-ho", false, false, false}, {5, "pirate-lairs", false, true, true},
		{3, "land-ho", true, true, true}, {6, "explorers-and-pirates", true, true, false},
	} {
		t.Run(fmt.Sprintf("%d/%s/city%t", tc.n, tc.scenario, tc.city), func(t *testing.T) {
			s := explorerHelpersNew(t, tc.n, tc.scenario, tc.city, tc.fish, tc.events, true)
			seen := map[string]int{}
			for step := 0; step < 16000 && !s.Finished; step++ {
				a := explorerHelpersStep(t, s)
				seen[a.Type]++
				if step%71 == 0 {
					explorerFishingRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || seen["catan_helper_choice"] == 0 {
				t.Fatal("natural helper game did not finish", s.Round, s.Phase)
			}
			explorerFishingRestore(t, s)
			t.Log("round", s.Round, "actions", seen)
		})
	}
}

// explorerHelpersFail saves the whole failing state and the recent actions so a
// CI shard can upload them, then reports the same detail inline.
func explorerHelpersFail(t *testing.T, s *State, a Action, err error) {
	t.Helper()
	detail := fmt.Sprintf("phase=%s turn=%d/%d roll=%d", s.Phase, s.Turn, s.CatanPendingActor(), s.Catan.RollID)
	if x := s.Catan.Explorer; x != nil && x.Economy != nil && x.Economy.Turn != nil {
		detail += fmt.Sprintf(" turnPhase=%s noProduction=%t", x.Economy.Turn.Phase, x.Economy.Turn.NoProduction)
	}
	if f := s.Catan.Fishing; f != nil {
		detail += fmt.Sprintf(" fishPending=%+v fishLastRoll=%d claims=%d", f.Pending, f.LastRollID, len(f.Tokens.Pending))
	}
	if k := s.Catan.CitiesKnights; k != nil {
		detail += fmt.Sprintf(" cityPending=%+v cityEvent=%t", k.Pending, k.Event != nil)
	}
	detail += fmt.Sprintf(" helper=%+v", s.Catan.HelperPending)
	path := filepath.Join("..", "..", ".local", "explorer-helpers-failure.json")
	if raw, marshalErr := json.Marshal(s); marshalErr == nil {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr == nil {
			if writeErr := os.WriteFile(path, raw, 0o600); writeErr == nil {
				detail += " state=" + path
			}
		}
	}
	t.Fatalf("action %s failed: %v trace=%v %s", a.Type, err, explorerHelpersTrace, detail)
}
