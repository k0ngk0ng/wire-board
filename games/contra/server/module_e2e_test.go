package contra

// The failure inventory in PLATFORM_CONTRACT.md precedes production code.
// These tests drive the actual module and replay its authoritative records.
import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testModule(t *testing.T) gameapi.Module {
	t.Helper()
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	id, _, e := runtime.ReadRulesBuild(root)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ModuleWithConfig(boundaryVisualConfig(t, id))
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func initial(t *testing.T, n int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m := testModule(t)
	c := gameapi.Context{Actor: 0, Players: n, Now: time.UnixMilli(100000), Automated: make([]bool, n)}
	g, e := m.Start(c, json.RawMessage(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	return m, c, g
}
func saved(t *testing.T, g gameapi.Match) State {
	t.Helper()
	var s State
	if e := json.Unmarshal(g.Data, &s); e != nil {
		t.Fatal(e)
	}
	return s
}
func wire(t *testing.T, m gameapi.Module, g gameapi.Match, a int) View {
	t.Helper()
	v, e := m.View(g, a)
	if e != nil {
		t.Fatal(e)
	}
	return v.(View)
}
func action(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, v any) gameapi.Match {
	t.Helper()
	b, _ := json.Marshal(v)
	out, e := m.Apply(c, g, b)
	if e != nil {
		t.Fatal(string(b), e)
	}
	if e = m.ValidateMatch(out, c.Players); e != nil {
		t.Fatal(e)
	}
	return out
}
func input(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, held engine.Buttons) gameapi.Match {
	t.Helper()
	v := wire(t, m, g, c.Actor)
	return action(t, m, c, g, map[string]any{"type": "input", "epoch": v.Epoch, "sequence": v.Inputs[c.Actor].Sequence + 1, "held": int(held)})
}
func ticks(t *testing.T, m gameapi.Module, c *gameapi.Context, g gameapi.Match, ms int64) gameapi.Match {
	t.Helper()
	c.Now = c.Now.Add(time.Duration(ms) * time.Millisecond)
	cc := *c
	cc.Actor = -1
	var out gameapi.Match
	var e error
	if g.Paused {
		out, e = m.OnPausedTimer(cc, g)
	} else {
		out, e = m.OnTimer(cc, g)
	}
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateMatch(out, c.Players); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestContraModuleStandardAndRejectClientAuthority(t *testing.T) {
	m, c, g := initial(t, 2)
	s := saved(t, g)
	if m.RatingPolicy != gameapi.RatingCoopCampaignV1 || !m.Realtime || g.HasComputerPlayers || s.Session.World.Active.Combat.Players[0].Lives != 2 || s.Session.World.Active.Combat.Players[1].Lives != 2 || s.Session.World.Campaign.Continues != 3 {
		t.Fatal("nonstandard start")
	}
	if e := m.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"", "version-one", strings.Repeat("g", 64)} {
		if _, e := ModuleWithConfig(Config{RulesBuildID: id}); e == nil {
			t.Fatal("invalid rule build")
		}
	}
	for _, raw := range []string{`{"seed":1}`, `{"stage":7}`, `{"state":{}}`, `{"lives":30}`, `null`, `{} {}`} {
		if _, e := m.Configure(2, []byte(raw)); e == nil {
			t.Fatal("accepted config", raw)
		}
	}
	for _, n := range []int{0, 3} {
		if _, e := m.Configure(n, []byte(`{}`)); e == nil {
			t.Fatal("invalid capacity")
		}
	}
	v := wire(t, m, g, -1)
	if v.Actor != -1 || !reflect.DeepEqual(v.SeatActors, []int{0, 1}) || v.Stage.Level != 0 || v.Bundle.ID == "" || v.RulesBuildID == "" {
		t.Fatal("identity projection")
	}
	for _, a := range []map[string]any{
		{"type": "input", "epoch": s.Epoch, "sequence": 1, "held": 256},
		{"type": "input", "epoch": "old", "sequence": 1, "held": 1},
		{"type": "input", "epoch": s.Epoch, "sequence": 0, "held": 1},
		{"type": "input", "epoch": s.Epoch, "sequence": 9007199254740992, "held": 1},
		{"type": "input", "epoch": s.Epoch, "sequence": 1, "held": 1, "seat": 1},
		{"type": "input", "epoch": s.Epoch, "sequence": 1, "held": 1, "state": map[string]any{}},
	} {
		raw, _ := json.Marshal(a)
		before := string(g.Data)
		if _, e := m.Apply(c, g, raw); e == nil {
			t.Fatal("accepted forbidden action", a)
		}
		if string(g.Data) != before {
			t.Fatal("rejected action mutated input")
		}
	}
	c.Actor = -1
	if _, e := m.Apply(c, g, []byte(`{"type":"input","epoch":"`+s.Epoch+`","sequence":1,"held":1}`)); e == nil {
		t.Fatal("spectator input")
	}
}
func TestContraModuleInputClockReplayAndRestore(t *testing.T) {
	m, c, g := initial(t, 2)
	start := saved(t, g)
	g = input(t, m, c, g, engine.ButtonA)
	g = input(t, m, c, g, 0)
	if saved(t, g).Session.Random.Frames != 0 {
		t.Fatal("Apply advanced rules")
	}
	first := saved(t, g).Inputs[0]
	a := map[string]any{"type": "input", "epoch": start.Epoch, "sequence": first.Sequence, "held": 0}
	old := g
	c.Now = c.Now.Add(time.Millisecond)
	g = action(t, m, c, g, a)
	if string(g.Data) != string(old.Data) {
		t.Fatal("retry refreshed lease")
	}
	raw, _ := json.Marshal(a)
	if !m.AcceptStale(c, g, raw) {
		t.Fatal("current epoch input cannot survive room version advance")
	}
	a["held"] = 1
	raw, _ = json.Marshal(a)
	if _, e := m.Apply(c, g, raw); e == nil {
		t.Fatal("conflicting duplicate")
	}
	g = ticks(t, m, &c, g, 49)
	v := wire(t, m, g, 0)
	if len(v.Frames) != 3 || v.Frames[0].Pads[0] != engine.ButtonA || v.Frames[1].Pads[0] != 0 {
		t.Fatal("short tap lost", v.Frames)
	}
	bundle, e := engine.BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	r, _ := bundle.Stage(0)
	for _, f := range v.Frames {
		if _, e = start.Session.StepPads(f.Frame, f.Pads, r); e != nil {
			t.Fatal(e)
		}
		digest, e := runtime.StateDigest(start.Session, v.Bundle.ID, v.RulesBuildID)
		if e != nil || digest != f.Digest {
			t.Fatal("native replay mismatch", e)
		}
	}
	sync, e := m.SyncSnapshot(g, 0)
	if e != nil {
		t.Fatal(e)
	}
	cp := sync.(SyncView).Checkpoint
	if cp.Digest != v.Snapshot.Digest || cp.State.Random.Frames != 3 {
		t.Fatal("checkpoint differs")
	}
	g = input(t, m, c, g, engine.ButtonRight)
	before := saved(t, g)
	g = ticks(t, m, &c, g, 5000)
	after := saved(t, g)
	if after.Session.Random.Frames-before.Session.Random.Frames > 15 || after.Inputs[0].Automated || after.Inputs[0].Held != 0 || after.Bots[0].LastTick != 0 {
		t.Fatal("catch-up or idle takeover")
	}
	g = input(t, m, c, g, engine.ButtonB)
	before = saved(t, g)
	c.Now = c.Now.Add(time.Second)
	restored, e := m.Restore(c, g)
	if e != nil {
		t.Fatal(e)
	}
	after = saved(t, restored)
	if after.Epoch != before.Epoch || after.Inputs[0].Sequence != before.Inputs[0].Sequence || after.Inputs[0].Held != 0 || !reflect.DeepEqual(after.Session, before.Session) {
		t.Fatal("restore reset rules or kept controls")
	}
}
func TestContraModulePauseResumeAndAutomation(t *testing.T) {
	m, c, g := initial(t, 2)
	// Each tick is 50 ms = three rules frames; original Start skips the intro.
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, 0)
	for i := 0; i < 20 && saved(t, g).Session.World.Active.Combat.World.LevelRoutine != 4; i++ {
		g = ticks(t, m, &c, g, 50)
	}
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	if !g.Paused {
		t.Fatal("Start did not pause")
	}
	before := saved(t, g)
	g = ticks(t, m, &c, g, 250)
	after := saved(t, g)
	if !g.Paused || after.Session.Random != before.Session.Random || after.Session.Controls != before.Session.Controls || after.Session.World.Campaign != before.Session.World.Campaign || after.Session.World.Active.Scores != before.Session.World.Active.Scores || after.Bots != before.Bots || after.Session.PresentationTicks <= before.Session.PresentationTicks {
		t.Fatal("pause changed gameplay or stopped presentation")
	}
	g = input(t, m, c, g, engine.ButtonStart)
	if !g.Paused {
		t.Fatal("held heartbeat resumed")
	}
	c.Actor = 1
	v := wire(t, m, g, 1)
	raw, _ := json.Marshal(map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": 1})
	if _, e := m.Apply(c, g, raw); e == nil {
		t.Fatal("P2 resumed")
	}
	c.Actor = 0
	g = input(t, m, c, g, 0)
	v = wire(t, m, g, 0)
	g = action(t, m, c, g, map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1})
	if g.Paused || !saved(t, g).Session.ResumeRequested {
		t.Fatal("Apply did not authorize resume")
	}
	g = ticks(t, m, &c, g, 50)
	if saved(t, g).Session.World.Campaign.Paused {
		t.Fatal("resume not consumed")
	}
	c.Automated[1] = true
	g = ticks(t, m, &c, g, 50)
	before = saved(t, g)
	if before.Bots[1].LastTick == 0 || before.Bots[0].LastTick != 0 || !before.Inputs[1].Automated {
		t.Fatal("host automation not honored")
	}
	c.Actor = 1
	g, e := m.Reclaim(c, g)
	if e != nil {
		t.Fatal(e)
	}
	after = saved(t, g)
	if !reflect.DeepEqual(after.Session, before.Session) || !reflect.DeepEqual(after.Inputs[0], before.Inputs[0]) || after.Inputs[1].Held != 0 || after.Inputs[1].Automated {
		t.Fatal("reclaim changed game or other seat")
	}
	c.Automated[1] = false
	g = ticks(t, m, &c, g, 50)
	if saved(t, g).Bots[1].LastTick != before.Bots[1].LastTick {
		t.Fatal("bot ran after reclaim")
	}
}
func boundary(t *testing.T, g gameapi.Match, level uint8) gameapi.Match {
	t.Helper()
	s := saved(t, g)
	s.Session.World.Active.Combat.World.Level = level
	s.Session.World.Active.Combat.World.LevelRoutine = 5
	s.Session.World.Active.Combat.World.BossDefeated = 1
	g.Data, _ = json.Marshal(s)
	return g
}
func TestContraModuleStageGateAndTerminalPublication(t *testing.T) {
	m, c, g := initial(t, 2)
	g = boundary(t, g, 0)
	g = ticks(t, m, &c, g, 50)
	v := wire(t, m, g, 0)
	if v.StageLoading == nil || v.Stage.Level != 1 || v.Snapshot.Frame != 1 {
		t.Fatal("advance did not stop at new stage")
	}
	f := v.Snapshot.Frame
	g = ticks(t, m, &c, g, 5000)
	if wire(t, m, g, 0).Snapshot.Frame != f+15 {
		t.Fatal("loading client stopped the running rule clock or caused unbounded catch-up")
	}
	f += 15
	ready := map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": 1, "level": 1, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "stageSHA256": v.Stage.SHA256}
	g = action(t, m, c, g, ready)
	c.Actor = 1
	ready["stageSHA256"] = "wrong"
	raw, _ := json.Marshal(ready)
	if _, e := m.Apply(c, g, raw); e == nil {
		t.Fatal("wrong stage ready")
	}
	ready["stageSHA256"] = v.Stage.SHA256
	g = action(t, m, c, g, ready)
	if wire(t, m, g, 0).StageLoading != nil {
		t.Fatal("ready gate remained")
	}
	g = ticks(t, m, &c, g, 50)
	if wire(t, m, g, 0).Snapshot.Frame != f+3 {
		t.Fatal("loading time simulated")
	}
	c.Actor = 0
	g = action(t, m, c, g, map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": 2, "level": 1, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "stageSHA256": v.Stage.SHA256})
	if wire(t, m, g, 0).StageLoading != nil {
		t.Fatal("reconnect re-gated")
	}
	for _, win := range []bool{true, false} {
		t.Run(map[bool]string{true: "shared-win", false: "shared-loss"}[win], func(t *testing.T) {
			m, c, g := initial(t, 2)
			g = boundary(t, g, 7)
			if !win {
				s := saved(t, g)
				s.Session.World.Active.Combat.World.BossDefeated = 0
				s.Session.World.Campaign.Continues = 0
				g.Data, _ = json.Marshal(s)
			}
			g = ticks(t, m, &c, g, 50)
			s := saved(t, g)
			if g.Result == nil || !g.Result.Cooperative || g.Result.Team || g.Result.Draw || !s.Session.Finalized || g.HasComputerPlayers || g.SystemDeadline != 0 {
				t.Fatal("invalid cooperative terminal")
			}
			if win && len(g.Result.Winners) != 2 || !win && len(g.Result.Winners) != 0 {
				t.Fatal("not all-or-none")
			}
			v := wire(t, m, g, 0)
			if len(v.Frames) != 2 || v.Frames[0].Kind != "pads" || v.Frames[1].Kind != "terminal" || v.Frames[1].Revision != v.Frames[0].Revision+1 {
				t.Fatal("terminal not separately recorded")
			}
		})
	}
}

// Extended E2E coverage from the original failure inventory: bounded replay
// recovery, queue overload/release, and changing explicit automation at a gate.
func TestContraModuleReplayWindowAndGateAutomation(t *testing.T) {
	m, c, g := initial(t, 2)
	checkpoint, e := m.SyncSnapshot(g, 0)
	if e != nil {
		t.Fatal(e)
	}
	replica := checkpoint.(SyncView).Checkpoint.State
	bundle, e := engine.BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	last := uint64(0)
	for n := 0; n < 60; n++ {
		if n == 1 || n == 30 {
			g = input(t, m, c, g, engine.ButtonStart)
		}
		if n == 2 || n == 31 {
			g = input(t, m, c, g, 0)
		}
		if n == 40 && g.Paused {
			v := wire(t, m, g, 0)
			g = action(t, m, c, g, map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1})
		}
		g = ticks(t, m, &c, g, 50)
		v := wire(t, m, g, 0)
		for _, f := range v.Frames {
			if f.Revision <= last {
				continue
			}
			if f.Revision != last+1 {
				t.Fatal("gap in frequent replay")
			}
			r, _ := bundle.Stage(replica.World.Active.Combat.World.Level)
			switch f.Kind {
			case "pads":
				_, e = replica.StepPads(f.Frame, f.Pads, r)
			case "resume":
				e = replica.RequestResume()
			case "present-paused":
				_, e = replica.PresentPaused(r)
			case "terminal":
				_, e = replica.FlushTerminal()
			default:
				t.Fatal("bad replay kind")
			}
			if e != nil {
				t.Fatal(e)
			}
			digest, e := runtime.StateDigest(replica, v.Bundle.ID, v.RulesBuildID)
			if e != nil || digest != f.Digest {
				t.Fatal("replay mismatch", f.Kind, f.Revision, e)
			}
			last = f.Revision
		}
	}
	v := wire(t, m, g, 0)
	if len(v.Frames) != 120 || v.Frames[0].Revision <= 1 {
		t.Fatal("ring not bounded")
	}
	recovered, e := m.SyncSnapshot(g, -1)
	if e != nil {
		t.Fatal(e)
	}
	sync := recovered.(SyncView)
	if sync.View.Actor != -1 || sync.Checkpoint.Digest != v.Snapshot.Digest || sync.Checkpoint.State.Revision != last {
		t.Fatal("explicit gap recovery differs")
	}
	m, c, g = initial(t, 2)
	// A lost client can release its queue even after the bounded edge queue fills.
	for n := 0; n < 64; n++ {
		g = input(t, m, c, g, engine.Buttons(1+n%2))
	}
	v = wire(t, m, g, 0)
	raw, _ := json.Marshal(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 65, "held": 3})
	if _, e = m.Apply(c, g, raw); e == nil {
		t.Fatal("unbounded input queue")
	}
	g = action(t, m, c, g, map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 65})
	g = ticks(t, m, &c, g, 50)
	for _, f := range wire(t, m, g, 0).Frames {
		if f.Pads[0] != 0 {
			t.Fatal("release replayed queued controls")
		}
	}
	m, c, g = initial(t, 2)
	g = boundary(t, g, 0)
	g = ticks(t, m, &c, g, 50)
	before := saved(t, g)
	c.Automated[1] = true
	g = ticks(t, m, &c, g, 50)
	s := saved(t, g)
	if s.StageLoading == nil || !s.StageLoading.Ready[1] || s.Session.Random.Frames != before.Session.Random.Frames+3 || s.Bots[1].LastTick == 0 {
		t.Fatal("unready human stopped the running clock or the explicit computer")
	}
	manualBotTick := s.Bots[1].LastTick
	c.Actor = 1
	g, e = m.Reclaim(c, g)
	if e != nil {
		t.Fatal(e)
	}
	c.Automated[1] = false
	if saved(t, g).StageLoading.Ready[1] {
		t.Fatal("returning manual seat bypassed stage readiness")
	}
	v = wire(t, m, g, 1)
	ready := map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": 1, "level": v.Stage.Level, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "stageSHA256": v.Stage.SHA256}
	g = action(t, m, c, g, ready)
	c.Automated[0] = true
	g = ticks(t, m, &c, g, 50)
	if saved(t, g).StageLoading != nil || saved(t, g).Session.Random.Frames != before.Session.Random.Frames+6 {
		t.Fatal("readiness acknowledgement reset the running frame schedule")
	}
	g = ticks(t, m, &c, g, 50)
	s = saved(t, g)
	if s.Bots[0].LastTick == 0 || s.Bots[1].LastTick != manualBotTick || s.Inputs[1].Automated {
		t.Fatal("wrong seat ran strategy")
	}
}

func TestContraModuleRejectsCorruptReplaySavepoint(t *testing.T) {
	m, c, g := initial(t, 2)
	g = ticks(t, m, &c, g, 50)
	for _, scenario := range []struct {
		name   string
		change func(*State)
	}{
		{"frame-gap", func(s *State) { s.Frames[1].Frame = 0 }},
		{"head-frame", func(s *State) { s.Frames[len(s.Frames)-1].Frame-- }},
		{"revision-gap", func(s *State) { s.Frames[1].Revision = 1 }},
		{"head-digest", func(s *State) { s.Frames[len(s.Frames)-1].Digest = strings.Repeat("0", 64) }},
		{"resource-identity", func(s *State) { s.BundleID = strings.Repeat("0", 64) }},
		{"rule-identity", func(s *State) { s.RulesBuildID = strings.Repeat("0", 64) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := saved(t, g)
			scenario.change(&s)
			bad := g
			bad.Data, _ = json.Marshal(s)
			if _, e := m.Restore(c, bad); e == nil {
				t.Fatal("corrupt savepoint accepted")
			}
			if _, e := m.SyncSnapshot(bad, -1); e == nil {
				t.Fatal("corrupt checkpoint published")
			}
		})
	}
}

// Gate failure inventory precedes this fix: old in-flight input can arrive
// before or after its own ready acknowledgement, and reclaim can race that
// transport. Gate acknowledgements must consume sequences without retaining
// controls. Releases remain accepted even while the host still marks automation.
func TestContraModuleStageLoadingDiscardsLateControls(t *testing.T) {
	for _, cancelBot := range []bool{false, true} {
		t.Run(map[bool]string{false: "both-manual", true: "reclaim-during-loading"}[cancelBot], func(t *testing.T) {
			m, c, g := initial(t, 2)
			g = input(t, m, c, g, engine.ButtonB|engine.ButtonRight)
			c.Automated[1] = cancelBot
			g = boundary(t, g, 0)
			g = ticks(t, m, &c, g, 50)
			entered := saved(t, g)
			if entered.StageLoading == nil {
				t.Fatal("missing real transition gate")
			}
			frame := entered.Session.Random.Frames
			release := func() {
				v := wire(t, m, g, c.Actor)
				g = action(t, m, c, g, map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[c.Actor].Sequence + 1})
			}
			if cancelBot {
				c.Actor = 1
				release()                                 // host metadata is still true, as during cancellation.
				g = input(t, m, c, g, engine.ButtonRight) // late before reclaim commits.
				afterRelease := saved(t, g)
				if !afterRelease.Inputs[1].Automated || afterRelease.Inputs[1].Held != 0 || len(afterRelease.Inputs[1].Queue) != 0 || !reflect.DeepEqual(afterRelease.Session, entered.Session) {
					t.Fatal("loading release/input changed automation or rule state")
				}
				var e error
				g, e = m.Reclaim(c, g)
				if e != nil {
					t.Fatal(e)
				}
				c.Automated[1] = false
				if saved(t, g).StageLoading.Ready[1] {
					t.Fatal("reclaimed human skipped readiness")
				}
				g = input(t, m, c, g, engine.ButtonA|engine.ButtonLeft)
			}
			c.Actor = 0
			g = input(t, m, c, g, engine.ButtonA|engine.ButtonLeft) // old stage packet arrives.
			if s := saved(t, g); s.Inputs[0].Held != 0 || s.Inputs[0].Applied != 0 || len(s.Inputs[0].Queue) != 0 || s.Inputs[0].ReceivedAt != 0 {
				t.Error("loading retained old control")
			}
			ready := func() {
				v := wire(t, m, g, c.Actor)
				g = action(t, m, c, g, map[string]any{"type": "stage-ready", "epoch": v.Epoch, "sequence": v.Inputs[c.Actor].Sequence + 1, "level": v.Stage.Level, "bundleID": v.Bundle.ID, "rulesBuildID": v.RulesBuildID, "stageSHA256": v.Stage.SHA256})
			}
			ready()
			v := wire(t, m, g, 0)
			late := map[string]any{"type": "input", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1, "held": int(engine.ButtonRight | engine.ButtonB)}
			g = action(t, m, c, g, late) // own ready allows play while the other human still loads.
			beforeReady := saved(t, g)
			c.Actor = 1
			ready()
			afterReady := saved(t, g)
			if afterReady.StageLoading != nil {
				t.Fatal("all ready did not release")
			}
			if !reflect.DeepEqual(beforeReady.Session, afterReady.Session) {
				t.Fatal("gate release reset session or changed time")
			}
			if afterReady.Inputs[0].Sequence != beforeReady.Inputs[0].Sequence || afterReady.Inputs[1].Sequence != beforeReady.Inputs[1].Sequence+1 {
				t.Fatal("gate rolled sequences backwards")
			}
			c.Actor = 0
			beforeRetry := string(g.Data)
			g = action(t, m, c, g, late)
			if string(g.Data) != beforeRetry {
				t.Fatal("discarded input retry revived old pad")
			}
			g = ticks(t, m, &c, g, 50)
			s := saved(t, g)
			if s.Session.Random.Frames != frame+3 {
				t.Fatal("gate changed 60 Hz frame schedule")
			}
			for _, f := range s.Frames {
				if f.Frame > frame && f.Kind == "pads" && f.Pads != [2]engine.Buttons{engine.ButtonRight | engine.ButtonB, 0} {
					t.Error("other seat readiness cleared the active human input", f.Frame, f.Pads)
				}
			}
			// Fresh post-load input remains usable and can create a new button edge.
			g = input(t, m, c, g, engine.ButtonStart|engine.ButtonB)
			g = ticks(t, m, &c, g, 50)
			s = saved(t, g)
			if s.Frames[len(s.Frames)-1].Pads[0] != (engine.ButtonStart | engine.ButtonB) {
				t.Fatal("fresh post-gate input suppressed")
			}
		})
	}
}
