package engine

import "testing"

// Before widening the03FF domain: reject a source-valid elevated receiver;
// clamp rather than reproduce source rise/drop; alter original normalization,
// fixedpoint fractions, gravity or horizontal saturation; accept unproved
// 0400+ or nonterminating zero-source/negative-target; mutate AI or source state.
func TestHighPassHeightOriginalCompleteLaunch(t *testing.T) {
	var rows []struct {
		Name               string
		Position, Target   Position
		Arc, SecondPlayerX Fixed
		Expected           Body
	}
	readAITeamModeFixture(t, "high-pass-height", &rows)
	if len(rows) != 19950 {
		t.Fatal("incomplete high-height source matrix", len(rows))
	}
	for _, r := range rows {
		got, e := LaunchPass(r.Position, r.Target, r.Arc, PassLaunchContext{SecondPlayerX: r.SecondPlayerX})
		if e != nil || got != r.Expected {
			t.Fatalf("%s got%+v want%+v err%v", r.Name, got, r.Expected, e)
		}
	}
	t.Logf("matched %d complete original03FF-domain launches", len(rows))
}
