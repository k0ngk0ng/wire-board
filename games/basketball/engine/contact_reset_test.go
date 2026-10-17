package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedContactReset struct {
	Rotation, StartingBallAction, Parity uint8
	Marks, Candidates, Links             [7]uint8
}

func (o observedContactReset) state() ContactPhaseState {
	s := ContactPhaseState{Rotation: o.Rotation, Marks: o.Marks, Links: o.Links, StartingBallAction: BallAction(o.StartingBallAction), Parity: o.Parity}
	for i, c := range o.Candidates {
		s.Candidates[i] = int8(c)
		if c >= 128 {
			s.Candidates[i] = -1
		}
	}
	return s
}
func TestOriginalContactResetPhase(t *testing.T) {
	b, err := os.ReadFile("testdata/contact-reset.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Ball          uint8
		Before, After observedContactReset
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1197 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		if got := BeginOriginalContacts(r.Before.state(), BallAction(r.Ball)); got != r.After.state() {
			t.Fatalf("frame %d got %+v want %+v", r.Frame, got, r.After.state())
		}
	}
	t.Log("matched all 1197 original contact preparation phases")
}
func TestContactResetPreservesLinksAndCounterWrap(t *testing.T) {
	s := ContactPhaseState{Rotation: 255, Links: [7]uint8{0, 0x40, 0x81, 0xc2, 0x30, 0x7f, 0xff}, Marks: [7]uint8{1, 2, 3, 4, 5, 6, 7}, Parity: 0x80}
	got := BeginOriginalContacts(s, BallAction(0x87))
	if got.Rotation != 0 || got.StartingBallAction != BallKnockback || got.Parity != 0x81 || got.Links != [7]uint8{0, 0, 0x81, 0x82, 0x30, 0x3f, 0xbf} || got.Marks != [7]uint8{} || got.Candidates != [7]int8{-1, -1, -1, -1, -1, -1, -1} {
		t.Fatal(got)
	}
	s.Rotation = 7
	if got := BeginOriginalContacts(s, BallHeld); got.Rotation != 8 {
		t.Fatal("counter prematurely masked", got)
	}
}
