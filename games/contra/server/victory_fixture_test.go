package contra

// Failure inventory before terminal-fixture support: a finalized/edited state
// is presented as live victory; source or boundary hashes differ; unrecorded life
// grants remain in the suffix; the timing gate advances rules/audio; a client
// action fabricates success; the formal terminal flush/result path is skipped.
// This is a declared assisted-checkpoint/rating-projection fixture, not standard
// reserve-life or fresh-campaign acceptance. It is compiled only in tests.
import (
	"bytes"
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestAssistedVictoryFixtureUsesRealTerminalPath(t *testing.T) {
	directory := os.Getenv("CONTRA_VICTORY_BOUNDARY")
	if directory == "" {
		t.Skip("explicit verified assisted boundary only")
	}
	boundary, err := loadVictoryBoundary(directory)
	if err != nil {
		t.Fatal(err)
	}
	gate := &atomic.Bool{}
	module := withVictoryBoundary(testModule(t), boundary, gate)
	_, context, match := initialWithModule(t, module, boundary.Players)
	initial := saved(t, match)
	before, _ := json.Marshal(initial.Session)
	if !bytes.Equal(before, boundary.Prefix) || initial.Session.Finalized || initial.Session.World.Campaign.Outcome != engine.CampaignOngoing || match.Result != nil {
		t.Fatal("fixture did not retain the actual preterminal state")
	}
	context.Actor = -1
	context.Now = context.Now.Add(5 * time.Second)
	match, err = module.OnTimer(context, match)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(saved(t, match).Session)
	if !bytes.Equal(before, after) {
		t.Fatal("test timing gate changed rule or audio state")
	}
	context.Actor = 0
	action, _ := json.Marshal(map[string]any{"type": "fixture-win", "epoch": initial.Epoch, "sequence": 1})
	if _, err = module.Apply(context, match, action); err == nil {
		t.Fatal("client can fabricate terminal result")
	}
	gate.Store(true)
	context.Actor = -1
	for count := 0; count < 100 && match.Result == nil; count++ {
		context.Now = context.Now.Add(50 * time.Millisecond)
		match, err = module.OnTimer(context, match)
		if err != nil {
			t.Fatal(err)
		}
	}
	final := saved(t, match)
	got, _ := json.Marshal(final.Session)
	if match.Result == nil || !match.Result.Cooperative || len(match.Result.Winners) != boundary.Players || !final.Session.Finalized || final.Session.World.Campaign.Outcome != engine.CampaignCleared || !bytes.Equal(got, boundary.Final) {
		t.Fatal("actual terminal transition differs from independently replayed diagnostic")
	}
	if match.SystemDeadline != 0 || match.Result.Draw {
		t.Fatal("invalid victory settlement metadata")
	}
	t.Logf("assisted source terminal projection: frame=%d revision=%d winners=%v; not standard campaign acceptance", final.Session.Random.Frames, final.Session.Revision, match.Result.Winners)
}
