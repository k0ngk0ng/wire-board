package starfort

// Failure inventory before implementation: old combat history grows every wire
// snapshot; recent complete batches get truncated; reconnect loses the cursor;
// old saves without atMS are filtered wrongly; projection mutates saved state.
import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestViewBoundsHistoricalEventsWithoutChangingMatch(t *testing.T) {
	s := centralCase(t, 1)
	s.Tick = 1000
	s.ElapsedMS = 200000
	for i := 1; i <= 1000; i++ {
		at := int64(i) * 200
		s.Events = append(s.Events, Event{ID: i, Tick: int64(i), AtMS: &at, Type: "shot"})
	}
	s.EventSerial = 1000
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte{}, m.Data...)
	v, err := view(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	var out struct {
		Events []Event `json:"events"`
	}
	json.Unmarshal(raw, &out)
	if len(out.Events) != 16 || out.Events[0].ID != 985 || out.Events[len(out.Events)-1].ID != 1000 {
		t.Fatalf("wire retained %d events, want last 3 seconds", len(out.Events))
	}
	if !bytes.Equal(before, m.Data) {
		t.Fatal("view mutated save")
	}
	restored, err := decode(m)
	if err != nil || len(restored.Events) != 1000 {
		t.Fatal("save history lost", err)
	}
}
func TestViewKeepsIdleCursorAndLegacyRecentEvents(t *testing.T) {
	s := centralCase(t, 1)
	s.Tick = 1000
	s.ElapsedMS = 200000
	s.EventSerial = 2
	s.Events = []Event{{ID: 1, Tick: 1, Type: "build"}, {ID: 2, Tick: 2, Type: "build"}}
	m, _ := s.Envelope()
	v, _ := view(m, 0)
	raw, _ := json.Marshal(v)
	var out struct {
		Events []Event `json:"events"`
	}
	json.Unmarshal(raw, &out)
	if len(out.Events) != 1 || out.Events[0].ID != 2 {
		t.Fatal("idle cursor lost", out.Events)
	}
	s.Events = append(s.Events, Event{ID: 3, Tick: 999, Type: "shot"})
	s.EventSerial = 3
	m, _ = s.Envelope()
	v, _ = view(m, 0)
	raw, _ = json.Marshal(v)
	json.Unmarshal(raw, &out)
	if len(out.Events) != 1 || out.Events[0].ID != 3 {
		t.Fatal("legacy event timing not respected")
	}
}

func TestWireViewOmitsSaveOnlyBookkeepingAndDuplicateMap(t *testing.T) {
	s := centralCase(t, 2)
	m, _ := s.Envelope()
	v, _ := view(m, 0)
	raw, _ := json.Marshal(v)
	var wire map[string]json.RawMessage
	json.Unmarshal(raw, &wire)
	for _, key := range []string{"spawnClock", "towerSerial", "enemySerial", "eventSerial", "craterSerial", "cursor", "log"} {
		if _, ok := wire[key]; ok {
			t.Errorf("save-only field on wire: %s", key)
		}
	}
	var world map[string]json.RawMessage
	json.Unmarshal(wire["world"], &world)
	if _, ok := world["map"]; ok {
		t.Error("map sent twice")
	}
	if len(wire["map"]) < 100 || len(wire["controls"]) < 10 {
		t.Fatal("playable state omitted")
	}
}
