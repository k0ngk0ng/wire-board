package cs16

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type matchState struct {
	Schema        int          `json:"schema"`
	Epoch         string       `json:"epoch"`
	BaseAt        int64        `json:"baseAt"`
	ObservedAt    int64        `json:"observedAt"`
	RulesSHA256   string       `json:"rulesSHA256"`
	World         engine.State `json:"world"`
	history       json.RawMessage
	historyPacked []byte
	historyBytes  int
}

type inputAction struct {
	Type   string `json:"type"`
	Epoch  string `json:"epoch"`
	Cancel bool   `json:"cancel,omitempty"`
	engine.Input
}

type buyAction struct {
	Type  string `json:"type"`
	Epoch string `json:"epoch"`
	Item  string `json:"item"`
	Life  int    `json:"life"`
	Round int    `json:"round"`
}

func Module() gameapi.Module {
	return gameapi.Module{ID: "cs16", Name: "反恐精英", MinPlayers: 2, MaxPlayers: 10, Configure: configure, Prepare: prepare, Start: start, Apply: apply, View: project, Realtime: true, RealtimeDeadlineFrames: true, RealtimeDeferredInput: deferredInput, StreamView: streamView, SyncSnapshot: syncSnapshot, OnTimer: onTimer, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, IsInputRelease: isInputRelease}
}

func computerParticipants(ctx gameapi.Context, previous bool) bool {
	for _, automated := range ctx.Automated {
		previous = previous || automated
	}
	return previous
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
	state := matchState{Schema: 1, Epoch: hex.EncodeToString(token[8:]), BaseAt: ctx.Now.UnixMilli(), RulesSHA256: rulesIdentities[world.MapID], World: *world}
	return envelope(&state, computerParticipants(ctx, false), ctx.Now.UnixMilli())
}

func decode(match gameapi.Match) (*matchState, error) {
	var state matchState
	if state.UnmarshalJSON(match.Data) != nil || !validStoredSchema(state.Schema) || len(state.Epoch) != 24 || state.BaseAt <= 0 || state.World.Frame < 0 || len(state.World.Players) < 2 || len(state.World.Players) > 10 {
		return nil, errors.New("反恐精英状态无法恢复")
	}
	state.World.World = engine.MapByID(state.World.MapID)
	if state.World.World.ID == "" || state.World.Options.Map != state.World.MapID || len(state.RulesSHA256) != 64 || state.RulesSHA256 != rulesIdentities[state.World.MapID] {
		return nil, errors.New("反恐精英地图版本无效")
	}
	return &state, nil
}

func released(input engine.Input) bool {
	return input.MoveX == 0 && input.MoveY == 0 && !input.Fire && !input.Secondary && !input.Jump && !input.Crouch && !input.Walk && !input.Reload && !input.Use && !input.Drop && input.Select == ""
}

func validatedInput(ctx gameapi.Context, state *matchState, raw json.RawMessage) (inputAction, error) {
	var action inputAction
	fields, err := object(raw, "type", "epoch", "seq", "life", "viewFrame", "moveX", "moveY", "yaw", "pitch", "fire", "secondary", "jump", "crouch", "walk", "reload", "use", "drop", "select", "cancel")
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
	if action.MoveX < -1000 || action.MoveX > 1000 || action.MoveY < -1000 || action.MoveY > 1000 || action.Yaw < 0 || action.Yaw > 65535 || action.Pitch < -16000 || action.Pitch > 16000 {
		return action, errors.New("移动或视角超出范围")
	}
	if action.ViewFrame < 0 || action.ViewFrame > state.World.Frame {
		return action, errors.New("瞄准参考帧超出权威窗口")
	}
	if action.Cancel && !released(action.Input) {
		return action, errors.New("取消操作必须释放全部输入")
	}
	if !player.Alive && !released(action.Input) {
		return action, errors.New("当前角色已经阵亡")
	}
	if ctx.Now.UnixMilli()-state.BaseAt-state.World.Frame*1000/engine.FPS > 200 && !released(action.Input) {
		return action, errors.New("服务器正在恢复对局时钟，请稍候")
	}
	if !ctx.InputReceivedAt.IsZero() {
		age := ctx.Now.UnixMilli() - ctx.InputReceivedAt.UnixMilli()
		if age > 200 || age < -50 {
			return action, errors.New("操作排队已过期")
		}
	}
	if !action.Cancel {
		receivedAt := ctx.Now.UnixMilli()
		if !ctx.InputReceivedAt.IsZero() {
			receivedAt = ctx.InputReceivedAt.UnixMilli()
		}
		transportAge := receivedAt - state.BaseAt - action.ViewFrame*1000/engine.FPS
		if transportAge > 500 || transportAge < -50 {
			return action, errors.New("操作在传输中已过期，请等待重新同步")
		}
	}
	return action, nil
}

func validatedBuy(ctx gameapi.Context, state *matchState, raw json.RawMessage) (buyAction, error) {
	var action buyAction
	fields, err := object(raw, "type", "epoch", "item", "life", "round")
	if err != nil {
		return action, err
	}
	if json.Unmarshal(raw, &action) != nil || action.Type != "buy" || action.Epoch != state.Epoch || fields["item"] == nil || fields["life"] == nil || fields["round"] == nil || len(action.Item) < 1 || len(action.Item) > 32 {
		return action, errors.New("购买操作已过期或物品无效")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(state.World.Players) || state.World.Finished || !engine.CanBuy(&state.World, ctx.Actor) {
		return action, errors.New("当前无法购买装备")
	}
	if action.Life != state.World.Players[ctx.Actor].Life || action.Round != state.World.Round {
		return action, errors.New("购买操作的回合或生命已过期")
	}
	if ctx.Now.UnixMilli()-state.BaseAt-state.World.Frame*1000/engine.FPS > 200 {
		return action, errors.New("服务器正在恢复对局时钟，请稍候")
	}
	return action, nil
}

func actionType(raw json.RawMessage) string {
	var value struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value.Type
}

func apply(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	if deferredInput(raw) {
		if archive, ok := readInputArchive(match, ctx.Actor); ok {
			return applyArchivedInput(ctx, match, raw, archive)
		}
	}
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	switch actionType(raw) {
	case "input":
		action, err := validatedInput(ctx, state, raw)
		if err != nil {
			return match, err
		}
		if action.Cancel {
			engine.ClearInput(&state.World, ctx.Actor)
			action.Yaw, action.Pitch = state.World.Players[ctx.Actor].Yaw, state.World.Players[ctx.Actor].Pitch
		}
		if err = engine.QueueInput(&state.World, ctx.Actor, action.Input); err != nil {
			return match, err
		}
		if action.Cancel {
			engine.Emit(&state.World, engine.Event{Kind: "input_cancel", Seat: ctx.Actor, Target: ctx.Actor, Life: action.Life})
		}
	case "buy":
		action, err := validatedBuy(ctx, state, raw)
		if err != nil {
			return match, err
		}
		if err = engine.Buy(&state.World, ctx.Actor, action.Item); err != nil {
			return match, err
		}
	default:
		return match, errors.New("未知游戏操作")
	}
	updated, err := envelope(state, computerParticipants(ctx, match.HasComputerPlayers), ctx.Now.UnixMilli())
	if err == nil {
		updated.SystemDeadline = match.SystemDeadline
	}
	return updated, err
}

func acceptStale(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
	if actionType(raw) == "input" {
		state, err := decodeInputState(match, ctx.Actor)
		if err != nil {
			return false
		}
		_, err = validatedInput(ctx, state, raw)
		return err == nil
	}
	state, err := decode(match)
	if err != nil {
		return false
	}
	if actionType(raw) == "buy" {
		_, err = validatedBuy(ctx, state, raw)
		return err == nil
	}
	return false
}

func isInputRelease(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
	state, err := decode(match)
	if err != nil {
		return false
	}
	action, err := validatedInput(ctx, state, raw)
	return err == nil && action.Cancel && released(action.Input)
}

func onTimer(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	frame := (ctx.Now.UnixMilli() - state.BaseAt) * engine.FPS / 1000
	if frame <= state.World.Frame {
		return match, nil
	}
	if err = loadHistory(state); err != nil {
		return match, err
	}
	if ctx.Now.UnixMilli()-state.BaseAt-state.World.Frame*1000/engine.FPS <= 200 {
		for seat := range state.World.Players {
			if seat >= len(ctx.Automated) || !ctx.Automated[seat] {
				continue
			}
			if item := engine.BotPurchase(&state.World, seat); item != "" {
				if err = engine.Buy(&state.World, seat, item); err != nil {
					return match, err
				}
				continue
			}
			if !state.World.Players[seat].Alive {
				continue
			}
			input := engine.BotInput(&state.World, seat)
			if err = engine.QueueInput(&state.World, seat, input); err != nil {
				return match, err
			}
		}
	}
	engine.Advance(&state.World, frame)
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers), ctx.Now.UnixMilli())
}

func botAction(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
	if ctx.Actor >= 0 && ctx.Actor < len(ctx.Automated) && ctx.Automated[ctx.Actor] {
		return nil, nil
	}
	state, err := decode(match)
	if err != nil {
		return nil, err
	}
	if state.World.Finished || ctx.Actor < 0 || ctx.Actor >= len(state.World.Players) {
		return nil, nil
	}
	if ctx.Now.UnixMilli()-state.BaseAt-state.World.Frame*1000/engine.FPS > 200 {
		return nil, nil
	}
	if item := engine.BotPurchase(&state.World, ctx.Actor); item != "" {
		return json.Marshal(buyAction{Type: "buy", Epoch: state.Epoch, Item: item, Life: state.World.Players[ctx.Actor].Life, Round: state.World.Round})
	}
	if !state.World.Players[ctx.Actor].Alive {
		return nil, nil
	}
	input := engine.BotInput(&state.World, ctx.Actor)
	input.QueuedFrame = 0
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
	updated, err := envelope(state, computerParticipants(ctx, match.HasComputerPlayers), ctx.Now.UnixMilli())
	if err == nil {
		updated.SystemDeadline = match.SystemDeadline
	}
	return updated, err
}

func restore(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	for seat := range state.World.Players {
		engine.ClearInput(&state.World, seat)
	}
	return envelope(state, computerParticipants(ctx, match.HasComputerPlayers), ctx.Now.UnixMilli())
}
