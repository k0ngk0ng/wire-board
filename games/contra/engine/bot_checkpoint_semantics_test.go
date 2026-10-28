package contra

// Empty omitempty audio maps have identical stored semantics. This must not
// hide a real public-HUD or rule-state change, and next-step replay must agree.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBotCheckpointComparesStoredSessionMeaning(t *testing.T) {
	s, err := NewSession(42, 2, "checkpoint-semantics")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored Session
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(*s, restored) {
		t.Fatal("fixture did not reproduce empty-map representation difference")
	}
	equal, err := botStoredSessionsEqual(s, &restored)
	if err != nil || !equal {
		t.Fatal("equivalent stored state rejected", err)
	}
	res := levelResources(t, "stage-1.json")
	for _, v := range []*Session{s, &restored} {
		if _, err = v.StepPads(1, [2]Buttons{}, res); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(*s, restored) {
		t.Fatal("next original step differs")
	}
	restored.PresentedLives[0]++
	if equal, err = botStoredSessionsEqual(s, &restored); err != nil || equal {
		t.Fatal("real submitted HUD change ignored", err)
	}
}
