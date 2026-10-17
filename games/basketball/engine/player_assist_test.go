package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Source failures: treating09 as hoop carrying, wrong friction clamp/depth
// crossing, gravity added before motion, lost linked-target launch, and clearing
// the linked actor on continuation instead of only0B entry. Carried-body09 must
// remain explicit until its paired movement is recovered.
func TestOriginalOrdinaryPlayerAssist(t *testing.T) {
	f, e := os.Open("testdata/player-assist.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Name          string
		Actor         uint8
		Before, After struct {
			observedAttack
			LinkedActor uint8
		}
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil || len(rows) != 576 {
		t.Fatal(e, len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		initial := PlayerAssistState{Player: playerFromObservation(b.observedPlayer), LinkedActor: int8(b.LinkedActor)}
		got, e := StepPlayerAssist(initial, PlayerAssistContext{Computer: b.Control&128 != 0, HasBall: b.Holder == int32(r.Actor)}, NewYorkGroundCourt())
		want := PlayerAssistState{Player: playerFromObservation(a.observedPlayer), LinkedActor: int8(a.LinkedActor)}
		if e != nil || got.State != want {
			t.Fatalf("%s before=%+v got=%+v want=%+v err=%v", r.Name, initial, got, want, e)
		}
		actions := b.Actions
		if got.ChangeTarget {
			actions[got.Target] = uint8(got.TargetAction)
		}
		actions[r.Actor] = a.Actions[r.Actor]
		if actions != a.Actions || b.Owners != a.Owners || got.Ball.Change || got.Ball.ChangeRequest {
			t.Fatal("unrepresented mutation", r.Name, got)
		}
	}
	t.Logf("matched %d ordinary09/0B source calls", len(rows))
}
func TestPlayerAssistRejectsUnknownPartnerAndCarriedBody(t *testing.T) {
	for _, c := range []struct {
		state PlayerAssistState
		ctx   PlayerAssistContext
	}{
		{PlayerAssistState{Player: Player{Action: 9}}, PlayerAssistContext{CarriedBody: true}},
		{PlayerAssistState{Player: Player{Action: 11}, LinkedActor: 4}, PlayerAssistContext{}},
	} {
		got, e := StepPlayerAssist(c.state, c.ctx, NewYorkGroundCourt())
		if e == nil || got.State != c.state {
			t.Fatal(got, e)
		}
	}
}
