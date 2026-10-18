package engine

import (
	"reflect"
	"testing"
)

type directShotAliasOracle struct {
	Direct  bool
	Plan    uint8
	Before  AIControlState
	View    AIControlView
	Context AIShotContext
	After   AIControlDecision
}

func TestOriginalShotPlansElevenThroughThirteen(t *testing.T) {
	var rows []directShotAliasOracle
	readPropAttackFixture(t, "testdata/ai-shot-plans11-13.json.gz", &rows)
	if len(rows) != 9376 {
		t.Fatal("missing original direct and dispatcher calls", len(rows))
	}
	for i, r := range rows {
		got := AIControlDecision{Control: r.Before, Action: r.View.Action, ActionEntered: r.View.ActionEntered, Heading: r.View.Heading, FacingLeft: r.View.FacingLeft, Random: r.View.Random, BindHoop: -1}
		if e := advanceOriginalShotPlan(&got, r.View, r.Context); e != nil {
			t.Fatal(i, r.Plan, r.Direct, e)
		}
		if !reflect.DeepEqual(got, r.After) {
			t.Fatalf("case%d plan%d direct%v actor%d action%x entered%v zone%x\ngot %+v\nwant %+v", i, r.Plan, r.Direct, r.View.Actor, r.View.Action, r.View.ActionEntered, r.View.CachedZones[r.View.Actor], got, r.After)
		}
	}
}
func TestOriginalShotWeightedSelectorCompleteDomain(t *testing.T) {
	var corpus struct {
		SelectorCalls []struct {
			Team, Region, RandomByte, Plan uint8
			Seed, AfterRandom              RandomState
		}
		SelectorPlans []uint8
	}
	readPropAttackFixture(t, "testdata/ai-shot-selector-audit.json.gz", &corpus)
	if len(corpus.SelectorCalls) != 4096 {
		t.Fatal("missing original chooser calls", len(corpus.SelectorCalls))
	}
	for i, r := range corpus.SelectorCalls {
		initial := AIControlDecision{Control: AIControlState{Mode: 6, PlanParameter: 13, Counter: 39, Plan: 9, TrackedObject: 2, Destination: 37}, Action: ActionDribbleRun, ActionEntered: true, Heading: HeadingUpRight, FacingLeft: true, Random: r.Seed, BindHoop: -1}
		got := initial
		zone := CourtZone(0x32)
		if r.Region != 0 {
			zone = 0x50
		}
		if e := chooseOriginalWeightedShotPlan(&got, AIControlView{Team: r.Team}, zone); e != nil {
			t.Fatal(i, e)
		}
		want := initial
		want.Control.PlanParameter = r.Plan
		want.Random = r.AfterRandom
		if !reflect.DeepEqual(got, want) {
			t.Fatal(i, "selector", got, want)
		}
		if got.Control.PlanParameter == 11 || got.Control.PlanParameter == 13 {
			t.Fatal("audited selector unexpectedly produces direct alias", i)
		}
	}
}
