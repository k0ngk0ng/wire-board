package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// This integration replay composes original reset, complete inputs and complete
// actions from one initial state. Source snapshots are expectations only. It
// checks ownership and physical pickup/carry through modes13->3->14->0, avoiding
// a synthetic action20 injection that would skip the original pickup animation.
func TestOriginalAuxiliaryCooperationPhaseChains(t *testing.T) {
	var chains []struct {
		worldInputFixture
		Actor, Selected uint8
		Records         []struct{ Before, AfterReset, AfterInput, After WorldInputState }
	}
	readPropAttackFixture(t, "testdata/ai-auxiliary-cooperation-chains.json.gz", &chains)
	if len(chains) != 8 {
		t.Fatal("missing original coupled phase chains", len(chains))
	}
	for ci, chain := range chains {
		s := chain.Records[0].Before
		c := chain.context(t)
		seen3, seen14 := false, false
		for i, r := range chain.Records {
			reset := BeginOriginalContacts(ContactPhaseState{Links: s.World.actionLinks()}, s.World.Ball.Action)
			s.World.setActionLinks(reset.Links)
			if s != r.AfterReset {
				t.Fatalf("chain%d tick%d reset: %s", ci, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.AfterReset), "input"))
			}
			input, e := StepWorldInputs(s, c)
			if e != nil {
				t.Fatal(ci, i, "inputs", e)
			}
			s = input.State
			if s != r.AfterInput {
				t.Fatalf("chain%d tick%d inputs: %s", ci, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.AfterInput), "input"))
			}
			seen3 = seen3 || s.Team.Players[chain.Actor].Mode == 3
			seen14 = seen14 || s.Team.Players[chain.Actor].Mode == 14
			actions, e := StepWorldActions(s.World, c.World)
			if e != nil {
				t.Fatal(ci, i, "actions", e)
			}
			s.World = actions.State
			if s != r.After {
				t.Fatalf("chain%d tick%d actions: %s", ci, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "input"))
			}
			data, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &s); e != nil {
				t.Fatal(e)
			}
		}
		if !seen3 || !seen14 || s.Team.Players[chain.Actor].Mode != 0 {
			t.Fatal("missing source mode transitions", ci)
		}
	}
}
