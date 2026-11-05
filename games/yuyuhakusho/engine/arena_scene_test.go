package engine

import (
	"reflect"
	"testing"
)

func TestArenaSceneFeedbackViews(t *testing.T) {
	s := ArenaSceneState{SourceA: -0x1800, SourceB: -0x15e0}
	v := DefaultArenaView{Pan: 7}
	for i := range s.RowsA {
		s.RowsA[i] = int16(i)
		s.RowsB[i] = int16(1000 + i)
		v.HUDPrefix[i] = int16(-1000 + i)
	}
	before := s
	got, err := ApplyArenaSceneFeedback(s, v)
	if err != nil || s != before {
		t.Fatal("feedback error or caller mutation", err)
	}
	for i := 0; i < 224; i++ {
		if got.PanA[i] != int16(i+7) || got.PanB[i] != int16(1023+i) {
			t.Fatal("feedback exact source rows lost", i, got.PanA[i], got.PanB[i])
		}
	}
	if got.SourceA != -0xa000 || got.SourceB != -0x9e00 {
		t.Fatal("feedback output source identity lost")
	}
	s.SourceA = -0x1a00
	s.SourceB = -0x1600
	got, err = ApplyArenaSceneFeedback(s, v)
	if err != nil || got.PanA[0] != -993 || got.PanB[223] != 1230 {
		t.Fatal("feedback HUD or final word view lost", err)
	}
}

func TestArenaSceneFeedbackErrorsAreAtomic(t *testing.T) {
	for _, source := range []int32{-0x15bf, -0x1a02, 0x6800, -0x9800, -0x1400} {
		s := ArenaSceneState{SourceA: -0x1800, SourceB: source}
		s.PanA[0] = 123
		s.PanB[0] = -456
		got, err := ApplyArenaSceneFeedback(s, DefaultArenaView{Pan: 9})
		if err == nil || !reflect.DeepEqual(got, s) {
			t.Fatal("feedback accepted unowned view or leaked partial output", source, err)
		}
	}
	s := ArenaSceneState{SourceA: 123, SourceB: 456}
	s.PanA[0] = 77
	got, err := ApplyArenaSceneFeedback(s, DefaultArenaView{})
	if err != nil || got != s {
		t.Fatal("Pan0 accessed unused sources", err)
	}
}
