package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"testing"
)

type arenaWorkCase struct {
	Name, Kind    string
	Before, After defaultArenaSnapshot
	Entry         [2]uint64
	Steps         [][2]uint64
}

func arenaWorkFixture(t *testing.T) (ArenaRuntimeLibrary, []arenaWorkCase) {
	t.Helper()
	packed, err := os.ReadFile("../verification/world-arena-work/native-arena-work.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version int
		Library ArenaRuntimeLibrary
		Cases   []arenaWorkCase
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 792 {
		t.Fatal("original complete callback coverage missing")
	}
	return fixture.Library, fixture.Cases
}
func TestArenaWorkOriginalClocksAndRules(t *testing.T) {
	library, rows := arenaWorkFixture(t)
	steps := 0
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			clock := row.Entry
			index := 0
			hook := func(op ArenaWorkOperation) error {
				if index >= len(row.Steps) {
					t.Fatal("extra work", op)
				}
				cycle, refresh, err := op.Advance(clock[0], clock[1])
				if err != nil {
					return err
				}
				clock = [2]uint64{cycle, refresh}
				if clock != row.Steps[index] {
					t.Fatalf("step %d phase %s clock=%v native=%v work=%+v", index, op.Phase, clock, row.Steps[index], op)
				}
				index++
				return nil
			}
			var result defaultArenaSnapshot
			if row.Kind == "camera" {
				before := decodeArenaCamera(t, row.Before)
				got, err := StepDefaultArenaCameraObserved(before, hook)
				if err != nil {
					t.Fatal(err)
				}
				result = encodeArenaCamera(row.Before, got)
			} else {
				before, view := decodeDefaultArena(t, row.Before)
				got, err := StepDefaultArenaObserved(before, view, library, hook)
				if err != nil {
					t.Fatal(err)
				}
				result = encodeDefaultArena(t, row.Before, got)
			}
			if index != len(row.Steps) || !reflect.DeepEqual(result, row.After) {
				t.Fatal("complete callback/clock differs", index, len(row.Steps))
			}
			steps += index
		})
	}
	t.Logf("original complete callbacks=%d atomic work steps=%d", len(rows), steps)
}
func TestArenaWorkFailuresAreAtomic(t *testing.T) {
	library, _ := arenaWorkFixture(t)
	state := DefaultArenaState{Stage: 2, Camera: DefaultArenaCamera{X: 864, Y: 16}}
	view := DefaultArenaView{Pan: 1}
	sentinel := errors.New("world work failure")
	for _, failAt := range []int{0, 10, 100, 1000} {
		calls := 0
		got, err := StepDefaultArenaObserved(state, view, library, func(op ArenaWorkOperation) error {
			index := calls
			calls++
			if index == failAt {
				return sentinel
			}
			return nil
		})
		if !errors.Is(err, sentinel) || got != state || calls != failAt+1 {
			t.Fatal("work failure leaked state or called again", failAt, calls, err)
		}
	}
	bad := state
	bad.Camera.Used = 2
	if got, err := StepDefaultArenaObserved(bad, view, library, nil); err == nil || got != bad {
		t.Fatal("invalid camera accepted")
	}
	missing := library
	missing.Tails = nil
	if got, err := StepDefaultArenaObserved(state, view, missing, nil); err == nil || got != state {
		t.Fatal("missing tail accepted")
	}
	for _, op := range []ArenaWorkOperation{{}, {Phase: "invalid", CPUCycles: 65535}, {Phase: "invalid", CPUCycles: 1, InternalRefresh: true}} {
		if _, _, err := op.Advance(0, 0); err == nil {
			t.Fatal("malformed work accepted", op)
		}
	}
	op := ArenaWorkOperation{Phase: "return", CPUCycles: 16}
	if _, _, err := op.Advance(math.MaxUint64-1, math.MaxUint64); err == nil {
		t.Fatal("cycle overflow accepted")
	}
	op = ArenaWorkOperation{Phase: "camera-divide", CPUCycles: 136, InternalRefresh: true}
	if _, _, err := op.Advance(math.MaxUint64-1000, math.MaxUint64-100); err == nil {
		t.Fatal("refresh overflow accepted")
	}
}
func TestArenaWorkActualWorldInterval(t *testing.T) {
	library, _ := arenaWorkFixture(t)
	rules := ChuWorldLibrary{Arena: library}
	world := ChuWorldState{Objects: WorldObjects{0x8180: {ObjectID: 0x8180, Header: MatchObjectHeader{Behavior: 0x20918, Size: 64, Stage: 2}}}, Camera: DefaultArenaCamera{X: 864, Y: 16}, SoundQueue: [4]uint8{4}}
	plain, events, err := StepChuWorldObjects(world, ChuWorldView{}, rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []bool{false, true} {
		state := world
		state.Objects = cloneWorldObjects(world.Objects)
		if failure {
			state.Objects[0x9fc0] = WorldObject{ObjectID: 0x9fc0, Header: MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
		}
		before, _ := json.Marshal(state)
		calls := 0
		sounds := 0
		got, seen, err := StepChuWorldObjectsWithHooks(state, ChuWorldView{}, rules, ChuWorldHooks{Sound: func(_ WorldSoundOperation, _ WorldSoundOwners, apply func() error) error { sounds++; return apply() }, Work: func(op WorldWorkOperation, owners WorldSoundOwners) error {
			if op.ObjectID != 0x8180 {
				t.Fatal(op)
			}
			calls++
			if calls == 100 {
				*owners.Queue = [4]uint8{0x91, 0x92}
				*owners.AudioPause = 0x80
			}
			return nil
		}})
		if calls < 500 || sounds != 0 {
			t.Fatal("missing request-free interior work", calls, sounds)
		}
		if failure {
			if err == nil || seen != nil || !reflect.DeepEqual(got, state) {
				t.Fatal("late world failure not atomic", err)
			}
		} else {
			want := plain
			want.SoundQueue = [4]uint8{0x91, 0x92}
			want.Clock.AudioPause = 0x80
			if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(seen, events) {
				t.Fatal("world work changed rules or overwrote sound", err)
			}
		}
		after, _ := json.Marshal(state)
		if string(before) != string(after) {
			t.Fatal("mutated caller")
		}
	}
}
