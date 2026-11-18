package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

const runtimeABI = "wire-board-pokepath-go-v1"
const maximumRequestBytes = 8 << 20
const maximumRetainedStates = 128

type retainedState struct {
	state            *engine.State
	revision         uint32
	geometryRevision uint32
}

var retainedStates = map[uint32]retainedState{}
var nextRetainedID uint32
var nextProjectionRevision uint32

type request struct {
	Op           string          `json:"op"`
	Options      json.RawMessage `json:"options,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	State        *engine.State   `json:"state,omitempty"`
	Action       json.RawMessage `json:"action,omitempty"`
	Actor        int             `json:"actor,omitempty"`
	Players      int             `json:"players,omitempty"`
	Seed         json.RawMessage `json:"seed,omitempty"`
	Now          int64           `json:"now,omitempty"`
	Milliseconds int64           `json:"milliseconds,omitempty"`
	PokemonID    int             `json:"pokemonID,omitempty"`
	TileID       int             `json:"tileID,omitempty"`
	Handle       uint32          `json:"handle,omitempty"`
	Handles      []uint32        `json:"handles,omitempty"`
	Compact      bool            `json:"compact,omitempty"`
}

type response struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}

func strict(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

func runRequest(raw []byte) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > maximumRequestBytes {
		return nil, errors.New("浏览器规则请求大小无效")
	}
	var input request
	if err := strict(raw, &input); err != nil {
		return nil, err
	}
	if input.Op == "info" {
		return json.Marshal(map[string]any{"abi": runtimeABI, "ready": true, "retainedStates": len(retainedStates), "retainedLimit": maximumRetainedStates})
	}
	if strings.HasPrefix(input.Op, "session.") {
		return runSessionRequest(input)
	}
	if input.Op == "installCatalog" || input.Op == "installRoute" {
		var err error
		if input.Op == "installCatalog" {
			err = engine.InstallCatalog(input.Data)
		} else {
			err = engine.InstallRoute(input.Data)
		}
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"installed": true})
	}
	if input.Op == "new" {
		var options engine.Options
		if err := strict(input.Options, &options); err != nil {
			return nil, err
		}
		seedText := string(input.Seed)
		if len(seedText) > 0 && seedText[0] == '"' {
			if err := json.Unmarshal(input.Seed, &seedText); err != nil {
				return nil, err
			}
		}
		seed, err := strconv.ParseUint(seedText, 10, 64)
		if err != nil {
			return nil, errors.New("浏览器随机种子无效")
		}
		state, err := engine.New(options, input.Players, seed, input.Now)
		if err != nil {
			return nil, err
		}
		return engine.Marshal(state)
	}
	state := input.State
	if err := engine.Validate(state); err != nil {
		return nil, err
	}
	switch input.Op {
	case "apply":
		var action engine.Action
		if err := strict(input.Action, &action); err != nil {
			return nil, err
		}
		if err := engine.Apply(state, input.Actor, action); err != nil {
			return nil, err
		}
	case "advance":
		if err := engine.Advance(state, input.Milliseconds); err != nil {
			return nil, err
		}
	case "project":
		return json.Marshal(engine.Project(state))
	case "clone", "restore":
	default:
		return nil, errors.New("未知浏览器规则操作")
	}
	return engine.Marshal(state)
}

func handleRequest(this js.Value, arguments []js.Value) (result any) {
	defer func() {
		if failure := recover(); failure != nil {
			encoded, _ := json.Marshal(response{Error: fmt.Sprint(failure)})
			result = string(encoded)
		}
	}()
	if len(arguments) != 1 || arguments[0].Type() != js.TypeString {
		encoded, _ := json.Marshal(response{Error: "浏览器规则请求必须为 JSON 字符串"})
		return string(encoded)
	}
	value, err := runRequest([]byte(arguments[0].String()))
	if err == nil {
		encoded := make([]byte, 0, len(value)+20)
		encoded = append(encoded, `{"ok":true,"value":`...)
		encoded = append(encoded, value...)
		encoded = append(encoded, '}')
		return string(encoded)
	}
	output := response{OK: err == nil, Value: value}
	if err != nil {
		output.Error = err.Error()
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		encoded, _ = json.Marshal(response{Error: "浏览器规则结果编码失败"})
	}
	return string(encoded)
}

func main() {
	bridge := js.Global().Get("Object").New()
	bridge.Set("abi", runtimeABI)
	bridge.Set("ready", true)
	bridge.Set("request", js.FuncOf(handleRequest))
	js.Global().Set("wireBoardPokepathNative", bridge)
	select {}
}

func retainState(state *engine.State, revision, geometryRevision uint32) (json.RawMessage, error) {
	if len(retainedStates) >= maximumRetainedStates {
		return nil, errors.New("预测状态未释放，无法继续分配")
	}
	nextRetainedID++
	if nextRetainedID == 0 {
		return nil, errors.New("预测状态编号已用尽")
	}
	retainedStates[nextRetainedID] = retainedState{state: state, revision: revision, geometryRevision: geometryRevision}
	return json.Marshal(map[string]any{"nativeHandle": nextRetainedID, "projectionRevision": revision, "geometryRevision": geometryRevision, "clock": state.Clock, "epoch": state.Epoch, "sequence": state.Sequence, "physicsClock": state.Tick, "tick": state.Tick, "battleFrame": state.BattleFrame, "remainderMS": state.RemainderMS, "speedRemainder": state.SpeedRemainder, "paused": state.Paused, "route": state.Route})
}

type compactEnemy struct {
	compactEnemyIdentity
	compactEnemyEffects
	NegativeZeros []string           `json:"negativeZeros,omitempty"`
	Sprite        *engine.SpriteSpec `json:"sprite,omitempty"`
	SpriteKey     string             `json:"spriteKey,omitempty"`
}

type compactPokemon struct {
	engine.Pokemon
	Sprite    *engine.SpriteSpec `json:"sprite,omitempty"`
	SpriteKey string             `json:"spriteKey,omitempty"`
}

type compactProjection struct {
	engine.View
	EnemyDefaults engine.Enemy     `json:"enemyDefaults"`
	Tiles         any              `json:"tiles,omitempty"`
	Paths         any              `json:"paths,omitempty"`
	Background    any              `json:"background,omitempty"`
	Width         any              `json:"width,omitempty"`
	Height        any              `json:"height,omitempty"`
	Team          []compactPokemon `json:"team"`
	Box           []compactPokemon `json:"box"`
	Enemies       []compactEnemy   `json:"enemies"`
}

func compactParty(pokemon []engine.Pokemon) []compactPokemon {
	if pokemon == nil {
		return nil
	}
	result := make([]compactPokemon, len(pokemon))
	catalog := engine.CatalogData()
	for index, member := range pokemon {
		sprite := member.Sprite
		result[index] = compactPokemon{Pokemon: member, Sprite: &sprite}
		if species, ok := catalog.Species[member.Species]; ok && species.Sprite == sprite {
			result[index].Sprite = nil
			result[index].SpriteKey = member.Species
		}
	}
	return result
}

func projectRetained(state *engine.State, compact bool) (json.RawMessage, error) {
	view := engine.ProjectTransient(state)
	if !compact {
		return json.Marshal(view)
	}
	result := compactProjection{View: view, EnemyDefaults: engine.Enemy{EnemyEffectState: engine.EnemyEffectState{Conditions: []engine.Condition{}}}, Team: compactParty(view.Team), Box: compactParty(view.Box)}
	if view.Enemies != nil {
		result.Enemies = make([]compactEnemy, len(view.Enemies))
		catalog := engine.CatalogData()
		for index, enemy := range view.Enemies {
			sprite := enemy.Sprite
			result.Enemies[index] = compactEnemy{compactEnemyIdentity: compactEnemyIdentity(enemy), compactEnemyEffects: compactEnemyEffects(enemy.EnemyEffectState), NegativeZeros: enemyNegativeZeros(&enemy), Sprite: &sprite}
			if species, ok := catalog.Enemies[enemy.Key]; ok && species.Sprite == sprite {
				result.Enemies[index].Sprite = nil
				result.Enemies[index].SpriteKey = enemy.Key
			}
		}
	}
	return json.Marshal(result)
}

func runSessionRequest(input request) (json.RawMessage, error) {
	if input.Op == "session.import" {
		state := input.State
		if err := engine.Validate(state); err != nil {
			return nil, err
		}
		nextProjectionRevision++
		return retainState(state, nextProjectionRevision, nextProjectionRevision)
	}
	if input.Op == "session.release" {
		if len(input.Handles) > maximumRetainedStates {
			return nil, errors.New("预测释放请求过大")
		}
		for _, id := range input.Handles {
			delete(retainedStates, id)
		}
		return json.Marshal(map[string]int{"live": len(retainedStates)})
	}
	stored, ok := retainedStates[input.Handle]
	if !ok {
		return nil, errors.New("预测状态已释放或不存在")
	}
	switch input.Op {
	case "session.project":
		return projectRetained(stored.state, input.Compact)
	case "session.export":
		return engine.Marshal(stored.state)
	case "session.deployment":
		return json.Marshal(engine.ProjectDeployment(stored.state, input.PokemonID))
	case "session.deploymentRange":
		footprint, err := engine.ProjectDeploymentRange(stored.state, input.PokemonID, input.TileID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(footprint)
	case "session.inspect":
		pokemon, err := engine.ProjectPokemon(stored.state, input.PokemonID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(pokemon)
	}
	if len(retainedStates) >= maximumRetainedStates {
		return nil, errors.New("预测状态未释放，无法继续分配")
	}
	state := new(engine.State)
	*state = *stored.state
	var err error
	revision := stored.revision
	switch input.Op {
	case "session.clone":
		state, err = engine.Clone(stored.state)
		if err != nil {
			return nil, err
		}
	case "session.apply":
		var action engine.Action
		if err = strict(input.Action, &action); err != nil {
			return nil, err
		}
		if err = engine.Apply(state, input.Actor, action); err != nil {
			return nil, err
		}
		nextProjectionRevision++
		revision = nextProjectionRevision
	case "session.advance":
		if err = engine.Advance(state, input.Milliseconds); err != nil {
			return nil, err
		}
		if state.Route != stored.state.Route {
			nextProjectionRevision++
			revision = nextProjectionRevision
		}
	default:
		return nil, errors.New("未知预测状态操作")
	}
	geometryRevision := stored.geometryRevision
	if state.Route != stored.state.Route || engine.ProgressionMirrored(state) != engine.ProgressionMirrored(stored.state) || !reflect.DeepEqual(state.Progression.CustomRoute, stored.state.Progression.CustomRoute) {
		nextProjectionRevision++
		geometryRevision = nextProjectionRevision
	}
	return retainState(state, revision, geometryRevision)
}
