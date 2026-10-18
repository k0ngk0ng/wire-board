package engine

// Failure inventory before implementation: omit 5A/5C owner cursor offsets,
// confuse mirrored/hidden ball frames, restart or truncate a persistent tail,
// lose uint8 elapsed wrapping, change motion/hand offsets or exclusion flags,
// or replace the source's full-sequence loop with a tail-only repeat.
import (
	"reflect"
	"testing"
)

func TestOriginalHeldBallLowJumpCalls(t *testing.T) {
	var rows []struct {
		worldActionFixture
		Label string
	}
	readPropAttackFixture(t, "testdata/held-low-jump-ball.json.gz", &rows)
	if len(rows) != 3210 {
		t.Fatal("incomplete original held-ball probes", len(rows))
	}
	for i, r := range rows {
		s := r.Before
		_, e := advanceWorldBallStep(&s, r.context(t), NewYorkBallCourt())
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if s != r.After {
			t.Fatal(i, r.Label, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
	}
}
