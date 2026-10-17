package basketball

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

// Boundary failures listed in PLAYER_MODES.md. These result fixtures test only
// projection and validation, not natural gameplay or original victory rules.
func TestPlayerModeResultProjection(t *testing.T) {
	for _, tc := range []struct {
		mode    string
		players int
	}{{"solo", 1}, {"cooperative", 2}, {"versus", 2}} {
		for winner := 0; winner < 2; winner++ {
			s := State{Options: Options{Court: "new-york", Mode: tc.mode}}
			s.Runtime.Outcome = &engine.RuntimeOutcome{WinningTeam: uint8(winner), Scores: [2]uint16{12, 30}, Confirmed: true}
			m, err := s.match()
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Validate(tc.players); err != nil {
				t.Fatalf("%s winner%d: %v", tc.mode, winner, err)
			}
			if !m.HasComputerPlayers {
				t.Fatal("fixed AI omitted from rating metadata")
			}
			want := []int{}
			if tc.mode == "versus" {
				want = []int{winner}
			} else if winner == 0 {
				for i := 0; i < tc.players; i++ {
					want = append(want, i)
				}
			}
			if !reflect.DeepEqual(m.Result.Winners, want) || m.Result.Cooperative != (tc.mode != "versus") {
				t.Fatal("wrong result", tc.mode, m.Result)
			}
			if tc.mode == "versus" && !reflect.DeepEqual(m.Result.Scores, []int{12, 30}) {
				t.Fatal("opponent score mapped to teammate")
			}
		}
	}
}
func TestPlayerModeContextAndSavedRosterValidation(t *testing.T) {
	ctx := gameapi.Context{Players: 2, Now: time.UnixMilli(1000000), Automated: []bool{false, false}}
	m, err := start(ctx, json.RawMessage(`{"mode":"versus"}`))
	if err != nil {
		t.Fatal(err)
	}
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.seatActors(), []int{0, 2}) {
		t.Fatal("wrong map")
	}
	for _, bad := range []gameapi.Context{
		{Players: 1, Now: ctx.Now, Actor: 0, Automated: []bool{false}},
		{Players: 2, Now: ctx.Now, Actor: 1, Automated: []bool{false}},
	} {
		bad.Now = ctx.Now.Add(time.Second)
		if _, err := onTimer(bad, m); err == nil {
			t.Fatal("mismatched context accepted")
		}
	}
	s.Inputs[1].Automated = false
	m.Data, _ = json.Marshal(s)
	if _, err := decode(m); err == nil {
		t.Fatal("saved fixed AI disabled")
	}
}
