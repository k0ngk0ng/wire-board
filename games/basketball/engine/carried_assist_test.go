package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

// Failures: 09 rejects valid paired motion, motion order reverses, support
// retains copied speed, rider keeps pre-contact speed, extra gravity, stale
// floor snapshot, and target mutation delayed until after court-device effects.
func TestOriginalCarriedAssistPairedMotion(t *testing.T) {
	f, e := os.Open("testdata/carried-assist.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []capturedOracleCall
	if e = json.NewDecoder(z).Decode(&rows); e != nil || len(rows) != 1440 {
		t.Fatal(e, len(rows))
	}
	for n, row := range rows {
		r := WorldActionStep{State: row.Before}
		c := row.context(t)
		court, e := originalWorldMotionCourt(&r, c, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if c.CourtID == 1 {
			court = NewYorkGroundCourt()
		}
		if e = advanceWorldPlayer(&r, row.Actor, c, court); e != nil {
			t.Fatal(n, e)
		}
		if r.State != row.After {
			t.Fatal(n, firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
		}
		if !slices.Equal(r.SoundIDs, row.Sounds) {
			t.Fatal(n, r.SoundIDs, row.Sounds)
		}
	}
	t.Log("matched1440 original paired-motion09 and attached0B calls including FF alias")
}
