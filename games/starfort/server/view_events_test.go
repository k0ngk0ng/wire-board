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
	// Old archives can still contain the full ring. Read-only projection must
	// accept them without rewriting the saved bytes.
	legacy, _ := json.Marshal(s)
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	m.Data = legacy
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
	if len(out.Events) != 5 || out.Events[0].ID != 996 || out.Events[len(out.Events)-1].ID != 1000 {
		t.Fatalf("wire retained %d events, want last complete one-second batch", len(out.Events))
	}
	if !bytes.Equal(before, m.Data) {
		t.Fatal("view mutated save")
	}
	restored, err := decode(m)
	if err != nil || len(restored.Events) != 1000 {
		t.Fatal("save history lost", err)
	}
}

// Before persistence compaction: the cutoff drops a fractional-time attack;
// legacy tick-only events use a different window; an idle cursor disappears;
// expired targets remain reachable in the slice tail; pruning hides invalid
// old events; or a failed envelope mutates the caller's event history.
func TestEnvelopeReleasesOnlyExpiredPresentationEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []Event
		want   []int
	}{
		{"fractional_boundary", []Event{
			{ID: 1, Tick: 994},
			{ID: 2, Tick: 995, AtMS: eventTime(199000)},
			{ID: 3, Tick: 996, AtMS: eventTime(199001)},
			{ID: 4, Tick: 996, AtMS: eventTime(199200)},
			{ID: 5, Tick: 1000, AtMS: eventTime(200000)},
		}, []int{3, 4, 5}},
		{"legacy_tick", []Event{{ID: 1, Tick: 995}, {ID: 2, Tick: 996}}, []int{2}},
		{"idle_cursor", []Event{{ID: 1, Tick: 1}, {ID: 2, Tick: 2}}, []int{2}},
		{"empty", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := centralCase(t, 1)
			s.Tick, s.ElapsedMS = 1000, 200000
			s.Events = tc.events
			s.EventSerial = len(tc.events)
			backing := s.Events
			if len(backing) > 1 {
				backing[0].TargetID = 1
				backing[0].Target = &ViewEnemy{Enemy: Enemy{ID: 1}}
				s.EnemySerial = 1
			}
			m, err := s.Envelope()
			if err != nil {
				t.Fatal(err)
			}
			restored, err := decode(m)
			if err != nil {
				t.Fatal(err)
			}
			if len(restored.Events) != len(tc.want) {
				t.Fatalf("saved %d events, want IDs %v", len(restored.Events), tc.want)
			}
			for i, id := range tc.want {
				if restored.Events[i].ID != id {
					t.Fatalf("event %d: got %d want %d", i, restored.Events[i].ID, id)
				}
			}
			if len(backing) > len(s.Events) {
				for _, old := range backing[len(s.Events):] {
					if old.Target != nil || old.AtMS != nil || old.ID != 0 {
						t.Fatal("expired event remains reachable in the backing array")
					}
				}
			}
			if restored.EventSerial != len(tc.events) {
				t.Fatal("event sequence reset")
			}
		})
	}
	s := centralCase(t, 1)
	s.Tick, s.ElapsedMS, s.EventSerial = 1000, 200000, 2
	s.Events = []Event{{ID: 1, Tick: 1, TargetID: 1, Target: &ViewEnemy{Enemy: Enemy{ID: 2}}}, {ID: 2, Tick: 1000}}
	before, _ := json.Marshal(s.Events)
	if _, err := s.Envelope(); err == nil {
		t.Fatal("invalid expired event accepted")
	}
	after, _ := json.Marshal(s.Events)
	if !bytes.Equal(before, after) {
		t.Fatal("failed envelope discarded event data")
	}
}

func eventTime(at int64) *int64 { return &at }
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
