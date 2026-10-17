package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestAITeamSupportIsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-team-support-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Actor, Action, Link, Holder, Zone uint8
		Entered, ActionEntered, Human     bool
		After                             struct{ Mode, Action, Heading uint8 }
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 15360 {
		t.Fatal("incomplete isolated A95F matrix", len(rows))
	}
	for i, r := range rows {
		control := AIControlState{Mode: 28, Entered: r.Entered, Plan: 8, Counter: 7, PlanParameter: 6, Destination: 32, TrackedObject: 2}
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(r.Zone)
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), ActionEntered: r.ActionEntered, Heading: HeadingUpRight, TeamHasHuman: r.Human, TeammateLink: r.Link, BallHolder: int8(r.Holder), CachedZones: &zones, Random: 0x12345678}
		got, err := StepOriginalAIControl(control, v)
		if err != nil {
			t.Fatal(i, err)
		}
		want := AIControlDecision{Control: control, Action: Action(r.After.Action & 127), ActionEntered: r.After.Action&128 != 0, Heading: HeadingNone, BindHoop: -1, Random: v.Random}
		want.Control.Mode, want.Control.Entered = AIMode(r.After.Mode&127), r.After.Mode&128 != 0
		if r.After.Heading != 255 {
			want.Heading = Heading(r.After.Heading / 32)
		}
		if got != want {
			t.Fatalf("isolated A95F case%d %+v\ngot %+v\nwant %+v", i, r, got, want)
		}
	}
	t.Log("matched 15360 isolated original A95F cases; no natural-call or full-match claim")
}
