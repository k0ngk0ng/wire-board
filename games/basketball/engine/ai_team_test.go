package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedTeamAI struct {
	TeamModes, TeamCounters                                   [2]uint8
	Modes, Plans, Counters, Targets, Destinations, Parameters [4]uint8
	Teams, HumanTactics, HumanCommands, HumanStatus           [2]uint8
	HasHuman                                                  [2]bool
	Holder, PreviousHolder, ScoreDifference, Court            uint8
	Objects                                                   [7]Position
	Airborne                                                  [4]bool
	Attachments                                               [4]uint8
	Hoops                                                     [6]struct {
		Position      Position
		Action, Owner uint8
	}
	Random [4]byte
}

func (o observedTeamAI) state() TeamAIState {
	var s TeamAIState
	for i := range s.Teams {
		s.Teams[i] = TeamStrategyState{Mode: o.TeamModes[i] & 127, Entered: o.TeamModes[i]&128 != 0, Counter: int8(o.TeamCounters[i])}
	}
	for i := range s.Players {
		s.Players[i] = AIControlState{Mode: AIMode(o.Modes[i] & 127), Entered: o.Modes[i]&128 != 0, Plan: AIMode(o.Plans[i]), Counter: int8(o.Counters[i]), TrackedObject: int8(o.Targets[i]), Destination: CourtZone(o.Destinations[i]), PlanParameter: o.Parameters[i]}
	}
	return s
}

func (o observedTeamAI) context() TeamAIContext {
	v := TeamAIContext{Teams: o.Teams, HasHuman: o.HasHuman, HumanTactics: o.HumanTactics, HumanCommands: o.HumanCommands, Holder: o.Holder, PreviousHolder: o.PreviousHolder, ScoreDifference: o.ScoreDifference, CourtID: o.Court, Objects: o.Objects, Airborne: o.Airborne, Attachments: o.Attachments}
	for i := range v.HumanStatus {
		v.HumanStatus[i] = int8(o.HumanStatus[i])
	}
	for i, h := range o.Hoops {
		v.Hoops[i] = AIHoopView{Position: h.Position, Action: HoopAction(h.Action & 127), Owner: int8(h.Owner)}
	}
	return v
}

func TestOriginalCompleteTeamAIPhase(t *testing.T) {
	data, err := os.ReadFile("testdata/ai-team.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Branches      [2]uint8
		Before, After observedTeamAI
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1197 {
		t.Fatal("missing complete original phases", len(rows))
	}
	branches := map[[2]uint8]int{}
	for _, r := range rows {
		before, ctx := r.Before.state(), r.Before.context()
		got, err := StepOriginalTeamAI(before, ctx)
		if err != nil || got.State != r.After.state() {
			t.Fatalf("frame %d branches %v\nbefore %+v\ncontext %+v\ngot %+v\nwant %+v\nerror %v", r.Frame, r.Branches, before, ctx, got, r.After.state(), err)
		}
		if r.Before.context() != r.After.context() || r.Before.Random != r.After.Random {
			t.Fatalf("original phase unexpectedly mutated context or RNG frame %d", r.Frame)
		}
		branches[[2]uint8{r.Branches[0] & 127, r.Branches[1] & 127}]++
	}
	t.Logf("matched all %d complete original team-AI phases: %v", len(rows), branches)
}

func TestOriginalTeamAIRolesAndRefresh(t *testing.T) {
	initial := TeamAIState{Teams: [2]TeamStrategyState{{Mode: 12}, {Mode: 0}}}
	initial.Players[0] = AIControlState{Mode: AICollectHoop, Entered: true, Plan: 9}
	initial.Players[1] = AIControlState{Mode: 9, Entered: true, Plan: 17}
	ctx := TeamAIContext{Holder: 6, PreviousHolder: 6}
	ctx.Objects[0].X, ctx.Objects[1].X, ctx.Objects[6].X = 100*Pixel, 100*Pixel, 200*Pixel
	ctx.Airborne[1] = true
	got, err := StepOriginalTeamAI(initial, ctx)
	if err != nil || got.State.Players[0] != initial.Players[0] || got.State.Players[1].Mode != AIRecover || got.State.Players[1].Plan != 2 || got.State.Players[1].Entered || got.State.Teams[0].Counter != 48 || !got.State.Teams[0].Entered {
		t.Fatal(got, err)
	}
	// An airborne ball carrier without an attachment pauses the team refresh.
	ctx.Holder, ctx.PreviousHolder = 1, 1
	got.State.Teams[0].Mode = 10
	paused := got.State
	got, err = StepOriginalTeamAI(paused, ctx)
	if err != nil || got.State != paused {
		t.Fatal("airborne holder did not preserve strategy timer", got, err)
	}
	ctx.Attachments[1] = 0x80
	got, err = StepOriginalTeamAI(paused, ctx)
	if err != nil || got.State.Teams[0].Counter != 47 {
		t.Fatal("attached holder did not resume strategy timer", got, err)
	}
	paused.Teams[0].Counter = 0
	got, err = StepOriginalTeamAI(paused, ctx)
	if err != nil || got.State.Teams[0].Entered || got.State.Teams[0].Counter != -1 {
		t.Fatal("expired strategy did not clear entered bit", got, err)
	}
}

func TestOriginalTeamAIUnsupportedCommandIsAtomic(t *testing.T) {
	initial := TeamAIState{Teams: [2]TeamStrategyState{{Mode: 12}, {Mode: 13}}}
	ctx := TeamAIContext{Holder: 6, PreviousHolder: 6, HasHuman: [2]bool{false, true}, HumanCommands: [2]uint8{0, 1}}
	got, err := StepOriginalTeamAI(initial, ctx)
	if err == nil || got.State != initial {
		t.Fatal("unrestored cooperation must retain the complete input state", got, err)
	}
}
