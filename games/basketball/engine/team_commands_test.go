package engine

// Failure inventory before implementation: acknowledge/reject the wrong mode,
// ignore the NewYork upper-platform gate, assign a command to an injured actor,
// lose negative status or a possession-change cancellation, pick wrong tactic
// subrow, replay entry writes, drop message/window/accepted-command bytes, lose
// JSON restore equivalence, or mutate state when a required source is absent.
import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestOriginalTeamCommandPhases(t *testing.T) {
	var rows []struct {
		Label         string
		Before, After TeamAIStep
		Context       TeamAIContext
		DirectTeam    *uint8
	}
	readPropAttackFixture(t, "testdata/team-commands.json.gz", &rows)
	if len(rows) != 2456 {
		t.Fatal("incomplete original command phases", len(rows))
	}
	for _, r := range rows {
		var got TeamAIStep
		var e error
		if r.DirectTeam == nil {
			got, e = StepOriginalTeamAI(r.Before.State, r.Context)
		} else {
			got = r.Before
			e = advanceOriginalTeamCommand(&got, *r.DirectTeam, r.Context)
		}
		if e != nil {
			t.Fatal(r.Label, e)
		}
		if got != r.After {
			t.Fatal(r.Label, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(r.After), "team"))
		}
	}
}
func TestRuntimeHumanCommandNormalButtons(t *testing.T) {
	for tactic := uint8(0); tactic < 6; tactic++ {
		for _, button := range []Buttons{ButtonA, ButtonB} {
			t.Run(fmt.Sprintf("tactic%d-button%d", tactic, button), func(t *testing.T) {
				c := DefaultRuntimeConfig()
				c.Match.Seed = 0x12345678
				c.HumanTactics[0] = tactic
				s, e := NewRuntime(c)
				if e != nil {
					t.Fatal(e)
				}
				began, requested, responded := false, false, false
				heldVideos := 0
				accepted, rejected := false, false
				modes := map[AIMode]bool{}
				for frame := 0; frame < 1200; frame++ {
					var held [4]Buttons
					if !began && s.Playing != nil && s.Playing.World.Preparation.Actions.Holder != 0 {
						began = true
					}
					if began && heldVideos < 18 {
						held[0] = button
						heldVideos++
					}
					before, _ := json.Marshal(s)
					next, e := StepVideo(s, held)
					if e != nil {
						t.Fatalf("video%d: %v\nbefore=%s", s.VideoFrames, e, before)
					}
					if frame%30 == 0 {
						var restore RuntimeState
						if e = json.Unmarshal(before, &restore); e != nil {
							t.Fatal(e)
						}
						again, e := StepVideo(restore, held)
						if e != nil || !reflect.DeepEqual(next, again) {
							t.Fatal("JSON replay differs", frame, e)
						}
					}
					s = next.State
					if s.Playing != nil {
						h := s.Playing.World.Input.Human[0]
						requested = requested || h.Request != 0
						accepted = accepted || h.Messages[1] == 1
						rejected = rejected || h.Messages[1] == 2
						modes[s.Playing.World.Preparation.TeamAI.Players[1].Mode] = true
						responded = responded || h.Messages[1] != 0
					}
				}
				t.Logf("tactic=%d button=%d accepted=%t rejected=%t modes=%v finalHuman=%+v", tactic, button, accepted, rejected, modes, s.Playing.World.Input.Human[0])
				wantMode := AIMode(36)
				if button == ButtonB {
					wantMode = [6]AIMode{37, 37, 30, 13, 38, 37}[tactic]
				}
				if s.Playing.World.Input.Human[0].Status != 0 || s.Playing.World.Input.Human[0].Request != 0 {
					t.Fatal("command did not release its busy/request state")
				}
				if !accepted || !modes[wantMode] {
					t.Fatal("command did not enter original tactic mode", wantMode)
				}
				if tactic == 3 && button == ButtonB && (!modes[3] || !modes[14]) {
					t.Fatal("auxiliary command did not continue through pickup/wait/carry")
				}
				if !began || !requested || !responded {
					t.Fatal("ordinary controller route missed command lifecycle", began, requested, responded)
				}
			})
		}
	}
}

// Explicit full-match extension of the ordinary-input command paths above.
// Keep the original two-repeat/clock/JSON/trajectory gate; do not seed state.
func TestRuntimeHumanCommandCompleteMatches(t *testing.T) {
	if os.Getenv("BASKETBALL_COMMAND_FULL_MATCH") != "1" {
		t.Skip("explicit command full-match matrix")
	}
	for tactic := uint8(0); tactic < 6; tactic++ {
		for _, button := range []Buttons{ButtonA, ButtonB} {
			t.Run(fmt.Sprintf("tactic%d-button%d", tactic, button), func(t *testing.T) {
				c := DefaultRuntimeConfig()
				c.Match.Seed = 0x12345678
				c.HumanTactics[0] = tactic
				began := [2]bool{}
				heldVideos := [2]int{}
				verifyColdVideoRuntimeWithInput(t, c, true, func(s RuntimeState, repeat int) [4]Buttons {
					var held [4]Buttons
					if !began[repeat] && s.Playing != nil && s.Playing.World.Preparation.Actions.Holder != 0 {
						began[repeat] = true
					}
					if began[repeat] && heldVideos[repeat] < 18 {
						held[0] = button
						heldVideos[repeat]++
					}
					return held
				})
			})
		}
	}
}
