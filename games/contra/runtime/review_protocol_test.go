package runtime

// Contract written before implementation: a real committed battlefield must
// survive terminal black palette and JSON/checkpoint restore; rule state and
// terminal audio retain their actual time rather than replaying old effects.
import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeTerminalUsesPersistedBattleReview(t *testing.T) {
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	resources, _ := bundle.Stage(0)
	session, err := engine.NewSession(42, 1, "terminal-review")
	if err != nil {
		t.Fatal(err)
	}
	for session.Review == nil {
		if session.Random.Frames > 1000 {
			t.Fatal("battle missing")
		}
		if _, err = session.StepPads(session.Random.Frames+1, [2]engine.Buttons{}, resources); err != nil {
			t.Fatal(err)
		}
	}
	review := *session.Review
	// Isolated terminal boundary, distinct from the assisted natural clear E2E.
	session.World.Campaign.Outcome = engine.CampaignCleared
	session.World.Campaign.Loops = 1
	session.World.Active.Combat.World.Level = 8
	session.World.Active.Combat.World.GameRoutine = 6
	session.World.Active.Combat.World.LevelRoutine = 0
	for i := range session.World.Active.Presentation.Palette.Desired {
		session.World.Active.Presentation.Palette.Desired[i] = 15
	}
	session.World.Active.Presentation.Palette.RequestedColors = 32
	if _, err = session.FlushTerminal(); err != nil {
		t.Fatal(err)
	}
	code := strings.Repeat("a", 64)
	view, err := View(session, bundle.Manifest().ID, code)
	if err != nil {
		t.Fatal(err)
	}
	if view.Review == nil || view.Review.Frame != review.Frame || view.Review.Revision != review.Revision || !reflect.DeepEqual(view.Presentation, review.Presentation) || view.Players[0].Lives != review.Lives[0] {
		t.Fatal("terminal lost recorded battlefield")
	}
	for _, color := range session.World.Active.Presentation.Palette.Applied {
		if color != 15 {
			t.Fatal("review rewrote real final palette")
		}
	}
	if view.Audio.Tick != session.Audio.Tick || view.AudioTick != session.Audio.Tick || view.Frame != session.Random.Frames || view.Revision != session.Revision {
		t.Fatal("review rewound rule/audio clock")
	}
	raw, _ := json.Marshal(session)
	var restored engine.Session
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.Validate(); err != nil {
		t.Fatal(err)
	}
	again, err := View(&restored, bundle.Manifest().ID, code)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(view, again) {
		t.Fatal("saved terminal changed retained view or sound state")
	}
}
