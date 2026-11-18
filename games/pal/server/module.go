package pal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const tickMS = 50
const fieldTickMS = 100
const battleTickMS = 40
const maxInputAgeMS = 500

type PartyMember struct {
	HP         int    `json:"hp"`
	MaxHP      int    `json:"maxHP"`
	MP         int    `json:"mp"`
	MaxMP      int    `json:"maxMP"`
	Role       int    `json:"role"`
	Name       string `json:"name"`
	Controller int    `json:"controller"`
}
type Dialogue struct {
	ID       uint64 `json:"id"`
	Text     string `json:"text"`
	Face     int    `json:"face"`
	Position int    `json:"position"`
	Read     []bool `json:"read"`
}
type View struct {
	Local         *LocalView            `json:"local,omitempty"`
	Audio         *AudioView            `json:"audio,omitempty"`
	GameOver      *engine.GameOverState `json:"gameOver,omitempty"`
	Ending        *engine.EndingState   `json:"ending,omitempty"`
	Field         *engine.FieldState    `json:"field,omitempty"`
	PaletteEffect *engine.PaletteEffect `json:"paletteEffect"`
	SceneReady    bool                  `json:"sceneReady"`
	ClearColor    int                   `json:"clearColor"`
	Raster        *engine.RasterGraph   `json:"raster,omitempty"`
	Cooperation   *Cooperation          `json:"cooperation,omitempty"`
	Battle        *BattleView           `json:"battle,omitempty"`
	BattleAssets  [][2]int              `json:"battleAssets,omitempty"`
	Epoch         uint64                `json:"epoch"`
	Seq           uint64                `json:"seq"`
	ControlAt     int64                 `json:"controlAt,omitempty"`
	DiscardedSeq  uint64                `json:"discardedSeq,omitempty"`
	Phase         string                `json:"phase"`
	Leader        int                   `json:"leader"`
	Party         []PartyMember         `json:"party"`
	Dialogue      *Dialogue             `json:"dialogue,omitempty"`
	Scene         int                   `json:"scene"`
	X             int                   `json:"x"`
	Y             int                   `json:"y"`
	Map           int                   `json:"map"`
	CameraX       int                   `json:"cameraX"`
	CameraY       int                   `json:"cameraY"`
	Palette       int                   `json:"palette"`
	Night         bool                  `json:"night"`
	Draws         []engine.DrawOp       `json:"draws"`
	Overlay       []engine.UIOp         `json:"overlay"`
	Menu          engine.MenuView       `json:"menu"`
	AppliedSeq    uint64                `json:"appliedSeq"`
	HP            int                   `json:"hp"`
	MP            int                   `json:"mp"`
	Cash          uint32                `json:"cash"`
}

type Cooperation struct {
	ID            uint64 `json:"id"`
	Initiator     int    `json:"initiator"`
	Magic         int    `json:"magic"`
	Name          string `json:"name"`
	Target        int    `json:"target"`
	AffectedRoles []int  `json:"affectedRoles"`
	Contributors  []int  `json:"contributors"`
	HPCost        int    `json:"hpCost"`
	Required      []int  `json:"required"`
	Approved      []bool `json:"approved"`
}

func (p Cooperation) complete() bool {
	for _, seat := range p.Required {
		if !p.Approved[seat] {
			return false
		}
	}
	return true
}

type BattleView struct {
	Phase    int                   `json:"phase"`
	State    int                   `json:"state"`
	Menu     int                   `json:"menu"`
	Current  int                   `json:"current"`
	Selected int                   `json:"selected"`
	Action   int                   `json:"action"`
	Object   int                   `json:"object"`
	Target   int                   `json:"target"`
	Players  []engine.BattlePlayer `json:"players"`
	Enemies  []BattleEnemy         `json:"enemies"`
}

type BattleEnemy struct {
	Object     int `json:"object"`
	Frame      int `json:"frame"`
	X          int `json:"x"`
	Y          int `json:"y"`
	ColorShift int `json:"colorShift"`
}
type Action struct {
	Boundary  string `json:"boundary,omitempty"`
	ControlAt int64  `json:"controlAt,omitempty"`
	Type      string `json:"type"`
	Epoch     uint64 `json:"epoch"`
	Prompt    uint64 `json:"prompt,omitempty"`
	Seq       uint64 `json:"seq,omitempty"`
	Direction int    `json:"direction,omitempty"`
	Keys      uint32 `json:"keys,omitempty"`
	Target    int    `json:"target,omitempty"`
}
type control struct {
	DiscardedSeq uint64        `json:"discardedSeq,omitempty"`
	Seq          uint64        `json:"seq"`
	Direction    int           `json:"direction"`
	Keys         uint32        `json:"keys"`
	Until        int64         `json:"until"`
	AppliedSeq   uint64        `json:"appliedSeq"`
	Edges        []keyEdge     `json:"edges,omitempty"`
	Levels       []levelChange `json:"levels,omitempty"`
}
type keyEdge struct {
	Seq   uint64 `json:"seq"`
	Keys  uint32 `json:"keys"`
	Until int64  `json:"until"`
	At    int64  `json:"at,omitempty"`
}
type levelChange struct {
	Seq       uint64 `json:"seq"`
	Direction int    `json:"direction"`
	At        int64  `json:"at"`
	Until     int64  `json:"until"`
}
type state struct {
	Native      []byte           `json:"native"`
	Local       *localState      `json:"local,omitempty"`
	Audio       *audioMemory     `json:"audio,omitempty"`
	BattleBot   *battleBotMemory `json:"battleBot,omitempty"`
	FieldBot    *fieldBotMemory  `json:"fieldBot,omitempty"`
	Public      View             `json:"public"`
	Players     int              `json:"players"`
	Controls    []control        `json:"controls"`
	RemainingMS uint32           `json:"remainingMS"`
	ServerAt    int64            `json:"serverAt"`
	Saving      bool             `json:"saving"`
	TickLeftMS  int64            `json:"tickLeftMS"`
	Revision    uint64           `json:"revision"`
}
type campaignRuntime interface {
	StartCampaignContinuationForPlayers(context.Context, uint32, int) (*engine.Continuation, error)
	Continue([]byte) (*engine.Continuation, error)
}
type adapter struct{ runtime campaignRuntime }

func strict(raw []byte, out any, limit int) error {
	if len(raw) == 0 || len(raw) > limit || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效仙剑数据")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("仙剑数据包含多余内容")
	}
	return nil
}
func configure(players int, options json.RawMessage) (json.RawMessage, error) {
	if players != 1 {
		return nil, errors.New("仙剑当前仅支持单人")
	}
	if len(options) > 0 {
		if e := strict(options, &struct{}{}, 4096); e != nil {
			return nil, e
		}
	}
	return json.RawMessage(`{}`), nil
}
func NewModule(runtime *engine.Runtime) gameapi.Module {
	if runtime == nil {
		return newModule(nil)
	}
	return newModule(runtime)
}

func NewVersionedModule(runtime *engine.RuntimeCatalog) gameapi.Module {
	if runtime == nil {
		return newModule(nil)
	}
	return newModule(runtime)
}

func newModule(runtime campaignRuntime) gameapi.Module {
	a := adapter{runtime}
	return gameapi.Module{ID: "pal", Name: "仙剑奇侠传", MinPlayers: 1, MaxPlayers: 1, SupportsSave: true, SupportsCheckpoint: true, Realtime: true, RealtimeDeadlineFrames: true,
		Controls:  controls(),
		Configure: configure, Start: a.start, Apply: a.apply, View: a.view, SyncSnapshot: a.syncView, StreamView: a.streamView,
		OnTimer: a.timer, AcceptStale: a.acceptStale, SaveControl: a.saveControl, ResumeSaved: a.resume, RestoreCheckpoint: a.restoreCheckpoint,
		BotAction: a.botAction, Reclaim: a.reclaim, Restore: a.restore, IsInputRelease: a.isRelease}
}
func execution() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
func (a adapter) start(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
	if _, e := configure(c.Players, options); e != nil {
		return gameapi.Match{}, e
	}
	if a.runtime == nil {
		return gameapi.Match{}, errors.New("仙剑规则尚未初始化")
	}
	ctx, cancel := execution()
	defer cancel()
	hash := sha256.Sum256([]byte(fmt.Sprintf("pal/%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Players)))
	native, e := a.runtime.StartCampaignContinuationForPlayers(ctx, binary.LittleEndian.Uint32(hash[:4]), c.Players)
	if e != nil {
		return gameapi.Match{}, e
	}
	s := state{Players: c.Players, Controls: make([]control, c.Players), ServerAt: c.Now.UnixMilli()}
	s.Public.Epoch = binary.LittleEndian.Uint64(hash[8:16])&((1<<52)-1) + 1
	s.clearControls()
	if e = a.observe(&s, native, true, c.Now.UnixMilli(), nil); e != nil {
		return gameapi.Match{}, e
	}
	return s.match()
}
func decode(m gameapi.Match) (state, error) {
	var s state
	if m.Schema != 1 {
		return s, errors.New("不支持的仙剑存档版本")
	}
	decoded, e := decodeStateJSON(m.Data)
	if e != nil {
		return s, e
	}
	s = decoded
	if !validLocalState(s.Local) {
		return s, errors.New("仙剑本地同步状态无效")
	}
	if s.Players != 1 {
		return s, errors.New("仙剑当前仅支持单人存档")
	}
	if s.Players < 1 || s.Players > 3 || len(s.Controls) != s.Players || s.Public.Epoch == 0 || s.Public.Leader < 0 || s.Public.Leader >= s.Players || len(s.Native) == 0 || len(s.Native) > 32<<20 || s.ServerAt <= 0 || s.Saving != m.Paused || s.TickLeftMS < 0 || s.TickLeftMS > fieldTickMS {
		return s, errors.New("仙剑存档结构无效")
	}
	if s.Public.Phase != "field" && s.Public.Phase != "dialogue" && s.Public.Phase != "script" && s.Public.Phase != "battle" && s.Public.Phase != "menu" && s.Public.Phase != "system" && s.Public.Phase != "cooperation" && s.Public.Phase != "ending" && s.Public.Phase != "finished" && s.Public.Phase != "gameover" {
		return s, errors.New("仙剑阶段无效")
	}
	if g := s.Public.GameOver; g != nil && (g.Stage < 1 || g.Stage > 2 || g.Slot < 0 || g.Slot > 5 || s.Public.Ending != nil) || (s.Public.Phase == "gameover") != (s.Public.GameOver != nil && s.Public.GameOver.Stage == 2) {
		return s, errors.New("仙剑失败状态无效")
	}
	if (s.Public.Phase == "ending" || s.Public.Phase == "finished") != (s.Public.Ending != nil) || s.Public.Ending != nil && s.Public.Ending.Finished != (s.Public.Phase == "finished") {
		return s, errors.New("仙剑结局状态无效")
	}
	if (s.Public.Phase == "dialogue") != (s.Public.Dialogue != nil) || len(s.Public.Party) < 1 || len(s.Public.Party) > 3 {
		return s, errors.New("仙剑队伍或对话无效")
	}
	if d := s.Public.Dialogue; d != nil && (d.ID == 0 || len(d.Read) != s.Players) {
		return s, errors.New("仙剑阅读记录无效")
	}
	for _, p := range s.Public.Party {
		if p.Role < 0 || p.Role >= 6 || p.Controller < 0 || p.Controller >= s.Players {
			return s, errors.New("仙剑角色分工无效")
		}
	}
	if (s.Public.Phase == "cooperation") != (s.Public.Cooperation != nil) {
		return s, errors.New("仙剑合击确认阶段无效")
	}
	if p := s.Public.Cooperation; p != nil {
		if p.ID == 0 || s.Public.Battle == nil || p.Initiator < 0 || p.Initiator >= s.Players || p.Magic <= 0 || p.Name == "" || p.HPCost < 0 || p.HPCost > 65535 || len(p.Approved) != s.Players || len(p.AffectedRoles) != len(s.Public.Party) || len(p.Required) == 0 || len(p.Required) > s.Players {
			return s, errors.New("仙剑合击确认无效")
		}
		required := make([]int, 0, s.Players)
		for seat := 0; seat < s.Players; seat++ {
			affected := false
			for i, member := range s.Public.Party {
				if p.AffectedRoles[i] != member.Role {
					return s, errors.New("仙剑合击队员无效")
				}
				affected = affected || member.Controller == seat
			}
			if affected {
				required = append(required, seat)
			} else if p.Approved[seat] {
				return s, errors.New("仙剑合击确认者无效")
			}
		}
		if !slices.Equal(required, p.Required) || !p.Approved[p.Initiator] || !slices.Contains(required, p.Initiator) || p.complete() {
			return s, errors.New("仙剑合击确认记录无效")
		}
		for i, role := range p.Contributors {
			if !slices.Contains(p.AffectedRoles, role) || slices.Contains(p.Contributors[:i], role) {
				return s, errors.New("仙剑合击体力队员无效")
			}
		}
	}
	for _, c := range s.Controls {
		if c.Direction < 0 || c.Direction > 4 || c.Keys != 0 || c.Until < 0 || len(c.Edges) > 32 || len(c.Levels) > 65 || (len(c.Levels) == 65 && c.Levels[64].Direction != 4) || c.AppliedSeq > c.Seq || c.DiscardedSeq > c.Seq {
			return s, errors.New("仙剑输入状态无效")
		}
		previous := c.AppliedSeq
		for _, edge := range c.Edges {
			if edge.Seq <= previous || edge.Seq > c.Seq || edge.Keys == 0 || edge.Keys&^uint32(0x3ffff) != 0 || edge.At < 0 || edge.Until < edge.At {
				return s, errors.New("仙剑输入队列无效")
			}
			previous = edge.Seq
		}
		previous, at := c.AppliedSeq, int64(0)
		for _, level := range c.Levels {
			if level.Seq <= previous || level.Seq > c.Seq || level.Direction < 0 || level.Direction > 4 || level.At <= 0 || level.At < at || level.Until < level.At {
				return s, errors.New("仙剑持续输入时间无效")
			}
			previous, at = level.Seq, level.At
		}
	}
	if !validFieldBotMemory(s.FieldBot) {
		return s, errors.New("仙剑探索记录无效")
	}
	if !validBattleBotMemory(s.BattleBot) {
		return s, errors.New("仙剑战斗记录无效")
	}
	if !validAudio(s) {
		return s, errors.New("仙剑音频状态无效")
	}
	return s, nil
}
func (s state) tickInterval() int64 {
	if s.Public.Phase == "field" {
		return min(fieldTickMS, max(1, int64(s.RemainingMS)))
	}
	if s.Public.Phase == "battle" && s.Public.Battle != nil {
		return min(battleTickMS, max(1, int64(s.RemainingMS)))
	}
	return tickMS
}

func (s state) match() (gameapi.Match, error) {
	data, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	m := gameapi.Match{Schema: 1, Unrated: true, Data: data, Paused: s.Saving, Actors: []int{}, Attention: []int{}, TimeoutActors: []int{}, Log: []string{}, CheckpointKey: fmt.Sprint(s.Public.Scene, "/", s.Revision)}
	m.PreserveSavedCheckpoint = s.Public.GameOver != nil
	allDead := len(s.Public.Party) > 0
	for _, p := range s.Public.Party {
		allDead = allDead && p.HP == 0
	}
	m.PreserveSavedCheckpoint = m.PreserveSavedCheckpoint || allDead
	if s.Public.Phase == "gameover" {
		m.Restartable = true
		return m, nil
	}
	if s.Public.Phase == "finished" {
		m.Result = &gameapi.Result{Unrated: true, Ranks: []int{1}, Winners: []int{0}}
		return m, nil
	}
	if !s.Saving {
		m.SystemDeadline = s.ServerAt + s.tickInterval()
		if d := s.Public.Dialogue; d != nil {
			for i, read := range d.Read {
				if !read {
					m.Actors = append(m.Actors, i)
					m.Attention = append(m.Attention, i)
				}
			}
		} else if p := s.Public.Cooperation; p != nil {
			for _, seat := range p.Required {
				m.Actors = append(m.Actors, seat)
				if !p.Approved[seat] {
					m.Attention = append(m.Attention, seat)
				}
			}
		} else if actor := s.inputActor(); actor >= 0 {
			m.Actors = append(m.Actors, actor)
			if s.Public.Phase != "field" {
				m.Attention = append(m.Attention, actor)
			}
		} else if s.Public.Phase == "system" {
			m.Actors = append(m.Actors, s.Public.Leader)
			m.Attention = append(m.Attention, s.Public.Leader)
		}
		if len(m.Actors) == 0 && s.Local != nil {
			m.Actors = []int{0}
		}
	}
	return m, nil
}
func (s state) inputActor() int {
	if s.Public.Phase == "field" || s.Public.Phase == "menu" || s.Public.Phase == "ending" {
		return s.Public.Leader
	}
	if b := s.Public.Battle; s.Public.Phase == "battle" && b != nil && b.Phase == 0 && b.State != 0 && b.Current >= 0 && b.Current < len(s.Public.Party) {
		return s.Public.Party[b.Current].Controller
	}
	return -1
}
func (s *state) clearControls() {
	for i := range s.Controls {
		s.Controls[i] = control{Direction: 4}
	}
}
func (a adapter) observe(s *state, native *engine.Continuation, encode bool, now int64, schedule *audioSchedule) error {
	v := native.View()
	var e error
	old := s.Public
	phase := "script"
	if v.GameOver != nil && v.GameOver.Stage == 2 {
		phase = "gameover"
	} else if v.Ending != nil && v.Ending.Finished {
		phase = "finished"
	} else if v.Ending != nil {
		phase = "ending"
	} else if native.Event().Kind == engine.CooperativeAction {
		phase = "cooperation"
	} else if native.Event().Kind == engine.Dialogue {
		phase = "dialogue"
	} else if native.Event().Kind == engine.SystemMenu {
		phase = "system"
	} else if v.Mode == engine.MenuMode {
		phase = "menu"
	} else if v.Mode == engine.FieldMode && !v.Entering && native.Event().Kind == engine.Frame {
		phase = "field"
	} else if v.Mode == engine.BattleMode {
		phase = "battle"
	}
	s.Public = View{Audio: old.Audio, Epoch: old.Epoch, Leader: old.Leader, Phase: phase, Scene: v.Scene, X: v.X, Y: v.Y, Map: v.Map, CameraX: v.CameraX, CameraY: v.CameraY, Palette: v.Palette, Night: v.Night, Draws: v.Draws, Overlay: v.Overlay, Menu: v.Menu, HP: v.HP, MP: v.MP, Cash: v.Cash, Party: make([]PartyMember, len(v.Party))}
	base := now
	var audio []engine.TimedAudioCommand
	if schedule != nil {
		base, audio = schedule.StartedAt, schedule.Commands
	}
	if e = syncNativeAudio(s, v.Audio, base, audio); e != nil {
		return e
	}
	pruneAudio(s, now)
	s.Public.GameOver = v.GameOver
	s.Public.Ending = v.Ending
	s.Public.Raster = v.Raster
	s.Public.SceneReady = v.SceneReady
	s.Public.ClearColor = v.ClearColor
	s.Public.PaletteEffect = v.PaletteEffect
	if phase == "field" {
		s.Public.Field = v.Field
	}
	if v.Raster != nil {
		s.Public.Overlay = nil
		s.Public.Draws = nil
	}
	if v.Battle.Active {
		b := v.Battle
		s.Public.BattleAssets = v.BattleAssets
		s.Public.Battle = &BattleView{Phase: b.Phase, State: b.State, Menu: b.Menu, Current: b.Current, Selected: b.Selected, Action: b.Action, Object: b.Object, Target: b.Target, Players: b.Players, Enemies: make([]BattleEnemy, len(b.Enemies))}
		for i, e := range b.Enemies {
			s.Public.Battle.Enemies[i] = BattleEnemy{e.Object, e.Frame, e.X, e.Y, e.ColorShift}
		}
	}
	boundary := old.Phase != phase || old.Menu.Kind != v.Menu.Kind
	if x, y := old.Battle, s.Public.Battle; x != nil && y != nil {
		boundary = boundary || x.Current != y.Current || x.State != y.State || x.Menu != y.Menu || x.Phase != y.Phase
	}
	if boundary {
		s.Public.Epoch++
		s.clearControls()
		ctx, cancel := execution()
		e = native.ReleaseInputs(ctx)
		cancel()
		if e != nil {
			return e
		}
	}
	owners := partyControllers(v.Party, old.Party, s.Players)
	for i, role := range v.Party {
		s.Public.Party[i] = PartyMember{Role: role, Name: v.PartyNames[i], Controller: owners[i]}
		stats := v.Battle.Players[i]
		s.Public.Party[i].HP, s.Public.Party[i].MaxHP, s.Public.Party[i].MP, s.Public.Party[i].MaxMP = stats.HP, stats.MaxHP, stats.MP, stats.MaxMP
	}
	if phase == "dialogue" {
		s.Revision++
		s.Public.Dialogue = &Dialogue{ID: s.Revision, Text: v.Text, Face: v.Face, Position: v.DialogPosition, Read: make([]bool, s.Players)}
		s.clearControls()
	}
	if phase == "cooperation" {
		request := v.Cooperation
		s.Revision++
		p := &Cooperation{ID: s.Revision, Initiator: s.Public.Party[request.Player].Controller, Magic: request.Magic, Name: request.Name, Target: request.Target, HPCost: request.HPCost, Approved: make([]bool, s.Players), AffectedRoles: make([]int, 0, len(v.Party)), Contributors: make([]int, 0, len(v.Party)), Required: make([]int, 0, s.Players)}
		for slot, role := range v.Party {
			if request.AffectedMask&(1<<slot) != 0 {
				p.AffectedRoles = append(p.AffectedRoles, role)
			}
			if request.ContributorMask&(1<<slot) != 0 {
				p.Contributors = append(p.Contributors, role)
			}
		}
		for seat := 0; seat < s.Players; seat++ {
			for _, member := range s.Public.Party {
				if member.Controller == seat && slices.Contains(p.AffectedRoles, member.Role) {
					p.Required = append(p.Required, seat)
					break
				}
			}
		}
		p.Approved[p.Initiator] = true
		s.Public.Cooperation = p
		if p.complete() {
			ctx, cancel := execution()
			defer cancel()
			if e = native.Step(ctx, 1); e != nil {
				return e
			}
			return a.observe(s, native, encode, now, nil)
		}
	}
	rememberFieldConversation(s.FieldBot, old, s.Public)
	if s.Public.Battle == nil {
		s.BattleBot = nil
	} else {
		observeBattleBot(s.BattleBot, s.Public)
	}
	s.RemainingMS = 0
	if native.Event().Kind == engine.Frame || native.Event().Kind == engine.Delay {
		s.RemainingMS = uint32(native.Event().Value)
	}
	if encode {
		s.Native, e = native.Encode()
		if e == nil {
			s.recordReplay(native)
		}
	}
	return e
}
func validAction(c gameapi.Context, s state, raw json.RawMessage) (Action, error) {
	var action Action
	if c.Players != s.Players || c.Actor < 0 || c.Actor >= s.Players {
		return action, errors.New("只有原队友可以操作")
	}
	if e := strict(raw, &action, 4096); e != nil {
		return action, e
	}
	if action.Type == "local_input" || action.Type == "local_release" {
		return action, validLocalAction(c, s, action)
	}
	if s.Saving || action.Epoch != s.Public.Epoch || action.Boundary != "" {
		return action, errors.New("游戏已暂停或操作已过期")
	}
	switch action.Type {
	case "coop_accept", "coop_cancel":
		if p := s.Public.Cooperation; p == nil || action.Prompt != p.ID || !slices.Contains(p.Required, c.Actor) || (action.Type == "coop_accept" && p.Approved[c.Actor]) || action.Seq != 0 || action.Direction != 0 || action.Keys != 0 || action.Target != 0 || action.ControlAt != 0 {
			return action, errors.New("当前合击请求已更新或无法确认")
		}
	case "read":
		if d := s.Public.Dialogue; d == nil || action.Prompt != d.ID || d.Read[c.Actor] || action.Seq != 0 || action.Direction != 0 || action.Keys != 0 || action.Target != 0 || action.ControlAt != 0 {
			return action, errors.New("当前对话已更新或已确认")
		}
	case "input":
		if c.Actor != s.inputActor() || action.Prompt != 0 || action.Target != 0 || action.Direction < 0 || action.Direction > 4 || action.Keys&^uint32(0x3ffff) != 0 || action.Seq <= s.Controls[c.Actor].Seq || action.Seq-s.Controls[c.Actor].Seq > 1024 || (action.Keys != 0 && len(s.Controls[c.Actor].Edges) >= 32) || (action.Direction != 4 && len(s.Controls[c.Actor].Levels) >= 64) {
			return action, errors.New("当前不能控制队伍")
		}
	case "system_close":
		if c.Actor != s.Public.Leader || s.Public.Phase != "system" || action.Prompt != 0 || action.Seq != 0 || action.Direction != 0 || action.Keys != 0 || action.Target != 0 || action.ControlAt != 0 {
			return action, errors.New("当前不能关闭系统操作")
		}
	case "leader":
		if s.Public.Phase != "field" || c.Actor != s.Public.Leader || action.Target < 0 || action.Target >= s.Players || action.Target == c.Actor || action.Prompt != 0 || action.Seq != 0 || action.Keys != 0 || action.Direction != 0 || action.ControlAt != 0 {
			return action, errors.New("只有当前领队可以交接队伍")
		}
	default:
		return action, errors.New("未知仙剑操作")
	}
	return action, nil
}
func (a adapter) apply(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	action, e := validAction(c, s, raw)
	if e != nil {
		return m, e
	}
	if s.Local != nil && s.Local.Seq > 0 && action.Type != "local_input" && action.Type != "local_release" && len(c.Automated) == s.Players && c.Automated[c.Actor] {
		if e = a.release(&s); e != nil {
			return m, e
		}
	}
	pruneAudio(&s, c.Now.UnixMilli())
	switch action.Type {
	case "local_input", "local_release":
		if e = a.queueLocal(c, &s, action); e != nil {
			return m, e
		}
	case "coop_accept", "coop_cancel":
		s.Revision++
		response := uint32(2)
		if action.Type == "coop_accept" {
			s.Public.Cooperation.Approved[c.Actor] = true
			response = 0
			if s.Public.Cooperation.complete() {
				response = 1
			}
		}
		if response != 0 {
			ctx, cancel := execution()
			defer cancel()
			native, e := a.runtime.Continue(s.Native)
			if e != nil {
				return m, e
			}
			if e = native.Step(ctx, response); e != nil {
				return m, e
			}
			if e = a.observe(&s, native, true, c.Now.UnixMilli(), nil); e != nil {
				return m, e
			}
		}
		s.ServerAt = c.Now.UnixMilli()
	case "input":
		ctrl := &s.Controls[c.Actor]
		at := max(c.Now.UnixMilli(), s.ServerAt)
		arrived := at
		if !c.InputReceivedAt.IsZero() {
			arrived = c.InputReceivedAt.UnixMilli()
		}
		if (action.Direction != 4 || action.Keys != 0) && (action.ControlAt <= 0 || action.ControlAt > arrived || action.ControlAt < arrived-maxInputAgeMS || arrived > at || arrived < at-maxInputAgeMS) {
			ctrl.Seq, ctrl.DiscardedSeq = action.Seq, action.Seq
			return s.match()
		}
		if len(ctrl.Levels) > 0 {
			at = max(at, ctrl.Levels[len(ctrl.Levels)-1].At)
		}
		if s.Public.Phase == "battle" && len(c.Automated) == s.Players && c.Automated[c.Actor] {
			if s.BattleBot == nil {
				s.BattleBot = &battleBotMemory{}
			}
			rememberBattleBotInput(s.BattleBot, s.Public, action.Keys)
		}
		if (s.Public.Phase == "field" || s.Public.Phase == "menu") && len(c.Automated) == s.Players && c.Automated[c.Actor] {
			if s.FieldBot == nil {
				s.FieldBot = &fieldBotMemory{}
			}
			rememberBotItemInput(s.FieldBot, s.Public, action.Keys)
		}
		if action.Keys&2 != 0 && s.Public.Phase == "field" && len(c.Automated) == s.Players && c.Automated[c.Actor] {
			rememberBotSearch(s.FieldBot, s.Public)
		}
		ctrl.Seq = action.Seq
		level := levelChange{action.Seq, action.Direction, at, at + 300}
		if len(ctrl.Levels) == 65 {
			ctrl.Levels[64].Seq, ctrl.Levels[64].Until = action.Seq, level.Until
		} else {
			ctrl.Levels = append(ctrl.Levels, level)
		}
		if action.Keys != 0 {
			ctrl.Edges = append(ctrl.Edges, keyEdge{Seq: action.Seq, Keys: action.Keys, At: at, Until: at + 300})
		}
	case "system_close":
		ctx, cancel := execution()
		defer cancel()
		native, e := a.runtime.Continue(s.Native)
		if e != nil {
			return m, e
		}
		if e = native.Step(ctx, 1); e != nil {
			return m, e
		}
		if e = a.observe(&s, native, true, c.Now.UnixMilli(), nil); e != nil {
			return m, e
		}
		s.Revision++
		s.ServerAt = c.Now.UnixMilli()
	case "leader":
		s.Revision++
		s.Public.Leader = action.Target
		if e = a.release(&s); e != nil {
			return m, e
		}
	case "read":
		if len(c.Automated) == s.Players && c.Automated[c.Actor] {
			if s.FieldBot == nil {
				s.FieldBot = &fieldBotMemory{}
			}
			rememberBotWine(s.FieldBot, s.Public)
		}
		s.Revision++
		s.Public.Dialogue.Read[c.Actor] = true
		if !slices.Contains(s.Public.Dialogue.Read, false) {
			ctx, cancel := execution()
			defer cancel()
			native, e := a.runtime.Continue(s.Native)
			if e != nil {
				return m, e
			}
			if e = native.ReleaseInputs(ctx); e != nil {
				return m, e
			}
			if e = native.Step(ctx, 1); e != nil {
				return m, e
			}
			if e = a.observe(&s, native, true, c.Now.UnixMilli(), nil); e != nil {
				return m, e
			}
			s.ServerAt = c.Now.UnixMilli()
		}
	}
	return s.match()
}
func (a adapter) timer(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	if m.Result != nil {
		return m, nil
	}
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Saving {
		return m, nil
	}
	pruneAudio(&s, c.Now.UnixMilli())
	if e = a.expireLocal(&s, c.Now.UnixMilli()); e != nil {
		return m, e
	}
	if e = a.drainLocal(&s, c.Now.UnixMilli()); e != nil {
		return m, e
	}
	if s.Public.Phase == "gameover" {
		return m, nil
	}
	if s.Public.Dialogue != nil || s.Public.Phase == "system" || s.Public.Cooperation != nil {
		s.ServerAt = c.Now.UnixMilli()
		return s.match()
	}
	if c.Players != s.Players {
		return m, errors.New("原队伍人数不符")
	}
	elapsed := c.Now.UnixMilli() - s.ServerAt
	if elapsed < s.tickInterval() {
		return s.match()
	}
	if elapsed > 250 {
		elapsed = 250
	}
	ctx, cancel := execution()
	defer cancel()
	native, e := a.runtime.Continue(s.Native)
	if e != nil {
		return m, e
	}
	actor := s.inputActor()
	var neutral control
	ctrl := &neutral
	if actor >= 0 {
		ctrl = &s.Controls[actor]
	}
	ctrl.Edges = slices.DeleteFunc(ctrl.Edges, func(edge keyEdge) bool { return edge.Until < c.Now.UnixMilli() })
	start := c.Now.UnixMilli() - elapsed
	advanced, e := native.AdvanceScheduled(ctx, s.RemainingMS, uint32(elapsed), func(offset uint32) engine.Input {
		if s.Local != nil && s.Local.Seq > 0 {
			return s.localInput(native.InputBoundary(), start+int64(offset))
		}
		at := start + int64(offset)
		for len(ctrl.Levels) > 0 && ctrl.Levels[0].At <= at {
			level := ctrl.Levels[0]
			ctrl.Direction, ctrl.Until = level.Direction, level.Until
			ctrl.Levels = ctrl.Levels[1:]
		}
		input := engine.Input{Direction: 4}
		if ctrl.Until >= at {
			input.Direction = ctrl.Direction
		}
		if len(ctrl.Edges) > 0 && ctrl.Edges[0].At <= at {
			input.Keys = ctrl.Edges[0].Keys
			ctrl.Edges = ctrl.Edges[1:]
		}
		ctrl.AppliedSeq = ctrl.Seq
		if len(ctrl.Levels) > 0 {
			ctrl.AppliedSeq = min(ctrl.AppliedSeq, ctrl.Levels[0].Seq-1)
		}
		if len(ctrl.Edges) > 0 {
			ctrl.AppliedSeq = min(ctrl.AppliedSeq, ctrl.Edges[0].Seq-1)
		}
		return input
	})
	if e != nil {
		return m, e
	}
	if advanced.Steps > 0 {
		epoch := s.Public.Epoch
		if e = a.observe(&s, native, true, c.Now.UnixMilli(), &audioSchedule{StartedAt: start, Commands: advanced.Audio}); e != nil {
			return m, e
		}
		if advanced.InputReset && s.Public.Epoch == epoch {
			s.Public.Epoch++
			s.clearControls()
		}
		if advanced.InputReset && s.Local != nil {
			s.Local.Direction, s.Local.Until = 4, 0
		}
	}
	s.RemainingMS = advanced.RemainingMS
	s.ServerAt = c.Now.UnixMilli()
	if e = a.drainLocal(&s, c.Now.UnixMilli()); e != nil {
		return m, e
	}
	return s.match()
}
func (a adapter) view(m gameapi.Match, viewer int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	return s.publicView(viewer), nil
}
func (s state) publicView(viewer int) View {
	v := s.Public
	v.Audio = cloneAudio(s.Public.Audio)
	v.ControlAt = s.ServerAt
	if viewer >= 0 && viewer < s.Players {
		v.Local = s.localView()
		v.Seq = s.Controls[viewer].Seq
		v.AppliedSeq = s.Controls[viewer].AppliedSeq
		v.DiscardedSeq = s.Controls[viewer].DiscardedSeq
		if s.Local != nil && s.Local.Seq > 0 {
			v.Seq, v.AppliedSeq, v.DiscardedSeq = s.Local.Seq, s.Local.AppliedSeq, s.Local.DiscardedSeq
		}
	}
	return v
}

func (a adapter) acceptStale(c gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	_, e = validAction(c, s, raw)
	return e == nil
}
func (a adapter) isRelease(c gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	action, e := validAction(c, s, raw)
	return e == nil && (action.Type == "input" || action.Type == "local_input" || action.Type == "local_release") && action.Direction == 4 && action.Keys == 0
}
func (a adapter) release(s *state) error {
	ctx, cancel := execution()
	defer cancel()
	native, e := a.runtime.Continue(s.Native)
	if e != nil {
		return e
	}
	if e = native.ReleaseInputs(ctx); e != nil {
		return e
	}
	s.Native, e = native.Encode()
	if e != nil {
		return e
	}
	s.Public.Epoch++
	s.clearControls()
	s.resetLocal(native)
	releaseBotItemInput(s.FieldBot)
	resetBattleBotInput(s.BattleBot)
	return nil
}
func (a adapter) saveControl(c gameapi.Context, m gameapi.Match, saving bool) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Players != s.Players {
		return m, errors.New("原队伍人数不符")
	}
	if s.Saving == saving {
		return m, nil
	}
	if e = a.release(&s); e != nil {
		return m, e
	}
	if saving {
		s.TickLeftMS = max(0, min(s.tickInterval(), s.ServerAt+s.tickInterval()-c.Now.UnixMilli()))
	} else {
		s.ServerAt = c.Now.UnixMilli() + s.TickLeftMS - s.tickInterval()
		s.TickLeftMS = 0
	}
	s.Saving = saving
	return s.match()
}
func (a adapter) resume(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Players != s.Players {
		return m, errors.New("原队伍人数不符")
	}
	if e = a.release(&s); e != nil {
		if errors.Is(e, engine.ErrRuntimeUnavailable) {
			return m, errors.New("此存档所需的游戏版本暂未加载，原存档已保留")
		}
		return m, e
	}
	if s.Saving {
		s.ServerAt = c.Now.UnixMilli() + s.TickLeftMS - s.tickInterval()
	} else {
		s.ServerAt = c.Now.UnixMilli()
	}
	s.Saving = false
	s.TickLeftMS = 0
	return s.match()
}
func (a adapter) restoreCheckpoint(c gameapi.Context, saved, active gameapi.Match) (gameapi.Match, error) {
	current, err := decode(active)
	if err != nil {
		return saved, err
	}
	if current.Public.Epoch >= (1<<53)-1 {
		return saved, errors.New("游戏操作代次超出可恢复范围，原存档已保留")
	}
	restored, err := a.resume(c, saved)
	if err != nil {
		return saved, err
	}
	next, err := decode(restored)
	if err != nil {
		return saved, err
	}
	next.Public.Epoch = max(next.Public.Epoch, current.Public.Epoch+1)
	if next.Local != nil && current.Local != nil {
		next.Local.Generation = max(next.Local.Generation, current.Local.Generation+1)
	}
	return next.match()
}
func (a adapter) reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor < 0 || c.Actor >= s.Players || c.Players != s.Players {
		return m, errors.New("只有原队友可以接管")
	}
	clearBattleBotOwner(s.BattleBot, s.Public, c.Actor)
	if owner := s.inputActor(); owner >= 0 && owner != c.Actor {
		s.Controls[c.Actor] = control{Direction: 4}
		return s.match()
	}
	if e = a.release(&s); e != nil {
		return m, e
	}
	return s.match()
}
func (a adapter) restore(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Players != s.Players {
		return m, errors.New("原队伍人数不符")
	}
	if e = a.release(&s); e != nil {
		return m, e
	}
	s.ServerAt = c.Now.UnixMilli()
	return s.match()
}
func (a adapter) botAction(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if c.Actor < 0 || c.Actor >= s.Players || s.Saving {
		return nil, nil
	}
	v := s.publicView(c.Actor)
	if d := v.Dialogue; d != nil && !d.Read[c.Actor] {
		return json.Marshal(Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
	}
	if p := v.Cooperation; p != nil && slices.Contains(p.Required, c.Actor) && !p.Approved[c.Actor] {
		kind := "coop_accept"
		for _, role := range p.Contributors {
			for _, player := range v.Battle.Players {
				if player.Role == role && player.HP <= p.HPCost*2 {
					kind = "coop_cancel"
				}
			}
		}
		return json.Marshal(Action{Type: kind, Epoch: v.Epoch, Prompt: p.ID})
	}
	if action, ok := battleBotAction(v, c.Actor, s.BattleBot); ok {
		return json.Marshal(action)
	}
	if action, ok := fieldBotAction(v, s.FieldBot, c.Actor); ok {
		return json.Marshal(action)
	}
	return nil, nil
}
