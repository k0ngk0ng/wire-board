package contra

// Failure inventory before persisted review: only a temporary browser Canvas
// survives terminal fade; pending rather than applied pixels/HUD are retained;
// capture changes rules/RNG/audio/controls, aliases later frames, or advances on
// retries; pause/JSON restore loses capture; invalid future markers pass; fading
// exit frames or original title/menu overwrite the last real battlefield.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSessionBattleReviewIsCommittedAndRestorable(t *testing.T) {
	resources := levelResources(t, "stage-1.json")
	session, err := NewSession(42, 1, "battle-review")
	if err != nil {
		t.Fatal(err)
	}
	for session.Review == nil {
		if session.Random.Frames > 1000 {
			t.Fatal("standard opening did not retain a battlefield")
		}
		if _, err = session.StepPads(session.Random.Frames+1, [2]Buttons{}, resources); err != nil {
			t.Fatal(err)
		}
	}
	review := *session.Review
	if review.Frame != session.Random.Frames || review.Revision != session.Revision || review.Level != 0 || review.Lives != session.PresentedLives || !reflect.DeepEqual(review.Presentation, session.World.Active.Presentation.Snapshot()) {
		t.Fatal("review is not the actually submitted snapshot")
	}
	raw, _ := json.Marshal(session)
	var restored Session
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err = session.StepPads(session.Random.Frames, [2]Buttons{}, resources); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(session)
	if string(raw) != string(again) {
		t.Fatal("duplicate changed capture")
	}
	for _, candidate := range []*Session{session, &restored} {
		if _, err = candidate.StepPads(candidate.Random.Frames+1, [2]Buttons{}, resources); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(session.Review, restored.Review) || reflect.DeepEqual(session.Review, &review) {
		t.Fatal("restored capture did not advance identically")
	}
	if !reflect.DeepEqual(review.Presentation, restored.Review.Presentation) && review.Frame != session.Random.Frames-1 {
		t.Fatal("prior immutable capture was altered")
	}
	for _, mode := range []string{"frame", "revision", "level", "routine", "pixels"} {
		copy := *session
		record := *session.Review
		copy.Review = &record
		switch mode {
		case "frame":
			record.Frame = session.Random.Frames + 1
		case "revision":
			record.Revision = session.Revision + 1
		case "level":
			record.Level = 8
		case "routine":
			record.Routine = 0
		case "pixels":
			record.Presentation.Patterns[0] = 255
		}
		if copy.Validate() == nil {
			t.Fatal("invalid review accepted", mode)
		}
	}
}
func TestSessionBattleReviewSelectionRejectsFadeAndMenus(t *testing.T) {
	for _, tc := range []struct {
		routine, exit uint8
		want          bool
	}{{0, 0, false}, {1, 0, false}, {4, 0, true}, {5, 0, false}, {6, 0, false}, {8, 0, true}, {9, 0, true}, {9, 2, true}, {9, 3, false}, {10, 0, false}, {11, 0, true}} {
		session, _ := NewSession(1, 1, "selection")
		session.World.Active.Combat.World.LevelRoutine = tc.routine
		session.World.Campaign.EndSequence = tc.exit
		if session.battleReviewCandidate() != tc.want {
			t.Fatal(tc, "wrong battlefield classification")
		}
	}
}
