package game

import (
	"reflect"
	"testing"
)

func gemCityTest(t *testing.T) *State {
	s := gemExpandedTest(t, SplendorOptions{})
	s.Splendor.Options.Cities = true
	s.Splendor.Cities = []GemCity{{Tile: 1, Name: "测试城市", Points: 14, Cost: [5]int{0, 4, 0, 0, 0}, Any: 4}}
	s.Splendor.Nobles = nil
	return s
}
func gemCityCards(s *State, player int, color, count int) {
	for i := 0; i < count; i++ {
		gemOwnFixture(s, player, orientFixture(1000+player*100+color*10+i, 1, color, ""))
	}
}
func TestSplendorCityPhysicalCardsAndDistinctAnyColor(t *testing.T) {
	s := gemCityTest(t)
	gemCityCards(s, 0, 1, 8)
	p := &s.Splendor.Players[0]
	p.Score = 14
	city := s.Splendor.Cities[0]
	if gemCityEligible(*p, city) {
		t.Fatal("same color fulfilled both requirements")
	}
	for i := 0; i < 2; i++ {
		gemOwnFixture(s, 0, orientFixture(2000+i, 2, 0, GemOrientDouble))
	}
	if gemCityEligible(*p, city) {
		t.Fatal("bonuses treated as cards")
	}
	gemCityCards(s, 0, 0, 2)
	if !gemCityEligible(*p, city) {
		t.Fatal("eligible city rejected")
	}
	p.Eliminated = true
	if gemCityEligible(*p, city) {
		t.Fatal("eliminated player eligible")
	}
}
func TestSplendorCityReplacesFifteenPointsAndNobles(t *testing.T) {
	s := gemCityTest(t)
	s.Splendor.Players[0].Score = 16
	s.Splendor.Nobles = []Noble{{ID: 1, Cost: make([]int, 5)}}
	s.gemAfter()
	if s.Splendor.LastRound || s.Finished || s.Turn != 1 || len(s.Splendor.Players[0].Nobles) != 0 {
		t.Fatal("base end/noble rules applied to cities")
	}
}
func TestSplendorCityFinalRoundOnlyEligiblePlayersAndTies(t *testing.T) {
	for _, tc := range []struct {
		name         string
		score, extra int
		want         []int
	}{
		{"only eligible", 13, 0, []int{0}},
		{"higher eligible score", 15, 0, []int{2}},
		{"fewer cards", 14, 1, []int{0}},
		{"shared", 14, 0, []int{0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := gemCityTest(t)
			for _, i := range []int{0, 2} {
				gemCityCards(s, i, 1, 4)
				gemCityCards(s, i, 0, 4)
			}
			s.Splendor.Players[0].Score = 14
			s.Splendor.Players[1].Score = 30 // This player fails the city requirements.
			s.Splendor.Players[2].Score = tc.score
			gemCityCards(s, 2, 2, tc.extra)
			s.gemAfter()
			if s.Finished || !s.Splendor.LastRound || s.Turn != 1 {
				t.Fatal("did not finish current round")
			}
			restored := clone(*s)
			s = &restored
			s.gemAfter()
			s.gemAfter()
			if !s.Finished || !reflect.DeepEqual(s.Winners, tc.want) || len(s.Splendor.Cities) != 1 {
				t.Fatal("incorrect eligible winner or city removed", s.Winners)
			}
		})
	}
}
func TestSplendorCityPostAcquiredBeforeEndCheck(t *testing.T) {
	s := gemCityTest(t)
	s.Splendor.Options.TradingPosts = true
	s.Splendor.Cities = []GemCity{{Name: "贸易城市", Points: 14, Any: 5}}
	gemCityCards(s, 0, 0, 5)
	s.Splendor.Players[0].Score = 13
	s.gemAfter()
	if !s.Splendor.LastRound || s.Splendor.Players[0].Score != 14 || !s.Splendor.Players[0].hasPost(GemPostPrestige) {
		t.Fatal("city checked before prestige post")
	}
}
func TestSplendorCityEliminatedTriggerDoesNotEndWithoutWinner(t *testing.T) {
	s := gemCityTest(t)
	gemCityCards(s, 0, 0, 4)
	gemCityCards(s, 0, 1, 4)
	s.Splendor.Players[0].Score = 14
	s.Splendor.LastRound = true
	// A trigger can have started before a persisted pending response/timeout.
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	s.gemAfter()
	s.gemAfter()
	if s.Finished || s.Splendor.LastRound || len(s.Winners) > 0 {
		t.Fatal("ended with no surviving city-eligible player")
	}
}
