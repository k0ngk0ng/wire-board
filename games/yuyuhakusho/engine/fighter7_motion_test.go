package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

type fighter7MotionSnapshot struct {
	chuSpecialMotionSnapshot
	CharacterID    uint8 `json:"character_id"`
	Left           int16
	Others         map[uint16]string
	MovementSource uint32   `json:"movement_source"`
	AnimationSet   uint32   `json:"animation_set"`
	ShapeSet       uint32   `json:"shape_set"`
	RandomSeed     uint32   `json:"random_seed"`
	PendingSound   uint8    `json:"pending_sound"`
	SoundQueue     [4]uint8 `json:"sound_queue"`
}

type fighter7RAMChange struct {
	Offset        int
	Before, After uint8
}

type fighter7MotionCase struct {
	Kind, Name    string
	Before, After fighter7MotionSnapshot
	EntrySP       uint32              `json:"entry_sp"`
	ReturnSP      uint32              `json:"return_sp"`
	RNGCalls      int                 `json:"rng_calls"`
	Changes       []fighter7RAMChange `json:"ram_changes"`
	External      []fighter7RAMChange `json:"external_changes"`
	Unexplained   []fighter7RAMChange `json:"unexplained_changes"`
	Interrupts    []struct{ Changes []fighter7RAMChange }
	QueueClears   []struct {
		Address       int
		Before, After uint8
	} `json:"external_queue_clears"`
	ExternalInput map[string]uint8  `json:"external_input"`
	RecordKey     AnimationFrameKey `json:"record_key"`
}

func decodeFighter7Motion(t *testing.T, raw fighter7MotionSnapshot) (Fighter7MotionState, []byte, []byte, []byte, map[uint16][]byte) {
	t.Helper()
	base, actor, children, pool := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if raw.CharacterID != 7 || actor[0x58] != 7 || len(raw.Others) != 3 {
		t.Fatal("source7 MAIN lost source identity or one of the other three parent views")
	}
	state := Fighter7MotionState{ChuSpecialMotionState: base, Left: raw.Left, OtherFighters: make(map[uint16]TailFighterEnergy)}
	for i := range state.ChildExtras {
		b := children[i*256:]
		state.ChildExtras[i] = Fighter4ChildExtra{Direction: int32(binary.BigEndian.Uint32(b[0x48:])),
			Held: b[0x71], Pressed: b[0x72], ContactFlags: b[0x89], DragonBank: binary.BigEndian.Uint16(b[0xfe:])}
	}
	others := make(map[uint16][]byte)
	for _, id := range []uint16{0x8400, 0x8800, 0x8c00, 0x9000} {
		if id == raw.ObjectID {
			if _, exists := raw.Others[id]; exists {
				t.Fatal("self HitStopTicks duplicated into peer view")
			}
			continue
		}
		b, err := hex.DecodeString(raw.Others[id])
		if err != nil || len(b) != 256 {
			t.Fatal("missing complete original peer", id, err, len(b))
		}
		others[id] = b
		state.OtherFighters[id] = TailFighterEnergy{Size: binary.BigEndian.Uint16(b[4:]), Team: b[0x6e],
			Health: int16(binary.BigEndian.Uint16(b[0x78:])), Energy: int16(binary.BigEndian.Uint16(b[0xb8:])), HitStopTicks: b[0x69]}
	}
	return state, actor, children, pool, others
}

func compareFighter7Motion(t *testing.T, state Fighter7MotionState, actor, children, pool []byte, others map[uint16][]byte, want fighter7MotionSnapshot) ([]byte, []byte, []byte, map[uint16][]byte) {
	t.Helper()
	expected, expectedActor, expectedChildren, expectedPool, expectedOthers := decodeFighter7Motion(t, want)
	actor, children, pool = encodeChuSpecialMotion(state.ChuSpecialMotionState, actor, children, pool)
	for i, extra := range state.ChildExtras {
		b := children[i*256:]
		binary.BigEndian.PutUint32(b[0x48:], uint32(extra.Direction))
		b[0x71], b[0x72], b[0x89] = extra.Held, extra.Pressed, extra.ContactFlags
		binary.BigEndian.PutUint16(b[0xfe:], extra.DragonBank)
	}
	encodedOthers := make(map[uint16][]byte)
	for id, resource := range state.OtherFighters {
		b := append([]byte(nil), others[id]...)
		if len(b) != 256 {
			t.Fatal("MAIN returned unexpected peer", id)
		}
		binary.BigEndian.PutUint16(b[4:], resource.Size)
		b[0x6e], b[0x69] = resource.Team, resource.HitStopTicks
		binary.BigEndian.PutUint16(b[0x78:], uint16(resource.Health))
		binary.BigEndian.PutUint16(b[0xb8:], uint16(resource.Energy))
		encodedOthers[id] = b
	}
	if !reflect.DeepEqual(state, expected) || !bytes.Equal(actor, expectedActor) || !bytes.Equal(children, expectedChildren) || !bytes.Equal(pool, expectedPool) || !reflect.DeepEqual(encodedOthers, expectedOthers) {
		diff := func(a, b []byte) []int {
			var offsets []int
			for i := range a {
				if a[i] != b[i] {
					offsets = append(offsets, i)
				}
			}
			return offsets
		}
		peerDiff := map[uint16][]int{}
		for id, b := range encodedOthers {
			peerDiff[id] = diff(b, expectedOthers[id])
		}
		t.Fatalf("source7 MAIN differs actor=%x children=%x pool=%x peers=%v body=%+v/%+v aux48=%x/%x energy=%x/%x extras=%+v/%+v", diff(actor, expectedActor), diff(children, expectedChildren), diff(pool, expectedPool), peerDiff, state.Body, expected.Body, state.AlternateVelocity, expected.AlternateVelocity, state.Energy, expected.Energy, state.ChildExtras, expected.ChildExtras)
	}
	return actor, children, pool, encodedOthers
}

func verifyFighter7MotionManifest(t *testing.T, name string, records, calls, fullDeltas int) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Records, Calls int
		Incomplete     int `json:"incomplete_calls"`
		RNG            int `json:"rng_calls"`
		Unexplained    int `json:"unexplained_change_calls"`
		FullDeltas     int `json:"full_ram_delta_calls"`
		Errors         []string
		Identity       struct {
			ROM  string `json:"rom_sha256"`
			Core string `json:"core_sha256"`
			RAM  int    `json:"ram_bytes"`
		}
		Binding struct {
			Source         uint8  `json:"source_tag"`
			Frame          int    `json:"checkpoint_frame"`
			Actor          string `json:"source_actor_sha256"`
			AnimationSet   uint32 `json:"animation_set"`
			ShapeSet       uint32 `json:"shape_set"`
			MovementSource uint32 `json:"movement_source"`
		}
		Baselines []struct {
			Input        string
			InputSHA     string `json:"input_sha256"`
			Reference    string
			ReferenceSHA string `json:"reference_raw_sha256"`
			Manifest     string `json:"reference_manifest"`
			ManifestSHA  string `json:"reference_manifest_sha256"`
			Frames       int    `json:"video_steps"`
			Match        bool   `json:"all_ram_video_match"`
		}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Records != records || m.Calls != calls || m.Incomplete != 0 || m.RNG != 0 || m.Unexplained != 0 || m.FullDeltas != fullDeltas || len(m.Errors) != 0 || m.Identity.RAM != 65536 || m.Identity.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || m.Identity.Core != "e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306" {
		t.Fatal("incomplete source7 MAIN evidence", name)
	}
	if m.Binding.Source != 7 || m.Binding.Frame != 2400 || m.Binding.Actor != "eaec8cd0b3614f03d890b7e448e47d4d8e7baf87872475a406a06a9222230559" || m.Binding.AnimationSet != 850330 || m.Binding.ShapeSet != 853658 || m.Binding.MovementSource != 75248 || len(m.Baselines) != 1 {
		t.Fatal("source7 MAIN is not bound to the actual legal source7 initialization")
	}
	for _, baseline := range m.Baselines {
		if baseline.Frames != 3602 || !baseline.Match || baseline.InputSHA != "fed38aa84737d5d17bdf7d02f78f109ec0836f1cb244de6e2def47a8b9fc904f" || baseline.ReferenceSHA != "ffebd6741b4a5a44a76832fc658045e144fd3607fec5fefd563a01be1809a65d" {
			t.Fatal("source7 MAIN control identity changed")
		}
		for path, want := range map[string]string{baseline.Input: baseline.InputSHA, baseline.Reference: baseline.ReferenceSHA, baseline.Manifest: baseline.ManifestSHA} {
			raw, err := os.ReadFile("../../../" + path)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			if hex.EncodeToString(digest[:]) != want {
				t.Fatal("source7 MAIN control artifact digest differs", path)
			}
		}
	}
}

func verifyFighter7MotionDelta(t *testing.T, row fighter7MotionCase) {
	t.Helper()
	if row.EntrySP != row.ReturnSP || row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed || len(row.Unexplained) != 0 {
		t.Fatal("incomplete original MAIN return or unexplained RAM/RNG change", row.Name)
	}
	seen := map[int]bool{}
	for _, change := range row.Changes {
		if change.Offset < 0 || change.Offset >= 65536 || seen[change.Offset] || change.Before == change.After {
			t.Fatal("invalid complete 64KiB delta", row.Name, change)
		}
		seen[change.Offset] = true
	}
	for _, delta := range row.External {
		if !seen[delta.Offset] {
			t.Fatal("external delta omitted from full RAM delta")
		}
		current, count := delta.Before, 0
		for _, irq := range row.Interrupts {
			for _, change := range irq.Changes {
				if change.Offset == delta.Offset {
					if change.Before != current {
						t.Fatal("IRQ before/after chain is discontinuous")
					}
					current, count = change.After, count+1
				}
			}
		}
		if count == 0 {
			for _, clear := range row.QueueClears {
				if clear.Address == delta.Offset {
					if clear.Before != current {
						t.Fatal("audio clear chain is discontinuous")
					}
					current, count = clear.After, count+1
				}
			}
		}
		if count == 0 || current != delta.After {
			t.Fatal("unexplained external write, including any history RAM", row.Name, delta)
		}
	}
}

// Failure ledger and original CPU fixtures were saved before this test file.
func TestFighter7MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		t.Run(suffix, func(t *testing.T) {
			kinds := map[string]int{}
			count, external := walkChuActor(t, "fighter7-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
				var row fighter7MotionCase
				if err := d.Decode(&row); err != nil {
					t.Fatal(err)
				}
				verifyFighter7MotionDelta(t, row)
				state, actor, children, pool, others := decodeFighter7Motion(t, row.Before)
				kind, err := strconv.ParseUint(row.Kind, 16, 8)
				if err != nil || state.Body.ActionKind != uint8(kind) {
					t.Fatal("MAIN action kind is not its entry")
				}
				originalMap := make(map[uint16]TailFighterEnergy)
				for id, resource := range state.OtherFighters {
					originalMap[id] = resource
				}
				next, err := StepFighter7Motion(state, row.Before.Parameters)
				if err != nil {
					t.Fatal(row.Name, err)
				}
				if !reflect.DeepEqual(state.OtherFighters, originalMap) {
					t.Fatal("successful MAIN modified caller peer map", row.Name)
				}
				compareFighter7Motion(t, next, actor, children, pool, others, row.After)
				kinds[row.Kind]++
			})
			if external != 0 {
				t.Fatal("ordinary natural control unexpectedly changed external audio coverage")
			}
			want := map[string]int{"11": 91}
			if suffix == "boundaries" {
				want = map[string]int{"11": 256, "0f": 294, "13": 294, "14": 294, "45": 906, "39": 1112, "3a": 80, "3b": 1152, "3c": 960, "3d": 1080}
			}
			if !reflect.DeepEqual(kinds, want) {
				t.Fatal("incomplete source7 MAIN CPU matrix", kinds, want)
			}
			verifyFighter7MotionManifest(t, "fighter7-motion-"+suffix, count, count, count)
			t.Logf("source7 %s calls=%d kinds=%v; ordinary natural evidence covers retreat only", suffix, count, kinds)
		})
	}
}

func TestFighter7MotionSingleInitialState(t *testing.T) {
	total, clocks, records, inputs := 0, 0, 0, 0
	count, _ := walkChuActor(t, "fighter7-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name          string
			InitialKind   string `json:"initial_kind"`
			Initial       fighter7MotionSnapshot
			InitialRecord fighter7MotionCase `json:"initial_record"`
			Calls         []fighter7MotionCase
			FinalKind     string `json:"final_kind"`
			Milestone     string
		}
		if err := d.Decode(&stream); err != nil {
			t.Fatal(err)
		}
		state, actor, children, pool, others := decodeFighter7Motion(t, stream.Initial)
		if stream.InitialRecord.Kind != "record" || stream.InitialRecord.After.ActorBytes != stream.Initial.ActorBytes {
			t.Fatal("missing initial own-record timing refresh")
		}
		context := stream.Initial
		for _, row := range stream.Calls {
			compareFighter7Motion(t, state, actor, children, pool, others, row.Before)
			switch row.Kind {
			case "clock":
				verifyFighter7MotionDelta(t, row)
				state.Body.Clock = StepAnimationClock(state.Body.Clock)
				clocks++
			case "record":
				key := AnimationFrameKey{AnimationSet: context.AnimationSet, ShapeSet: context.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
				data, ok := library[key]
				if !ok || row.RecordKey != key {
					t.Fatal("timing key is not from this trajectory's own source7 animation")
				}
				state.Body.Clock.Duration, state.Body.Clock.Flags = data.Duration, data.Flags
				records++
			case "input":
				if len(row.ExternalInput) != 2 {
					t.Fatal("undeclared input channel count")
				}
				for key, value := range row.ExternalInput {
					switch key {
					case "held":
						state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
					case "pressed":
						state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
					default:
						t.Fatal("undeclared input owner")
					}
				}
				inputs++
			default:
				verifyFighter7MotionDelta(t, row)
				var err error
				state, err = StepFighter7Motion(state, context.Parameters)
				if err != nil {
					t.Fatal(stream.Name, row.Name, err)
				}
			}
			actor, children, pool, others = compareFighter7Motion(t, state, actor, children, pool, others, row.After)
			total++
		}
		if stream.Milestone != "action-transition" || stream.InitialKind == stream.FinalKind || stream.FinalKind != strconv.FormatUint(uint64(state.Body.ActionKind), 16) && stream.FinalKind != "0"+strconv.FormatUint(uint64(state.Body.ActionKind), 16) {
			t.Fatal("MAIN trajectory did not reach its declared action transition", stream.Name)
		}
	})
	if count != 34 || total != 2758 || clocks != 916 || records != 916 || inputs != 10 {
		t.Fatal("incomplete single-initial-state source7 clock trajectories", count, total, clocks, records, inputs)
	}
	verifyFighter7MotionManifest(t, "fighter7-motion-sequences", count, total, clocks*2)
	t.Logf("source7 single-initial trajectories=%d original clocks=%d MAIN=%d own-record refreshes=%d declared inputs=%d", count, clocks, clocks, records, inputs)
}

func fighter7MotionAtomicState() Fighter7MotionState {
	return Fighter7MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: 0x8400, Body: FighterBodyState{GroundActionState: GroundActionState{ActionKind: 0x3d, Team: 1, BaseTeam: 1, Clock: AnimationClock{FrameIndex: 2}}, Ground: 320 << 16, Y: 300 << 16}, AlternateVelocity: 0x03123456, Energy: 0x0010abcd},
		Left: -13, OtherFighters: map[uint16]TailFighterEnergy{0x8800: {Team: 0, Size: 0, Health: -1, HitStopTicks: 1}, 0x8c00: {Team: 1, Size: 256, Health: 0, HitStopTicks: 2}, 0x9000: {Team: 2, Size: 256, Health: 111, HitStopTicks: 3}}}
}

func TestFighter7MotionErrorsAreAtomic(t *testing.T) {
	check := func(base Fighter7MotionState) {
		t.Helper()
		before, _ := json.Marshal(base)
		got, err := StepFighter7Motion(base, JumpParameters{})
		after, _ := json.Marshal(base)
		if err == nil || !reflect.DeepEqual(got, base) || !bytes.Equal(before, after) {
			t.Fatal("invalid source7 MAIN input leaked a field or peer map mutation", err)
		}
	}
	base := fighter7MotionAtomicState()
	base.Body.ActionKind = 0
	check(base)
	for _, id := range []uint16{0, 0x8401, 0x8500, 0x9400} {
		base = fighter7MotionAtomicState()
		base.ObjectID = id
		check(base)
	}
	for _, id := range []uint16{0x8800, 0x8c00, 0x9000} {
		base = fighter7MotionAtomicState()
		delete(base.OtherFighters, id)
		check(base)
	}
	for _, id := range []uint16{0x8400, 0x8500} {
		base = fighter7MotionAtomicState()
		base.OtherFighters[id] = TailFighterEnergy{}
		check(base)
	}
	base = fighter7MotionAtomicState()
	base.OtherFighters = nil
	check(base)
	for _, index := range []uint8{8, 128, 255} {
		base = fighter7MotionAtomicState()
		base.Body.ActionKind = 0x11
		base.Body.Clock.FrameIndex = index
		check(base)
	}
	base.Body.Y = base.Body.Ground
	base.Body.Clock.Completed = 1
	if got, err := StepFighter7Motion(base, JumpParameters{}); err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed retreat incorrectly consulted finite frame table", err)
	}
	for _, kind := range []uint8{0x0f, 0x13, 0x14, 0x45} {
		base = fighter7MotionAtomicState()
		base.Pool.Cursor = 1
		base.Body.ActionKind = kind
		base.Body.Clock.FrameIndex = 1
		base.Body.Clock.Ticks = 0
		if kind == 0x0f {
			base.Body.Y = base.Body.Ground
		}
		check(base)
	}
	base = fighter7MotionAtomicState()
	base.Pool.Cursor = 1
	base.Children[2].Size = 0x100
	before, _ := json.Marshal(base)
	got, err := StepFighter7Motion(base, JumpParameters{})
	after, _ := json.Marshal(base)
	if err != nil || !bytes.Equal(before, after) || got.OtherFighters[0x8800].HitStopTicks != 255 || got.OtherFighters[0x8c00].HitStopTicks != 2 || got.OtherFighters[0x9000].HitStopTicks != 255 || got.Children != base.Children {
		t.Fatal("busy third child did not preserve side effects for empty/dead opposing parents", err)
	}
}
