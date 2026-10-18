package engine

import (
	"encoding/json"
	"testing"
)

// Failure inventory: AI_LATE_DIFFICULTY_VERIFICATION.md, written before this
// source-driven acceptance. No change to AI or the match clock is assumed.
func TestOriginalAILateDifficultySource(t *testing.T) {
	var f struct {
		Calls  []aiTeamModeCase
		Chains []struct {
			Mode              AIMode
			Actor, Difficulty uint8
			Human             bool
			Records           []aiTeamModeCase
		}
	}
	readAITeamModeFixture(t, "ai-late-difficulty", &f)
	if len(f.Calls) != 5376 || len(f.Chains) != 448 {
		t.Fatal("incomplete source matrix", len(f.Calls), len(f.Chains))
	}
	for _, r := range f.Calls {
		state := r.Before
		applyAITeamModeCase(t, r, &state)
	}
	chainCalls := 0
	for _, c := range f.Chains {
		expected := 14 - int(c.Difficulty/4)*2
		if c.Human {
			expected = 10
		}
		if len(c.Records) != expected {
			t.Fatal("unexpected original handoff length", c.Mode, c.Actor, c.Difficulty, c.Human, len(c.Records), expected)
		}
		state := c.Records[0].Before
		for _, r := range c.Records {
			applyAITeamModeCase(t, r, &state)
			data, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored WorldInputState
			if err = json.Unmarshal(data, &restored); err != nil || restored != state {
				t.Fatal("JSON restore changed continuation", err)
			}
			state = restored
			chainCalls++
		}
		if c.Records[len(c.Records)-1].Before.Team.Players[c.Actor].Mode != 7 {
			t.Fatal("source engagement not consumed")
		}
	}
	t.Logf("matched %d original calls and %d continuous AI-only chains (%d calls), covering playable difficulties 0–27", len(f.Calls), len(f.Chains), chainCalls)
}
