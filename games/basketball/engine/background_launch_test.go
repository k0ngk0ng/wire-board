package engine

// Before implementation: ownerFF reads background-buffer bytes, not player0;
// preserve the prop's own facing; lower-row19 tile and upper-row2 merged
// attribute must use the last generated column and its initialization phase.
// Unknown owner-height rejects atomically. Zero and high-byte values differ.
import (
	"testing"
)

func TestOriginalBackgroundColumnLaunchAlias(t *testing.T) {
	var rows []struct{ Court, Phase, Column, Low, High uint8 }
	readPropAttackFixture(t, "testdata/background-launch-alias.json.gz", &rows)
	if len(rows) != 1024 {
		t.Fatal("column source coverage", len(rows))
	}
	for i, row := range rows {
		got, e := OriginalBackgroundLaunchHeight(row.Court, row.Column, row.Phase)
		want := Fixed(uint16(row.Low)|uint16(row.High)<<8) * Pixel
		if e != nil || got != want {
			t.Fatal(i, got, want, e)
		}
	}
	if _, e := OriginalBackgroundLaunchHeight(8, 0, 1); e == nil {
		t.Fatal("invalid court accepted")
	}
	if _, e := OriginalBackgroundLaunchHeight(5, 64, 1); e == nil {
		t.Fatal("invalid column accepted")
	}
}

func TestOriginalUnownedAuxiliaryLobFromBackgroundColumn(t *testing.T) {
	var rows []struct {
		Object, Column, Phase, ArcFlags uint8
		OwnerHeight                     Fixed
		Before, After                   WorldActionState
	}
	readPropAttackFixture(t, "testdata/auxiliary-unowned-lob.json.gz", &rows)
	if len(rows) != 1024 {
		t.Fatal("unowned lob source coverage", len(rows))
	}
	for i, row := range rows {
		height, e := OriginalBackgroundLaunchHeight(5, row.Column, row.Phase)
		if e != nil || height != row.OwnerHeight {
			t.Fatal("source column height", i, e)
		}
		world := WorldActionStep{State: row.Before}
		court, e := originalWorldMotionCourt(&world, WorldActionContext{CourtID: 5, Teams: [2]uint8{0, 1}}, row.Object)
		if e != nil {
			t.Fatal(e)
		}
		court.passLaunchContext = func() PassLaunchContext {
			return PassLaunchContext{SecondPlayerX: row.Before.Actors[1].Player.Body.Position.X}
		}
		ctx := AuxiliaryContext{Object: row.Object, Court: 5, Launch: &AuxiliaryLaunchContext{ArcFlags: row.ArcFlags, OwnerHeight: &height}}
		got, e := StepAuxiliaryCarryThrow(row.Before.Auxiliary[row.Object-4], ctx, court)
		if e != nil || got.State != row.After.Auxiliary[row.Object-4] || got.LaunchPlan == nil || got.LaunchPlan.Target != row.After.LaunchTarget || got.LaunchPlan.ArcHeight != row.After.LaunchArcHeight {
			t.Fatalf("case%d got%+v want%+v target%+v err%v", i, got, row.After.Auxiliary[row.Object-4], row.After.LaunchTarget, e)
		}
	}
}

func TestUnownedAuxiliaryLobRequiresExplicitHeight(t *testing.T) {
	s := AuxiliaryState{Action: 2, Owner: -1}
	ctx := AuxiliaryContext{Object: 4, Court: 5, Launch: &AuxiliaryLaunchContext{}}
	got, e := StepAuxiliaryCarryThrow(s, ctx, FlatCourt{})
	if e == nil || got.State != s {
		t.Fatal("unknown alias guessed", got, e)
	}
}
