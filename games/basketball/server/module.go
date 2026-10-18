// Package basketball hosts the recovered engine in the platform's room/timer
// contract. It is not yet included in the production game registry.
package basketball

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const frameMS int64 = 100
const idleMS int64 = 15000
const lostInputMS int64 = 750
const maxSequence int64 = 1<<53 - 1

type Options struct {
	Campaign   *CampaignOptions `json:"campaign,omitempty"`
	Teams      [2]uint8         `json:"teams"`
	Characters [4]uint8         `json:"characters"`
	Mode       string           `json:"mode"`
	Court      string           `json:"court"`
}
type InputChange struct {
	At      int64          `json:"at"`
	Buttons engine.Buttons `json:"buttons"`
}
type InputState struct {
	Sequence       int64          `json:"sequence"`
	Buttons        engine.Buttons `json:"buttons"`
	Pending        engine.Buttons `json:"pending"`
	LastPacketAt   int64          `json:"lastPacketAt"`
	IdleAt         int64          `json:"idleAt"`
	Automated      bool           `json:"automated"`
	AppliedButtons engine.Buttons `json:"appliedButtons"`
	Changes        []InputChange  `json:"changes,omitempty"`
}
type SoundEvent struct {
	Sequence   uint64 `json:"sequence"`
	VideoFrame uint64 `json:"videoFrame"`
	IDs        []int  `json:"ids"`
}
type State struct {
	Campaign      *CampaignState              `json:"campaign,omitempty"`
	Options       Options                     `json:"options"`
	Epoch         string                      `json:"epoch"`
	ServerAt      int64                       `json:"serverAt"`
	Remainder     int64                       `json:"remainder"`
	PauseAt       int64                       `json:"pauseAt"`
	Runtime       engine.RuntimeState         `json:"runtime"`
	Inputs        [4]InputState               `json:"inputs"`
	Events        []SoundEvent                `json:"events"`
	SoundSequence uint64                      `json:"soundSequence"`
	VideoInputs   []engine.RecordedVideoInput `json:"videoInputs,omitempty"`
	Music         MusicState                  `json:"music"`
}
type View struct {
	Campaign      *CampaignView                   `json:"campaign,omitempty"`
	Preparation   *engine.CampaignPreparationView `json:"preparation,omitempty"`
	CanPrepare    bool                            `json:"canPrepare"`
	SeatActors    []int                           `json:"seatActors"`
	LocalActor    int                             `json:"localActor"`
	Options       Options                         `json:"options"`
	Epoch         string                          `json:"epoch"`
	ServerAt      int64                           `json:"serverAt"`
	Phase         string                          `json:"phase"`
	Runtime       engine.RuntimeState             `json:"runtime"`
	Inputs        [4]InputState                   `json:"inputs"`
	Events        []SoundEvent                    `json:"events"`
	SoundSequence uint64                          `json:"soundSequence"`
	Scores        [2]uint16                       `json:"scores"`
	CanInput      bool                            `json:"canInput"`
	VideoInputs   []engine.RecordedVideoInput     `json:"videoInputs"`
	Music         MusicView                       `json:"music"`
}
type action struct {
	Campaign    *CampaignAction                   `json:"campaign,omitempty"`
	Preparation *engine.CampaignPreparationAction `json:"preparation,omitempty"`
	Type        string                            `json:"type"`
	Epoch       string                            `json:"epoch"`
	Sequence    int64                             `json:"sequence"`
	Buttons     engine.Buttons                    `json:"buttons"`
	Seat        *int                              `json:"seat,omitempty"`
	BasedOn     *int64                            `json:"basedOn,omitempty"`
	Events      []inputEvent                      `json:"events,omitempty"`
}

// RuntimeAdvanceError retains the exact failing input for developer diagnostics.
// The room transaction still returns its unchanged pre-frame Match. Error()
// deliberately omits snapshots; ordinary host logs remain compact.
type RuntimeAdvanceError struct {
	Before engine.RuntimeState
	Held   [4]engine.Buttons
	Cause  error
}

func (e *RuntimeAdvanceError) Error() string {
	return fmt.Sprintf("篮球阶段尚未恢复（视频帧 %d）：%v", e.Before.VideoFrames, e.Cause)
}
func (e *RuntimeAdvanceError) Unwrap() error { return e.Cause }

func strict(data json.RawMessage, value any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("请求包含多余内容")
	}
	return nil
}
func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	o := defaultOptions()
	if len(raw) > 0 {
		if err := strict(raw, &o); err != nil {
			return nil, fmt.Errorf("无效篮球设置")
		}
	}
	if o.Mode == "" {
		switch players {
		case 1:
			o.Mode = "solo"
		case 2:
			o.Mode = "cooperative"
		case 4:
			o.Mode = "four-player"
		}
	}
	if o.Mode == "" || len(o.seatActors()) != players || players == 0 {
		return nil, fmt.Errorf("人数与比赛模式不一致，请等待房间席位就绪")
	}
	if o.Campaign != nil {
		var err error
		o, err = configureCampaign(o)
		if err != nil {
			return nil, err
		}
		return json.Marshal(o)
	}
	if _, ok := preparedCourt(o.Court); !ok {
		return nil, fmt.Errorf("该球场尚未完成接入验收")
	}
	if err := validateRoster(o); err != nil {
		return nil, err
	}
	return json.Marshal(o)
}

// Actors remain in the original four-character order. Network identity and
// host deadlines/results always use the platform seat, never this actor index.
func (o Options) seatActors() []int {
	switch o.Mode {
	case "solo":
		return []int{0}
	case "cooperative":
		return []int{0, 1}
	case "versus":
		return []int{0, 2}
	// Empty mode is a schema-1 development snapshot written before room modes.
	case "", "four-player":
		return []int{0, 1, 2, 3}
	default:
		return nil
	}
}
func (s State) seatActors() []int { return s.Options.seatActors() }
func (s State) actorForSeat(seat int) int {
	actors := s.seatActors()
	if seat < 0 || seat >= len(actors) {
		return -1
	}
	return actors[seat]
}
func automated(ctx gameapi.Context, o Options) ([4]bool, error) {
	a := [4]bool{true, true, true, true}
	actors := o.seatActors()
	if len(actors) == 0 || ctx.Players != len(actors) || (len(ctx.Automated) != 0 && len(ctx.Automated) != ctx.Players) {
		return a, fmt.Errorf("无效席位控制信息")
	}
	for seat, actor := range actors {
		a[actor] = len(ctx.Automated) > 0 && ctx.Automated[seat]
	}
	return a, nil
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "basketball", Name: "热血篮球", MinPlayers: 1, MaxPlayers: 2, Realtime: true,
		Configure: configure, Prepare: prepare, Start: start, Apply: apply, View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var options Options
	if err = json.Unmarshal(o, &options); err != nil {
		return gameapi.Match{}, err
	}
	flags, err := automated(ctx, options)
	if err != nil {
		return gameapi.Match{}, err
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return gameapi.Match{}, err
	}
	c := engine.DefaultRuntimeConfig()
	c.Match.Mode = 128
	c.Match.Court, _ = preparedCourt(options.Court)
	c.Match.Teams = options.Teams
	c.Match.Characters = options.Characters
	c.Match.Seed = engine.RandomState(binary.LittleEndian.Uint32(token[:4]))
	for i, a := range flags {
		c.Match.ControllerAssignments[i] = int8(i)
		if a {
			c.Match.ControllerAssignments[i] = -1
		}
	}
	var r engine.RuntimeState
	if options.Campaign != nil {
		r, err = newCampaignMatch(options, c.Match.Seed)
		if err == nil {
			r, err = engine.SetRuntimeAutomation(r, flags)
		}
	} else {
		r, err = engine.NewRuntime(c)
	}
	if err != nil {
		return gameapi.Match{}, err
	}
	s := State{Epoch: hex.EncodeToString(token[4:]), ServerAt: ctx.Now.UnixMilli(), Runtime: r, Events: []SoundEvent{}}
	if err = json.Unmarshal(o, &s.Options); err != nil {
		return gameapi.Match{}, err
	}
	if options.Campaign != nil {
		s.Campaign = &CampaignState{Stage: "playing", Round: 1}
		s.Options.Characters = r.Config.Match.Characters
	}
	for i, a := range flags {
		s.Inputs[i] = InputState{Automated: a, IdleAt: s.ServerAt + idleMS, LastPacketAt: s.ServerAt}
	}
	return s.match()
}
func decode(m gameapi.Match) (State, error) {
	s := State{Options: defaultOptions()}
	if m.Schema != 1 {
		return s, fmt.Errorf("不支持的篮球存档版本")
	}
	if err := strict(m.Data, &s); err != nil {
		return s, err
	}
	court, knownCourt := preparedCourt(s.Options.Court)
	if s.Runtime.Config.Campaign != nil && s.Options.Court == "court-7" {
		court, knownCourt = 7, true
	}
	if !knownCourt || validateSavedRoster(s) != nil || s.Runtime.Config.Match.Court != court || s.Runtime.Config.Match.Teams != s.Options.Teams || s.Runtime.Config.Match.Characters != s.Options.Characters || len(s.Epoch) != 24 || s.ServerAt < 0 || s.Remainder < 0 || s.Remainder >= 1000 || s.PauseAt < 0 {
		return s, fmt.Errorf("无效篮球存档")
	}
	if _, err := hex.DecodeString(s.Epoch); err != nil {
		return s, err
	}
	if len(s.seatActors()) == 0 {
		return s, fmt.Errorf("无效篮球分队存档")
	}
	owned := [4]bool{}
	for _, actor := range s.seatActors() {
		owned[actor] = true
	}
	for actor, input := range s.Inputs {
		if !owned[actor] && (!input.Automated || s.Runtime.Config.Match.ControllerAssignments[actor] >= 0 || input.Buttons != 0 || input.Pending != 0 || input.AppliedButtons != 0 || len(input.Changes) != 0) {
			return s, fmt.Errorf("无效电脑角色存档")
		}
	}
	for _, input := range s.Inputs {
		if input.Sequence < 0 || input.Sequence > maxSequence || input.IdleAt < 0 || input.LastPacketAt < 0 {
			return s, fmt.Errorf("无效输入存档")
		}
		if len(input.Changes) > 64 {
			return s, fmt.Errorf("输入队列过长")
		}
		var previous int64
		for _, change := range input.Changes {
			if change.At < previous || change.At < 0 || change.At > maxSequence {
				return s, fmt.Errorf("无效输入时间线")
			}
			previous = change.At
		}
	}
	if len(s.Events) > 64 || s.SoundSequence > uint64(maxSequence) {
		return s, fmt.Errorf("无效声音事件")
	}
	if err := s.Music.validate(s.SoundSequence); err != nil {
		return s, err
	}
	if len(s.VideoInputs) > 120 {
		return s, fmt.Errorf("视频输入历史过长")
	}
	for n, input := range s.VideoInputs {
		if input.Frame == 0 || input.Frame > s.Runtime.VideoFrames || n > 0 && input.Frame != s.VideoInputs[n-1].Frame+1 {
			return s, fmt.Errorf("无效视频输入历史")
		}
	}
	if n := len(s.VideoInputs); n > 0 && s.VideoInputs[n-1].Frame != s.Runtime.VideoFrames {
		return s, fmt.Errorf("视频历史与当前帧不一致")
	}
	if err := engine.ValidateRuntimeState(s.Runtime); err != nil {
		return s, err
	}
	return s, s.validateCampaignState()
}
func (s State) phase() string {
	if s.campaignMenu() {
		return "campaign-" + s.Campaign.Stage
	}
	if s.Runtime.Outcome != nil {
		return "result"
	}
	if s.Runtime.AwaitingNextPeriod {
		return "intermission"
	}
	if s.Runtime.Playing == nil {
		return "opening"
	}
	if s.Runtime.Playing.Match.Phase == engine.MatchPaused {
		return "paused"
	}
	return "playing"
}
func (s State) scores() [2]uint16 {
	if s.Runtime.Outcome != nil {
		return s.Runtime.Outcome.Scores
	}
	if s.Runtime.Playing != nil {
		return s.Runtime.Playing.World.Preparation.Scores
	}
	if s.Runtime.Transition != nil {
		return s.Runtime.Transition.Scene.Transition.Scores
	}
	return s.Runtime.Opening.Transition.Scores
}
func (s State) match() (gameapi.Match, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{Schema: 1, Data: data, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, HasComputerPlayers: len(s.seatActors()) < 4, Log: []string{s.rosterDescription()}}
	if s.campaignMenu() {
		return s.campaignMatch(m)
	}
	if out := s.Runtime.Outcome; out != nil && out.Confirmed {
		cooperative := s.Options.Mode == "solo" || s.Options.Mode == "cooperative"
		r := &gameapi.Result{Team: !cooperative, Cooperative: cooperative, Ranks: make([]int, len(s.seatActors())), Scores: make([]int, len(s.seatActors())), Winners: []int{}}
		for seat, actor := range s.seatActors() {
			r.Ranks[seat] = 2
			if cooperative {
				r.Ranks[seat] = 1
			}
			r.Scores[seat] = int(out.Scores[actor/2])
			if actor/2 == int(out.WinningTeam) {
				r.Ranks[seat] = 1
				r.Winners = append(r.Winners, seat)
			}
		}
		r.Details, _ = json.Marshal(out)
		m.Result = r
		return m, nil
	}
	m.SystemDeadline = s.ServerAt + frameMS
	for seat, actor := range s.seatActors() {
		input := s.Inputs[actor]
		m.Actors = append(m.Actors, seat)
		if !input.Automated && s.phase() != "paused" && s.Runtime.Outcome == nil && (m.Deadline == 0 || input.IdleAt < m.Deadline) {
			m.Deadline = input.IdleAt
		}
	}
	if m.Deadline > 0 {
		for seat, actor := range s.seatActors() {
			input := s.Inputs[actor]
			if !input.Automated && input.IdleAt == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, seat)
			}
		}
	}
	return m, nil
}
func validAction(ctx gameapi.Context, s State, raw json.RawMessage) (action, error) {
	var a action
	if err := strict(raw, &a); err != nil {
		return a, fmt.Errorf("无效篮球操作")
	}
	if a.Type == "campaign" {
		_, err := applyCampaignAction(ctx, s, a)
		return a, err
	}
	if a.Campaign != nil || s.campaignMenu() {
		return a, fmt.Errorf("当前不能提交控制器或中场操作")
	}
	if a.Type == "campaign-preparation" {
		_, err := applyCampaignPreparation(ctx, s, a)
		return a, err
	}
	if a.Preparation != nil {
		return a, fmt.Errorf("控制器输入不能同时修改中场准备")
	}
	flags, err := automated(ctx, s.Options)
	if err != nil {
		return a, err
	}
	if ctx.Actor < 0 || s.actorForSeat(ctx.Actor) < 0 || flags[s.actorForSeat(ctx.Actor)] || s.Runtime.Outcome != nil && s.Runtime.Outcome.Confirmed {
		return a, fmt.Errorf("当前席位不能操作")
	}
	if a.Epoch != s.Epoch || a.Sequence <= 0 || a.Sequence > maxSequence || a.Sequence-s.Inputs[s.actorForSeat(ctx.Actor)].Sequence > 1024 {
		return a, fmt.Errorf("输入已过期或超出范围")
	}
	if a.Type == "inputs" {
		_, err := prepareBatch(ctx, s, a)
		return a, err
	}
	if a.Sequence <= s.Inputs[s.actorForSeat(ctx.Actor)].Sequence || a.Seat != nil || a.BasedOn != nil || a.Events != nil {
		return a, fmt.Errorf("输入已过期或包含无效字段")
	}
	if a.Type != "buttons" && a.Type != "release" {
		return a, fmt.Errorf("未知篮球操作")
	}
	if a.Type == "release" && a.Buttons != 0 {
		return a, fmt.Errorf("取消输入不能同时按键")
	}
	if a.Type == "buttons" && a.Buttons != s.Inputs[s.actorForSeat(ctx.Actor)].Buttons && len(s.Inputs[s.actorForSeat(ctx.Actor)].Changes) >= 64 {
		return a, fmt.Errorf("输入过于频繁，请稍后重试")
	}
	return a, nil
}
func clearTransport(input *InputState) {
	input.Buttons, input.Pending, input.AppliedButtons = 0, 0, 0
	input.Changes = nil
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	a, err := validAction(ctx, s, raw)
	if err != nil {
		return m, err
	}
	if a.Type == "campaign" {
		s, err = applyCampaignAction(ctx, s, a)
		if err != nil {
			return m, err
		}
		return s.match()
	}
	if a.Type == "campaign-preparation" {
		s, err = applyCampaignPreparation(ctx, s, a)
		if err != nil {
			return m, err
		}
		return s.match()
	}
	if a.Type == "inputs" {
		if a.Sequence <= s.Inputs[s.actorForSeat(ctx.Actor)].Sequence {
			return m, nil // Retries must not refresh deadlines or held inputs.
		}
		s, err = prepareBatch(ctx, s, a)
		if err != nil {
			return m, err
		}
		return s.match()
	}
	i := &s.Inputs[s.actorForSeat(ctx.Actor)]
	i.Sequence = a.Sequence
	i.LastPacketAt = ctx.Now.UnixMilli()
	if a.Type == "release" {
		clearTransport(i)
		s.Runtime, err = engine.ClearRuntimeInput(s.Runtime, uint8(s.actorForSeat(ctx.Actor)))
		if err != nil {
			return m, err
		}
	} else {
		i.Pending |= a.Buttons &^ i.Buttons
		if a.Buttons != i.Buttons {
			at := i.LastPacketAt
			if at < s.ServerAt {
				at = s.ServerAt
			}
			if n := len(i.Changes); n > 0 && at < i.Changes[n-1].At {
				at = i.Changes[n-1].At
			}
			i.Changes = append(i.Changes, InputChange{At: at, Buttons: a.Buttons})
		}
		if a.Buttons != 0 || i.Buttons != 0 {
			i.IdleAt = i.LastPacketAt + idleMS
		}
		i.Buttons = a.Buttons
	}
	return s.match()
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, err := decode(m)
	if err != nil {
		return false
	}
	_, err = validAction(ctx, s, raw)
	return err == nil
}

// Original AI is evaluated at the original world-input stage in StepVideo,
// using the same action dispatcher as human commands. A second host strategy
// would run at the wrong cadence and duplicate decisions, so no substitute
// input is proposed here. OnTimer consumes authoritative Automated metadata.
func botAction(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }

func (s *State) syncAutomation(flags [4]bool) error {
	if s.Runtime.Outcome == nil {
		r, err := engine.SetRuntimeAutomation(s.Runtime, flags)
		if err != nil {
			return err
		}
		s.Runtime = r
	}
	for seat, a := range flags {
		if s.Inputs[seat].Automated != a {
			clearTransport(&s.Inputs[seat])
		}
		s.Inputs[seat].Automated = a
	}
	return nil
}
func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	now := ctx.Now.UnixMilli()
	if m.Result != nil || now < s.ServerAt+frameMS {
		return m, nil
	}
	flags, err := automated(ctx, s.Options)
	if err != nil {
		return m, err
	}
	wasAutomatic := s.Inputs[0].Automated
	if err = s.syncAutomation(flags); err != nil {
		return m, err
	}
	if s.campaignMenu() {
		s.ServerAt = now
		if !s.campaignTerminal() && flags[0] && !wasAutomatic {
			c := *s.Campaign
			c.DecisionAt = now + 3000
			s.Campaign = &c
		}
		if !s.campaignTerminal() && flags[0] && now >= s.Campaign.DecisionAt {
			kind := "continue"
			if s.Campaign.Stage == "prepare" {
				kind = "start"
			}
			s, err = advanceCampaignMenu(ctx, s, CampaignAction{Type: kind, Revision: s.Campaign.Revision}, flags)
			if err != nil {
				return m, err
			}
		}
		return s.match()
	}
	elapsed := now - s.ServerAt
	if elapsed > 250 {
		elapsed = 250
	}
	previousRemainder := s.Remainder
	accumulator := previousRemainder + elapsed*60
	ticks := accumulator / 1000
	s.Remainder = accumulator % 1000
	s.ServerAt = now
	startAutomatic := s.Inputs[0].Automated
	for seat := range s.Inputs {
		i := &s.Inputs[seat]
		if !i.Automated && now-i.LastPacketAt > lostInputMS {
			clearTransport(i)
			s.Runtime, err = engine.ClearRuntimeInput(s.Runtime, uint8(seat))
			if err != nil {
				return m, err
			}
		}
	}
	for tick := int64(0); tick < ticks; tick++ {
		if s.Runtime.Outcome != nil && s.Runtime.Outcome.Confirmed {
			break
		}
		// Apply accepted changes at their actual server-receipt position on
		// the 60 Hz timeline. Multiple presses in one 100 ms host frame must
		// not collapse into a single edge (notably 10–15 Hz turbo input).
		tickAt := (now-elapsed)*60 + 1000 - previousRemainder + tick*1000
		var held [4]engine.Buttons
		for seat := range s.Inputs {
			i := &s.Inputs[seat]
			if i.Automated {
				continue
			}
			for len(i.Changes) > 0 && i.Changes[0].At*60 <= tickAt {
				change := i.Changes[0]
				i.Changes = i.Changes[1:]
				s.Runtime.PendingPressed[seat] |= change.Buttons &^ i.AppliedButtons
				i.AppliedButtons = change.Buttons
			}
			held[seat] = i.AppliedButtons
		}
		if startAutomatic && (s.Runtime.AwaitingNextPeriod && s.Runtime.WaitVideoFrames == 0 || s.phase() == "paused") {
			s.Runtime.PendingPressed[0] |= engine.ButtonStart
		}
		input := engine.RecordedVideoInput{Frame: s.Runtime.VideoFrames + 1, Held: held, PriorHeld: s.Runtime.Held, PendingPressed: s.Runtime.PendingPressed}
		for seat, controller := range s.Runtime.Config.Match.ControllerAssignments {
			input.Automated[seat] = controller < 0
		}
		step, e := engine.StepVideo(s.Runtime, held)
		if e != nil {
			return m, &RuntimeAdvanceError{Before: s.Runtime, Held: held, Cause: e}
		}
		s.Runtime = step.State
		if s.Runtime.Config.Campaign != nil {
			// Original halftime confirmation can select a new opponent roster
			// as well as commit the captain's own pair. Persist both atomically.
			s.Options.Characters = s.Runtime.Config.Match.Characters
		}
		s.VideoInputs = append(s.VideoInputs, input)
		if len(s.VideoInputs) > 120 {
			s.VideoInputs = s.VideoInputs[len(s.VideoInputs)-120:]
		}
		if len(step.SoundIDs) > 0 {
			s.SoundSequence++
			event := SoundEvent{Sequence: s.SoundSequence, VideoFrame: s.Runtime.VideoFrames, IDs: make([]int, len(step.SoundIDs))}
			for index, id := range step.SoundIDs {
				event.IDs[index] = int(id)
			}
			if err = s.Music.Apply(event.IDs, event.Sequence, tickAt/60); err != nil {
				return m, err
			}
			s.Events = append(s.Events, event)
			if len(s.Events) > 64 {
				s.Events = s.Events[len(s.Events)-64:]
			}
		}
	}
	for seat := range s.Inputs {
		i := &s.Inputs[seat]
		i.Pending = 0
		held := i.AppliedButtons
		for _, change := range i.Changes {
			i.Pending |= change.Buttons &^ held
			held = change.Buttons
		}
	}
	if s.phase() == "paused" && s.PauseAt == 0 {
		s.PauseAt = now
	}
	if s.phase() != "paused" && s.PauseAt != 0 {
		for seat := range s.Inputs {
			if !s.Inputs[seat].Automated {
				since := s.PauseAt
				if active := s.Inputs[seat].IdleAt - idleMS; active > since {
					since = active
				}
				if now > since {
					s.Inputs[seat].IdleAt += now - since
				}
			}
		}
		s.PauseAt = 0
	}
	if s.Runtime.Outcome != nil && s.Runtime.Outcome.Confirmed {
		s.Runtime.Display, err = engine.CommitDisplayVideo(s.Runtime.Display, 0, 0)
		if err != nil {
			return m, err
		}
	}
	if err = s.settleCampaign(); err != nil {
		return m, err
	}
	return s.match()
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if ctx.Actor < 0 || s.actorForSeat(ctx.Actor) < 0 || m.Result != nil {
		return m, nil
	}
	if _, err = automated(ctx, s.Options); err != nil {
		return m, err
	}
	var flags [4]bool
	for seat, input := range s.Inputs {
		flags[seat] = input.Automated
	}
	flags[s.actorForSeat(ctx.Actor)] = false
	if err = s.syncAutomation(flags); err != nil {
		return m, err
	}
	i := &s.Inputs[s.actorForSeat(ctx.Actor)]
	clearTransport(i)
	i.IdleAt = ctx.Now.UnixMilli() + idleMS
	i.LastPacketAt = ctx.Now.UnixMilli()
	if s.campaignMenu() {
		return s.match()
	}
	s.Runtime, err = engine.ClearRuntimeInput(s.Runtime, uint8(s.actorForSeat(ctx.Actor)))
	if err != nil {
		return m, err
	}
	return s.match()
}
func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if m.Result != nil {
		return m, nil
	}
	s.ServerAt = ctx.Now.UnixMilli()
	if s.campaignMenu() {
		return s.match()
	}
	for seat := range s.Inputs {
		clearTransport(&s.Inputs[seat])
		s.Runtime, err = engine.ClearRuntimeInput(s.Runtime, uint8(seat))
		if err != nil {
			return m, err
		}
	}
	return s.match()
}
func project(m gameapi.Match, seat int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	actor := s.actorForSeat(seat)
	v := View{SeatActors: s.seatActors(), LocalActor: actor, Options: s.Options, Epoch: s.Epoch, ServerAt: s.ServerAt, Phase: s.phase(), Runtime: s.Runtime, Inputs: s.Inputs, Events: s.Events, SoundSequence: s.SoundSequence, Scores: s.scores(), CanInput: actor >= 0 && !s.Inputs[actor].Automated && m.Result == nil, VideoInputs: s.VideoInputs, Music: s.Music.view(s.ServerAt)}
	v.Campaign, err = s.campaignView(actor)
	if err != nil {
		return nil, err
	}
	if s.campaignMenu() {
		v.CanInput = false
	}
	if s.Runtime.Config.Campaign != nil && s.Runtime.AwaitingNextPeriod {
		p, err := engine.ViewCampaignPreparation(s.Runtime)
		if err != nil {
			return nil, err
		}
		v.Preparation = &p
		v.CanPrepare = actor == 0 && v.CanInput
	}
	return v, nil
}

func (s State) rosterDescription() string {
	switch s.Options.Mode {
	case "solo":
		return "热血篮球：单人 · 你与原版电脑队友对阵电脑队伍"
	case "cooperative":
		return "热血篮球：双人合作 · 两位玩家同属 A 队，对阵电脑 B 队"
	case "versus":
		return "热血篮球：双人对战 · 第一席 A 队，第二席 B 队，各有一名原版电脑队友"
	default:
		return "热血篮球：A 队为席位 1、2，B 队为席位 3、4"
	}
}
