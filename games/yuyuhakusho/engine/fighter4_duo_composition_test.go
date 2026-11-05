package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func fighter4DuoManifest(t *testing.T, kind string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/fighter4-composition-" + kind + "-duo-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass        bool
		VideoSteps  int    `json:"video_steps"`
		Entry       int    `json:"accepted_entry_video_limit"`
		Drain       int    `json:"drain_video_steps"`
		NoRAMWrites bool   `json:"no_ram_writes"`
		NaturalOnly bool   `json:"natural_only"`
		InputSHA    string `json:"input_sha256"`
		BaselineSHA string `json:"baseline_sha256"`
		Binding     struct {
			SourceTag         uint8  `json:"source_tag"`
			SourceObject      string `json:"source_object"`
			InputChannel      uint8  `json:"input_channel"`
			InputMode         int8   `json:"input_mode"`
			OtherSourceTag    uint8  `json:"other_source_tag"`
			OtherSourceObject string `json:"other_source_object"`
			OtherInputChannel uint8  `json:"other_input_channel"`
			OtherInputMode    int8   `json:"other_input_mode"`
		}
	}
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	b := m.Binding
	if !m.Pass || m.VideoSteps != 5502 || m.Entry != 5500 || m.Drain != 2 || !m.NoRAMWrites || !m.NaturalOnly ||
		m.InputSHA != "b303ca8622f90747052902f81b2b7f2bdde39f59e47feb6f36a6bf8287a6ac34" || m.BaselineSHA != "9f282f53a016d184d45052b30eaf8c482d4b568132f7979f7eebff4a7faa5917" ||
		b.SourceTag != 4 || b.SourceObject != "0x9000" || b.InputChannel != 1 || b.InputMode != 0 || b.OtherSourceTag != 3 || b.OtherSourceObject != "0x8400" || b.OtherInputChannel != 0 || b.OtherInputMode != 0 {
		t.Fatal("Duo original-control, full length, no-write or two-human source binding missing")
	}
}
func fighter4DuoIdentity(t *testing.T, id uint16, frame, exit int, raw string) {
	t.Helper()
	b := attackContactBytes(t, raw, 256)
	if id != 0x9000 || b[0x58] != 4 || b[0x76] != 1 || b[0xc9] != 0 || frame <= 0 || frame > 5500 || exit < frame || exit > 5502 {
		t.Fatal("Duo actual source4/channel1 natural return or bounded full exit missing", id, frame, exit)
	}
}

type fighter4DuoLife struct{ first, last, boost, swallow, recovered int }

func (l *fighter4DuoLife) observe(frame int, before, after ChuActionState) {
	if before.MovementSource == 0x12578 && after.MovementSource == 0x15946 {
		l.swallow++
	}
	if before.MovementSource == 0x15946 || after.MovementSource == 0x15946 {
		if l.first == 0 {
			l.first = frame
		}
		l.last = frame
		l.boost++
	}
	if l.first != 0 && frame > l.last && before.MovementSource == 0x12578 && after.MovementSource == 0x12578 && after.Energy < 0 && after.Body.ActionKind == 0 && after.Children[0].Size == 0 {
		l.recovered++
	}
}
func (l fighter4DuoLife) require(t *testing.T) {
	t.Helper()
	if l.swallow == 0 || l.boost < 2 || l.recovered == 0 || l.last <= l.first {
		t.Fatal("Duo natural swallow, active boost and later default/energy/idle recovery absent", l)
	}
	t.Logf("natural Duo swallow=%d active boost rows=%d frames=%d..%d recovered rows=%d", l.swallow, l.boost, l.first, l.last, l.recovered)
}
func TestFighter4DuoCompositionOriginalActions(t *testing.T) {
	fighter4DuoManifest(t, "action")
	commands := actionCommandLibrary(t)
	life := fighter4DuoLife{}
	count, _ := walkChuActor(t, "fighter4-composition-action-duo", func(d *json.Decoder, library AnimationLibrary) {
		var row struct {
			fighter4CompositionAction
			Frame int
			Exit  int `json:"exit_frame"`
		}
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		fighter4DuoIdentity(t, row.ObjectID, row.Frame, row.Exit, row.Before.Tail.ActorBytes)
		checkFighter4CompositionAction(t, row.fighter4CompositionAction, library, commands)
		life.observe(row.Frame, decodeChuAction(t, row.Before, row.ObjectID), decodeChuAction(t, row.After, row.ObjectID))
	})
	if count < 2000 {
		t.Fatal("Duo full natural action evidence truncated", count)
	}
	life.require(t)
	t.Logf("Duo original full actions=%d", count)
}
func TestFighter4DuoCompositionOriginalInputs(t *testing.T) {
	fighter4DuoManifest(t, "input")
	routes := map[string]int{}
	boostRows := 0
	count, _ := walkChuActor(t, "fighter4-composition-input-duo", func(d *json.Decoder, _ AnimationLibrary) {
		var row struct {
			fighter4CompositionInput
			Frame int
			Exit  int `json:"exit_frame"`
		}
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		fighter4DuoIdentity(t, row.ObjectID, row.Frame, row.Exit, row.Before.AI.Actor)
		checkFighter4CompositionInput(t, row.fighter4CompositionInput)
		if row.AICalls != 0 || row.RNGCalls != 0 || (row.Route != "human" && row.Route != "disabled") {
			t.Fatal("Duo human control unexpectedly executes AI/recording", row.Route)
		}
		actor := attackContactBytes(t, row.Before.AI.Actor, 256)
		if actor[0x90] == 0 && actor[0x91] == 1 && actor[0x92] == 0x59 && actor[0x93] == 0x46 {
			boostRows++
		}
		routes[row.Route]++
	})
	if count < 2000 || routes["human"] == 0 || boostRows < 2 {
		t.Fatal("Duo human input full boost interval missing", count, routes, boostRows)
	}
	t.Logf("Duo original full inputs=%d boost rows=%d routes=%v AI=0", count, boostRows, routes)
}
func TestFighter4DuoCompositionOriginalActors(t *testing.T) {
	fighter4DuoManifest(t, "actor")
	commands := actionCommandLibrary(t)
	life := fighter4DuoLife{}
	count, _ := walkChuActor(t, "fighter4-composition-actor-duo", func(d *json.Decoder, library AnimationLibrary) {
		var row struct {
			fighter4CompositionActor
			AICalls int `json:"ai_calls"`
			Frame   int
			Exit    int `json:"exit_frame"`
		}
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		fighter4DuoIdentity(t, row.ObjectID, row.Frame, row.Exit, row.Before.Action.Tail.ActorBytes)
		checkFighter4CompositionActor(t, row.fighter4CompositionActor, library, commands)
		before, after := decodeChuAction(t, row.Before.Action, row.ObjectID), decodeChuAction(t, row.After.Action, row.ObjectID)
		if (before.MovementSource == 0x15946 || after.MovementSource == 0x15946) && (row.View.Pause&128 != 0 || row.View.Slow != 0) {
			t.Fatal("Duo boost evidence is not active")
		}
		if row.AICalls != 0 {
			t.Fatal("Duo human actor unexpectedly executes AI")
		}
		life.observe(row.Frame, before, after)
	})
	if count < 2000 {
		t.Fatal("Duo full natural actor evidence truncated", count)
	}
	life.require(t)
	t.Logf("Duo original full actors=%d", count)
}
