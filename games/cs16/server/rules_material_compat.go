package cs16

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/cs16/data"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type materialRulesTransition struct {
	MapID          string `json:"mapId"`
	From           string `json:"from"`
	To             string `json:"to"`
	GeometrySHA256 string `json:"geometrySHA256"`
}

var materialRulesCompatibility = loadMaterialRulesCompatibility()

func materialRulesGeometry(raw json.RawMessage) (string, error) {
	var rule map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&rule); err != nil {
		return "", err
	}
	if values, ok := rule["brushes"].([]any); ok {
		for _, value := range values {
			brush, valid := value.(map[string]any)
			if !valid {
				return "", errors.New("地图实体材料格式无效")
			}
			delete(brush, "material")
		}
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(rule); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}))), nil
}

func loadMaterialRulesCompatibility() map[string]materialRulesTransition {
	var transitions []materialRulesTransition
	var rows []json.RawMessage
	if json.Unmarshal([]byte(data.MaterialRulesCompatibilityJSON), &transitions) != nil || json.Unmarshal([]byte(data.MapsJSON), &rows) != nil {
		panic("地图材料兼容记录无法读取")
	}
	geometry := map[string]string{}
	for _, row := range rows {
		var header struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(row, &header) != nil {
			panic("地图材料身份无法读取")
		}
		identity, err := materialRulesGeometry(row)
		if err != nil {
			panic(err)
		}
		geometry[header.ID] = identity
	}
	known := map[string]materialRulesTransition{}
	for _, row := range transitions {
		from, err := hex.DecodeString(row.From)
		if err != nil || len(from) != 32 || row.From == row.To || row.To != rulesIdentities[row.MapID] || row.GeometrySHA256 != geometry[row.MapID] || known[row.From].From != "" {
			panic("地图材料兼容记录与当前规则不一致")
		}
		known[row.From] = row
	}
	return known
}

func migrateMaterialRules(match gameapi.Match) (gameapi.Match, error) {
	var header struct {
		RulesSHA256 string `json:"rulesSHA256"`
		World       struct {
			MapID   string `json:"mapId"`
			Options struct {
				Map string `json:"map"`
			} `json:"options"`
		} `json:"world"`
	}
	if err := json.Unmarshal(match.Data, &header); err != nil {
		return match, err
	}
	transition, known := materialRulesCompatibility[header.RulesSHA256]
	if !known {
		return match, nil
	}
	if transition.MapID != header.World.MapID || header.World.Options.Map != header.World.MapID {
		return match, errors.New("旧地图材料身份与房间地图不一致")
	}
	archive, valid := scanInputArchive(match.Data)
	span := archive.root["rulesSHA256"]
	if !valid || archive.duplicate || span.begin < 0 || span.end <= span.begin {
		return match, errors.New("旧地图材料检查点格式无效")
	}
	identity, err := json.Marshal(transition.To)
	if err != nil {
		return match, err
	}
	updated := make([]byte, 0, len(match.Data)+len(identity)-(span.end-span.begin))
	updated = append(updated, match.Data[:span.begin]...)
	updated = append(updated, identity...)
	updated = append(updated, match.Data[span.end:]...)
	match.Data = updated
	return match, nil
}
