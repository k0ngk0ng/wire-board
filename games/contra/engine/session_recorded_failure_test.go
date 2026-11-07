package contra

// Reproduce the recorded natural module failure from its original seed and
// legal input timestamps, including a save/load at the first reserve-loss edge.
// No original runtime snapshot, collision patch or terminal injection is used.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSessionRecordedStandardSharedFailure(t *testing.T) {
	var recording struct {
		Seed                      uint64 `json:"seed,string"`
		Epoch                     string `json:"epoch"`
		InitialAtMS               int64  `json:"initialAtMS"`
		TimerStepMS               int64  `json:"timerStepMS"`
		FinalFrame, FinalRevision uint64
		Inputs                    []struct {
			AtMS  int64
			Actor int
			Held  Buttons
		}
	}
	raw, err := os.ReadFile("../../../docs/research/nes-contra/evidence/natural-shared-failure-replay.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &recording); err != nil {
		t.Fatal(err)
	}
	if recording.TimerStepMS != 50 || len(recording.Inputs) == 0 || recording.FinalFrame == 0 || recording.FinalFrame > 12000 {
		t.Fatal("invalid recorded clock or input scope")
	}
	s, err := NewSession(recording.Seed, 2, recording.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	resources := levelResources(t, "stage-1.json")
	var restored *Session
	var queues [2][]Buttons
	var pads [2]Buttons
	index := 0
	for at := recording.InitialAtMS; s.Random.Frames < recording.FinalFrame; at += 50 {
		for index < len(recording.Inputs) && recording.Inputs[index].AtMS <= at {
			input := recording.Inputs[index]
			if input.AtMS != at || input.Actor < 0 || input.Actor > 1 {
				t.Fatal("input clock/order differs")
			}
			queues[input.Actor] = append(queues[input.Actor], input.Held)
			index++
		}
		for step := 0; step < 3 && s.Random.Frames < recording.FinalFrame; step++ {
			for actor := range pads {
				if len(queues[actor]) > 0 {
					pads[actor], queues[actor] = queues[actor][0], queues[actor][1:]
				}
			}
			before := s.World.Active.Combat.Players[0].Lives
			if _, err = s.StepPads(s.Random.Frames+1, pads, resources); err != nil {
				t.Fatal(err)
			}
			if restored != nil {
				if _, err = restored.StepPads(restored.Random.Frames+1, pads, resources); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(s, restored) {
					t.Fatal("recorded save/load replay diverged", s.Random.Frames)
				}
			} else if s.World.Active.Combat.Players[0].Lives < before {
				raw, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				restored = new(Session)
				if err = json.Unmarshal(raw, restored); err != nil {
					t.Fatal(err)
				}
				t.Logf("restored at natural reserve-loss frame %d", s.Random.Frames)
			}
		}
	}
	if index != len(recording.Inputs) || restored == nil || s.World.Campaign.Outcome != CampaignFailed {
		t.Fatal("recording did not reproduce natural shared failure")
	}
	for _, candidate := range []*Session{s, restored} {
		if _, err = candidate.FlushTerminal(); err != nil {
			t.Fatal(err)
		}
		if !candidate.Finalized || candidate.Revision != recording.FinalRevision {
			t.Fatal("recorded terminal boundary differs")
		}
	}
	if !reflect.DeepEqual(s, restored) {
		t.Fatal("restored terminal view differs")
	}
	t.Logf("replayed %d recorded actions through frame %d revision %d", index, s.Random.Frames, s.Revision)
}
