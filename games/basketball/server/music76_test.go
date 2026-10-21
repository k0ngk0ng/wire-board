package basketball_test

// Failure inventory before implementation:76 retains its old empty revision,
// restore resets the cumulative wall clock, pause time advances, or new76 fails
// to restart its intro epoch. Legacy empty-revision76 must not acquire a guessed
// asset merely because effects pass through its saved state.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"reflect"
	"testing"
)

func TestMusic76VersionedMediaClockAndRestore(t *testing.T) {
	var s game.MusicState
	if e := s.Apply([]int{76}, 1, 1000); e != nil {
		t.Fatal(e)
	}
	if len(s.AssetSHA256) != 64 || s.PositionMS(1901500) != 1900500 {
		t.Fatal("unversioned76 or wrong wall time", s)
	}
	raw, _ := json.Marshal(s)
	var restored game.MusicState
	if e := json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s, restored) || restored.PositionMS(1901500) != 1900500 {
		t.Fatal("restore changed timeline")
	}
	if e := restored.Apply([]int{253}, 2, 1901500); e != nil {
		t.Fatal(e)
	}
	if restored.PositionMS(2901500) != 1900500 {
		t.Fatal("paused position changed")
	}
	if e := restored.Apply([]int{255}, 3, 2901500); e != nil {
		t.Fatal(e)
	}
	if restored.PositionMS(2902500) != 1901500 {
		t.Fatal("resume lost wall time")
	}
	if e := restored.Apply([]int{76}, 4, 3000000); e != nil {
		t.Fatal(e)
	}
	if restored.Epoch != 2 || restored.PositionMS(3000100) != 100 || restored.AssetSHA256 != s.AssetSHA256 {
		t.Fatal("new76 failed fresh intro epoch", restored)
	}
}
func TestMusic76LegacyUnversionedStateStaysUnboundUntilNewRequest(t *testing.T) {
	s := game.MusicState{Epoch: 1, TrackID: 76, ControlSequence: 1, StartedAt: 1000}
	if e := s.Apply([]int{19}, 2, 2000); e != nil {
		t.Fatal(e)
	}
	if s.AssetSHA256 != "" || s.PositionMS(2000) != 1000 {
		t.Fatal("effect guessed a76 migration", s)
	}
	if e := s.Apply([]int{76}, 3, 3000); e != nil {
		t.Fatal(e)
	}
	if len(s.AssetSHA256) != 64 || s.Epoch != 2 || s.PositionMS(3100) != 100 {
		t.Fatal("new76 did not bind verified delivery", s)
	}
}
