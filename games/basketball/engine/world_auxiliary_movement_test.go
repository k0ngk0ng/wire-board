package engine

// Before wiring: the world keeps routing group20 movement to hoop handlers;
// equipment or support data are replaced by zero; command flags are lost;
// takeoff clears attachment after physics; auxiliary throws change landing or
// selected-hoop metadata instead of only original shared target/arc fields.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func decodeAuxiliaryWorldFixture(t *testing.T, name string, out any) {
	t.Helper()
	f, e := os.Open("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	if e = json.NewDecoder(z).Decode(out); e != nil {
		t.Fatal(e)
	}
}

func TestWorldAuxiliaryMovementUsesOriginalCommands(t *testing.T) {
	var rows []struct {
		Before, After                              WorldActionState
		Computer, EnhancementEnabled               bool
		Team, RunGrade, JumpGrade, JumpEnhancement uint8
		Sounds                                     []uint8
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-movement.json.gz", &rows)
	for i, r := range rows {
		profile := KunioProfile()
		profile.RunSpeed, _ = RunSpeedForGrade(r.RunGrade)
		profile.JumpVelocity, _ = JumpVelocityForGrade(r.JumpGrade)
		gear := [4]uint8{r.JumpEnhancement}
		ctx := WorldActionContext{CourtID: 1, Teams: [2]uint8{r.Team, 1}, EnhancementEnabled: r.EnhancementEnabled, JumpEnhancements: &gear}
		ctx.Profiles[0] = profile
		if r.Computer {
			ctx.Controls[0] = 128
		}
		result := WorldActionStep{State: r.Before}
		err := advanceWorldPlayer(&result, 0, ctx, NewYorkGroundCourt())
		if err != nil || result.State.Ball != r.After.Ball || result.State.Holder != r.After.Holder || result.State.Actors[0] != r.After.Actors[0] || result.State.Auxiliary != r.After.Auxiliary || result.State.PlayerLinks != r.After.PlayerLinks || !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatalf("row%d action%x: %s aux:%s links%v/%v sounds%v/%v err%v", i, r.Before.Actors[0].Player.Action, firstWorldLoopDifference(reflect.ValueOf(result.State.Actors[0]), reflect.ValueOf(r.After.Actors[0]), "actor"), firstWorldLoopDifference(reflect.ValueOf(result.State.Auxiliary), reflect.ValueOf(r.After.Auxiliary), "auxiliary"), result.State.PlayerLinks, r.After.PlayerLinks, result.SoundIDs, r.Sounds, err)
		}
	}
	if len(rows) != 4672 {
		t.Fatal(len(rows))
	}
}

func TestWorldAuxiliaryObjectSourceAndLaunchCommands(t *testing.T) {
	for _, name := range []string{"auxiliary-carry-probes.json.gz", "auxiliary-lob.json.gz"} {
		var rows []struct {
			Object        uint8
			Before, After WorldActionState
		}
		decodeAuxiliaryWorldFixture(t, name, &rows)
		for i, r := range rows {
			object := r.Object
			if object == 0 {
				object = 5
			}
			result := WorldActionStep{State: r.Before}
			ctx := WorldActionContext{CourtID: 1, Teams: [2]uint8{0, 1}}
			court := auxiliaryNewYorkCourt()
			court.passLaunchContext = func() PassLaunchContext {
				return PassLaunchContext{SecondPlayerX: result.State.Actors[1].Player.Body.Position.X}
			}
			err := advanceWorldAuxiliary(&result, object, ctx, court)
			if err != nil || result.State != r.After {
				t.Fatalf("%s row%d: %s err%v", name, i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"), err)
			}
		}
	}
}

func TestWorldAuxiliarySupportUsesDynamicPropKind(t *testing.T) {
	s, ctx := auxiliaryWorldScenario(t)
	s.Actors[0].Player.Action = 0x27
	s.Actors[0].Player.Entered = false
	s.PlayerLinks[0] = 132
	s.Auxiliary[1].Owner = 0
	s.Auxiliary[0].Kind = 14
	s.Auxiliary[0].Body.Position = Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 20 * Pixel}
	result := WorldActionStep{State: s}
	err := advanceWorldPlayer(&result, 0, ctx, NewYorkGroundCourt())
	if err != nil || result.State.Actors[0].Player.Body.Position.Height != 33*Pixel || result.State.Actors[0].Player.Animation.NextAction != ActionRiding {
		t.Fatal("attached throw did not use current Kind14 support", result.State.Actors[0], err)
	}
}

func TestWorldAuxiliaryFollowOwnerAndSupportStayDistinct(t *testing.T) {
	s, ctx := auxiliaryWorldScenario(t)
	s.Auxiliary[1].Action = 1
	s.Auxiliary[1].Owner = 0
	s.Auxiliary[1].Attachment = 129
	s.Auxiliary[1].Airborne = true
	s.Actors[1].Player.Body.Position = Position{X: 220 * Pixel, Depth: 42 * Pixel, Height: 64 * Pixel}
	result := WorldActionStep{State: s}
	err := advanceWorldAuxiliary(&result, 5, ctx, auxiliaryNewYorkCourt())
	got := result.State.Auxiliary[1]
	if err != nil || result.State.PoseSource != 0 || got.Body.Position != (Position{X: 220*Pixel + 17, Depth: 42*Pixel + 19, Height: 96*Pixel + 23}) {
		t.Fatal("owner-follow source replaced the actual attachment support", got, result.State.PoseSource, err)
	}
}
