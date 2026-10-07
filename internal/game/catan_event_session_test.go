package game

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func referenceEventGame(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, err := newCatanReferenceEvents(n)
	if err != nil {
		t.Fatal(err)
	}
	for setup && s.Catan.setup() {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	return s
}

func referenceEventRestore(t *testing.T, s *State) *State {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, &restored) {
		t.Fatal("save changed game state")
	}
	if err := restored.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	return &restored
}

// Swap a particular reference face to the top of a fresh, otherwise valid deck.
func referenceEventTop(t *testing.T, s *State, kind string) int {
	t.Helper()
	d := &s.Catan.EventDeck.Deck
	for i, id := range d.DrawPile {
		if id < catanEventNormalCards && catanEventReferenceFaces[id].Kind == kind {
			last := len(d.DrawPile) - 1
			d.DrawPile[i], d.DrawPile[last] = d.DrawPile[last], d.DrawPile[i]
			return id
		}
	}
	t.Fatal("no undrawn reference face", kind)
	return -1
}

func TestCatanEventSessionReferenceFactsAndPublicGate(t *testing.T) {
	f, err := os.Open("../../docs/research/catan-event-cards-legacy-reference.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 != len(catanEventReferenceFaces) {
		t.Fatal("reference count drift")
	}
	for i, row := range rows[1:] {
		production, err := strconv.Atoi(row[0])
		if err != nil || catanEventReferenceFaces[i].Production != production || catanEventReferenceFaces[i].Kind != row[2] {
			t.Fatal("reference fact drift", i)
		}
	}
	s, err := NewCatan(3, CatanOptions{})
	if err != nil || s.Catan.EventDeck != nil {
		t.Fatal("reference deck exposed through public constructor")
	}
	for _, n := range []int{2, 7} {
		if _, err := newCatanReferenceEvents(n); err == nil {
			t.Fatal("invalid player count")
		}
	}
}

func TestCatanEventSessionAllFacesDrawRespondRestore(t *testing.T) {
	for kind := range catanCardEventNames {
		t.Run(kind, func(t *testing.T) {
			s := referenceEventGame(t, 3, true)
			id := referenceEventTop(t, s, kind)
			face := catanEventReferenceFaces[id]
			helperReject(t, s, (s.Turn+1)%3, Action{Type: "catan_roll"})
			// Neither the card ID nor a client-supplied effect overrides the server draw.
			helperApply(t, s, s.Turn, Action{Type: "catan_roll", Card: 999, Choice: "arbitrary", Take: []int{24, 24, 24, 24, 24}})
			if s.Catan.RevealedEvent.Kind != kind || s.Catan.RevealedEvent.Production != face.Production || s.Catan.RollID != 1 {
				t.Fatal("wrong actual draw")
			}
			for step := 0; s.Phase != "catan_turn" && step < 30; step++ {
				s = referenceEventRestore(t, s)
				helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, actor, a)
				if len(s.Catan.EventDeck.Deck.Discard) != 1 || s.Catan.RollID != 1 {
					t.Fatal("response drew again")
				}
				catanCheck(t, s)
			}
			if s.Phase != "catan_turn" || s.Catan.CardEvent != nil || !s.Catan.RevealedEvent.ProductionStarted {
				t.Fatal("event did not finish")
			}
			s = referenceEventRestore(t, s)
			helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
		})
	}
}

func TestCatanEventSessionNewYearPrivacyAndHiddenOrder(t *testing.T) {
	s := referenceEventGame(t, 3, true)
	// This controlled loop performs no building: enough real turns to cross New Year.
	for draw := 1; draw <= 33; draw++ {
		before, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		other := clone(*s)
		pile := other.Catan.EventDeck.Deck.DrawPile
		pile[0], pile[1] = pile[1], pile[0]
		after, err := other.BotAction(other.Turn)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("bot consulted hidden order")
		}
		for viewer := -1; viewer < 3; viewer++ {
			if !reflect.DeepEqual(s.View(viewer), other.View(viewer)) {
				t.Fatal("view leaked hidden order")
			}
			v := s.View(viewer)["catan"].(map[string]any)
			data, _ := json.Marshal(v["eventDeck"])
			if strings.Contains(string(data), "drawPile") || strings.Contains(string(data), "\"deck\"") || !strings.Contains(string(data), "\"referenceOnly\":true") {
				t.Fatal("unsafe deck view", string(data))
			}
		}
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		s = referenceEventRestore(t, s)
		for step := 0; s.Phase != "catan_turn" && step < 30; step++ {
			actor := ckActor(s)
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, actor, a)
			s = referenceEventRestore(t, s)
		}
		if s.Phase != "catan_turn" || s.Catan.RollID != draw {
			t.Fatal("stuck or duplicate production", draw, s.Phase)
		}
		catanCheck(t, s)
		wantCycle := uint64((draw-1)/31 + 1)
		if s.Catan.EventDeck.Deck.Cycle != wantCycle {
			t.Fatal("incorrect New Year boundary")
		}
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	}
	found := 0
	for _, line := range s.Log {
		if strings.HasPrefix(line, "新年：") {
			found++
		}
	}
	if found != 1 {
		t.Fatal("missing or duplicate new year log", found)
	}
}

func TestCatanEventSessionInvalidStateAtomicAndSafeView(t *testing.T) {
	seed := referenceEventGame(t, 3, true)
	referenceEventTop(t, seed, "plentiful_year")
	helperApply(t, seed, seed.Turn, Action{Type: "catan_roll"})
	if seed.Catan.CardEvent == nil {
		t.Fatal("fixture needs pending response")
	}
	for name, damage := range map[string]func(*State){
		"catalogue":     func(s *State) { s.Catan.EventDeck.Catalogue = "2025-unverified" },
		"duplicate":     func(s *State) { s.Catan.EventDeck.Deck.DrawPile[0] = s.Catan.EventDeck.Deck.DrawPile[1] },
		"cycle":         func(s *State) { s.Catan.EventDeck.Deck.Cycle = ^uint64(0) },
		"roll":          func(s *State) { s.Catan.RollID++ },
		"negative-roll": func(s *State) { s.Catan.RollID = -1 },
		"face":          func(s *State) { s.Catan.RevealedEvent.Kind = "earthquake" },
		"pending":       func(s *State) { s.Catan.CardEvent.Production = 12 },
		"actor":         func(s *State) { s.Catan.CardEvent.Players = []int{99} },
		"produced":      func(s *State) { s.Catan.RevealedEvent.ProductionStarted = true },
		"helpers":       func(s *State) { s.Catan.Options.Helpers = true },
		"cities":        func(s *State) { s.Catan.CitiesKnights = &CatanCitiesKnights{} },
		"attack":        func(s *State) { s.Catan.Attack = &catanAttack{} },
	} {
		t.Run(name, func(t *testing.T) {
			s := clone(*seed)
			damage(&s)
			helperReject(t, &s, s.Turn, Action{Type: "catan_roll"})
			// Only inspect the projection for states whose unrelated scenario fields
			// form valid views; invalid hybrid state is still rejected before cloning.
			if name != "cities" && name != "attack" {
				if _, exists := s.View(-1)["catan"].(map[string]any)["eventDeck"]; exists {
					t.Fatal("invalid hidden storage exposed")
				}
			}
		})
	}
}

func TestCatanEventSessionNaturalGames(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := referenceEventGame(t, n, false)
			for step := 0; step < 8000 && !s.Finished; step++ {
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatalf("step %d %s: %v", step, s.Phase, err)
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step %d %s %+v: %v", step, s.Phase, a, err)
				}
				if step%37 == 0 || s.Catan.CardEvent != nil {
					s = referenceEventRestore(t, s)
				}
				for c, total := range s.Catan.Bank {
					if total < 0 {
						t.Fatal("negative bank")
					}
					for _, p := range s.Catan.Players {
						if p.Resources[c] < 0 {
							t.Fatal("negative hand")
						}
						total += p.Resources[c]
					}
					want := 19
					if n > 4 {
						want = 24
					}
					if total != want {
						t.Fatal("resource conservation", c, total)
					}
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
				t.Fatal("natural game did not finish", s.Round)
			}
			if s.Catan.RollID == 0 {
				t.Fatal("game never drew a card")
			}
			t.Logf("%d players: %d draws / %d deck cycles", n, s.Catan.RollID, s.Catan.EventDeck.Deck.Cycle)
		})
	}
}
