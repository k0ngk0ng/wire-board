package game

import "testing"

// Two-player 商队＋新海岸 with helpers, events, friendly robber and harbors once
// ran past the shared step budget. Keep a bounded natural-game guard: every
// game must reach the victory target instead of stalling in a late phase.
func TestCatanTwoCaravansShoresLongGames(t *testing.T) {
	const runs = 4
	for run := 0; run < runs; run++ {
		s, err := NewCatanCaravansShoresSeafarers(2)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.EnableCatanTradersHelpers(true); err != nil {
			t.Fatal(err)
		}
		if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
		placed := false
		step := 0
		for ; step < 3000 && !s.Finished; step++ {
			p := ckActor(s)
			a, actionErr := s.BotAction(p)
			if actionErr != nil {
				t.Fatal(run, step, s.Phase, actionErr)
			}
			if a.Type == "catan_caravan_place" {
				placed = true
			}
			if actionErr = s.Apply(p, a); actionErr != nil {
				t.Fatal(run, step, s.Phase, a, actionErr)
			}
		}
		if !s.Finished {
			t.Fatalf("run %d unfinished after %d steps, round %d, phase %s", run, step, s.Round, s.Phase)
		}
		if !placed {
			t.Fatalf("run %d finished without any caravan placement", run)
		}
		best := 0
		for _, player := range s.Catan.Players {
			if player.Score > best {
				best = player.Score
			}
		}
		if best < s.Catan.victoryTarget() {
			t.Fatalf("run %d finished at %d points, target %d", run, best, s.Catan.victoryTarget())
		}
		t.Logf("run %d round %d steps %d score %d/%d", run, s.Round, step,
			s.Catan.Players[0].Score, s.Catan.Players[1].Score)
	}
}
