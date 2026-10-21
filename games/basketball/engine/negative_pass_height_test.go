package engine

import "testing"

// Failure inventory is in NEGATIVE_PASS_HEIGHT_VERIFICATION.md. Expectations
// are independent complete original ROM calls, including the actual producer.
func TestNegativeReceiverHeightOriginalLaunch(t *testing.T) {
	var rows []struct {
		Name               string
		Position, Target   Position
		Arc, SecondPlayerX Fixed
		Expected           Body
	}
	readAITeamModeFixture(t, "negative-pass-height", &rows)
	if len(rows) != 4125 {
		t.Fatal("missing signed receiver source domain", len(rows))
	}
	for _, r := range rows {
		got, err := LaunchPass(r.Position, r.Target, r.Arc, PassLaunchContext{SecondPlayerX: r.SecondPlayerX})
		if err != nil || got != r.Expected {
			t.Fatalf("%s: got%+v want%+v err=%v", r.Name, got, r.Expected, err)
		}
	}
	t.Logf("matched %d unmodified original negative receiver launches", len(rows))
}

func TestNegativeReceiverHeightUnsupportedDomains(t *testing.T) {
	for _, tc := range []struct {
		name                string
		source, target, arc Fixed
	}{
		{"zero-source-never-rises", 0, -Pixel, 0},
		{"fractional-source-never-rises", 255, -Pixel, 0},
		{"zero-source-arc-still-negative", 0, -256 * Pixel, 255 * Pixel},
		{"negative-launch-source", -1, -Pixel, 0},
		{"receiver-below-FF-domain", 30 * Pixel, -256*Pixel - 1, 0},
		{"unproved-positive-upper-bound", 30 * Pixel, 1024 * Pixel, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			position := Position{X: 189*Pixel + 104, Depth: 86*Pixel + 4, Height: tc.source}
			got, err := LaunchPass(position, Position{X: 241 * Pixel, Depth: 77 * Pixel, Height: tc.target}, tc.arc)
			if err == nil || got != (Body{Position: position}) {
				t.Fatal("unproved/nonterminating input accepted or partially launched", got, err)
			}
		})
	}
}
