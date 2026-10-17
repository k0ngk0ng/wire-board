package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestPursuitAlias88IsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/pursuit-alias88-function.json.gz")
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
		Actor, PoseSource, Depth, XHigh, XLow, BallDepth uint8
		BallX                                            uint16
		Target                                           int8
		ReadDepth, ReadX                                 bool
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 14112 {
		t.Fatal("incomplete original B100 alias88 matrix", len(rows))
	}
	for i, r := range rows {
		v := AIControlView{Actor: r.Actor, PursuitWorld: &WorldActionState{PoseSource: r.PoseSource}, PursuitReserved: &PursuitReservedBytes{Depth88: &r.Depth, XHigh88: &r.XHigh}, BallPosition: Position{X: Fixed(r.BallX) * Pixel, Depth: Fixed(r.BallDepth) * Pixel}}
		v.PlayerActions[(r.Actor&2)^2] = Action(87)
		v.Hoops[3].Position.Height = Fixed(r.XLow) << 16
		target, err := opponentNearOriginalBall(v)
		if err != nil || target != r.Target {
			t.Fatal(i, r, target, err)
		}
		action, err := originalPursuitAction(v, 88)
		if err != nil || action != Action(r.PoseSource&127) {
			t.Fatal(i, action, err)
		}
		if r.ReadDepth {
			depth, err := originalPursuitDepth(v, 88)
			if err != nil || depth != r.Depth {
				t.Fatal(i, depth, err)
			}
		}
		if r.ReadX {
			x, err := originalPursuitX(v, 88)
			if err != nil || x != uint16(r.XLow)|uint16(r.XHigh)<<8 {
				t.Fatal(i, x, err)
			}
		}
	}
	t.Log("matched 14112 isolated original B100 alias88 cases; source, flags, lazy branches and hits preserved")
}

func TestPursuitAlias88RequiresOnlyActuallyReadBytes(t *testing.T) {
	v := AIControlView{Actor: 0}
	v.PlayerActions[2] = 87
	if _, err := opponentNearOriginalBall(v); err == nil {
		t.Fatal("missing canonical pose source defaulted")
	}
	v.PursuitWorld = &WorldActionState{PoseSource: 6}
	if target, err := opponentNearOriginalBall(v); err != nil || target != -1 {
		t.Fatal("rejected action unnecessarily required reserved data", target, err)
	}
	v.PursuitWorld.PoseSource = 2
	v.BallPosition.Depth = 100 * Pixel
	if _, err := opponentNearOriginalBall(v); err == nil {
		t.Fatal("needed depth defaulted")
	}
	depth := uint8(0)
	v.PursuitReserved = &PursuitReservedBytes{Depth88: &depth}
	if target, err := opponentNearOriginalBall(v); err != nil || target != -1 {
		t.Fatal("depth rejection unnecessarily required X data", target, err)
	}
	v.BallPosition.Depth = 0
	if _, err := opponentNearOriginalBall(v); err == nil {
		t.Fatal("needed X high byte defaulted")
	}
	high := uint8(1)
	v.PursuitReserved.XHigh88 = &high
	v.BallPosition.X = 256 * Pixel
	if target, err := opponentNearOriginalBall(v); err != nil || target != 88 {
		t.Fatal("known nonzero alias coordinates ignored", target, err)
	}
}

func TestPursuitAlias88ColdInitializationBytes(t *testing.T) {
	_, ctx, err := WorldFromOriginalOpening(allComputerColdOpening(t), OpeningWorldOptions{LogicFrame: 35})
	if err != nil {
		t.Fatal(err)
	}
	p := ctx.PursuitReserved
	if p == nil || p.Depth88 == nil || *p.Depth88 != 0 || p.XHigh88 == nil || *p.XHigh88 != 255 {
		t.Fatal("cold FA8C/A0D3 defaults missing", p)
	}
}
