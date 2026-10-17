package engine

import "fmt"

// HumanCommandState keeps the original teammate-command windows and message
// timers. It is separate from controller chord memory and team AI strategy.
type HumanCommandState struct {
	// The even-seat byte adjacent to Status in original 0657..065A.
	EvenStatus          int8 `json:"evenStatus,omitempty"`
	Request             uint8
	Status              int8
	ButtonWindows       [2]uint8
	Timers              [2]int8
	Messages, Overrides [2]uint8
}

type WorldInputState struct {
	World         WorldActionState
	Team          TeamAIState
	Memory        [4]InputMemory
	Held, Pressed [4]Buttons
	Human         [2]HumanCommandState
}

type WorldInputContext struct {
	PursuitReserved  *PursuitReservedBytes
	PursuitAlias74   *PursuitAlias74
	World            WorldActionContext
	Frames           [4]ControllerFrame
	Difficulty       uint8
	ScoreDifference  uint8
	CachedZones      [4]CourtZone
	ObjectKinds      [13]uint8
	HoopLinks        [6]uint8
	HumanTactics     [2]uint8
	PickupFlags      [4]uint8
	JumpEnhancements [4]uint8
}

type WorldInputStep struct {
	State             WorldInputState
	Calls             []uint8
	ChangePickupFlags [4]bool
	PickupFlags       [4]uint8
}

func worldInputAttachment(s WorldActionState, ctx WorldInputContext, object uint8) (uint8, error) {
	if object < 7 {
		return s.actionLinks()[object], nil
	}
	if object < 13 {
		return ctx.HoopLinks[object-7], nil
	}
	return 0, fmt.Errorf("input attachment object %d not reconstructed", object)
}
func worldInputAirborne(s WorldActionState, object uint8) (bool, error) {
	switch {
	case object < 4:
		return s.Actors[object].Player.Airborne, nil
	case object < 6:
		return s.Auxiliary[object-4].Airborne, nil
	case object == 6:
		return s.Ball.Airborne, nil
	case object < 13:
		return s.Hoops[object-7].Airborne, nil
	}
	return false, fmt.Errorf("invalid input parent object %d", object)
}
func setWorldInputRequest(s *WorldActionState, object uint8, action Action) error {
	switch {
	case object < 4:
		s.Actors[object].Player.Request = ActionRequest{Action: action, Pending: true}
	case object < 6:
		s.Auxiliary[object-4].Request = ActionRequest{Action: action, Pending: true}
	case object == 6:
		s.Ball.Request = BallRequest{Action: BallAction(action), Pending: true}
	case object < 13:
		s.Hoops[object-7].Request = HoopRequest{Action: HoopAction(action), Pending: true}
	default:
		return fmt.Errorf("invalid released carrier request target %d", object)
	}
	return nil
}

// originalActorInputPrelude restores $8000..$804B without changing the
// scheduler's processed bits or performing any actor movement.
func originalActorInputPrelude(s *WorldInputState, actor uint8, ctx WorldInputContext) error {
	w := &s.World
	a := &w.Actors[actor]
	if a.LinkedActor >= 0 {
		link, err := worldInputAttachment(*w, ctx, uint8(a.LinkedActor))
		if err != nil {
			return err
		}
		if link&128 == 0 {
			a.LinkedActor = -1
		}
	}
	attachment := w.PlayerLinks[actor]
	keepJumpPhase := false
	if attachment&128 == 0 {
		keepJumpPhase = a.Player.Airborne
	} else {
		parent := attachment & 15
		if parent >= 13 {
			return fmt.Errorf("invalid attached input parent %d", parent)
		}
		parentLink, err := worldInputAttachment(*w, ctx, parent)
		if err != nil {
			return err
		}
		if ctx.ObjectKinds[parent] == 12 && parentLink&128 == 0 {
			keepJumpPhase, err = worldInputAirborne(*w, parent)
			if err != nil {
				return err
			}
		}
	}
	if !keepJumpPhase {
		a.JumpPhase = 0
	}
	if w.CarriedPlayer == int8(actor) && attachment&128 == 0 {
		if err := setWorldInputRequest(w, attachment&15, ActionIdle); err != nil {
			return err
		}
		w.CarriedPlayer = -1
	}
	return nil
}

var originalHumanCommandMessages = [6][4]uint8{{4, 3, 5, 6}, {4, 3, 5, 6}, {7, 3, 5, 8}, {7, 9, 5, 10}, {7, 3, 5, 11}, {7, 3, 5, 6}}

func advanceOriginalHumanCommands(s *WorldInputState, actor uint8, ctx WorldInputContext) error {
	if actor&1 != 0 || ctx.World.Controls[actor+1]&128 == 0 {
		return nil
	}
	h := &s.Human[actor/2]
	for i := range h.Timers {
		if h.Timers[i] >= 0 {
			h.Timers[i]--
			if h.Timers[i] < 0 {
				h.Messages[i] = 0
			} else if h.Overrides[i] != 0 {
				h.Messages[i] = h.Overrides[i]
			}
		}
	}
	if s.World.Holder == int8(actor) || h.Status == -1 {
		return nil
	}
	frame := ctx.Frames[actor]
	for i, button := range [2]Buttons{ButtonA, ButtonB} {
		if frame.Pressed&button != 0 {
			h.ButtonWindows[i] = 3
			continue
		}
		if h.ButtonWindows[i] == 0 {
			continue
		}
		if frame.Held&button == 0 {
			h.ButtonWindows[i] = 0
			continue
		}
		h.ButtonWindows[i]--
		if h.ButtonWindows[i] == 0 || int8(h.ButtonWindows[i]) < 0 {
			h.Request = uint8(i + 1)
		}
	}
	if h.Request != 0 {
		command := h.Request & 127
		if command < 1 || command > 2 || ctx.HumanTactics[actor/2] >= 6 {
			return fmt.Errorf("human teammate command profile not reconstructed")
		}
		if s.World.Holder != int8(actor+1) {
			command += 2
		}
		h.Messages[0] = originalHumanCommandMessages[ctx.HumanTactics[actor/2]][command-1]
		h.Messages[1] = 0
		h.Timers[0] = 16
	}
	return nil
}

func advanceWorldHumanInput(s *WorldInputState, actor uint8, ctx WorldInputContext) error {
	_, err := advanceWorldHumanInputWithEffects(s, actor, ctx)
	return err
}

func advanceWorldComputerInput(s *WorldInputState, actor uint8, ctx WorldInputContext) error {
	w := &s.World
	a := &w.Actors[actor]
	p := a.Player
	options, err := OriginalPursuitOptions(ctx.World.Teams[actor/2], ctx.World.Characters[actor])
	if err != nil {
		return err
	}
	v := AIControlView{Actor: actor, Action: p.Action, ActionEntered: p.Entered, Heading: p.Heading, FacingLeft: p.FacingLeft, Position: p.Body.Position, HasBall: w.Holder == int8(actor), BallPosition: w.Ball.Body.Position, BallHolder: w.Holder, Random: w.Random, BallAction: w.Ball.Action, BallFacingLeft: w.Ball.FacingLeft, BallLanding: w.BallLanding, PursuitOptions: &options, Team: ctx.World.Teams[actor/2], Difficulty: ctx.Difficulty, CourtID: ctx.World.CourtID, CachedZones: &ctx.CachedZones, ShotContext: AIShotContext{MountFlags: &w.MountFlags, JumpVelocity: ctx.World.Profiles[actor].JumpVelocity, Character: ctx.World.Characters[actor], Court: ctx.World.CourtID, JumpPhase: a.JumpPhase, VerticalSpeed: int8(p.Body.Velocity.Height >> 8), HorizontalSpeed: int8(p.Body.Velocity.X >> 8), ShotSkill: ctx.World.Profiles[actor].ShotSkill}, TeammateLink: w.PlayerLinks[actor^1], TeamHasHuman: ctx.World.Controls[actor&2]&ctx.World.Controls[(actor&2)+1]&128 == 0, Airborne: p.Airborne}
	for i, other := range w.Actors {
		v.Players[i], v.PlayerActions[i] = other.Player.Body.Position, other.Player.Action
	}
	v.ScoreDifference = ctx.ScoreDifference
	v.Auxiliary = &w.Auxiliary
	v.PursuitAlias74 = ctx.PursuitAlias74
	v.PursuitWorld, v.PlayerControls, v.PursuitReserved = w, &ctx.World.Controls, ctx.PursuitReserved
	v.HumanTactics = &ctx.HumanTactics
	v.TeamAI = &s.Team
	for i, h := range w.Hoops {
		v.Hoops[i] = AIHoopView{Position: h.Position, Action: h.Action, Owner: int8(h.Owner)}
	}
	decision, err := StepOriginalAIControl(s.Team.Players[actor], v)
	if err != nil {
		return err
	}
	if decision.TeammateControl.Change && decision.TeammateControl.Actor >= 4 {
		return fmt.Errorf("AI teammate command is outside player seats")
	}
	if decision.ClearCommandStatus {
		if actor&1 == 0 {
			s.Human[actor/2].EvenStatus = 0
		} else {
			s.Human[actor/2].Status = 0
		}
	}
	applied := ApplyOriginalAIActorDecision(ComputerActorState{Carrier: HoopCarrier{Player: p, ShotCycle: a.ShotCycle, AimWindow: a.AimWindow}, Control: s.Team.Players[actor], JumpPhase: a.JumpPhase, Dodge: a.Dodge}, decision)
	a.Player, a.ShotCycle, a.AimWindow, a.JumpPhase, a.Dodge = applied.Carrier.Player, applied.Carrier.ShotCycle, applied.Carrier.AimWindow, applied.JumpPhase, applied.Dodge
	s.Team.Players[actor] = applied.Control
	if command := decision.TeammateControl; command.Change {
		peer := &s.Team.Players[command.Actor]
		peer.Mode, peer.Entered = command.Mode, false
		if command.ChangePlan {
			peer.Plan = command.Plan
		}
	}
	w.Random = decision.Random
	if decision.BindHoop >= 0 {
		if decision.BindHoop >= 6 {
			return fmt.Errorf("AI hoop target outside original object set")
		}
		w.Hoops[decision.BindHoop].Owner = actor
	}
	if decision.BindBall {
		w.Holder = int8(actor)
	}
	w.Ball = ApplyBallCommand(w.Ball, decision.Ball)
	return nil
}

// StepWorldInputs restores D32C..D345. It executes each actor against the live
// updated world, then performs the one owned-ball input check at object six.
// No action scheduler, animation, contact resolution or physics runs here.
func StepWorldInputs(original WorldInputState, ctx WorldInputContext) (WorldInputStep, error) {
	initial := WorldInputStep{State: original, PickupFlags: ctx.PickupFlags}
	result := initial
	for actor := uint8(0); actor < 4; actor++ {
		result.Calls = append(result.Calls, actor)
		if err := originalActorInputPrelude(&result.State, actor, ctx); err != nil {
			return initial, fmt.Errorf("actor %d prelude: %w", actor, err)
		}
		var err error
		if ctx.World.Controls[actor]&128 == 0 {
			var effects HumanInputEffects
			effects, err = advanceWorldHumanInputWithEffects(&result.State, actor, ctx)
			if effects.ChangePickupFlags {
				result.ChangePickupFlags[actor] = true
				result.PickupFlags[actor] = effects.PickupFlags
			}
		} else if ctx.World.Controls[actor]&64 == 0 {
			err = advanceWorldComputerInput(&result.State, actor, ctx)
		}
		if err != nil {
			return initial, fmt.Errorf("actor %d input: %w", actor, err)
		}
	}
	for object := uint8(4); object < 10; object++ {
		result.Calls = append(result.Calls, object)
		if object != 6 {
			continue
		}
		w := &result.State.World
		if w.Holder == 6 {
			continue
		}
		if w.Holder < 0 || w.Holder >= 4 {
			return initial, fmt.Errorf("object input for ball holder %d not reconstructed", w.Holder)
		}
		w.Ball = StepOwnedBallInput(w.Ball, &w.Actors[w.Holder].Player)
	}
	return result, nil
}
