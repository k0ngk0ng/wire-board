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

func jsonValueEnd(raw []byte, position int) int {
	depth := 0
	quoted := false
	for index := position; index < len(raw); index++ {
		value := raw[index]
		if quoted {
			if value == '\\' {
				index++
			} else if value == '"' {
				quoted = false
				if depth == 0 {
					return index + 1
				}
			}
			continue
		}
		switch value {
		case '"':
			quoted = true
		case '{', '[':
			depth++
		case '}', ']':
			if depth == 0 {
				return index
			}
			depth--
			if depth == 0 {
				return index + 1
			}
		case ',', ' ', '\n', '\r', '\t':
			if depth == 0 {
				return index
			}
		}
	}
	return len(raw)
}

func jsonMembers(raw []byte) (map[string]jsonSpan, bool) {
	position := jsonSpace(raw, 0)
	if position >= len(raw) || raw[position] != '{' {
		return nil, false
	}
	fields := make(map[string]jsonSpan)
	position = jsonSpace(raw, position+1)
	for position < len(raw) && raw[position] != '}' {
		end := jsonValueEnd(raw, position)
		var key string
		if json.Unmarshal(raw[position:end], &key) != nil {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}
		position = jsonSpace(raw, end)
		if position >= len(raw) || raw[position] != ':' {
			return nil, false
		}
		position = jsonSpace(raw, position+1)
		end = jsonValueEnd(raw, position)
		fields[key] = jsonSpan{begin: position, end: end}
		position = jsonSpace(raw, end)
		if position < len(raw) && raw[position] == ',' {
			position = jsonSpace(raw, position+1)
		}
	}
	return fields, true
}

func archiveField(raw []byte, fields map[string]jsonSpan, key string, target any) bool {
	span, exists := fields[key]
	return !exists || json.Unmarshal(raw[span.begin:span.end], target) == nil
}

func readInputArchive(match gameapi.Match, actor int) (inputArchive, bool) {
	var archive inputArchive
	raw := match.Data
	if !json.Valid(raw) {
		return archive, false
	}
	root, ok := jsonMembers(raw)
	if !ok {
		return archive, false
	}
	worldSpan, worldExists := root["world"]
	observed, observedExists := root["observedAt"]
	if !worldExists || !observedExists {
		return archive, false
	}
	worldRaw := raw[worldSpan.begin:worldSpan.end]
	world, ok := jsonMembers(worldRaw)
	if !ok {
		return archive, false
	}
	state := &matchState{}
	if !archiveField(raw, root, "schema", &state.Schema) || !archiveField(raw, root, "epoch", &state.Epoch) || !archiveField(raw, root, "baseAt", &state.BaseAt) || !archiveField(raw, root, "observedAt", &state.ObservedAt) || !archiveField(raw, root, "rulesSHA256", &state.RulesSHA256) || !archiveField(worldRaw, world, "frame", &state.World.Frame) || !archiveField(worldRaw, world, "mapId", &state.World.MapID) || !archiveField(worldRaw, world, "finished", &state.World.Finished) {
		return archive, false
	}
	var options struct {
		Map string `json:"map"`
	}
	if !archiveField(worldRaw, world, "options", &options) || !validStoredSchema(state.Schema) || len(state.Epoch) != 24 || state.BaseAt <= 0 || state.World.Frame < 0 || options.Map != state.World.MapID || len(state.RulesSHA256) != 64 || rulesIdentities[state.World.MapID] != state.RulesSHA256 {
		return archive, false
	}
	playersSpan, exists := world["players"]
	if !exists {
		return archive, false
	}
	players := worldRaw[playersSpan.begin:playersSpan.end]
	position := jsonSpace(players, 0)
	if position >= len(players) || players[position] != '[' {
		return archive, false
	}
	spans := make([]jsonSpan, 0, 10)
	position = jsonSpace(players, position+1)
	for position < len(players) && players[position] != ']' {
		end := jsonValueEnd(players, position)
		spans = append(spans, jsonSpan{begin: position, end: end})
		position = jsonSpace(players, end)
		if position < len(players) && players[position] == ',' {
			position = jsonSpace(players, position+1)
		}
	}
	if len(spans) < 2 || len(spans) > 10 || actor < 0 || actor >= len(spans) {
		return archive, false
	}
	state.World.Options.Map = options.Map
	state.World.Players = make([]engine.Player, len(spans))
	selected := spans[actor]
	if json.Unmarshal(players[selected.begin:selected.end], &state.World.Players[actor]) != nil {
		return archive, false
	}
	selected.begin += worldSpan.begin + playersSpan.begin
	selected.end += worldSpan.begin + playersSpan.begin
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
