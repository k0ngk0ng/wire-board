package engine

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

type RuntimeConfig struct {
	// Campaign is the immutable starting preparation snapshot, not live progress.
	Campaign     *campaign.Progress         `json:"campaign,omitempty"`
	Match        OriginalMatchConfiguration `json:"match"`
	HumanTactics [2]uint8                   `json:"humanTactics"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{Match: DefaultOriginalMatchConfiguration()}
}

type RuntimePlayingState struct {
	World          WorldLoopState   `json:"world"`
	Match          MatchState       `json:"match"`
	Context        WorldLoopContext `json:"context"`
	TacticCache    [4]uint8         `json:"tacticCache"`
	TeamWinBits    [2]uint8         `json:"teamWinBits"`
	ResultExitFlag int8             `json:"resultExitFlag"`
}

// Confirmed marks completion of the result scene, not its destination menu.
type RuntimeOutcome struct {
	WinningTeam uint8      `json:"winningTeam"`
	Scores      [2]uint16  `json:"scores"`
	Confirmed   bool       `json:"confirmed"`
	NextScene   uint8      `json:"nextScene"`
	NextPhase   MatchPhase `json:"nextPhase"`
}
type RuntimeTransitionState struct {
	Scene OpeningState `json:"scene"`
}

// Exactly one stage pointer is active. Completed stage objects are discarded;
// they are never a second authoritative copy of the playing world.
type RuntimeState struct {
	Outcome    *RuntimeOutcome         `json:"outcome,omitempty"`
	Config     RuntimeConfig           `json:"config"`
	Display    DisplayState            `json:"display"`
	Opening    *OpeningState           `json:"opening,omitempty"`
	Playing    *RuntimePlayingState    `json:"playing,omitempty"`
	Transition *RuntimeTransitionState `json:"transition,omitempty"`
	// Frozen preceding-period fields that the original reset deliberately keeps.
	// It is consumed at handoff and is never an active playing world.
	Carry              *WorldLoopState `json:"carry,omitempty"`
	VideoFrames        uint64          `json:"videoFrames"`
	LogicSteps         uint64          `json:"logicSteps"`
	LogicFrame         uint8           `json:"logicFrame"`
	FramesUntilLogic   uint8           `json:"framesUntilLogic"`
	WaitVideoFrames    uint16          `json:"waitVideoFrames"`
	Held               [4]Buttons      `json:"held"`
	PendingPressed     [4]Buttons      `json:"pendingPressed"`
	AwaitingNextPeriod bool            `json:"awaitingNextPeriod"`
}
type RuntimeStep struct {
	State           RuntimeState `json:"state"`
	SoundIDs        []uint8      `json:"soundIDs,omitempty"`
	LogicAdvanced   bool         `json:"logicAdvanced"`
	FadeIn, FadeOut bool
	Backgrounds     []uint8 `json:"backgrounds,omitempty"`
}

func validateRuntimeConfig(c RuntimeConfig) error {
	if err := validateCampaignConfiguration(c); err != nil {
		return err
	}
	if c.Match.Court >= 8 {
		return fmt.Errorf("runtime court is not yet supported by complete motion/action integration")
	}
	for _, team := range c.Match.Teams {
		if team >= 8 {
			return fmt.Errorf("invalid team")
		}
	}
	for i, ch := range c.Match.Characters {
		if ch >= 4 {
			return fmt.Errorf("invalid character for seat %d", i)
		}
	}
	for _, controller := range c.Match.ControllerAssignments {
		if controller < -1 || controller > 3 {
			return fmt.Errorf("invalid controller assignment")
		}
	}
	for _, slots := range c.Match.Equipment {
		for _, item := range slots {
			if item < -1 || item >= 32 {
				return fmt.Errorf("invalid equipment selection")
			}
		}
	}
	for _, tactic := range c.HumanTactics {
		if tactic >= 6 {
			return fmt.Errorf("invalid human tactic")
		}
	}
	return nil
}
func NewRuntime(c RuntimeConfig) (RuntimeState, error) {
	var s RuntimeState
	if c.Campaign != nil {
		copy := *c.Campaign
		c.Campaign = &copy
	}
	if e := validateRuntimeConfig(c); e != nil {
		return s, e
	}
	opening, e := NewOriginalMatch(c.Match)
	if e != nil {
		return s, e
	}
	if c.Campaign != nil {
		opening.Transition.Match.Period = c.Campaign.Period
		opening.Transition.TeamWinBits = c.Campaign.WinBits
	}
	display, e := NewOriginalDisplay(c.Match)
	if e != nil {
		return RuntimeState{}, e
	}
	s.Display = display
	s.Config = c
	s.Opening = &opening
	return s, nil
}
func runtimeMatch(s *RuntimeState) *MatchState {
	if s.Opening != nil {
		return &s.Opening.Transition.Match
	}
	if s.Playing != nil {
		return &s.Playing.Match
	}
	if s.Transition != nil {
		return &s.Transition.Scene.Transition.Match
	}
	return nil
}
func cloneRuntime(s RuntimeState) RuntimeState {
	if s.Config.Campaign != nil {
		copy := *s.Config.Campaign
		s.Config.Campaign = &copy
	}
	if s.Outcome != nil {
		v := *s.Outcome
		s.Outcome = &v
	}
	if s.Opening != nil {
		v := *s.Opening
		s.Opening = &v
	}
	if s.Playing != nil {
		v := *s.Playing
		s.Playing = &v
	}
	if s.Transition != nil {
		v := *s.Transition
		s.Transition = &v
	}
	if s.Carry != nil {
		v := *s.Carry
		s.Carry = &v
	}
	return s
}

// ValidateRuntimeState reads only. Restore never repairs or rewrites state.
func ValidateRuntimeState(s RuntimeState) error {
	if e := validateRuntimeConfig(s.Config); e != nil {
		return e
	}
	count := 0
	if s.Opening != nil {
		count++
	}
	if s.Playing != nil {
		count++
	}
	if s.Transition != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("runtime requires exactly one active stage")
	}
	if s.VideoFrames > 1<<53-1 || s.LogicSteps > 1<<53-1 || s.FramesUntilLogic > 3 || s.WaitVideoFrames > 600 {
		return fmt.Errorf("invalid runtime counter")
	}
	var t *MatchTransitionState
	if s.Opening != nil {
		t = &s.Opening.Transition
		if t.Scene != 1 || (t.Match.Phase != 0 && t.Match.Phase != 8 && t.Match.Phase != MatchPlaying && t.Match.Phase != 3) {
			return fmt.Errorf("invalid opening phase")
		}
		if t.Match.Phase == MatchPlaying && t.Match.Entered {
			return fmt.Errorf("completed opening must hand off to playing")
		}
	} else if s.Transition != nil {
		t = &s.Transition.Scene.Transition
		if (s.Outcome == nil || !s.Outcome.Confirmed) && t.Match.Phase != MatchPeriodEnding && t.Match.Phase != 4 && t.Match.Phase != 6 {
			return fmt.Errorf("unsupported transition phase")
		}
		if s.AwaitingNextPeriod != (t.Scene == 2 && (s.Outcome == nil || !s.Outcome.Confirmed)) {
			return fmt.Errorf("invalid intermission stage")
		}
	} else {
		p := s.Playing
		if p.Match.Entered != (p.Match.Phase == MatchPlaying) {
			return fmt.Errorf("invalid entered bit for playing or paused phase")
		}
		if e := validateRuntimePlayingConfig(s.Config, p); e != nil {
			return e
		}
		if s.Carry != nil {
			return fmt.Errorf("playing stage cannot retain a previous world")
		}
		if (p.Match.Phase != MatchPlaying && p.Match.Phase != MatchPaused) || !p.World.Started || p.World.Updates == 0 || p.World.Updates > s.LogicSteps {
			return fmt.Errorf("invalid playing phase or counter")
		}
		if p.Context.Actions.CourtID != s.Config.Match.Court || p.Context.Actions.Teams != s.Config.Match.Teams || p.Context.Actions.Characters != s.Config.Match.Characters || p.Context.HumanTactics != s.Config.HumanTactics || p.Context.LayoutID != 1 || p.Context.PursuitAlias74 == nil {
			return fmt.Errorf("playing configuration does not match runtime")
		}
		if p.World.LogicFrame != s.LogicFrame {
			return fmt.Errorf("world and runtime logical counters differ")
		}
	}
	if s.Opening != nil {
		if e := validateRuntimeOpeningConfig(s.Config, *s.Opening); e != nil {
			return e
		}
	}
	if s.Transition != nil && (s.Outcome == nil || !s.Outcome.Confirmed) {
		if e := validateRuntimeOpeningConfig(s.Config, s.Transition.Scene); e != nil {
			return e
		}
	}
	if t != nil && (s.Outcome == nil || !s.Outcome.Confirmed) && (t.Court != s.Config.Match.Court || t.Teams != s.Config.Match.Teams || t.Characters != s.Config.Match.Characters || t.ControllerAssignments != s.Config.Match.ControllerAssignments) {
		return fmt.Errorf("stage configuration does not match runtime")
	}
	if s.Transition == nil && s.AwaitingNextPeriod {
		return fmt.Errorf("intermission without transition")
	}
	if e := validateRuntimeOutcome(s); e != nil {
		return e
	}
	if e := validateCampaignMatchProgress(s); e != nil {
		return e
	}
	m := runtimeMatch(&s)
	if m.Clock.Tens > 5 || m.Clock.Ones > 9 || m.Clock.Minutes > 9 || m.Clock.HUDTurn > 3 {
		return fmt.Errorf("invalid decimal match clock or HUD turn")
	}
	return validateRuntimeDisplay(s.Display)
}

// StepVideo is the browser entry point. It advances one video interrupt and
// automatically invokes StepLogic when due; callers must not also tick logic.
// Platform edge latching preserves a short press until one logical sample.
func StepVideo(original RuntimeState, held [4]Buttons) (RuntimeStep, error) {
	initial := RuntimeStep{State: original}
	if e := ValidateRuntimeState(original); e != nil {
		return initial, e
	}
	if original.VideoFrames >= 1<<53-1 {
		return initial, fmt.Errorf("video counter exhausted")
	}
	s := cloneRuntime(original)
	s.VideoFrames++
	for i, h := range held {
		s.PendingPressed[i] |= h &^ s.Held[i]
		s.Held[i] = h
	}
	m := runtimeMatch(&s)
	*m = AdvanceMatchVideoTick(*m).State
	src := runtimeDisplaySource(s)
	display, e := CommitDisplayVideo(s.Display, src.DisplayFlags, src.DetachedCount)
	if e != nil {
		return initial, e
	}
	s.Display = display
	if s.WaitVideoFrames > 0 {
		s.WaitVideoFrames--
		return RuntimeStep{State: s}, nil
	}
	if s.FramesUntilLogic > 0 {
		s.FramesUntilLogic--
	}
	if s.FramesUntilLogic > 0 {
		return RuntimeStep{State: s}, nil
	}
	var frames [4]ControllerFrame
	for i := range frames {
		// A buffered tap must remain a valid controller sample: pressed implies
		// held for this one logic update, even if already physically released.
		frames[i] = ControllerFrame{Held: s.Held[i] | s.PendingPressed[i], Pressed: s.PendingPressed[i]}
	}
	step, e := StepLogic(s, frames)
	if e != nil {
		return initial, e
	}
	// Keep physical input separate from the normalized logical sample so a
	// new tap immediately after release still generates its own edge.
	step.State.Held = held
	return step, nil
}
func runtimeWorldInput(c RuntimeConfig, input [4]ControllerFrame) WorldLoopInput {
	result := WorldLoopInput{PhysicalControllers: &input}
	for actor, controller := range c.Match.ControllerAssignments {
		if controller >= 0 {
			result.Frames[actor] = input[controller]
		}
	}
	return result
}

// StepLogic is also a replay/testing entry point. It never advances video time.
func StepLogic(original RuntimeState, input [4]ControllerFrame) (RuntimeStep, error) {
	initial := RuntimeStep{State: original}
	if e := ValidateRuntimeState(original); e != nil {
		return initial, e
	}
	if original.WaitVideoFrames != 0 {
		return initial, fmt.Errorf("logical update requested during an explicit presentation wait")
	}
	if original.LogicSteps >= 1<<53-1 {
		return initial, fmt.Errorf("logical counter exhausted")
	}
	s := cloneRuntime(original)
	s.LogicSteps++
	s.LogicFrame++
	s.FramesUntilLogic = 3
	s.PendingPressed = [4]Buttons{}
	for i := range input {
		s.Held[i] = input[i].Held
	}
	result := RuntimeStep{State: s, LogicAdvanced: true}
	var e error
	// Menus and an uninitialized next court are not court display sources.
	// Retain the last valid submission until original court initialization has
	// supplied its camera and objects, both at cold start and between periods.
	if runtimeHasCourtDisplaySource(original) {
		display, err := StageDisplay(s.Display, runtimeDisplaySource(original))
		if err != nil {
			return initial, err
		}
		result.State.Display = display
	}
	if s.Opening != nil {
		e = stepRuntimeOpening(&result, input)
	} else if s.Playing != nil {
		e = stepRuntimePlaying(&result, input)
	} else {
		e = stepRuntimeTransition(&result, input)
	}
	if e != nil {
		return initial, e
	}
	if result.State.AwaitingNextPeriod {
		result.State.FramesUntilLogic = 1
	}
	if e = ValidateRuntimeState(result.State); e != nil {
		return initial, fmt.Errorf("runtime postcondition: %w", e)
	}
	return result, nil
}

func stepRuntimeOpening(r *RuntimeStep, input [4]ControllerFrame) error {
	s := &r.State
	o := s.Opening
	o.Transition.CameraScroll[0], o.Transition.CameraScroll[1] = o.Transition.CameraScroll[2], o.Transition.CameraScroll[3]
	o.Projection = resetOriginalWorldDrawList(o.Projection)
	o.Transition.Random, _ = NextRandom(o.Transition.Random)
	var next OpeningStep
	var err error
	initialCourt := o.Transition.Match.Phase == 0 && !o.Transition.Match.Entered
	startingCourt := o.Transition.Match.Phase == 0 && o.Transition.Match.Entered
	switch o.Transition.Match.Phase {
	case 0:
		if o.Transition.Match.Entered {
			next, err = StartOriginalCourt(*o)
		} else {
			next, err = InitializeOriginalCourt(*o)
		}
	case 8:
		next, err = StepOriginalRefereeOpening(*o)
	case MatchPlaying:
		next, err = InitializeOpeningPeriod(*o)
	case 3:
		return fmt.Errorf("opening phase 3 requires an existing playing world")
	default:
		return fmt.Errorf("unrestored opening phase %d", o.Transition.Match.Phase)
	}
	if err != nil {
		return err
	}
	*o = next.State
	// D231 selects the court palette at StartOriginalCourt, after the earlier
	// background/ring reset. Upload remains pending until the video boundary.
	if startingCourt {
		display, e := QueueDisplayPaletteLevel(s.Display, s.Config.Match.Court+1, 0)
		if e != nil {
			return e
		}
		s.Display = display
	}
	if initialCourt {
		if e := resetRuntimeCourtDisplay(s); e != nil {
			return e
		}
	}
	display, e := ObserveDisplayMotionForCourt(s.Display, next.MotionSurfaceWrites, s.Config.Match.Court)
	if e != nil {
		return e
	}
	s.Display = display
	r.SoundIDs = append(r.SoundIDs, next.SoundIDs...)
	r.FadeIn = next.FadeIn
	r.Backgrounds = next.Backgrounds
	// The original palette fade takes eight video interrupts. Tile transfer
	// work is replaced by the renderer's prepared court, not CPU emulation.
	if next.FadeIn {
		display, err := QueueDisplayFade(s.Display, false)
		if err != nil {
			return err
		}
		s.Display = display
		s.WaitVideoFrames = 8
	}
	if !next.BeginWorldInputs {
		return nil
	}
	options := OpeningWorldOptions{LogicFrame: s.LogicFrame, HumanTactics: s.Config.HumanTactics}
	if s.Config.Campaign != nil {
		options.Inventory = &s.Config.Campaign.Inventory
	}
	world, ctx, err := WorldFromOriginalOpening(*o, options)
	if err != nil {
		return err
	}
	if s.Carry != nil {
		world = mergeRuntimeCarriedWorld(world, *s.Carry)
	}
	p := RuntimePlayingState{World: world, Match: o.Transition.Match, Context: ctx, TacticCache: o.TacticCache, TeamWinBits: o.Transition.TeamWinBits, ResultExitFlag: o.Transition.ResultExitFlag}
	// D303 enters inputs in this same logical update. StepWorldLoop's first
	// update deliberately omits its normal outer RNG and preparation boundary.
	worldStep, err := StepWorldLoop(p.World, runtimeWorldInput(s.Config, input), p.Context)
	if err != nil {
		return err
	}
	p.World = worldStep.State
	if e := observeRuntimeWorldDisplay(s, worldStep, p.Match); e != nil {
		return e
	}
	r.SoundIDs = append(r.SoundIDs, worldStep.SoundIDs...)
	courtSounds, e := observeRuntimeCourtAnimation(s, &p.World, p.Context, p.Match)
	if e != nil {
		return e
	}
	r.SoundIDs = append(r.SoundIDs, courtSounds...)
	s.Opening = nil
	s.Playing = &p
	s.Carry = nil
	return nil
}
func stepRuntimePlaying(r *RuntimeStep, input [4]ControllerFrame) error {
	s := &r.State
	p := s.Playing
	if p.Match.Phase == MatchPaused {
		// CAA4 still consumes random while paused. D3C4 only projects the frozen
		// objects, even on the update that requests resume.
		p.World.Preparation.Actions.Random, _ = NextRandom(p.World.Preparation.Actions.Random)
		p.World.LogicFrame = s.LogicFrame
		p.World.Camera.DisplayedX = p.World.Camera.X
		p.World.DisplayedCameraY = p.World.Camera.Y
		p.Match = CheckMatchResume(p.Match, uint8(input[0].Pressed)).State
		pc := ProjectionContext{CameraX: p.World.Camera.X, CameraY: p.World.Camera.Y, LayoutID: 1, Poses: worldLoopPoses(p.World.Preparation.Actions), Positions: worldLoopPositions(p.World.Preparation.Actions)}
		projection, err := StepOriginalProjection(resetOriginalWorldDrawList(p.World.Projection), pc)
		if err != nil {
			return err
		}
		p.World.Projection = projection
		return nil
	}
	p.Match = CheckMatchPause(p.Match, uint8(input[0].Pressed)).State
	end := CheckMatchPeriodEnd(p.Match, MatchContext{Holder: uint8(p.World.Preparation.Actions.Holder), BallAirborne: p.World.Preparation.Actions.Ball.Airborne})
	p.Match = end.State
	r.SoundIDs = append(r.SoundIDs, end.SoundIDs...)
	worldStep, err := StepWorldLoop(p.World, runtimeWorldInput(s.Config, input), p.Context)
	if err != nil {
		return err
	}
	p.World = worldStep.State
	if e := observeRuntimeWorldDisplay(s, worldStep, p.Match); e != nil {
		return e
	}
	r.SoundIDs = append(r.SoundIDs, worldStep.SoundIDs...)
	courtSounds, e := observeRuntimeCourtAnimation(s, &p.World, p.Context, p.Match)
	if e != nil {
		return e
	}
	r.SoundIDs = append(r.SoundIDs, courtSounds...)
	if worldStep.HUD != nil {
		h := worldStep.HUD
		beforeClock := p.Match
		clock := StepMatchHUD(p.Match, MatchContext{FrameParity: h.LogicFrame & 1, Scores: h.Scores, Tactics: h.Tactics, TacticCache: p.TacticCache})
		p.Match = clock.State
		if h.LogicFrame&1 == 0 {
			display, e := ObserveDisplayHUD(s.Display, beforeClock, p.Match, h.Tactics)
			if e != nil {
				return e
			}
			s.Display = display
		}
		r.SoundIDs = append(r.SoundIDs, clock.SoundIDs...)
	}
	if p.Match.Phase == MatchPeriodEnding {
		opening, err := runtimeOpeningFromPlaying(*p, s.Config)
		if err != nil {
			return err
		}
		carry := p.World
		s.Carry = &carry
		s.Playing = nil
		s.Transition = &RuntimeTransitionState{Scene: opening}
	}
	return nil
}
func stepRuntimeTransition(r *RuntimeStep, input [4]ControllerFrame) error {
	s := &r.State
	o := &s.Transition.Scene
	t := &o.Transition
	if s.Outcome != nil && s.Outcome.Confirmed {
		return fmt.Errorf("unrestored post-result scene %d phase %d", t.Scene, t.Match.Phase)
	}
	t.CameraScroll[0], t.CameraScroll[1] = t.CameraScroll[2], t.CameraScroll[3]
	o.Projection = resetOriginalWorldDrawList(o.Projection)
	t.Random, _ = NextRandom(t.Random)
	if s.AwaitingNextPeriod {
		// Original Scene 2 menus are represented by the platform's committed
		// selections. Start confirms them; no automatic next-period countdown.
		if input[0].Pressed&ButtonStart == 0 {
			return nil
		}
		t.Scene = 1
		t.Match.Phase, t.Match.Entered = 0, false
		next, err := InitializeOpeningGrades(*o)
		if err != nil {
			return err
		}
		s.Opening = &next.State
		s.Transition = nil
		s.AwaitingNextPeriod = false
		return nil
	}
	var next MatchTransitionStep
	var err error
	switch t.Match.Phase {
	case MatchPeriodEnding:
		next, err = WaitOriginalPeriodEnd(*t)
	case 4:
		next, err = ResetOriginalPeriod(*t)
	case 6:
		initialResult := !t.Match.Entered
		var result OriginalResultStep
		result, err = StepOriginalResultPresentation(*o, uint8(input[0].Pressed))
		if err == nil {
			*o = result.State
			next = result.Transition
			if initialResult {
				s.Display = initializeRuntimeResultDisplay(s.Display)
				// BA40 loads the result background and fades it in before BC96
				// submits its first celebration sprites. The core actor update
				// is retained for the next normal submission after this wait.
				source := runtimeDisplaySource(*s)
				display, e := StageDisplay(s.Display, source)
				if e != nil {
					return e
				}
				display.Submitted.DrawOrder = []int{}
				display, e = QueueDisplayFade(display, false)
				if e != nil {
					return e
				}
				s.Display = display
				s.WaitVideoFrames = 8
				r.FadeIn = true
			}
			if s.Outcome == nil {
				s.Outcome = &RuntimeOutcome{WinningTeam: next.WinningTeam, Scores: t.Scores}
			}
			if t.Match.Phase != 6 {
				s.Outcome.Confirmed = true
				s.Outcome.NextScene = t.Scene
				s.Outcome.NextPhase = t.Match.Phase
			}
		}
	default:
		return fmt.Errorf("unrestored runtime transition phase %d", t.Match.Phase)
	}
	if err != nil {
		return err
	}
	*t = next.State
	r.SoundIDs = append(r.SoundIDs, next.SoundIDs...)
	r.FadeOut = next.FadeOut
	r.Backgrounds = next.Backgrounds
	if next.DelayCPUCycles > 0 {
		// NTSC CPU cycles per video frame are 29780.5. This preserves the source
		// busy-work duration without interpreting CPU/graphics interrupt overhead.
		s.WaitVideoFrames = uint16((uint64(next.DelayCPUCycles)*2 + 59560) / 59561)
	}
	if next.FadeOut {
		// The platform retains the final result instead of leaving for an
		// original menu. Do not darken that retained image after confirmation.
		if s.Outcome == nil || !s.Outcome.Confirmed {
			display, e := QueueDisplayFade(s.Display, true)
			if e != nil {
				return e
			}
			s.Display = display
		}
		s.WaitVideoFrames += 8
	}
	if next.NeedsTeamSelection && (s.Outcome == nil || !s.Outcome.Confirmed) {
		s.AwaitingNextPeriod = true
	}
	return nil
}

// ClearRuntimeInput is an explicit device/focus cancellation, unlike normal
// button release. It discards only one controller's unconsumed platform input;
// it never rewinds actions, resets rule memory, pauses time or clears another
// controller. Snapshot restore itself must not call this helper implicitly.
func ClearRuntimeInput(original RuntimeState, controller uint8) (RuntimeState, error) {
	return ClearRuntimeButtons(original, controller, 255)
}

// ClearRuntimeButtons cancels only the logical bits owned by a lost gesture or
// device. The platform computes the mask after combining keyboard/gamepad/
// touch sources, so another source holding the same button is preserved.
func ClearRuntimeButtons(original RuntimeState, controller uint8, mask Buttons) (RuntimeState, error) {
	if controller >= 4 {
		return original, fmt.Errorf("invalid runtime controller %d", controller)
	}
	if err := ValidateRuntimeState(original); err != nil {
		return original, err
	}
	s := original
	s.Held[controller] &^= mask
	s.PendingPressed[controller] &^= mask
	return s, nil
}
