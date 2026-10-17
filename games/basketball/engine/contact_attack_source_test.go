package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestColdAttack93SelectionAndRequestBeforeGeometry(t *testing.T) {
	// Original E18B retains pending29 while action103 runs; bank6 A1F4's
	// team0/character1 selector pair is variant93/handler0, not stale metadata.
	p := Player{Action: Action(103), Request: ActionRequest{Action: 29, Pending: true}, Heading: HeadingRight, Body: Body{Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}}}
	accepted, err := ResolvePlayerRequest(p)
	if err != nil || accepted != p {
		t.Fatal("request29 incorrectly replaced attack103", accepted, err)
	}
	selected, err := StepPlayerAttack(AttackState{Player: accepted}, AttackContext{Team: 0, Character: 1, Computer: true}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || selected.State.AttackVariant != 93 || selected.State.Player.Action != 103 || !selected.State.Player.Entered || selected.State.Player.Pose.Frame != 40 {
		t.Fatal(selected, err)
	}
	for frame := uint8(0); frame < 3; frame++ {
		shape := [8]uint8{128, 32, 8, 248, 0, 32, 8, 248}
		if frame > 0 {
			shape = [8]uint8{8, 16, 20, 0, 0, 32, 8, 248}
		}
		want := observedContactProfile(observedContactObject{AttackFlags: 0x28, ReceiveFlags: 0x28, Shape: shape})
		got, err := OriginalContactProfile(ContactProfileKey{Kind: 0, Action: 103, Variant: 93, Frame: frame})
		if err != nil || got != want {
			t.Fatal("legal attack93 geometry", frame, got, want, err)
		}
	}
}

func TestAttackContactSourceMatchesIsolatedOriginalReader(t *testing.T) {
	f, err := os.Open("testdata/contact-attack-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Records []struct {
			Variant, Frame uint8
			Shape          [8]uint8
		}
		Flags    [10]uint8
		Selector struct {
			Action, Request, Variant uint8
			Handler                  uint16
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Selector.Action != 103 || oracle.Selector.Request != 29 || oracle.Selector.Variant != 93 || oracle.Selector.Handler != 0xa40a {
		t.Fatal("original selector audit differs", oracle.Selector)
	}
	seen := map[uint8]int{}
	for _, r := range oracle.Records {
		for kind := uint8(0); kind < 8; kind++ {
			for action := uint8(96); action < 106; action++ {
				flags := oracle.Flags[action-96]
				want := observedContactProfile(observedContactObject{Shape: r.Shape, AttackFlags: flags, ReceiveFlags: flags})
				got, err := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: action | 128, Variant: r.Variant, Frame: r.Frame})
				if err != nil || got != want {
					t.Fatalf("kind%d action%d variant%d frame%d: %+v want %+v %v", kind, action, r.Variant, r.Frame, got, want, err)
				}
			}
		}
		seen[r.Variant]++
	}
	if len(seen) != len(originalAttackAnimations) {
		t.Fatal("not every restored animation supplied source geometry", len(seen), len(originalAttackAnimations))
	}
	for variant, spec := range originalAttackAnimations {
		if seen[variant] != len(spec.Frames) {
			t.Fatal("animation/source frame count mismatch", variant, seen[variant], len(spec.Frames))
		}
		if _, err := OriginalContactProfile(ContactProfileKey{Kind: 0, Action: 103, Variant: variant, Frame: uint8(len(spec.Frames))}); err == nil {
			t.Fatal("shape lookup crossed animation domain", variant)
		}
	}
	if _, err := OriginalContactProfile(ContactProfileKey{Kind: 0, Action: 103, Variant: 255}); err == nil {
		t.Fatal("unknown variant accepted")
	}
	t.Logf("all %d original shape reads match across 8 player kinds and 10 attack flags; isolated function evidence", len(oracle.Records))
}
