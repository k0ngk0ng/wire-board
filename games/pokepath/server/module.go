package pokepath

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const moduleSchema = 2
const tickMS int64 = 100
const journalLimit = 128
const receiptLimit = 32
const maxSequence = 1<<53 - 1
const maxEpoch = 1000000000
const maxStateBytes = 16 << 20

type operation struct {
	Type          string         `json:"type"`
	Action        *engine.Action `json:"action,omitempty"`
	Milliseconds  int64          `json:"milliseconds,omitempty"`
	Actor         int            `json:"actor"`
	ClientInputID string         `json:"clientInputID,omitempty"`
}

type journalEntry struct {
	Sequence  uint64    `json:"sequence"`
	Operation operation `json:"operation"`
}

type inputReceipt struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type state struct {
	Schema         int              `json:"schema"`
	Hash           string           `json:"hash"`
	Snapshot       *engine.State    `json:"snapshot"`
	Sequence       uint64           `json:"sequence"`
	ServerAt       int64            `json:"serverAt"`
	NextTickAt     int64            `json:"nextTickAt"`
	TickLeftMS     int64            `json:"tickLeftMS"`
	UserTickLeftMS int64            `json:"userTickLeftMS"`
	Saving         bool             `json:"saving"`
	Journal        []journalEntry   `json:"journal"`
	Inputs         [][]inputReceipt `json:"inputs"`
}

type View struct {
	engine.View
	Sequence       uint64        `json:"sequence"`
	EngineSequence uint64        `json:"engineSequence"`
	ServerAt       int64         `json:"serverAt"`
	Players        int           `json:"players"`
	Paused         bool          `json:"paused"`
	Saving         bool          `json:"saving"`
	Snapshot       *engine.State `json:"snapshot,omitempty"`
	FromSequence   *uint64       `json:"fromSequence,omitempty"`
	Operations     []operation   `json:"operations,omitempty"`
	AcceptedInputs []string      `json:"acceptedInputs,omitempty"`
}

type streamUpdate struct {
	Epoch          int         `json:"epoch"`
	Sequence       uint64      `json:"sequence"`
	EngineSequence uint64      `json:"engineSequence"`
	Clock          int64       `json:"clock"`
	ServerAt       int64       `json:"serverAt"`
	Players        int         `json:"players"`
	Paused         bool        `json:"paused"`
	Saving         bool        `json:"saving"`
	Gold           int64       `json:"gold"`
	Health         int         `json:"health"`
	Wave           int         `json:"wave"`
	Route          int         `json:"route"`
	FromSequence   uint64      `json:"fromSequence"`
	Operations     []operation `json:"operations"`
}

func decodeObject(raw json.RawMessage, limit int) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > limit {
		return nil, errors.New("巡径守卫请求大小无效")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errors.New("巡径守卫请求必须为对象")
	}
	object := map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("巡径守卫请求字段无效")
		}
		if _, exists := object[key]; exists {
			return nil, errors.New("巡径守卫请求包含重复字段")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("巡径守卫请求包含多余内容")
	}
	return object, nil
}

func strict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("巡径守卫数据包含多余内容")
	}
	return nil
}

func ruleHash(version string) string {
	catalog := engine.CatalogData()
	if catalog == nil || catalog.Version == "" || catalog.Source.SourceSHA256 == "" {
		return ""
	}
	identity := fmt.Sprintf("pokepath-independent-go:%d:%d:%s:%s:%s", moduleSchema, engine.SchemaVersion, version, catalog.Version, catalog.Source.SourceSHA256)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players < 1 || players > 4 {
		return nil, errors.New("巡径守卫需要 1～4 人合作")
	}
	options := engine.Options{Starter: "charmander"}
	if len(raw) != 0 {
		if _, err := decodeObject(raw, 4096); err != nil {
			return nil, err
		}
		if err := strict(raw, &options); err != nil {
			return nil, errors.New("巡径守卫设置无效")
		}
	}
	if _, err := engine.New(options, players, 1, 0); err != nil {
		return nil, err
	}
	return json.Marshal(options)
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "pokepath", Name: "宝可梦：巡径守卫", MinPlayers: 1, MaxPlayers: 4,
		Controls:     controls(),
		SupportsSave: true, Realtime: true, Configure: configure, Start: start,
		Apply: apply, View: view, SyncSnapshot: view, StreamView: streamView,
		OnTimer: timer, BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: acceptStale, SaveControl: saveControl, ResumeSaved: resumeSaved,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	normalized, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var options engine.Options
	_ = json.Unmarshal(normalized, &options)
	seedBytes := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%d:%s", ctx.Now.UnixNano(), ctx.Players, ctx.Version, normalized)))
	seed := uint64(binary.LittleEndian.Uint32(seedBytes[:4])) | 1
	world, err := engine.New(options, ctx.Players, seed, ctx.Now.UnixMilli())
	if err != nil {
		return gameapi.Match{}, err
	}
	saved := state{Schema: moduleSchema, Hash: ruleHash(world.RulesVersion), Snapshot: world, ServerAt: ctx.Now.UnixMilli(), NextTickAt: ctx.Now.UnixMilli() + tickMS, Journal: []journalEntry{}, Inputs: make([][]inputReceipt, ctx.Players)}
	return saved.match()
}

func decode(match gameapi.Match) (state, error) {
	var saved state
	if match.Schema != moduleSchema || len(match.Data) == 0 || len(match.Data) > maxStateBytes || !json.Valid(match.Data) {
		return saved, errors.New("巡径守卫存档格式不兼容，原存档已保留")
	}
	if err := strict(match.Data, &saved); err != nil {
		return saved, err
	}
	if saved.Schema != moduleSchema || saved.Snapshot == nil || saved.Hash == "" || saved.Hash != ruleHash(saved.Snapshot.RulesVersion) {
		return saved, errors.New("巡径守卫规则或资源版本不兼容，原存档已保留")
	}
	if err := engine.Validate(saved.Snapshot); err != nil {
		return saved, err
	}
	if saved.NextTickAt == 0 {
		saved.NextTickAt = saved.ServerAt + tickMS
	}
	if saved.ServerAt < 0 || saved.ServerAt > maxSequence || saved.NextTickAt < 0 || saved.NextTickAt > maxSequence || saved.TickLeftMS < 0 || saved.TickLeftMS > tickMS || saved.UserTickLeftMS < 0 || saved.UserTickLeftMS > tickMS || saved.Sequence > maxSequence || len(saved.Journal) > journalLimit || len(saved.Inputs) != saved.Snapshot.Players {
		return saved, errors.New("巡径守卫平台快照无效")
	}
	for _, receipts := range saved.Inputs {
		if len(receipts) > receiptLimit {
			return saved, errors.New("巡径守卫输入确认超出上限")
		}
		for _, receipt := range receipts {
			if !validInputID(receipt.ID) || len(receipt.Digest) != 64 {
				return saved, errors.New("巡径守卫输入确认内容无效")
			}
		}
	}
	return saved, nil
}

func (saved state) paused() bool {
	return saved.Saving || saved.Snapshot.Paused
}

func (saved state) match() (gameapi.Match, error) {
	if saved.Hash == "" {
		return gameapi.Match{}, errors.New("巡径守卫规则数据尚未就绪")
	}
	if err := engine.Validate(saved.Snapshot); err != nil {
		return gameapi.Match{}, err
	}
	data, err := json.Marshal(saved)
	if err != nil || len(data) > maxStateBytes {
		return gameapi.Match{}, errors.New("巡径守卫状态过大或无效")
	}
	match := gameapi.Match{Schema: moduleSchema, Unrated: true, Data: data, Actors: []int{}, Attention: []int{}, TimeoutActors: []int{}, Paused: saved.paused()}
	for actor := 0; actor < saved.Snapshot.Players; actor++ {
		match.Actors = append(match.Actors, actor)
	}
	if !match.Paused {
		match.SystemDeadline = saved.NextTickAt
	}
	match.CheckpointKey = fmt.Sprintf("native:%d:%d:%d:%s:%t", saved.Snapshot.Epoch, saved.Snapshot.Route, saved.Snapshot.Wave, saved.Snapshot.Phase, saved.Snapshot.WaveActive)
	return match, nil
}

func validInputID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, character := range id {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validAction(ctx gameapi.Context, saved state, raw json.RawMessage) (engine.Action, string, error) {
	var action engine.Action
	if ctx.Players != saved.Snapshot.Players || ctx.Actor < 0 || ctx.Actor >= saved.Snapshot.Players || saved.Saving {
		return action, "", errors.New("当前席位不能操作巡径守卫")
	}
	object, err := decodeObject(raw, 65536)
	if err != nil {
		return action, "", err
	}
	id := ""
	if value, exists := object["clientInputID"]; exists {
		if json.Unmarshal(value, &id) != nil || !validInputID(id) {
			return action, "", errors.New("巡径守卫输入标识无效")
		}
		delete(object, "clientInputID")
	}
	native, err := json.Marshal(object)
	if err != nil || strict(native, &action) != nil {
		return action, "", errors.New("巡径守卫行动字段无效")
	}
	if action.Epoch != saved.Snapshot.Epoch || action.Type == "" {
		return action, "", errors.New("巡径守卫操作已经过期，请同步后重试")
	}
	if action.Type == "egg" && action.Quote <= 0 || action.Type == "buyItem" && action.ItemID == "" {
		return action, "", errors.New("请先确认商店当前的报价")
	}
	return action, id, nil
}

func actionDigest(action engine.Action) string {
	raw, _ := json.Marshal(action)
	var canonical any
	if json.Unmarshal(raw, &canonical) == nil {
		if encoded, err := json.Marshal(canonical); err == nil {
			raw = encoded
		}
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (saved state) inputReceipt(actor int, id string) *inputReceipt {
	for _, receipt := range saved.Inputs[actor] {
		if receipt.ID == id {
			copy := receipt
			return &copy
		}
	}
	return nil
}

func (saved *state) appendOperation(next operation) error {
	if saved.Sequence >= maxSequence {
		return errors.New("巡径守卫同步序号已达上限")
	}
	saved.Sequence++
	saved.Journal = append(saved.Journal, journalEntry{Sequence: saved.Sequence, Operation: next})
	if len(saved.Journal) > journalLimit {
		saved.Journal = saved.Journal[len(saved.Journal)-journalLimit:]
	}
	return nil
}

func (saved *state) advance(now int64) error {
	if saved.paused() || now <= saved.ServerAt {
		return nil
	}
	milliseconds := min(1000, now-saved.ServerAt)
	oldEpoch := saved.Snapshot.Epoch
	if err := engine.Advance(saved.Snapshot, milliseconds); err != nil {
		return err
	}
	saved.ServerAt = now
	if saved.Snapshot.Epoch != oldEpoch {
		saved.Journal = []journalEntry{}
		saved.Inputs = make([][]inputReceipt, saved.Snapshot.Players)
	}
	return saved.appendOperation(operation{Type: "tick", Milliseconds: milliseconds, Actor: -1})
}

func (saved state) remainingTick() int64 {
	if saved.Snapshot.Paused {
		return saved.UserTickLeftMS
	}
	if saved.Saving {
		return saved.TickLeftMS
	}
	return max(0, min(tickMS, saved.NextTickAt-saved.ServerAt))
}

func apply(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	action, id, err := validAction(ctx, saved, raw)
	if err != nil {
		return match, err
	}
	digest := ""
	if id != "" {
		digest = actionDigest(action)
		if receipt := saved.inputReceipt(ctx.Actor, id); receipt != nil {
			if receipt.Digest != digest {
				return match, errors.New("同一输入标识不能用于不同操作，请同步后重试")
			}
			return match, nil
		}
	}
	oldEpoch, oldPause := saved.Snapshot.Epoch, saved.Snapshot.Paused
	if err := saved.advance(ctx.Now.UnixMilli()); err != nil {
		return match, err
	}
	if err := engine.Apply(saved.Snapshot, ctx.Actor, action); err != nil {
		return match, err
	}
	if saved.Snapshot.Epoch != oldEpoch {
		saved.Journal = []journalEntry{}
		saved.Inputs = make([][]inputReceipt, saved.Snapshot.Players)
	}
	if saved.Snapshot.Paused != oldPause {
		saved.ServerAt = ctx.Now.UnixMilli()
		if saved.Snapshot.Paused {
			saved.UserTickLeftMS = max(0, min(tickMS, saved.NextTickAt-ctx.Now.UnixMilli()))
		} else {
			saved.NextTickAt = ctx.Now.UnixMilli() + saved.UserTickLeftMS
			saved.UserTickLeftMS = 0
		}
	}
	if err := saved.appendOperation(operation{Type: "action", Action: &action, Actor: ctx.Actor, ClientInputID: id}); err != nil {
		return match, err
	}
	if id != "" {
		saved.Inputs[ctx.Actor] = append(saved.Inputs[ctx.Actor], inputReceipt{ID: id, Digest: digest})
		if len(saved.Inputs[ctx.Actor]) > receiptLimit {
			saved.Inputs[ctx.Actor] = saved.Inputs[ctx.Actor][len(saved.Inputs[ctx.Actor])-receiptLimit:]
		}
	}
	return saved.match()
}

func timer(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Actor != -1 || ctx.Players != saved.Snapshot.Players {
		return match, errors.New("巡径守卫系统时钟身份无效")
	}
	if saved.paused() || ctx.Now.UnixMilli() < saved.NextTickAt {
		return match, nil
	}
	if err := saved.advance(ctx.Now.UnixMilli()); err != nil {
		return match, err
	}
	if saved.NextTickAt <= ctx.Now.UnixMilli() {
		saved.NextTickAt += ((ctx.Now.UnixMilli()-saved.NextTickAt)/tickMS + 1) * tickMS
	}
	return saved.match()
}

func project(saved state) View {
	native := engine.Project(saved.Snapshot)
	return View{View: native, Sequence: saved.Sequence, EngineSequence: native.Sequence, ServerAt: saved.ServerAt, Players: saved.Snapshot.Players, Paused: saved.paused(), Saving: saved.Saving}
}

func acceptedInputs(saved state, viewer int) []string {
	if viewer < 0 {
		return nil
	}
	ids := make([]string, 0, len(saved.Inputs[viewer]))
	for _, receipt := range saved.Inputs[viewer] {
		ids = append(ids, receipt.ID)
	}
	return ids
}

func view(match gameapi.Match, viewer int) (any, error) {
	saved, err := decode(match)
	if err != nil {
		return nil, err
	}
	if viewer < -1 || viewer >= saved.Snapshot.Players {
		return nil, errors.New("巡径守卫观看席位无效")
	}
	result := project(saved)
	result.Snapshot = saved.Snapshot
	result.AcceptedInputs = acceptedInputs(saved, viewer)
	return result, nil
}

func streamView(match gameapi.Match, viewer int, cursor string) (gameapi.StreamViewResult, error) {
	saved, err := decode(match)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	if viewer < -1 || viewer >= saved.Snapshot.Players {
		return gameapi.StreamViewResult{}, errors.New("巡径守卫观看席位无效")
	}
	parts := strings.Split(cursor, ":")
	valid := len(parts) == 2
	var epoch int
	var sequence uint64
	if valid {
		epoch, err = strconv.Atoi(parts[0])
		if err != nil {
			valid = false
		}
		sequence, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil || epoch != saved.Snapshot.Epoch || sequence > saved.Sequence {
			valid = false
		}
	}
	if valid && sequence < saved.Sequence && (len(saved.Journal) == 0 || sequence+1 < saved.Journal[0].Sequence) {
		valid = false
	}
	if valid {
		world := saved.Snapshot
		result := streamUpdate{Epoch: world.Epoch, Sequence: saved.Sequence, EngineSequence: world.Sequence, Clock: world.Clock, ServerAt: saved.ServerAt, Players: world.Players, Paused: saved.paused(), Saving: saved.Saving, Gold: world.Gold, Health: world.Health, Wave: world.Wave, Route: world.Route, FromSequence: sequence, Operations: []operation{}}
		for _, event := range saved.Journal {
			if event.Sequence > sequence {
				result.Operations = append(result.Operations, event.Operation)
			}
		}
		return gameapi.StreamViewResult{Data: result, Cursor: fmt.Sprintf("%d:%d", world.Epoch, saved.Sequence)}, nil
	}
	result := project(saved)
	result.Snapshot = saved.Snapshot
	result.AcceptedInputs = acceptedInputs(saved, viewer)
	return gameapi.StreamViewResult{Data: result, Cursor: fmt.Sprintf("%d:%d", saved.Snapshot.Epoch, saved.Sequence)}, nil
}

func acceptStale(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) bool {
	saved, err := decode(match)
	if err != nil {
		return false
	}
	action, id, err := validAction(ctx, saved, raw)
	if err != nil {
		return false
	}
	if id != "" {
		if receipt := saved.inputReceipt(ctx.Actor, id); receipt != nil {
			return receipt.Digest == actionDigest(action)
		}
	}
	return true
}

func botAction(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
	saved, err := decode(match)
	if err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= saved.Snapshot.Players || ctx.Players != saved.Snapshot.Players || saved.paused() {
		return nil, nil
	}
	if len(ctx.Automated) == ctx.Players {
		for actor, controlled := range ctx.Automated {
			if controlled {
				if actor != ctx.Actor {
					return nil, nil
				}
				break
			}
		}
	}
	var action *engine.Action
	phase := saved.Snapshot.Phase
	if phase == "route_complete" || phase == "practice_complete" || phase == "defeat" {
		if len(ctx.Automated) != ctx.Players {
			return nil, nil
		}
		for _, controlled := range ctx.Automated {
			if !controlled {
				return nil, nil
			}
		}
		switch phase {
		case "route_complete":
			action = &engine.Action{Type: "continueRoute"}
		case "practice_complete":
			action = &engine.Action{Type: "custom_retry"}
		case "defeat":
			action = engine.Propose(saved.Snapshot)
			if action == nil {
				action = &engine.Action{Type: "retry", Mode: "restart"}
			}
		}
	} else {
		action = engine.Propose(saved.Snapshot)
	}
	if action == nil {
		return nil, nil
	}
	action.Epoch = saved.Snapshot.Epoch
	return json.Marshal(action)
}

func bumpEpoch(saved *state) error {
	if saved.Snapshot.Epoch >= maxEpoch {
		return errors.New("巡径守卫操作代次已达上限，原存档已保留")
	}
	saved.Snapshot.Epoch++
	saved.Journal = []journalEntry{}
	saved.Inputs = make([][]inputReceipt, saved.Snapshot.Players)
	return nil
}

func reclaim(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Players != saved.Snapshot.Players || ctx.Actor < 0 || ctx.Actor >= ctx.Players {
		return match, errors.New("当前不能接管巡径守卫")
	}
	if err := bumpEpoch(&saved); err != nil {
		return match, err
	}
	return saved.match()
}

func saveControl(ctx gameapi.Context, match gameapi.Match, saving bool) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Players != saved.Snapshot.Players || ctx.Actor < 0 || ctx.Actor >= ctx.Players {
		return match, errors.New("原合作队伍身份不符")
	}
	if saved.Saving == saving {
		return match, nil
	}
	if saving {
		if err := saved.advance(ctx.Now.UnixMilli()); err != nil {
			return match, err
		}
		if saved.Snapshot.Paused {
			saved.TickLeftMS = saved.UserTickLeftMS
		} else {
			saved.TickLeftMS = max(0, min(tickMS, saved.NextTickAt-ctx.Now.UnixMilli()))
		}
	} else {
		saved.ServerAt = ctx.Now.UnixMilli()
		saved.NextTickAt = ctx.Now.UnixMilli() + saved.TickLeftMS
		saved.TickLeftMS = 0
	}
	saved.Saving = saving
	if err := bumpEpoch(&saved); err != nil {
		return match, err
	}
	return saved.match()
}

func resumeSaved(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Players != saved.Snapshot.Players {
		return match, errors.New("原合作队伍人数不符")
	}
	remaining := saved.remainingTick()
	saved.ServerAt = ctx.Now.UnixMilli()
	saved.NextTickAt = ctx.Now.UnixMilli() + remaining
	saved.Saving, saved.TickLeftMS = false, 0
	if err := bumpEpoch(&saved); err != nil {
		return match, err
	}
	return saved.match()
}

func restore(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
	saved, err := decode(match)
	if err != nil {
		return match, err
	}
	if ctx.Players != saved.Snapshot.Players {
		return match, errors.New("原合作队伍人数不符")
	}
	remaining := saved.remainingTick()
	saved.ServerAt = ctx.Now.UnixMilli()
	saved.NextTickAt = ctx.Now.UnixMilli() + remaining
	if err := bumpEpoch(&saved); err != nil {
		return match, err
	}
	return saved.match()
}
