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

// Failure cases and full original pass captures precede chu_world.go. These
// adapters exist only in tests; production never executes or rehydrates RAM.
type chuWorldRaw struct {
	Objects, Globals, Shapes, Stream, Clock string
	DragonHistory                           string `json:"dragon_history,omitempty"`
	Seed                                    uint32
	PaletteRegisters                        string `json:"palette_registers"`
	Palettes, Cells, Samples, Registration  string
	Stats, Options                          string
	Queue                                   [4]uint8
	AudioPause                              uint8  `json:"audio_pause"`
	HUDCursor                               uint16 `json:"hud_cursor"`
	Controllers                             [8]ControllerFrame
	Vertical                                string
	PanA, PanB                              string `json:"-"`
	Sources                                 string
	ScrollMode                              uint8  `json:"scroll_mode"`
	ResourceList                            uint32 `json:"resource_list"`
	ResourceBusy                            uint8  `json:"resource_busy"`
}

// Explicit tags avoid giving both pan fields the same JSON name.
func (r *chuWorldRaw) UnmarshalJSON(data []byte) error {
	type plain chuWorldRaw
	var v struct {
		*plain
		PanA string `json:"pan_a"`
		PanB string `json:"pan_b"`
	}
	v.plain = (*plain)(r)
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	r.PanA, r.PanB = v.PanA, v.PanB
	return nil
}

type chuWorldOriginalCall struct {
	ObjectID    uint16 `json:"object_id"`
	Behavior    uint32
	Size, Stage uint16
	SavedStride uint16 `json:"saved_stride"`
}

type chuWorldOriginalPass struct {
	Frame         int
	DriverEntries []int `json:"driver_entries"`
	Calls         []chuWorldOriginalCall
	Draws         []WorldDraw
	Before, After chuWorldRaw
	Cleared       bool `json:"cleared_world"`
	QueueClears   []struct {
		PC      string
		Address uint16
		Before  uint8
	} `json:"queue_clears"`
}

func chuWorldSpanTest(id uint16) int {
	if id < 0x8400 || id >= 0x9e00 {
		return 64
	}
	if id < 0x9400 {
		return 256
	}
	return 128
}

func decodeChuWorldTest(t *testing.T, raw chuWorldRaw) ChuWorldState {
	t.Helper()
	b := attackContactBytes(t, raw.Objects, 8192)
	g := attackContactBytes(t, raw.Globals, 512)
	c := attackContactBytes(t, raw.Clock, 64)
	p := attackContactBytes(t, raw.PaletteRegisters, 20)
	stats := attackContactBytes(t, raw.Stats, 160)
	options := attackContactBytes(t, raw.Options, 8)
	w := func(b []byte, i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(b []byte, i int) int32 { return int32(binary.BigEndian.Uint32(b[i:])) }
	s := ChuWorldState{Objects: make(WorldObjects), RandomSeed: raw.Seed, Controllers: raw.Controllers,
		Clock: WorldClockState{PauseFlags: c[0], PauseMask: c[1], ControllerCount: w(c, 2), AudioPause: raw.AudioPause, SlowFlag: c[10],
			ScreenShake: w(c, 22), DurationRequest: w(c, 24), AmplitudeRequest: w(c, 26), Amplitude: w(c, 28), Decay: w(c, 30), PanAmplitude: w(c, 32), PanOffset: w(c, 34),
			Frame: w(c, 52), FrameDelay: w(c, 54), LogicFrame: w(c, 56), LogicDelay: w(c, 58)},
		SlowGate: c[11], SlowTicks: w(c, 8), EffectCursor: w(g, 0x36), EffectTileCursor: g[0x31], OccupiedLanes: g[0x42], ComboFlags: g[0xa4], DragonBankCursor: w(g, 0x38),
		PendingSound: g[0x30], SoundQueue: raw.Queue, Stream: attackContactBytes(t, raw.Stream, 1024), Stopped: g[0x9a],
		Difficulty: w(options, 2), ResetEnabled: options[7], SceneMode: w(c, 48), SceneRequest: w(c, 50), InterruptMode: w(p, 16), VideoMode: w(p, 18),
		ScrollMode: raw.ScrollMode, ResourceList: raw.ResourceList, ResourceBusy: raw.ResourceBusy,
		Match: ChuWorldMatch{Arena: w(g, 0x60), Rule: w(g, 0x62), Layout: w(g, 0x70), Round: w(g, 0x7e), RoundPair: w(g, 0x72), RoundIndex: int16(w(g, 0x6a)), Grouping: w(g, 0x88),
			ModeFlag: g[0x96], Disabled: g[0x32], OptionA097: g[0x97], EligibleChannels: g[0x7c], TeamResult: g[0x7a],
			OpeningTicks: w(g, 0x3c), ResultTicks: w(g, 0x3a), AttributeMask: w(g, 0x3e), Selected: w(g, 0x64), WinnerCode: int16(w(g, 0x74)), TeamTitles: [2]uint16{w(g, 0x84), w(g, 0x86)}}}
	for id := uint16(0x8000); id < 0xa000; {
		n := chuWorldSpanTest(id)
		s.Objects[id] = worldFirstBytesTest(t, id, b[int(id)-0x8000:int(id)-0x8000+n])
		id += uint16(n)
	}
	for i, frame := range raw.Controllers {
		s.Clock.Pressed[i] = uint8(frame.Pressed)
	}
	s.Palette = decodeArenaPalette(t, defaultArenaSnapshot{PaletteRegisters: hex.EncodeToString(p[:8]), Palettes: raw.Palettes})
	s.Camera = decodeArenaCamera(t, defaultArenaSnapshot{Camera: hex.EncodeToString(g[0xe:0x1c]), Samples: raw.Samples})
	s.Registration = FighterRegistrationState{Used: w(g, 0x34), Owners: envelopeWords(t, raw.Registration, 128)}
	copy(s.Cells[:], envelopeWords(t, raw.Cells, 1536))
	copy(s.HeadToHead[:], stats[:16])
	copy(s.WinsByCharacter[:], stats[32:96])
	copy(s.LossesByCharacter[:], stats[96:160])
	copy(s.Match.Characters[:], g[0x100:0x104])
	copy(s.Match.Controllers[:], g[0x104:0x108])
	copy(s.Match.Colors[:], g[0x108:0x10c])
	copy(s.Match.BaseTeams[:], g[0x10c:0x110])
	copy(s.Match.Options[:], options[4:7])
	if raw.DragonHistory != "" {
		history := attackContactBytes(t, raw.DragonHistory, 2048)
		for bank := range s.DragonHistory {
			for i := range s.DragonHistory[bank] {
				b := history[bank*512+i*2:]
				s.DragonHistory[bank][i] = Fighter4DragonSample{X: int16(w(b, 0)), Y: int16(w(b, 0x40)), Angle: w(b, 0x80), Depth: w(b, 0xc0), Layer: b[0x100], SkipGraphics: b[0x101], Status: b[0x140]}
			}
		}
	}
	shapes := attackContactBytes(t, raw.Shapes, 1536)
	for group := range s.Buffers {
		s.Buffers[group].Offset = w(g, 4+group*2)
		for index := range s.Buffers[group].Records {
			v := shapes[group*256+index*16:]
			s.Buffers[group].Records[index] = AnimationShapeRecord{X: int16(w(v, 0)), Y: int16(w(v, 4)), Width: v[8], Height: v[9], Owner: w(v, 12), Preserved: [4]uint16{w(v, 2), w(v, 6), w(v, 10), w(v, 14)}}
		}
	}
	s.Throws.Offset, s.Throws.Records = w(g, 12), make([]ThrowContactRecord, 16)
	for i := range s.Throws.Records {
		v := shapes[1024+i*32:]
		s.Throws.Records[i] = ThrowContactRecord{X: int16(w(v, 0)), Y: int16(w(v, 2)), Width: v[4], Height: v[5], SourcePose: v[6], TargetPose: v[7], SourceAction: v[8], TargetAction: v[9], TargetFacing: v[10], Owner: w(v, 12), OffsetX: int16(w(v, 16)), OffsetY: int16(w(v, 18))}
	}
	v := attackContactBytes(t, raw.Vertical, 64)
	s.Vertical = [2]int16{int16(w(v, 0)), int16(w(v, 40))}
	copy(s.PanA[:], arenaWords(t, raw.PanA, 224))
	copy(s.PanB[:], arenaWords(t, raw.PanB, 224))
	src := attackContactBytes(t, raw.Sources, 8)
	s.ScrollSources = [2]int32{l(src, 0), l(src, 4)}
	return s
}

func chuWorldLibrariesTest(t *testing.T) ChuWorldLibrary {
	t.Helper()
	read := func(path string, target any) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) >= 2 && b[0] == 31 && b[1] == 139 {
			z, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			b, err = io.ReadAll(z)
			z.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := json.Unmarshal(b, target); err != nil {
			t.Fatal(path, err)
		}
	}
	library := ChuWorldLibrary{Animations: make(AnimationLibrary), MovementSources: make(map[uint32]FighterMovementSpec)}
	library.Fighters, _ = initializationSpecs(t)
	for _, spec := range library.Fighters {
		library.MovementSources[spec.MovementSource] = spec.Movement
	}
	library.MovementSources[0x15946] = fighter4CompositionMovements(t)[0x15946]
	var commands struct{ Library CommandLibrary }
	read("../data/command-programs.json", &commands)
	library.Commands = commands.Library
	for _, path := range []string{"../data/animation-catalog.json.gz", "../data/chu-projectile-animation.json", "../data/fighter2-projectile-rules.json", "../data/fighter1-projectile-rules.json"} {
		var catalog struct {
			Frames []struct {
				Key  AnimationFrameKey
				Data AnimationRecordData
			}
			Animations []struct {
				Key  AnimationFrameKey
				Data AnimationRecordData
			}
		}
		read(path, &catalog)
		for _, frame := range append(catalog.Frames, catalog.Animations...) {
			library.Animations[frame.Key] = frame.Data
		}
	}
	var fighter0 struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
		Sequences []struct {
			Identity uint32
			Frames   []ChuAttachmentFrame
		}
	}
	read("../data/fighter0-projectile-rules.json", &fighter0)
	for _, frame := range fighter0.Animations {
		library.Animations[frame.Key] = frame.Data
	}
	var fighter4 struct {
		Animations []chuWorldAnimationRule
		Curve      Fighter4DragonCurve
	}
	read("testdata/fighter4-projectile-rules.json", &fighter4)
	for _, frame := range fighter4.Animations {
		library.Animations[frame.Key] = frame.Data
	}
	library.Fighter4Dragon = Fighter4DragonLibrary{Animations: library.Animations, Curve: fighter4.Curve}
	var fighter5 struct{ Frames []chuWorldAnimationRule }
	read("testdata/fighter5-projectile-data.json", &fighter5)
	for _, frame := range fighter5.Frames {
		library.Animations[frame.Key] = frame.Data
	}
	library.Fighter0Projectiles = Fighter0ProjectileLibrary{Animations: library.Animations, Sequences: make(map[uint32][]ChuAttachmentFrame)}
	for _, sequence := range fighter0.Sequences {
		library.Fighter0Projectiles.Sequences[sequence.Identity] = sequence.Frames
	}
	_, library.Attachments, _ = readAttachmentOriginal(t, "chu-attachments-oracle")
	var effects struct{ Library EffectMotionLibrary }
	read("../data/effect-sequences.json", &effects)
	library.Effects = effects.Library
	var attached attachedFXFixture
	read("testdata/attached-effects-data.json", &attached.Library)
	library.AttachedEffects = AttachedEffectLibrary{Sequences: make(map[uint32][]AttachedEffectFrame), OrbitX: attached.Library.OrbitX, OrbitY: attached.Library.OrbitY, OffsetY: attached.Library.OffsetY}
	for _, sequence := range attached.Library.Sequences {
		library.AttachedEffects.Sequences[sequence.ID] = sequence.Frames
	}
	library.MatchStart = matchStartLibrary(t)
	var arena struct{ Library ArenaRuntimeLibrary }
	read("../data/arena-runtime-rules.json", &arena)
	library.Arena = arena.Library
	return library
}

func visitChuWorldTest(t *testing.T, handle func(chuWorldOriginalPass), fixtureName ...string) int {
	t.Helper()
	name, wantRecords, wantFrames := "chu-world-oracle", 2957, 5500
	if len(fixtureName) != 0 {
		name = fixtureName[0]
		if name == "chu-world-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter10" {
			wantRecords, wantFrames = 2789, 5500
		} else if name == "chu-world-fighter0" {
			wantRecords, wantFrames = 3156, 5500
		} else if name == "chu-world-fighter0-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter3" {
			wantRecords, wantFrames = 2796, 5500
		} else if name == "chu-world-fighter3-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter2" {
			wantRecords, wantFrames = 3165, 5500
		} else if name == "chu-world-fighter2-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter8" {
			wantRecords, wantFrames = 3154, 5500
		} else if name == "chu-world-fighter8-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter1" {
			wantRecords, wantFrames = 2955, 5500
		} else if name == "chu-world-fighter1-special" {
			wantRecords, wantFrames = 1801, 3600
		} else if name == "chu-world-fighter6" {
			wantRecords, wantFrames = 2685, 5500
		} else if name == "chu-world-fighter6-special" {
			wantRecords, wantFrames = 2401, 4200
		} else if name == "chu-world-fighter6-ai" {
			wantRecords, wantFrames = 2280, 5500
		} else if name == "chu-world-fighter4" {
			wantRecords, wantFrames = 3032, 5500
		} else if name == "chu-world-fighter4-special" {
			wantRecords, wantFrames = 1801, 3602
		} else if name == "chu-world-fighter4-duo" {
			wantRecords, wantFrames = 3001, 5502
		} else if name == "chu-world-fighter5" {
			wantRecords, wantFrames = 3116, 5500
		} else if name == "chu-world-fighter5-special" {
			wantRecords, wantFrames = 1801, 3602
		}
	}
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass, Closed  bool
		Records       int
		Baseline      bool   `json:"frames_match_original_core"`
		FixtureSHA    string `json:"fixture_sha256"`
		GzipSHA       string `json:"gzip_sha256"`
		CaptureSHA    string `json:"capture_sha256"`
		Incomplete    int    `json:"incomplete_calls"`
		CheckedFrames int    `json:"frames_checked"`
	}
	if err = json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	captureName := "capture_chu_world.py"
	if name == "chu-world-fighter4-special" {
		captureName = "capture_fighter4_world.py"
	}
	if name == "chu-world-fighter4-duo" {
		captureName = "capture_fighter4_duo_world.py"
	}
	if name == "chu-world-fighter5-special" {
		captureName = "capture_fighter5_world.py"
	}
	if name == "chu-world-fighter6-ai" {
		captureName = "capture_fighter6_ai_world.py"
	}
	capture, err := os.ReadFile("../../../scripts/yuyuhakusho/" + captureName)
	if err != nil {
		t.Fatal(err)
	}
	ch := sha256.Sum256(capture)
	if !m.Pass || !m.Baseline || m.Records != wantRecords || m.Incomplete != 0 || m.CheckedFrames != wantFrames || hex.EncodeToString(zh[:]) != m.GzipSHA || hex.EncodeToString(ch[:]) != m.CaptureSHA {
		t.Fatal("whole original object passes are incomplete or not bound to capture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	reader := io.TeeReader(z, h)
	d := json.NewDecoder(reader)
	if token, err := d.Token(); err != nil || token != json.Delim('[') {
		t.Fatal("invalid original world array", err)
	}
	count := 0
	for d.More() {
		var row chuWorldOriginalPass
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		firstEntries := 0
		if name == "chu-world-fighter10" || name == "chu-world-fighter1" || name == "chu-world-fighter1-special" || name == "chu-world-fighter6" || name == "chu-world-fighter6-special" || name == "chu-world-fighter6-ai" || name == "chu-world-fighter4-duo" {
			// These inputs reach the first captured pass after3a2e in the
			// same video label. The other inputs cross that boundary earlier.
			firstEntries = 1
		}
		if count == 0 && (len(row.DriverEntries) != firstEntries || firstEntries == 1 && row.DriverEntries[0] != row.Frame) || count != 0 && (len(row.DriverEntries) != 1 || row.DriverEntries[0] < row.Frame-1 || row.DriverEntries[0] > row.Frame) {
			t.Fatal("original pass driver-entry coverage differs", row.Frame, row.DriverEntries)
		}
		handle(row)
		count++
	}
	if token, err := d.Token(); err != nil || token != json.Delim(']') {
		t.Fatal("incomplete world array", err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil || count != m.Records || hex.EncodeToString(h.Sum(nil)) != m.FixtureSHA {
		t.Fatal("original world fixture digest/count mismatch", err)
	}
	return count
}

// Only known renderer cache words and independently consumed sound queue are
// outside this rules comparison. In particular +28/+2a geometry is retained.
func comparableChuWorldTest(state ChuWorldState) ChuWorldState {
	state.Objects = cloneWorldObjects(state.Objects)
	for id, o := range state.Objects {
		o.GraphicCache = [4]uint32{}
		state.Objects[id] = o
	}
	state.SoundQueue = [4]uint8{}
	return state
}

func compareChuWorldTest(t *testing.T, got, want ChuWorldState, frame int) {
	t.Helper()
	g, w := comparableChuWorldTest(got), comparableChuWorldTest(want)
	w.MatchComplete = got.MatchComplete
	if reflect.DeepEqual(g, w) {
		return
	}
	for id, expected := range w.Objects {
		if g.Objects[id] != expected {
			t.Fatalf("world frame=%d object=%04x differs\ngot=%+v\nwant=%+v", frame, id, g.Objects[id], expected)
		}
	}
	g.Objects, w.Objects = nil, nil
	gv, wv := reflect.ValueOf(g), reflect.ValueOf(w)
	for i := 0; i < gv.NumField(); i++ {
		if !reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
			t.Fatalf("world frame=%d field=%s differs\ngot=%+v\nwant=%+v", frame, gv.Type().Field(i).Name, gv.Field(i).Interface(), wv.Field(i).Interface())
		}
	}
	t.Fatal("world differs without diagnosed field")
}

func compareChuWorldOrderTest(t *testing.T, row chuWorldOriginalPass, events []ChuWorldEvent) {
	t.Helper()
	var calls []chuWorldOriginalCall
	for _, event := range events {
		if event.Kind == "callback" {
			calls = append(calls, chuWorldOriginalCall{ObjectID: event.ObjectID, Behavior: event.Header.Behavior, Size: event.Header.Size, Stage: event.Header.Stage, SavedStride: event.Stride})
		}
	}
	if !reflect.DeepEqual(calls, row.Calls) {
		t.Fatalf("world frame=%d callback order/entry/stride differs\ngot=%+v\nwant=%+v", row.Frame, calls, row.Calls)
	}
}

// Serialize only for verification, into the original input's unowned bytes.
// Full clear events really clear the prefix; no expected output is inserted.
func compareChuWorldBytesTest(t *testing.T, state ChuWorldState, row chuWorldOriginalPass, events []ChuWorldEvent) {
	t.Helper()
	objects := attackContactBytes(t, row.Before.Objects, 8192)
	cleared := make(map[uint16]uint16)
	for _, event := range events {
		if event.Kind == "clear" {
			cleared[event.ObjectID] = event.ClearedBytes
		} else if event.Kind == "clear-world" {
			clear(objects[int(event.ObjectID)-0x8000:])
		}
	}
	for id := uint16(0x8000); id < 0xa000; {
		n := chuWorldSpanTest(id)
		base := make([]byte, 256)
		copy(base, objects[int(id)-0x8000:int(id)-0x8000+n])
		encoded := encodeWorldObjectTest(t, state.Objects[id], hex.EncodeToString(base), cleared[id])
		copy(objects[int(id)-0x8000:], attackContactBytes(t, encoded, 256)[:n])
		id += uint16(n)
	}
	wantObjects := attackContactBytes(t, row.After.Objects, 8192)
	for id := uint16(0x8000); id < 0xa000; {
		// Exactly the renderer-owned2c..3b cache. Geometry28/2a is compared.
		clear(objects[int(id)-0x8000+0x2c : int(id)-0x8000+0x3c])
		clear(wantObjects[int(id)-0x8000+0x2c : int(id)-0x8000+0x3c])
		id += uint16(chuWorldSpanTest(id))
	}
	assertBytes := func(name string, got, want []byte) {
		if !bytes.Equal(got, want) {
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("world frame=%d raw %s offset=%x got=%02x want=%02x", row.Frame, name, i, got[i], want[i])
				}
			}
		}
	}
	assertBytes("objects", objects, wantObjects)
	if row.Before.DragonHistory != "" {
		history := attackContactBytes(t, row.Before.DragonHistory, 2048)
		for bank := range state.DragonHistory {
			for i, sample := range state.DragonHistory[bank] {
				b := history[bank*512+i*2:]
				binary.BigEndian.PutUint16(b, uint16(sample.X))
				binary.BigEndian.PutUint16(b[0x40:], uint16(sample.Y))
				binary.BigEndian.PutUint16(b[0x80:], sample.Angle)
				binary.BigEndian.PutUint16(b[0xc0:], sample.Depth)
				b[0x100], b[0x101], b[0x140] = sample.Layer, sample.SkipGraphics, sample.Status
			}
		}
		assertBytes("dragon-history", history, attackContactBytes(t, row.After.DragonHistory, 2048))
	}
	g := attackContactBytes(t, row.Before.Globals, 512)
	w := func(b []byte, o int, v uint16) { binary.BigEndian.PutUint16(b[o:], v) }
	for i, buffer := range state.Buffers {
		w(g, 4+i*2, buffer.Offset)
	}
	for _, field := range []struct {
		o int
		v uint16
	}{{0xc, state.Throws.Offset}, {0xe, state.Camera.Used}, {0x10, uint16(state.Camera.Left)}, {0x12, uint16(state.Camera.Right)}, {0x14, uint16(state.Camera.X)}, {0x16, uint16(state.Camera.Y)}, {0x18, uint16(state.Camera.TargetX)}, {0x1a, uint16(state.Camera.TargetY)},
		{0x34, state.Registration.Used}, {0x36, state.EffectCursor}, {0x38, state.DragonBankCursor}, {0x3a, state.Match.ResultTicks}, {0x3c, state.Match.OpeningTicks}, {0x3e, state.Match.AttributeMask},
		{0x60, state.Match.Arena}, {0x62, state.Match.Rule}, {0x64, state.Match.Selected}, {0x6a, uint16(state.Match.RoundIndex)}, {0x70, state.Match.Layout}, {0x72, state.Match.RoundPair}, {0x74, uint16(state.Match.WinnerCode)}, {0x7e, state.Match.Round}, {0x84, state.Match.TeamTitles[0]}, {0x86, state.Match.TeamTitles[1]}, {0x88, state.Match.Grouping}} {
		w(g, field.o, field.v)
	}
	g[0x30], g[0x31], g[0x32], g[0x42] = state.PendingSound, state.EffectTileCursor, state.Match.Disabled, state.OccupiedLanes
	g[0x7a], g[0x7c], g[0x96], g[0x97], g[0x9a], g[0xa4] = state.Match.TeamResult, state.Match.EligibleChannels, state.Match.ModeFlag, state.Match.OptionA097, state.Stopped, state.ComboFlags
	copy(g[0x100:], state.Match.Characters[:])
	copy(g[0x104:], state.Match.Controllers[:])
	copy(g[0x108:], state.Match.Colors[:])
	copy(g[0x10c:], state.Match.BaseTeams[:])
	assertBytes("globals", g, attackContactBytes(t, row.After.Globals, 512))
	c := attackContactBytes(t, row.Before.Clock, 64)
	c[0], c[1], c[10], c[11] = state.Clock.PauseFlags, state.Clock.PauseMask, state.Clock.SlowFlag, state.SlowGate
	for _, field := range []struct {
		o int
		v uint16
	}{{2, state.Clock.ControllerCount}, {8, state.SlowTicks}, {22, state.Clock.ScreenShake}, {24, state.Clock.DurationRequest}, {26, state.Clock.AmplitudeRequest}, {28, state.Clock.Amplitude}, {30, state.Clock.Decay}, {32, state.Clock.PanAmplitude}, {34, state.Clock.PanOffset}, {48, state.SceneMode}, {50, state.SceneRequest}, {52, state.Clock.Frame}, {54, state.Clock.FrameDelay}, {56, state.Clock.LogicFrame}, {58, state.Clock.LogicDelay}} {
		w(c, field.o, field.v)
	}
	wantClock := attackContactBytes(t, row.After.Clock, 64)
	// Sprite1580 owns F578 count and F57c output cursor. Both are covered
	// by the separate original sprite renderer; rule clocks stay compared.
	for _, offset := range []int{0x28, 0x29, 0x2c, 0x2d} {
		c[offset], wantClock[offset] = 0, 0
	}
	assertBytes("clock", c, wantClock)
	shapes := attackContactBytes(t, row.Before.Shapes, 1536)
	for group, buffer := range state.Buffers {
		for i, record := range buffer.Records {
			v := shapes[group*256+i*16:]
			w(v, 0, uint16(record.X))
			w(v, 4, uint16(record.Y))
			v[8], v[9] = record.Width, record.Height
			w(v, 12, record.Owner)
			for i, offset := range []int{2, 6, 10, 14} {
				w(v, offset, record.Preserved[i])
			}
		}
	}
	for i, record := range state.Throws.Records {
		v := shapes[1024+i*32:]
		w(v, 0, uint16(record.X))
		w(v, 2, uint16(record.Y))
		v[4], v[5], v[6], v[7], v[8], v[9], v[10] = record.Width, record.Height, record.SourcePose, record.TargetPose, record.SourceAction, record.TargetAction, record.TargetFacing
		w(v, 12, record.Owner)
		w(v, 16, uint16(record.OffsetX))
		w(v, 18, uint16(record.OffsetY))
	}
	assertBytes("shapes", shapes, attackContactBytes(t, row.After.Shapes, 1536))
	stats := attackContactBytes(t, row.Before.Stats, 160)
	copy(stats[:16], state.HeadToHead[:])
	copy(stats[32:96], state.WinsByCharacter[:])
	copy(stats[96:160], state.LossesByCharacter[:])
	assertBytes("stats", stats, attackContactBytes(t, row.After.Stats, 160))
}

func TestChuWorldOriginalObjectPasses(t *testing.T) {
	library := chuWorldLibrariesTest(t)
	for _, scenario := range []struct {
		name                      string
		passes, callbacks, clears int
	}{{"chu-world-oracle", 2957, 38448, 1}, {"chu-world-special", 1801, 23885, 0}} {
		t.Run(scenario.name, func(t *testing.T) {
			calls, cleared := 0, 0
			count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
				state := decodeChuWorldTest(t, row.Before)
				copy := decodeChuWorldTest(t, row.Before)
				got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
				if err != nil {
					t.Fatal(row.Frame, err)
				}
				if !reflect.DeepEqual(state, copy) {
					t.Fatal("object pass modified caller-owned state", row.Frame)
				}
				want := decodeChuWorldTest(t, row.After)
				compareChuWorldOrderTest(t, row, events)
				compareChuWorldTest(t, got, want, row.Frame)
				compareChuWorldBytesTest(t, got, row, events)
				if len(row.QueueClears) == 0 && got.SoundQueue != row.After.Queue {
					t.Fatal("non-interleaved sound queue differs", row.Frame, got.SoundQueue, row.After.Queue)
				}
				if got.MatchComplete != row.Cleared {
					t.Fatal("clear-world terminal boundary differs", row.Frame)
				}
				calls += len(row.Calls)
				if row.Cleared {
					cleared++
				}
			}, scenario.name)
			if count != scenario.passes || calls != scenario.callbacks || cleared != scenario.clears {
				t.Fatal("incomplete original match pass coverage", count, calls, cleared)
			}
			t.Logf("original complete object passes=%d callbacks=%d clears=%d", count, calls, cleared)
		})
	}
}

func TestChuWorldSingleInitialMatch(t *testing.T) {
	library := chuWorldLibrariesTest(t)
	for _, scenario := range []struct {
		name                                 string
		passes, callbacks, clears, lastFrame int
	}{{"chu-world-oracle", 2957, 38448, 1, 4757}, {"chu-world-special", 1801, 23885, 0, 3600}} {
		t.Run(scenario.name, func(t *testing.T) {
			var state ChuWorldState
			initialized, previousFrame := false, 0
			calls, cleared := 0, 0
			seen := make(map[uint32]int)
			count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
				if !initialized {
					state = decodeChuWorldTest(t, row.Before)
					initialized = true
				} else {
					// Trusted controller samples are the only external input. Original
					// video label4581 has no3592 invocation, and no3a2e was observed
					// at video label4580. The CPU can cross video boundaries. Every
					// subsequent pass has one driver entry; labels are not rule ticks.
					begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
					if err != nil || outcome.ResetRequested {
						t.Fatal("continuous frame begin failed", row.Frame, err, outcome)
					}
					state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
				}
				got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
				if err != nil {
					t.Fatal("continuous object pass", row.Frame, err)
				}
				compareChuWorldOrderTest(t, row, events)
				compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
				compareChuWorldBytesTest(t, got, row, events)
				if got.MatchComplete != row.Cleared {
					t.Fatal("continuous match terminal boundary differs", row.Frame)
				}
				state = got
				state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
				if err != nil {
					t.Fatal(err)
				}
				previousFrame = row.Frame
				calls += len(row.Calls)
				for _, call := range row.Calls {
					seen[call.Behavior]++
				}
				if row.Cleared {
					cleared++
				}
			}, scenario.name)
			if count != scenario.passes || calls != scenario.callbacks || cleared != scenario.clears || state.MatchComplete != (scenario.clears != 0) || previousFrame != scenario.lastFrame {
				t.Fatal("single-initial ready match is incomplete")
			}
			if scenario.name == "chu-world-special" && (seen[0x1313e] != 3 || seen[0x132de] != 164 || seen[0x1361e] != 13) {
				t.Fatal("single-initial special stream missed projectile/charge/release callbacks", seen)
			}
			t.Logf("single initial ready match passes=%d callbacks=%d terminal_frame=%d", count, calls, previousFrame)
		})
	}
}

func TestChuWorldFutureSlotsAndSavedStride(t *testing.T) {
	state := ChuWorldState{Objects: WorldObjects{
		0x8000: {ObjectID: 0x8000, Header: MatchObjectHeader{Behavior: 0x9ad0, Size: 64, Stage: 20}},
		0x8040: {ObjectID: 0x8040, Header: MatchObjectHeader{Behavior: 0xf2a4, Size: 64, Stage: 14}},
		0x8300: {ObjectID: 0x8300, Header: MatchObjectHeader{Behavior: 0x101f2, Size: 64, Stage: 4}},
		0x8340: {ObjectID: 0x8340}, 0x8380: {ObjectID: 0x8380},
	}}
	got, events, err := StepChuWorldObjects(state, ChuWorldView{}, ChuWorldLibrary{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint16
	for _, e := range events {
		if e.Kind == "callback" {
			ids = append(ids, e.ObjectID)
		}
	}
	if !reflect.DeepEqual(ids, []uint16{0x8000, 0x8040, 0x8300, 0x8340, 0x8380}) || got.Objects[0x8340].Header.Stage != 2 || got.Objects[0x8380].Header.Stage != 2 {
		t.Fatal("spawned future slots did not execute within this ordered pass", ids)
	}
	state.Objects[0x8300] = WorldObject{ObjectID: 0x8300, Header: MatchObjectHeader{Behavior: 0x101f2, Size: 64, Stage: 12}, Aux0A: 0xffff, Aux18: -1}
	state.Objects[0x8340] = WorldObject{ObjectID: 0x8340, Header: MatchObjectHeader{Behavior: 0xfe78, Size: 64, Stage: 2}}
	got, events, err = StepChuWorldObjects(state, ChuWorldView{}, ChuWorldLibrary{})
	if err != nil || got.Objects[0x8300].Header.Behavior != 0 || got.Objects[0x8300].Aux0A != 0 || got.Objects[0x8300].Aux18 != 0 {
		t.Fatal("whole canonical prefix was not cleared", err)
	}
	ids = nil
	for _, e := range events {
		if e.Kind == "callback" {
			ids = append(ids, e.ObjectID)
		}
	}
	if !reflect.DeepEqual(ids, []uint16{0x8000, 0x8040, 0x8300, 0x8340}) {
		t.Fatal("deleted object stride was reread after callback", ids)
	}
}

func TestChuWorldErrorsAreAtomic(t *testing.T) {
	base := ChuWorldState{Objects: WorldObjects{
		0x8040: {ObjectID: 0x8040, Header: MatchObjectHeader{Behavior: 0xf2a4, Size: 64, Stage: 10}},
		0x8200: {ObjectID: 0x8200, Header: MatchObjectHeader{Behavior: 0xfe78, Size: 64}},
	}, Registration: FighterRegistrationState{Owners: make([]uint16, 64)}, Throws: ThrowContactBuffer{Records: make([]ThrowContactRecord, 16)}, Stream: make([]byte, 1024)}
	for _, kind := range []string{"unknown", "stride-zero", "stride-unaligned", "identity", "fighter", "wait-stage", "world-id"} {
		t.Run(kind, func(t *testing.T) {
			state := base
			state.Objects = cloneWorldObjects(base.Objects)
			o := WorldObject{ObjectID: 0x8400, Header: MatchObjectHeader{Behavior: 0xdeadbeef, Size: 256}}
			switch kind {
			case "stride-zero":
				o.Header = MatchObjectHeader{Behavior: 0xfe78}
			case "stride-unaligned":
				o.Header = MatchObjectHeader{Behavior: 0xfe78, Size: 15}
			case "identity":
				o.ObjectID++
			case "fighter":
				o.Header.Behavior, o.Header.Stage, o.Tag = 0x12668, 2, 1
			case "wait-stage":
				o.Header.Behavior, o.Header.Stage = 0x9ad0, 18
			case "world-id":
				o.ObjectID = 0xa000
			}
			state.Objects[0x8400] = o
			before, _ := json.Marshal(state)
			got, events, err := StepChuWorldObjects(state, ChuWorldView{}, ChuWorldLibrary{})
			after, _ := json.Marshal(state)
			if err == nil || len(events) != 0 || !reflect.DeepEqual(got, state) || !bytes.Equal(before, after) {
				t.Fatal("late object error leaked preceding mutations", kind, err)
			}
		})
	}
}

func TestChuWorldRuleBundle(t *testing.T) {
	zipped, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	library, err := DecodeChuWorldRules(raw)
	if err != nil || !reflect.DeepEqual(library, chuWorldLibrariesTest(t)) {
		t.Fatal("semantic runtime bundle differs from independently decoded source libraries", err)
	}
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`{"schema":999}`), []byte(`{"schema":1,"fighters":[]}`)} {
		if _, err := DecodeChuWorldRules(raw); err == nil {
			t.Fatal("incomplete semantic rules accepted")
		}
	}
}
