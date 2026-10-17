package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type mode5Plan11Case struct {
	worldInputFixture
	Actor uint8
	Name  string
}

func mode5Plan11Cases(t *testing.T, name string, count int) []mode5Plan11Case {
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
	var rows []mode5Plan11Case
	if err := json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != count {
		t.Fatal("missing original AI cases", name, len(rows))
	}
	return rows
}

func checkMode5Plan11Case(t *testing.T, r mode5Plan11Case) {
	t.Helper()
	got := r.Before
	if err := advanceWorldComputerInput(&got, r.Actor, r.context(t)); err != nil {
		t.Fatal(r.Name, err)
	}
	if got != r.After {
		t.Fatalf("%s: %s", r.Name, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(r.After), "state"))
	}
}

func TestOriginalAIMode5FunctionMatrix(t *testing.T) {
	for _, r := range mode5Plan11Cases(t, "ai-mode5-isolated", 1584) {
		checkMode5Plan11Case(t, r)
	}
}

func TestOriginalAIPlan11FunctionMatrix(t *testing.T) {
	for _, r := range mode5Plan11Cases(t, "ai-plan11-isolated", 1200) {
		checkMode5Plan11Case(t, r)
	}
}

func TestOriginalAITacticSelectionReachesMode5(t *testing.T) {
	var r struct {
		PressedRight                                         bool
		CursorBefore, CursorAfter, TacticBefore, TacticAfter uint8
		Before, After                                        TeamAIState
		Context                                              TeamAIContext
		Call                                                 mode5Plan11Case
	}
	raw, err := os.ReadFile("testdata/ai-mode5-selection-chain.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if !r.PressedRight || r.CursorBefore != 3 || r.CursorAfter != 4 || r.TacticBefore != 4 || r.TacticAfter != 5 || r.Call.Before.Team.Players[1].Mode != 5 {
		t.Fatal("original selection chain lost", r)
	}
	team, err := StepOriginalTeamAI(r.Before, r.Context)
	if err != nil || team.State != r.After {
		t.Fatal(team, r.After, err)
	}
	checkMode5Plan11Case(t, r.Call)
}

func TestOriginalAIForwardOpponentSelectsPlan11(t *testing.T) {
	rows := mode5Plan11Cases(t, "ai-plan11-entry-chain", 2)
	if rows[0].Before.Team.Players[2].Mode != 17 || rows[0].Before.World.Random != 0x12345678 || rows[0].After.Team.Players[2].Mode != 26 || rows[1].After.Team.Players[2].PlanParameter != 11 {
		t.Fatal("original obstacle/selection chain changed")
	}
	for _, r := range rows {
		checkMode5Plan11Case(t, r)
	}
}
