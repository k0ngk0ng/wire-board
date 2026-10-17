package engine

import "testing"

func TestPlayerKindsUseOriginalSharedContactTables(t *testing.T) {
	// Bank5 8CF2 has the same 8ED8 action map for all eight character kinds;
	// 9301 likewise selects 9321 for each. A newly selected team must not
	// lose a restored action simply because that key wasn't observed before.
	want := ContactProfile{AttackFlags: 0x8c, ReceiveFlags: 0x8c, Shape: ContactShape{
		Attack: ContactBoxSpec{MinX: -8, MaxX: 8, MinHeight: -128, MaxHeight: 32},
		Body:   ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MaxHeight: 32},
	}}
	for kind := uint8(0); kind < 8; kind++ {
		got, err := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: uint8(ActionCarrierJump)})
		if err != nil || got != want {
			t.Fatalf("character kind %d shared jump geometry: %+v %v", kind, got, err)
		}
	}
	if _, err := OriginalContactProfile(ContactProfileKey{Kind: 255, Action: uint8(ActionCarrierJump)}); err == nil {
		t.Fatal("unknown kind must remain unsupported")
	}
}
