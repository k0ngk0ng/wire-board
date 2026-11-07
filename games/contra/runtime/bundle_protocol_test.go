package runtime

// Bundle protocol failures, written before stage installation: one-stage JSON
// hash replaces the stable bundle identity; missing/wrong stage data advances a
// session; installing another stage rewrites existing maps; restore implicitly
// preloads all eight maps; a changed manifest rebinds a live session.
import (
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"testing"
)

func TestRuntimeBundleStageInstallationAndRestore(t *testing.T) {
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	manifest := bundle.Manifest()
	build := testRulesBuild(t)
	b := New(build)
	if _, err = b.Call(Request{Op: "bundle", Bundle: &manifest}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Call(Request{Op: "new", Seed: "1", Players: 1, Epoch: "bundle-one"}); err == nil {
		t.Fatal("missing initial stage accepted")
	}
	first, _ := bundle.Payload(0)
	second, _ := bundle.Payload(1)
	if _, err = b.Call(Request{Op: "stage", Level: 0, StageData: string(first)}); err != nil {
		t.Fatal(err)
	}
	value, err := b.Call(Request{Op: "new", Seed: "1", Players: 1, Epoch: "bundle-one"})
	if err != nil {
		t.Fatal(err)
	}
	view := value.(Snapshot)
	if view.Resources != manifest.ID || len(b.stages) != 1 {
		t.Fatal("stage hash replaced bundle or all maps loaded")
	}
	for _, request := range []Request{{Op: "stage", Level: 0, StageData: string(second)}, {Op: "stage", Level: 0, StageData: string(first) + " "}} {
		if _, err = b.Call(request); err == nil {
			t.Fatal("wrong stage payload accepted")
		}
	}
	changed := manifest
	changed.ID = "different"
	if _, err = b.Call(Request{Op: "bundle", Bundle: &changed}); err == nil {
		t.Fatal("changed bundle accepted")
	}
	if _, err = b.Call(Request{Op: "bundle", Bundle: &manifest}); err != nil {
		t.Fatal(err)
	}
	after, err := b.Call(Request{Op: "snapshot", Session: view.Session, Epoch: view.Epoch})
	if err != nil || after.(Snapshot).Digest != view.Digest {
		t.Fatal("stage installation mutated current session", err)
	}
	// A valid controlled initial state at the next stage tests resource admission,
	// not gameplay traversal. The production browser cannot upload this to host.
	state, err := engine.NewSession(3, 1, "next-stage")
	if err != nil {
		t.Fatal(err)
	}
	state.World.Active.Combat.World.Level = 1
	if err = state.Validate(); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := CheckpointOf(state, manifest.ID, build)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Call(Request{Op: "restore", Epoch: state.Audio.Epoch, Checkpoint: &checkpoint}); err == nil {
		t.Fatal("restore silently installed missing stage")
	}
	if _, err = b.Call(Request{Op: "stage", Level: 1, StageData: string(second)}); err != nil {
		t.Fatal(err)
	}
	restored, err := b.Call(Request{Op: "restore", Epoch: state.Audio.Epoch, Checkpoint: &checkpoint})
	if err != nil || restored.(Snapshot).Resources != manifest.ID || restored.(Snapshot).Level != 1 || len(b.stages) != 2 {
		t.Fatal("next-stage restore lost shared identity", err)
	}
	if _, err = b.Call(Request{Op: "pads", Session: view.Session, Epoch: view.Epoch, Frame: 1, Pads: []int{0, 0}}); err != nil {
		t.Fatal("second stage replaced first stage resources", err)
	}
}

// A cleared checkpoint uses the last stage's presentation resources; requiring
// a nonexistent ninth stage would make refresh of the final board impossible.
func TestRuntimeBundleClearedCheckpointUsesLastStage(t *testing.T) {
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	manifest := bundle.Manifest()
	code := testRulesBuild(t)
	b := New(code)
	if _, err = b.Call(Request{Op: "bundle", Bundle: &manifest}); err != nil {
		t.Fatal(err)
	}
	raw, _ := bundle.Payload(7)
	if _, err = b.Call(Request{Op: "stage", Level: 7, StageData: string(raw)}); err != nil {
		t.Fatal(err)
	}
	s, err := engine.NewSession(1, 1, "terminal-checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	// Isolated valid terminal-boundary fixture, not a claimed full campaign.
	s.World.Campaign.Outcome = engine.CampaignCleared
	s.World.Active.Combat.World.Level = 8
	s.World.Active.Combat.World.GameRoutine = 6
	if _, err = s.FlushTerminal(); err != nil {
		t.Fatal(err)
	}
	c, err := CheckpointOf(s, manifest.ID, code)
	if err != nil {
		t.Fatal(err)
	}
	value, err := b.Call(Request{Op: "restore", Epoch: s.Audio.Epoch, Checkpoint: &c})
	if err != nil {
		t.Fatal(err)
	}
	view := value.(Snapshot)
	if !view.Finalized || view.Outcome != engine.CampaignCleared || view.Level != 8 || len(b.stages) != 1 {
		t.Fatal("terminal restore changed outcome or eagerly loaded other maps", view)
	}
}
