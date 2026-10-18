package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type commandPassOracle struct {
	Direct                    bool
	Before                    AIControlState
	View                      AIControlView
	After                     AIControlDecision
	StatusBefore, StatusAfter [4]uint8
}

func commandPassRows(t *testing.T) ([]commandPassOracle, [][]commandPassOracle) {
	t.Helper()
	var f struct {
		Calls  []commandPassOracle
		Chains [][]commandPassOracle
	}
	readPropAttackFixture(t, "testdata/ai-command-pass.json.gz", &f)
	if len(f.Calls) != 6304 || len(f.Chains) != 4 {
		t.Fatal("incomplete original command pass oracle", len(f.Calls), len(f.Chains))
	}
	return f.Calls, f.Chains
}
func checkCommandPass(t *testing.T, i int, r commandPassOracle, s AIControlState, v AIControlView) AIControlDecision {
	t.Helper()
	result := AIControlDecision{Control: s, Action: v.Action, ActionEntered: v.ActionEntered, Heading: v.Heading, FacingLeft: v.FacingLeft, BindHoop: -1, Random: v.Random}
	var e error
	if r.Direct {
		e = advanceOriginalCommandPass(&result, v)
	} else {
		result, e = StepOriginalAIControl(s, v)
	}
	if e != nil {
		t.Fatal(i, "mode", s.Mode, "action", v.Action, "direct", r.Direct, e)
	}
	if !reflect.DeepEqual(result, r.After) {
		t.Fatalf("case%d mode%d action%x direct%v\ngot %+v\nwant %+v", i, s.Mode, v.Action, r.Direct, result, r.After)
	}
	statuses := r.StatusBefore
	if result.ClearCommandStatus {
		statuses[v.Actor] = 0
	}
	if statuses != r.StatusAfter {
		t.Fatal(i, "command status", statuses, r.StatusAfter)
	}
	return result
}
func TestOriginalCommandPassFunctions(t *testing.T) {
	rows, _ := commandPassRows(t)
	for i, r := range rows {
		if r.Direct {
			checkCommandPass(t, i, r, r.Before, r.View)
		}
	}
}
func TestOriginalCommandPassDispatcher(t *testing.T) {
	rows, _ := commandPassRows(t)
	for i, r := range rows {
		if !r.Direct {
			checkCommandPass(t, i, r, r.Before, r.View)
		}
	}
}
func TestOriginalCommandPassMode28ProducerChains(t *testing.T) {
	_, chains := commandPassRows(t)
	for _, chain := range chains {
		s, v := chain[0].Before, chain[0].View
		for i, r := range chain {
			got := checkCommandPass(t, i, r, s, v)
			data, e := json.Marshal(got)
			if e != nil {
				t.Fatal(e)
			}
			var restored AIControlDecision
			if e = json.Unmarshal(data, &restored); e != nil || !reflect.DeepEqual(got, restored) {
				t.Fatal("restore", e)
			}
			s = restored.Control
			v.Action, v.ActionEntered, v.Heading, v.FacingLeft, v.Random = restored.Action, restored.ActionEntered, restored.Heading, restored.FacingLeft, restored.Random
		}
		if s.Mode != 1 || v.Action != ActionPass || v.ActionEntered {
			t.Fatal("missing source producer transition", s, v.Action)
		}
	}
}
