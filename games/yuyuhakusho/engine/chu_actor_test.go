package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type chuActorSnapshot struct {
	Action           chuActionSnapshot
	FamilyWorld      string `json:"family_world"`
	Stream           string
	Stopped          uint8
	Markers          [4]uint8
	Registration     string
	RegistrationUsed uint16 `json:"registration_used"`
	Samples          string
	SamplesUsed      uint16 `json:"samples_used"`
	HUDCells         string `json:"hud_cells"`
	HUDQueue         string `json:"hud_queue"`
	HUDCursor        uint16 `json:"hud_cursor"`
}

type chuActorViewFixture struct {
	Pause, Slow, Disabled, Blink uint8
	Difficulty                   uint16
	CameraX                      int16  `json:"camera_x"`
	CameraY                      int16  `json:"camera_y"`
	CameraShake                  int16  `json:"camera_shake"`
	StreamBase                   uint32 `json:"stream_base"`
	Controllers                  [8]ControllerFrame
}

func testActorView(t *testing.T, raw chuActorSnapshot, fixture chuActorViewFixture) ChuActorView {
	t.Helper()
	v := ChuActorView{Pause: fixture.Pause, Slow: fixture.Slow, Disabled: fixture.Disabled, Blink: fixture.Blink, Difficulty: fixture.Difficulty,
		CameraX: fixture.CameraX, CameraY: fixture.CameraY, CameraShake: fixture.CameraShake, StreamBase: fixture.StreamBase, Controllers: fixture.Controllers,
		ArenaLeft: raw.Action.Left, ArenaRight: raw.Action.Right, BattleState: raw.Action.Tail.BattleState,
		Parameters: ChuActionParameters{ChuTailParameters: raw.Action.Tail.Parameters, WalkSpeed: raw.Action.WalkSpeed}}
	b := attackContactBytes(t, raw.FamilyWorld, 4096)
	for i := range v.Fighters {
		a := b[i*1024:]
		w := func(off int) int16 { return int16(binary.BigEndian.Uint16(a[off:])) }
		v.Fighters[i] = ChuAIFighter{Size: uint16(w(4)), Team: a[0x6e], Health: w(0x78), X: w(0x40), Y: w(0x44), Ground: w(0x9c),
			FrameFlags: a[0x5c], AttackFlags: a[0x64], ChildX: w(0x140), ChildY: w(0x144), ChildGround: w(0x19c), ChildAttackFlags: a[0x164]}
	}
	return v
}

func decodeChuActor(t *testing.T, raw chuActorSnapshot, id uint16) ChuActorState {
	t.Helper()
	a := decodeChuAction(t, raw.Action, id)
	b := attackContactBytes(t, raw.Action.Tail.ActorBytes, 256)
	family := attackContactBytes(t, raw.FamilyWorld, 4096)
	index := int(id) - 0x8400
	if index < 0 || index+1024 > len(family) || !bytes.Equal(family[index:index+256], b) || hex.EncodeToString(family[index+256:index+1024]) != raw.Action.Children {
		t.Fatal("actor/children do not match source family")
	}
	f := ChuActorFighter{ObjectID: id, Size: binary.BigEndian.Uint16(b[4:]), Body: a.Body, Controls: a.Controls, Energy: int32(binary.BigEndian.Uint32(b[0xb8:])),
		InputMode: int8(b[0xc9]), InputChannel: b[0x76], StreamPosition: binary.BigEndian.Uint32(b[0xca:]), AIBehavior: b[0xce],
		AITargetCursor: binary.BigEndian.Uint16(b[0xd0:]), AICountdown: binary.BigEndian.Uint16(b[0xd2:]), AIElapsed: binary.BigEndian.Uint16(b[0xd4:]), AIFollowTarget: binary.BigEndian.Uint16(b[0xd6:]),
		ScreenX: int16(binary.BigEndian.Uint16(b[8:])), ScreenY: int16(binary.BigEndian.Uint16(b[12:])), ScreenOffsetX: a.ScreenOffsetX, ScreenOffsetY: int16(binary.BigEndian.Uint16(b[0x56:])),
		DisplayedHealth: int16(binary.BigEndian.Uint16(b[0x7a:])), DisplayedEnergy: int16(binary.BigEndian.Uint16(b[0xbc:])), HUDOffset: binary.BigEndian.Uint16(b[0x8a:]),
		AnimationSet: a.AnimationSet, ShapeSet: a.ShapeSet, GraphicOffsetX: a.GraphicOffsetX, GraphicOffsetY: a.GraphicOffsetY, GraphicParameter: a.GraphicParameter,
		ChargeTicks: a.ChargeTicks, GuardMask: a.GuardMask, ContactFlags: a.ContactFlags, TargetMask: a.TargetMask, AttackValue: a.AttackValue, StunDamage: a.StunDamage,
		EffectKind: a.EffectKind, Reaction: a.Reaction, AttackScale: a.AttackScale, HitStopTicks: a.HitStopTicks, Shake: a.Shake, StatusDelay: a.StatusDelay,
		Residual: a.Residual, StunTicks: a.StunTicks, TargetBit: a.TargetBit, Power: a.Power, AlternateVelocity: a.AlternateVelocity, Children: a.Children}
	for i := range f.ChildAttackFlags {
		f.ChildAttackFlags[i] = family[index+256*(i+1)+0x64]
	}
	w := ChuActorWorld{Pool: a.Pool, EffectExtras: a.EffectExtras, OtherEnergy: make(map[uint16]int16), OccupiedLanes: a.OccupiedLanes, Buffers: a.Buffers, Throws: a.Throws,
		PendingSound: a.PendingSound, SoundQueue: a.SoundQueue, RandomSeed: a.RandomSeed, GlobalF568: a.GlobalF568, GlobalF56A: a.GlobalF56A,
		Stream: attackContactBytes(t, raw.Stream, 1024), Stopped: raw.Stopped, EndMarkers: raw.Markers,
		Registration: FighterRegistrationState{Used: raw.RegistrationUsed, Owners: envelopeWords(t, raw.Registration, 128)},
		SampleUsed:   raw.SamplesUsed, HUDCells: envelopeWords(t, raw.HUDCells, 1024), HUDCursor: raw.HUDCursor}
	for id, resource := range a.OtherFighters {
		w.OtherEnergy[id] = resource.Energy
	}
	samples := attackContactBytes(t, raw.Samples, 128)
	for i := range w.SamplesX {
		w.SamplesX[i], w.SamplesY[i] = int32(binary.BigEndian.Uint32(samples[i*4:])), int32(binary.BigEndian.Uint32(samples[64+i*4:]))
	}
	return ChuActorState{Fighter: f, World: w}
}

// This independent adapter reassembles fields into the input bytes. It never
// uses expected output bytes as state and compares the whole source family.
func encodeChuActor(t *testing.T, state ChuActorState, view ChuActorView, emitted ChuActorEmissions, previous chuActorSnapshot) chuActorSnapshot {
	t.Helper()
	f, w := state.Fighter, state.World
	resources := make(map[uint16]TailFighterEnergy, len(w.OtherEnergy))
	for id, energy := range w.OtherEnergy {
		i := (int(id) - 0x8400) / 1024
		v := view.Fighters[i]
		resources[id] = TailFighterEnergy{Size: v.Size, Team: v.Team, Health: v.Health, Energy: energy}
	}
	action := ChuActionState{ChuTailState: ChuTailState{Body: f.Body, Controls: f.Controls, ObjectID: f.ObjectID, Energy: int16(f.Energy >> 16), ChildSize: f.Children[0].Size,
		Pool: w.Pool, Throws: w.Throws, OtherFighters: resources, ChargeTicks: f.ChargeTicks, OccupiedLanes: w.OccupiedLanes, BattleState: view.BattleState,
		AnimationSet: f.AnimationSet, ShapeSet: f.ShapeSet, GraphicOffsetX: f.GraphicOffsetX, GraphicOffsetY: f.GraphicOffsetY, GraphicParameter: f.GraphicParameter,
		GuardMask: f.GuardMask, ContactFlags: f.ContactFlags, TargetMask: f.TargetMask, AttackValue: f.AttackValue, StunDamage: f.StunDamage, EffectKind: f.EffectKind, Reaction: f.Reaction,
		AttackScale: f.AttackScale, RandomSeed: w.RandomSeed, Buffers: w.Buffers, PendingSound: w.PendingSound, SoundQueue: w.SoundQueue},
		EnergyFraction: uint16(f.Energy), HitStopTicks: f.HitStopTicks, Shake: f.Shake, ScreenOffsetX: f.ScreenOffsetX, StatusDelay: f.StatusDelay, Residual: f.Residual,
		StunTicks: f.StunTicks, TargetBit: f.TargetBit, Power: f.Power, AlternateVelocity: f.AlternateVelocity, Children: f.Children, EffectExtras: w.EffectExtras,
		GlobalF568: w.GlobalF568, GlobalF56A: w.GlobalF56A, Left: view.ArenaLeft, Right: view.ArenaRight}
	raw := previous
	raw.Action = encodeChuAction(t, action, previous.Action)
	b := attackContactBytes(t, raw.Action.Tail.ActorBytes, 256)
	w16 := func(off int, value uint16) { binary.BigEndian.PutUint16(b[off:], value) }
	w16(4, f.Size)
	w16(8, uint16(f.ScreenX))
	w16(12, uint16(f.ScreenY))
	w16(0x54, uint16(f.ScreenOffsetX))
	w16(0x56, uint16(f.ScreenOffsetY))
	w16(0x78, uint16(f.Body.Health))
	w16(0x7e, uint16(f.Body.Auxiliary7E))
	w16(0x80, uint16(f.Body.Auxiliary80))
	b[0xc8] = f.Body.TransitionDelay
	b[0xc9], b[0x76], b[0xce] = uint8(f.InputMode), f.InputChannel, f.AIBehavior
	binary.BigEndian.PutUint32(b[0xca:], f.StreamPosition)
	w16(0xd0, f.AITargetCursor)
	w16(0xd2, f.AICountdown)
	w16(0xd4, f.AIElapsed)
	w16(0xd6, f.AIFollowTarget)
	hud := encodeEnvelopeHUD(t, FighterHUDState{Health: f.Body.Health, DisplayedHealth: f.DisplayedHealth, Energy: int16(f.Energy >> 16), DisplayedEnergy: f.DisplayedEnergy,
		Offset: f.HUDOffset, TargetBit: f.TargetBit, Blink: view.Blink, Cells: w.HUDCells, TransferCursor: w.HUDCursor}, emitted.HUDTransfers,
		envelopeSnapshot{Actor: hex.EncodeToString(b), Cells: previous.HUDCells, Queue: previous.HUDQueue, Cursor: previous.HUDCursor, Blink: view.Blink})
	raw.Action.Tail.ActorBytes = hud.Actor
	children := attackContactBytes(t, raw.Action.Children, 768)
	for i, flags := range f.ChildAttackFlags {
		children[i*256+0x64] = flags
	}
	raw.Action.Children, raw.Action.Tail.ChildBytes = hex.EncodeToString(children), hex.EncodeToString(children[:256])
	family := attackContactBytes(t, previous.FamilyWorld, 4096)
	index := int(f.ObjectID) - 0x8400
	copy(family[index:index+256], attackContactBytes(t, hud.Actor, 256))
	copy(family[index+256:index+1024], children)
	for id, actor := range raw.Action.Tail.Others {
		copy(family[int(id)-0x8400:], attackContactBytes(t, actor, 256))
	}
	raw.FamilyWorld = hex.EncodeToString(family)
	raw.Stream, raw.Stopped, raw.Markers = hex.EncodeToString(w.Stream), w.Stopped, w.EndMarkers
	raw.Registration, raw.RegistrationUsed = encodeEnvelopeWords(w.Registration.Owners), w.Registration.Used
	samples := make([]byte, 128)
	for i := range w.SamplesX {
		binary.BigEndian.PutUint32(samples[i*4:], uint32(w.SamplesX[i]))
		binary.BigEndian.PutUint32(samples[64+i*4:], uint32(w.SamplesY[i]))
	}
	raw.Samples, raw.SamplesUsed = hex.EncodeToString(samples), w.SampleUsed
	raw.HUDCells, raw.HUDQueue, raw.HUDCursor = hud.Cells, hud.Queue, w.HUDCursor
	return raw
}

type chuActorCall struct {
	Name, Scenario string
	ObjectID       uint16 `json:"object_id"`
	View           chuActorViewFixture
	Before, After  chuActorSnapshot
	RNGCalls       int `json:"rng_calls"`
	Stages         map[string]int
	Clears         []struct {
		PC, Address string
		Before      uint8
		SR, SP      uint32
	} `json:"external_queue_clears"`
	External *struct {
		Controllers [8]ControllerFrame
		Pause, Slow uint8
	} `json:"external_view"`
}

// Streaming large source records keeps the Wasm evidence checker bounded.
func walkChuActor(t *testing.T, name string, apply func(*json.Decoder, AnimationLibrary)) (int, int) {
	t.Helper()
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass     bool
		Records  int
		External int    `json:"external_queue_calls"`
		Match    bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err = json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !m.Pass || !m.Match || m.Records == 0 || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified wholeactor fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	tee := io.TeeReader(z, h)
	d := json.NewDecoder(tee)
	if token, e := d.Token(); e != nil || token != json.Delim('{') {
		t.Fatal("invalid actor evidence object", e)
	}
	if key, e := d.Token(); e != nil || key != "library" {
		t.Fatal("missing first immutable actor library", e)
	}
	var frames []struct {
		Key  AnimationFrameKey
		Data AnimationRecordData
	}
	if err = d.Decode(&frames); err != nil {
		t.Fatal(err)
	}
	immutable, _ := json.Marshal(frames)
	library := make(AnimationLibrary, len(frames))
	for _, f := range frames {
		if _, ok := library[f.Key]; ok {
			t.Fatal("duplicate actor frame")
		}
		library[f.Key] = f.Data
	}
	if key, e := d.Token(); e != nil || key != "calls" {
		t.Fatal("missing actor calls", e)
	}
	if token, e := d.Token(); e != nil || token != json.Delim('[') {
		t.Fatal("invalid actor call array", e)
	}
	count := 0
	for d.More() {
		apply(d, library)
		count++
	}
	if token, e := d.Token(); e != nil || token != json.Delim(']') {
		t.Fatal("unfinished actor calls", e)
	}
	if token, e := d.Token(); e != nil || token != json.Delim('}') {
		t.Fatal("unfinished actor object", e)
	}
	if _, err = io.Copy(io.Discard, tee); err != nil {
		t.Fatal(err)
	}
	if count != m.Records || hex.EncodeToString(h.Sum(nil)) != m.SHA {
		t.Fatal("actor call count or raw digest mismatch")
	}
	for i := range frames {
		frames[i].Data = library[frames[i].Key]
	}
	after, _ := json.Marshal(frames)
	if !bytes.Equal(immutable, after) {
		t.Fatal("actor mutated immutable frame library")
	}
	return count, m.External
}

func checkChuActor(t *testing.T, row chuActorCall, library AnimationLibrary, commands CommandLibrary) bool {
	t.Helper()
	state, copy := decodeChuActor(t, row.Before, row.ObjectID), decodeChuActor(t, row.Before, row.ObjectID)
	view := testActorView(t, row.Before, row.View)
	got, emitted, err := StepChuActor(state, view, commands, library)
	want := decodeChuActor(t, row.After, row.ObjectID)
	queueDifference := got.World.SoundQueue != want.World.SoundQueue
	expected := row.After
	if len(row.Clears) != 0 {
		for _, c := range row.Clears {
			if (c.PC != "0x37c56" && c.PC != "0x37cac") || c.Address < "0xf80a" || c.Address > "0xf80d" {
				t.Fatal("unidentified actor async audio write")
			}
		}
		want.World.SoundQueue = got.World.SoundQueue
		expected.Action.Tail.SoundQueue = got.World.SoundQueue
	}
	if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeChuActor(t, got, view, emitted, row.Before), expected) {
		t.Fatalf("actor %s/%s complete state differs: %v", row.Scenario, row.Name, err)
	}
	if !emitted.DrawRequested || emitted.ObjectID != row.ObjectID || !reflect.DeepEqual(state, copy) {
		t.Fatal("missing draw request or caller mutation")
	}
	active := row.View.Pause&128 == 0 && row.View.Slow == 0
	if (row.RNGCalls != 0) != active {
		t.Fatal("original RNG gate evidence is inconsistent")
	}
	return queueDifference
}

// Full original12682→126ba fixtures and these tests precede chu_actor.go.
func TestChuActorOriginalRules(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, name := range []string{"chu-actor-oracle", "chu-actor-boundaries"} {
		t.Run(name, func(t *testing.T) {
			external, queueDifferences := 0, 0
			count, wantExternal := walkChuActor(t, name, func(d *json.Decoder, library AnimationLibrary) {
				var row chuActorCall
				if err := d.Decode(&row); err != nil {
					t.Fatal(err)
				}
				if len(row.Clears) != 0 {
					external++
				}
				if checkChuActor(t, row, library, commands) {
					queueDifferences++
				}
			})
			if external != wantExternal {
				t.Fatal("missing external actor evidence")
			}
			t.Logf("complete actor rule callbacks=%d externalqueue rows=%d actualqueue differences=%d", count, external, queueDifferences)
		})
	}
}

func TestChuActorSingleInitialState(t *testing.T) {
	commands := actionCommandLibrary(t)
	steps := 0
	count, external := walkChuActor(t, "chu-actor-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Initial chuActorSnapshot
			View    chuActorViewFixture
			Calls   []chuActorCall
		}
		if err := d.Decode(&stream); err != nil {
			t.Fatal(err)
		}
		if len(stream.Calls) == 0 {
			t.Fatal("empty actor stream")
		}
		state := decodeChuActor(t, stream.Initial, stream.Calls[0].ObjectID)
		view := testActorView(t, stream.Initial, stream.View)
		encoded := stream.Initial
		for i, call := range stream.Calls {
			if call.External == nil {
				t.Fatal("missing actual external actor view input")
			}
			view.Controllers, view.Pause, view.Slow = call.External.Controllers, call.External.Pause, call.External.Slow
			var err error
			var emit ChuActorEmissions
			state, emit, err = StepChuActor(state, view, commands, library)
			encoded = encodeChuActor(t, state, view, emit, encoded)
			if err != nil || !reflect.DeepEqual(state, decodeChuActor(t, call.After, call.ObjectID)) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("wholeactor stream step%d differs: %v", i, err)
			}
			steps++
		}
	})
	if count != 6 || external != 0 || steps != 144 {
		t.Fatal("missing full actor rule streams")
	}
}

func TestChuActorLateFailureIsAtomic(t *testing.T) {
	commands := actionCommandLibrary(t)
	checked := false
	walkChuActor(t, "chu-actor-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row chuActorCall
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checked {
			return
		}
		state := decodeChuActor(t, row.Before, row.ObjectID)
		if state.Fighter.InputMode != -1 || state.Fighter.InputChannel != 255 || row.View.Pause&128 != 0 || row.View.Slow != 0 {
			return
		}
		state.World.HUDCells = nil
		copy := decodeChuActor(t, row.Before, row.ObjectID)
		copy.World.HUDCells = nil
		view := testActorView(t, row.Before, row.View)
		got, emit, err := StepChuActor(state, view, commands, library)
		if err == nil || !reflect.DeepEqual(got, state) || !reflect.DeepEqual(state, copy) || emit.DrawRequested || len(emit.HUDTransfers) != 0 {
			t.Fatal("late HUD error leaked registration, AI, stream or other rule mutation")
		}
		checked = true
	})
	if !checked {
		t.Fatal("missing actual AI-record callback for late-failure test")
	}
}
