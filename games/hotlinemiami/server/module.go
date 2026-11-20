package hotlinemiami

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type matchState struct {
	Schema int          `json:"schema"`
	Epoch  string       `json:"epoch"`
	BaseAt int64        `json:"baseAt"`
	World  engine.State `json:"world"`
}

type inputAction struct {
	Type  string `json:"type"`
	Epoch string `json:"epoch"`
	engine.Input
}

func computerParticipants(ctx gameapi.Context, previous bool) bool {
	for _, automated := range ctx.Automated {
		previous = previous || automated
	}
	return previous
}

func Module() gameapi.Module {
	return gameapi.Module{ID: "hotlinemiami", Name: "迈阿密热线", MinPlayers: 2, MaxPlayers: 4, Configure: configure, Prepare: prepare, Start: start, Apply: apply, View: project, Realtime: true, RealtimeDeadlineFrames: true, StreamView: streamView, SyncSnapshot: syncSnapshot, OnTimer: onTimer, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, IsInputRelease: isInputRelease}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	configured, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var options engine.Options
	if err = json.Unmarshal(configured, &options); err != nil {
		return gameapi.Match{}, err
	}
	var token [20]byte
	if _, err = rand.Read(token[:]); err != nil {
		return gameapi.Match{}, err
	}
	world, err := engine.New(options, ctx.Players, binary.LittleEndian.Uint64(token[:8]))
	if err != nil {
		return gameapi.Match{}, err
	}
	s := matchState{Schema: 1, Epoch: hex.EncodeToString(token[8:]), BaseAt: ctx.Now.UnixMilli(), World: *world}
	computer := false
	for _, automated := range ctx.Automated {
		computer = computer || automated
	}
	return envelope(&s, computer)
}

func decode(match gameapi.Match) (*matchState, error) {
	var state matchState
	if json.Unmarshal(match.Data, &state) != nil || state.Schema != 1 || state.Epoch == "" || len(state.World.Players) < 2 || len(state.World.Players) > 4 {
		return nil, errors.New("混战状态无法恢复")
	}
	state.World.World = engine.MapByID(state.World.MapID)
	if state.World.World.ID == "" {
		return nil, errors.New("混战地图版本无效")
	}
	return &state, nil
}

func validatedInput(ctx gameapi.Context, state *matchState, raw json.RawMessage) (inputAction, error) {
	var action inputAction
	fields, err := object(raw, "type", "epoch", "seq", "life", "moveX", "moveY", "aim", "fire", "throw", "pickup", "finish", "interact", "viewFrame")
	if err != nil {
		return action, err
	}
	if json.Unmarshal(raw, &action) != nil || action.Type != "input" || action.Epoch != state.Epoch || fields["seq"] == nil || fields["life"] == nil {
		return action, errors.New("操作已过期，请重新连接")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(state.World.Players) || state.World.Finished {
		return action, errors.New("当前席位不能操作")
	}
	player := state.World.Players[ctx.Actor]
	if action.Life != player.Life || action.Seq <= player.ReceivedSeq || action.Seq-player.ReceivedSeq > 4096 || action.Seq > engine.MaxInputSeq {
		return action, errors.New("操作序号或生命代次已过期")
	}
	if action.MoveX < -1000 || action.MoveX > 1000 || action.MoveY < -1000 || action.MoveY > 1000 || action.Aim < 0 || action.Aim > 1023 {
		return action, errors.New("移动或瞄准超出范围")
	}
	if action.ViewFrame < 0 || action.ViewFrame > state.World.Frame {
		return action, errors.New("瞄准参考帧超出权威窗口")
	}
	if !ctx.InputReceivedAt.IsZero() {
		age := ctx.Now.UnixMilli() - ctx.InputReceivedAt.UnixMilli()
		if age > 200 || age < -50 {
			return action, errors.New("操作排队已过期")
		}
	}
	return action, nil
}

func apply(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	action, err := validatedInput(ctx, state, raw)
	if err != nil {
		return match, err
	}
	if err = engine.QueueInput(&state.World, ctx.Actor, action.Input); err != nil {
		return match, err
	}
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers))
}

func acceptStale(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
	state, err := decode(match)
	if err != nil {
		return false
	}
	_, err = validatedInput(ctx, state, raw)
	return err == nil
}

func isInputRelease(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
	state, err := decode(match)
	if err != nil {
		return false
	}
	action, err := validatedInput(ctx, state, raw)
	return err == nil && action.MoveX == 0 && action.MoveY == 0 && !action.Fire && !action.Throw && !action.Pickup && !action.Finish && !action.Interact
}

func onTimer(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	frame := (ctx.Now.UnixMilli() - state.BaseAt) * engine.FPS / 1000
	if frame < state.World.Frame {
		return match, nil
	}
	frame = min(frame, state.World.EndFrame)
	engine.Advance(&state.World, frame)
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers))
}

func botAction(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
	state, err := decode(match)
	if err != nil {
		return nil, err
	}
	if state.World.Finished || ctx.Actor < 0 || ctx.Actor >= len(state.World.Players) {
		return nil, nil
	}
	input := engine.BotInput(&state.World, ctx.Actor)
	return json.Marshal(inputAction{Type: "input", Epoch: state.Epoch, Input: input})
}

func reclaim(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Actor >= 0 && ctx.Actor < len(state.World.Players) {
		engine.ClearInput(&state.World, ctx.Actor)
	}
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers))
}

func restore(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	for seat := range state.World.Players {
		engine.ClearInput(&state.World, seat)
	}
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers))
}
