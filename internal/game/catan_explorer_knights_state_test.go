package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func explorerCityStateRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanExplorer(); err != nil {
		t.Fatal(s.Phase, err)
	}
	after, _ := json.Marshal(&next)
	if string(raw) != string(after) {
		t.Fatal("aggregate restore changed snapshot")
	}
	*s = next
}
func explorerCityStateAct(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if x := s.Catan.Explorer; x.Setup != nil {
		a.Prompt = x.Setup.Step + 1
	}
	oldID := s.Catan.Explorer.ActionID
	if err := s.Apply(p, a); err != nil {
		t.Fatal(s.Phase, p, a, err)
	}
	if s.Catan.Explorer.ActionID != oldID+1 {
		t.Fatal("aggregate action did not publish one new snapshot")
	}
	explorerCityStateRestore(t, s)
}
func explorerCityStateReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before := clone(*s)
	if err := s.Apply(p, a); err == nil {
		t.Fatal("invalid aggregate action accepted", p, a)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("rejected aggregate action changed snapshot")
	}
}
func explorerCityStateStarted(t *testing.T, n int, scenario string) *State {
	t.Helper()
	// Explicit synthetic tokens for internal acceptance, not a release recipe.
	var numbers []int
	if catanExplorerMissionScenario(scenario) {
		numbers = []int{3, 4, 5, 9, 10, 11}
		if n > 4 {
			numbers = append(numbers, 3, 4)
		}
	}
	s, err := newCatanExplorerCityState(n, scenario, numbers)
	if err != nil {
		t.Fatal(err)
	}
	x := s.Catan.Explorer
	q := explorerSetupFixture{s.Catan, x.Board, x.Fleet, x.Cargo, x.Economy, x.Setup}
	path := explorerKnightsCompletableOpening(t, q, rand.New(rand.NewSource(307)))
	for _, target := range path {
		step := s.Catan.Explorer.Setup.current(n)
		a := Action{Type: "catan_explorer_setup", Choice: step.Kind, Target: target, Prompt: s.Catan.Explorer.Setup.Step + 1}
		explorerCityStateReject(t, s, (step.Player+1)%n, a)
		bad := a
		bad.Prompt--
		explorerCityStateReject(t, s, step.Player, bad)
		explorerCityStateAct(t, s, step.Player, a)
	}
	if s.Catan.Explorer.Setup != nil || s.Phase != "catan_roll" || s.Turn != s.Catan.StartPlayer || s.Catan.RollID != 0 {
		t.Fatal("aggregate setup did not end at production")
	}
	if !strings.Contains(strings.Join(s.Log, " "), "起始城市") {
		t.Fatal("city-first opening log lost building name")
	}
	return s
}

// Real server dice may request several independent replies; every response
// here uses State.Apply, including off-turn players and seven discards.
func explorerCityStateReady(t *testing.T, s *State) {
	t.Helper()
	for steps := 0; steps < 100; steps++ {
		if s.Phase == "catan_turn" {
			return
		}
		p := s.Turn
		a := Action{}
		g, x := s.Catan, s.Catan.Explorer
		switch s.Phase {
		case "catan_roll":
			a.Type = "catan_roll"
		case "catan_discard":
			for seat, due := range g.DiscardDue {
				if due > 0 {
					p = seat
					break
				}
			}
			a.Type = "catan_discard"
			a.Tokens = make([]int, 8)
			left := g.DiscardDue[p]
			for r, count := range g.Players[p].Resources {
				a.Tokens[r] = min(count, left)
				left -= a.Tokens[r]
			}
		case "catan_explorer_pirate_place":
			a = Action{Type: s.Phase, Target: x.Pirate.destinations(g, x.Board)[0]}
		case "catan_explorer_pirate_steal":
			a = Action{Type: s.Phase, Target: x.Pirate.victims(g, x.Fleet, x.Economy)[0]}
		case "catan_pillage":
			p = g.CitiesKnights.Pending.Players[0]
			a = Action{Type: s.Phase, Vertex: g.pillageSites(p)[0]}
		case "catan_defender_reward":
			p = g.CitiesKnights.Pending.Players[0]
			a.Type = s.Phase
			for track, deck := range g.CitiesKnights.ProgressDecks {
				if len(deck) > 0 {
					a.Color = track
					break
				}
			}
		case "catan_progress_discard":
			p = g.CitiesKnights.Pending.Players[0]
			hand := g.CitiesKnights.Players[p].Progress
			a = Action{Type: s.Phase, Cards: slices.Clone(hand[:len(hand)-4])}
		case "catan_aqueduct":
			a.Type = s.Phase
			p = g.CitiesKnights.Pending.Players[0]
			for r, count := range g.Bank[:5] {
				if count > 0 {
					a.Color = r
					break
				}
			}
		default:
			t.Fatal("unexpected pre-action phase", s.Phase)
		}
		explorerCityStateAct(t, s, p, a)
	}
	t.Fatal("production response did not reach action")
}

func TestCatanExplorerCityAggregateSetupAndUnifiedTurns(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := explorerCityStateStarted(t, n, scenario)
				for portion := 0; portion < 4; portion++ {
					explorerCityStateReady(t, s)
					p, serial := s.Turn, s.Catan.TurnSerial
					explorerCityStateReject(t, s, -1, Action{Type: "catan_explorer_begin_move", Prompt: int(serial)})
					explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
					sail := explorerCityFlowChoice(t, s, "catan_explorer_sail")
					start := s.Catan.Explorer.Fleet.Positions[sail.Slot]
					explorerCityStateAct(t, s, p, sail)
					m := s.Catan.Explorer.Motion
					if m == nil || m.Ship != sail.Slot || m.Path[0] != start || m.Path[len(m.Path)-1] != sail.Targets[len(sail.Targets)-1] {
						t.Fatal("unified movement lost public animation")
					}
					explorerCityStateAct(t, s, p, Action{Type: "catan_end"})
					if s.Catan.TurnSerial != serial+1 {
						t.Fatal("unified end did not advance exactly once")
					}
				}
			})
		}
	}
}

func TestCatanExplorerCityAggregateProductionReplies(t *testing.T) {
	for _, kind := range []string{"pillage", "progress", "seven"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerCityStateStarted(t, 3, "pirate-lairs")
			p, other := s.Turn, (s.Turn+1)%3
			switch kind {
			case "pillage":
				s.Catan.CitiesKnights.BarbarianPosition = 6 // Controlled next attack.
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
				if s.Phase != "catan_pillage" {
					t.Fatal("no pillage response")
				}
			case "progress":
				s.Catan.CitiesKnights.Players[other].Improvements[0] = 1
				ckProgressGive(t, s, other, 1, 2, 3, 4)
				ckProgressTop(t, s, 0, 5)
				if err := s.catanExplorerCityRoll(1, 1, 0); err != nil {
					t.Fatal(err)
				}
				if s.Phase != "catan_progress_discard" {
					t.Fatal("no off-turn progress response")
				}
			case "seven":
				explorerDevelopmentGrant(t, s, other, 5, 8)
				if err := s.catanExplorerCityRoll(3, 4, 3); err != nil {
					t.Fatal(err)
				}
				if s.Phase != "catan_discard" {
					t.Fatal("no seven response")
				}
			}
			explorerCityStateRestore(t, s)
			explorerCityStateReject(t, s, p, Action{Type: "catan_explorer_begin_move", Prompt: int(s.Catan.TurnSerial)})
			explorerCityStateReady(t, s)
			if s.Turn != p || s.Catan.RollID != 1 {
				t.Fatal("reply duplicated production or skipped actor")
			}
		})
	}
}

func TestCatanExplorerCityAggregateProgressResponsesAndOverflow(t *testing.T) {
	s := explorerCityStateStarted(t, 6, "explorers-and-pirates")
	explorerCityStateReady(t, s)
	p, other := s.Turn, (s.Turn+1)%6
	ckProgressGive(t, s, p, 10, 0, 1, 2, 3, 4, 5)
	explorerDevelopmentGrant(t, s, p, 0, 1)
	explorerDevelopmentGrant(t, s, other, 5, 1)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
	explorerCityStateAct(t, s, p, Action{Type: "catan_commercial_offer", Target: other, Color: 0})
	for viewer := -1; viewer < 6; viewer++ {
		pending := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)["pending"].(map[string]any)
		if _, ok := pending["color"]; ok != (viewer == p || viewer == other) {
			t.Fatal("unified offer disclosed resource")
		}
	}
	explorerCityStateReject(t, s, p, Action{Type: "catan_commercial_harbor", Color: 5, Prompt: int(s.Catan.TurnSerial)})
	explorerCityStateAct(t, s, other, Action{Type: "catan_commercial_harbor", Color: 5})
	explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	if s.Phase != "catan_progress_end" {
		t.Fatal("overflow omitted from unified action")
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress_discard", Cards: []int{0, 1}})
	if s.Phase != "catan_explorer_move" || s.Turn != p {
		t.Fatal("overflow used base end-turn")
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_end"})
	if s.Turn != (p+3)%6 || s.Phase != "catan_turn" {
		t.Fatal("paired player lost action")
	}
}

func TestCatanExplorerCityAggregateCorruptSaveAtomic(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	explorerCityStateReady(t, s)
	p := s.Turn
	for name, damage := range map[string]func(*State){
		"players":      func(s *State) { s.Catan.CitiesKnights.Players = nil },
		"board":        func(s *State) { s.Catan.Explorer.Board.CitiesKnights = false },
		"missing-card": func(s *State) { s.Catan.CitiesKnights.ProgressDecks[0] = s.Catan.CitiesKnights.ProgressDecks[0][1:] },
		"bad-card":     func(s *State) { s.Catan.CitiesKnights.ProgressDecks[0][0] = 99 },
		"wrong-track":  func(s *State) { s.Catan.CitiesKnights.ProgressDecks[0][0] = 10 },
		"private-vp":   func(s *State) { ckProgressGive(t, s, p, 9) },
		"public-non-vp": func(s *State) {
			ckProgressGive(t, s, p, 0)
			k := s.Catan.CitiesKnights
			k.Players[p].Progress = nil
			k.Players[p].PublicProgress = []int{0}
			k.Players[p].ProgressPoints = 1
		},
		"vp-points":     func(s *State) { s.Catan.CitiesKnights.Players[p].ProgressPoints = 1 },
		"defense":       func(s *State) { s.Catan.CitiesKnights.Players[p].DefenderPoints = -1 },
		"event-die":     func(s *State) { s.Catan.CitiesKnights.EventDie = 6 },
		"barbarian":     func(s *State) { s.Catan.CitiesKnights.BarbarianPosition = 7 },
		"round":         func(s *State) { s.Round++ },
		"roll-count":    func(s *State) { s.Catan.RollID++ },
		"skip":          func(s *State) { s.Catan.Explorer.SkippedRolls = 1 },
		"winner":        func(s *State) { s.Winners = []int{p} },
		"event-privacy": func(s *State) { id := 0; s.Catan.CitiesKnights.recordProgress("draw", p, -1, 0, 1, &id) },
		"event-order": func(s *State) {
			s.Catan.CitiesKnights.recordProgress("draw", p, -1, 0, 1, nil)
			s.Catan.CitiesKnights.ProgressEvents[0].ID++
		},
		"off-turn-limit": func(s *State) { ckProgressGive(t, s, (p+1)%3, 0, 1, 2, 3, 4) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			damage(&bad)
			if bad.validateCatanExplorer() == nil {
				t.Fatal("corrupt combination save accepted")
			}
			explorerCityStateReject(t, &bad, p, Action{Type: "catan_explorer_begin_move", Prompt: int(bad.Catan.TurnSerial)})
		})
	}
}

func TestCatanExplorerCityAggregateEventQueueAtomic(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	s.Catan.CitiesKnights.BarbarianPosition = 6
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	explorerCityStateRestore(t, s)
	if len(s.Catan.CitiesKnights.Event.Tasks) != 3 {
		t.Fatal("expected all three pillage tasks")
	}
	p := s.Catan.CitiesKnights.Pending.Players[0]
	a := Action{Type: "catan_pillage", Vertex: s.Catan.pillageSites(p)[0], Prompt: int(s.Catan.TurnSerial)}
	invalid := a
	invalid.Skill = "forge"
	explorerCityStateReject(t, s, p, invalid)
	for name, damage := range map[string]func(*State){
		"missing":        func(s *State) { s.Catan.CitiesKnights.Event = nil },
		"head-player":    func(s *State) { s.Catan.CitiesKnights.Event.Tasks[0].Player = (p + 1) % 3 },
		"head-kind":      func(s *State) { s.Catan.CitiesKnights.Event.Tasks[0].Kind = "defender_reward" },
		"duplicate":      func(s *State) { e := s.Catan.CitiesKnights.Event; e.Tasks[1] = e.Tasks[0] },
		"order":          func(s *State) { e := s.Catan.CitiesKnights.Event; e.Tasks[1], e.Tasks[2] = e.Tasks[2], e.Tasks[1] },
		"invalid-player": func(s *State) { s.Catan.CitiesKnights.Event.Tasks[0].Player = 99 },
		"attack":         func(s *State) { s.Catan.CitiesKnights.Event.Attack = false },
		"source":         func(s *State) { s.Catan.CitiesKnights.Pending.Source = "explorer_movement" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			damage(&bad)
			if bad.validateCatanExplorer() == nil {
				t.Fatal("corrupt event queue accepted")
			}
			explorerCityStateReject(t, &bad, p, a)
		})
	}
	explorerCityStateReady(t, s)
	if s.Catan.CitiesKnights.Invasions != 1 || s.Catan.CitiesKnights.BarbarianPosition != 0 {
		t.Fatal("valid attack failed after corruption tests")
	}
}

func TestCatanExplorerCityAggregateVictoryAndConstructorGates(t *testing.T) {
	for _, n := range []int{2, 7} {
		if _, err := newCatanExplorerCityState(n, "pirate-lairs", []int{3, 4, 5, 9, 10, 11}); err == nil {
			t.Fatal("unsupported player count accepted")
		}
	}
	for _, config := range []struct {
		scenario string
		numbers  []int
	}{{"land-ho", nil}, {"pirate-lairs", nil}, {"spices-for-catan", []int{3, 4, 5, 9, 10, 11}}} {
		if _, err := newCatanExplorerCityState(3, config.scenario, config.numbers); err == nil {
			t.Fatal("invalid combination recipe accepted", config)
		}
	}
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	explorerCityStateReady(t, s)
	p := s.Turn
	// Explicit score boundary; the public action actually plays/transfers Merchant.
	s.Catan.CitiesKnights.Players[p].DefenderPoints = s.Catan.Explorer.Board.Target - 1 - s.Catan.Players[p].Score
	s.catanScores()
	ckProgressGive(t, s, p, 12)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 12, Tile: explorerMerchantTile(t, s, p)})
	if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{p}) {
		t.Fatal("unified action omitted immediate victory")
	}
	explorerCityStateReject(t, s, p, Action{Type: "catan_explorer_begin_move", Prompt: int(s.Catan.TurnSerial)})
	for _, damage := range []func(*State){func(s *State) { s.Winners = []int{(p + 1) % 3} }, func(s *State) { s.Phase = "catan_turn" }, func(s *State) { s.Catan.CitiesKnights.Players[p].DefenderPoints--; s.catanScores() }} {
		bad := clone(*s)
		damage(&bad)
		if bad.validateCatanExplorer() == nil {
			t.Fatal("invalid finished save accepted")
		}
	}
}

func TestCatanExplorerCityAggregateAlchemyAndPublicEventWindow(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "spices-for-catan")
	p := s.Turn
	ckProgressGive(t, s, p, 0)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}})
	explorerCityStateReady(t, s)
	if !slices.Equal(s.Catan.Dice, []int{1, 1}) || s.Catan.RollID != 1 || slices.Contains(s.Catan.CitiesKnights.Players[p].Progress, 0) {
		t.Fatal("unified Alchemy failed owned card/dice/production contract")
	}
	// Controlled replenishment from the real deck exercises pruning/restoration
	// of public play events without manufacturing extra physical cards.
	for range 20 {
		ckProgressGive(t, s, p, 14)
		explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 14, Choice: "skip"})
	}
	k := s.Catan.CitiesKnights
	if len(k.ProgressEvents) != 18 || k.ProgressEventID != 21 || k.ProgressEvents[0].ID != 4 {
		t.Fatal("public event window lost sequence after restore")
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
		if len(v["progressEvents"].([]any)) != 18 {
			t.Fatal("public events missing for player/spectator")
		}
	}
}

func TestCatanExplorerCityAggregateRejectedActionPreservesLongLog(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "spices-for-catan")
	// Older/private stored snapshots can predate the public 80-line log cap.
	// Rejecting a request must not silently rewrite any of that snapshot.
	s.Log = make([]string, 90)
	for i := range s.Log {
		s.Log[i] = fmt.Sprintf("历史记录%d", i)
	}
	explorerCityStateReject(t, s, (s.Turn+1)%3, Action{Type: "catan_roll", Prompt: int(s.Catan.TurnSerial)})
	explorerCityStateAct(t, s, s.Turn, Action{Type: "catan_roll"})
	if len(s.Log) > 80 {
		t.Fatal("successful action did not enforce normal public log bound")
	}
}
