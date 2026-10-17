package engine

// Failure cases: a no-op passes natural no-contact rows; wrong high-bit gates,
// missing CPU tactic limits, inclusive/exclusive depth/X/height boundaries,
// reused support seats, reciprocal links, prop-specific action selection,
// clearing links for ineligible actions, and partial effects on unsupported data.
import (
	"encoding/json"
	"os"
	"testing"
)

func TestRiderLinkPhaseAgainstOriginal(t *testing.T) {
	verifyOriginalRiderPhases(t, "rider-links.json", 1997, 0)
}
func TestRiderLinkPhaseNaturalShoulderMount(t *testing.T) {
	verifyOriginalRiderPhases(t, "rider-links-natural.json", 57, 1)
}
func verifyOriginalRiderPhases(t *testing.T, filename string, count, changes int) {
	t.Helper()
	var rows []struct {
		Frame         int
		Before, After struct {
			Objects [6]struct {
				X, Depth, Height, VZ           int32
				Kind, Action, Air, Control, AI uint8
			}
			Links [7]uint8
		}
	}
	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for _, r := range rows {
		var before, want RiderLinkState
		for i := range before.Objects {
			for side, state := range []*RiderLinkState{&before, &want} {
				src := r.Before
				if side == 1 {
					src = r.After
				}
				o := src.Objects[i]
				state.Objects[i] = RiderLinkObject{Position: Position{X: Fixed(o.X), Depth: Fixed(o.Depth), Height: signed24(Fixed(o.Height))}, HeightVelocity: signed16(Fixed(o.VZ)), Kind: o.Kind, Action: Action(o.Action & 127), Entered: o.Action&128 != 0, Airborne: o.Air&128 != 0, Control: o.Control, AIMode: o.AI}
			}
		}
		before.Links, want.Links = r.Before.Links, r.After.Links
		got, err := StepOriginalRiderLinks(before)
		if err != nil || got != want {
			t.Fatalf("frame %d\ngot %+v\nwant %+v\nerr %v", r.Frame, got, want, err)
		}
		if before != want {
			changed++
		}
	}
	if len(rows) != count || changed != changes {
		t.Fatal("original phase scope changed", len(rows), changed)
	}
	t.Logf("matched %d original phases, %d natural link changes", count, changes)
}

func riderFixture() RiderLinkState {
	var s RiderLinkState
	for i := range s.Objects {
		s.Objects[i] = RiderLinkObject{Kind: 9, Action: 2, Position: Position{X: 400 * Pixel, Depth: 80 * Pixel}}
	}
	s.Objects[0] = RiderLinkObject{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 32 * Pixel}, HeightVelocity: -4 * Pixel, Kind: 0, Action: ActionJump, Airborne: true, Entered: true}
	s.Objects[1] = RiderLinkObject{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Kind: 0, Action: ActionIdle, Entered: true}
	return s
}
func TestRiderLinksEstablishAndBoundaryGates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*RiderLinkState)
		linked bool
		target uint8
		action Action
	}{
		{"teammate", func(s *RiderLinkState) {}, true, 1, 0x3d},
		{"ascending", func(s *RiderLinkState) { s.Objects[0].HeightVelocity = Pixel }, false, 0, 0},
		{"not crossing head", func(s *RiderLinkState) { s.Objects[0].Position.Height = 35 * Pixel }, false, 0, 0},
		{"lands exactly at head", func(s *RiderLinkState) { s.Objects[0].Position.Height = 33 * Pixel }, true, 1, 0x3d},
		{"already under head", func(s *RiderLinkState) { s.Objects[0].Position.Height = 28 * Pixel }, false, 0, 0},
		{"x lower outside", func(s *RiderLinkState) { s.Objects[1].Position.X = 191 * Pixel }, false, 0, 0},
		{"x lower inside", func(s *RiderLinkState) { s.Objects[1].Position.X = 192 * Pixel }, true, 1, 0x3d},
		{"x upper inside", func(s *RiderLinkState) { s.Objects[1].Position.X = 208 * Pixel }, true, 1, 0x3d},
		{"x upper outside", func(s *RiderLinkState) { s.Objects[1].Position.X = 209 * Pixel }, false, 0, 0},
		{"depth lower inside", func(s *RiderLinkState) { s.Objects[1].Position.Depth = 33 * Pixel }, true, 1, 0x3d},
		{"depth lower outside", func(s *RiderLinkState) { s.Objects[1].Position.Depth = 32 * Pixel }, false, 0, 0},
		{"depth upper outside", func(s *RiderLinkState) { s.Objects[1].Position.Depth = 48 * Pixel }, false, 0, 0},
		{"enemy", func(s *RiderLinkState) { s.Objects[2] = s.Objects[1]; s.Objects[1].Position.X = 400 * Pixel }, false, 0, 0},
		{"occupied", func(s *RiderLinkState) { s.Links[6] = 0x81 }, false, 0, 0},
		{"occupancy slot four ignored", func(s *RiderLinkState) { s.Links[4] = 0x81 }, true, 1, 0x3d},
		{"reciprocal", func(s *RiderLinkState) { s.Links[1] = 0x80 }, false, 0, 0},
		{"cpu normal tactic", func(s *RiderLinkState) { s.Objects[0].Control = 128; s.Objects[0].AIMode = 9 }, false, 0, 0},
		{"cpu shoulder tactic", func(s *RiderLinkState) { s.Objects[0].Control = 128; s.Objects[0].AIMode = 0x9d }, true, 1, 0x3d},
		{"cpu second tactic", func(s *RiderLinkState) { s.Objects[0].Control = 128; s.Objects[0].AIMode = 0x22 }, true, 1, 0x3d},
		{"special support", func(s *RiderLinkState) {
			s.Objects[4] = s.Objects[1]
			s.Objects[4].Kind = 12
			s.Objects[4].Position.Height = 13 * Pixel
			s.Objects[1].Position.X = 400 * Pixel
		}, true, 4, 0x40},
		{"excluded kind", func(s *RiderLinkState) { s.Objects[1].Kind = 13 }, false, 0, 0},
		{"unsupported source action", func(s *RiderLinkState) { s.Objects[0].Action = ActionFalling }, false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := riderFixture()
			tc.change(&s)
			got, err := StepOriginalRiderLinks(s)
			if err != nil {
				t.Fatal(err)
			}
			if tc.linked {
				if got.Links[0] != (0x80|tc.target) || got.Objects[0].Action != tc.action || got.Objects[0].Entered {
					t.Fatal(got)
				}
			} else if got != s {
				t.Fatal("unexpected effect", got)
			}
		})
	}
}
func TestRiderLinksReleaseOnlyEligibleGroundedActor(t *testing.T) {
	for _, action := range []Action{ActionJump, ActionIdle} {
		s := riderFixture()
		s.Objects[0].Action = action
		s.Objects[0].Airborne = false
		s.Links[0] = 0x81
		got, err := StepOriginalRiderLinks(s)
		if err != nil {
			t.Fatal(err)
		}
		want := uint8(0x81)
		if action == ActionJump {
			want = 0
		}
		if got.Links[0] != want || got.Objects != s.Objects {
			t.Fatal(got)
		}
	}
	s := riderFixture()
	s.Objects[0].Position.Height = Pixel
	s.Objects[0].HeightVelocity = -4 * Pixel
	s.Objects[1].Kind = 9
	s.Objects[1].Position.Height = 0
	got, err := StepOriginalRiderLinks(s)
	if err != nil || got.Links[0] != 0x81 {
		t.Fatal("negative predicted height should clamp to zero", got, err)
	}
}
