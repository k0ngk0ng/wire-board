package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOriginalAICommandAttackSourceCalls(t *testing.T) {
	var f struct {
		Calls  []aiTeamModeCase
		Chains []struct{ Records []aiTeamModeCase }
	}
	readAITeamModeFixture(t, "ai-command-attack", &f)
	if len(f.Calls) != 6552 || len(f.Chains) != 36 {
		t.Fatal("missing command-attack source calls", len(f.Calls), len(f.Chains))
	}
	for _, r := range f.Calls {
		s := r.Before
		applyAITeamModeCase(t, r, &s)
	}
	for _, c := range f.Chains {
		if len(c.Records) != 2 {
			t.Fatal("missing original handoff")
		}
		s := c.Records[0].Before
		for _, r := range c.Records {
			applyAITeamModeCase(t, r, &s)
			data, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			var restored WorldInputState
			if e = json.Unmarshal(data, &restored); e != nil || restored != s {
				t.Fatal("JSON state", e)
			}
			s = restored
		}
		if c.Records[1].Before.Team.Players[c.Records[1].Actor].Mode != 7 {
			t.Fatal("mode7 was not consumed")
		}
	}
}

func TestOriginalAICommandAttackGuards(t *testing.T) {
	initial := AIControlDecision{Control: AIControlState{Mode: 36, Entered: true, TrackedObject: 6}, Random: 0x12345678, Action: ActionRun, ActionEntered: true}
	v := AIControlView{Actor: 0}
	for _, bad := range []AIControlDecision{initial, {Control: AIControlState{Mode: 15}, Random: 99}} {
		got := bad
		if e := advanceOriginalCommandAttack(&got, v); e == nil || !reflect.DeepEqual(got, bad) {
			t.Fatal("invalid source domain changed state", got, e)
		}
	}
	got := initial
	if e := selectOriginalCommandOpponent(&got, AIControlView{Actor: 4}); e == nil || !reflect.DeepEqual(got, initial) {
		t.Fatal("invalid actor", got, e)
	}
}
