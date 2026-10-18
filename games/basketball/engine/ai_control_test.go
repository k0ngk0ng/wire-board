package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedAIControl struct {
	Mode, NextMode, Counter, Target, Destination, Zone, Action, Request, Heading, Facing, Holder uint8
	Parameter                                                                                    uint8
	Random                                                                                       [4]byte
	Players                                                                                      [4]Position
	Ball                                                                                         Position
	X, Depth, Height                                                                             int32
	Hoops                                                                                        [6]struct {
		Action, Owner    uint8
		X, Depth, Height int32
	}
}

func aiControlFromObservation(r observedAIControl) AIControlState {
	return AIControlState{Mode: AIMode(r.Mode & 127), Entered: r.Mode&128 != 0, Plan: AIMode(r.NextMode), Counter: int8(r.Counter), TrackedObject: int8(r.Target), Destination: CourtZone(r.Destination), PlanParameter: r.Parameter}
}

func TestCourtZonesAgainstOriginal(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-zones.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor    uint8
		Frame    int
		X, Depth int32
		Zone     uint8
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4788 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		zone, err := CourtZoneForActor(Position{X: Fixed(r.X), Depth: Fixed(r.Depth)}, r.Actor)
		if err != nil || zone != CourtZone(r.Zone) {
			t.Fatalf("actor %d frame %d got %x want %x: %v", r.Actor, r.Frame, zone, r.Zone, err)
		}
	}
	t.Log("matched 4788 original team-relative court-zone calculations")
}

func TestOriginalAIControlAndHoopCollection(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-control.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After observedAIControl
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1341 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		heading := HeadingNone
		if r.Before.Heading != 255 {
			heading = Heading(r.Before.Heading / 32)
		}
		view := AIControlView{Actor: r.Actor, Action: Action(r.Before.Action & 127), ActionEntered: r.Before.Action&128 != 0, Heading: heading, FacingLeft: r.Before.Facing&128 != 0, Position: Position{X: Fixed(r.Before.X), Depth: Fixed(r.Before.Depth), Height: Fixed(r.Before.Height)}, HasBall: r.Before.Holder == r.Actor}
		view.Players, view.BallPosition, view.BallHolder, view.Random = r.Before.Players, r.Before.Ball, int8(r.Before.Holder), RandomState(binary.BigEndian.Uint32(r.Before.Random[:]))
		for i, h := range r.Before.Hoops {
			owner := int8(-1)
			if h.Owner < 4 {
				owner = int8(h.Owner)
			}
			view.Hoops[i] = AIHoopView{Action: HoopAction(h.Action & 127), Owner: owner, Position: Position{X: Fixed(h.X), Depth: Fixed(h.Depth), Height: Fixed(h.Height)}}
		}
		got, err := StepOriginalAIControl(aiControlFromObservation(r.Before), view)
		if err != nil {
			t.Fatalf("actor %d frame %d: %v", r.Actor, r.Frame, err)
		}
		wantHeading := HeadingNone
		if r.After.Heading != 255 {
			wantHeading = Heading(r.After.Heading / 32)
		}
		if got.Control != aiControlFromObservation(r.After) || got.Action != Action(r.After.Action&127) || got.ActionEntered != (r.After.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (r.After.Facing&128 != 0) {
			t.Fatalf("actor %d frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Actor, r.Frame, r.Before, got, r.After)
		}
		owners := [6]uint8{}
		wantOwners := [6]uint8{}
		for i, h := range r.Before.Hoops {
			owners[i] = h.Owner
			wantOwners[i] = r.After.Hoops[i].Owner
		}
		if got.BindHoop >= 0 {
			owners[got.BindHoop] = r.Actor
		}
		if owners != wantOwners || r.Before.Request != r.After.Request || r.Before.X != r.After.X || r.Before.Depth != r.After.Depth || r.Before.Height != r.After.Height {
			t.Fatal("AI changed unsupported world fields", r.Frame, got)
		}
		if r.Before.Mode&127 == 9 {
			if got.Random != RandomState(binary.BigEndian.Uint32(r.After.Random[:])) {
				t.Fatal("defence RNG consumption differs", r.Frame, got.Random, r.After.Random)
			}
		} else if got.Random != view.Random {
			t.Fatal("non-random strategy consumed randomness", r.Frame)
		}
	}
	t.Log("matched 1341 original recovery, defence, idle-wait, delay and hoop-collection AI calls")
}

func TestUnrestoredAIStrategyDoesNotInventActions(t *testing.T) {
	// Mode41 now has an original selector; mode15 remains an unresolved alias.
	s := AIControlState{Mode: 15}
	view := AIControlView{Action: ActionIdle, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}
	got, err := StepOriginalAIControl(s, view)
	if err == nil || got.Control != s || got.Action != view.Action || got.BindHoop != -1 {
		t.Fatal(got, err)
	}
}

func TestDefenceDoesNotInventMissingTacticalProfile(t *testing.T) {
	s := AIControlState{Mode: AIDefend, Entered: true}
	v := AIControlView{Actor: 2, Action: ActionRun, Position: Position{X: 450 * Pixel, Depth: 40 * Pixel}, BallHolder: 1}
	v.BallAction = BallDribble
	v.Players[1] = Position{X: 400 * Pixel, Depth: 40 * Pixel}
	got, err := StepOriginalAIControl(s, v)
	if err == nil || got.Control != s || got.Action != v.Action || got.BindHoop != -1 {
		t.Fatal(got, err)
	}
}
