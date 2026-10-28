package contra

// Production entry gate before implementation: no reference RAM initializes a
// new match; original reserve lives and solo/co-op mapping; entropy advances once
// per accepted frame, never per retry; checkpoint and audio identities survive;
// presentation/audio submit before rules; invalid/skipped frames are atomic.
import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestNewSessionRunsAndRestores(t *testing.T) {
	resources := levelResources(t, "stage-1.json")
	for _, players := range []int{1, 2} {
		left, e := NewSession(math.MaxUint64, players, "room-match-1")
		if e != nil {
			t.Fatal(e)
		}
		if left.World.Active.Combat.Players[0].Lives != 2 || left.World.Campaign.Continues != 3 || left.World.Active.Combat.World.PlayerMode != uint8(players-1) {
			t.Fatal("wrong original resources")
		}
		if (left.World.Active.Combat.Players[1].GameOver != 0) != (players == 1) {
			t.Fatal("solo actor mapping")
		}
		raw, _ := json.Marshal(left)
		var right Session
		if e = json.Unmarshal(raw, &right); e != nil {
			t.Fatal(e)
		}
		seenBattle := false
		seenMusic := false
		frames := 900
		if players == 2 {
			frames = 400
		}
		for frame := uint64(1); frame <= uint64(frames); frame++ {
			var inputs [2]PlayerInput
			if players == 2 && frame == 2 {
				inputs[0] = PlayerInput{Held: ButtonStart, Pressed: ButtonStart}
			}
			if left.World.Active.Combat.World.LevelRoutine == 4 {
				seenBattle = true
				if frame == 160 || frame == 175 {
					inputs[0] = PlayerInput{Held: ButtonStart, Pressed: ButtonStart}
				}
			}
			one, e := left.Step(frame, inputs, resources)
			if e != nil {
				t.Fatal(players, frame, e)
			}
			two, e := right.Step(frame, inputs, resources)
			if e != nil || !reflect.DeepEqual(one, two) || !reflect.DeepEqual(*left, right) {
				t.Fatal(players, frame, "restore/replay diverged", e)
			}
			if left.Random.Frames != frame || left.World.Campaign.LastTick != frame || left.Audio.Tick != frame {
				t.Fatal("frame owners drifted")
			}
			if len(one.Audio.UnsupportedIDs) != 0 {
				t.Fatal("unresolved legal sound", one.Audio.UnsupportedIDs)
			}
			for _, id := range one.Audio.View.SlotIDs {
				if id == 0x46 {
					seenMusic = true
				}
			}
			if frame%23 == 0 {
				raw, _ = json.Marshal(right)
				if e = json.Unmarshal(raw, &right); e != nil {
					t.Fatal(e)
				}
			}
			before, _ := json.Marshal(left)
			duplicate, e := left.Step(frame, inputs, resources)
			if e != nil || !duplicate.Duplicate || len(duplicate.Audio.Events) != 0 {
				t.Fatal("retry replayed effects", e)
			}
			after, _ := json.Marshal(left)
			if string(before) != string(after) {
				t.Fatal("retry consumed state")
			}
			if _, e = left.Step(frame+2, inputs, resources); e == nil {
				t.Fatal("skipped frame accepted")
			}
			after, _ = json.Marshal(left)
			if string(before) != string(after) {
				t.Fatal("invalid frame changed state")
			}
		}
		if !seenBattle || !seenMusic {
			t.Fatal("new game did not reach real battle/music", players)
		}
	}
}

func TestSessionValidationAndAudioProjection(t *testing.T) {
	for _, players := range []int{-1, 0, 3} {
		if _, e := NewSession(1, players, "epoch"); e == nil {
			t.Fatal("invalid seats accepted")
		}
	}
	if _, e := NewSession(1, 2, ""); e == nil {
		t.Fatal("empty audio epoch accepted")
	}
	for _, sample := range []struct{ locked, reset, addClear, want bool }{{false, false, false, false}, {true, false, false, true}, {true, true, false, false}, {false, false, true, true}, {true, true, true, true}} {
		a := NewAudioState("projection")
		if sample.locked {
			a.Slots[1].ID = 0x62
		}
		prefix := EnemyAudioPrefix{Reset: sample.reset}
		if sample.addClear {
			prefix.Sounds = []uint8{0x62}
		}
		before, _ := json.Marshal(a)
		carry, e := projectEnemySound(&a, 0x2b, prefix)
		if e != nil || carry != sample.want {
			t.Fatal(sample, carry, e)
		}
		after, _ := json.Marshal(a)
		if string(before) != string(after) {
			t.Fatal("projection queued actual sound")
		}
	}
}
