package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCatanCardConflictArmyHolderOnAnotherPlayersTurn(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.ArmyOwner = 0
	g.Players[0].Knights, g.Players[1].Knights = 3, 3
	helperGrant(s, 2, []int{0, 2, 0, 0, 0})
	g.FriendlyRobber = &CatanFriendlyRobber{Rules: CatanFriendlyRobberRules}
	beginCardEvent(t, s, "conflict", 6, 0, 0)
	if s.CatanPendingActor() != 0 || s.Catan.CardEvent.Optional || s.Turn != 1 {
		t.Fatal("army holder did not act independently of production turn")
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		want := []int{}
		if viewer == 0 {
			want = []int{2}
		}
		if !slices.Equal(v["legal"].(map[string][]int)["eventTargets"], want) || v["cardEvent"].(map[string]any)["canSkip"] != false {
			t.Fatal("wrong conflict legal targets or optionality")
		}
	}
	helperReject(t, s, 0, Action{Type: "catan_event_skip"})
	helperReject(t, s, 1, Action{Type: "catan_event_steal", Target: 2})
	helperApply(t, s, 0, Action{Type: "catan_event_steal", Target: 2})
	if s.Catan.Robber != -1 || len(s.Catan.Victims) != 0 || s.Catan.Players[0].Resources[1] != 1 || s.Catan.Players[2].Resources[1] != 1 {
		t.Fatal("card theft used robber protection or moved robber")
	}
	for _, p := range s.Catan.Players {
		if p.Resources[0] != 1 {
			t.Fatal("production missing or repeated after conflict")
		}
	}
	if s.Phase != "catan_turn" || s.Turn != 1 || s.Catan.CardEvent != nil || s.Catan.RollID != 1 {
		t.Fatal("conflict changed active turn or failed to continue")
	}
	catanCheck(t, s)
}

func TestCatanCardConflictUnawardedLeaderCanDeclineAfterRestore(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Players[2].Knights = 2
	for range 4 {
		catanCard(s.Catan, 1, 0) // Hidden knights cannot take away eligibility.
	}
	helperGrant(s, 0, []int{0, 1, 0, 0, 0})
	beginCardEvent(t, s, "conflict", 2, 0, 0)
	if s.CatanPendingActor() != 2 || !s.Catan.CardEvent.Optional {
		t.Fatal("unique fallback leader missing optional response")
	}
	saved := clone(*s)
	if !reflect.DeepEqual(*s, saved) {
		t.Fatal("fallback optionality changed on restore")
	}
	s = &saved
	for viewer := -1; viewer < 3; viewer++ {
		q := s.View(viewer)["catan"].(map[string]any)["cardEvent"].(map[string]any)
		if q["canSkip"] != (viewer == 2) {
			t.Fatal("skip hint leaked to wrong viewer")
		}
	}
	helperApply(t, s, 2, Action{Type: "catan_event_skip"})
	if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[1] != 1 || sum(s.Catan.Players[2].Resources) != 0 {
		t.Fatal("declining conflict moved cards or stalled")
	}
	catanCheck(t, s)
}

func TestCatanCardConflictNoEligibleTheftContinues(t *testing.T) {
	for _, variant := range []string{"all_zero", "tied", "eliminated", "empty_opponents"} {
		s := cardEarthquakeFixture(t)
		switch variant {
		case "tied":
			s.Catan.Players[0].Knights, s.Catan.Players[1].Knights = 2, 2
		case "eliminated":
			s.Catan.ArmyOwner = 2
			s.Catan.Players[2].Knights, s.Catan.Players[2].Eliminated = 4, true
		case "empty_opponents":
			s.Catan.ArmyOwner = 0
			s.Catan.Players[0].Knights = 3
			helperGrant(s, 0, []int{0, 1, 0, 0, 0})
		}
		beginCardEvent(t, s, "conflict", 6, 0, 0)
		if s.Phase != "catan_turn" || s.Catan.CardEvent != nil || s.CatanPendingActor() != -1 {
			t.Fatal("conflict stalled without eligible theft", variant)
		}
		// Cards acquired in the subsequent production cannot become a theft
		// target retroactively when every opponent was empty at event time.
		if s.Catan.Players[1].Resources[0] != 1 || s.Catan.Players[0].Resources[0] != 1 {
			t.Fatal("conflict ran again after production", variant)
		}
	}
}

func TestCatanCardConflictCityStrengthAndCommodityBeforeFirstInvasion(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	s.Turn = 2
	g.CitiesKnights.Knights = []CatanKnight{
		{Owner: 0, Vertex: 0, Strength: 2, Active: true},
		{Owner: 1, Vertex: 1, Strength: 1, Active: true},
		{Owner: 2, Vertex: 2, Strength: 3, Active: false},
	}
	g.ArmyOwner = 2 // No Largest Army award in C&K.
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 0, 2, 0})
	beginCardEvent(t, s, "conflict", 2, 6, 0)
	if s.CatanPendingActor() != 0 || s.Catan.CardEvent.Optional {
		t.Fatal("wrong active strength winner or optional C&K theft")
	}
	helperReject(t, s, 0, Action{Type: "catan_event_skip"})
	saved := clone(*s)
	s = &saved
	s.AutoCatanPending()
	if s.Catan.Players[0].Resources[6] != 1 || s.Catan.Players[1].Resources[6] != 1 || s.Catan.Robber != -1 || s.Catan.CitiesKnights.Invasions != 0 || s.Phase != "catan_turn" {
		t.Fatal("commodity theft incorrectly blocked by sleeping robber")
	}
	ckSupply(t, s.Catan)
	// Same strength from two active basic knights is a tie: no theft.
	s = ckEvent(t)
	s.Catan.CitiesKnights.Knights = []CatanKnight{
		{Owner: 0, Vertex: 0, Strength: 2, Active: true},
		{Owner: 1, Vertex: 1, Strength: 1, Active: true},
		{Owner: 1, Vertex: 2, Strength: 1, Active: true},
	}
	helperGrant(s, 2, []int{1, 0, 0, 0, 0, 0, 0, 0})
	beginCardEvent(t, s, "conflict", 2, 6, 0)
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || s.Catan.Players[2].Resources[0] != 1 {
		t.Fatal("tied C&K strength incorrectly stole a card")
	}
}

func TestCatanCardConflictInvalidTargetsAndRollback(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Players[1].Knights = 1
	helperGrant(s, 0, []int{0, 1, 0, 0, 0})
	beginCardEvent(t, s, "conflict", 2, 0, 0)
	for _, target := range []int{-1, 1, 2, 3} {
		helperReject(t, s, 1, Action{Type: "catan_event_steal", Target: target})
	}
	for _, action := range []Action{
		{Type: "catan_steal", Target: 0},
		{Type: "catan_event_steal", Target: 0, Choice: "skip"},
		{Type: "catan_event_steal", Target: 0, Take: []int{0, 1, 0, 0, 0}},
		{Type: "catan_event_steal", Target: 0, Cards: []int{1}},
	} {
		helperReject(t, s, 1, action)
	}
	s.Catan.Players[0].Eliminated = true
	helperReject(t, s, 1, Action{Type: "catan_event_steal", Target: 0})
	s.Catan.Players[0].Eliminated = false
	s.Catan.CitiesKnights = &CatanCitiesKnights{}
	// Invalid city continuation must also roll back the random theft.
	helperReject(t, s, 1, Action{Type: "catan_event_steal", Target: 0})
	if s.Catan.Players[0].Resources[1] != 1 || sum(s.Catan.Players[1].Resources) != 0 {
		t.Fatal("failed continuation partially stole a card")
	}
}

func TestCatanCardConflictBotAndSpectatorCannotReadHiddenColors(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.ArmyOwner = 1
	s.Catan.Players[1].Knights = 3
	helperGrant(s, 0, []int{0, 2, 0, 0, 0})
	helperGrant(s, 2, []int{0, 0, 0, 2, 0})
	s.catanScores()
	beginCardEvent(t, s, "conflict", 2, 0, 0)
	other := clone(*s)
	// Exchange concealed colors between opponents, conserving public supply.
	other.Catan.Players[0].Resources, other.Catan.Players[2].Resources = other.Catan.Players[2].Resources, other.Catan.Players[0].Resources
	// A hidden VP changes total score but not public score or bot preference.
	catanCard(other.Catan, 2, 4)
	other.Catan.Players[2].Score++
	slices.Reverse(other.Catan.DevDeck)
	a, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("theft bot inspected hidden colors or VP cards")
	}
	// Compare spectator views with only resource colors changed, before and
	// after stealing: public counts/logs must not identify the drawn color.
	other = clone(*s)
	other.Catan.Players[0].Resources, other.Catan.Players[2].Resources = other.Catan.Players[2].Resources, other.Catan.Players[0].Resources
	for step := 0; step < 2; step++ {
		left, _ := json.Marshal(s.View(-1))
		right, _ := json.Marshal(other.View(-1))
		if string(left) != string(right) {
			t.Fatal("spectator can distinguish private stolen color")
		}
		if step == 0 {
			helperApply(t, s, 1, Action{Type: "catan_event_steal", Target: 0})
			helperApply(t, &other, 1, Action{Type: "catan_event_steal", Target: 0})
		}
	}
	for _, log := range s.Log {
		if strings.Contains(log, "随机偷取") && (strings.Contains(log, "砖") || strings.Contains(log, "麦")) {
			t.Fatal("stolen color leaked through public log")
		}
	}
	catanCheck(t, s)
	catanCheck(t, &other)
}

func TestCatanCardConflictTheftThenGoldChoicesRestore(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Seafarers = &CatanSeafarers{Pirate: -1}
	s.Catan.Tiles[0].Resource = CatanGold
	s.Catan.Players[2].Knights = 1
	helperGrant(s, 0, []int{0, 1, 0, 0, 0})
	beginCardEvent(t, s, "conflict", 6, 0, 0)
	helperApply(t, s, 2, Action{Type: "catan_event_steal", Target: 0})
	if s.Phase != "catan_gold" || s.Catan.CardEvent != nil || s.Catan.Players[2].Resources[1] != 1 {
		t.Fatal("gold did not follow theft")
	}
	saved := clone(*s)
	s = &saved
	for range 3 {
		s.AutoCatanPending()
	}
	if s.Phase != "catan_turn" || sum(s.Catan.Players[2].Resources) != 2 || sum(s.Catan.Players[0].Resources) != 1 || s.Catan.RollID != 1 {
		t.Fatal("restored gold repeated or lost theft")
	}
	catanCheck(t, s)
}

func TestCatanCardConflictBeforeBarbarianDeactivationAndProduction(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 2, 2})
	s.Phase, s.Turn = "catan_roll", 2
	g := s.Catan
	g.Tiles[0].Vertices = []int{0, 3, 6}
	for p, v := range []int{0, 3, 6} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
	}
	g.CitiesKnights.Knights = []CatanKnight{
		{Owner: 0, Vertex: 1, Strength: 2, Active: true},
		{Owner: 1, Vertex: 4, Strength: 1, Active: true},
	}
	g.CitiesKnights.BarbarianPosition, g.CitiesKnights.RobberStart = 6, -1
	helperGrant(s, 2, []int{0, 0, 0, 0, 0, 0, 0, 1})
	beginCardEvent(t, s, "conflict", 6, 6, 3)
	if s.CatanPendingActor() != 0 || s.Catan.CitiesKnights.Invasions != 0 || s.Catan.CitiesKnights.BarbarianPosition != 6 {
		t.Fatal("barbarians changed conflict eligibility before theft")
	}
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 0, Action{Type: "catan_event_steal", Target: 2})
	g = s.Catan
	if g.CitiesKnights.Invasions != 1 || g.CitiesKnights.Players[0].DefenderPoints != 1 || g.Players[0].Resources[7] != 1 || g.Players[2].Resources[7] != 0 {
		t.Fatal("theft or subsequent barbarian reward missing")
	}
	for _, knight := range g.CitiesKnights.Knights {
		if knight.Active {
			t.Fatal("knights not deactivated after conflict and invasion")
		}
	}
	for _, p := range g.Players {
		if p.Resources[0] != 1 || p.Resources[5] != 1 {
			t.Fatal("production did not follow conflict and barbarians")
		}
	}
	if s.Phase != "catan_turn" || s.Turn != 2 || g.CardEvent != nil || g.CitiesKnights.Event != nil || g.RollID != 1 {
		t.Fatal("conflict/barbarians advanced production turn incorrectly")
	}
	ckSupply(t, g)
}
