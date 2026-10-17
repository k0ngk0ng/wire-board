package engine

import "errors"

const ActionDodge Action = 0x2e

// DodgeMemory preserves the original entry-speed sign and acceleration cursor.
// The initial speed is a signed integer byte; the original discards its fraction.
type DodgeMemory struct {
	Step         uint8 `json:"step"`
	InitialXHigh int8  `json:"initialXHigh"`
}

type DodgeStep struct {
	CarrierStep
	Memory DodgeMemory
}

// StepCarrierDodge restores bank 6 $949F. Despite its name the same action is
// valid without a ball, and then returns to ordinary running instead of dribbling.
func StepCarrierDodge(original Carrier, memory DodgeMemory, ctx CarrierContext, profile CharacterProfile, court FlatCourt) (DodgeStep, error) {
	initial := DodgeStep{CarrierStep: CarrierStep{Carrier: original}, Memory: memory}
	result := initial
	p := &result.Carrier.Player
	if p.Action != ActionDodge {
		return initial, errors.New("player is not dodging")
	}
	// E18B retains all rejected requests, including an old CatchBall request
	// when AI changes the action directly. Only accepted cross-handler requests
	// require the world dispatcher; this component must not clear stale input.
	resolved, err := ResolvePlayerRequest(*p)
	if err != nil {
		return initial, err
	}
	if resolved.Action != ActionDodge {
		return initial, errors.New("dodge request requires another action handler")
	}
	*p = resolved
	if !p.Entered {
		initializeAction(p)
		result.Memory = DodgeMemory{InitialXHigh: int8(p.Body.Velocity.X >> 8)}
		p.Animation.NextAction, p.Animation.HasNextAction = ActionRun, true
		if ctx.HasBall {
			p.Animation.NextAction = ActionDribbleRun
		}
		depth := -4 * Pixel
		if p.Heading == HeadingUp {
			depth = 4 * Pixel
		}
		p.Body.Velocity.Depth = depth + (p.Body.Velocity.Depth & 255)
	}
	if result.Memory.Step >= 5 {
		return initial, errors.New("dodge acceleration cursor outside original table")
	}
	if err := animatePlayer(p, profile, ActionDodge); err != nil {
		return initial, err
	}
	speed := Fixed([5]int8{0, 2, 4, 6, 8}[result.Memory.Step]) * Pixel
	if result.Memory.InitialXHigh < 0 {
		speed = -speed
	}
	p.Body.Velocity.X = speed
	result.Memory.Step++
	if result.Memory.Step >= 5 {
		p.Body.Velocity.X = Fixed(result.Memory.InitialXHigh) * Pixel
	}
	terrain, err := advancePlayerOnCourt(p, playerTerrainContext{HasBall: ctx.HasBall, Computer: ctx.Computer}, court)
	if err != nil {
		return initial, err
	}
	result.Ball = terrain
	return result, nil
}
