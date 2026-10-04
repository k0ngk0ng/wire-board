package game

import (
	"reflect"
	"testing"
)

func goldGame(t *testing.T) *State {
	s := seaTestGame(t)
	g := s.Catan
	// Separate single-vertex fields isolate ordinary and gold production amounts.
	g.Tiles = []CatanTile{{ID: 0, Resource: CatanGold, Number: 6, Vertices: []int{0, 1}}, {ID: 1, Resource: 0, Number: 6, Vertices: []int{2}}, {ID: 2, Resource: CatanDesert}}
	g.Vertices = []CatanVertex{{ID: 0, Owner: 1, Level: 1}, {ID: 1, Owner: 2, Level: 2}, {ID: 2, Owner: 0, Level: 1}}
	g.Edges = nil
	g.Ports = nil
	g.Robber = 2
	g.Dice = []int{3, 3}
	return s
}
func TestCatanGoldProductionOrderChoicesAndRestore(t *testing.T) {
	s := goldGame(t)
	s.catanRoll(6)
	g := s.Catan
	if s.Phase != "catan_gold" || s.Turn != 0 || g.Players[0].Resources[0] != 1 || s.CatanPendingActor() != 1 {
		t.Fatal("production was not distributed before gold")
	}
	if !reflect.DeepEqual(g.GoldPending.Claims, []CatanGoldClaim{{Player: 1, Count: 1}, {Player: 2, Count: 2}}) {
		t.Fatal("gold city or clockwise order incorrect")
	}
	helperReject(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
	helperReject(t, s, 1, Action{Type: "catan_end"})
	for _, take := range [][]int{{1, 1, 0, 0, 0}, {0, 0, 0, 0, 0}, {-1, 2, 0, 0, 0}, {1, 0}} {
		helperReject(t, s, 1, Action{Type: "catan_gold", Take: take})
	}
	if err := s.EliminateCatan(0); err == nil {
		t.Fatal("active player removed during someone else's gold choice")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 1, Action{Type: "catan_gold", Take: []int{0, 1, 0, 0, 0}})
	if s.CatanPendingActor() != 2 || s.Turn != 0 {
		t.Fatal("lost second claimant or turn owner")
	}
	helperApply(t, s, 2, Action{Type: "catan_gold", Take: []int{0, 0, 1, 0, 1}})
	if s.Phase != "catan_turn" || s.Catan.GoldPending != nil || s.CatanPendingActor() != -1 {
		t.Fatal("gold resolution did not resume turn")
	}
	if s.Catan.Players[2].Resources[2] != 1 || s.Catan.Players[2].Resources[4] != 1 {
		t.Fatal("city could not split its resources")
	}
	catanCheck(t, s)
}
func TestCatanGoldStartsFromActivePlayerAndCanChooseSameResource(t *testing.T) {
	s := goldGame(t)
	s.Turn = 2
	s.catanRoll(6)
	if s.CatanPendingActor() != 2 {
		t.Fatal("gold queue ignores active seat")
	}
	helperApply(t, s, 2, Action{Type: "catan_gold", Take: []int{0, 0, 0, 2, 0}})
	if s.Catan.Players[2].Resources[3] != 2 || s.CatanPendingActor() != 1 {
		t.Fatal("same-color selection or clockwise wrap incorrect")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || s.Turn != 2 {
		t.Fatal("timeout changed production turn")
	}
}
func TestCatanGoldRobberAndEliminationPreventProduction(t *testing.T) {
	for _, robbed := range []bool{true, false} {
		s := goldGame(t)
		if robbed {
			s.Catan.Robber = 0
		} else {
			s.Catan.Players[1].Eliminated = true
			s.Catan.Players[2].Eliminated = true
		}
		s.catanRoll(6)
		if s.Catan.GoldPending != nil || s.Phase != "catan_turn" || sum(s.Catan.Players[1].Resources) != 0 || sum(s.Catan.Players[2].Resources) != 0 {
			t.Fatal("blocked gold produced resources")
		}
	}
}
func TestCatanGoldBankShortageAndNoSupply(t *testing.T) {
	s := goldGame(t)
	for color := 0; color < 5; color++ {
		n := s.Catan.Bank[color]
		if color == 4 {
			n -= 1
		}
		helperGrant(s, 0, func() []int { v := make([]int, 5); v[color] = n; return v }())
	}
	s.Turn = 2
	s.catanRoll(6)
	helperReject(t, s, 2, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
	helperApply(t, s, 2, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, 1}})
	if s.Catan.GoldPending != nil || s.Phase != "catan_turn" || s.Catan.Players[2].Resources[4] != 1 || sum(s.Catan.Players[1].Resources) != 0 {
		t.Fatal("empty bank left gold queue stuck")
	}
	catanCheck(t, s)
}
func TestCatanGoldHildaWaitsForActualProduction(t *testing.T) {
	for _, holder := range []int{0, 1} {
		s := goldGame(t)
		g := s.Catan
		g.Options.Helpers = true
		g.TurnSerial = 2
		g.Players[holder].Helper = &CatanHelperSeat{ID: 3}
		g.HelperDisplay = []int{1, 2, 4}
		// Player zero receives no ordinary resource in this fixture.
		g.Tiles[1].Number = 5
		s.catanRoll(6)
		if s.Phase != "catan_gold" || s.Catan.HelperPending != nil {
			t.Fatal("Hilda evaluated before gold")
		}
		for s.Catan.GoldPending != nil {
			s.AutoCatanPending()
		}
		if holder == 0 {
			if s.Phase != "catan_helper" || s.Catan.HelperPending.Player != 0 {
				t.Fatal("unpaid player lost Hilda compensation")
			}
			s.AutoCatanPending()
			if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 1 {
				t.Fatal("Hilda did not resume production turn")
			}
		} else if s.Phase != "catan_turn" || s.Catan.HelperPending != nil {
			t.Fatal("gold recipient incorrectly received Hilda compensation")
		}
	}
}
func TestCatanGoldSecondSettlementChoiceBeforeStartingRoute(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	g.Tiles[0].Resource = CatanGold
	g.Robber = 3
	g.SetupStep = len(g.Players)
	s.Phase = "catan_setup_settlement"
	vertex := g.Tiles[0].Vertices[0]
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: vertex})
	if s.Phase != "catan_gold" || s.Catan.GoldPending.Resume != "catan_setup_road" {
		t.Fatal("second village gold missing")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, 1}})
	if s.Phase != "catan_setup_road" || s.Catan.SetupStep != 3 || s.Catan.SetupVertex != vertex {
		t.Fatal("setup gold skipped starting route")
	}
	s.AutoCatanPending()
	if s.Catan.SetupStep != 4 || s.Phase != "catan_setup_settlement" {
		t.Fatal("setup timeout cannot continue after gold")
	}
	catanCheck(t, s)
}
func TestCatanGoldBotsUseOnlyTheirOwnHand(t *testing.T) {
	s := goldGame(t)
	s.catanRoll(6)
	before := clone(*s)
	a, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BotAction(0); err == nil {
		t.Fatal("active seat chose opponent's gold")
	}
	changed := clone(*s)
	changed.Catan.Players[2].Resources = []int{0, 0, 19, 0, 0}
	other, err := changed.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, other) {
		t.Fatal("gold bot inspected opponent's hand")
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("gold bot mutated live state")
	}
	helperApply(t, s, 1, a)
}

func TestCatanSeafarersFixedMapsBotsCompleteWithGoldAndHelpers(t *testing.T) {
	fogGames, fogDiscoveries := 0, 0
	desertGames, desertRewards := 0, 0
	tribeGames, tribeRewards := 0, 0
	clothGames, clothCollected := 0, 0
	for _, scenario := range []struct {
		name             string
		players, victory int
		build            func(*Catan) error
	}{
		{"shores3", 3, 14, (*Catan).makeSeafarersShoresThree},
		{"shores4", 4, 14, (*Catan).makeSeafarersShoresFour},
		{"islands3", 3, 13, (*Catan).makeSeafarersIslandsThree},
		{"islands4", 4, 13, (*Catan).makeSeafarersIslandsFour},
		{"shores5", 5, 14, (*Catan).makeSeafarersShoresSix},
		{"shores6", 6, 14, (*Catan).makeSeafarersShoresSix},
		{"islands5", 5, 13, (*Catan).makeSeafarersIslandsSix},
		{"islands6", 6, 13, (*Catan).makeSeafarersIslandsSix},
		{"fog3", 3, 12, (*Catan).makeSeafarersFogThree},
		{"fog4", 4, 12, (*Catan).makeSeafarersFogFour},
		{"fog5", 5, 12, (*Catan).makeSeafarersFogSix},
		{"fog6", 6, 12, (*Catan).makeSeafarersFogSix},
		{"desert3", 3, 14, (*Catan).makeSeafarersDesertThree},
		{"desert4", 4, 14, (*Catan).makeSeafarersDesertFour},
		{"desert5", 5, 14, (*Catan).makeSeafarersDesertSix},
		{"desert6", 6, 14, (*Catan).makeSeafarersDesertSix},
		{"tribe3", 3, 13, (*Catan).makeSeafarersTribeFour},
		{"tribe4", 4, 13, (*Catan).makeSeafarersTribeFour},
		{"tribe5", 5, 13, (*Catan).makeSeafarersTribeSix},
		{"tribe6", 6, 13, (*Catan).makeSeafarersTribeSix},
		{"cloth3", 3, 14, (*Catan).makeSeafarersClothFour},
		{"cloth4", 4, 14, (*Catan).makeSeafarersClothFour},
		{"cloth5", 5, 14, (*Catan).makeSeafarersClothSix},
		{"cloth6", 6, 14, (*Catan).makeSeafarersClothSix},
	} {
		for _, variable := range []bool{false, true} {
			if variable && scenario.players > 4 {
				continue
			}
			for _, helpers := range []bool{false, true} {
				t.Run(scenario.name+"/"+map[bool]string{false: "fixed", true: "variable"}[variable]+"/"+map[bool]string{false: "standard", true: "helpers"}[helpers], func(t *testing.T) {
					s, err := NewCatan(scenario.players, CatanOptions{Helpers: helpers, AllHelpers: helpers, FiveSix: scenario.players > 4})
					if err != nil {
						t.Fatal(err)
					}
					if err = scenario.build(s.Catan); err != nil {
						t.Fatal(err)
					}
					if variable {
						if err = s.randomizeCatanSeafarersMap(); err != nil {
							t.Fatal(err)
						}
					}
					resourceTotal, developmentTotal := 19, 25
					if scenario.players > 4 {
						resourceTotal, developmentTotal = 24, 34
					}
					var fogTerrain, fogNumbers []int
					initialFog := 0
					if s.Catan.Seafarers.Fog != nil {
						fogTerrain, fogNumbers = fogInventory(s.Catan)
						initialFog = len(s.Catan.Seafarers.Fog.Terrain)
					}
					for steps := 0; !s.Finished && steps < 8000; steps++ {
						player := s.Turn
						if actor := s.CatanPendingActor(); actor >= 0 {
							player = actor
						} else if s.Phase == "catan_discard" {
							for i, due := range s.Catan.DiscardDue {
								if due > 0 {
									player = i
									break
								}
							}
						}
						a, err := s.BotAction(player)
						if err != nil {
							t.Fatal(steps, s.Phase, player, err)
						}
						helperApply(t, s, player, a)
						g := s.Catan
						if g.cloth() != nil {
							assertClothInventory(t, g)
						}
						if g.Seafarers.Fog != nil {
							terrain, numbers := fogInventory(g)
							if !reflect.DeepEqual(terrain, fogTerrain) || !reflect.DeepEqual(numbers, fogNumbers) {
								t.Fatal("exploration components not conserved")
							}
						}
						for color, bank := range g.Bank {
							total := bank
							if bank < 0 {
								t.Fatal("negative bank")
							}
							for _, p := range g.Players {
								if p.Resources[color] < 0 {
									t.Fatal("negative hand")
								}
								total += p.Resources[color]
							}
							if total != resourceTotal {
								t.Fatal("resource conservation", color, total)
							}
						}
						dev := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
						if tribe := g.tribe(); tribe != nil {
							dev += len(tribe.Development)
							assertTribeInventory(t, g)
						}
						if g.HelperPending != nil {
							dev += len(g.HelperPending.Cards)
						}
						for i, p := range g.Players {
							dev += sum(p.Dev)
							roads, villages, cities := g.pieces(i)
							if roads > 15 || g.shipCount(i) > 15 || villages > 5 || cities > 4 {
								t.Fatal("piece supply exceeded")
							}
						}
						if dev != developmentTotal {
							t.Fatal("development deck conservation", dev)
						}
						if steps%37 == 0 {
							restored := clone(*s)
							s = &restored
						}
					}
					if s.Catan.Seafarers.Fog != nil {
						discovered := initialFog - len(s.Catan.Seafarers.Fog.Terrain)
						fogGames++
						fogDiscoveries += discovered
						t.Logf("finished after revealing %d of %d fog hexes", discovered, initialFog)
					}
					if s.Catan.Seafarers.Scenario == "desert" {
						desertGames++
						rewards := 0
						for _, seat := range s.Catan.Seafarers.Seats {
							rewards += seat.IslandPoints
						}
						desertRewards += rewards
						t.Logf("finished with %d exploration points", rewards)
					}
					if tribe := s.Catan.tribe(); tribe != nil {
						tribeGames++
						tribeRewards += sum(tribe.Points)
						t.Logf("finished with %d tribe points, %d unclaimed development cards, %d placed ports", sum(tribe.Points), len(tribe.Development), len(s.Catan.Ports))
					}
					if c := s.Catan.cloth(); c != nil {
						clothGames++
						clothCollected += sum(c.Held)
						t.Logf("finished with %d cloth distributed, %d common stock", sum(c.Held), c.Stock)
						assertClothFinished(t, s)
					} else if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < scenario.victory {
						t.Fatal("scenario bots did not finish", s.Round, s.Phase)
					}
				})
			}
		}
	}
	if clothGames > 0 && clothCollected == 0 {
		t.Fatal("cloth simulations never traded for cloth")
	}
	if desertGames > 0 && desertRewards == 0 {
		t.Fatal("desert simulations never exercised exploration rewards")
	}
	if tribeGames > 0 && tribeRewards == 0 {
		t.Fatal("tribe simulations never collected rewards")
	}
	if fogGames > 0 && fogDiscoveries == 0 {
		t.Fatal("fog simulations never exercised discovery")
	}
}

func TestCatanGoldRobberHelpersUseChosenResource(t *testing.T) {
	for _, id := range []int{10, 11} {
		s := helperTestGame(t, id)
		g := s.Catan
		g.Seafarers = &CatanSeafarers{Pirate: -1}
		tile := -1
		for _, v := range g.Tiles {
			if v.Resource != CatanDesert {
				tile = v.ID
				break
			}
		}
		g.Tiles[tile].Resource = CatanGold
		g.Robber = tile
		helperReject(t, s, 0, Action{Type: "catan_helper", Color: 7})
		before := g.Players[0].Resources[3]
		helperApply(t, s, 0, Action{Type: "catan_helper", Color: 3})
		g = s.Catan
		if g.Players[0].Resources[3] != before+1 || s.Phase != "catan_helper" {
			t.Fatal("gold helper did not grant chosen resource")
		}
		if id == 10 && g.Tiles[g.Robber].Resource != CatanDesert {
			t.Fatal("Digur failed to move robber")
		}
		if id == 11 && g.Robber != tile {
			t.Fatal("Kaja moved the robber")
		}
	}
}
func TestCatanGoldDigurBotAvoidsEmptyStockAndMissingDesert(t *testing.T) {
	s := helperTestGame(t, 10)
	g := s.Catan
	tile := -1
	for _, v := range g.Tiles {
		if v.Resource != CatanDesert {
			tile = v.ID
			break
		}
	}
	g.Tiles[tile].Resource = CatanGold
	g.Robber = tile
	g.Bank[0] = 0
	s.Phase = "catan_roll"
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_helper" || a.Color == 0 {
		t.Fatal("Digur chose an empty resource", a, err)
	}
	for i := range g.Tiles {
		if g.Tiles[i].Resource == CatanDesert {
			g.Tiles[i].Resource = 0
		}
	}
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_roll" {
		t.Fatal("Digur bot stuck without desert", a, err)
	}
}
