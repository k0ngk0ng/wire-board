// Package contra adapts the independent recovered rules to authenticated rooms.
// The module is deliberately not registered in the production game catalog yet.
package contra

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"strconv"
	"strings"
)

const frameMS int64 = 50
const leaseMS int64 = 750
const maxSequence int64 = 1<<53 - 1
const replaySize = 120
const maxQueue = 64

type Config struct {
	RulesBuildID   string
	RuntimeBuildID string
	Visuals        *BotVisualResources
	ObserveBot     BotObserver
}
type InputState struct {
	Sequence   int64            `json:"sequence"`
	Held       engine.Buttons   `json:"held"`
	Automated  bool             `json:"automated"`
	Applied    engine.Buttons   `json:"applied"`
	ReceivedAt int64            `json:"receivedAt"`
	Queue      []engine.Buttons `json:"queue,omitempty"`
	LastAction string           `json:"lastAction,omitempty"`
}
type InputView struct {
	Sequence  int64          `json:"sequence"`
	Held      engine.Buttons `json:"held"`
	Automated bool           `json:"automated"`
}
type FrameRecord struct {
	Kind     string            `json:"kind"`
	Revision uint64            `json:"revision,string"`
	Frame    uint64            `json:"frame,string"`
	Pads     [2]engine.Buttons `json:"pads"`
	Digest   string            `json:"digest"`
	Level    uint8             `json:"level"`
}
type StageLoading struct {
	RuntimeBuildID string `json:"runtimeBuildID,omitempty"`
	Level          uint8  `json:"level"`
	Ready          []bool `json:"ready"`
}
type State struct {
	VisualsID    string          `json:"visualsID"`
	Version      int             `json:"version"`
	Epoch        string          `json:"epoch"`
	RulesBuildID string          `json:"rulesBuildID"`
	BundleID     string          `json:"bundleID"`
	Players      int             `json:"players"`
	Session      *engine.Session `json:"session"`
	Inputs       []InputState    `json:"inputs"`
	Bots         [2]engine.Bot   `json:"bots"`
	ServerAt     int64           `json:"serverAt"`
	Remainder    int64           `json:"remainder"`
	Frames       []FrameRecord   `json:"frames"`
	StageLoading *StageLoading   `json:"stageLoading,omitempty"`
}
type View struct {
	RuntimeBuildID string                       `json:"runtimeBuildID,omitempty"`
	VisualsID      string                       `json:"visualsID"`
	Protocol       int                          `json:"protocol"`
	Epoch          string                       `json:"epoch"`
	Actor          int                          `json:"actor"`
	SeatActors     []int                        `json:"seatActors"`
	Inputs         []InputView                  `json:"inputs"`
	Snapshot       runtime.Snapshot             `json:"snapshot"`
	Revision       uint64                       `json:"revision,string"`
	Frames         []FrameRecord                `json:"frames"`
	Stage          engine.StageResourceIdentity `json:"stage"`
	Bundle         engine.LevelBundleManifest   `json:"bundle"`
	RulesBuildID   string                       `json:"rulesBuildID"`
	StageLoading   *StageLoading                `json:"stageLoading,omitempty"`
}
type SyncView struct {
	View       View               `json:"view"`
	Checkpoint runtime.Checkpoint `json:"checkpoint"`
}
type host struct {
	runtimeBuild string
	visuals      BotVisualResources
	observeBot   BotObserver
	build        string
	bundle       *engine.LevelBundle
}

func validSHA(v string) bool {
	b, e := hex.DecodeString(v)
	return e == nil && len(b) == 32 && v == strings.ToLower(v)
}
func strict(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players < 1 || players > 2 {
		return nil, errors.New("魂斗罗只支持一人或双人合作")
	}
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	var o *struct{}
	if e := strict(raw, &o); e != nil || o == nil {
		return nil, errors.New("魂斗罗使用原版标准战役设置")
	}
	return json.RawMessage(`{}`), nil
}
func ModuleWithConfig(c Config) (gameapi.Module, error) {
	if !validSHA(c.RulesBuildID) {
		return gameapi.Module{}, errors.New("missing canonical Contra rule-build SHA-256")
	}
	if c.RuntimeBuildID != "" && !validSHA(c.RuntimeBuildID) {
		return gameapi.Module{}, errors.New("invalid Contra runtime asset identity")
	}
	if c.Visuals == nil || !validSHA(c.Visuals.ID()) || c.Visuals.catalog == nil || c.ObserveBot == nil {
		return gameapi.Module{}, errors.New("missing verified visual resources or pure bot observer")
	}
	bundle, e := engine.BuiltinLevelBundle()
	if e != nil {
		return gameapi.Module{}, e
	}
	h := &host{build: c.RulesBuildID, runtimeBuild: c.RuntimeBuildID, bundle: bundle, visuals: *c.Visuals, observeBot: c.ObserveBot}
	m := gameapi.Module{ID: "contra", Name: "魂斗罗", MinPlayers: 1, MaxPlayers: 2, RatingPolicy: gameapi.RatingCoopCampaignV1, Realtime: true, Configure: configure, Start: h.start, Apply: h.apply, View: h.view, IsInputRelease: h.isInputRelease, SyncSnapshot: h.sync, BotAction: func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }, Reclaim: h.reclaim, Restore: h.restore, AcceptStale: h.acceptStale, OnTimer: h.timer, OnPausedTimer: h.pausedTimer}
	return m, m.Validate()
}
func flags(c gameapi.Context, n int) ([]bool, error) {
	if c.Players != n || len(c.Automated) != 0 && len(c.Automated) != n {
		return nil, errors.New("invalid authenticated seat metadata")
	}
	out := make([]bool, n)
	copy(out, c.Automated)
	return out, nil
}
func (h *host) start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	if _, e := configure(c.Players, raw); e != nil {
		return gameapi.Match{}, e
	}
	a, e := flags(c, c.Players)
	if e != nil {
		return gameapi.Match{}, e
	}
	var nonce [24]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return gameapi.Match{}, e
	}
	epoch := hex.EncodeToString(nonce[8:])
	session, e := engine.NewSession(binary.LittleEndian.Uint64(nonce[:8]), c.Players, epoch)
	if e != nil {
		return gameapi.Match{}, e
	}
	s := State{VisualsID: h.visuals.ID(), Version: 1, Epoch: epoch, RulesBuildID: h.build, BundleID: h.bundle.Manifest().ID, Players: c.Players, Session: session, Inputs: make([]InputState, c.Players), ServerAt: c.Now.UnixMilli(), Frames: []FrameRecord{}}
	for i := range a {
		s.Inputs[i].Automated = a[i]
	}
	g := gameapi.Match{Schema: 1, Actors: seats(c.Players), Attention: []int{}, TimeoutActors: []int{}, SystemDeadline: c.Now.UnixMilli() + frameMS, CheckpointKey: epoch + ":start"}
	return store(g, &s)
}
func seats(n int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = i
	}
	return a
}
func store(g gameapi.Match, s *State) (gameapi.Match, error) {
	b, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	g.Data = b
	return g, nil
}
func (h *host) decode(g gameapi.Match) (State, error) {
	var s State
	if e := strict(g.Data, &s); e != nil {
		return s, e
	}
	if g.Schema != 1 || s.Version != 1 || s.VisualsID != h.visuals.ID() || s.Players < 1 || s.Players > 2 || len(s.Inputs) != s.Players || s.Epoch == "" || s.RulesBuildID != h.build || s.BundleID != h.bundle.Manifest().ID || s.ServerAt < 0 || s.Remainder < 0 || s.Remainder >= 1000 || s.Session == nil {
		return s, errors.New("incompatible Contra savepoint")
	}
	if e := s.Session.Validate(); e != nil {
		return s, e
	}
	if s.Session.Audio.Epoch != s.Epoch || int(s.Session.World.Active.Combat.World.PlayerMode)+1 != s.Players || g.Paused != (s.Session.World.Campaign.Paused && !s.Session.ResumeRequested) || g.Result != nil && !s.Session.Finalized {
		return s, errors.New("inconsistent Contra session envelope")
	}
	for _, i := range s.Inputs {
		if i.Sequence < 0 || i.Sequence > maxSequence || i.ReceivedAt < 0 || len(i.Queue) > maxQueue {
			return s, errors.New("invalid saved input")
		}
	}
	if len(s.Frames) > replaySize {
		return s, errors.New("oversized replay ring")
	}
	for i, f := range s.Frames {
		if !validSHA(f.Digest) || f.Revision == 0 || f.Revision > s.Session.Revision || f.Frame > s.Session.Random.Frames || f.Level > 8 {
			return s, errors.New("invalid replay entry")
		}
		switch f.Kind {
		case "pads", "present-paused", "resume", "terminal":
		default:
			return s, errors.New("unknown replay operation")
		}
		if i > 0 {
			previous := s.Frames[i-1]
			nextFrame := previous.Frame
			if f.Kind == "pads" {
				nextFrame++
			}
			if f.Revision != previous.Revision+1 || f.Frame != nextFrame {
				return s, errors.New("replay frame or revision gap")
			}
		}
	}
	if len(s.Frames) > 0 {
		head := s.Frames[len(s.Frames)-1]
		digest, err := runtime.StateDigest(s.Session, s.BundleID, h.build)
		if err != nil || head.Revision != s.Session.Revision || head.Frame != s.Session.Random.Frames || head.Digest != digest {
			return s, errors.New("replay head differs")
		}
	}
	if gate := s.StageLoading; gate != nil {
		if gate.RuntimeBuildID != "" && !validSHA(gate.RuntimeBuildID) {
			return s, errors.New("invalid stage runtime identity")
		}
		if gate.Level >= 8 || gate.Level != s.Session.World.Active.Combat.World.Level || len(gate.Ready) != s.Players || s.Session.World.Campaign.Outcome != engine.CampaignOngoing || g.Paused {
			return s, errors.New("invalid stage gate")
		}
	}
	return s, nil
}
func (h *host) view(g gameapi.Match, actor int) (any, error) {
	s, e := h.decode(g)
	if e != nil {
		return nil, e
	}
	if actor < -1 || actor >= s.Players {
		return nil, errors.New("invalid viewer")
	}
	snapshot, e := runtime.View(s.Session, s.BundleID, h.build)
	if e != nil {
		return nil, e
	}
	v := View{RuntimeBuildID: h.runtimeBuild, VisualsID: s.VisualsID, Protocol: 1, Epoch: s.Epoch, Actor: actor, SeatActors: seats(s.Players), Inputs: make([]InputView, s.Players), Snapshot: snapshot, Revision: s.Session.Revision, Frames: s.Frames, Stage: h.bundle.Manifest().Stages[min(7, snapshot.Level)], Bundle: h.bundle.Manifest(), RulesBuildID: h.build, StageLoading: s.StageLoading}
	for i, input := range s.Inputs {
		v.Inputs[i] = InputView{input.Sequence, input.Held, input.Automated}
	}
	return v, nil
}
func (h *host) sync(g gameapi.Match, actor int) (any, error) {
	v, e := h.view(g, actor)
	if e != nil {
		return nil, e
	}
	s, e := h.decode(g)
	if e != nil {
		return nil, e
	}
	cp, e := runtime.CheckpointOf(s.Session, s.BundleID, h.build)
	if e != nil {
		return nil, e
	}
	return SyncView{View: v.(View), Checkpoint: cp}, nil
}
func (h *host) record(s *State, kind string, pads [2]engine.Buttons) error {
	digest, e := runtime.StateDigest(s.Session, s.BundleID, h.build)
	if e != nil {
		return e
	}
	s.Frames = append(s.Frames, FrameRecord{kind, s.Session.Revision, s.Session.Random.Frames, pads, digest, s.Session.World.Active.Combat.World.Level})
	if len(s.Frames) > replaySize {
		s.Frames = append([]FrameRecord(nil), s.Frames[len(s.Frames)-replaySize:]...)
	}
	return nil
}
func critical(g *gameapi.Match, s *State, label string) {
	g.CheckpointKey = s.Epoch + ":" + label + ":" + strconv.FormatUint(s.Session.Revision, 10)
}
func clearInput(i *InputState) { i.Held = 0; i.Applied = 0; i.Queue = nil; i.ReceivedAt = 0 }
func (h *host) reclaim(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
	s, e := h.decode(g)
	if e != nil {
		return g, e
	}
	if _, e = flags(c, s.Players); e != nil || c.Actor < 0 || c.Actor >= s.Players {
		return g, errors.New("invalid reclaim seat")
	}
	clearInput(&s.Inputs[c.Actor])
	s.Inputs[c.Actor].Automated = false
	if s.StageLoading != nil {
		s.StageLoading.Ready[c.Actor] = false
	}
	return store(g, &s)
}
func (h *host) restore(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
	s, e := h.decode(g)
	if e != nil {
		return g, e
	}
	if _, e = flags(c, s.Players); e != nil {
		return g, e
	}
	if s.Session.Finalized {
		return g, nil
	}
	for i := range s.Inputs {
		clearInput(&s.Inputs[i])
	}
	if s.StageLoading != nil && s.StageLoading.RuntimeBuildID != h.runtimeBuild {
		auto, _ := flags(c, s.Players)
		s.StageLoading.RuntimeBuildID = h.runtimeBuild
		s.StageLoading.Ready = append([]bool(nil), auto...)
	}
	s.ServerAt = c.Now.UnixMilli()
	s.Remainder = 0
	g.SystemDeadline = s.ServerAt + frameMS
	return store(g, &s)
}
func (h *host) finish(g *gameapi.Match, s *State) error {
	if s.Session.World.Campaign.Outcome == engine.CampaignOngoing {
		return nil
	}
	if !s.Session.Finalized {
		if _, e := s.Session.FlushTerminal(); e != nil {
			return e
		}
		if e := h.record(s, "terminal", [2]engine.Buttons{}); e != nil {
			return e
		}
	}
	result := &gameapi.Result{Cooperative: true, Ranks: make([]int, s.Players), Winners: []int{}, Scores: make([]int, s.Players)}
	for i := range result.Ranks {
		result.Ranks[i] = 1
		result.Scores[i] = int(s.Session.World.Active.Scores.Hundreds[i]) * 100
		if s.Session.World.Campaign.Outcome == engine.CampaignCleared {
			result.Winners = append(result.Winners, i)
		}
	}
	g.Result = result
	g.Paused = false
	g.Actors = []int{}
	g.Attention = []int{}
	g.TimeoutActors = []int{}
	g.SystemDeadline = 0
	g.Deadline = 0
	s.StageLoading = nil
	critical(g, s, "terminal")
	return nil
}
func contextError(c gameapi.Context, s *State) error {
	if c.Now.UnixMilli() < s.ServerAt {
		return fmt.Errorf("authoritative clock moved backwards")
	}
	return nil
}
