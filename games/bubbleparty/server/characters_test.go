package bubbleparty

// Failure inventory written before implementation:
// - BomberAttr record order confused with Cxx character ID (C02/C03/C05/C06).
// - Every character accidentally receives the same speed, bubble or range caps.
// - C08/C09 hidden characters become selectable without original evidence.
// - Invalid IDs silently resolve to a different character.
// - Integer timing truncates native 40px cells / 30ms ticks in the UI helper.
import "testing"

func TestOriginalCharacterProfiles(t *testing.T) {
	want := [10][6]int{
		{5, 6, 1, 10, 1, 6}, {4, 7, 2, 7, 1, 8},
		{4, 7, 1, 8, 2, 7}, {5, 7, 1, 10, 1, 5},
		{4, 8, 2, 9, 1, 5}, {5, 7, 1, 10, 1, 5},
		{5, 9, 1, 6, 1, 7}, {4, 7, 1, 9, 2, 6},
		{6, 9, 3, 9, 3, 7}, {5, 9, 3, 9, 3, 8},
	}
	for id, values := range want {
		p, ok := characterProfile(id)
		if !ok || p.ID != id || [6]int{p.InitialSpeed, p.MaximumSpeed, p.InitialBombs, p.MaximumBombs, p.InitialRange, p.MaximumRange} != values {
			t.Fatalf("C%02d profile: %+v, want %v", id, p, values)
		}
		if p.Hidden != (id >= RegularCharacterCount) {
			t.Fatalf("C%02d hidden marker: %v", id, p.Hidden)
		}
	}
	if CharacterCount != 10 || RegularCharacterCount != 8 {
		t.Fatalf("original character counts changed")
	}
	for _, id := range []int{-1, 10, 100} {
		if _, ok := characterProfile(id); ok {
			t.Fatalf("invalid character accepted: %d", id)
		}
	}
}

func TestCharacterMoveMSForDisplay(t *testing.T) {
	for speed, want := range map[int]int{4: 300, 5: 240, 6: 200, 7: 172, 8: 150, 9: 134} {
		if got := characterMoveMS(speed); got != want {
			t.Fatalf("speed %d: %dms, want %dms", speed, got, want)
		}
	}
}
