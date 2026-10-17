package engine

import (
	"encoding/json"
	"os"
	"testing"
)

// Original contact response deliberately jumps over the first terminator into
// its hit follow-through. Reject missing tails, universal cursor acceptance,
// changing the world cursor, and tail endings resetting to the tail start.
func TestOriginalAttackContactAnimationTails(t *testing.T) {
	verifyOriginalAttackCalls(t, "testdata/attack-contact-tails.json.gz")
}
func TestOriginalAttackContactTailGeometry(t *testing.T) {
	var oracle struct {
		Geometry []struct {
			Variant, Frame uint8
			Shape          [8]uint8
		}
	}
	data, e := os.ReadFile("testdata/attack-contact-tails-source.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &oracle); e != nil {
		t.Fatal(e)
	}
	if len(oracle.Geometry) != 15 {
		t.Fatal("missing source calls")
	}
	for _, r := range oracle.Geometry {
		for kind := uint8(0); kind < 8; kind++ {
			for action := uint8(96); action < 104; action++ {
				allowed := false
				for ch := uint8(0); ch < 4; ch++ {
					allowed = allowed || originalAttackProfiles[action-96][kind*4+ch].Variant == r.Variant
				}
				got, e := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: action, Variant: r.Variant, Frame: r.Frame})
				if !allowed {
					if e == nil {
						t.Fatal("unselected tail accepted", kind, action, r)
					}
					continue
				}
				flags := originalAttackContactFlags[action-96]
				want := observedContactProfile(observedContactObject{Shape: r.Shape, AttackFlags: flags, ReceiveFlags: flags})
				if e != nil || got != want {
					t.Fatal(kind, action, r, got, want, e)
				}
			}
		}
	}
}
