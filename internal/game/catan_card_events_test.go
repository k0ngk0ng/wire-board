package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func cardEarthquakeFixture(t *testing.T) *State {
	t.Helper()
	s := earthquakeGraph(t, []int{0, 0, 1, 1, 2, 2})
	s.Turn, s.Phase = 1, "catan_roll"
	s.Catan.Robber = -1
	for p, v := range []int{0, 3, 6} {
		s.Catan.Vertices[v].Owner, s.Catan.Vertices[v].Level = p, 1
	}
	return s
}

func beginCardEvent(t *testing.T, s *State, kind string, total, red, face int) {
	t.Helper()
	if err := s.catanBeginCardEvent(kind, total, red, face); err != nil {
		t.Fatal(err)
	}
}

func rejectCardEvent(t *testing.T, s *State, kind string, total, red, face int) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.catanBeginCardEvent(kind, total, red, face); err == nil {
		t.Fatal("invalid card event accepted")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("invalid card event partially applied")
	}
}

func TestCatanCardEventEarthquakeQueuePrivacyRestoreAndProduction(t *testing.T) {
	s := cardEarthquakeFixture(t)
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	if s.Phase != "catan_card_event" || !slices.Equal(s.Catan.CardEvent.Players, []int{1, 2, 0}) || s.Turn != 1 || s.Catan.RollID != 1 {
		t.Fatal("queue must start at the active seat without changing turn")
	}
	for index, actor := range []int{1, 2, 0} {
		saved := clone(*s)
		if !reflect.DeepEqual(*s, saved) {
			t.Fatal("event state changed on restore")
		}
		s = &saved
		for _, p := range s.Catan.Players {
			if sum(p.Resources) != 0 {
				t.Fatal("resources produced before all road choices")
			}
		}
		for _, viewer := range []int{-1, 0, 1, 2} {
			v := s.View(viewer)["catan"].(map[string]any)
			legal := v["legal"].(map[string][]int)
			if (len(legal["earthquakeRoads"]) == 2) != (viewer == actor) || len(legal["roads"])+len(legal["repairRoads"])+len(legal["settlements"]) != 0 {
				t.Fatal("wrong event legal actions", viewer, legal)
			}
			for p, raw := range v["players"].([]any) {
				_, visible := raw.(map[string]any)["resources"]
				if visible != (viewer == p) {
					t.Fatal("event response leaked a hand")
				}
			}
		}
		helperReject(t, s, (actor+1)%3, Action{Type: "catan_earthquake", Edge: actor * 2})
		helperReject(t, s, actor, Action{Type: "catan_earthquake", Edge: (actor + 1) % 3 * 2})
		helperReject(t, s, actor, Action{Type: "catan_earthquake", Edge: actor * 2, Choice: "skip"})
		helperReject(t, s, actor, Action{Type: "catan_repair_road", Edge: actor * 2})
		helperReject(t, s, actor, Action{Type: "catan_end"})
		helperReject(t, s, 1, Action{Type: "catan_roll"})
		helperReject(t, s, actor, Action{Type: "catan_trade_accept"})
		if err := s.EliminateCatan(s.Turn); err == nil {
			t.Fatal("removal during mandatory choice")
		}
		if index == 1 {
			s.AutoCatanPending()
		} else {
			helperApply(t, s, actor, Action{Type: "catan_earthquake", Edge: actor * 2})
		}
		if s.Turn != 1 || s.Catan.RollID != 1 {
			t.Fatal("response rerolled or moved active turn")
		}
	}
	if s.CatanPendingActor() != -1 || s.Catan.CardEvent != nil || s.Phase != "catan_turn" || !slices.Equal(s.Catan.Dice, []int{0, 0}) {
		t.Fatal("event did not resume production without fabricated dice")
	}
	for p, hand := range s.Catan.Players {
		if hand.Resources[0] != 1 || len(s.Catan.earthquakeRoads(p)) != 1 {
			t.Fatal("must damage exactly one road and produce once", p)
		}
	}
	helperReject(t, s, 1, Action{Type: "catan_earthquake", Edge: 3})
	catanCheck(t, s)
}

func TestCatanCardEventSkipsNoRoadAndFullyDamagedSeats(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Edges[0].Damaged, s.Catan.Edges[1].Damaged = true, true
	s.Catan.Edges[2].Ship, s.Catan.Edges[3].Ship = true, true
	s.Catan.Players[2].Eliminated = true
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	if s.CatanPendingActor() != -1 || s.Catan.CardEvent != nil || s.Phase != "catan_turn" {
		t.Fatal("no damageable roads should not create an impossible response")
	}
	if s.Catan.Edges[2].Damaged || s.Catan.Edges[3].Damaged || s.Catan.Edges[4].Damaged || s.Catan.Players[2].Resources[0] != 0 {
		t.Fatal("ship or eliminated player affected")
	}
	for _, p := range []int{0, 1} {
		if s.Catan.Players[p].Resources[0] != 1 {
			t.Fatal("skipped event lost normal production")
		}
	}
}

func TestCatanCardEventEntryGuardsAndBotPrivacy(t *testing.T) {
	s := cardEarthquakeFixture(t)
	for _, tuple := range [][3]int{{1, 0, 0}, {13, 0, 0}, {6, 1, 0}, {6, 0, 1}} {
		rejectCardEvent(t, s, "earthquake", tuple[0], tuple[1], tuple[2])
	}
	rejectCardEvent(t, s, "unknown", 6, 0, 0)
	s.Phase = "catan_turn"
	rejectCardEvent(t, s, "earthquake", 6, 0, 0)
	s.Phase = "catan_roll"
	s.Catan.Options.Helpers = true
	rejectCardEvent(t, s, "earthquake", 6, 0, 0)
	s.Catan.Options.Helpers = false
	s.Catan.Seafarers = &CatanSeafarers{Pirate: -1, PirateIslands: &CatanPirateIslands{}}
	rejectCardEvent(t, s, "earthquake", 6, 0, 0)
	s.Catan.Seafarers = nil
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	rejectCardEvent(t, s, "earthquake", 6, 0, 0)
	if _, err := s.BotAction(0); err == nil {
		t.Fatal("wrong seat selected a road")
	}
	a, err := s.BotAction(1)
	if err != nil || a.Type != "catan_earthquake" {
		t.Fatal("earthquake bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[0].Resources = []int{9, 8, 7, 6, 5}
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("event bot inspected hidden data")
	}
}

func TestCatanCardEventSixPlayerQueueBeforePairedAction(t *testing.T) {
	s, err := NewCatan(6, CatanOptions{FiveSix: true})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	primary, secondary := s.Catan.Paired.Primary, s.Catan.Paired.Secondary
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	for offset := range 6 {
		if s.CatanPendingActor() != (primary+offset)%6 || s.Turn != primary || s.Catan.Paired.Second {
			t.Fatal("paired player took over before global event resolved")
		}
		saved := clone(*s)
		s = &saved
		s.AutoCatanPending()
	}
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || s.Turn != primary {
		t.Fatal("event failed to resume primary action")
	}
	for p := range 6 {
		if len(s.Catan.earthquakeRoads(p)) != 1 {
			t.Fatal("six-player event missed a seat")
		}
	}
	helperApply(t, s, primary, Action{Type: "catan_end"})
	if s.Turn != secondary || !s.Catan.Paired.Second || s.Phase != "catan_turn" || s.Catan.RollID != 1 {
		t.Fatal("paired secondary rerolled or lost normal action")
	}
}

func TestCatanCardEventCityDiceAfterRoadChoicesAndSavedNumber(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 2, 2})
	g := s.Catan
	s.Phase = "catan_roll"
	g.Tiles[0].Vertices = []int{0, 3, 6}
	for p, v := range []int{0, 3, 6} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
	}
	k := g.CitiesKnights
	k.RobberStart = -1
	k.BarbarianPosition = 6
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 1, Active: true}}
	// Red=2 intentionally differs from production=6. A yellow die is not rolled.
	beginCardEvent(t, s, "earthquake", 6, 2, 3)
	for player := range 3 {
		if s.Catan.CitiesKnights.BarbarianPosition != 6 || s.Catan.CitiesKnights.Event != nil {
			t.Fatal("C&K dice resolved before event-card text")
		}
		helperApply(t, s, player, Action{Type: "catan_earthquake", Edge: player * 2})
	}
	g, k = s.Catan, s.Catan.CitiesKnights
	if s.Phase != "catan_pillage" || s.CatanPendingActor() != 1 || k.Event.Production != 6 || k.Event.Red != 2 || k.Event.Yellow != 0 || g.RollID != 1 {
		t.Fatal("city event lost card number or required response")
	}
	for _, p := range g.Players {
		if sum(p.Resources) != 0 {
			t.Fatal("produced before barbarian resolution")
		}
	}
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_pillage", Vertex: 3})
	s.AutoCatanPending()
	g, k = s.Catan, s.Catan.CitiesKnights
	if s.Phase != "catan_turn" || g.CardEvent != nil || k.Event != nil || k.Pending != nil || k.Invasions != 1 || k.BarbarianPosition != 0 || g.RollID != 1 {
		t.Fatal("city production continuation did not finish")
	}
	for p, hand := range g.Players {
		if hand.Resources[0] != 1 || (p == 0 && hand.Resources[5] != 1) || (p != 0 && hand.Resources[5] != 0) {
			t.Fatal("wrong number or pre-pillage production used", p, hand.Resources)
		}
	}
	ckProgressStock(t, g)
	ckKnightStock(t, g)
}

func TestCatanCardEventRedDieDrawThenDiscardThenProduction(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 1, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Tiles[0].Vertices = []int{0}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.CitiesKnights.Players[1].Improvements[CatanScience] = 1
	ckProgressGive(t, s, 1, 3, 4, 5, 6)
	ckProgressTop(t, s, 0)
	for _, tuple := range [][2]int{{0, 0}, {7, 0}, {2, -1}, {2, 6}} {
		rejectCardEvent(t, s, "beautiful_day", 6, tuple[0], tuple[1])
	}
	beginCardEvent(t, s, "beautiful_day", 6, 2, CatanScience)
	if s.Phase != "catan_progress_discard" || s.CatanPendingActor() != 1 || s.Catan.CitiesKnights.Event.Production != 6 || len(s.Catan.CitiesKnights.Players[1].Progress) != 5 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("red die did not control progress draw separately from production")
	}
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3}})
	if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[0] != 1 || s.Catan.CitiesKnights.Event != nil {
		t.Fatal("progress discard lost card-number production")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanCardEventGoldAfterEarthquakeAndAtomicContinuationFailure(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Pirate: -1}
	g.Tiles[0].Resource = CatanGold
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	for range 3 {
		s.AutoCatanPending()
	}
	if s.Catan.CardEvent != nil || s.Phase != "catan_gold" || s.CatanPendingActor() != 1 {
		t.Fatal("gold production failed to follow earthquake")
	}
	for range 3 {
		s.AutoCatanPending()
	}
	if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 {
		t.Fatal("gold response pipeline stalled")
	}
	for _, p := range s.Catan.Players {
		if sum(p.Resources) != 1 {
			t.Fatal("gold reward count")
		}
	}
	catanCheck(t, s)
	// An invalid saved continuation must roll back the final road damage too.
	s = ckKnightGraph(t, []int{0})
	s.Phase = "catan_roll"
	beginCardEvent(t, s, "earthquake", 6, 2, 0)
	s.Catan.CardEvent.Red = 7
	helperReject(t, s, 0, Action{Type: "catan_earthquake", Edge: 0})
	if s.Catan.Edges[0].Damaged {
		t.Fatal("failed continuation retained damage")
	}
}
