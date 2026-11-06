package rootcontrol_test

import (
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
)

func TestRootStateHasOnlyIndependentOwners(t *testing.T) {
	typeOf := reflect.TypeFor[rootcontrol.State]()
	want := []string{"Priority", "Selected", "ChoiceBits", "Fade", "PauseRestore"}
	if typeOf.NumField() != len(want) {
		t.Fatal("unexpected Root owner", typeOf)
	}
	for i, n := range want {
		if typeOf.Field(i).Name != n {
			t.Fatal("duplicated or missing Root field", i, typeOf.Field(i).Name)
		}
	}
	if typeOf.Field(3).Type != reflect.TypeFor[rootcontrol.FadeState]() {
		t.Fatal("fade counters were copied outside their component")
	}
	if typeOf.Field(4).Type != reflect.TypeFor[*rootcontrol.PauseLevels]() {
		t.Fatal("pause restore became another chip/score image")
	}
}

func TestRootInitialAndRestoreAreIndependent(t *testing.T) {
	s := rootcontrol.InitialState()
	if s.Selected != 255 || s.Priority != 0 || s.ChoiceBits != 0 || s.Fade != (rootcontrol.FadeState{}) || s.PauseRestore != nil {
		t.Fatal("invalid explicit Root start", s)
	}
	s.Priority = 83
	s.Selected = 5
	s.ChoiceBits = 1
	s.Fade = rootcontrol.FadeState{Steps: 0, Remaining: 255}
	s.PauseRestore = &rootcontrol.PauseLevels{}
	s.PauseRestore.Levels[4][2] = 37
	got, err := rootcontrol.RestoreState(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) || got.PauseRestore == s.PauseRestore {
		t.Fatal("Root restore aliases its caller or normalized counters")
	}
	s.PauseRestore.Levels[4][2] = 12
	if got.PauseRestore.Levels[4][2] != 37 {
		t.Fatal("pending levels had two mutable owners")
	}
}

func TestRootRestoreRejectsUnsupportedWithoutPartialState(t *testing.T) {
	for _, s := range []rootcontrol.State{{ChoiceBits: 2}, {PauseRestore: &rootcontrol.PauseLevels{Levels: [6][4]uint8{{128}}}}} {
		got, err := rootcontrol.RestoreState(s)
		if err == nil || !reflect.DeepEqual(got, rootcontrol.State{}) {
			t.Fatal("unsupported Root profile accepted", got, err)
		}
	}
}
