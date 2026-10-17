package engine

// World integration failures before wiring: the newly accepted player action
// reaches a hoop/free handler; auxiliary binding is applied after its visit;
// preparation 14 is skipped; owner is cleared before facing is read; shared
// follow source is lost; pickup marks are silently synthesized for a scene.
import (
	"reflect"
	"testing"
)

func auxiliaryWorldScenario(t *testing.T) (WorldActionState, WorldActionContext) {
	row := worldActionFixtures(t)[0]
	s, ctx := row.Before, row.context(t)
	for i := range s.Actors {
		s.Actors[i].Player = Player{Action: ActionIdle, Heading: HeadingNone, Body: Body{Position: Position{X: Fixed(100+i*30) * Pixel, Depth: 40 * Pixel}}}
		s.PlayerLinks[i] = 0
	}
	s.Holder = 6
	s.SpecialHolder = -1
	s.Ball = Ball{Action: BallRest, Entered: true, Body: Body{Position: Position{X: 400 * Pixel, Depth: 40 * Pixel}}}
	s.Actors[0].Player.Body.Position = Position{X: 300*Pixel + 11, Depth: 40*Pixel + 13}
	s.Actors[0].Player.Action = 0x4e
	s.Auxiliary[1] = AuxiliaryState{Action: 0, Entered: true, Owner: -1, Attachment: 0, Body: Body{Position: Position{X: 300*Pixel + 17, Depth: 40*Pixel + 19, Height: 23}}}
	flags := [4]uint8{32, 0, 0, 0}
	ctx.PickupFlags = &flags
	return s, ctx
}

func TestWorldAuxiliaryPickupPreparationReleaseAndLand(t *testing.T) {
	s, ctx := auxiliaryWorldScenario(t)
	advance := func(state WorldActionState) (WorldActionStep, error) {
		// Restore the real next-action-phase boundary; processed bits are
		// cleared by 8469 before inputs, not by E130 itself.
		contact := BeginOriginalContacts(ContactPhaseState{Links: state.actionLinks()}, state.Ball.Action)
		state.setActionLinks(contact.Links)
		return StepWorldActions(state, ctx)
	}
	first, err := advance(s)
	if err != nil {
		t.Fatal(err)
	}
	s = first.State
	a := s.Auxiliary[1]
	if a.Action != 1 || a.Owner != 0 || a.Attachment != 192 || a.Body.Position.X != 300*Pixel+17 || a.Body.Position.Depth != 40*Pixel+19 || a.Body.Position.Height != 24*Pixel+23 || s.PoseSource != 0 {
		t.Fatal("pickup not bound before auxiliary phase", a, s.PoseSource)
	}
	for n := 0; s.Actors[0].Player.Action != 0x20 && n < 10; n++ {
		step, e := advance(s)
		if e != nil {
			t.Fatal(e)
		}
		s = step.State
	}
	if s.Actors[0].Player.Action != 0x20 {
		t.Fatal("pickup animation did not complete")
	}
	s.Actors[0].Player.Request = ActionRequest{Action: 0x27, Pending: true}
	step, err := advance(s)
	if err != nil {
		t.Fatal(err)
	}
	s = step.State
	if s.Actors[0].Player.Action != 0x27 || s.Auxiliary[1].Action != 14 || s.Auxiliary[1].Owner != 0 || s.Auxiliary[1].Attachment != 192 {
		t.Fatal("throw preparation bypassed original ownership", s.Auxiliary[1])
	}
	step, err = advance(s)
	if err != nil {
		t.Fatal(err)
	}
	s = step.State
	if s.Auxiliary[1].Action != 3 || s.Auxiliary[1].Entered || s.Auxiliary[1].Owner != 0 {
		t.Fatal("owner lost before throw handler", s.Auxiliary[1])
	}
	step, err = advance(s)
	if err != nil {
		t.Fatal(err)
	}
	s = step.State
	if s.Auxiliary[1].Action != 3 || !s.Auxiliary[1].Entered || s.Auxiliary[1].Owner != -1 || s.Auxiliary[1].Attachment != 64 || s.Auxiliary[1].Body.Velocity.X != 12*Pixel {
		t.Fatal("throw handler did not release", s.Auxiliary[1])
	}
	for n := 0; s.Auxiliary[1].Action != 0 && n < 30; n++ {
		step, e := advance(s)
		if e != nil {
			t.Fatal(e)
		}
		s = step.State
	}
	if s.Auxiliary[1].Action != 0 || s.Auxiliary[1].Airborne {
		t.Fatal("original thrown prop did not land", s.Auxiliary[1])
	}
}

func TestWorldAuxiliaryPickupRequiresAuthoritativeMarks(t *testing.T) {
	s, ctx := auxiliaryWorldScenario(t)
	ctx.PickupFlags = nil
	got, err := StepWorldActions(s, ctx)
	if err == nil || !reflect.DeepEqual(got.State, s) {
		t.Fatal("missing scene pickup marks were silently treated as zero", err)
	}
}
