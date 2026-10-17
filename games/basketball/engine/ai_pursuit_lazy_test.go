package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestPursuitSearchAllOriginalLazyBranches(t *testing.T) {
	f, err := os.Open("testdata/pursuit-search.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Frame    int
		Actor    uint8
		Target   int8
		Before   WorldActionState
		Controls [4]uint8
		Reserved PursuitReservedBytes
		Alias74  PursuitAlias74
		Probes   []struct {
			Index            uint8
			Action           Action
			Depth            uint8
			X                uint16
			ReadDepth, ReadX bool
		}
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1112 {
		t.Fatal("incomplete original search calls", len(rows))
	}
	counts := map[uint8]int{}
	for i, r := range rows {
		v := AIControlView{Actor: r.Actor, PursuitWorld: &r.Before, PlayerControls: &r.Controls, PursuitReserved: &r.Reserved, PursuitAlias74: &r.Alias74, Auxiliary: &r.Before.Auxiliary, BallPosition: r.Before.Ball.Body.Position}
		for j, a := range r.Before.Actors {
			v.Players[j] = a.Player.Body.Position
			v.PlayerActions[j] = a.Player.Action
		}
		for j, h := range r.Before.Hoops {
			v.Hoops[j] = AIHoopView{Position: h.Position, Action: h.Action, Owner: int8(h.Owner)}
		}
		got, err := opponentNearOriginalBall(v)
		if err != nil || got != r.Target {
			t.Fatalf("call %d frame %d actor %d target %d want %d: %v", i, r.Frame, r.Actor, got, r.Target, err)
		}
		for _, p := range r.Probes {
			action, err := originalPursuitAction(v, p.Index)
			if err != nil || action != p.Action {
				t.Fatal(i, p, action, err)
			}
			if p.ReadDepth {
				depth, err := originalPursuitDepth(v, p.Index)
				if err != nil || depth != p.Depth {
					t.Fatal(i, p, depth, err)
				}
			}
			if p.ReadX {
				x, err := originalPursuitX(v, p.Index)
				if err != nil || x != p.X {
					t.Fatal(i, p, x, err)
				}
			}
			counts[p.Index]++
		}
	}
	if counts[73] != 20 || counts[75] != 5 {
		t.Fatal("missing naturally reached hurt aliases", counts)
	}
	t.Logf("matched all 1112 original B100 calls, actual lazy reads and targets: %v", counts)
}

func TestPursuitFlagsRejectBeforeUnknownCoordinates(t *testing.T) {
	world := WorldActionState{ExcludedCatcher: -1, CarriedPlayer: -1, SelectedHoop: -1}
	v := AIControlView{Actor: 3, PursuitWorld: &world}
	for _, action := range []Action{78, 86, 74} {
		v.PlayerActions[0] = action
		if target, err := opponentNearOriginalBall(v); err != nil || target != -1 {
			t.Fatal("rejected action requested unused coordinates", action, target, err)
		}
	}
	v.PlayerActions[0] = 78
	world.ExcludedCatcher = 0
	if _, err := opponentNearOriginalBall(v); err == nil {
		t.Fatal("needed unknown depth silently defaulted")
	}
}

func TestPursuitCanonicalActionAliases(t *testing.T) {
	world := WorldActionState{SelectedHoop: 11, ShotOriginX: Fixed(0x1245) * Pixel, ExcludedCatcher: 3, CarriedPlayer: -1}
	world.Actors[1].Player.Request.Action = Action(0x86)
	world.Hoops[3].Request.Action = HoopAction(0x81)
	world.Actors[1].Dodge.InitialXHigh = -2
	controls := [4]uint8{0x80, 2}
	v := AIControlView{PursuitWorld: &world, PlayerControls: &controls}
	v.Hoops[0].Action = HoopAction(0x81)
	v.Hoops[2].Owner = -119
	for _, tc := range []struct {
		index uint8
		want  Action
	}{{7, 1}, {14, 6}, {23, 1}, {39, 0}, {40, 2}, {48, 9}, {55, 126}, {75, 11}, {76, 69}, {77, 18}, {79, 3}, {87, 127}} {
		got, err := originalPursuitAction(v, tc.index)
		if err != nil || got != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	if _, err := originalPursuitAction(AIControlView{}, 79); err == nil {
		t.Fatal("missing canonical world context accepted")
	}
}

func TestPursuitReservedBytesAreExplicitAndNonzero(t *testing.T) {
	if _, err := originalPursuitAction(AIControlView{}, 73); err == nil {
		t.Fatal("missing reserved byte accepted")
	}
	v := AIControlView{PursuitReserved: &PursuitReservedBytes{Action73: 0x83, Depth73: 201, Depth75: 71}, PursuitWorld: &WorldActionState{SelectedHoop: 8}}
	v.Hoops[1].Position.Height = Fixed(0x1234)*Pixel + 127
	v.Hoops[3].Position.Height = Fixed(0x5678)*Pixel + 21
	a, err := originalPursuitAction(v, 73)
	if err != nil || a != 3 {
		t.Fatal(a, err)
	}
	d, err := originalPursuitDepth(v, 73)
	if err != nil || d != 201 {
		t.Fatal(d, err)
	}
	x, err := originalPursuitX(v, 73)
	if err != nil || x != 0x1234 {
		t.Fatal(x, err)
	}
	d, err = originalPursuitDepth(v, 75)
	if err != nil || d != 71 {
		t.Fatal(d, err)
	}
	x, err = originalPursuitX(v, 75)
	if err != nil || x != 0x5678 {
		t.Fatal(x, err)
	}
}
