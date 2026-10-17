package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedAuxiliary struct {
	observedPlayer
	Control    uint8
	ObjectLink uint8
	Poses      [13]uint8
}

func auxiliaryFromObservation(v observedAuxiliary) AuxiliaryState {
	p := playerFromObservation(v.observedPlayer)
	return AuxiliaryState{Body: p.Body, Action: uint8(p.Action), Entered: p.Entered, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne, FacingLeft: p.FacingLeft, Request: p.Request, NewYorkPassage: p.NewYorkPassage, Owner: int8(v.Control), Attachment: uint8(v.Attachment), ObjectLink: int8(v.ObjectLink)}
}
func TestOriginalAuxiliaryRestTrace(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After observedAuxiliary
	}
	raw, err := os.ReadFile("testdata/auxiliary-rest-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3996 {
		t.Fatal("missing auxiliary calls", len(rows))
	}
	counts := map[uint8]int{}
	for _, r := range rows {
		got, err := StepAuxiliaryRest(auxiliaryFromObservation(r.Before), AuxiliaryContext{Object: r.Actor, Court: 1})
		want := auxiliaryFromObservation(r.After)
		if err != nil || got.State != want {
			t.Fatalf("frame %d object %d\nbefore %+v\ngot %+v\nwant %+v err=%v", r.Frame, r.Actor, r.Before, got, want, err)
		}
		poses := r.Before.Poses
		poses[r.Actor] = got.State.Pose.Frame
		if got.State.Pose.Mirrored {
			poses[r.Actor] |= 128
		}
		if got.ChangeLinkedPose {
			poses[got.LinkedObject] = got.LinkedPose.Frame
			if got.LinkedPose.Mirrored {
				poses[got.LinkedObject] |= 128
			}
		}
		if poses != r.After.Poses {
			t.Fatal("unexpected linked pose effect", r.Frame, got)
		}
		counts[uint8(r.Before.Action&127)]++
	}
	t.Logf("matched %d original auxiliary calls: %v; no natural action12 calls", len(rows), counts)
}
func TestAuxiliaryRestBoundsAndEntry(t *testing.T) {
	for _, tc := range []struct{ x, want Fixed }{{48 * Pixel, 48 * Pixel}, {464 * Pixel, 464 * Pixel}, {32*Pixel + 123, 48*Pixel + 123}, {480*Pixel + 123, 479*Pixel + 123}} {
		s := AuxiliaryState{Body: Body{Position: Position{X: tc.x, Depth: 70 * Pixel}, Velocity: Velocity{X: Pixel, Depth: Pixel}}, Owner: -1, Animation: AnimationState{Elapsed: 32}}
		got, err := StepAuxiliaryRest(s, AuxiliaryContext{Object: 4, Court: 1})
		if err != nil || got.State.Body.Position.X != tc.want || got.State.Body.Position.Depth != 70*Pixel || got.State.Animation != s.Animation || !got.State.Entered {
			t.Fatal(got, err)
		}
	}
	s := AuxiliaryState{Body: Body{Position: Position{X: 48 * Pixel, Depth: 70 * Pixel}}, Owner: 7}
	got, err := StepAuxiliaryRest(s, AuxiliaryContext{Object: 4, Court: 1})
	if err != nil || !got.ChangeLinkedPose || got.LinkedObject != 7 || got.LinkedPose != (Pose{Frame: 2}) {
		t.Fatal(got, err)
	}
}
func TestAuxiliaryFallingOrderAndUnknownActions(t *testing.T) {
	s := AuxiliaryState{Action: 12, Airborne: true, Owner: -1, ObjectLink: 5, Body: Body{Position: Position{X: 48 * Pixel, Depth: 70 * Pixel}}}
	ctx := AuxiliaryContext{Object: 4, Court: 1}
	got, err := StepAuxiliaryRest(s, ctx)
	if err != nil || got.State.Action != 12 || got.State.Airborne || got.State.ObjectLink != -1 || got.State.Owner != -1 || !got.State.Entered {
		t.Fatal(got, err)
	}
	next, err := StepAuxiliaryRest(got.State, ctx)
	if err != nil || next.State.Action != 0 || next.State.Entered {
		t.Fatal(next, err)
	}
	s.Action = 0
	s.Entered = true
	s.Body.Position.Height = 20 * Pixel
	s.Owner = -1
	got, err = StepAuxiliaryRest(s, ctx)
	if err != nil || got.State.Action != 12 || got.State.Entered {
		t.Fatal(got, err)
	}
	s.Action = 8
	got, err = StepAuxiliaryRest(s, ctx)
	if err == nil || got.State != s {
		t.Fatal("unknown thrown action accepted", got, err)
	}
	s.Action = 0
	s.Attachment = 128
	got, err = StepAuxiliaryRest(s, ctx)
	if err == nil || got.State != s {
		t.Fatal("unknown attachment accepted", got, err)
	}
}
