package rootcontrol_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func orchestrationNoop() rootcontrol.RuleHooks {
	return rootcontrol.RuleHooks{Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Operation: func(score.Operation) error { return nil }, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }}, CaptureLevels: func() (rootcontrol.PauseLevels, error) { return rootcontrol.PauseLevels{}, nil }}
}
func orchestrationOwners(t *testing.T) (*rootcontrol.State, rootcontrol.RuleOwners) {
	t.Helper()
	d, err := score.New(orchestrationCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	root, queue, pause := rootcontrol.InitialState(), [4]uint8{}, uint8(0)
	return &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}
}
func TestOrchestratorNormalStageOrderAndLiveQueue(t *testing.T) {
	root, owners := orchestrationOwners(t)
	var phases []rootcontrol.RulePhase
	var writes []rootcontrol.FadeWrite
	hooks := orchestrationNoop()
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if op.Kind == rootcontrol.RuleEnterPhase {
			phases = append(phases, op.Phase)
			if op.Phase == rootcontrol.RuleQueue {
				owners.Queue[0] = 1
			}
		}
		if op.Kind == rootcontrol.RuleFadeWrite {
			writes = append(writes, op.FadeWrite)
		}
		return nil
	}
	priorities := engine.SoundPriorityTable{}
	priorities[0] = 255
	if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
		t.Fatal(err)
	}
	want := []rootcontrol.RulePhase{rootcontrol.RuleGate, rootcontrol.RuleControlIdle, rootcontrol.RuleTempo, rootcontrol.RuleFade, rootcontrol.RuleQueue, rootcontrol.RuleDispatch, rootcontrol.RuleVoices, rootcontrol.RuleReturn}
	if !reflect.DeepEqual(phases, want) || !reflect.DeepEqual(writes, []rootcontrol.FadeWrite{{Counter: rootcontrol.FadeRemainingCounter, Value: 3}, {Counter: rootcontrol.FadeStepsCounter, Value: 40}}) || root.Selected != 255 || *owners.Queue != ([4]uint8{}) {
		t.Fatal("stage order, live queue or 01 write order changed", phases, writes, root, *owners.Queue)
	}
	if owners.Score.Snapshot().Ticks != 1 {
		t.Fatal("control tick duplicated time")
	}
}
func TestOrchestratorPauseReadsUseLiveFieldAndAllPauseRoutesSuspend(t *testing.T) {
	root, owners := orchestrationOwners(t)
	root.PauseRestore = &rootcontrol.PauseLevels{}
	*owners.AudioPause = 1
	hooks := orchestrationNoop()
	reads := 0
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if op.Kind == rootcontrol.RulePauseRead {
			reads++
			if reads == 2 {
				*owners.AudioPause = 255
			}
		}
		if op.Kind == rootcontrol.RuleChipWrite {
			t.Fatal("hold wrote a chip")
		}
		return nil
	}
	before := owners.Score.Snapshot()
	out, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks)
	if err != nil || out.Route != rootcontrol.RulePauseHold || reads != 2 {
		t.Fatal("gate cached entry pause", out, reads, err)
	}
	before.Ticks++
	if owners.Score.Snapshot() != before {
		t.Fatal("hold advanced score")
	}
	for _, request := range []uint8{128, 1, 255, 128} {
		*owners.AudioPause = request
		if request == 1 {
			root.PauseRestore = nil
		}
		prior := owners.Score.Snapshot()
		phases := []rootcontrol.RulePhase{}
		chipWrites := []score.Intent{}
		hooks = orchestrationNoop()
		hooks.Operation = func(op rootcontrol.RuleOperation) error {
			if op.Kind == rootcontrol.RuleEnterPhase {
				phases = append(phases, op.Phase)
			}
			if op.Kind == rootcontrol.RuleChipWrite {
				chipWrites = append(chipWrites, op.Write)
			}
			return nil
		}
		_, err = rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks)
		if err != nil {
			t.Fatal(err)
		}
		prior.Ticks++
		if owners.Score.Snapshot() != prior || !reflect.DeepEqual(phases, []rootcontrol.RulePhase{rootcontrol.RuleGate, rootcontrol.RuleReturn}) {
			t.Fatal("pause route advanced a normal phase", request, phases)
		}
		wantCount := 24
		if request == 1 {
			wantCount = 28
		}
		if request == 255 {
			wantCount = 0
		}
		if len(chipWrites) != wantCount {
			t.Fatal("wrong pause request count", request, len(chipWrites))
		}
		if request == 128 {
			for _, w := range chipWrites {
				if w.Kind != "ym" {
					t.Fatal("resume invented PSG restore")
				}
			}
			if root.PauseRestore != nil || *owners.AudioPause != 0 {
				t.Fatal("successful resume retained pending payload")
			}
		}
	}
}
func TestOrchestrator04IsOneExplicitRulesBulkAndKeepsExternalOwners(t *testing.T) {
	root, owners := orchestrationOwners(t)
	root.Priority = 85
	root.ChoiceBits = 1
	root.Fade = rootcontrol.FadeState{Steps: 7, Remaining: 2}
	owners.Queue[0] = 4
	priorities := engine.SoundPriorityTable{}
	priorities[3] = 255
	hooks := orchestrationNoop()
	events := []string{}
	writes := []score.Intent{}
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if op.Kind == rootcontrol.RuleBulkClear {
			if op.HeaderLongs != 16 || op.VoiceLongs != 216 || op.MusicOnly {
				t.Fatal("fake clear granularity", op)
			}
			events = append(events, "bulk")
		}
		if op.Kind == rootcontrol.RuleChipWrite {
			writes = append(writes, op.Write)
			events = append(events, "write")
		}
		return nil
	}
	if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
		t.Fatal(err)
	}
	if len(events) != 36 || events[0] != "write" || events[1] != "bulk" || len(writes) != 35 {
		t.Fatal("04 effect order/count", events, len(writes))
	}
	if writes[0] != (score.Intent{Kind: "ym", Register: 0x27}) {
		t.Fatal("04 did not begin YM27", writes[0])
	}
	for i, value := range []uint8{2, 6, 1, 5, 0, 4} {
		if writes[i+1] != (score.Intent{Kind: "ym", Register: 0x28, Value: value}) {
			t.Fatal("keyoff order", writes)
		}
	}
	for i, w := range rootcontrol.PauseMuteWrites() {
		if writes[7+i] != (score.Intent{Kind: "ym", Bank: w.Bank, Register: w.Register, Value: w.Value}) {
			t.Fatal("TL order")
		}
	}
	for i, value := range []uint8{0x9f, 0xbf, 0xdf, 0xff} {
		if writes[31+i] != (score.Intent{Kind: "psg", Value: value}) {
			t.Fatal("PSG mute order")
		}
	}
	if *root != rootcontrol.InitialState() || *owners.Queue != ([4]uint8{}) || *owners.AudioPause != 0 || owners.Score.Snapshot() != (score.State{Ticks: 1}) {
		t.Fatal("04 unique owners not cleared")
	}
}
func TestOrchestratorMusicResetPriorityLiveNodesAndQueueReplace(t *testing.T) {
	root, owners := orchestrationOwners(t)
	root.Priority = 85
	root.ChoiceBits = 1
	root.Fade = rootcontrol.FadeState{Steps: 7, Remaining: 2}
	owners.Queue[0] = 0x82
	hooks := orchestrationNoop()
	nodes := []score.OperationKind{}
	hooks.Score.Operation = func(op score.Operation) error {
		if op.Kind == score.OpPrioritySave || op.Kind == score.OpHeaderClear || op.Kind == score.OpPriorityRestore || op.Kind == score.OpSelectedReset {
			nodes = append(nodes, op.Kind)
			// StopPrimary has already cleared entry priority. A live external
			// owner update at the actual save point must be preserved instead.
			if op.Kind == score.OpPrioritySave {
				root.Priority = 85
			}
			if op.Kind == score.OpPriorityRestore && root.Priority != 0 {
				t.Fatal("bulk failed to clear root before restore")
			}
			if op.Kind == score.OpSelectedReset && root.Priority != 85 {
				t.Fatal("priority not restored at actual node")
			}
		}
		return nil
	}
	priorities := engine.SoundPriorityTable{}
	priorities[0x81] = 255
	if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nodes, []score.OperationKind{score.OpPrioritySave, score.OpHeaderClear, score.OpPriorityRestore, score.OpSelectedReset}) || root.Priority != 85 || root.Selected != 255 || root.ChoiceBits != 0 || root.Fade != (rootcontrol.FadeState{}) {
		t.Fatal("music-reset live effect order", nodes, root)
	}
}
func TestOrchestratorUnsupportedAndCostFailBeforeEffects(t *testing.T) {
	for _, input := range []rootcontrol.RuleInput{{RequireCost: true}, {Idle: rootcontrol.RuleIdleProjection{ControlMode: 1}}, {Idle: rootcontrol.RuleIdleProjection{DuckPhase: 1}}, {Idle: rootcontrol.RuleIdleProjection{FMDelta: 1}}, {Idle: rootcontrol.RuleIdleProjection{PSGDelta: 1}}, {Idle: rootcontrol.RuleIdleProjection{Signal: 1}}, {Idle: rootcontrol.RuleIdleProjection{FM3Mode: 1}}, {Idle: rootcontrol.RuleIdleProjection{FM3Offsets: [8]uint16{1}}}} {
		root, owners := orchestrationOwners(t)
		before := *root
		count := 0
		hooks := orchestrationNoop()
		hooks.Operation = func(rootcontrol.RuleOperation) error { count++; return nil }
		_, err := rootcontrol.AdvanceRules(root, owners, input, hooks)
		if err == nil || count != 0 || *root != before || owners.Score.Snapshot().Ticks != 0 {
			t.Fatal("unsupported profile reached effects", input, err, count)
		}
		if input.RequireCost && !errors.Is(err, rootcontrol.ErrRuleCostUnavailable) {
			t.Fatal("cost silently treated as zero", err)
		}
	}
	for _, id := range []uint8{2, 3, 0x73, 0x7f, 0x80, 0xa7, 0xac, 0xfe} {
		root, owners := orchestrationOwners(t)
		owners.Queue[0] = id
		priorities := engine.SoundPriorityTable{}
		priorities[id-1] = 255
		if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, orchestrationNoop()); !errors.Is(err, rootcontrol.ErrRuleUnsupported) {
			t.Fatal("unsupported dispatch silently accepted", id, err)
		}
	}
}
func TestOrchestratorFailedHooksDoNotCommitScoreOrContinue(t *testing.T) {
	root, owners := orchestrationOwners(t)
	before := owners.Score.Snapshot()
	sentinel := errors.New("owner failure")
	failed := false
	hooks := orchestrationNoop()
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if failed {
			t.Fatal("effect continued after failure")
		}
		if op.Kind == rootcontrol.RuleEnterPhase && op.Phase == rootcontrol.RuleReturn {
			failed = true
			return sentinel
		}
		return nil
	}
	if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks); !errors.Is(err, sentinel) || owners.Score.Snapshot() != before {
		t.Fatal("failed detached transaction committed score", err)
	}
	if _, err := owners.Score.Advance(score.Input{}); err != nil {
		t.Fatal("failed Root retained score lease", err)
	}
}

func TestOrchestratorScoreQueueWriteAndDACFeedbackStayAtLiveNodes(t *testing.T) {
	root, owners := orchestrationOwners(t)
	owners.Queue[0] = 0xab
	priorities := engine.SoundPriorityTable{}
	priorities[0xaa] = 255
	hooks := orchestrationNoop()
	pendingQueue := false
	nodes := 0
	hooks.AcceptDispatch = func(id uint8) error {
		if id != 0xab {
			return errors.New("unexpected stress")
		}
		return nil
	}
	hooks.Score.Operation = func(op score.Operation) error {
		if op.Kind == score.OpQueueReplace {
			pendingQueue = true
			nodes++
			if op.Writes[0].SoundID != 0x7d {
				t.Fatal("catalog edge changed")
			}
		}
		return nil
	}
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if pendingQueue && op.Kind == rootcontrol.RuleEnterPhase && op.Phase == rootcontrol.RuleReturn && owners.Queue[0] != 0x7d {
			t.Fatal("queue replaced at tick end instead of actual score effect")
		}
		return nil
	}
	for i := 0; i < 12 && nodes == 0; i++ {
		if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
			t.Fatal(err)
		}
	}
	if nodes != 1 || owners.Queue[0] != 0x7d {
		t.Fatal("score queue edge did not reach unique World owner", nodes, *owners.Queue)
	}

	root, owners = orchestrationOwners(t)
	owners.Queue[0] = 0x82
	priorities[0x81] = 255
	hooks = orchestrationNoop()
	ready := false
	reads := 0
	hooks.Score.Operation = func(op score.Operation) error {
		if op.Kind == score.OpFeedbackRead {
			ready = true
		}
		return nil
	}
	hooks.Score.ReadDAC = func(score.FeedbackRead) (score.DACFeedback, error) {
		if !ready {
			t.Fatal("feedback reused entrance value")
		}
		ready = false
		reads++
		return score.DACFeedback{}, nil
	}
	for i := 0; i < 3; i++ {
		if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
			t.Fatal(err)
		}
	}
	if reads == 0 {
		t.Fatal("DAC read path not exercised")
	}
}
func TestOrchestratorTerminalFadeClearsBeforeQueueAndDoesNotStep(t *testing.T) {
	root, owners := orchestrationOwners(t)
	root.Fade = rootcontrol.FadeState{Steps: 1}
	owners.Queue[0] = 1
	priorities := engine.SoundPriorityTable{}
	priorities[0] = 255
	controls := 0
	bulks := 0
	hooks := orchestrationNoop()
	hooks.Operation = func(op rootcontrol.RuleOperation) error {
		if op.Kind == rootcontrol.RuleBulkClear {
			bulks++
		}
		return nil
	}
	hooks.Score.Operation = func(op score.Operation) error {
		if op.Action == score.ActionFadeMusicStep || op.Action == score.ActionStopMusicDACFlags {
			controls++
		}
		return nil
	}
	if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{Priorities: priorities}, hooks); err != nil {
		t.Fatal(err)
	}
	if bulks != 1 || controls != 0 || *root != rootcontrol.InitialState() || *owners.Queue != ([4]uint8{}) {
		t.Fatal("terminal fade consumed old queue or repeated step", bulks, controls, root, *owners.Queue)
	}
}
func TestOrchestratorPausePayloadOnlyCommitsAfterPhysicalEffects(t *testing.T) {
	for _, resume := range []bool{false, true} {
		root, owners := orchestrationOwners(t)
		*owners.AudioPause = 1
		if resume {
			*owners.AudioPause = 128
			root.PauseRestore = &rootcontrol.PauseLevels{}
		}
		original := root.PauseRestore
		writes := 0
		sentinel := errors.New("physical chip failure")
		hooks := orchestrationNoop()
		hooks.Operation = func(op rootcontrol.RuleOperation) error {
			if op.Kind == rootcontrol.RuleChipWrite {
				writes++
				if writes == 24 {
					return sentinel
				}
			}
			return nil
		}
		if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks); !errors.Is(err, sentinel) {
			t.Fatal("physical failure lost", err)
		}
		if root.PauseRestore != original || owners.Score.Snapshot().Ticks != 0 {
			t.Fatal("failed mute/restore changed pending lifecycle or committed score")
		}
	}
	for _, request := range []uint8{128, 255} {
		root, owners := orchestrationOwners(t)
		*owners.AudioPause = request
		if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, orchestrationNoop()); !errors.Is(err, rootcontrol.ErrRuleUnsupported) {
			t.Fatal("pause without pending information accepted", request, err)
		}
	}
}

func TestOrchestratorLeasePrecedesEveryRootAndScoreCallback(t *testing.T) {
	for _, source := range []string{"root-gate", "score-enter"} {
		t.Run(source, func(t *testing.T) {
			root, owners := orchestrationOwners(t)
			hooks := orchestrationNoop()
			called := false
			nestedOperations := 0
			var nestedError error
			reenter := func() {
				if called {
					return
				}
				called = true
				nested := orchestrationNoop()
				nested.Operation = func(rootcontrol.RuleOperation) error { nestedOperations++; return nil }
				_, nestedError = rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, nested)
			}
			if source == "root-gate" {
				hooks.Operation = func(op rootcontrol.RuleOperation) error {
					if op.Kind == rootcontrol.RuleEnterPhase && op.Phase == rootcontrol.RuleGate {
						reenter()
					}
					return nil
				}
			} else {
				hooks.Score.Operation = func(op score.Operation) error {
					if op.Kind == score.OpEnter && op.Routine == score.RoutineTick {
						reenter()
					}
					return nil
				}
			}
			if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks); err != nil {
				t.Fatal(err)
			}
			if !called || nestedError == nil || nestedOperations != 0 || owners.Score.Snapshot().Ticks != 1 {
				t.Fatal("reentrant Root ran before acquiring unique score lease", called, nestedError, nestedOperations, owners.Score.Snapshot().Ticks)
			}
		})
	}
}

func TestOrchestratorRequiresBothOperationConsumersBeforeCallbacks(t *testing.T) {
	for _, missing := range []string{"root", "score"} {
		t.Run(missing, func(t *testing.T) {
			root, owners := orchestrationOwners(t)
			hooks := orchestrationNoop()
			calls := 0
			hooks.Operation = func(rootcontrol.RuleOperation) error { calls++; return nil }
			hooks.Score.Operation = func(score.Operation) error { calls++; return nil }
			if missing == "root" {
				hooks.Operation = nil
			} else {
				hooks.Score.Operation = nil
			}
			if _, err := rootcontrol.AdvanceRules(root, owners, rootcontrol.RuleInput{}, hooks); !errors.Is(err, rootcontrol.ErrRuleUnsupported) || calls != 0 || owners.Score.Snapshot().Ticks != 0 {
				t.Fatal("missing effect consumer silently succeeded", missing, err, calls)
			}
		})
	}
}
