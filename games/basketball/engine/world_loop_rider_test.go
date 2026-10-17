package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestWorldLoopNaturalRiderWithActiveInput(t *testing.T) {
	f, err := os.Open("testdata/world-loop-rider.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture worldLoopFixture
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Records) != 57 {
		t.Fatal("missing complete active input updates")
	}
	state := fixture.Initial
	ctx := fixture.context(t)
	for i, row := range fixture.Records {
		step, err := StepWorldLoop(state, row.Input, ctx)
		if err != nil {
			t.Fatalf("update %d frame %d: %v", i, row.InputFrame, err)
		}
		if step.State != row.Expected {
			t.Fatalf("update %d frame %d: %s", i, row.InputFrame, firstWorldLoopDifference(reflect.ValueOf(step.State), reflect.ValueOf(row.Expected), "state"))
		}
		state = step.State
	}
	t.Log("all 57 completed natural updates matched with one initialization and active human inputs; final partial capture boundary retained separately")
}
