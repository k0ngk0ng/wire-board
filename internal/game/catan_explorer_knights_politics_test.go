package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCatanExplorerCityPoliticsEspionagePrivacyAndUse(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for _, mode := range []string{"take", "skip", "empty"} {
			t.Run(fmt.Sprintf("%d/%s", config.n, mode), func(t *testing.T) {
				s := explorerTradeProgressFixture(t, config.n, config.secondary)
				actor, target := s.Turn, (s.Turn+1)%config.n
				ckProgressGive(t, s, actor, 18)
				if mode != "empty" {
					ckProgressGive(t, s, target, 4, 5)
				}
				for _, invalid := range []int{-1, actor, config.n} {
					explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: 18, Target: invalid, Prompt: int(s.Catan.TurnSerial)})
				}
				before := clone(*s.Catan.Explorer)
				logStart := len(s.Log)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 18, Target: target})
				if mode == "empty" {
					if s.Phase != "catan_turn" {
						t.Fatal("empty progress hand blocked game")
					}
					return
				}
				for viewer := -1; viewer < config.n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
					hand, ok := v["pending"].(map[string]any)["progress"]
					if ok != (viewer == actor) || ok && !reflect.DeepEqual(hand, []int{4, 5}) {
						t.Fatal("spy revealed hand to wrong player")
					}
					for p, raw := range v["players"].([]any) {
						_, visible := raw.(map[string]any)["progress"]
						if visible != (viewer == p) {
							t.Fatal("ordinary progress privacy changed")
						}
					}
				}
				for _, card := range []int{-1, 9, 23, 99} {
					explorerCityReject(t, s, actor, Action{Type: "catan_espionage", Card: card, Prompt: int(s.Catan.TurnSerial)})
				}
				explorerCityReject(t, s, target, Action{Type: "catan_espionage", Card: 4, Prompt: int(s.Catan.TurnSerial)})
				a := Action{Type: "catan_espionage", Card: 4}
				if mode == "skip" {
					a.Choice = "skip"
				}
				explorerTradeProgressRespond(t, s, actor, a)
				if !reflect.DeepEqual(*s.Catan.Explorer, before) || s.Phase != "catan_turn" || s.Turn != actor {
					t.Fatal("spy changed production/cargo or actor")
				}
				for _, line := range s.Log[logStart:] {
					if strings.Contains(line, "灌溉") || strings.Contains(line, "医学") {
						t.Fatal("stolen card exposed in log")
					}
				}
				if mode == "take" {
					if !slices.Equal(s.Catan.CitiesKnights.Players[target].Progress, []int{5}) {
						t.Fatal("wrong progress card transferred")
					}
					for _, event := range s.Catan.CitiesKnights.ProgressEvents {
						if event.Kind == "transfer" && event.Card != nil {
							t.Fatal("transfer event exposes card identity")
						}
					}
					explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 4}) // Usable immediately.
				} else if len(s.Catan.CitiesKnights.Players[actor].Progress) != 0 || len(s.Catan.CitiesKnights.Players[target].Progress) != 2 {
					t.Fatal("skip transferred a card")
				}
			})
		}
	}
}

func TestCatanExplorerCityPoliticsWeddingAndSabotageQueue(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for _, card := range []int{20, 24} {
			t.Run(fmt.Sprintf("%d/%d", config.n, card), func(t *testing.T) {
				s := explorerTradeProgressFixture(t, config.n, config.secondary)
				actor := s.Turn
				kind := "wedding"
				if card == 20 {
					kind = "sabotage"
				}
				s.Catan.CitiesKnights.Players[actor].DefenderPoints = 1
				expected := []int{}
				for offset := 1; offset < config.n; offset++ {
					p := (actor + offset) % config.n
					// First is higher, second equal, later lower; both odd/even
					// hands contain ordinary resources and commodities.
					points := 0
					if offset == 1 {
						points = 2
					}
					if offset == 2 {
						points = 1
					}
					s.Catan.CitiesKnights.Players[p].DefenderPoints = points
					explorerDevelopmentGrant(t, s, p, 0, 1)
					explorerDevelopmentGrant(t, s, p, 5, offset%2+1)
					if points > 1 || card == 20 && points == 1 {
						expected = append(expected, p)
					}
				}
				s.catanScores()
				ckProgressGive(t, s, actor, card)
				before := clone(*s.Catan)
				logStart := len(s.Log)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card})
				if !slices.Equal(s.Catan.CitiesKnights.Pending.Players, expected) {
					t.Fatal("wrong score threshold or clockwise response order")
				}
				for _, p := range expected {
					if s.CatanPendingActor() != p {
						t.Fatal("response queue skipped player")
					}
					hand := slices.Clone(s.Catan.Players[p].Resources)
					due := min(2, sum(hand))
					if card == 20 {
						due = sum(hand) / 2
					}
					give := make([]int, 8)
					for r, count := range hand {
						give[r] = min(count, due-sum(give))
					}
					explorerCityReject(t, s, actor, Action{Type: "catan_" + kind, Give: give, Prompt: int(s.Catan.TurnSerial)})
					explorerCityReject(t, s, p, Action{Type: "catan_" + kind, Give: make([]int, 8), Prompt: int(s.Catan.TurnSerial)})
					explorerTradeProgressRespond(t, s, p, Action{Type: "catan_" + kind, Give: give})
					if sum(s.Catan.Players[p].Resources) != sum(hand)-due {
						t.Fatal("wrong forced amount")
					}
				}
				if s.Turn != actor || s.Phase != "catan_turn" || !reflect.DeepEqual(s.Catan.Explorer, before.Explorer) {
					t.Fatal("politics response restarted explorer turn")
				}
				if card == 24 && !slices.Equal(s.Catan.Bank, before.Bank) {
					t.Fatal("wedding paid bank instead of actor")
				}
				if card == 20 && sum(s.Catan.Players[actor].Resources) != 0 {
					t.Fatal("sabotage paid actor instead of bank")
				}
				for _, line := range s.Log[logStart:] {
					if strings.Contains(line, "木材") || strings.Contains(line, "纸张") {
						t.Fatal("private choice revealed")
					}
				}
			})
		}
	}
}

func explorerPoliticsKnightFixture(t *testing.T, n int, secondary bool) (*State, [3]int, [2]int) {
	t.Helper()
	s, v, e := explorerKnightFixture(t, n)
	if secondary {
		explorerCityFinishAction(t, s)
		for i := range s.Catan.Edges {
			if s.Catan.Edges[i].Owner == 0 {
				s.Catan.Edges[i].Owner = s.Turn
			}
		}
	}
	return s, v, e
}

func TestCatanExplorerCityPoliticsEncouragementAndIntrigue(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		t.Run(fmt.Sprintf("encourage/%d", config.n), func(t *testing.T) {
			s, v, _ := explorerPoliticsKnightFixture(t, config.n, config.secondary)
			p := s.Turn
			s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: v[0], Strength: 1}, {Owner: p, Vertex: v[2], Strength: 1, Active: true}}
			ckProgressGive(t, s, p, 17, 17)
			bank := slices.Clone(s.Catan.Bank)
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 17})
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 17})
			if !s.Catan.knightAt(v[0]).Active || !s.Catan.knightCanAct(s.Catan.knightAt(v[2])) || !slices.Equal(bank, s.Catan.Bank) {
				t.Fatal("encouragement changed cost or prior activation")
			}
			explorerCityActionReject(t, s, p, Action{Type: "catan_knight_move", Vertex: v[0], Target: v[1], Prompt: int(s.Catan.TurnSerial)})
			explorerKnightAct(t, s, Action{Type: "catan_knight_move", Vertex: v[2], Target: v[1]})
		})
		for _, escape := range []bool{false, true} {
			t.Run(fmt.Sprintf("intrigue/%d/%t", config.n, escape), func(t *testing.T) {
				s, v, e := explorerPoliticsKnightFixture(t, config.n, config.secondary)
				actor, other := s.Turn, (s.Turn+1)%config.n
				s.Catan.Edges[e[1]].Owner = -1
				if escape {
					s.Catan.Edges[e[1]].Owner = other
				}
				n := CatanKnight{Owner: other, Vertex: v[1], Strength: 3, Active: true, PromotedAt: s.Catan.TurnSerial}
				s.Catan.CitiesKnights.Knights = []CatanKnight{n}
				ckProgressGive(t, s, actor, 19)
				before := clone(*s.Catan.Explorer)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 19, Vertex: v[1]})
				if !escape {
					if s.Phase != "catan_turn" || s.Catan.knightCount(other, 3) != 0 {
						t.Fatal("trapped knight not returned")
					}
					return
				}
				if s.CatanPendingActor() != other || s.Catan.CitiesKnights.Pending.Source != "intrigue" || s.Catan.knightAt(v[1]) != nil {
					t.Fatal("intrigue created attack knight or wrong retreat")
				}
				for _, source := range []string{"", "invalid"} {
					q := clone(*s)
					q.Catan.CitiesKnights.Pending.Source = source
					explorerCityReject(t, &q, other, Action{Type: "catan_knight_retreat", Vertex: v[2], Prompt: int(s.Catan.TurnSerial)})
				}
				explorerCityReject(t, s, actor, Action{Type: "catan_knight_retreat", Vertex: v[2], Prompt: int(s.Catan.TurnSerial)})
				explorerTradeProgressRespond(t, s, other, Action{Type: "catan_knight_retreat", Vertex: v[2]})
				n.Vertex = v[2]
				if !reflect.DeepEqual(*s.Catan.knightAt(v[2]), n) || !reflect.DeepEqual(*s.Catan.Explorer, before) || s.Turn != actor {
					t.Fatal("retreat lost activation/locks or changed explorer state")
				}
			})
		}
	}
}

func TestCatanExplorerCityPoliticsDiplomacyRemovalAndRebuild(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for _, mode := range []string{"rebuild", "skip", "opponent", "closed"} {
			t.Run(fmt.Sprintf("%d/%s", config.n, mode), func(t *testing.T) {
				s, v, e := explorerPoliticsKnightFixture(t, config.n, config.secondary)
				p := s.Turn
				explorerRoadsEmptyHand(t, s)
				if mode == "opponent" {
					s.Catan.Edges[e[0]].Owner = (p + 1) % config.n
				}
				if mode == "closed" {
					s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: v[0], Strength: 1}, {Owner: p, Vertex: v[2], Strength: 1}}
				}
				ckProgressGive(t, s, p, 16)
				before := clone(*s.Catan)
				a := Action{Type: "catan_progress", Card: 16, Edge: e[0], Prompt: int(s.Catan.TurnSerial)}
				if mode == "closed" {
					explorerCityActionReject(t, s, p, a)
					return
				}
				explorerKnightAct(t, s, a)
				if s.Catan.Edges[e[0]].Owner != -1 {
					t.Fatal("diplomacy did not remove chosen road")
				}
				if mode == "opponent" {
					if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Pending != nil {
						t.Fatal("opponent road gave free rebuild")
					}
					return
				}
				if s.Phase != "catan_diplomacy" || s.CatanPendingActor() != p {
					t.Fatal("missing own free rebuild")
				}
				for _, id := range s.Catan.diplomacyPlacements(p) {
					if !catanExplorerLandEdge(s.Catan, id) {
						t.Fatal("diplomacy offered sea or fog")
					}
				}
				explorerCityReject(t, s, p, Action{Type: "catan_diplomacy", Edge: -1, Prompt: int(s.Catan.TurnSerial)})
				explorerCityReject(t, s, (p+1)%config.n, Action{Type: "catan_diplomacy", Choice: "skip", Prompt: int(s.Catan.TurnSerial)})
				a = Action{Type: "catan_diplomacy", Choice: "skip"}
				if mode == "rebuild" {
					a.Choice = ""
					a.Edge = -1
					for _, id := range s.Catan.diplomacyPlacements(p) {
						if id != e[0] {
							a.Edge = id
							break
						}
					}
					if a.Edge < 0 {
						t.Fatal("fixture lacks different rebuild site")
					}
				}
				explorerTradeProgressRespond(t, s, p, a)
				if mode == "rebuild" && s.Catan.Edges[a.Edge].Owner != p {
					t.Fatal("free rebuild missing")
				}
				if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 || !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Explorer, before.Explorer) || s.Catan.LongestOwner != -1 {
					t.Fatal("diplomacy charged resources, altered ships or granted route award")
				}
			})
		}
	}
}

func TestCatanExplorerCityPoliticsTreasonStrengthStatusAndNoPlacement(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for _, mode := range []string{"mighty-active", "basic-inactive", "skip", "no-road", "no-stock"} {
			t.Run(fmt.Sprintf("%d/%s", config.n, mode), func(t *testing.T) {
				s, v, e := explorerPoliticsKnightFixture(t, config.n, config.secondary)
				actor, other := s.Turn, (s.Turn+1)%config.n
				s.Catan.Edges[e[1]].Owner = other
				strength, active := 3, mode == "mighty-active"
				if mode == "basic-inactive" || mode == "no-stock" {
					strength = 1
				}
				s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: other, Vertex: v[2], Strength: strength, Active: active}}
				if mode == "no-road" {
					s.Catan.Edges[e[0]].Owner = -1
				}
				if mode == "no-stock" {
					s.Catan.CitiesKnights.Knights = append(s.Catan.CitiesKnights.Knights, CatanKnight{Owner: actor, Vertex: v[0], Strength: 1}, CatanKnight{Owner: actor, Vertex: v[1], Strength: 1})
				}
				ckProgressGive(t, s, actor, 22)
				before := clone(*s.Catan)
				explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: 22, Target: actor, Prompt: int(s.Catan.TurnSerial)})
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 22, Target: other})
				if s.CatanPendingActor() != other || s.Phase != "catan_treason_remove" {
					t.Fatal("opponent cannot choose knight")
				}
				explorerCityReject(t, s, actor, Action{Type: "catan_treason_remove", Vertex: v[2], Prompt: int(s.Catan.TurnSerial)})
				explorerCityReject(t, s, other, Action{Type: "catan_treason_remove", Vertex: v[0], Prompt: int(s.Catan.TurnSerial)})
				explorerTradeProgressRespond(t, s, other, Action{Type: "catan_treason_remove", Vertex: v[2]})
				if s.Catan.knightAt(v[2]) != nil || s.Catan.knightCount(other, strength) != 0 {
					t.Fatal("removed knight retained in inventory")
				}
				if mode == "no-road" || mode == "no-stock" {
					if s.Phase != "catan_turn" {
						t.Fatal("treason without placement stalled")
					}
					return
				}
				if s.Phase != "catan_treason_place" || s.CatanPendingActor() != actor {
					t.Fatal("missing replacement step")
				}
				explorerCityReject(t, s, actor, Action{Type: "catan_treason_place", Vertex: v[0], Color: 4, Prompt: int(s.Catan.TurnSerial)})
				explorerCityReject(t, s, other, Action{Type: "catan_treason_place", Vertex: v[0], Color: strength, Prompt: int(s.Catan.TurnSerial)})
				a := Action{Type: "catan_treason_place", Vertex: v[0], Color: strength}
				if mode == "skip" {
					a.Choice = "skip"
				}
				explorerTradeProgressRespond(t, s, actor, a)
				if mode != "skip" {
					piece := s.Catan.knightAt(v[0])
					if piece == nil || piece.Owner != actor || piece.Strength != strength || piece.Active != active {
						t.Fatal("replacement lost strength or activation")
					}
				}
				if !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Explorer, before.Explorer) || s.Phase != "catan_turn" || s.Turn != actor {
					t.Fatal("treason charged resources or altered explorer turn")
				}
			})
		}
	}
}

func TestCatanExplorerCityPoliticsCorruptPendingAndSkip(t *testing.T) {
	for _, card := range []int{16, 17, 18, 19, 20, 22, 24} {
		s := explorerTradeProgressFixture(t, 3, false)
		ckProgressGive(t, s, 0, card)
		explorerCityActionReject(t, s, 1, Action{Type: "catan_progress", Card: card, Choice: "skip", Prompt: 1})
		explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card, Choice: "skip"})
		if len(s.Catan.CitiesKnights.Players[0].Progress) != 0 || s.Catan.CitiesKnights.Pending != nil {
			t.Fatal("waived card retained or applied effect")
		}
	}
	s := explorerTradeProgressFixture(t, 6, true)
	for _, p := range []int{4, 5, 0} {
		s.Catan.CitiesKnights.Players[p].DefenderPoints = 1
		explorerDevelopmentGrant(t, s, p, 0, 2)
	}
	s.catanScores()
	ckProgressGive(t, s, 3, 24)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 24})
	for _, kind := range []string{"empty", "negative", "outside", "repeat", "order", "target", "self", "empty-hand", "score", "ship", "source"} {
		q := clone(*s)
		pending := q.Catan.CitiesKnights.Pending
		switch kind {
		case "empty":
			pending.Players = nil
		case "negative":
			pending.Players = []int{-1}
		case "outside":
			pending.Players = []int{6}
		case "repeat":
			pending.Players = []int{4, 4}
		case "order":
			pending.Players = []int{5, 4}
		case "target":
			pending.Target = 0
		case "self":
			pending.Players = []int{3}
		case "empty-hand":
			q.Catan.Bank[0] += 2
			q.Catan.Players[4].Resources[0] = 0
		case "score":
			q.Catan.CitiesKnights.Players[4].DefenderPoints = 0
			q.catanScores()
		case "ship":
			pending.Ship = true
		case "source":
			pending.Source = "intrigue"
		}
		explorerCityReject(t, &q, 4, Action{Type: "catan_wedding", Give: []int{2, 0, 0, 0, 0, 0, 0, 0}, Prompt: int(q.Catan.TurnSerial)})
	}
	s = explorerTradeProgressFixture(t, 3, false)
	ckProgressGive(t, s, 0, 18)
	ckProgressGive(t, s, 1, 4)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 18, Target: 1})
	for _, kind := range []string{"target", "empty-hand", "bad-card", "victory-card", "actor"} {
		q := clone(*s)
		k := q.Catan.CitiesKnights
		switch kind {
		case "target":
			k.Pending.Target = 3
		case "empty-hand":
			k.Players[1].Progress = nil
		case "bad-card":
			k.Players[1].Progress = []int{99}
		case "victory-card":
			k.Players[1].Progress = []int{9}
		case "actor":
			k.Pending.Players = []int{2}
		}
		explorerCityReject(t, &q, 0, Action{Type: "catan_espionage", Card: 99, Prompt: 1})
	}
}

func TestCatanExplorerCityPoliticsSmallHandsAndTreasonCorruption(t *testing.T) {
	for _, card := range []int{20, 24} {
		for _, higher := range []bool{false, true} {
			s := explorerTradeProgressFixture(t, 3, false)
			if higher {
				s.Catan.CitiesKnights.Players[1].DefenderPoints = 1
				s.catanScores()
			}
			explorerDevelopmentGrant(t, s, 1, 7, 1)
			ckProgressGive(t, s, 0, card)
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card})
			if card == 24 && higher {
				explorerTradeProgressRespond(t, s, 1, Action{Type: "catan_wedding", Give: []int{0, 0, 0, 0, 0, 0, 0, 1}})
				if s.Catan.Players[0].Resources[7] != 1 {
					t.Fatal("one-card wedding did not transfer all")
				}
			} else if s.Phase != "catan_turn" || s.Catan.Players[1].Resources[7] != 1 {
				t.Fatal("zero discard or nonqualifying wedding blocked or moved cards")
			}
		}
	}
	s, v, _ := explorerKnightFixture(t, 3)
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: v[2], Strength: 3, Active: true}}
	ckProgressGive(t, s, 0, 22)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 22, Target: 1})
	for _, kind := range []string{"target", "actor", "no-knight"} {
		q := clone(*s)
		switch kind {
		case "target":
			q.Catan.CitiesKnights.Pending.Target = 2
		case "actor":
			q.Catan.CitiesKnights.Pending.Players = []int{0}
		case "no-knight":
			q.Catan.CitiesKnights.Knights = nil
		}
		explorerCityReject(t, &q, 1, Action{Type: "catan_treason_remove", Vertex: v[2], Prompt: 1})
	}
	explorerTradeProgressRespond(t, s, 1, Action{Type: "catan_treason_remove", Vertex: v[2]})
	for _, kind := range []string{"nil", "owner", "strength", "vertex", "future", "occupied"} {
		q := clone(*s)
		k := q.Catan.CitiesKnights
		switch kind {
		case "nil":
			k.Pending.Knight = nil
		case "owner":
			k.Pending.Knight.Owner = 0
		case "strength":
			k.Pending.Knight.Strength = 4
		case "vertex":
			k.Pending.Knight.Vertex = len(q.Catan.Vertices)
		case "future":
			k.Pending.Knight.ActivatedAt = 2
		case "occupied":
			k.Knights = append(k.Knights, CatanKnight{Owner: 1, Vertex: v[2], Strength: 1})
		}
		explorerCityReject(t, &q, 0, Action{Type: "catan_treason_place", Vertex: v[0], Color: 3, Prompt: 1})
	}
	// A lower-strength substitute is legal and remains active, without the
	// ordinary city-improvement prerequisite for recruiting a mighty knight.
	explorerTradeProgressRespond(t, s, 0, Action{Type: "catan_treason_place", Vertex: v[0], Color: 2})
	if s.Catan.knightAt(v[0]).Strength != 2 || !s.Catan.knightAt(v[0]).Active {
		t.Fatal("lower-strength active replacement rejected")
	}
}
