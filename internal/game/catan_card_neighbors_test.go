package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func neighborGift(g *Catan, color int) Action {
	give := make([]int, len(g.Bank))
	give[color] = 1
	return Action{Type: "catan_event_gift", Give: give}
}

func TestCatanCardNeighborsSimultaneousRestorePrivacyAndProduction(t *testing.T) {
	s := cardEarthquakeFixture(t)
	for p := range 3 {
		cards := make([]int, 5)
		cards[p+1] = 1
		helperGrant(s, p, cards)
	}
	beginCardEvent(t, s, "good_neighbors", 6, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{1, 2, 0}) {
		t.Fatal("wrong neighbor response order")
	}
	initial := clone(*s).Catan.Players
	for step, actor := range []int{1, 2, 0} {
		for viewer := -1; viewer < 3; viewer++ {
			v := s.View(viewer)["catan"].(map[string]any)
			q := v["cardEvent"].(map[string]any)
			if _, exposed := q["gifts"]; exposed {
				t.Fatal("private gift selections exposed")
			}
			if gift, ok := q["ownGift"].(CatanEventGift); ok {
				if gift.From != viewer || gift.To != (viewer+1)%3 {
					t.Fatal("another player's selection exposed")
				}
			} else if viewer >= 0 {
				t.Fatal("missing own selection")
			}
			if (len(v["legal"].(map[string][]int)["eventGifts"]) == 1) != (viewer == actor) {
				t.Fatal("gift choices exposed to wrong viewer")
			}
		}
		helperApply(t, s, actor, neighborGift(s.Catan, actor+1))
		saved := clone(*s)
		if !reflect.DeepEqual(*s, saved) {
			t.Fatal("private selections not preserved on restore")
		}
		s = &saved
		if step < 2 && !reflect.DeepEqual(s.Catan.Players, initial) {
			t.Fatal("gift/production applied before all selections")
		}
	}
	for p := range 3 {
		want := make([]int, 5)
		want[0], want[(p+2)%3+1] = 1, 1
		if !slices.Equal(s.Catan.Players[p].Resources, want) {
			t.Fatal("wrong simultaneous gift/production", p, s.Catan.Players[p].Resources)
		}
	}
	if s.Phase != "catan_turn" || s.Turn != 1 || s.Catan.RollID != 1 || s.Catan.CardEvent != nil {
		t.Fatal("event continuation did not finish once")
	}
	catanCheck(t, s)
}

func TestCatanCardNeighborsEmptyHandsAndEliminatedSeat(t *testing.T) {
	for _, scenario := range []string{"empty", "one_giver", "eliminated"} {
		t.Run(scenario, func(t *testing.T) {
			s := cardEarthquakeFixture(t)
			if scenario != "empty" {
				helperGrant(s, 1, []int{0, 1, 0, 0, 0})
			}
			if scenario == "eliminated" {
				s.Catan.Players[2].Eliminated = true
			}
			beginCardEvent(t, s, "good_neighbors", 2, 0, 0)
			if scenario != "empty" {
				if !slices.Equal(s.Catan.CardEvent.Players, []int{1}) {
					t.Fatal("empty hand forced to give incoming card")
				}
				s.AutoCatanPending()
				recipient := 2
				if scenario == "eliminated" {
					recipient = 0
				}
				if s.Catan.Players[recipient].Resources[1] != 1 || sum(s.Catan.Players[1].Resources) != 0 {
					t.Fatal("gift went to wrong neighbor")
				}
			}
			if s.Phase != "catan_turn" || s.Catan.CardEvent != nil {
				t.Fatal("empty or partial gift event stalled")
			}
			catanCheck(t, s)
		})
	}
}

func TestCatanCardNeighborsInvalidChoicesAtomic(t *testing.T) {
	s := cardEarthquakeFixture(t)
	helperGrant(s, 1, []int{0, 1, 0, 0, 0})
	helperGrant(s, 0, []int{1, 0, 0, 0, 0})
	beginCardEvent(t, s, "good_neighbors", 2, 0, 0)
	for _, give := range [][]int{nil, {}, {0, 0, 0, 0, 0}, {0, 2, 0, 0, 0}, {1, 0, 0, 0, 0}, {-1, 2, 0, 0, 0}, {0, 0, 0, 0, 0, 1, 0, 0}} {
		helperReject(t, s, 1, Action{Type: "catan_event_gift", Give: give})
	}
	helperReject(t, s, 0, neighborGift(s.Catan, 0))
	a := neighborGift(s.Catan, 1)
	a.Choice = "skip"
	helperReject(t, s, 1, a)
	a.Choice, a.Type = "", "catan_event_resource"
	helperReject(t, s, 1, a)
	helperReject(t, s, 1, Action{Type: "catan_end"})
	if err := s.EliminateCatan(1); err == nil {
		t.Fatal("eliminated player with gifts pending")
	}
	helperApply(t, s, 1, neighborGift(s.Catan, 1))
	helperReject(t, s, 1, neighborGift(s.Catan, 1))
	// A failed continuation must roll back both the last selection and transfer.
	s.Catan.CardEvent.Red = 0
	s.Catan.CitiesKnights = &CatanCitiesKnights{}
	helperReject(t, s, 0, neighborGift(s.Catan, 0))
	if s.Catan.CardEvent.Gifts[1].Color != -1 || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[1].Resources[1] != 1 {
		t.Fatal("failed continuation partially transferred gifts")
	}
}

func TestCatanCardNeighborsCommoditiesAndBotPrivacy(t *testing.T) {
	s := ckEvent(t)
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 2, 0, 0})
	helperGrant(s, 1, []int{1, 0, 0, 0, 0, 0, 0, 0})
	beginCardEvent(t, s, "good_neighbors", 2, 6, 0)
	a, err := s.BotAction(0)
	if err != nil || !slices.Equal(a.Give, []int{0, 0, 0, 0, 0, 1, 0, 0}) {
		t.Fatal("commodity gift bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{0, 0, 0, 0, 0, 0, 0, 10}
	other.Catan.CardEvent.Gifts[1].Color = 7
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[0])
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("gift bot read hidden information")
	}
	left, _ := json.Marshal(s.View(0))
	// Same hand count; keep public counts identical when comparing views.
	other.Catan.Players[1].Resources[7] = 1
	right, _ := json.Marshal(other.View(0))
	if string(left) != string(right) {
		t.Fatal("view leaked another player's gift choice or hand color")
	}
	helperApply(t, s, 0, a)
	s.AutoCatanPending()
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || s.Catan.Players[1].Resources[5] != 1 || s.Catan.Players[2].Resources[0] != 1 {
		t.Fatal("commodity transfer failed")
	}
	ckSupply(t, s.Catan)
}

func TestCatanCardNeighborsSixSeats(t *testing.T) {
	s, err := NewCatan(6, CatanOptions{FiveSix: true})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep, g.TurnSerial = 12, 1
	s.Phase, s.Turn = "catan_roll", 4
	for p := range 6 {
		helperGrant(s, p, []int{1, 0, 0, 0, 0})
	}
	beginCardEvent(t, s, "good_neighbors", 2, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{4, 5, 0, 1, 2, 3}) {
		t.Fatal("six seat response ordering")
	}
	for range 6 {
		s.AutoCatanPending()
	}
	for _, p := range s.Catan.Players {
		if sum(p.Resources) != 1 {
			t.Fatal("six seat gift duplicated")
		}
	}
	if s.Turn != 4 || s.Phase != "catan_turn" || s.Catan.CardEvent != nil {
		t.Fatal("gift response advanced primary turn")
	}
	for color, stock := range s.Catan.Bank {
		for _, p := range s.Catan.Players {
			stock += p.Resources[color]
		}
		if stock != 24 {
			t.Fatal("six-player inventory not conserved", color, stock)
		}
	}
}
