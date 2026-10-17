package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type aiTeamModeCase struct {
	worldInputFixture
	Actor uint8
	Name  string
}

func readAITeamModeFixture(t *testing.T, name string, target any) {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(target); err != nil {
		t.Fatal(err)
	}
}
func applyAITeamModeCase(t *testing.T, r aiTeamModeCase, s *WorldInputState) {
	t.Helper()
	if *s != r.Before {
		t.Fatalf("%s input continuity: %s", r.Name, firstWorldLoopDifference(reflect.ValueOf(*s), reflect.ValueOf(r.Before), "state"))
	}
	if err := advanceWorldComputerInput(s, r.Actor, r.context(t)); err != nil {
		t.Fatal(r.Name, err)
	}
	if *s != r.After {
		t.Fatalf("%s: %s", r.Name, firstWorldLoopDifference(reflect.ValueOf(*s), reflect.ValueOf(r.After), "state"))
	}
}
func TestOriginalAITeamModeFunctions(t *testing.T) {
	for _, fixture := range []struct {
		Name  string
		Count int
	}{{"ai-mode33-isolated", 1152}, {"ai-mode34-isolated", 1600}, {"ai-mode35-isolated", 1920}} {
		t.Run(fixture.Name, func(t *testing.T) {
			var rows []aiTeamModeCase
			readAITeamModeFixture(t, fixture.Name, &rows)
			if len(rows) != fixture.Count {
				t.Fatal("missing original calls", len(rows))
			}
			for _, r := range rows {
				s := r.Before
				applyAITeamModeCase(t, r, &s)
			}
		})
	}
}
func TestOriginalAITeamSpecialSelectionChains(t *testing.T) {
	var rows []struct {
		Before, After TeamAIState
		Context       TeamAIContext
		Calls         []aiTeamModeCase
	}
	readAITeamModeFixture(t, "ai-team-special-selection-chains", &rows)
	if len(rows) != 224 {
		t.Fatal("missing original selections", len(rows))
	}
	entered := map[AIMode]int{}
	chains := 0
	for _, r := range rows {
		got, err := StepOriginalTeamAI(r.Before, r.Context)
		if err != nil || got.State != r.After {
			t.Fatal("team selection differs", got, r.After, err)
		}
		if len(r.Calls) == 0 {
			continue
		}
		chains++
		s := r.Calls[0].Before
		for _, c := range r.Calls {
			entered[c.Before.Team.Players[c.Actor].Mode]++
			applyAITeamModeCase(t, c, &s)
		}
	}
	if chains != 80 || entered[33] == 0 || entered[34] == 0 || entered[35] == 0 {
		t.Fatal("missing target-mode selection chains", chains, entered)
	}
}
func TestOriginalAIMode34LaterTeammateSeesCommand(t *testing.T) {
	var rows []aiTeamModeCase
	readAITeamModeFixture(t, "ai-mode34-teammate-order", &rows)
	if len(rows) != 2 {
		t.Fatal(len(rows))
	}
	s := rows[0].Before
	for _, r := range rows {
		applyAITeamModeCase(t, r, &s)
	}
	if rows[0].After.Team.Players[3].Mode != 28 || rows[1].Before.Team.Players[3].Mode != 28 {
		t.Fatal("teammate source chain lost")
	}
}
func TestOriginalAITeamModeSourcesAreLazyAndAtomic(t *testing.T) {
	initial := AIControlState{Mode: 34, Plan: 9}
	v := AIControlView{Actor: 0, Action: ActionDribbleRun, ActionEntered: true, Random: 0}
	got, err := StepOriginalAIControl(initial, v)
	if err == nil || got.Control != initial || got.Random != v.Random || got.TeammateControl.Change {
		t.Fatal("missing teammate mode source did not fail atomically", got, err)
	}
	v.Random = 0x12345678
	if got, err = StepOriginalAIControl(initial, v); err != nil || got.Control.Mode != 17 {
		t.Fatal("early RNG rejection required unread teammate source", got, err)
	}
	team := TeamAIState{}
	team.Players[1].Mode = 40
	v.Random = 0
	v.TeamAI = &team
	if got, err = StepOriginalAIControl(initial, v); err != nil || got.Control.Mode != 17 {
		t.Fatal("collector early return required unread world state", got, err)
	}
	team.Players[1].Mode = 2
	before := team
	world := WorldActionState{}
	v.PursuitWorld = &world
	v.Action = ActionCarrierJump
	if got, err = StepOriginalAIControl(initial, v); err == nil || team != before || got.Control != initial || got.TeammateControl.Change {
		t.Fatal("late profile failure mutated teammate", got, err)
	}
	v = AIControlView{Actor: 0, Action: ActionRun, Difficulty: 255, ShotContext: AIShotContext{Character: 0}}
	if got, err = StepOriginalAIControl(AIControlState{Mode: 35}, v); err != nil || got.Control.Mode != 2 || got.Random != 0 {
		t.Fatal("even-character early return touched difficulty/RNG", got, err)
	}
}
