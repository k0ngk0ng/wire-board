package engine

import "testing"

func TestOneVsThreeStatisticsAndTauntUseActualTeams(t *testing.T) {
	for _, winner := range []int16{1, 4} {
		state := MatchLifecycleState{Rule: 2, Grouping: 6, WinnerCode: winner, Characters: [4]uint8{6, 1, 2, 3}, Controllers: [4]uint8{1, 2, 3, 4}}
		for seat, team := range []uint8{1, 4, 4, 4} {
			state.Fighters[seat] = MatchFighter{Behavior: 0x12668, BaseTeam: team}
		}
		if err := recordMatchStatistics(&state); err != nil {
			t.Fatal(err)
		}
		for a := 0; a < 4; a++ {
			for b := 0; b < 4; b++ {
				want := uint8(0)
				if int16(state.Fighters[a].BaseTeam) == winner && int16(state.Fighters[b].BaseTeam) != winner {
					want = 1
				}
				if state.HeadToHead[a*4+b] != want {
					t.Fatal("statistics counted a teammate or missed an opponent", winner, a, b, state.HeadToHead)
				}
			}
		}
		state.ResultTicks = 384
		if winner == 4 {
			state.ResultTicks = 480
		}
		if err := maybeMatchTaunt(&state, nil); err != nil || state.SoundQueue[0] != 0x8f {
			t.Fatal("taunt duration used the wrong team", winner, err, state.SoundQueue)
		}
	}
}
