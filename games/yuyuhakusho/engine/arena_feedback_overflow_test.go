package engine

import (
	"strconv"
	"testing"
)

// An arbitrary int32 source must not wrap into a valid owned row address.
func TestArenaSceneFeedbackRejectsLongOverflow(t *testing.T) {
	for _, source := range []int32{0x7ffffffe, 0x7fffff00, -0x7fffffff - 1, -0x7ffffffe} {
		t.Run(strconv.FormatInt(int64(source), 10), func(t *testing.T) {
			defer func() {
				if value := recover(); value != nil {
					t.Fatalf("feedback panicked on unowned long source %d: %v", source, value)
				}
			}()
			s := ArenaSceneState{SourceA: -0x1800, SourceB: source}
			s.PanA[0] = 123
			got, err := ApplyArenaSceneFeedback(s, DefaultArenaView{Pan: 9})
			if err == nil || got != s {
				t.Fatal("overflowing feedback source accepted or partially applied", source, err)
			}
		})
	}
}
