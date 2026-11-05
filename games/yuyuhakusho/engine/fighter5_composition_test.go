package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func fighter5CompositionActionState(t *testing.T, raw chuActionSnapshot, id uint16) ChuActionState {
	t.Helper()
	state := decodeChuAction(t, raw, id)
	children := attackContactBytes(t, raw.Children, 768)
	for i := range state.Fighter5ChildExtras {
		b := children[i*256:]
		state.Fighter5ChildExtras[i] = Fighter5ChildExtra{Gravity: int32(binary.BigEndian.Uint32(b[0x4c:])), Owner: binary.BigEndian.Uint16(b[0xfe:])}
		// Source5 owns +FE as child Owner, not the source4 DragonBank view.
		state.ChildExtras[i].DragonBank = 0
	}
	return state
}

func fighter5CompositionEncodeAction(t *testing.T, state ChuActionState, previous chuActionSnapshot) chuActionSnapshot {
	t.Helper()
	raw := encodeChuAction(t, state, previous)
	children := attackContactBytes(t, raw.Children, 768)
	for i, extra := range state.Fighter5ChildExtras {
		b := children[i*256:]
		binary.BigEndian.PutUint32(b[0x4c:], uint32(extra.Gravity))
		binary.BigEndian.PutUint16(b[0xfe:], extra.Owner)
	}
	raw.Children = hex.EncodeToString(children)
	raw.Tail.ChildBytes = hex.EncodeToString(children[:256])
	return raw
}

func fighter5CompositionActorState(t *testing.T, raw chuActorSnapshot, id uint16) ChuActorState {
	t.Helper()
	state := decodeChuActor(t, raw, id)
	action := fighter5CompositionActionState(t, raw.Action, id)
	state.Fighter.Fighter5ChildExtras = action.Fighter5ChildExtras
	state.Fighter.ChildExtras = action.ChildExtras
	return state
}

func fighter5CompositionEncodeActor(t *testing.T, state ChuActorState, view ChuActorView, emitted ChuActorEmissions, previous chuActorSnapshot) chuActorSnapshot {
	t.Helper()
	raw := encodeChuActor(t, state, view, emitted, previous)
	children := attackContactBytes(t, raw.Action.Children, 768)
	for i, extra := range state.Fighter.Fighter5ChildExtras {
		b := children[i*256:]
		binary.BigEndian.PutUint32(b[0x4c:], uint32(extra.Gravity))
		binary.BigEndian.PutUint16(b[0xfe:], extra.Owner)
	}
	raw.Action.Children, raw.Action.Tail.ChildBytes = hex.EncodeToString(children), hex.EncodeToString(children[:256])
	family := attackContactBytes(t, raw.FamilyWorld, 4096)
	copy(family[int(state.Fighter.ObjectID)-0x8400+256:], children)
	raw.FamilyWorld = hex.EncodeToString(family)
	return raw
}

func fighter5CompositionAliases(t *testing.T, state ChuActionState, raw chuActionSnapshot) {
	t.Helper()
	a := attackContactBytes(t, raw.Tail.ActorBytes, 256)
	c := attackContactBytes(t, raw.Children, 768)
	if state.MovementSource != binary.BigEndian.Uint32(a[0x90:]) || state.AlternateVelocity != int32(binary.BigEndian.Uint32(a[0x48:])) || state.fullActionEnergy() != int32(binary.BigEndian.Uint32(a[0xb8:])) || state.Power != int16(binary.BigEndian.Uint16(a[0x7c:])) {
		t.Fatal("source5 canonical movement/resource views missing")
	}
	for i, extra := range state.Fighter5ChildExtras {
		b := c[i*256:]
		if extra.Gravity != int32(binary.BigEndian.Uint32(b[0x4c:])) || extra.Owner != binary.BigEndian.Uint16(b[0xfe:]) || state.ChildExtras[i].DragonBank != 0 {
			t.Fatal("source5 child gravity/owner not decoded from actual offsets", i)
		}
	}
	probe := state
	probe.MovementSource = 0
	probe.AlternateVelocity ^= 0x123456
	probe.EnergyFraction ^= 0x4321
	probe.Power ^= 0x1234
	probe.Fighter5ChildExtras[0].Gravity ^= 0x5a123456
	probe.Fighter5ChildExtras[0].Owner ^= 0x1234
	encoded := fighter5CompositionEncodeAction(t, probe, raw)
	aa := attackContactBytes(t, encoded.Tail.ActorBytes, 256)
	cc := attackContactBytes(t, encoded.Children, 768)
	if binary.BigEndian.Uint32(aa[0x90:]) != 0 || int32(binary.BigEndian.Uint32(aa[0x48:])) != probe.AlternateVelocity || binary.BigEndian.Uint16(aa[0xba:]) != probe.EnergyFraction || int16(binary.BigEndian.Uint16(aa[0x7c:])) != probe.Power || int32(binary.BigEndian.Uint32(cc[0x4c:])) != probe.Fighter5ChildExtras[0].Gravity || binary.BigEndian.Uint16(cc[0xfe:]) != probe.Fighter5ChildExtras[0].Owner {
		t.Fatal("source5 encoder hides owned writes behind previous raw bytes")
	}
}

type fighter5CompositionAction struct {
	chuActionCall
	SourceRegister string `json:"source_register"`
}

type fighter5CompositionInput struct {
	chuInputCall
	SourceRegister string `json:"source_register"`
	RNGCalls       int    `json:"rng_calls"`
	ThreatBranches int    `json:"threat_branches"`
}

type fighter5CompositionActor struct {
	chuActorCall
	SourceRegister string `json:"source_register"`
	ThreatBranches int    `json:"threat_branches"`
}

func fighter5CompositionInputState(t *testing.T, raw chuInputSnapshot, id uint16) ChuInputState {
	t.Helper()
	state := decodeChuInput(t, raw, id)
	world := attackContactBytes(t, raw.AI.World, 4096)
	for i := range state.AI.Fighters {
		state.AI.Fighters[i].ChildAttackValue = world[i*1024+0x165]
	}
	return state
}

func fighter5CompositionView(t *testing.T, raw chuActorSnapshot, fixture chuActorViewFixture) ChuActorView {
	t.Helper()
	view := testActorView(t, raw, fixture)
	family := attackContactBytes(t, raw.FamilyWorld, 4096)
	for i := range view.Fighters {
		view.Fighters[i].ChildAttackValue = family[i*1024+0x165]
	}
	return view
}

func fighter5CompositionSource(t *testing.T, id uint16, register string, rawActor string) {
	t.Helper()
	if register != fmt.Sprintf("0xffff%04x", id) {
		t.Fatal("noncanonical original A5 source", register, id)
	}
	if attackContactBytes(t, rawActor, 256)[0x58] != 5 {
		t.Fatal("full composition did not use actual source5 object")
	}
}

func fighter5CompositionClear(t *testing.T, pc, address string) {
	t.Helper()
	if (pc != "0x37c56" && pc != "0x37cac") || address < "0xf80a" || address > "0xf80d" {
		t.Fatal("unidentified natural audio queue clear", pc, address)
	}
}

func checkFighter5CompositionAction(t *testing.T, row fighter5CompositionAction, library AnimationLibrary, commands CommandLibrary) {
	t.Helper()
	fighter5CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.Tail.ActorBytes)
	state, copy := fighter5CompositionActionState(t, row.Before, row.ObjectID), fighter5CompositionActionState(t, row.Before, row.ObjectID)
	fighter5CompositionAliases(t, state, row.Before)
	got, err := StepFighterAction(state, 5, actionParameters(row.Before), commands, library)
	want, expected := fighter5CompositionActionState(t, row.After, row.ObjectID), row.After
	if len(row.Clears) != 0 {
		for _, clear := range row.Clears {
			fighter5CompositionClear(t, clear.PC, clear.Address)
		}
		want.SoundQueue, expected.Tail.SoundQueue = got.SoundQueue, got.SoundQueue
	}
	if err != nil || row.RNGCalls != 1 || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(fighter5CompositionEncodeAction(t, got, row.Before), expected) {
		t.Fatalf("full source5 action %s kind=%x differs: %v", row.Name, state.Body.ActionKind, err)
	}
	if !reflect.DeepEqual(state, copy) {
		t.Fatal("full action mutated caller storage")
	}
}

func checkFighter5CompositionInput(t *testing.T, row fighter5CompositionInput) {
	t.Helper()
	fighter5CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.AI.Actor)
	state, copy := fighter5CompositionInputState(t, row.Before, row.ObjectID), fighter5CompositionInputState(t, row.Before, row.ObjectID)
	got, err := StepFighterInput(state)
	want := fighter5CompositionInputState(t, row.After, row.ObjectID)
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuInput(t, got, row.Before), row.After) {
		t.Fatalf("full source5 input %s/%s differs: %v", row.Route, row.Name, err)
	}
	if !reflect.DeepEqual(state, copy) || got.Envelope.Controls != got.AI.Actor.Controls {
		t.Fatal("shared full input controls split or caller mutated")
	}
	if row.AICalls == 0 && (row.RNGCalls != 0 || got.AI.RandomSeed != state.AI.RandomSeed) {
		t.Fatal("non-AI input consumed RNG")
	}
}

func checkFighter5CompositionActor(t *testing.T, row fighter5CompositionActor, library AnimationLibrary, commands CommandLibrary) {
	t.Helper()
	fighter5CompositionSource(t, row.ObjectID, row.SourceRegister, row.Before.Action.Tail.ActorBytes)
	state, copy := fighter5CompositionActorState(t, row.Before, row.ObjectID), fighter5CompositionActorState(t, row.Before, row.ObjectID)
	if state.Fighter.MovementSource != binary.BigEndian.Uint32(attackContactBytes(t, row.Before.Action.Tail.ActorBytes, 256)[0x90:]) || state.World.DragonBankCursor != row.Before.Action.GlobalA038 {
		t.Fatal("actor source5 views missing canonical movement/history cursor")
	}
	view := fighter5CompositionView(t, row.Before, row.View)
	got, emitted, err := StepFighterActor(state, 5, view, commands, library)
	want, expected := fighter5CompositionActorState(t, row.After, row.ObjectID), row.After
	if len(row.Clears) != 0 {
		for _, clear := range row.Clears {
			fighter5CompositionClear(t, clear.PC, clear.Address)
		}
		want.World.SoundQueue, expected.Action.Tail.SoundQueue = got.World.SoundQueue, got.World.SoundQueue
	}
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(fighter5CompositionEncodeActor(t, got, view, emitted, row.Before), expected) {
		t.Fatalf("full source5 actor %s kind=%x differs: %v", row.Name, state.Fighter.Body.ActionKind, err)
	}
	if !emitted.DrawRequested || emitted.ObjectID != row.ObjectID || !reflect.DeepEqual(state, copy) {
		t.Fatal("actor draw missing or caller mutated")
	}
	active := row.View.Pause&128 == 0 && row.View.Slow == 0
	if active != (row.RNGCalls >= 1) {
		t.Fatal("actor active gate/RNG count differs")
	}
}

// These source5 tests precede production support in the complete action/actor
// APIs. Input already uses all-character AI. Captures use actual full returns;
// no production mock or reference-state interpreter is introduced.
func TestFighter5CompositionOriginalActions(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		seen := map[uint8]int{}
		count, _ := walkChuActor(t, "fighter5-composition-action-"+suffix, func(d *json.Decoder, library AnimationLibrary) {
			var row fighter5CompositionAction
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter5CompositionAction(t, row, library, commands)
			seen[fighter5CompositionActionState(t, row.Before, row.ObjectID).Body.ActionKind]++
		})
		if count == 0 {
			t.Fatal("missing full original action evidence")
		}
		if suffix == "boundaries" {
			for _, action := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 0x0f, 0x11, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32} {
				if seen[action] == 0 {
					t.Fatal("missing full action slot", action)
				}
			}
		}
		t.Logf("original full actions %s=%d slots=%v", suffix, count, seen)
	}
}

func TestFighter5CompositionOriginalInputs(t *testing.T) {
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		routes := map[string]int{}
		ai, threat := 0, 0
		count, _ := walkChuActor(t, "fighter5-composition-input-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter5CompositionInput
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter5CompositionInput(t, row)
			routes[row.Route]++
			ai += row.AICalls
			threat += row.ThreatBranches
		})
		if suffix == "boundaries" {
			for _, route := range []string{"disabled", "human", "ai", "human-record", "ai-record", "playback", "playback-stopped"} {
				if routes[route] == 0 {
					t.Fatal("missing full source5 input route", route)
				}
			}
			if ai == 0 {
				t.Fatal("private source5 original AI not actually executed")
			}
		} else if ai != 0 {
			t.Fatal("human source5 natural flow unexpectedly contains AI evidence")
		}
		t.Logf("original full inputs %s=%d AI calls=%d source5 threat branch=%d routes=%v", suffix, count, ai, threat, routes)
	}
}

func TestFighter5CompositionOriginalActors(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, suffix := range []string{"natural", "special", "boundaries"} {
		identities := map[uint16]bool{}
		paused, slow := 0, 0
		count, _ := walkChuActor(t, "fighter5-composition-actor-"+suffix, func(d *json.Decoder, library AnimationLibrary) {
			var row fighter5CompositionActor
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			checkFighter5CompositionActor(t, row, library, commands)
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

func TestFighter5CompositionActionStreams(t *testing.T) {
	commands := actionCommandLibrary(t)
	steps := 0
	count, external := walkChuActor(t, "fighter5-composition-action-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Initial chuActionSnapshot
			Calls   []fighter5CompositionAction
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete action stream", err)
		}
		state, encoded := fighter5CompositionActionState(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		for index, call := range stream.Calls {
			state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
			state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
			state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
			state.Controls.RawHeld = call.External.RawHeld
			if !reflect.DeepEqual(state, fighter5CompositionActionState(t, call.Before, call.ObjectID)) {
				t.Fatal("independent action stream entry diverged", index)
			}
			var err error
			state, err = StepFighterAction(state, 5, actionParameters(stream.Initial), commands, library)
			encoded = fighter5CompositionEncodeAction(t, state, encoded)
			if err != nil || !reflect.DeepEqual(state, fighter5CompositionActionState(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("full action stream step%d differs: %v", index, err)
			}
			steps++
		}
	})
	if count < 10 || steps < count*2 || external != 0 {
		t.Fatal("missing independently initialized action streams")
	}
	t.Logf("full action streams=%d complete calls=%d", count, steps)
}

func TestFighter5CompositionInputAndActorStreams(t *testing.T) {
	inputSteps, actorSteps := 0, 0
	inputCount, external := walkChuActor(t, "fighter5-composition-input-sequences", func(d *json.Decoder, _ AnimationLibrary) {
		var stream struct {
			Initial chuInputSnapshot
			Calls   []fighter5CompositionInput
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete input stream", err)
		}
		state, encoded := fighter5CompositionInputState(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		for index, call := range stream.Calls {
			state.Envelope.Input = call.External
			var err error
			state, err = StepFighterInput(state)
			encoded = encodeChuInput(t, state, encoded)
			if err != nil || !reflect.DeepEqual(state, fighter5CompositionInputState(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("full input stream step%d differs: %v", index, err)
			}
			inputSteps++
		}
	})
	if inputCount < 2 || inputSteps < inputCount*2 || external != 0 {
		t.Fatal("missing full input streams")
	}
	commands := actionCommandLibrary(t)
	actorCount, external := walkChuActor(t, "fighter5-composition-actor-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Initial chuActorSnapshot
			View    chuActorViewFixture
			Calls   []fighter5CompositionActor
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("invalid complete actor stream", err)
		}
		state, encoded := fighter5CompositionActorState(t, stream.Initial, stream.Calls[0].ObjectID), stream.Initial
		view := fighter5CompositionView(t, stream.Initial, stream.View)
		for index, call := range stream.Calls {
			if call.External == nil {
				t.Fatal("undeclared external actor input")
			}
			view.Controllers, view.Pause, view.Slow = call.External.Controllers, call.External.Pause, call.External.Slow
			var err error
			var emitted ChuActorEmissions
			state, emitted, err = StepFighterActor(state, 5, view, commands, library)
			encoded = fighter5CompositionEncodeActor(t, state, view, emitted, encoded)
			if err != nil || !reflect.DeepEqual(state, fighter5CompositionActorState(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
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

func TestFighter5CompositionFailuresAreAtomic(t *testing.T) {
	commands := actionCommandLibrary(t)
	checkedAction, checkedInput, checkedActor := false, false, false
	walkChuActor(t, "fighter5-composition-action-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter5CompositionAction
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedAction {
			return
		}
		base := fighter5CompositionActionState(t, row.Before, row.ObjectID)
		for _, rules := range []struct {
			Commands   CommandLibrary
			Animations AnimationLibrary
		}{{nil, library}, {commands, nil}} {
			got, err := StepFighterAction(base, 5, actionParameters(row.Before), rules.Commands, rules.Animations)
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
	walkChuActor(t, "fighter5-composition-input-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter5CompositionInput
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedInput || row.Route != "ai-record" {
			return
		}
		base := fighter5CompositionInputState(t, row.Before, row.ObjectID)
		base.Envelope.Position = base.Envelope.StreamBase + uint32(len(base.Envelope.Stream)) - 2
		copy := fighter5CompositionInputState(t, row.Before, row.ObjectID)
		copy.Envelope.Position = base.Envelope.Position
		got, err := StepFighterInput(base)
		if err == nil || !reflect.DeepEqual(got, base) || !reflect.DeepEqual(base, copy) {
			t.Fatal("late record failure leaked real AI or caller stream writes")
		}
		checkedInput = true
	})
	walkChuActor(t, "fighter5-composition-actor-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter5CompositionActor
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checkedActor || row.View.Pause&128 != 0 || row.View.Slow != 0 {
			return
		}
		base := fighter5CompositionActorState(t, row.Before, row.ObjectID)
		if base.Fighter.InputMode != -1 || base.Fighter.InputChannel != 255 {
			return
		}
		base.World.HUDCells = nil
		copy := fighter5CompositionActorState(t, row.Before, row.ObjectID)
		copy.World.HUDCells = nil
		got, emitted, err := StepFighterActor(base, 5, fighter5CompositionView(t, row.Before, row.View), commands, library)
		if err == nil || !reflect.DeepEqual(got, base) || !reflect.DeepEqual(base, copy) || emitted.DrawRequested || len(emitted.HUDTransfers) != 0 {
			t.Fatal("late HUD failure leaked registration, AI, recording, action, bounds or events")
		}
		checkedActor = true
	})
	if !checkedAction || !checkedInput || !checkedActor {
		t.Fatal("missing real cases for full-API atomic checks")
	}
}
