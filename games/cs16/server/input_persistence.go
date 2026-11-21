package cs16

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type jsonSpan struct {
	begin int
	end   int
}

type inputArchive struct {
	state    *matchState
	actor    jsonSpan
	observed jsonSpan
}

func jsonSpace(raw []byte, position int) int {
	for position < len(raw) && (raw[position] == ' ' || raw[position] == '\n' || raw[position] == '\r' || raw[position] == '\t') {
		position++
	}
	return position
}

func archiveField(raw []byte, fields map[string]jsonSpan, key string, target any) bool {
	span, exists := fields[key]
	return !exists || json.Unmarshal(raw[span.begin:span.end], target) == nil
}

func readInputArchive(match gameapi.Match, actor int) (inputArchive, bool) {
	return readInputArchiveState(match, actor, true)
}

func readInputArchiveState(match gameapi.Match, actor int, complete bool) (inputArchive, bool) {
	var archive inputArchive
	raw := match.Data
	spans, valid := scanInputArchive(raw)
	if !valid || spans.duplicate {
		return archive, false
	}
	root, world := spans.root, spans.world
	observed, observedExists := root["observedAt"]
	if root["world"].end == 0 || !observedExists {
		return archive, false
	}
	state := &matchState{}
	if !archiveField(raw, root, "schema", &state.Schema) || !archiveField(raw, root, "epoch", &state.Epoch) || !archiveField(raw, root, "baseAt", &state.BaseAt) || !archiveField(raw, root, "observedAt", &state.ObservedAt) || !archiveField(raw, root, "rulesSHA256", &state.RulesSHA256) || !archiveField(raw, world, "frame", &state.World.Frame) || !archiveField(raw, world, "mapId", &state.World.MapID) || !archiveField(raw, world, "finished", &state.World.Finished) {
		return archive, false
	}
	var options struct {
		Map string `json:"map"`
	}
	if !archiveField(raw, world, "options", &options) || !validStoredSchema(state.Schema) || len(state.Epoch) != 24 || state.BaseAt <= 0 || state.World.Frame < 0 || options.Map != state.World.MapID || len(state.RulesSHA256) != 64 || rulesIdentities[state.World.MapID] != state.RulesSHA256 {
		return archive, false
	}
	if len(spans.players) < 2 || len(spans.players) > 10 || actor < 0 || actor >= len(spans.players) {
		return archive, false
	}
	state.World.Options.Map = options.Map
	state.World.Players = make([]engine.Player, len(spans.players))
	selected := spans.players[actor]
	if complete {
		if json.Unmarshal(raw[selected.begin:selected.end], &state.World.Players[actor]) != nil {
			return archive, false
		}
	} else {
		var player struct {
			Alive       bool  `json:"alive"`
			Life        int   `json:"life"`
			ReceivedSeq int64 `json:"receivedSeq"`
		}
		if json.Unmarshal(raw[selected.begin:selected.end], &player) != nil {
			return archive, false
		}
		state.World.Players[actor] = engine.Player{Alive: player.Alive, Life: player.Life, ReceivedSeq: player.ReceivedSeq}
	}
	archive = inputArchive{state: state, actor: selected, observed: observed}
	return archive, true
}

func applyArchivedInput(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage, archive inputArchive) (gameapi.Match, error) {
	state := archive.state
	action, err := validatedInput(ctx, state, raw)
	if err != nil {
		return match, err
	}
	if err = engine.QueueInput(&state.World, ctx.Actor, action.Input); err != nil {
		return match, err
	}
	player, err := json.Marshal(state.World.Players[ctx.Actor])
	if err != nil {
		return match, err
	}
	observed := strconv.AppendInt(nil, max(state.ObservedAt, ctx.Now.UnixMilli()), 10)
	first, second := archive.observed, archive.actor
	left, right := observed, player
	if first.begin > second.begin {
		first, second = second, first
		left, right = right, left
	}
	var data bytes.Buffer
	data.Grow(len(match.Data) + len(left) + len(right))
	data.Write(match.Data[:first.begin])
	data.Write(left)
	data.Write(match.Data[first.end:second.begin])
	data.Write(right)
	data.Write(match.Data[second.end:])
	match.Data = data.Bytes()
	match.HasComputerPlayers = computerParticipants(ctx, match.HasComputerPlayers)
	return match, nil
}
