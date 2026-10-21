package fortressblue

// Failure inventory before implementation: new rooms default to hard; old saves
// silently weaken; invalid levels enter state; hard aim changes; weaker bots
// exceed legal angle/power, change physics, or reroll their error on retries.
import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestBotDifficultyDefaultsAndRestore(t *testing.T) {
	raw, err := configure(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	var o Options
	_ = json.Unmarshal(raw, &o)
	if o.BotDifficulty != "normal" {
		t.Fatalf("new default: %+v", o)
	}
	for _, level := range []string{"easy", "normal", "hard"} {
		if _, err = configure(2, []byte(fmt.Sprintf(`{"botDifficulty":%q}`, level))); err != nil {
			t.Fatal(level, err)
		}
	}
	if _, err = configure(2, []byte(`{"botDifficulty":"expert"}`)); err == nil {
		t.Fatal("unknown difficulty accepted")
	}
	s, _ := startAt(t, 2, "stage-16")
	s.Options.BotDifficulty = ""
	m, _ := s.envelope()
	restored, err := restore(ctx(2, 0, time.Unix(1700000000, 0)), m)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := decode(restored)
	if got.Options.BotDifficulty != "hard" {
		t.Fatal("legacy save changed bot strength")
	}
}
func TestBotDifficultyAimErrorDeterministicAndLegal(t *testing.T) {
	s, _ := startAt(t, 2, "stage-16")
	spec, _ := tankSpec(1)
	base := botAim{Angle: 45, Power: 50, Facing: 1, Score: 1}
	totals := map[string]float64{}
	for i := 0; i < 100; i++ {
		s.Epoch = fmt.Sprint("difficulty-", i)
		s.Turn = i + 1
		for _, level := range []string{"easy", "normal", "hard"} {
			s.Options.BotDifficulty = level
			a := botAimError(s, 0, spec, base)
			b := botAimError(s, 0, spec, base)
			if a != b {
				t.Fatal("aim rerolled")
			}
			if a.Angle < float64(spec.MinAngle) || a.Angle > float64(spec.Angle) || a.Power < 0 || a.Power > 100 {
				t.Fatal("illegal aim")
			}
			totals[level] += math.Abs(a.Angle-base.Angle) + math.Abs(a.Power-base.Power)
			if level == "hard" {
				want := base
				want.Angle += float64(hashString(s.Epoch, "aim", string(rune(0)), string(rune(s.Turn)))%5) - 2
				if a != want {
					t.Fatalf("hard changed: %+v != %+v", a, want)
				}
			}
		}
	}
	if !(totals["easy"] > totals["normal"] && totals["normal"] > totals["hard"]) {
		t.Fatal(totals)
	}
}
