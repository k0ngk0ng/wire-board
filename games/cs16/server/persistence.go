package cs16

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type historyWorld struct {
	engine.State
	History json.RawMessage `json:"history"`
}

type storedMatchState struct {
	Schema        int          `json:"schema"`
	Epoch         string       `json:"epoch"`
	BaseAt        int64        `json:"baseAt"`
	ObservedAt    int64        `json:"observedAt"`
	RulesSHA256   string       `json:"rulesSHA256"`
	World         historyWorld `json:"world"`
	HistoryPacked []byte       `json:"historyPacked,omitempty"`
	HistoryBytes  int          `json:"historyBytes,omitempty"`
}

func (state *matchState) UnmarshalJSON(raw []byte) error {
	var stored storedMatchState
	if err := json.Unmarshal(raw, &stored); err != nil {
		return err
	}
	if stored.Schema == 2 {
		if len(stored.HistoryPacked) == 0 || len(stored.HistoryPacked) > maxHistoryBytes || stored.HistoryBytes <= 0 || stored.HistoryBytes > maxHistoryBytes || len(stored.World.History) != 0 && !bytes.Equal(bytes.TrimSpace(stored.World.History), []byte("null")) {
			return errors.New("历史检查点封装无效")
		}
	} else if len(stored.HistoryPacked) != 0 || stored.HistoryBytes != 0 {
		return errors.New("历史检查点版本无效")
	}
	state.Schema, state.Epoch, state.BaseAt, state.ObservedAt, state.RulesSHA256 = stored.Schema, stored.Epoch, stored.BaseAt, stored.ObservedAt, stored.RulesSHA256
	state.World = stored.World.State
	state.history = stored.World.History
	state.historyPacked, state.historyBytes = stored.HistoryPacked, stored.HistoryBytes
	return nil
}

func (state *matchState) MarshalJSON() ([]byte, error) {
	packed, size := state.historyPacked, state.historyBytes
	if state.World.History != nil || len(packed) == 0 {
		history := state.history
		var err error
		if state.World.History != nil || len(history) == 0 {
			history, err = json.Marshal(state.World.History)
			if err != nil {
				return nil, err
			}
		}
		packed, err = compressHistory(history)
		if err != nil {
			return nil, err
		}
		size = len(history)
	}
	return json.Marshal(storedMatchState{Schema: 2, Epoch: state.Epoch, BaseAt: state.BaseAt, ObservedAt: state.ObservedAt, RulesSHA256: state.RulesSHA256, World: historyWorld{State: state.World}, HistoryPacked: packed, HistoryBytes: size})
}

func loadHistory(state *matchState) error {
	if state.World.History != nil {
		return nil
	}
	history := state.history
	if len(state.historyPacked) != 0 {
		var err error
		history, err = expandHistory(state.historyPacked, state.historyBytes)
		if err != nil {
			return err
		}
	}
	if len(history) == 0 {
		return nil
	}
	if err := json.Unmarshal(history, &state.World.History); err != nil {
		return errors.New("历史检查点无法恢复")
	}
	if len(state.World.History) > engine.MaxHistory {
		return errors.New("历史检查点超出窗口")
	}
	previous := int64(-1)
	for _, frame := range state.World.History {
		if frame.Frame <= previous || frame.Frame > state.World.Frame || len(frame.Players) != len(state.World.Players) {
			return errors.New("历史检查点时钟或席位无效")
		}
		previous = frame.Frame
	}
	return nil
}

func unchangedProjection(match gameapi.Match, seat int, cursor string) (bool, error) {
	if cursor == "" {
		return false, nil
	}
	var header struct {
		Schema      int    `json:"schema"`
		Epoch       string `json:"epoch"`
		BaseAt      int64  `json:"baseAt"`
		ObservedAt  int64  `json:"observedAt"`
		RulesSHA256 string `json:"rulesSHA256"`
		World       struct {
			Frame    int64  `json:"frame"`
			EventID  int64  `json:"eventId"`
			Round    int    `json:"round"`
			Finished bool   `json:"finished"`
			MapID    string `json:"mapId"`
			Options  struct {
				Map string `json:"map"`
			} `json:"options"`
			Players []struct {
				Alive bool `json:"alive"`
			} `json:"players"`
			Brushes json.RawMessage `json:"brushes"`
		} `json:"world"`
	}
	if err := json.Unmarshal(match.Data, &header); err != nil {
		return false, err
	}
	if !validStoredSchema(header.Schema) || len(header.Epoch) != 24 || header.BaseAt <= 0 || header.World.Frame < 0 || len(header.World.Players) < 2 || len(header.World.Players) > 10 || header.World.Options.Map != header.World.MapID || len(header.RulesSHA256) != 64 || header.RulesSHA256 != rulesIdentities[header.World.MapID] {
		return false, errors.New("实时投影身份无效")
	}
	catchingUp := header.ObservedAt-header.BaseAt-header.World.Frame*1000/engine.FPS > 200
	canAct := seat >= 0 && seat < len(header.World.Players) && header.World.Players[seat].Alive && !header.World.Finished && !catchingUp
	canActBit, catchingUpBit := 0, 0
	if canAct {
		canActBit = 1
	}
	if catchingUp {
		catchingUpBit = 1
	}
	expected := fmt.Sprintf("%s:%d:%d:%d:%x:%d:%d:%d", header.Epoch, header.World.Frame, header.World.EventID, header.World.Round, sha256.Sum256(header.World.Brushes), canActBit, catchingUpBit, seat)
	return cursor == expected, nil
}

func decodeInputState(match gameapi.Match, actor int) (*matchState, error) {
	if archive, ok := readInputArchive(match, actor); ok {
		return archive.state, nil
	}
	var header struct {
		Schema      int    `json:"schema"`
		Epoch       string `json:"epoch"`
		BaseAt      int64  `json:"baseAt"`
		RulesSHA256 string `json:"rulesSHA256"`
		World       struct {
			Frame    int64  `json:"frame"`
			Finished bool   `json:"finished"`
			MapID    string `json:"mapId"`
			Options  struct {
				Map string `json:"map"`
			} `json:"options"`
			Players []struct {
				Alive       bool  `json:"alive"`
				Life        int   `json:"life"`
				ReceivedSeq int64 `json:"receivedSeq"`
			} `json:"players"`
		} `json:"world"`
	}
	if err := json.Unmarshal(match.Data, &header); err != nil {
		return nil, err
	}
	if !validStoredSchema(header.Schema) || len(header.Epoch) != 24 || header.BaseAt <= 0 || header.World.Frame < 0 || len(header.World.Players) < 2 || len(header.World.Players) > 10 || header.World.MapID != header.World.Options.Map || len(header.RulesSHA256) != 64 || header.RulesSHA256 != rulesIdentities[header.World.MapID] {
		return nil, errors.New("实时输入身份无效")
	}
	state := &matchState{Schema: header.Schema, Epoch: header.Epoch, BaseAt: header.BaseAt, RulesSHA256: header.RulesSHA256, World: engine.State{Frame: header.World.Frame, Finished: header.World.Finished, Players: make([]engine.Player, len(header.World.Players))}}
	if actor >= 0 && actor < len(header.World.Players) {
		player := header.World.Players[actor]
		state.World.Players[actor] = engine.Player{Alive: player.Alive, Life: player.Life, ReceivedSeq: player.ReceivedSeq}
	}
	return state, nil
}
