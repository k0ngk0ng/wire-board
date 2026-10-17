package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type motionSurfaceFixture struct {
	Frame    int
	Writes   []MotionSurfaceWrite
	Surfaces []uint8
}

func readMotionSurfaceFixture(t *testing.T, name string) []motionSurfaceFixture {
	t.Helper()
	file, err := os.Open("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []motionSurfaceFixture
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestMotionSurfaceEveryOriginalActionPhase(t *testing.T) {
	observations := readMotionSurfaceFixture(t, "motion-surface")
	stages := worldActionFixtures(t)
	if len(observations) != len(stages) {
		t.Fatal("incomplete motion/action correspondence")
	}
	count := 0
	for i, row := range stages {
		observed := observations[i]
		if row.Frame != observed.Frame {
			t.Fatal(i, row.Frame, observed.Frame)
		}
		step, err := StepWorldActions(row.Before, row.context(t))
		if err != nil {
			t.Fatal(i, err)
		}
		if !reflect.DeepEqual(step.MotionSurfaceWrites, observed.Writes) {
			t.Fatalf("phase %d frame %d physical surface writes\ngot %+v\nwant %+v", i, row.Frame, step.MotionSurfaceWrites, observed.Writes)
		}
		if step.State != row.After {
			t.Fatalf("surface instrumentation changed action state at %d", i)
		}
		count += len(step.MotionSurfaceWrites)
	}
	if count != 14456 {
		t.Fatal(count)
	}
	t.Logf("all %d original ordered surface writes match across %d complete action stages", count, len(stages))
}

func TestMotionSurfaceRiderLoopAndSkippedVisits(t *testing.T) {
	observations := readMotionSurfaceFixture(t, "motion-surface-rider")
	file, err := os.Open("testdata/world-loop-rider.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture worldLoopFixture
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(observations) != len(fixture.Records) {
		t.Fatal("incomplete rider motion correspondence")
	}
	state := fixture.Initial
	ctx := fixture.context(t)
	count := 0
	for i, row := range fixture.Records {
		step, err := StepWorldLoop(state, row.Input, ctx)
		if err != nil {
			t.Fatal(i, err)
		}
		if !reflect.DeepEqual(step.MotionSurfaceWrites, observations[i].Writes) {
			t.Fatalf("rider phase %d frame %d physical surface writes\ngot %+v\nwant %+v", i, row.InputFrame, step.MotionSurfaceWrites, observations[i].Writes)
		}
		if step.State != row.Expected {
			t.Fatal("surface event integration changed rider state", i)
		}
		state = step.State
		count += len(step.MotionSurfaceWrites)
	}
	if count != 389 {
		t.Fatal(count)
	}
	t.Logf("all %d rider physical surface writes match; attachment and no-motion revisit paths retain original absence", count)
}

func TestMotionSurfaceWritesAreTransactionalAndCallLocal(t *testing.T) {
	row := worldActionFixtures(t)[0]
	good, err := StepWorldActions(row.Before, row.context(t))
	if err != nil {
		t.Fatal(err)
	}
	saved := append([]MotionSurfaceWrite(nil), good.MotionSurfaceWrites...)
	broken := row.Before
	broken.Hoops[5].Action = HoopAction(127)
	failed, err := StepWorldActions(broken, row.context(t))
	if err == nil || failed.State != broken || len(failed.MotionSurfaceWrites) != 0 {
		t.Fatal("late failure exposed partial state/events", err)
	}
	again, err := StepWorldActions(row.Before, row.context(t))
	if err != nil || !reflect.DeepEqual(again.MotionSurfaceWrites, saved) {
		t.Fatal("another call retained failed-call events", err)
	}
	again.MotionSurfaceWrites[0].Origin.X++
	if !reflect.DeepEqual(good.MotionSurfaceWrites, saved) {
		t.Fatal("calls shared the event slice")
	}
}
