// Package sessiontick applies one authoritative/replayed input boundary using
// independent rules and bounded pixel-free presentation metadata.
package sessiontick

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

// Advance changes controller assignment only at this effective tick. Transport
// latches do not own World.Controllers or fighter rule history; neither is reset.
// Automation is host metadata carried in the same replay record as held input.
func Advance(world engine.ChuWorldState, presentation renderstate.State, options engine.ChuSessionOptions, held [4]engine.Buttons, automated uint8, rules engine.ChuWorldLibrary, graphics renderstate.Library) (engine.ChuWorldState, renderstate.State, []engine.ChuWorldEvent, engine.WorldFrameOutcome, error) {
	return AdvanceObserved(world, presentation, options, held, automated, rules, graphics, nil)
}

// AdvanceObserved exposes transient sound producers within the same rule and
// presentation transaction. Callers publish observations only after success.
func AdvanceObserved(world engine.ChuWorldState, presentation renderstate.State, options engine.ChuSessionOptions, held [4]engine.Buttons, automated uint8, rules engine.ChuWorldLibrary, graphics renderstate.Library, sound engine.WorldSoundHook) (engine.ChuWorldState, renderstate.State, []engine.ChuWorldEvent, engine.WorldFrameOutcome, error) {
	before, beforePresentation := world, presentation
	fail := func(err error) (engine.ChuWorldState, renderstate.State, []engine.ChuWorldEvent, engine.WorldFrameOutcome, error) {
		return before, beforePresentation, nil, engine.WorldFrameOutcome{}, err
	}
	n := len(options.Players)
	if n < 2 || n > 4 || automated&^uint8((1<<n)-1) != 0 || world.Match.Arena != options.Arena || options.TeamBattle != (world.Match.Grouping == 4) {
		return fail(errors.New("invalid shared tick configuration"))
	}
	slots := []uint8{0, 3}
	if n == 3 {
		slots = []uint8{0, 1, 3}
	} else if n == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	for seat, buttons := range held {
		if err := engine.ValidateControllerButtons(buttons); err != nil {
			return fail(err)
		}
		if seat >= n && buttons != 0 {
			return fail(errors.New("unoccupied input channel has held buttons"))
		}
	}
	for seat, p := range options.Players {
		if p.Slot != slots[seat] || p.HumanChannel != uint8(seat) || p.CharacterID > 10 || world.Match.Characters[p.Slot] != p.CharacterID+1 {
			return fail(errors.New("shared tick seat/slot/character differs"))
		}
	}
	if world.MatchComplete {
		return world, presentation, nil, engine.WorldFrameOutcome{}, nil
	}
	// Only Match.Controllers and the fighter's existing channel field are
	// written here. Each is its own original owner, not a duplicate host latch.
	world.Objects = make(engine.WorldObjects, len(before.Objects))
	for id, object := range before.Objects {
		world.Objects[id] = object
	}
	for seat, p := range options.Players {
		channel, assignment := uint8(seat), uint8(seat+1)
		if automated&(1<<seat) != 0 {
			channel, assignment = 255, 0
		}
		world.Match.Controllers[p.Slot] = assignment
		id := uint16(0x8400) + uint16(p.Slot)*0x400
		object, ok := world.Objects[id]
		if ok && object.Header.Behavior == 0x12668 {
			object.InputChannel = channel
			world.Objects[id] = object
		}
	}
	var input [8]engine.Buttons
	copy(input[:4], held[:])
	next, events, outcome, err := engine.StepChuSessionFrameObserved(world, input, rules, sound)
	if err != nil {
		return fail(err)
	}
	if outcome.ResetRequested {
		return fail(errors.New("independent match reset requested"))
	}
	next, visual, err := renderstate.Advance(next, presentation, events, graphics)
	if err != nil {
		return fail(err)
	}
	return next, visual, events, outcome, nil
}
