package contra

// Failure inventory: HUD exposes post-rule lives while OAM still shows the
// previous composition; restore loses that boundary; paused/terminal commits
// omit the HUD; retries or rejected frames change it. Natural injury below uses
// standard lives and legal rightward movement, without injecting damage or outcomes.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSessionPresentedLivesNaturalLossAndRestore(t *testing.T) {
	res := levelResources(t, "stage-1.json")
	s, err := NewSession(42, 1, "hud-natural")
	if err != nil {
		t.Fatal(err)
	}
	if s.PresentedLives != [2]uint8{2, 0} {
		t.Fatal("initial public reserves differ")
	}
	for frame := uint64(1); frame <= 6000; frame++ {
		before := [2]uint8{s.World.Active.Combat.Players[0].Lives, s.World.Active.Combat.Players[1].Lives}
		if _, err = s.StepPads(frame, [2]Buttons{ButtonRight, 0}, res); err != nil {
			t.Fatal(frame, err)
		}
		if s.PresentedLives != before {
			t.Fatalf("frame %d HUD advanced beyond submitted OAM", frame)
		}
		if s.World.Active.Combat.Players[0].Lives >= before[0] {
			continue
		}
		t.Logf("natural reserve loss at frame %d: submitted=%d pending=%d", frame, s.PresentedLives[0], s.World.Active.Combat.Players[0].Lives)
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var restored Session
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if restored.PresentedLives != before {
			t.Fatal("restore rewrote displayed HUD")
		}
		if _, err = s.StepPads(frame, [2]Buttons{}, res); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*s, restored) {
			t.Fatal("retry changed HUD or rules")
		}
		if _, err = s.StepPads(frame+2, [2]Buttons{}, res); err == nil {
			t.Fatal("skipped frame accepted")
		}
		if !reflect.DeepEqual(*s, restored) {
			t.Fatal("rejected frame changed HUD")
		}
		for _, candidate := range []*Session{s, &restored} {
			if _, err = candidate.StepPads(frame+1, [2]Buttons{}, res); err != nil {
				t.Fatal(err)
			}
			if candidate.PresentedLives[0] != before[0]-1 {
				t.Fatal("next display boundary lost reserve change")
			}
		}
		if !reflect.DeepEqual(*s, restored) {
			t.Fatal("restored display boundary diverged")
		}
		return
	}
	t.Fatal("standard rightward-input run did not encounter natural reserve loss")
}

func TestSessionPresentedLivesPausedAndTerminalBoundaries(t *testing.T) {
	res := levelResources(t, "stage-1.json")
	s, err := NewSession(42, 2, "hud-boundaries")
	if err != nil {
		t.Fatal(err)
	}
	for s.World.Active.Combat.World.LevelRoutine != 4 {
		if s.Random.Frames >= 1000 {
			t.Fatal("battle not reached")
		}
		if _, err = s.StepPads(s.Random.Frames+1, [2]Buttons{}, res); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.StepPads(s.Random.Frames+1, [2]Buttons{ButtonStart, 0}, res); err != nil {
		t.Fatal(err)
	}
	if !s.World.Campaign.Paused {
		t.Fatal("Start did not pause")
	}
	// Isolated boundary inputs: pending versus applied values differ. This
	// checks submission, not a claim that gameplay can lose lives while paused.
	s.PresentedLives = [2]uint8{9, 9}
	if _, err = s.PresentPaused(res); err != nil {
		t.Fatal(err)
	}
	want := [2]uint8{s.World.Active.Combat.Players[0].Lives, s.World.Active.Combat.Players[1].Lives}
	if s.PresentedLives != want {
		t.Fatal("paused commit omitted HUD")
	}
	s.World.Campaign.Paused = false
	s.World.Campaign.Outcome = CampaignFailed
	s.World.Active.Combat.World.GameRoutine = 0
	s.PresentedLives = [2]uint8{9, 9}
	if _, err = s.FlushTerminal(); err != nil {
		t.Fatal(err)
	}
	if s.PresentedLives != want {
		t.Fatal("terminal commit omitted HUD")
	}
}
