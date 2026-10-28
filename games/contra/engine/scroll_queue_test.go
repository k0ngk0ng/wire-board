package contra

// Integration failures defined before queued scrolling: splitting seven attrs
// into seven packets changes original budget; prior queue ownership is lost;
// allocation failure commits scroll/collision but not graphics; scroll writes
// bypass the same budget later used by destructible-terrain requests.

import (
	"encoding/hex"
	"reflect"
	"testing"
)

func TestOriginalJungleScrollQueued(t *testing.T) {
	d := loadScrollOracle(t)
	for _, r := range d.Records {
		s, w, c, enemy := scrollFromOriginal(t, d, r.Before)
		wantS, wantW, wantC, wantEnemy := scrollFromOriginal(t, d, r.After)
		before, _ := hex.DecodeString(r.Before.Queue)
		expected, _ := hex.DecodeString(r.After.Queue)
		q := GraphicsQueue{Used: uint8(len(before))}
		result, err := s.AdvanceQueued(&w, &c, &q)
		if err != nil {
			t.Fatalf("frame %d: %v", r.Frame, err)
		}
		if result.ResetEnemyReadOffset {
			enemy = 0
		}
		if s != wantS || w != wantW || c != wantC || enemy != wantEnemy {
			t.Fatalf("frame %d queued simulation differs", r.Frame)
		}
		encoded := append([]uint8{}, before...)
		for _, p := range q.Packets {
			v, err := p.Encode()
			if err != nil {
				t.Fatal(err)
			}
			encoded = append(encoded, v...)
		}
		if !reflect.DeepEqual(encoded, expected) || int(q.Used) != len(expected) {
			t.Fatalf("frame %d queue got=%x/%d want=%x/%d", r.Frame, encoded, q.Used, expected, len(expected))
		}
	}
}

func TestJungleScrollQueueFailureIsAtomic(t *testing.T) {
	d := loadScrollOracle(t)
	for _, r := range d.Records {
		if r.Before.Queue == r.After.Queue {
			continue
		}
		s, w, c, _ := scrollFromOriginal(t, d, r.Before)
		beforeS, beforeW, beforeC := s, w, c
		q := GraphicsQueue{Used: 250}
		beforeQ := q
		if _, err := s.AdvanceQueued(&w, &c, &q); err == nil {
			t.Fatal("overflow unexpectedly committed")
		}
		if s != beforeS || w != beforeW || c != beforeC || !reflect.DeepEqual(q, beforeQ) {
			t.Fatal("queue failure partially changed scene")
		}
		return
	}
	t.Fatal("no queued write in original fixture")
}

func TestScrollAndEnemyRequestsShareQueueBudget(t *testing.T) {
	d := loadScrollOracle(t)
	for _, r := range d.Records {
		s, w, c, _ := scrollFromOriginal(t, d, r.Before)
		if w.ScreenScroll&15 != 2 || w.FrameScroll != 1 {
			continue
		}
		var q GraphicsQueue
		if _, err := s.AdvanceQueued(&w, &c, &q); err != nil {
			t.Fatal(err)
		}
		if q.Used != 24 {
			t.Fatalf("attribute grouping cost changed: %d", q.Used)
		}
		tileset, err := JungleGraphicsTileset()
		if err != nil {
			t.Fatal(err)
		}
		context := GraphicsContext{Scroll: w.Scroll, Supertiles: &s.Supertiles, Tileset: tileset}
		for i := 0; i < 2; i++ {
			result, err := q.RequestSupertile(32, 48, 0, context)
			if err != nil || !result.Accepted {
				t.Fatal("original pre-admission budget must accept", i, result, err)
			}
		}
		if q.Used < 64 {
			t.Fatal("request budget was not consumed", q.Used)
		}
		before := q.Used
		result, err := q.RequestSupertile(32, 48, 0, context)
		if err != nil || result.Accepted || q.Used != before {
			t.Fatal("full queue did not defer enemy terrain update", result, err)
		}
		return
	}
	t.Fatal("no natural attribute write")
}
