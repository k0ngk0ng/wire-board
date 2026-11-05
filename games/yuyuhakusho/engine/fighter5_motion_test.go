package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
)

type fighter5MotionSnapshot struct {
	fighter0MotionSnapshot
	CharacterID    uint8  `json:"character_id"`
	MovementSource uint32 `json:"movement_source"`
}

type fighter5MotionChange struct {
	Offset        uint16
	Before, After uint8
}

type fighter5MotionCase struct {
	Kind           uint8
	Name, Scenario string
	SourceRegister string `json:"source_register"`
	EntrySP        uint32 `json:"entry_sp"`
	ExitSP         uint32 `json:"exit_sp"`
	RAMBytes       int    `json:"ram_bytes"`
	RAMBeforeSHA   string `json:"ram_before_sha256"`
	RAMAfterSHA    string `json:"ram_after_sha256"`
	Before, After  fighter5MotionSnapshot
	Allocations    []int
	RNGCalls       int `json:"rng_calls"`
	Clears         []struct {
		PC, Address string
		Before      uint8
	} `json:"external_queue_clears"`
	Interrupts []struct {
		Entry, Exit   string
		EntryRegister string `json:"entry_register"`
		Changes       []fighter5MotionChange
	}
	RAMChanges         []fighter5MotionChange `json:"ram_changes"`
	ExternalChanges    []fighter5MotionChange `json:"external_changes"`
	UnexplainedChanges []fighter5MotionChange `json:"unexplained_changes"`
	AdvanceClock       bool                   `json:"advance_clock"`
	External           map[string]uint8       `json:"external_input"`
	RecordAfter        AnimationFrameKey      `json:"record_after"`
	AfterRecord        fighter5MotionSnapshot `json:"after_record"`
}

func decodeFighter5Motion(t *testing.T, raw fighter5MotionSnapshot) (Fighter5MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, actor, children, pool := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if raw.CharacterID != 5 || actor[0x58] != 5 || binary.BigEndian.Uint32(actor[0x90:]) != raw.MovementSource {
		t.Fatal("source5 identity or immutable movement source differs")
	}
	state := Fighter5MotionState{ChuSpecialMotionState: base}
	for i := range state.ChildExtras {
		b := children[i*256:]
		state.ChildExtras[i] = Fighter5ChildExtra{Gravity: int32(binary.BigEndian.Uint32(b[0x4c:])), Owner: binary.BigEndian.Uint16(b[0xfe:])}
	}
	return state, actor, children, pool
}

func compareFighter5Motion(t *testing.T, state Fighter5MotionState, actor, children, pool []byte, want fighter5MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeFighter5Motion(t, want)
	actor, children, pool = encodeChuSpecialMotion(state.ChuSpecialMotionState, actor, children, pool)
	for i, extra := range state.ChildExtras {
		b := children[i*256:]
		binary.BigEndian.PutUint32(b[0x4c:], uint32(extra.Gravity))
		binary.BigEndian.PutUint16(b[0xfe:], extra.Owner)
	}
	if state != expected || !bytes.Equal(actor, wa) || !bytes.Equal(children, wc) || !bytes.Equal(pool, wp) {
		diff := func(a, b []byte) []int {
			var offsets []int
			for i := range a {
				if a[i] != b[i] {
					offsets = append(offsets, i)
				}
			}
			return offsets
		}
		t.Fatalf("source5 MAIN differs actor=%x children=%x pool=%x body=%+v/%+v aux48=%x/%x extras=%+v/%+v", diff(actor, wa), diff(children, wc), diff(pool, wp), state.Body, expected.Body, state.AlternateVelocity, expected.AlternateVelocity, state.ChildExtras, expected.ChildExtras)
	}
	return actor, children, pool
}

func checkFighter5MotionManifest(t *testing.T, name string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Incomplete  int `json:"incomplete_calls"`
		Unexplained int `json:"unexplained_change_calls"`
		Unknown     int `json:"private_unknown_ram_writes"`
		RNG         int `json:"rng_calls"`
		Allocations int `json:"allocation_attempts"`
		Identity    struct {
			ROM  string `json:"rom_sha256"`
			Core string `json:"core_sha256"`
		}
		Baselines []struct {
			Input            string
			InputSHA         string `json:"input_sha256"`
			Reference        string
			ReferenceSHA     string `json:"reference_raw_sha256"`
			Manifest         string `json:"reference_manifest"`
			ManifestSHA      string `json:"reference_manifest_sha256"`
			Steps            int    `json:"video_steps"`
			EntryLimit       int    `json:"entry_frame_limit"`
			AllRAMVideoMatch bool   `json:"all_ram_video_match"`
		}
		Table struct {
			Bytes string
			SHA   string `json:"sha256"`
		} `json:"retreat_table"`
		Binding struct {
			Tag        uint8  `json:"source_tag"`
			Pointer    uint32 `json:"parameter_pointer"`
			Checkpoint int    `json:"checkpoint_frame"`
			Header     string `json:"source_header"`
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Incomplete != 0 || m.Unexplained != 0 || m.Unknown != 0 || m.RNG != 0 || m.Allocations != 0 || len(m.Baselines) == 0 || m.Identity.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || m.Identity.Core != "e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306" {
		t.Fatal("incomplete source5 CPU evidence")
	}
	if m.Table.Bytes != "0044000e000c000a0008000600380036" {
		t.Fatal("shared retreat table differs")
	}
	decoded, err := hex.DecodeString(m.Table.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(decoded)
	if hex.EncodeToString(h[:]) != m.Table.SHA {
		t.Fatal("unbound retreat table")
	}
	if m.Binding.Tag != 5 || m.Binding.Pointer != 0x125b4 || m.Binding.Checkpoint != 2400 || m.Binding.Header != "0001266801000002" {
		t.Fatal("private source5 checkpoint header or movement context differs")
	}
	for _, baseline := range m.Baselines {
		if !baseline.AllRAMVideoMatch || baseline.Steps < 3602 || baseline.Steps-baseline.EntryLimit != 2 {
			t.Fatal("incomplete source5 control or drain")
		}
		for _, file := range []struct{ Path, SHA string }{{baseline.Input, baseline.InputSHA}, {baseline.Reference, baseline.ReferenceSHA}, {baseline.Manifest, baseline.ManifestSHA}} {
			data, err := os.ReadFile("../../../" + file.Path)
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.Sum256(data)
			if hex.EncodeToString(h[:]) != file.SHA {
				t.Fatal("independent original control binding differs", file.Path)
			}
		}
		data, err := os.ReadFile("../../../" + baseline.Manifest)
		var control struct {
			Observer bool `json:"observer_attached"`
			Identity struct {
				Core string `json:"core_sha256"`
			}
		}
		if err != nil || json.Unmarshal(data, &control) != nil || control.Observer || control.Identity.Core != "1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da" {
			t.Fatal("reference was not produced by the independent unobserved original core")
		}
	}
}

func auditFighter5Motion(t *testing.T, row fighter5MotionCase) {
	t.Helper()
	if row.EntrySP != row.ExitSP || row.RAMBytes != 65536 || len(row.RAMBeforeSHA) != 64 || len(row.RAMAfterSHA) != 64 || len(row.UnexplainedChanges) != 0 || row.RNGCalls != 0 || len(row.Allocations) != 0 || row.Before.Pool != row.After.Pool || row.Before.RandomSeed != row.After.RandomSeed || row.Before.PendingSound != row.After.PendingSound {
		t.Fatal("incomplete source5 whole-RAM or external side-effect audit", row.Name)
	}
	clears := make(map[uint16]bool)
	for _, clear := range row.Clears {
		address, err := strconv.ParseUint(clear.Address, 0, 16)
		if err != nil || address < 0xf80a || address > 0xf80d || (clear.PC != "0x37c56" && clear.PC != "0x37cac") {
			t.Fatal("unidentified asynchronous audio consumer")
		}
		clears[uint16(address)] = true
	}
	for _, irq := range row.Interrupts {
		if irq.Entry != "0xfff000" || (irq.EntryRegister != "0xfff000" && irq.EntryRegister != "0xfffff000") || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
			t.Fatal("unidentified IRQ domain")
		}
	}
	before, err := hex.DecodeString(row.Before.ActorBytes + row.Before.Children)
	if err != nil {
		t.Fatal(err)
	}
	after, err := hex.DecodeString(row.After.ActorBytes + row.After.Children)
	if err != nil {
		t.Fatal(err)
	}
	changes := make(map[uint16]fighter5MotionChange, len(row.RAMChanges))
	external := 0
	for index, change := range row.RAMChanges {
		if change.Before == change.After || index > 0 && row.RAMChanges[index-1].Offset >= change.Offset {
			t.Fatal("noncanonical original RAM delta")
		}
		changes[change.Offset] = change
		if change.Offset >= row.Before.ObjectID && uint32(change.Offset) < uint32(row.Before.ObjectID)+1024 {
			i := change.Offset - row.Before.ObjectID
			if before[i] != change.Before || after[i] != change.After {
				t.Fatal("complete family bytes disagree with RAM audit")
			}
			continue
		}
		if uint16(row.EntrySP)-change.Offset < 0x180 || clears[change.Offset] {
			continue
		}
		external++
		current, explained := change.Before, 0
		for _, irq := range row.Interrupts {
			for _, delta := range irq.Changes {
				if delta.Offset == change.Offset {
					if delta.Before != current {
						t.Fatal("broken independent IRQ change chain")
					}
					current, explained = delta.After, explained+1
				}
			}
		}
		if explained == 0 || current != change.After {
			t.Fatal("unexplained external original RAM change", change.Offset)
		}
	}
	if external != len(row.ExternalChanges) {
		t.Fatal("whole-RAM external change count differs")
	}
	for i := range before {
		if before[i] != after[i] {
			if _, ok := changes[row.Before.ObjectID+uint16(i)]; !ok {
				t.Fatal("missing family byte in complete RAM audit")
			}
		}
	}
}

func readFighter5MotionParameters(t *testing.T) map[uint32]JumpParameters {
	t.Helper()
	raw, err := os.ReadFile("testdata/fighter5-motion-parameters.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("testdata/fighter5-motion-sequences-manifest.json")
	var binding struct {
		SHA string `json:"movement_parameters_sha256"`
	}
	if err != nil || json.Unmarshal(manifest, &binding) != nil {
		t.Fatal("missing immutable source5 movement binding", err)
	}
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != binding.SHA {
		t.Fatal("source5 movement artifact digest differs")
	}
	var data struct {
		ROM     string `json:"rom_sha256"`
		Records []struct {
			Source uint32
			Raw    string `json:"raw_bytes"`
			Data   JumpParameters
		}
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(data.Records) != 1 {
		t.Fatal("incomplete source5 immutable movement data", err)
	}
	parameters := make(map[uint32]JumpParameters)
	for _, row := range data.Records {
		b, err := hex.DecodeString(row.Raw)
		if err != nil || len(b) != 16 || row.Source != 0x125b4 {
			t.Fatal("invalid source5 movement ROM record")
		}
		decoded := JumpParameters{VerticalVelocity: int32(binary.BigEndian.Uint32(b[4:])), Gravity: int32(binary.BigEndian.Uint32(b[8:])), HorizontalVelocity: int32(binary.BigEndian.Uint32(b[12:]))}
		if decoded != row.Data {
			t.Fatal("source5 movement semantics differ from immutable bytes")
		}
		parameters[row.Source] = decoded
	}
	return parameters
}

// The failure ledger and two byte-identical CPU fixture captures precede this
// frozen test file; StepFighter5Motion must still be absent for its first run.
func TestFighter5MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		name := "fighter5-motion-" + suffix
		checkFighter5MotionManifest(t, name)
		seen := map[uint8]int{}
		count, external := walkChuActor(t, name, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter5MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, a, c, p := decodeFighter5Motion(t, row.Before)
			if row.Kind != state.Body.ActionKind || row.SourceRegister != fmt.Sprintf("0xffff%04x", state.ObjectID) {
				t.Fatal("source5 MAIN action or register identity differs")
			}
			got, err := StepFighter5Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter5Motion(t, got, a, c, p, row.After)
			auditFighter5Motion(t, row)
			seen[row.Kind]++
		})
		if suffix == "boundaries" {
			want := map[uint8]int{0x11: 1152, 0x2d: 630, 0x2e: 512, 0x2f: 708, 0x30: 708, 0x31: 1056, 0x32: 708}
			if count != 5474 || len(seen) != len(want) || external != 0 {
				t.Fatal("incomplete source5 original private matrix", count, seen, external)
			}
			for kind, expected := range want {
				if seen[kind] != expected {
					t.Fatal("missing source5 boundary slot", kind, seen[kind], expected)
				}
			}
		} else if count < 33 || seen[0x11] < 33 {
			t.Fatal("missing actual legal-input source5 retreat calls")
		}
		t.Logf("source5 MAIN %s=%d kinds=%v independent-audio=%d", suffix, count, seen, external)
	}
}

func TestFighter5MotionRealTiming(t *testing.T) {
	checkFighter5MotionManifest(t, "fighter5-motion-sequences")
	parameters := readFighter5MotionParameters(t)
	steps, ground, air, reaction, chained := 0, 0, 0, 0, 0
	count, external := walkChuActor(t, "fighter5-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name     string
			Kind     uint8
			Initial  fighter5MotionSnapshot
			Steps    []fighter5MotionCase
			Terminal uint8
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source5 actual-clock stream", err)
		}
		state, a, c, p := decodeFighter5Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			for channel, value := range step.External {
				switch channel {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared source5 external input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing original19c96 clock call")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			a, c, p = compareFighter5Motion(t, state, a, c, p, step.Before)
			current, ok := parameters[binary.BigEndian.Uint32(a[0x90:])]
			if !ok || current != step.Before.Parameters {
				t.Fatal("current source5 movement source did not select immutable parameters")
			}
			var err error
			state, err = StepFighter5Motion(state, current)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			a, c, p = compareFighter5Motion(t, state, a, c, p, step.After)
			key := AnimationFrameKey{AnimationSet: binary.BigEndian.Uint32(a[0x50:]), ShapeSet: binary.BigEndian.Uint32(a[0x8c:]), Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("independently selected source5 timing key differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing immutable original source5 timing record")
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			a, c, p = compareFighter5Motion(t, state, a, c, p, step.AfterRecord)
			auditFighter5Motion(t, step)
			if len(step.Interrupts) != 0 || len(step.Clears) != 0 {
				t.Fatal("private source5 stream acquired an independent interrupt")
			}
			steps++
		}
		if state.Body.ActionKind != stream.Terminal {
			t.Fatal("source5 continuous endpoint differs", stream.Name, state.Body.ActionKind, stream.Terminal)
		}
		switch stream.Terminal {
		case 0:
			ground++
		case 2:
			air++
		case 3:
			reaction++
		default:
			t.Fatal("unexpected source5 terminal action")
		}
		if stream.Kind == 0x31 {
			seen := false
			for _, step := range stream.Steps {
				seen = seen || step.Kind == 0x32
			}
			if !seen {
				t.Fatal("missing actual31-to32 continuation")
			}
			chained++
		}
	})
	if count != 48 || steps != 2749 || ground != 42 || air != 4 || reaction != 2 || chained != 12 || external != 0 {
		t.Fatal("incomplete single-initial source5 real-timing evidence", count, steps, ground, air, reaction, chained, external)
	}
	t.Logf("source5 MAIN real-timing streams=%d steps=%d ground=%d air=%d action03=%d chained31-to32=%d", count, steps, ground, air, reaction, chained)
}

func TestFighter5MotionCanonicalOwnership(t *testing.T) {
	created := map[uint8]int{}
	final, preservedOwner := 0, 0
	walkChuActor(t, "fighter5-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter5MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		before, _, _, _ := decodeFighter5Motion(t, row.Before)
		after, _, _, _ := decodeFighter5Motion(t, row.After)
		if row.Kind == 0x31 {
			if uint16(before.AlternateVelocity) != uint16(after.AlternateVelocity) || uint16(uint32(before.AlternateVelocity)>>16)+1 != uint16(uint32(after.AlternateVelocity)>>16) {
				t.Fatal("source5 counter is not the high-word view of one canonical long")
			}
			if after.Body.ActionKind == 0x32 {
				final++
			}
		}
		for i, child := range before.Children {
			if child.Size == 0 && after.Children[i].Size == 256 {
				created[row.Kind]++
				if i == 0 {
					if after.ChildExtras[i].Owner != before.ObjectID || uint32(before.ChildExtras[i].Gravity)&0xffffff != uint32(after.ChildExtras[i].Gravity)&0xffffff || uint8(uint32(before.Body.Gravity)>>24) != uint8(uint32(after.ChildExtras[i].Gravity)>>24) {
						t.Fatal("projectile initializer duplicated a Gravity alias or lost the exact+fe owner")
					}
				} else if i == 1 {
					if uint16(child.VelocityX) != uint16(after.Children[i].VelocityX) || uint16(child.VerticalVelocity) != uint16(after.Children[i].VerticalVelocity) || before.ChildExtras[i] != after.ChildExtras[i] {
						t.Fatal("charge initializer overwrote unused velocity fraction, Gravity or Owner")
					}
					preservedOwner++
				} else {
					t.Fatal("unexpected source5 third child initialization")
				}
			}
		}
	})
	if created[0x2f] != 543 || created[0x30] != 543 || created[0x32] != 543 || created[0x31] != 192 || final != 144 || preservedOwner != 192 {
		t.Fatal("missing original canonical initializer paths", created, final, preservedOwner)
	}
	t.Logf("source5 original creations=%v final-constructors=%d preserved-charge-extras=%d", created, final, preservedOwner)
}

func TestFighter5MotionRetreatPoseRange(t *testing.T) {
	packed, err := os.ReadFile("../data/animation-catalog.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Frames []struct {
			CharacterID uint8 `json:"character_id"`
			Key         AnimationFrameKey
			Data        AnimationRecordData
		}
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	seen := map[uint8]bool{}
	for _, row := range catalog.Frames {
		if row.CharacterID == 5 && row.Key.Pose == 12 {
			seen[row.Key.Index] = true
			if row.Key.Index == 6 && row.Data.Flags&128 == 0 {
				t.Fatal("source5 actual last retreat record is not terminal")
			}
		}
	}
	if len(seen) != 7 {
		t.Fatal("source5 actual retreat pose has a different record count")
	}
	for index := uint8(0); index < 7; index++ {
		if !seen[index] {
			t.Fatal("missing source5 actual retreat record", index)
		}
	}
}

func TestFighter5MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter5MotionState{ChuSpecialMotionState: ChuSpecialMotionState{AlternateVelocity: 0x12345678}, ChildExtras: [3]Fighter5ChildExtra{{Gravity: -12345, Owner: 0x9000}}}
	for _, kind := range []uint8{0, 0x33, 255} {
		base.Body.ActionKind = kind
		got, err := StepFighter5Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("unsupported source5 action changed state", kind)
		}
	}
	base.Body.ActionKind = 0x11
	for _, index := range []uint8{8, 128, 255} {
		base.Body.Clock.FrameIndex = index
		got, err := StepFighter5Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("source5 retreat read outside the recovered shared table", index)
		}
	}
	base.Body.Clock.Completed = 1
	got, err := StepFighter5Motion(base, JumpParameters{})
	if err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed source5 retreat incorrectly read an invalid frame", err)
	}
	base.Body.Clock = AnimationClock{}
	base.Pool.Cursor = 1
	for _, kind := range []uint8{0x11, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32} {
		base.Body.ActionKind = kind
		if _, err := StepFighter5Motion(base, JumpParameters{}); err != nil {
			t.Fatal("source5 MAIN accessed an unused invalid FX cursor", kind, err)
		}
	}
}
