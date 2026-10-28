package contra

// Source: bank12/13 848A selects 84E0/84E2, then 849F writes the low
// spread-velocity pointer to generation compatibility. This precedes the World
// soldier generator. Disabled/full/indoor firing must not report a write.
import "testing"

func TestSpreadFireGenerationEffect(t *testing.T) {
	for _, stage := range []uint8{0, 1, 0x80} {
		for _, rapid := range []uint8{0, 16} {
			for _, full := range []bool{false, true} {
				s := WeaponSystem{}
				if full {
					for i := range s.Projectiles {
						s.Projectiles[i].Kind = 1
					}
				}
				r := s.Fire(FireRequest{Actor: 0, X: 80, Y: 100, Aim: 2, Weapon: 3 | rapid, StageType: stage, Held: ButtonB, Pressed: ButtonB})
				changed := stage != 1 && !full
				want := uint8(0xf4)
				if rapid != 0 {
					want = 0x44
				}
				if r.GenerationBiasChanged != changed || changed && r.GenerationBiasValue != want {
					t.Fatal(stage, rapid, full, r)
				}
			}
		}
	}
	for _, weapon := range []uint8{0, 1, 2, 3, 4} {
		s := WeaponSystem{}
		r := s.Fire(FireRequest{Actor: 0, X: 80, Y: 100, Aim: 2, Weapon: weapon})
		if r.GenerationBiasChanged {
			t.Fatal("idle fire reports compatibility write", weapon)
		}
	}
}
