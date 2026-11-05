package yuyuhakusho

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

var matchSerial atomic.Uint64

func Module() gameapi.Module {
	return gameapi.Module{ID: "yuyuhakusho", Name: "魔强统一战", MinPlayers: 2, MaxPlayers: 4, Realtime: true, Controls: &gameapi.ControllerDefinition{Layout: "md-6", LegacyProfile: "md-6"}, Configure: configure, Prepare: prepare, Start: start, Apply: apply, View: project, SyncSnapshot: syncSnapshot, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	if resources.err != nil {
		return gameapi.Match{}, resources.err
	}
	normalized, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var o Options
	if err = json.Unmarshal(normalized, &o); err != nil {
		return gameapi.Match{}, err
	}
	flags, err := hostAutomation(ctx, ctx.Players)
	if err != nil {
		return gameapi.Match{}, err
	}
	// Entropy is acquired once during package initialization. Match callbacks
	// perform only deterministic in-memory token derivation and rule evaluation.
	seedInput := append([]byte(nil), resources.nonce[:]...)
	seedInput = binary.BigEndian.AppendUint64(seedInput, matchSerial.Add(1))
	seedInput = binary.BigEndian.AppendUint64(seedInput, uint64(ctx.Now.UnixNano()))
	sum := sha256.Sum256(seedInput)
	config, err := sessionOptions(o, flags, binary.BigEndian.Uint32(sum[:4]))
	if err != nil {
		return gameapi.Match{}, err
	}
	world, events, err := engine.NewChuSession(config, resources.rules)
	if err != nil {
		return gameapi.Match{}, err
	}
	visual, err := renderstate.New(resources.graphics)
	if err != nil {
		return gameapi.Match{}, err
	}
	world, visual, err = renderstate.Advance(world, visual, events, resources.graphics)
	if err != nil {
		return gameapi.Match{}, err
	}
	s := State{ResourceDescriptorSHA256: resources.descriptorSHA, Options: o, Epoch: hex.EncodeToString(sum[4:16]), ServerAt: ctx.Now.UnixMilli(), World: world, Presentation: visual, Inputs: make([]InputState, ctx.Players), Loading: &LoadingState{Ready: make([]bool, ctx.Players)}, History: []syncwire.InputFrame{}}
	for seat := range s.Inputs {
		s.Inputs[seat].LastPacketAt = s.ServerAt
	}
	s.syncAutomation(flags)
	if err = s.finishLoading(s.ServerAt); err != nil {
		return gameapi.Match{}, err
	}
	return s.match()
}
func (s *State) finishLoading(now int64) error {
	if s.Loading == nil {
		return nil
	}
	for _, ready := range s.Loading.Ready {
		if !ready {
			return nil
		}
	}
	resource, err := engine.ChuSessionResourceForArena(s.Options.Arena, resources.rules)
	if err != nil {
		return err
	}
	world, err := engine.CompleteChuSessionResource(s.World, resource, resources.rules)
	if err != nil {
		return err
	}
	s.World = world
	s.Loading = nil
	s.ServerAt = now
	s.Remainder = 0
	return nil
}
func parseAction(raw json.RawMessage) (action, error) {
	var a action
	if len(raw) == 0 || len(raw) > maxActionBytes {
		return a, errors.New("操作大小无效")
	}
	fields, err := strictOptionsObject(raw, "type", "epoch", "sequence", "buttons")
	if err != nil {
		return a, err
	}
	if len(fields["type"]) == 0 || len(fields["epoch"]) == 0 || json.Unmarshal(fields["type"], &a.Type) != nil || json.Unmarshal(fields["epoch"], &a.Epoch) != nil {
		return a, errors.New("操作格式无效")
	}
	if a.Type == "loaded" {
		if fields["sequence"] != nil || fields["buttons"] != nil {
			return a, errors.New("加载确认包含多余字段")
		}
		return a, nil
	}
	if a.Type != "input" && a.Type != "release" {
		return a, errors.New("未知操作")
	}
	if fields["sequence"] == nil || json.Unmarshal(fields["sequence"], &a.Sequence) != nil {
		return a, errors.New("输入序号无效")
	}
	if a.Type == "input" && fields["buttons"] == nil {
		return a, errors.New("缺少按键状态")
	}
	if fields["buttons"] != nil && json.Unmarshal(fields["buttons"], &a.Buttons) != nil {
		return a, errors.New("按键状态无效")
	}
	if a.Type == "release" && a.Buttons != 0 {
		return a, errors.New("释放不能同时按键")
	}
	return a, engine.ValidateControllerButtons(a.Buttons)
}
func validAction(ctx gameapi.Context, s State, raw json.RawMessage) (action, []bool, error) {
	a, err := parseAction(raw)
	if err != nil {
		return a, nil, err
	}
	flags, err := hostAutomation(ctx, len(s.Inputs))
	if err != nil {
		return a, nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Inputs) || a.Epoch != s.Epoch || s.World.MatchComplete {
		return a, nil, errors.New("当前席位或对局不能操作")
	}
	if ctx.Now.UnixMilli() < s.ServerAt {
		return a, nil, errors.New("操作时钟已过期")
	}
	if a.Type == "loaded" {
		return a, flags, nil
	}
	if s.Loading != nil || flags[ctx.Actor] {
		return a, nil, errors.New("当前席位不能提交按键")
	}
	previous := s.Inputs[ctx.Actor].Sequence
	if a.Sequence <= previous || a.Sequence > maxSequence || a.Sequence <= 0 || a.Sequence-previous > maxSequenceJump {
		return a, nil, errors.New("输入序号已过期或超出范围")
	}
	if len(s.Inputs[ctx.Actor].Changes) >= maxInputQueue {
		return a, nil, errors.New("输入队列已满")
	}
	return a, flags, nil
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	a, flags, err := validAction(ctx, s, raw)
	if err != nil {
		return m, err
	}
	if a.Type == "loaded" {
		if s.Loading == nil {
			return m, nil
		}
		s.syncAutomation(flags)
		s.Loading.Ready[ctx.Actor] = true
		if err = s.finishLoading(ctx.Now.UnixMilli()); err != nil {
			return m, err
		}
		return s.match()
	}
	s.syncAutomation(flags)
	if err = s.advance(ctx.Now.UnixMilli()); err != nil {
		return m, err
	}
	if s.World.MatchComplete {
		return m, errors.New("比赛已经结束")
	}
	input := &s.Inputs[ctx.Actor]
	target := s.Frame + 1
	if target == 0 {
		return m, errors.New("帧计数已满")
	}
	if n := len(input.Changes); n > 0 && input.Changes[n-1].Frame >= target {
		target = input.Changes[n-1].Frame + 1
		if target == 0 {
			return m, errors.New("输入帧计数已满")
		}
	}
	input.Sequence = a.Sequence
	input.Buttons = a.Buttons
	input.LastPacketAt = ctx.Now.UnixMilli()
	input.Changes = append(input.Changes, InputChange{Frame: target, Sequence: a.Sequence, Buttons: a.Buttons})
	return s.match()
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, err := decode(m)
	if err != nil {
		return false
	}
	a, _, err := validAction(ctx, s, raw)
	return err == nil && a.Type != "loaded"
}
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	flags, err := hostAutomation(ctx, len(s.Inputs))
	if err != nil {
		return nil, err
	}
	if s.Loading != nil && ctx.Actor >= 0 && ctx.Actor < len(flags) && flags[ctx.Actor] && !s.Loading.Ready[ctx.Actor] {
		return json.Marshal(struct {
			Type  string `json:"type"`
			Epoch string `json:"epoch"`
		}{"loaded", s.Epoch})
	}
	return nil, nil
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Inputs) || s.World.MatchComplete {
		return m, nil
	}
	flags, err := hostAutomation(ctx, len(s.Inputs))
	if err != nil {
		return m, err
	}
	flags[ctx.Actor] = false
	s.syncAutomation(flags)
	input := &s.Inputs[ctx.Actor]
	input.Buttons = 0
	input.Changes = nil
	input.LastPacketAt = ctx.Now.UnixMilli()
	return s.match()
}
func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if s.World.MatchComplete {
		return m, nil
	}
	if _, err = hostAutomation(ctx, len(s.Inputs)); err != nil {
		return m, err
	}
	for seat := range s.Inputs {
		s.Inputs[seat].Buttons = 0
		s.Inputs[seat].Changes = nil
		s.Inputs[seat].LastPacketAt = ctx.Now.UnixMilli()
	}
	return s.match()
}
func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if s.Loading != nil || s.World.MatchComplete {
		return m, nil
	}
	flags, err := hostAutomation(ctx, len(s.Inputs))
	if err != nil {
		return m, err
	}
	s.syncAutomation(flags)
	if err = s.advance(ctx.Now.UnixMilli()); err != nil {
		return m, err
	}
	return s.match()
}
