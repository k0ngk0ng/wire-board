package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Legal completed setup, with an explicitly controlled post-invasion midgame.
// A separate test below actually resolves the first invasion before playing.
func explorerTaxationFixture(t *testing.T, n int, secondary bool) *State {
	t.Helper()
	s := explorerTradeProgressFixture(t, n, secondary)
	s.Catan.CitiesKnights.Invasions = 1
	ckProgressGive(t, s, s.Turn, 21)
	explorerCityRestore(t, s)
	return s
}

// Move the victim's already-loaded ship to a legal offshore edge. A second
// empty ship on the same edge checks owner deduplication, not extra thefts.
func explorerTaxationVictim(t *testing.T, s *State, victim int, duplicate bool) int {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	for _, tile := range x.Pirate.destinations(g, x.Board) {
		for _, edge := range g.Edges {
			if !catanExplorerSeaEdge(g, edge.ID) || !slices.Contains(edge.Tiles, tile) || slices.Contains(x.Fleet.Positions, edge.ID) {
				continue
			}
			x.Fleet.Positions[victim*3] = edge.ID
			if duplicate {
				x.Fleet.Positions[victim*3+1] = edge.ID
			}
			explorerCityRestore(t, s)
			return tile
		}
	}
	t.Fatal("no offshore ship fixture")
	return -1
}

func TestCatanExplorerCityTaxationResumeAndRejection(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, false}, {6, true}} {
		t.Run(fmt.Sprintf("%d/%t", config.n, config.secondary), func(t *testing.T) {
			s := explorerTaxationFixture(t, config.n, config.secondary)
			p := s.Turn
			// Exercise real bank/card actions before Taxation, rather than just seeding counters.
			ckProgressGive(t, s, p, 13, 10)
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 13, Color: 5})
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 10})
			explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: -1, Target: 0})
			before := clone(*s.Catan)
			play := Action{Type: "catan_progress", Card: 21, Prompt: int(s.Catan.TurnSerial)}
			explorerCityActionReject(t, s, (p+1)%config.n, play)
			stale := play
			stale.Prompt = 0
			explorerCityActionReject(t, s, p, stale)
			s.Catan.CitiesKnights.Invasions = 0
			explorerCityActionReject(t, s, p, play)
			s.Catan.CitiesKnights.Invasions = 1
			explorerKnightAct(t, s, play)
			q := s.Catan.Explorer.Pirate.Pending
			if s.Phase != "catan_explorer_pirate_place" || q.Source != "taxation" || q.Resume != "city_action" || slices.Contains(s.Catan.CitiesKnights.Players[p].Progress, 21) {
				t.Fatal("Taxation did not consume card and suspend action")
			}
			for _, a := range []Action{{Type: "catan_end"}, {Type: "catan_progress", Card: 21}, {Type: "catan_explorer_pirate_place", Target: -1}, {Type: "catan_explorer_pirate_steal", Target: (p + 1) % config.n}} {
				a.Prompt = int(s.Catan.TurnSerial)
				explorerCityActionReject(t, s, p, a)
			}
			tile := s.Catan.Explorer.Pirate.destinations(s.Catan, s.Catan.Explorer.Board)[0]
			explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: tile})
			g := s.Catan
			if g.Explorer.Pirate.Pending != nil || s.Phase != "catan_turn" || s.Turn != p || g.Robber != -1 || g.TurnSerial != before.TurnSerial || g.RollID != before.RollID || !slices.Equal(g.Dice, before.Dice) || !reflect.DeepEqual(g.Explorer.Economy, before.Explorer.Economy) || !reflect.DeepEqual(g.Explorer.Cargo, before.Explorer.Cargo) || !reflect.DeepEqual(g.Explorer.Fleet, before.Explorer.Fleet) || !reflect.DeepEqual(g.CitiesKnights.TradePowers, before.CitiesKnights.TradePowers) {
				t.Fatal("Taxation reset action, ship cargo, dice or trade allowance")
			}
			explorerCityActionReject(t, s, p, play) // No refund/reuse after resolution.
		})
	}
}

func TestCatanExplorerCityTaxationTheftAndPrivacy(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for color := range 8 {
			t.Run(fmt.Sprintf("%d/%d", config.n, color), func(t *testing.T) {
				s := explorerTaxationFixture(t, config.n, config.secondary)
				p, victim := s.Turn, (s.Turn+1)%config.n
				tile := explorerTaxationVictim(t, s, victim, true)
				for r := range 8 {
					explorerDevelopmentGrant(t, s, victim, r, 1)
				}
				ckProgressGive(t, s, victim, 3)
				before := clone(*s.Catan)
				logStart := len(s.Log)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 21})
				explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: tile})
				if got := s.Catan.Explorer.Pirate.victims(s.Catan, s.Catan.Explorer.Fleet, s.Catan.Explorer.Economy); !slices.Equal(got, []int{victim}) {
					t.Fatal("ships duplicated victim", got)
				}
				a := Action{Type: "catan_explorer_pirate_steal", Target: victim, Prompt: int(s.Catan.TurnSerial)}
				for _, random := range []func(int) int{nil, func(int) int { return -1 }, func(n int) int { return n }} {
					saved := clone(*s)
					if err := s.catanExplorerCityPirateAction(p, a, random); err == nil || !reflect.DeepEqual(*s, saved) {
						t.Fatal("invalid random theft was not atomic")
					}
				}
				skip := a
				skip.Choice = "skip"
				explorerCityActionReject(t, s, p, skip)
				wrong := a
				wrong.Target = p
				explorerCityActionReject(t, s, p, wrong)
				explorerCityActionReject(t, s, victim, a)
				if err := s.catanExplorerCityPirateAction(p, a, func(n int) int {
					if n != 8 {
						t.Fatal("progress or gold included in random pool")
					}
					return color
				}); err != nil {
					t.Fatal(err)
				}
				explorerCityRestore(t, s)
				g := s.Catan
				if g.Players[p].Resources[color] != 1 || g.Players[victim].Resources[color] != 0 || sum(g.Players[victim].Resources) != 7 || !slices.Equal(g.Bank, before.Bank) || !reflect.DeepEqual(g.CitiesKnights.Players[victim].Progress, before.CitiesKnights.Players[victim].Progress) || !reflect.DeepEqual(g.Explorer.Economy, before.Explorer.Economy) || !reflect.DeepEqual(g.Explorer.Cargo, before.Explorer.Cargo) || s.Phase != "catan_turn" {
					t.Fatal("wrong theft quantity, cards or resumed action")
				}
				for viewer := -1; viewer < config.n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					for seat, raw := range v["players"].([]any) {
						if _, ok := raw.(map[string]any)["resources"]; ok != (viewer == seat) {
							t.Fatal("theft exposed another hand")
						}
					}
				}
				for _, name := range []string{"木材", "砖块", "羊毛", "粮食", "矿石", "纸张", "布匹", "钱币"} {
					if strings.Contains(strings.Join(s.Log[logStart:], " "), name) {
						t.Fatal("public theft log disclosed resource", name)
					}
				}
			})
		}
	}
}

func TestCatanExplorerCityTaxationGoldChoice(t *testing.T) {
	for _, decline := range []bool{false, true} {
		s := explorerTaxationFixture(t, 3, false)
		tile := explorerTaxationVictim(t, s, 1, false)
		before := clone(*s.Catan.Explorer.Economy)
		explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 21})
		explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: tile})
		a := Action{Type: "catan_explorer_pirate_steal", Target: 1}
		if decline {
			a.Choice = "skip"
		}
		explorerKnightAct(t, s, a)
		want := clone(before)
		if !decline {
			want.Gold[0]++
			want.Gold[1]--
		}
		if !reflect.DeepEqual(*s.Catan.Explorer.Economy, want) || s.Phase != "catan_turn" {
			t.Fatal("wrong optional gold theft")
		}
	}
	s := explorerTaxationFixture(t, 3, false)
	tile := explorerTaxationVictim(t, s, 1, false)
	e := s.Catan.Explorer.Economy
	e.GoldBank += e.Gold[1]
	e.Gold[1] = 0
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 21})
	explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: tile})
	if s.Phase != "catan_turn" {
		t.Fatal("penniless victim stalled pirate")
	}
}

func TestCatanExplorerCityTaxationFirstInvasion(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	s.Catan.CitiesKnights.BarbarianPosition = 6 // Next real event triggers attack.
	ckProgressGive(t, s, 0, 21)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_pillage" {
		p := s.CatanPendingActor()
		explorerTradeProgressRespond(t, s, p, Action{Type: "catan_pillage", Vertex: s.Catan.pillageSites(p)[0]})
	}
	if s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("first invasion not resolved")
	}
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 21})
	explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: s.Catan.Explorer.Pirate.destinations(s.Catan, s.Catan.Explorer.Board)[0]})
	if s.Phase != "catan_turn" || s.Catan.Robber != -1 {
		t.Fatal("post-invasion Taxation activated base robber")
	}
}

func TestCatanExplorerCityTaxationCorruptPending(t *testing.T) {
	s := explorerTaxationFixture(t, 3, false)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 21})
	for name, damage := range map[string]func(*State){
		"source":       func(s *State) { s.Catan.Explorer.Pirate.Pending.Source = "seven" },
		"resume":       func(s *State) { s.Catan.Explorer.Pirate.Pending.Resume = "action" },
		"owner":        func(s *State) { s.Catan.Explorer.Pirate.Pending.Player = 1 },
		"sequence":     func(s *State) { s.Catan.Explorer.Pirate.Pending.Sequence++ },
		"stage":        func(s *State) { s.Catan.Explorer.Pirate.Pending.Stage = "steal" },
		"invasion":     func(s *State) { s.Catan.CitiesKnights.Invasions = 0 },
		"cargo":        func(s *State) { s.Catan.Explorer.Cargo.Turn.Phase = "movement" },
		"city-pending": func(s *State) { s.Catan.CitiesKnights.Pending = &CatanCityPending{Kind: "aqueduct", Players: []int{0}} },
		"event":        func(s *State) { s.Catan.CitiesKnights.Event = &CatanCityEvent{} },
		"free-roads":   func(s *State) { s.Catan.FreeRoads = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			damage(&bad)
			if bad.validateExplorerCityProduction() == nil {
				t.Fatal("invalid save accepted")
			}
			explorerCityActionReject(t, &bad, 0, Action{Type: "catan_explorer_pirate_place", Target: s.Catan.Explorer.Pirate.destinations(s.Catan, s.Catan.Explorer.Board)[0], Prompt: 1})
		})
	}
}

func TestCatanExplorerCitySevenPrivateDispatcher(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	for p := range s.Catan.Players {
		for r, n := range s.Catan.Players[p].Resources {
			s.Catan.Bank[r] += n
			s.Catan.Players[p].Resources[r] = 0
		}
	}
	for _, p := range []int{0, 2} {
		explorerDevelopmentGrant(t, s, p, 0, 5)
		explorerDevelopmentGrant(t, s, p, 5, 3)
	}
	if err := s.catanExplorerCityRoll(3, 4, 3); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Catan.DiscardDue, []int{4, 0, 4}) {
		t.Fatal("incorrect independent discard obligations")
	}
	cards := []int{2, 0, 0, 0, 0, 2, 0, 0}
	discard := Action{Type: "catan_discard", Tokens: cards, Prompt: 1}
	explorerCityActionReject(t, s, 1, discard)
	invalid := discard
	invalid.Prompt = 0
	explorerCityActionReject(t, s, 2, invalid)
	invalid = discard
	invalid.Tokens = []int{4, 0, 0, 0, 0}
	explorerCityActionReject(t, s, 2, invalid)
	if err := s.catanExplorerCityAction(2, discard); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
	if s.Phase != "catan_discard" || s.Catan.Explorer.Cargo.Turn != nil {
		t.Fatal("action started before everyone discarded")
	}
	explorerCityActionReject(t, s, 2, discard)
	if err := s.catanExplorerCityAction(0, discard); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
	if s.Phase != "catan_explorer_pirate_place" || s.Catan.Explorer.Pirate.Pending.Resume != "action" {
		t.Fatal("last discard did not start seven pirate")
	}
	tile := s.Catan.Explorer.Pirate.destinations(s.Catan, s.Catan.Explorer.Board)[0]
	explorerKnightAct(t, s, Action{Type: "catan_explorer_pirate_place", Target: tile})
	if s.Phase != "catan_turn" || s.Catan.Explorer.Cargo.Turn == nil || s.Catan.Explorer.Cargo.Turn.Phase != "action" || s.Catan.Explorer.Economy.Turn.Bought != 0 {
		t.Fatal("seven failed to begin action")
	}
	explorerCityActionReject(t, s, 0, discard)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_pirate_place", Target: tile, Prompt: 1})
}
