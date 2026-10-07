package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// The legal city-first opening is real. Lair token faces are explicit synthetic
// acceptance components, not a claim about the still-unverified retail set.
func explorerCityFlowFixture(t *testing.T, n int, scenario string) *State {
	t.Helper()
	s := explorerCityScenarioFixture(t, n, scenario)
	x := s.Catan.Explorer
	if catanExplorerMissionScenario(scenario) {
		numbers := []int{3, 4, 5, 9, 10, 11}
		if n > 4 {
			numbers = append(numbers, 3, 4)
		}
		var err error
		x.Lairs, err = newCatanExplorerLairs(n, numbers)
		if err != nil {
			t.Fatal(err)
		}
	}
	if catanExplorerFishScenario(scenario) {
		x.Fish = &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}
	}
	if catanExplorerSpiceScenario(scenario) {
		x.Spice = &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}
	}
	explorerCityFlowRestore(t, s)
	return s
}
func explorerCityFlowRestore(t *testing.T, s *State) {
	t.Helper()
	explorerCityRestore(t, s)
	if err := s.validateExplorerCityFlow(); err != nil {
		t.Fatal(err)
	}
}
func explorerCityFlowAct(t *testing.T, s *State, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.catanExplorerCityAction(s.Turn, a); err != nil {
		t.Fatal(a, err)
	}
	explorerCityFlowRestore(t, s)
}
func explorerCityFlowChoice(t *testing.T, s *State, kind string) Action {
	t.Helper()
	for _, a := range s.catanExplorerChoices(s.Turn) {
		if a.Type == kind {
			return a
		}
	}
	t.Fatal("missing choice", kind, s.Phase)
	return Action{}
}
func explorerCityFlowRoll(t *testing.T, s *State) {
	t.Helper()
	// Use a science event without upgrades, avoiding incidental barbarian battles.
	if err := s.catanExplorerCityRoll(1, 1, 0); err != nil {
		t.Fatal(err)
	}
	explorerCityFlowRestore(t, s)
}

func TestCatanExplorerCityFlowRoundsAndPairedMovement(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := explorerCityFlowFixture(t, n, scenario)
				portions := 2 * n
				if n > 4 {
					portions *= 2
				}
				rolls := 0
				for step := 0; step < portions; step++ {
					if s.Phase == "catan_roll" {
						explorerCityFlowRoll(t, s)
						rolls++
					}
					p, serial, rollID := s.Turn, s.Catan.TurnSerial, s.Catan.RollID
					if rollID != rolls {
						t.Fatal("paired action repeated production")
					}
					before := clone(*s.Catan.Explorer.Economy)
					explorerCityActionReject(t, s, p, Action{Type: "catan_end", Prompt: int(serial)})
					explorerCityActionReject(t, s, (p+1)%n, Action{Type: "catan_explorer_begin_move", Prompt: int(serial)})
					explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
					if s.Turn != p || s.Catan.TurnSerial != serial || s.Phase != "catan_explorer_move" || !reflect.DeepEqual(*s.Catan.Explorer.Economy, before) {
						t.Fatal("begin movement changed production/player/economy")
					}
					explorerCityActionReject(t, s, p, Action{Type: "catan_explorer_begin_move", Prompt: int(serial)})
					for _, kind := range []string{"catan_explorer_bank", "catan_roll", "catan_progress", "catan_knight_activate", "catan_trade_offer"} {
						explorerCityActionReject(t, s, p, Action{Type: kind, Prompt: int(serial)})
					}
					// Move an actual ship and verify points survive JSON restoration.
					sail := explorerCityFlowChoice(t, s, "catan_explorer_sail")
					ship := sail.Slot
					prior := s.Catan.Explorer.Fleet.Turn.Ships[ship].Remaining
					explorerCityFlowAct(t, s, sail)
					if s.Catan.Explorer.Fleet.Positions[ship] != sail.Targets[len(sail.Targets)-1] || s.Catan.Explorer.Fleet.Turn.Ships[ship].Remaining >= prior {
						t.Fatal("sailing did not move ship and consume allowance")
					}
					explorerCityFlowAct(t, s, Action{Type: "catan_end"})
					if s.Catan.TurnSerial != serial+1 || s.Catan.CitiesKnights.ActionSerial != serial+1 || s.Catan.RollID != rollID || s.Catan.Explorer.Fleet.Turn.Open {
						t.Fatal("handoff reset rolls or kept movement open")
					}
					explorerCityActionReject(t, s, s.Turn, Action{Type: "catan_end", Prompt: int(serial)})
				}
				if s.Round != 3 || s.Turn != 0 || s.Phase != "catan_roll" {
					t.Fatal("ordinary/paired rotation drifted", s.Round, s.Turn, s.Phase)
				}
				if err := s.Catan.Explorer.validate(s.Catan); err == nil {
					t.Fatal("public combination gate opened")
				}
			})
		}
	}
}

func TestCatanExplorerCityFlowProgressLimitBeforeMovement(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, secondary := range []bool{false, true} {
			if n == 3 && secondary {
				continue
			}
			t.Run(fmt.Sprintf("%d/%t", n, secondary), func(t *testing.T) {
				s := explorerCityFlowFixture(t, n, "pirate-lairs")
				explorerCityFlowRoll(t, s)
				if secondary {
					explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
					explorerCityFlowAct(t, s, Action{Type: "catan_end"})
				}
				p := s.Turn
				ckProgressGive(t, s, p, 0, 1, 2, 3, 4, 5, 13, 10)
				explorerCityFlowAct(t, s, Action{Type: "catan_progress", Card: 13, Color: 5})
				explorerCityFlowAct(t, s, Action{Type: "catan_progress", Card: 10})
				explorerCityFlowAct(t, s, Action{Type: "catan_explorer_bank", Color: -1, Target: 0})
				before := clone(*s.Catan)
				explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
				q := s.Catan.CitiesKnights.Pending
				if s.Phase != "catan_progress_end" || q.Source != "explorer_movement" || q.Players[0] != p || !reflect.DeepEqual(s.Catan.Explorer.Fleet, before.Explorer.Fleet) || s.Catan.Explorer.Cargo.Turn.Phase != "action" {
					t.Fatal("overflow began sailing or handed off too early")
				}
				for _, a := range []Action{{Type: "catan_progress", Card: 4}, {Type: "catan_explorer_begin_move"}, {Type: "catan_end"}, {Type: "catan_progress_discard", Cards: []int{0}}, {Type: "catan_progress_discard", Cards: []int{0, 0}}, {Type: "catan_progress_discard", Cards: []int{0, 21}}, {Type: "catan_progress_discard", Cards: []int{0, 1}, Choice: "skip"}} {
					a.Prompt = int(s.Catan.TurnSerial)
					explorerCityActionReject(t, s, p, a)
				}
				discard := Action{Type: "catan_progress_discard", Cards: []int{0, 1}, Prompt: int(s.Catan.TurnSerial)}
				explorerCityReject(t, s, (p+1)%n, discard)
				stale := discard
				stale.Prompt = 0
				explorerCityReject(t, s, p, stale)
				// Exercise the common pending-response entry, which must not invoke base end-turn.
				if err := s.catanExplorerCityRespond(p, discard); err != nil {
					t.Fatal(err)
				}
				explorerCityFlowRestore(t, s)
				if s.Phase != "catan_explorer_move" || s.Turn != p || s.Catan.TurnSerial != before.TurnSerial || len(s.Catan.CitiesKnights.Players[p].Progress) != 4 || s.Catan.CitiesKnights.Pending != nil || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Explorer.Economy) || !reflect.DeepEqual(s.Catan.CitiesKnights.TradePowers, before.CitiesKnights.TradePowers) {
					t.Fatal("overflow response did not resume same movement portion")
				}
				if !slices.Equal(s.Catan.CitiesKnights.ProgressDecks[0][:2], []int{1, 0}) {
					t.Fatal("discard not returned face down to matching deck bottom")
				}
				explorerCityFlowAct(t, s, Action{Type: "catan_end"})
				if s.Catan.CitiesKnights.TradePowers != nil || s.Catan.Explorer.Economy.Turn.Bought != 0 {
					t.Fatal("next actor inherited bank/trade allowance")
				}
			})
		}
	}
}

func TestCatanExplorerCityFlowShipAndCrewConstruction(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := explorerCityFlowFixture(t, n, "explorers-and-pirates")
		explorerCityFlowRoll(t, s)
		for r, amount := range []int{2, 0, 3, 0, 2} {
			explorerDevelopmentGrant(t, s, 0, r, amount)
		}
		before := clone(*s.Catan)
		ship := explorerCityFlowChoice(t, s, "catan_explorer_ship")
		explorerCityFlowAct(t, s, ship)
		if s.Catan.Players[0].Resources[0] != before.Players[0].Resources[0]-1 || s.Catan.Players[0].Resources[2] != before.Players[0].Resources[2]-1 || s.Catan.Explorer.Fleet.Positions[ship.Slot] != ship.Edge {
			t.Fatal("wrong ship construction or payment")
		}
		var crew Action
		found := false
		for _, a := range s.catanExplorerChoices(0) {
			if a.Type == "catan_explorer_unit" && a.Card%11 >= 2 {
				crew = a
				found = true
				break
			}
		}
		if !found {
			t.Fatal("no legal crew construction")
		}
		before = clone(*s.Catan)
		explorerCityFlowAct(t, s, crew)
		if s.Catan.Players[0].Resources[2] != before.Players[0].Resources[2]-1 || s.Catan.Players[0].Resources[4] != before.Players[0].Resources[4]-1 || s.Catan.Explorer.Cargo.Units[crew.Card] != (catanExplorerCargoLocation{crew.Choice, crew.Target}) {
			t.Fatal("wrong crew location/cost")
		}
		explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
		explorerCityFlowAct(t, s, Action{Type: "catan_explorer_wool", Slot: ship.Slot})
		if s.Catan.Explorer.Fleet.Turn.Ships[ship.Slot].Remaining != 6 {
			t.Fatal("wool did not grant independent ship allowance")
		}
		explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_wool", Slot: ship.Slot, Prompt: int(s.Catan.TurnSerial)})
	}
}

func TestCatanExplorerCityFlowDiscoveryAtomicAndCommoditySafe(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityFlowFixture(t, n, "explorers-and-pirates")
			discovered := false
			for step := 0; step < 5*n*2 && !discovered; step++ {
				if s.Phase == "catan_roll" {
					explorerCityFlowRoll(t, s)
				}
				p := s.Turn
				explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
				if p == 0 {
					g, x := s.Catan, s.Catan.Explorer
					path := explorerWorldFogPath(g, x.Fleet.Positions[0])
					if len(path) == 0 {
						t.Fatal("no public path to fog")
					}
					path = path[:min(4, len(path))]
					a := Action{Type: "catan_explorer_sail", Slot: 0, Targets: path, Prompt: int(g.TurnSerial)}
					quote, err := x.Fleet.quote(g, p, g.TurnSerial, 0, path, -1, -1)
					if err != nil {
						t.Fatal(err)
					}
					before := clone(*s.Catan)
					if len(quote.Exploring) > 0 {
						// Empty the corresponding reward bank without breaking conservation.
						// Resource shortages still reject atomically; empty coins now
						// use ledger credits without concealing a legal destination.
						bad := clone(*s)
						hidden := x.Board.Hidden[slices.IndexFunc(x.Board.Hidden, func(h catanExplorerHidden) bool { return h.Tile == quote.Exploring[0] })]
						if hidden.Resource < 5 {
							r := hidden.Resource
							bad.Catan.Players[1].Resources[r] += bad.Catan.Bank[r]
							bad.Catan.Bank[r] = 0
						} else {
							e := bad.Catan.Explorer.Economy
							e.Gold[1] += e.GoldBank
							e.GoldBank = 0
						}
						explorerCityFlowRestore(t, &bad)
						if hidden.Resource < 5 {
							explorerCityActionReject(t, &bad, 0, a)
						} else {
							explorerCityFlowAct(t, &bad, a)
							if bad.Catan.Explorer.Economy.GoldIssued == 0 {
								t.Fatal("gold discovery failed to use ledger")
							}
						}
					}
					explorerCityFlowAct(t, s, a)
					if !reflect.DeepEqual(s.Catan.CitiesKnights, before.CitiesKnights) || !slices.Equal(s.Catan.Players[0].Resources[5:], before.Players[0].Resources[5:]) {
						t.Fatal("exploration changed city components or minted commodities")
					}
					if len(quote.Exploring) > 0 {
						discovered = true
						wantCards := slices.Clone(before.Players[0].Resources)
						wantGold := before.Explorer.Economy.Gold[0]
						for _, tile := range quote.Exploring {
							r := s.Catan.Tiles[tile].Resource
							if r == CatanFog {
								t.Fatal("discovery remained hidden")
							}
							if r < 5 {
								wantCards[r]++
							} else {
								wantGold += 2
							}
						}
						if !slices.Equal(s.Catan.Players[0].Resources, wantCards) || s.Catan.Explorer.Economy.Gold[0] != wantGold || !s.Catan.Explorer.Fleet.Turn.Ships[0].Closed || len(s.Catan.Explorer.Fleet.Turn.Exploring) != 0 {
							t.Fatal("wrong exploration reward or ship not stopped")
						}
						explorerCityActionReject(t, s, 0, a)
					}
				}
				explorerCityFlowAct(t, s, Action{Type: "catan_end"})
			}
			if !discovered {
				t.Fatal("never reached fog through real turns")
			}
		})
	}
}

func TestCatanExplorerCityFlowCorruptionAndVictory(t *testing.T) {
	s := explorerCityFlowFixture(t, 3, "pirate-lairs")
	explorerCityFlowRoll(t, s)
	ckProgressGive(t, s, 0, 0, 1, 2, 3, 4)
	explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
	for name, damage := range map[string]func(*State){
		"source":  func(s *State) { s.Catan.CitiesKnights.Pending.Source = "" },
		"actor":   func(s *State) { s.Catan.CitiesKnights.Pending.Players = []int{1} },
		"phase":   func(s *State) { s.Phase = "catan_progress_discard" },
		"mission": func(s *State) { s.Catan.Explorer.Lairs = nil },
		"discard": func(s *State) { s.Catan.DiscardDue[0] = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			damage(&bad)
			if bad.validateExplorerCityFlow() == nil {
				t.Fatal("corrupt flow save accepted")
			}
			explorerCityActionReject(t, &bad, 0, Action{Type: "catan_progress_discard", Cards: []int{0}, Prompt: int(s.Catan.TurnSerial)})
		})
	}
	winner := explorerCityFlowFixture(t, 3, "pirate-lairs")
	explorerCityFlowRoll(t, winner)
	// Controlled score tests immediate victory before the overflow prompt.
	winner.Catan.CitiesKnights.Players[0].DefenderPoints = winner.Catan.Explorer.Board.Target - 4
	winner.catanScores()
	ckProgressGive(t, winner, 0, 0, 1, 2, 3, 4)
	explorerCityFlowAct(t, winner, Action{Type: "catan_explorer_begin_move"})
	if !winner.Finished || winner.Phase != "finished" || !slices.Equal(winner.Winners, []int{0}) || winner.Catan.CitiesKnights.Pending != nil || winner.Catan.Explorer.Fleet.Turn != nil {
		t.Fatal("victory started discard or movement")
	}
}

// Reveal/position controlled midgames for mission boundary checks. Natural
// movement to the first fog is tested separately above.
func explorerCityFlowReveal(t *testing.T, s *State) {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	for _, h := range slices.Clone(x.Board.Hidden) {
		if !h.Revealed {
			if _, err := x.discover(g, s.Turn, []int{h.Tile}); err != nil {
				t.Fatal(err)
			}
		}
	}
	explorerCityFlowRestore(t, s)
}

func TestCatanExplorerCityFlowMissionDeliveryVictory(t *testing.T) {
	for _, kind := range []string{"fish", "spice"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerCityFlowFixture(t, 3, "explorers-and-pirates")
			explorerCityFlowRoll(t, s)
			explorerCityFlowReveal(t, s)
			g, x := s.Catan, s.Catan.Explorer
			x.Cargo.Units[0] = catanExplorerCargoLocation{"supply", -1}
			action := Action{Type: "catan_explorer_fish_deliver", Slot: 0, Card: 0}
			if kind == "fish" {
				x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", 0}
			} else {
				farm := x.Board.publicView().Farms[0].Tile
				sack := explorerSpiceClaimFixture(t, s, 0, farm, 2, catanExplorerCargoLocation{"ship", 0})
				action = Action{Type: "catan_explorer_spice_deliver", Slot: 0, Card: sack}
			}
			explorerSpiceDock(t, s, 0, func(e CatanEdge) bool {
				return slices.Contains(x.Board.Council.Anchors, e.A) || slices.Contains(x.Board.Council.Anchors, e.B)
			})
			g.CitiesKnights.Players[0].DefenderPoints = x.Board.Target - 1 - g.Players[0].Score
			s.catanScores()
			explorerCityFlowRestore(t, s)
			explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
			explorerCityFlowAct(t, s, action)
			if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{0}) || s.Catan.TurnSerial != 1 || s.Catan.Players[0].Score < s.Catan.Explorer.Board.Target {
				t.Fatal("mission reward did not stop movement on combined victory")
			}
		})
	}
}

func TestCatanExplorerCityFlowLairHandoffAndImmediateVictory(t *testing.T) {
	for _, win := range []bool{false, true} {
		t.Run(fmt.Sprint(win), func(t *testing.T) {
			s := explorerCityFlowFixture(t, 6, "pirate-lairs")
			explorerCityFlowRoll(t, s)
			explorerCityFlowReveal(t, s)
			x := s.Catan.Explorer
			site := &x.Lairs.Sites[0]
			tile := site.Tile
			site.Ready, site.Captor = 1, 0
			for _, unit := range []int{2, 3, 4} {
				x.Cargo.Units[unit] = catanExplorerCargoLocation{"lair", tile}
			}
			if win {
				// Add both public VP progress cards, Merchant and a real metropolis so
				// each source must be counted before granting the later hero reward.
				ckProgressTop(t, s, 0, 9)
				s.catanDrawProgress(0, 0)
				ckProgressTop(t, s, 2, 23)
				s.catanDrawProgress(0, 2)
				ckProgressGive(t, s, 0, 12)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: explorerMerchantTile(t, s, 0)})
				for level := 1; level <= 4; level++ {
					explorerDevelopmentGrant(t, s, 0, 5, level)
					explorerKnightAct(t, s, Action{Type: "catan_improvement", Color: 0})
				}
				if s.Phase == "catan_metropolis" {
					explorerTradeProgressRespond(t, s, 0, Action{Type: "catan_metropolis", Vertex: s.Catan.cityMetropolisSites(0)[0]})
				}
				if s.Catan.CitiesKnights.Metropolises[0] < 0 {
					t.Fatal("missing metropolis")
				}
				s.Catan.CitiesKnights.Players[0].DefenderPoints = s.Catan.Explorer.Board.Target - 2 - s.Catan.Players[0].Score
				s.catanScores()
			}
			explorerCityFlowRestore(t, s)
			explorerCityFlowAct(t, s, Action{Type: "catan_explorer_begin_move"})
			explorerCityFlowAct(t, s, Action{Type: "catan_end"})
			if s.Phase != "catan_explorer_resolve" || s.Turn != 0 || s.Catan.TurnSerial != 1 {
				t.Fatal("handoff skipped ready lair")
			}
			before := clone(*s.Catan.Explorer.Economy)
			explorerCityFlowAct(t, s, Action{Type: "catan_explorer_resolve", Target: tile})
			x = s.Catan.Explorer
			result := x.Lairs.Sites[x.Lairs.site(tile)]
			if win {
				if !s.Finished || s.Phase != "finished" || x.Lairs.RewardVictory == nil || x.Lairs.Battle != nil || result.Resolved != 0 || result.Hero != -1 || x.Lairs.Progress[0] != 1 || s.Catan.TurnSerial != 1 || s.Catan.Players[0].Score != x.Board.Target || x.Economy.Gold[0] != before.Gold[0]+2 {
					t.Fatal("city bonus ignored or hero/next turn proceeded after victory")
				}
			} else if s.Finished || s.Turn != 3 || s.Catan.TurnSerial != 2 || s.Phase != "catan_turn" || result.Hero != 0 || result.Resolved != 1 || x.Lairs.Progress[0] != 2 {
				t.Fatal("completed mission did not hand off to paired actor")
			}
		})
	}
}
