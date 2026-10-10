package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func explorerCityViewChoices(t *testing.T, s *State, p int) []Action {
	t.Helper()
	before := clone(*s)
	v := s.View(p)["catan"].(map[string]any)
	raw, err := json.Marshal(v["explorer"].(map[string]any)["choices"])
	if err != nil {
		t.Fatal(err)
	}
	var actions []Action
	if err = json.Unmarshal(raw, &actions); err != nil {
		t.Fatal(err)
	}
	want := s.catanExplorerChoices(p)
	if len(actions) != len(want) {
		t.Fatal("wire projection lost choices")
	}
	// Optional empty freight lists are omitted on the wire; nil and [] have
	// identical action semantics. Every nonempty list and every ID must survive.
	for i := range want {
		for _, field := range []*[]int{&want[i].Cards, &want[i].Targets, &want[i].SpiceLoad, &want[i].SpiceUnload} {
			if len(*field) == 0 {
				*field = nil
			}
		}
		for _, field := range []*[]int{&actions[i].Cards, &actions[i].Targets, &actions[i].SpiceLoad, &actions[i].SpiceUnload} {
			if len(*field) == 0 {
				*field = nil
			}
		}
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatal("wire projection lost action fields", actions, want)
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("preview mutated the live game")
	}
	return actions
}

func explorerCityPreviewExecutable(t *testing.T, s *State, p int) map[string]int {
	t.Helper()
	seen := map[string]int{}
	for _, a := range explorerCityViewChoices(t, s, p) {
		trial := clone(*s)
		if err := trial.Apply(p, a); err != nil {
			t.Fatal("advertised choice rejected", s.Phase, p, a, err)
		}
		explorerCityStateRestore(t, &trial)
		seen[a.Type]++
	}
	return seen
}

func TestCatanExplorerCityPreviewBuildingsBankAndMovement(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := explorerCityStateStarted(t, n, scenario)
				explorerCityStateReady(t, s)
				p := s.Turn
				for color := 0; color < 8; color++ {
					explorerDevelopmentGrant(t, s, p, color, 4)
				}
				seen := explorerCityPreviewExecutable(t, s, p)
				for _, kind := range []string{"catan_wall", "catan_improvement", "catan_explorer_bank", "catan_road", "catan_explorer_ship", "catan_explorer_unit", "catan_explorer_begin_move"} {
					if seen[kind] == 0 {
						t.Fatal("missing combined action", kind, seen)
					}
				}
				commodityGive, commodityTake := false, false
				for _, a := range s.catanExplorerChoices(p) {
					if a.Type == "catan_wall" && s.Catan.Vertices[a.Vertex].Harbor {
						t.Fatal("harbor advertised as city")
					}
					if a.Type == "catan_explorer_bank" {
						commodityGive = commodityGive || a.Color >= 5
						commodityTake = commodityTake || a.Target >= 5
						if a.Color == -1 && a.Target >= 5 || a.Color >= 5 && a.Target == -1 {
							t.Fatal("illegal gold/commodity bank option", a)
						}
					}
				}
				if !commodityGive || !commodityTake {
					t.Fatal("commodity bank options omitted")
				}
				for viewer := -1; viewer < n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					for seat, raw := range v["players"].([]any) {
						if !slices.Equal(raw.(map[string]any)["rates"].([]int), s.Catan.explorerCityRates(seat)) {
							t.Fatal("wrong public combination bank rates")
						}
					}
					if viewer != p && (len(explorerCityViewChoices(t, s, viewer)) != 0 || v["explorer"].(map[string]any)["canRespond"] != false) {
						t.Fatal("action choices sent to nonactor")
					}
				}
				explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
				seen = explorerCityPreviewExecutable(t, s, p)
				if seen["catan_explorer_sail"] == 0 || seen["catan_end"] != 1 {
					t.Fatal("missing movement preview", seen)
				}
			})
		}
	}
}

func TestCatanExplorerCityPreviewPillageAndFallenCity(t *testing.T) {
	s := explorerCityStateStarted(t, 6, "explorers-and-pirates")
	s.Catan.CitiesKnights.BarbarianPosition = 6
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	for s.Phase == "catan_pillage" {
		actor := s.CatanPendingActor()
		seen := explorerCityPreviewExecutable(t, s, actor)
		if seen["catan_pillage"] == 0 || len(seen) != 1 {
			t.Fatal("wrong city response choices", seen)
		}
		for viewer := -1; viewer < 6; viewer++ {
			v := s.View(viewer)["catan"].(map[string]any)
			if v["explorer"].(map[string]any)["canRespond"] != (viewer == actor) {
				t.Fatal("response permissions assigned to turn owner")
			}
			if viewer != actor && len(explorerCityViewChoices(t, s, viewer)) > 0 {
				t.Fatal("nonresponder has pillage actions")
			}
		}
		explorerCityBotStep(t, s, actor)
	}
	p := s.Turn
	ckProgressGive(t, s, p, 2, 5)
	for color := 0; color < 8; color++ {
		explorerDevelopmentGrant(t, s, p, color, 3)
	}
	seen := explorerCityPreviewExecutable(t, s, p)
	if seen["catan_wall"] != 0 || seen["catan_improvement"] != 0 || seen["catan_city"] == 0 {
		t.Fatal("harbor incorrectly substitutes for fallen city", seen)
	}
	v := s.View(p)["catan"].(map[string]any)
	if len(v["medicineHarbors"].([]int)) != 0 {
		t.Fatal("fallen city offered as Medicine harbor")
	}
	for _, a := range s.catanExplorerChoices(p) {
		if a.Type == "catan_explorer_harbor" && slices.Contains(s.Catan.CitiesKnights.FallenCities, a.Vertex) {
			t.Fatal("fallen city offered as ordinary harbor")
		}
	}
}

func TestCatanExplorerCityPreviewFreeRoadsAndProgressWindow(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	p := s.Turn
	ckProgressGive(t, s, p, 0, 7, 21)
	v := s.View(p)["catan"].(map[string]any)
	if !slices.Equal(v["progressPlayable"].([]int), []int{0}) {
		t.Fatal("wrong Alchemy window", v["progressPlayable"])
	}
	explorerCityStateReady(t, s)
	v = s.View(p)["catan"].(map[string]any)
	if !slices.Equal(v["progressPlayable"].([]int), []int{7}) {
		t.Fatal("Taxation allowed before invasion", v["progressPlayable"])
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 7})
	for s.Phase == "catan_roads" {
		for c, n := range s.Catan.Players[p].Resources {
			s.Catan.Bank[c] += n
			s.Catan.Players[p].Resources[c] = 0
		}
		seen := explorerCityPreviewExecutable(t, s, p)
		if len(seen) != 1 || seen["catan_road"] == 0 {
			t.Fatal("free road preview requires resources or offers normal action", seen)
		}
		explorerCityStateAct(t, s, p, s.catanExplorerChoices(p)[0])
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	v = s.View(p)["catan"].(map[string]any)
	if len(v["progressPlayable"].([]int)) != 0 {
		t.Fatal("progress allowed during sailing")
	}
}

func TestCatanExplorerCityPreviewPrivateTradeAndBundles(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	explorerCityStateReady(t, s)
	p, other := s.Turn, (s.Turn+1)%3
	ckProgressGive(t, s, p, 10, 11, 0, 1, 2, 3, 4, 5)
	explorerDevelopmentGrant(t, s, p, 0, 1)
	explorerDevelopmentGrant(t, s, other, 5, 1)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
	explorerCityStateAct(t, s, p, Action{Type: "catan_commercial_offer", Target: other, Color: 0})
	if seen := explorerCityPreviewExecutable(t, s, other); seen["catan_commercial_harbor"] != 1 {
		t.Fatal("commodity response preview missing", seen)
	}
	for viewer := -1; viewer < 3; viewer++ {
		if viewer != other && len(explorerCityViewChoices(t, s, viewer)) != 0 {
			t.Fatal("private commodity choices leaked")
		}
	}
	explorerCityBotStep(t, s, other)
	explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	v := s.View(p)["catan"].(map[string]any)
	q := v["explorer"].(map[string]any)["response"].(map[string]any)
	if s.Phase != "catan_progress_end" || q["type"] != "catan_progress_discard" || q["field"] != "cards" || q["count"] != 3 {
		t.Fatal("sailing overflow lacks owned-card selection contract", s.Phase, q)
	}
	for viewer := -1; viewer < 3; viewer++ {
		if viewer != p {
			x := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
			if _, ok := x["response"]; ok || x["canRespond"] != false {
				t.Fatal("response details leaked")
			}
		}
	}
	explorerCityBotStep(t, s, p)
}

func TestCatanExplorerCityPreviewSimultaneousDiscard(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	actor := s.Turn
	for _, p := range []int{(actor + 1) % 3, (actor + 2) % 3} {
		explorerDevelopmentGrant(t, s, p, 5+p, 8)
	}
	if err := s.catanExplorerCityRoll(3, 4, 0); err != nil {
		t.Fatal(err)
	}
	for viewer := -1; viewer < 3; viewer++ {
		x := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
		want := viewer >= 0 && s.Catan.DiscardDue[viewer] > 0
		if x["canRespond"] != want {
			t.Fatal("simultaneous discard capability differs", viewer)
		}
		if want {
			q := x["response"].(map[string]any)
			if q["field"] != "tokens" || q["count"] != s.Catan.DiscardDue[viewer] {
				t.Fatal("discard bundle quota missing")
			}
		}
	}
	s.AutoCatanPending()
	explorerCityStateRestore(t, s)
}

func TestCatanExplorerCityPreviewMedicineAndDiscountRates(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	explorerCityStateReady(t, s)
	p, g := s.Turn, s.Catan
	// Explicit legal coastal settlement fixture, standing for a prior settler
	// landing. This test checks upgrade quotes, not the landing trajectory.
	vertex := -1
	for _, v := range g.Vertices {
		if catanExplorerCoast(g, v.ID) && catanExplorerBotSite(g, p, v.ID, false) {
			vertex = v.ID
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no public coastal settlement fixture")
	}
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = p, 1
	s.catanScores()
	for c, n := range g.Players[p].Resources {
		g.Bank[c] += n
		g.Players[p].Resources[c] = 0
	}
	explorerDevelopmentGrant(t, s, p, 3, 1)
	explorerDevelopmentGrant(t, s, p, 4, 1)
	ckProgressGive(t, s, p, 5, 13)
	explorerCityStateRestore(t, s)
	v := s.View(p)["catan"].(map[string]any)
	if !slices.Contains(v["medicineHarbors"].([]int), vertex) || !slices.Contains(v["legal"].(map[string][]int)["cities"], vertex) {
		t.Fatal("discounted harbor omitted or city geometry requires full cost")
	}
	for _, a := range explorerCityViewChoices(t, s, p) {
		if a.Type == "catan_city" || a.Type == "catan_explorer_harbor" {
			t.Fatal("unaffordable ordinary upgrade advertised", a)
		}
	}
	copy := clone(*s)
	explorerCityStateAct(t, &copy, p, Action{Type: "catan_progress", Card: 5, Choice: "harbor", Vertex: vertex})
	if !copy.Catan.Vertices[vertex].Harbor || sum(copy.Catan.Players[p].Resources) != 0 {
		t.Fatal("Medicine harbor quote differs from actual cost")
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 13, Color: 7})
	explorerDevelopmentGrant(t, s, p, 7, 2)
	v = s.View(p)["catan"].(map[string]any)
	if v["players"].([]any)[p].(map[string]any)["rates"].([]int)[7] != 2 {
		t.Fatal("public commodity discount omitted")
	}
	found := false
	for _, a := range explorerCityViewChoices(t, s, p) {
		if a.Type == "catan_explorer_bank" && a.Color == 7 && a.Target == 5 {
			found = true
			explorerCityStateAct(t, s, p, a)
			break
		}
	}
	if !found || s.Catan.Players[p].Resources[7] != 0 || s.Catan.Players[p].Resources[5] != 1 {
		t.Fatal("discounted commodity exchange not executable")
	}
}

func TestCatanExplorerCityPreviewPrivateWorldAndEspionage(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "explorers-and-pirates")
	explorerCityStateReady(t, s)
	p, other := s.Turn, (s.Turn+1)%3
	for color := 0; color < 8; color++ {
		explorerDevelopmentGrant(t, s, p, color, 2)
	}
	explorerDevelopmentGrant(t, s, other, 6, 2)
	explorerDevelopmentGrant(t, s, (p+2)%3, 7, 2)
	ckProgressGive(t, s, p, 18)
	ckProgressGive(t, s, other, 1, 4, 4)
	first, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	// This helper validates changed fog, regional/lair numbers and opponent
	// resource composition, then compares the whole actor view and bot choice.
	assertExplorerFullBotPrivate(t, s, p, first)
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 18, Target: other})
	if seen := explorerCityPreviewExecutable(t, s, p); seen["catan_espionage"] != 3 {
		t.Fatal("authorized espionage must offer each distinct card and skip", seen)
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		pending := v["citiesKnights"].(map[string]any)["pending"].(map[string]any)
		_, revealed := pending["progress"]
		if revealed != (viewer == p) || viewer != p && len(explorerCityViewChoices(t, s, viewer)) != 0 {
			t.Fatal("espionage preview leaked to uninvolved seat")
		}
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_espionage", Choice: "skip"})
	explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
	first, err = s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	assertExplorerFullBotPrivate(t, s, p, first)
}

func TestCatanExplorerCityPreviewGuildDuesAndSabotageQuotas(t *testing.T) {
	for _, card := range []int{11, 20, 24} {
		t.Run(fmt.Sprint(card), func(t *testing.T) {
			s := explorerCityStateStarted(t, 3, "pirate-lairs")
			explorerCityStateReady(t, s)
			p, other := s.Turn, (s.Turn+1)%3
			// Explicit public score difference and physical hand, to trigger
			// the card's real eligibility without forging a Pending structure.
			s.Catan.CitiesKnights.Players[other].DefenderPoints = 1
			s.catanScores()
			explorerDevelopmentGrant(t, s, other, 5, 5)
			ckProgressGive(t, s, p, card)
			explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: card, Target: other})
			for steps := 0; s.CatanPendingActor() >= 0; steps++ {
				if steps >= 3 {
					t.Fatal("resource response queue stalled")
				}
				actor := s.CatanPendingActor()
				v := s.View(actor)["catan"].(map[string]any)
				q := v["explorer"].(map[string]any)["response"].(map[string]any)
				want := min(2, sum(s.Catan.Players[actor].Resources))
				field := "give"
				if card == 11 {
					want, field = min(2, sum(s.Catan.Players[other].Resources)), "take"
				}
				if card == 20 {
					want = sum(s.Catan.Players[actor].Resources) / 2
				}
				if q["count"] != want || q["field"] != field || q["prompt"] != int(s.Catan.TurnSerial) {
					t.Fatal("wrong private response quota", q)
				}
				for viewer := -1; viewer < 3; viewer++ {
					if viewer == actor {
						continue
					}
					view := s.View(viewer)["catan"].(map[string]any)
					if _, ok := view["explorer"].(map[string]any)["response"]; ok {
						t.Fatal("bundle selection sent to nonresponder")
					}
					if _, ok := view["citiesKnights"].(map[string]any)["pending"].(map[string]any)["resources"]; ok {
						t.Fatal("revealed Guild Dues hand leaked")
					}
				}
				explorerCityBotStep(t, s, actor)
			}
		})
	}
}

func TestCatanExplorerCityPreviewKnightMovesIncludeEmptySites(t *testing.T) {
	s := explorerCityStateStarted(t, 3, "pirate-lairs")
	explorerCityStateReady(t, s)
	p := s.Turn
	for _, color := range []int{0, 1} {
		explorerDevelopmentGrant(t, s, p, color, 6)
	}
	// Build a real public road network before placing a real recruited knight.
	for range 6 {
		edge := catanExplorerBotRoad(s.Catan, p)
		if edge < 0 {
			break
		}
		explorerCityStateAct(t, s, p, Action{Type: "catan_road", Edge: edge})
	}
	for _, color := range []int{2, 3, 4} {
		explorerDevelopmentGrant(t, s, p, color, 2)
	}
	vertex := -1
	for _, a := range s.catanExplorerChoices(p) {
		if a.Type == "catan_knight_recruit" {
			n := CatanKnight{Owner: p, Vertex: a.Vertex, Strength: 1}
			if len(s.Catan.knightDestinations(n, false)) > 0 {
				vertex = a.Vertex
				explorerCityStateAct(t, s, p, a)
				break
			}
		}
	}
	if vertex < 0 {
		t.Fatal("no recruitable connected knight fixture")
	}
	explorerCityStateAct(t, s, p, Action{Type: "catan_knight_activate", Vertex: vertex})
	for _, a := range explorerCityViewChoices(t, s, p) {
		if a.Type == "catan_knight_move" {
			t.Fatal("newly activated knight can move in same segment")
		}
	}
	// Actual turn handoffs make the activated knight eligible to move.
	for range 3 {
		actor := s.Turn
		explorerCityStateAct(t, s, actor, Action{Type: "catan_explorer_begin_move"})
		explorerCityStateAct(t, s, actor, Action{Type: "catan_end"})
		explorerCityStateReady(t, s)
	}
	if s.Turn != p {
		t.Fatal("failed to return to original actor")
	}
	seen := explorerCityPreviewExecutable(t, s, p)
	if seen["catan_knight_move"] == 0 {
		t.Fatal("human preview omitted moves into empty sites", seen)
	}
	v := s.View(p)["catan"].(map[string]any)
	if len(v["knightMoves"].(map[int][]int)[vertex]) == 0 || len(v["legal"].(map[string][]int)["knightChasePirate"]) != 0 {
		t.Fatal("knight geometry missing or unconfirmed pirate rule offered")
	}
}

// A helper response belongs to the player the helper asked. The explorer city
// panel used to hand canRespond to the turn owner whenever no cities-and-knights
// prompt was pending, so a helper call aimed at another seat offered the wrong
// player a response.
func TestCatanExplorerCityHelperResponseOwner(t *testing.T) {
	s := explorerCityStateStarted(t, 6, "explorers-and-pirates")
	s.Turn = 0
	s.Catan.Options.Helpers = true
	s.catanHelperAsk(CatanHelperPending{Player: 1, Kind: "resource", Resume: "catan_turn"})
	if s.CatanPendingActor() != 1 {
		t.Fatal("pending actor", s.CatanPendingActor())
	}
	for viewer := -1; viewer < 6; viewer++ {
		x := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
		want := viewer == 1
		if x["canRespond"] != want {
			t.Fatal("explorer helper response owner", viewer, x["canRespond"], want)
		}
	}
}
