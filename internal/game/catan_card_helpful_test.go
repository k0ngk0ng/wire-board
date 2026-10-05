package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func helpfulFixture(t *testing.T) *State {
	s := cardEarthquakeFixture(t)
	s.Catan.Vertices[0].Level, s.Catan.Vertices[3].Level = 2, 2
	s.catanScores()
	helperGrant(s, 0, []int{0, 1, 0, 0, 0})
	helperGrant(s, 1, []int{0, 0, 1, 0, 0})
	return s
}

func TestCatanCardHelpfulTiedLeadersEachGiveOneThenProduce(t *testing.T) {
	s := helpfulFixture(t)
	beginCardEvent(t, s, "helpful_neighbor", 6, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{1, 0}) || !slices.Equal(s.Catan.CardEvent.Targets, []int{2}) {
		t.Fatal("wrong helpful leader/recipient queue")
	}
	for step, actor := range []int{1, 0} {
		for viewer := -1; viewer < 3; viewer++ {
			legal := s.View(viewer)["catan"].(map[string]any)["legal"].(map[string][]int)
			if (len(legal["eventTargets"]) == 1 && len(legal["eventGifts"]) == 1) != (viewer == actor) {
				t.Fatal("helpful choices exposed to wrong actor")
			}
		}
		a := neighborGift(s.Catan, actor+1)
		a.Target = 2
		helperApply(t, s, actor, a)
		saved := clone(*s)
		if !reflect.DeepEqual(*s, saved) {
			t.Fatal("helpful response/transfer lost on restore")
		}
		s = &saved
		if step == 0 && (s.Catan.Players[2].Resources[2] != 1 || s.Catan.Players[0].Resources[0] != 0) {
			t.Fatal("first gift not transferred, or production ran early")
		}
	}
	for p, want := range [][]int{{2, 0, 0, 0, 0}, {2, 0, 0, 0, 0}, {1, 1, 1, 0, 0}} {
		if !slices.Equal(s.Catan.Players[p].Resources, want) {
			t.Fatal("incorrect helpful gift/production", p, s.Catan.Players[p].Resources)
		}
	}
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || s.Turn != 1 || s.Catan.RollID != 1 {
		t.Fatal("helpful event did not finish on original turn")
	}
	catanCheck(t, s)
}

func TestCatanCardHelpfulChoosesOneLowerPlayerNotEveryPlayer(t *testing.T) {
	s := helpfulFixture(t)
	s.Catan.Vertices[3].Level = 1
	s.catanScores()
	beginCardEvent(t, s, "helpful_neighbor", 2, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{0}) || !slices.Equal(s.Catan.CardEvent.Targets, []int{1, 2}) {
		t.Fatal("single leader's lower-score targets")
	}
	a := neighborGift(s.Catan, 1)
	a.Target = 1
	helperApply(t, s, 0, a)
	if s.Catan.Players[1].Resources[1] != 1 || sum(s.Catan.Players[2].Resources) != 0 || s.Phase != "catan_turn" {
		t.Fatal("gave to more than one lower-score player")
	}
	catanCheck(t, s)
}

func TestCatanCardHelpfulEmptyHighestAllTiedAndEliminated(t *testing.T) {
	for _, variant := range []string{"empty_highest", "all_tied", "one_empty_leader", "eliminated_highest"} {
		s := helpfulFixture(t)
		switch variant {
		case "empty_highest":
			catanMove(s.Catan.Players[0].Resources, s.Catan.Bank, []int{0, 1, 0, 0, 0})
			s.Catan.Vertices[3].Level = 1
		case "all_tied":
			s.Catan.Vertices[6].Level = 2
		case "one_empty_leader":
			catanMove(s.Catan.Players[0].Resources, s.Catan.Bank, []int{0, 1, 0, 0, 0})
		case "eliminated_highest":
			s.Catan.Players[0].Eliminated = true
		}
		s.catanScores()
		beginCardEvent(t, s, "helpful_neighbor", 2, 0, 0)
		if variant == "one_empty_leader" || variant == "eliminated_highest" {
			if !slices.Equal(s.Catan.CardEvent.Players, []int{1}) || !slices.Equal(s.Catan.CardEvent.Targets, []int{2}) {
				t.Fatal("empty/eliminated highest player affected eligibility", variant)
			}
			s.AutoCatanPending()
			if sum(s.Catan.Players[2].Resources) != 1 {
				t.Fatal("sole able leader failed to give")
			}
		} else if sum(s.Catan.Players[2].Resources) != 0 {
			t.Fatal("all-tied or empty-highest event transferred a card")
		}
		if s.Catan.CardEvent != nil || s.Phase != "catan_turn" {
			t.Fatal("helpful event stalled without an actionable gift", variant)
		}
		catanCheck(t, s)
	}
}

func TestCatanCardHelpfulHiddenVictoryCardsDoNotChangePublicEligibility(t *testing.T) {
	s := helpfulFixture(t)
	other := clone(*s)
	// Preserve public development-card count while changing its hidden kind.
	catanCard(s.Catan, 2, 0)
	catanCard(other.Catan, 2, 4)
	s.catanScores()
	other.catanScores()
	beginCardEvent(t, s, "helpful_neighbor", 2, 0, 0)
	beginCardEvent(t, &other, "helpful_neighbor", 2, 0, 0)
	if !reflect.DeepEqual(s.Catan.CardEvent, other.Catan.CardEvent) {
		t.Fatal("hidden victory card changed public event queue/targets")
	}
	for _, viewer := range []int{-1, 0, 1} {
		left, _ := json.Marshal(s.View(viewer))
		right, _ := json.Marshal(other.View(viewer))
		if string(left) != string(right) {
			t.Fatal("event view leaks hidden victory point", viewer)
		}
	}
	a, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	other.Catan.Players[2].Resources = []int{9, 8, 7, 6, 5}
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("helpful bot used hidden recipient information")
	}
}

func TestCatanCardHelpfulInvalidActionsAndFinalContinuationAtomic(t *testing.T) {
	s := helpfulFixture(t)
	beginCardEvent(t, s, "helpful_neighbor", 2, 0, 0)
	a := neighborGift(s.Catan, 2)
	for _, target := range []int{-1, 0, 1, 3} {
		a.Target = target
		helperReject(t, s, 1, a)
	}
	a.Target = 2
	helperReject(t, s, 0, a)
	for _, give := range [][]int{nil, {0, 0, 0, 0, 0}, {0, 0, 2, 0, 0}, {0, 0, 0, 1, 0}, {0, 0, 0, 0, 0, 1, 0, 0}, {-1, 0, 2, 0, 0}} {
		a.Give = give
		helperReject(t, s, 1, a)
	}
	helperReject(t, s, 1, Action{Type: "catan_event_skip"})
	a = neighborGift(s.Catan, 2)
	a.Target = 2
	helperApply(t, s, 1, a)
	helperReject(t, s, 1, a)
	a = neighborGift(s.Catan, 1)
	a.Target = 2
	s.Catan.CitiesKnights = &CatanCitiesKnights{}
	helperReject(t, s, 0, a)
	if s.Catan.Players[0].Resources[1] != 1 || s.Catan.Players[2].Resources[1] != 0 || s.Catan.Players[2].Resources[2] != 1 {
		t.Fatal("failed final continuation did not roll back only current gift")
	}
}

func TestCatanCardHelpfulCityCommodityAndAqueduct(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
	g.CitiesKnights.Players[0].Improvements[CatanScience] = 3
	s.catanScores()
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 0, 1, 0})
	beginCardEvent(t, s, "helpful_neighbor", 2, 6, 0)
	if s.CatanPendingActor() != 0 || len(s.Catan.CardEvent.Targets) != 2 {
		t.Fatal("city helpful targets")
	}
	a := neighborGift(s.Catan, 6)
	a.Target = 2
	helperApply(t, s, 0, a)
	if s.Phase != "catan_aqueduct" || s.Catan.Players[2].Resources[6] != 1 || s.Catan.CardEvent != nil {
		t.Fatal("commodity gift or aqueduct continuation missing")
	}
	saved := clone(*s)
	s = &saved
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 1 || s.Catan.Players[2].Resources[6] != 1 {
		t.Fatal("restored aqueduct duplicated helpful gift")
	}
	ckSupply(t, s.Catan)
}
