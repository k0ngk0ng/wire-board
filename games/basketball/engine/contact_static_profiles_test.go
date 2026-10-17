package engine

import "testing"

func TestBallRollingContactUsesOriginalSourceTable(t *testing.T) {
	// bank5: kind8 map 8F38, action5 -> group4; 8FA5[4] -> 900F.
	// The 15 source entries 900F..902C all point at 9151, whose bytes are
	// 00 10 08 F8 00 10 08 F8. Both flags tables resolve to 938B[5]=20.
	want := ContactProfile{AttackFlags: 0x20, ReceiveFlags: 0x20, Shape: ContactShape{
		Attack: ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MinHeight: 0, MaxHeight: 16},
		Body:   ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MinHeight: 0, MaxHeight: 16},
	}}
	for frame := uint8(0); frame < 15; frame++ {
		for _, action := range []uint8{5, 0x85} {
			got, err := OriginalContactProfile(ContactProfileKey{Kind: 8, Action: action, Variant: 211, Frame: frame})
			if err != nil || got != want {
				t.Fatalf("rolling source frame %d: %+v %v", frame, got, err)
			}
		}
	}
	if _, err := OriginalContactProfile(ContactProfileKey{Kind: 8, Action: 5, Frame: 15}); err == nil {
		t.Fatal("rolling lookup crossed source array boundary")
	}
}

func TestNormalContactSourceFramesRemainSeparateFromAttackVariants(t *testing.T) {
	// The common player group1 source array has eight entries; kind0..7 all
	// use 8ED8/9321. This is source-data availability, not an action-completeness claim.
	for kind := uint8(0); kind < 8; kind++ {
		profile, err := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: 4, Frame: 7})
		if err != nil || profile.AttackFlags != 0x9c || profile.Shape.Body.MaxHeight != 32 {
			t.Fatal(kind, profile, err)
		}
	}
	for _, key := range []ContactProfileKey{{Kind: 0, Action: 4, Frame: 255}, {Kind: 8, Action: 25}, {Kind: 0, Action: 0x61, Variant: 255}, {Kind: 255, Action: 5}} {
		if _, err := OriginalContactProfile(key); err == nil {
			t.Fatal("undefined source domain silently accepted", key)
		}
	}
}
