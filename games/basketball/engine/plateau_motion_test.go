package engine

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"testing"
)

func TestOriginalPlateauMotion(t *testing.T) {
	f, e := os.Open("testdata/plateau-motion.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rec [4101]byte
	var counts [7]int
	attached := 0
	for {
		_, e = io.ReadFull(z, rec[:])
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		frame := binary.LittleEndian.Uint32(rec[:4])
		o := int(rec[4])
		b, a := rec[5:2053], rec[2053:]
		if b[0x642+o]&128 != 0 {
			attached++
			continue
		}
		before, after := boardContactRAM(b), boardContactRAM(a)
		if o < 4 {
			got := before.Players[o]
			cmd, err := advancePlayerOnCourt(&got, playerTerrainContext{HasBall: int(before.Holder) == o, Computer: b[0x539+o]&128 != 0}, NewYorkGroundCourt())
			if err != nil || got != after.Players[o] {
				t.Fatalf("frame %d object %d err %v\nbefore %+v\ngot %+v\nwant %+v", frame, o, err, before.Players[o], got, after.Players[o])
			}
			ball := before.Ball
			if cmd.Change {
				ball.Action, ball.Entered = cmd.Action, false
			}
			if cmd.ChangeRequest {
				ball.Request = cmd.Request
			}
			if ball != after.Ball {
				t.Fatal("terrain ball command differs", frame, o, cmd)
			}
		} else if o == 6 {
			got := before.Ball
			err := advanceBallOnCourt(&got, NewYorkBallCourt())
			if err != nil || got != after.Ball {
				t.Fatalf("frame %d ball err %v\nbefore %+v\ngot %+v\nwant %+v", frame, err, before.Ball, got, after.Ball)
			}
		} else {
			continue
		}
		counts[o]++
	}
	if counts != ([7]int{2000, 2027, 2017, 2011, 0, 0, 1178}) || attached != 0 {
		t.Fatal("original terrain coverage changed", counts, attached)
	}
	t.Logf("matched unattached EEFF calls by object %v; separate attached follow excluded %d", counts, attached)
}
