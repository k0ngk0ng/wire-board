package runtimebridge

// Failure inventory, written before bridge changes: confirmed match stops an
// active reward; completion flush overwrites a post-result image; batch keeps
// advancing after terminal handoff; reconnect replays old reward/audio; replay
// stops before presentation frames; no-campaign legacy results start a route.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

func TestBridgeEarnedPresentationLifecycle(t *testing.T) {
	raw, err := os.ReadFile("../engine/testdata/campaign-home-entry.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Config       engine.OriginalMatchConfiguration
		HumanTactics [2]uint8
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	progress, err := campaign.DecodePassword("020010")
	if err != nil {
		t.Fatal(err)
	}
	progress, err = campaign.Equip(progress, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	state, err := engine.NewCampaignRuntime(progress, engine.CampaignMatchSelection{Opponent: f.Config.Teams[1], Characters: f.Config.Characters, HumanTactics: f.HumanTactics, Seed: f.Config.Seed})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 48000 && (state.Outcome == nil || !state.Outcome.Confirmed); i++ {
		held := [4]engine.Buttons{}
		if state.AwaitingNextPeriod && state.WaitVideoFrames == 0 {
			held[0] = engine.ButtonStart
		}
		step, e := engine.StepVideo(state, held)
		if e != nil {
			t.Fatal(state.VideoFrames, e)
		}
		state = step.State
	}
	if !engine.RuntimePresentationActive(state) || engine.RuntimeFinished(state) || state.VideoFrames < 21600 {
		t.Fatal("no earned reward boundary", state.VideoFrames)
	}
	start := state
	b := New()
	created := b.create(state)
	id := created.Data.(View).Session
	advance := func(ticks int) View {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"op": "advance", "session": id, "ticks": ticks})
		r := b.Handle(raw)
		if !r.OK {
			t.Fatal(r.Error)
		}
		return r.Data.(View)
	}
	expected := state
	for i := 0; i < 60; i++ {
		step, e := engine.StepVideo(expected, [4]engine.Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		expected = step.State
	}
	got := advance(60)
	if !reflect.DeepEqual(got.State, expected) {
		t.Fatal("bridge stopped at confirmed result or changed presentation state", got.State.VideoFrames, expected.VideoFrames)
	}
	middle := got.State
	replayBridge := New()
	replayID := replayBridge.create(start).Data.(View).Session
	inputs := []engine.RecordedVideoInput{}
	cursor := start
	for i := 0; i < 60; i++ {
		input := engine.RecordedVideoInput{Frame: cursor.VideoFrames + 1, PriorHeld: cursor.Held, PendingPressed: cursor.PendingPressed}
		inputs = append(inputs, input)
		step, e := engine.StepRecordedVideo(cursor, input)
		if e != nil {
			t.Fatal(e)
		}
		cursor = step.State
	}
	request, _ := json.Marshal(map[string]any{"op": "replay", "session": replayID, "inputs": inputs})
	replayed := replayBridge.Handle(request)
	if !replayed.OK {
		t.Fatal(replayed.Error)
	}
	if v := replayed.Data.(View); len(v.Frames) != 60 || !reflect.DeepEqual(v.State, expected) || len(v.SoundIDs) != 0 {
		t.Fatal("replay differs or repeats audio")
	}
	finish := advance(600)
	if !engine.RuntimeFinished(finish.State) || finish.State.VideoFrames <= middle.VideoFrames {
		t.Fatal("reward did not reach terminal handoff")
	}
	frozen := advance(600)
	if !reflect.DeepEqual(finish.State, frozen.State) || len(frozen.SoundIDs) != 0 || frozen.LogicUpdates != 0 {
		t.Fatal("completed reward restarted")
	}
	saved, _ := json.Marshal(Checkpoint{Version: 1, State: middle})
	raw, _ = json.Marshal(map[string]any{"op": "restore", "checkpoint": json.RawMessage(saved)})
	restored := b.Handle(raw)
	if !restored.OK {
		t.Fatal(restored.Error)
	}
	if v := restored.Data.(View); !reflect.DeepEqual(v.State, middle) || len(v.SoundIDs) != 0 {
		t.Fatal("restore replayed instead of resuming")
	}
	fixture := map[string]any{"start": start, "middle": middle, "finish": finish.State, "stepsToMiddle": 60, "inputs": inputs, "scope": "earned original configured match, reward playback; not a 14-win ending or original-console timing oracle"}
	data, _ := json.Marshal(fixture)
	out := "../../../.local/presentation-client"
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "earned-reward.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("earned reward video %d -> %d -> %d", start.VideoFrames, middle.VideoFrames, finish.State.VideoFrames)
}
