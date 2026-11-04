package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

type fighter8CompositionAction struct {
	chuActionCall
	SourceRegister string `json:"source_register"`
}

type fighter8CompositionInput struct {
	chuInputCall
	SourceRegister string `json:"source_register"`
	RNGCalls       int    `json:"rng_calls"`
	ThreatBranches int    `json:"threat_branches"`
}

type fighter8CompositionActor struct {
	chuActorCall
	SourceRegister string `json:"source_register"`
	ThreatBranches int    `json:"threat_branches"`
}

func fighter8CompositionInputState(t *testing.T, raw chuInputSnapshot, id uint16) ChuInputState {
	t.Helper()
	state := decodeChuInput(t, raw, id)
	world := attackContactBytes(t, raw.AI.World, 4096)
	for i := range state.AI.Fighters {
		state.AI.Fighters[i].ChildAttackValue = world[i*1024+0x165]
	}
	return state
}

func fighter8CompositionView(t *testing.T, raw chuActorSnapshot, fixture chuActorViewFixture) ChuActorView {
	t.Helper()
	view := testActorView(t, raw, fixture)
	family := attackContactBytes(t, raw.FamilyWorld, 4096)
	for i := range view.Fighters {
		view.Fighters[i].ChildAttackValue = family[i*1024+0x165]
	}
	return view
}

func fighter8CompositionSource(t *testing.T, id uint16, register string, rawActor string) {
	t.Helper()
	if register != fmt.Sprintf("0xffff%04x", id) {
		t.Fatal("noncanonical original A5 source", register, id)
	}
	if attackContactBytes(t, rawActor, 256)[0x58] != 8 {
		t.Fatal("full composition did not use actual source8 object")
	}
}

func fighter8CompositionClear(t *testing.T, pc, address string) {
	t.Helper()
	if (pc != "0x37c56" && pc != "0x37cac") || address < "0xf80a" || address > "0xf80d" {
		t.Fatal("unidentified natural audio queue clear", pc, address)
	}
}

func checkFighter8CompositionAction(t *testing.T, row fighter8CompositionAction, library AnimationLibrary, commands CommandLibrary) {
	t.Helper()
	fighter8CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.Tail.ActorBytes)
	state, copy := decodeChuAction(t, row.Before, row.ObjectID), decodeChuAction(t, row.Before, row.ObjectID)
	got, err := StepFighterAction(state, 8, actionParameters(row.Before), commands, library)
	want, expected := decodeChuAction(t, row.After, row.ObjectID), row.After
	if len(row.Clears) != 0 {
		for _, clear := range row.Clears {
			fighter8CompositionClear(t, clear.PC, clear.Address)
		}
		want.SoundQueue, expected.Tail.SoundQueue = got.SoundQueue, got.SoundQueue
	}
	if err != nil || row.RNGCalls != 1 || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuAction(t, got, row.Before), expected) {
		t.Fatalf("full source8 action %s kind=%x differs: %v", row.Name, state.Body.ActionKind, err)
	}
	if !reflect.DeepEqual(state, copy) {
		t.Fatal("full action mutated caller storage")
	}
}

func checkFighter8CompositionInput(t *testing.T, row fighter8CompositionInput) {
	t.Helper()
	fighter8CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.AI.Actor)
	state, copy := fighter8CompositionInputState(t, row.Before, row.ObjectID), fighter8CompositionInputState(t, row.Before, row.ObjectID)
	got, err := StepFighterInput(state)
	want := fighter8CompositionInputState(t, row.After, row.ObjectID)
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuInput(t, got, row.Before), row.After) {
		t.Fatalf("full source8 input %s/%s differs: %v", row.Route, row.Name, err)
	}
	if !reflect.DeepEqual(state, copy) || got.Envelope.Controls != got.AI.Actor.Controls {
		t.Fatal("shared full input controls split or caller mutated")
	}
	if row.AICalls == 0 && (row.RNGCalls != 0 || got.AI.RandomSeed != state.AI.RandomSeed) {
		t.Fatal("non-AI input consumed RNG")
	}
}

func checkFighter8CompositionActor(t *testing.T, row fighter8CompositionActor, library AnimationLibrary, commands CommandLibrary) {
	t.Helper()
	fighter8CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.Action.Tail.ActorBytes)
	state, copy := decodeChuActor(t, row.Before, row.ObjectID), decodeChuActor(t, row.Before, row.ObjectID)
	view := fighter8CompositionView(t, row.Before, row.View)
	got, emitted, err := StepFighterActor(state, 8, view, commands, library)
	want, expected := decodeChuActor(t, row.After, row.ObjectID), row.After
	if len(row.Clears) != 0 {
		for _, clear := range row.Clears {
			fighter8CompositionClear(t, clear.PC, clear.Address)
		}
		want.World.SoundQueue, expected.Action.Tail.SoundQueue = got.World.SoundQueue, got.World.SoundQueue
	}
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuActor(t, got, view, emitted, row.Before), expected) {
		t.Fatalf("full source8 actor %s kind=%x differs: %v", row.Name, state.Fighter.Body.ActionKind, err)
	}
	if !emitted.DrawRequested || emitted.ObjectID != row.ObjectID || !reflect.DeepEqual(state, copy) {
		t.Fatal("actor draw missing or caller mutated")
	}
	active := row.View.Pause&128 == 0 && row.View.Slow == 0
	if active != (row.RNGCalls >= 1) {
		t.Fatal("actor active gate/RNG count differs")
	}
}

// These source8 tests precede production support in the complete action/actor
// APIs. Input already uses all-character AI. Captures use actual full returns;
// no production mock or reference-state interpreter is introduced.
func TestFighter8CompositionOriginalActions(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		seen := map[uint8]int{}
		count, _ := walkChuActor(t, "fighter8-composition-action-"+suffix, func(d *json.Decoder, library AnimationLibrary) {
			var row fighter8CompositionAction
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter8CompositionAction(t, row, library, commands)
			seen[decodeChuAction(t, row.Before, row.ObjectID).Body.ActionKind]++
		})
		if count == 0 {
			t.Fatal("missing full original action evidence")
		}
		if suffix == "boundaries" {
			for _, action := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 0x0f, 0x11, 0x3e, 0x3f, 0x40} {
				if seen[action] == 0 {
					t.Fatal("missing full action slot", action)
				}
			}
		}
		t.Logf("original full actions %s=%d slots=%v", suffix, count, seen)
	}
}

func TestFighter8CompositionOriginalInputs(t *testing.T) {
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		routes := map[string]int{}
		ai, threat := 0, 0
		count, _ := walkChuActor(t, "fighter8-composition-input-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter8CompositionInput
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter8CompositionInput(t, row)
			routes[row.Route]++
			ai += row.AICalls
			threat += row.ThreatBranches
		})
		if suffix == "boundaries" {
			for _, route := range []string{"disabled", "human", "ai", "human-record", "ai-record", "playback", "playback-stopped"} {
				if routes[route] == 0 {
					t.Fatal("missing full source8 input route", route)
				}
			}
			if ai == 0 {
				t.Fatal("private source8 original AI not actually executed")
			}
		} else if ai != 0 {
			t.Fatal("human source8 natural flow unexpectedly contains AI evidence")
		}
		t.Logf("original full inputs %s=%d AI calls=%d source8 threat branch=%d routes=%v", suffix, count, ai, threat, routes)
	}
}

func TestFighter8CompositionOriginalActors(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		identities := map[uint16]bool{}
		paused, slow := 0, 0
		count, _ := walkChuActor(t, "fighter8-composition-actor-"+suffix, func(d *json.Decoder, library AnimationLibrary) {
			var row fighter8CompositionActor
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter8CompositionActor(t, row, library, commands)
			identities[row.ObjectID] = true
			if row.View.Pause&128 != 0 {
				paused++
			}
			if row.View.Slow != 0 {
				slow++
			}
		})
		if suffix == "boundaries" && (len(identities) != 4 || paused == 0 || slow == 0) {
			t.Fatal("missing canonical identity or active gate coverage")
		}
		t.Logf("original full actors %s=%d identities=%v pause=%d slow=%d", suffix, count, identities, paused, slow)
	}
}

func TestFighter8CompositionActionStreams(t *testing.T) {
	commands := actionCommandLibrary(t)
	steps := 0
	count, external := walkChuActor(t, "fighter8-composition-action-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Initial chuActionSnapshot
			Calls   []fighter8CompositionAction
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete action stream", err)
		}
		state, encoded := decodeChuAction(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		for index, call := range stream.Calls {
			state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
			state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
			state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
			state.Controls.RawHeld = call.External.RawHeld
			if !reflect.DeepEqual(state, decodeChuAction(t, call.Before, call.ObjectID)) {
				t.Fatal("independent action stream entry diverged", index)
			}
			var err error
			state, err = StepFighterAction(state, 8, actionParameters(stream.Initial), commands, library)
			encoded = encodeChuAction(t, state, encoded)
			if err != nil || !reflect.DeepEqual(state, decodeChuAction(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("full action stream step%d differs: %v", index, err)
			}
			steps++
		}
	})
	if count < 5 || steps < count*2 || external != 0 {
		t.Fatal("missing independently initialized action streams")
	}
	t.Logf("full action streams=%d complete calls=%d", count, steps)
}

func TestFighter8CompositionInputAndActorStreams(t *testing.T) {
	inputSteps, actorSteps := 0, 0
	inputCount, external := walkChuActor(t, "fighter8-composition-input-sequences", func(d *json.Decoder, _ AnimationLibrary) {
		var stream struct {
			Initial chuInputSnapshot
			Calls   []fighter8CompositionInput
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete input stream", err)
		}
		state, encoded := fighter8CompositionInputState(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		for index, call := range stream.Calls {
			state.Envelope.Input = call.External
			var err error
			state, err = StepFighterInput(state)
			encoded = encodeChuInput(t, state, encoded)
			if err != nil || !reflect.DeepEqual(state, fighter8CompositionInputState(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("full input stream step%d differs: %v", index, err)
			}
			inputSteps++
		}
	})
	if inputCount < 2 || inputSteps < inputCount*2 || external != 0 {
		t.Fatal("missing full input streams")
	}
	commands := actionCommandLibrary(t)
	actorCount, external := walkChuActor(t, "fighter8-composition-actor-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Initial chuActorSnapshot
			View    chuActorViewFixture
			Calls   []fighter8CompositionActor
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete actor stream", err)
		}
		state, encoded := decodeChuActor(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		view := fighter8CompositionView(t, stream.Initial, stream.View)
		for index, call := range stream.Calls {
			if call.External == nil {
				t.Fatal("undeclared external actor input")
			}
			view.Controllers, view.Pause, view.Slow = call.External.Controllers, call.External.Pause, call.External.Slow
			var err error
			var emitted ChuActorEmissions
			state, emitted, err = StepFighterActor(state, 8, view, commands, library)
			encoded = encodeChuActor(t, state, view, emitted, encoded)
			if err != nil || !reflect.DeepEqual(state, decodeChuActor(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("full actor stream step%d differs: %v", index, err)
			}
			actorSteps++
		}
	})
	if actorCount < 3 || actorSteps < actorCount*2 || external != 0 {
		t.Fatal("missing composed actor/input/action streams")
	}
	t.Logf("full input streams=%d steps=%d; composed actor streams=%d steps=%d", inputCount, inputSteps, actorCount, actorSteps)
}

func TestFighter8CompositionFailuresAreAtomic(t *testing.T) {
	commands := actionCommandLibrary(t)
	checkedAction, checkedInput, checkedActor := false, false, false
	walkChuActor(t, "fighter8-composition-action-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter8CompositionAction
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedAction {
			return
		}
		base := decodeChuAction(t, row.Before, row.ObjectID)
		for _, rules := range []struct {
			Commands   CommandLibrary
			Animations AnimationLibrary
		}{{nil, library}, {commands, nil}} {
			got, err := StepFighterAction(base, 8, actionParameters(row.Before), rules.Commands, rules.Animations)
			if err == nil || !reflect.DeepEqual(got, base) {
				t.Fatal("early/late missing data leaked full action state")
			}
		}
		got, err := StepFighterAction(base, 255, actionParameters(row.Before), commands, library)
		if err == nil || !reflect.DeepEqual(got, base) {
			t.Fatal("unsupported source leaked command or action state")
		}
		checkedAction = true
	})
	walkChuActor(t, "fighter8-composition-input-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter8CompositionInput
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedInput || row.Route != "ai-record" {
			return
		}
		base := fighter8CompositionInputState(t, row.Before, row.ObjectID)
		base.Envelope.Position = base.Envelope.StreamBase + uint32(len(base.Envelope.Stream)) - 2
		copy := fighter8CompositionInputState(t, row.Before, row.ObjectID)
		copy.Envelope.Position = base.Envelope.Position
		got, err := StepFighterInput(base)
		if err == nil || !reflect.DeepEqual(got, base) || !reflect.DeepEqual(base, copy) {
			t.Fatal("late record failure leaked real AI or caller stream writes")
		}
		checkedInput = true
	})
	walkChuActor(t, "fighter8-composition-actor-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter8CompositionActor
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedActor || row.View.Pause&128 != 0 || row.View.Slow != 0 {
			return
		}
		base := decodeChuActor(t, row.Before, row.ObjectID)
		if base.Fighter.InputMode != -1 || base.Fighter.InputChannel != 255 {
			return
		}
		base.World.HUDCells = nil
		copy := decodeChuActor(t, row.Before, row.ObjectID)
		copy.World.HUDCells = nil
		got, emitted, err := StepFighterActor(base, 8, fighter8CompositionView(t, row.Before, row.View), commands, library)
		if err == nil || !reflect.DeepEqual(got, base) || !reflect.DeepEqual(base, copy) || emitted.DrawRequested || len(emitted.HUDTransfers) != 0 {
			t.Fatal("late HUD failure leaked registration, AI, recording, action, bounds or events")
		}
		checkedActor = true
	})
	if !checkedAction || !checkedInput || !checkedActor {
		t.Fatal("missing real cases for full-API atomic checks")
	}
}
