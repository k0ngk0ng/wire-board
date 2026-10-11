package game

import "testing"

// The legacy 3v3 mode drafts sixteen generals, adds one maximum hit point to
// each leader, hands the action right to the camp leader and ends the moment a
// leader dies, with the whole camp credited for the win.
func TestSanguoshaThreeV3DraftAndVictory(t *testing.T) {
	for game := 0; game < 3; game++ {
		s, err := NewSanguosha(6, SGOptions{Mode: "3v3"})
		if err != nil {
			t.Fatal(err)
		}
		if three := s.Sanguosha.ThreeV3; three == nil || three.Rules != SGThreeV3Rules || len(three.Pool) != 16 || three.Stage != "pickFirst" {
			t.Fatalf("opening state %+v", three)
		}
		if s.Sanguosha.Players[s.Sanguosha.ThreeV3.Leaders[sgThreeCold]].Role != "" || len(s.Winners) != 0 {
			t.Fatal("3v3 seats must not carry identity roles")
		}
		steps := 0
		for ; !s.Finished && steps < 20000; steps++ {
			actor := s.SanguoshaActor()
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatalf("game %d step %d phase %s prompt %+v: %v", game, steps, s.Phase, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(actor, a); err != nil {
				t.Fatalf("game %d step %d action %+v: %v", game, steps, a, err)
			}
			if steps%2000 == 0 {
				cur := s.Sanguosha.ThreeV3
				t.Logf("game %d step %d stage %s side %d round %d alive %d", game, steps, cur.Stage, cur.Side, s.Round, len(s.sgOrder(0)))
			}
		}
		three := s.Sanguosha.ThreeV3
		if !s.Finished {
			t.Fatalf("game %d did not finish in %d steps (round %d stage %s)", game, steps, s.Round, three.Stage)
		}
		if len(three.Picked[sgThreeCold]) != 8 || len(three.Picked[sgThreeWarm]) != 8 {
			t.Fatal("draft sizes", len(three.Picked[sgThreeCold]), len(three.Picked[sgThreeWarm]))
		}
		for camp := sgThreeCold; camp <= sgThreeWarm; camp++ {
			if len(three.Assigned[camp]) != 3 {
				t.Fatal("assignment", camp, three.Assigned[camp])
			}
			leader := three.Leaders[camp]
			want := sgGeneral(s.Sanguosha.Players[leader].General).HP + 1
			if got := s.Sanguosha.Players[leader].MaxHP; got != want {
				t.Fatal("leader maximum hit points", camp, got, want)
			}
			for _, member := range s.sgThreeMembers(camp) {
				if s.Sanguosha.Players[member].General == "" {
					t.Fatal("seat without a general", camp, member)
				}
			}
		}
		if len(s.Winners) != 3 {
			t.Fatal("winning camp must be credited as a whole", s.Winners)
		}
		camp := s.sgThreeCamp(s.Winners[0])
		for _, w := range s.Winners {
			if s.sgThreeCamp(w) != camp {
				t.Fatal("mixed camps among the winners", s.Winners)
			}
		}
		alive := 0
		for _, w := range s.Winners {
			if s.sgAlive(w) {
				alive++
			}
		}
		if alive == 0 {
			t.Fatal("winning camp has no survivor", s.Winners)
		}
		loser := 1 - camp
		if !s.Sanguosha.Players[three.Leaders[loser]].Dead {
			t.Fatal("losing leader should have fallen")
		}
		if !s.Sanguosha.Players[three.Leaders[camp]].Dead {
			// A normal finish: the winner's leader survived.
			continue
		}
		// Both leaders fell, so the tiebreak compared survivors and losses.
		t.Logf("game %d ended with both leaders down: winners %v", game, s.Winners)
		_ = steps
	}
}
