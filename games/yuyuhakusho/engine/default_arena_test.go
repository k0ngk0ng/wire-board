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

type defaultArenaSnapshot struct {
	Arena, Sky, Camera, Samples, Prefix string
	ScrollA                             string `json:"scroll_a"`
	ScrollB                             string `json:"scroll_b"`
	PanA                                string `json:"pan_a"`
	PanB                                string `json:"pan_b"`
	SourceA                             int32  `json:"source_a"`
	SourceB                             int32  `json:"source_b"`
	Vertical                            string
	ScrollMode                          uint8  `json:"scroll_mode"`
	PaletteRegisters                    string `json:"palette_registers"`
	Palettes                            string
	Seed                                uint32
	Pause, Slow                         uint8
	Shake, Pan                          int16
}

func arenaBytes(t *testing.T, s string, length int) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != length {
		t.Fatalf("invalid original arena bytes: %v %d/%d", err, len(b), length)
	}
	return b
}
func arenaWords(t *testing.T, s string, n int) []int16 {
	t.Helper()
	b := arenaBytes(t, s, n*2)
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(binary.BigEndian.Uint16(b[i*2:]))
	}
	return out
}
func arenaWordString(words []int16) string {
	b := make([]byte, len(words)*2)
	for i, w := range words {
		binary.BigEndian.PutUint16(b[i*2:], uint16(w))
	}
	return hex.EncodeToString(b)
}
func decodeArenaCamera(t *testing.T, s defaultArenaSnapshot) DefaultArenaCamera {
	b := arenaBytes(t, s.Camera, 14)
	r := DefaultArenaCamera{Used: binary.BigEndian.Uint16(b), Left: int16(binary.BigEndian.Uint16(b[2:])), Right: int16(binary.BigEndian.Uint16(b[4:])), X: int16(binary.BigEndian.Uint16(b[6:])), Y: int16(binary.BigEndian.Uint16(b[8:])), TargetX: int16(binary.BigEndian.Uint16(b[10:])), TargetY: int16(binary.BigEndian.Uint16(b[12:]))}
	p := arenaBytes(t, s.Samples, 128)
	for i := 0; i < 16; i++ {
		r.SamplesX[i] = int32(binary.BigEndian.Uint32(p[i*4:]))
		r.SamplesY[i] = int32(binary.BigEndian.Uint32(p[64+i*4:]))
	}
	return r
}
func encodeArenaCamera(s defaultArenaSnapshot, c DefaultArenaCamera) defaultArenaSnapshot {
	b := make([]byte, 14)
	for i, v := range []uint16{c.Used, uint16(c.Left), uint16(c.Right), uint16(c.X), uint16(c.Y), uint16(c.TargetX), uint16(c.TargetY)} {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	s.Camera = hex.EncodeToString(b)
	p := make([]byte, 128)
	for i := 0; i < 16; i++ {
		binary.BigEndian.PutUint32(p[i*4:], uint32(c.SamplesX[i]))
		binary.BigEndian.PutUint32(p[64+i*4:], uint32(c.SamplesY[i]))
	}
	s.Samples = hex.EncodeToString(p)
	return s
}
func decodeArenaPalette(t *testing.T, s defaultArenaSnapshot) ArenaPaletteState {
	b := arenaBytes(t, s.PaletteRegisters, 8)
	r := ArenaPaletteState{Flags: b[0], Status: b[1], Mode: binary.BigEndian.Uint16(b[2:]), Step: binary.BigEndian.Uint16(b[4:]), Target: binary.BigEndian.Uint16(b[6:])}
	p := arenaBytes(t, s.Palettes, 384)
	for i := 0; i < 64; i++ {
		r.Current[i] = binary.BigEndian.Uint16(p[i*2:])
		r.Primary[i] = binary.BigEndian.Uint16(p[128+i*2:])
		r.Secondary[i] = binary.BigEndian.Uint16(p[256+i*2:])
	}
	return r
}
func encodeArenaPalette(s defaultArenaSnapshot, p ArenaPaletteState) defaultArenaSnapshot {
	b := make([]byte, 8)
	b[0], b[1] = p.Flags, p.Status
	binary.BigEndian.PutUint16(b[2:], p.Mode)
	binary.BigEndian.PutUint16(b[4:], p.Step)
	binary.BigEndian.PutUint16(b[6:], p.Target)
	s.PaletteRegisters = hex.EncodeToString(b)
	b = make([]byte, 384)
	for i := 0; i < 64; i++ {
		binary.BigEndian.PutUint16(b[i*2:], p.Current[i])
		binary.BigEndian.PutUint16(b[128+i*2:], p.Primary[i])
		binary.BigEndian.PutUint16(b[256+i*2:], p.Secondary[i])
	}
	s.Palettes = hex.EncodeToString(b)
	return s
}
func decodeDefaultArena(t *testing.T, s defaultArenaSnapshot) (DefaultArenaState, DefaultArenaView) {
	b := arenaBytes(t, s.Arena, 128)
	r := DefaultArenaState{Stage: binary.BigEndian.Uint16(b[6:]), Camera: decodeArenaCamera(t, s), Mode: s.ScrollMode, SourceA: s.SourceA, SourceB: s.SourceB}
	for i, off := range []int{8, 12, 16} {
		r.Offsets[i] = int16(binary.BigEndian.Uint16(b[off:]))
	}
	for i, off := range []int{20, 24} {
		r.Drifts[i] = int32(binary.BigEndian.Uint32(b[off:]))
	}
	r.Phases = [2]uint16{binary.BigEndian.Uint16(b[32:]), binary.BigEndian.Uint16(b[34:])}
	copy(r.RowsA[:], arenaWords(t, s.ScrollA, 256))
	copy(r.RowsB[:], arenaWords(t, s.ScrollB, 256))
	copy(r.PanA[:], arenaWords(t, s.PanA, 224))
	copy(r.PanB[:], arenaWords(t, s.PanB, 224))
	v := arenaBytes(t, s.Vertical, 64)
	r.Vertical = [2]int16{int16(binary.BigEndian.Uint16(v)), int16(binary.BigEndian.Uint16(v[40:]))}
	w := DefaultArenaView{PauseFlags: s.Pause, SlowFlag: s.Slow, Shake: s.Shake, Pan: s.Pan}
	copy(w.HUDPrefix[:], arenaWords(t, s.Prefix, 256))
	return r, w
}
func encodeDefaultArena(t *testing.T, s defaultArenaSnapshot, r DefaultArenaState) defaultArenaSnapshot {
	b := arenaBytes(t, s.Arena, 128)
	binary.BigEndian.PutUint16(b[6:], r.Stage)
	for i, off := range []int{8, 12, 16} {
		binary.BigEndian.PutUint16(b[off:], uint16(r.Offsets[i]))
	}
	for i, off := range []int{20, 24} {
		binary.BigEndian.PutUint32(b[off:], uint32(r.Drifts[i]))
	}
	binary.BigEndian.PutUint16(b[32:], r.Phases[0])
	binary.BigEndian.PutUint16(b[34:], r.Phases[1])
	s.Arena = hex.EncodeToString(b)
	s = encodeArenaCamera(s, r.Camera)
	s.ScrollMode, s.SourceA, s.SourceB = r.Mode, r.SourceA, r.SourceB
	s.ScrollA, s.ScrollB, s.PanA, s.PanB = arenaWordString(r.RowsA[:]), arenaWordString(r.RowsB[:]), arenaWordString(r.PanA[:]), arenaWordString(r.PanB[:])
	b = arenaBytes(t, s.Vertical, 64)
	binary.BigEndian.PutUint16(b, uint16(r.Vertical[0]))
	binary.BigEndian.PutUint16(b[40:], uint16(r.Vertical[1]))
	s.Vertical = hex.EncodeToString(b)
	return s
}
func arenaSkyKind(t *testing.T, kind string) (ArenaSkyKind, int) {
	t.Helper()
	switch kind {
	case "sun":
		return ArenaSun, 0
	case "moon":
		return ArenaMoon, 128
	case "spark":
		return ArenaSpark, 256
	}
	t.Fatal("unknown sky callback")
	return 0, 0
}
func decodeArenaSky(t *testing.T, s defaultArenaSnapshot, kind string) (ArenaSkyState, ArenaSkyWorld) {
	k, off := arenaSkyKind(t, kind)
	sky := arenaBytes(t, s.Sky, 384)
	b := sky[off : off+128]
	w := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(b[i:]) }
	r := ArenaSkyState{Kind: k, ObjectID: uint16(0x9c80 + off), Behavior: l(0), Size: w(4), Stage: w(6), ScreenX: int16(w(8)), ScreenXFraction: w(10), ScreenY: int16(w(12)), GraphicsSet: l(28), TilesBase: l(32), Attributes: w(36), Layer: b[38], GraphicOffsetX: int16(w(40)), GraphicOffsetY: int16(w(42)), Parameter: w(60), Depth: w(62), X: int32(l(64)), Y: int32(l(68)), Phase: l(72), Sequence: l(80), Index: b[89], Ticks: b[91], Flags: b[92]}
	for i := range r.Cache {
		r.Cache[i] = l(44 + i*4)
	}
	p := decodeArenaPalette(t, s)
	world := ArenaSkyWorld{PauseFlags: s.Pause, SlowFlag: s.Slow, CameraY: decodeArenaCamera(t, s).Y, Pan: s.Pan, PaletteStatus: p.Status, Primary: p.Primary, RandomSeed: s.Seed}
	if k == ArenaMoon {
		world.Next = MatchObjectHeader{Behavior: binary.BigEndian.Uint32(sky[256:]), Size: binary.BigEndian.Uint16(sky[260:]), Stage: binary.BigEndian.Uint16(sky[262:])}
	}
	return r, world
}
func encodeArenaSky(t *testing.T, s defaultArenaSnapshot, r ArenaSkyState, world ArenaSkyWorld, out ArenaSkyOutcome) defaultArenaSnapshot {
	_, off := arenaSkyKind(t, map[ArenaSkyKind]string{ArenaSun: "sun", ArenaMoon: "moon", ArenaSpark: "spark"}[r.Kind])
	sky := arenaBytes(t, s.Sky, 384)
	b := sky[off : off+128]
	if out.ClearedBytes != 0 {
		clear(b[:out.ClearedBytes])
	}
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	l := func(i int, v uint32) { binary.BigEndian.PutUint32(b[i:], v) }
	l(0, r.Behavior)
	w(4, r.Size)
	w(6, r.Stage)
	w(8, uint16(r.ScreenX))
	w(10, r.ScreenXFraction)
	w(12, uint16(r.ScreenY))
	l(28, r.GraphicsSet)
	l(32, r.TilesBase)
	w(36, r.Attributes)
	b[38] = r.Layer
	w(40, uint16(r.GraphicOffsetX))
	w(42, uint16(r.GraphicOffsetY))
	for i, v := range r.Cache {
		l(44+i*4, v)
	}
	w(60, r.Parameter)
	w(62, r.Depth)
	l(64, uint32(r.X))
	l(68, uint32(r.Y))
	l(72, r.Phase)
	l(80, r.Sequence)
	b[89], b[91], b[92] = r.Index, r.Ticks, r.Flags
	if r.Kind == ArenaMoon {
		binary.BigEndian.PutUint32(sky[256:], world.Next.Behavior)
		binary.BigEndian.PutUint16(sky[260:], world.Next.Size)
		binary.BigEndian.PutUint16(sky[262:], world.Next.Stage)
	}
	s.Sky, s.Seed = hex.EncodeToString(sky), world.RandomSeed
	p := decodeArenaPalette(t, s)
	p.Primary = world.Primary
	return encodeArenaPalette(s, p)
}

type defaultArenaCall struct {
	Kind, Name             string
	Before, After          defaultArenaSnapshot
	Draw, Deleted, Spawned bool
	ClearedBytes           uint16 `json:"cleared_bytes"`
	ReferenceOnly          string `json:"reference_only"`
	External               *struct {
		SamplesX []int32 `json:"samples_x"`
		SamplesY []int32 `json:"samples_y"`
	} `json:"external_input"`
}

func runDefaultArena(t *testing.T, s defaultArenaSnapshot, kind string, library ArenaRuntimeLibrary) (defaultArenaSnapshot, ArenaSkyOutcome, error) {
	t.Helper()
	switch kind {
	case "camera":
		c, err := StepDefaultArenaCamera(decodeArenaCamera(t, s))
		return encodeArenaCamera(s, c), ArenaSkyOutcome{}, err
	case "arena":
		r, v := decodeDefaultArena(t, s)
		r, err := StepDefaultArena(r, v, library)
		return encodeDefaultArena(t, s, r), ArenaSkyOutcome{}, err
	case "sun", "moon", "spark":
		r, w := decodeArenaSky(t, s, kind)
		r, w, o, err := StepArenaSky(r, w, library)
		return encodeArenaSky(t, s, r, w, o), o, err
	case "progress":
		return encodeArenaPalette(s, AdvancePaletteFade(decodeArenaPalette(t, s))), ArenaSkyOutcome{}, nil
	case "colors":
		p, err := ApplyPaletteFade(decodeArenaPalette(t, s), library)
		return encodeArenaPalette(s, p), ArenaSkyOutcome{}, err
	}
	t.Fatal("unknown original arena call")
	return s, ArenaSkyOutcome{}, nil
}

// Stream the large natural fixture: original RAM is retained only by the test
// adapter, while production APIs receive named state and immutable rule data.
func walkDefaultArenaFixture(t *testing.T, name string, handle func(*json.Decoder, ArenaRuntimeLibrary)) (int, int, int) {
	t.Helper()
	path := "testdata/" + name + ".json.gz"
	zipped, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass       bool
		Records    int
		Calls      int
		Baseline   bool   `json:"frames_match_original_core"`
		SHA        string `json:"fixture_sha256"`
		Gzip       string `json:"gzip_sha256"`
		References int    `json:"reference_only"`
	}
	if err = json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	if _, err = io.Copy(h, z); err != nil {
		t.Fatal(err)
	}
	z.Close()
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h.Sum(nil)) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original arena fixture")
	}
	z, err = gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	d := json.NewDecoder(z)
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		t.Fatal("invalid arena payload")
	}
	token, _ = d.Token()
	if token != "library" {
		t.Fatal("library must precede call stream")
	}
	var library ArenaRuntimeLibrary
	if err = d.Decode(&library); err != nil {
		t.Fatal(err)
	}
	token, _ = d.Token()
	if token != "calls" {
		t.Fatal("missing arena calls")
	}
	token, _ = d.Token()
	if token != json.Delim('[') {
		t.Fatal("missing arena call array")
	}
	before, _ := json.Marshal(library)
	handle(d, library)
	if _, err = d.Token(); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Token(); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(library)
	if !bytes.Equal(before, after) {
		t.Fatal("arena operation mutated immutable library")
	}
	return m.Records, m.References, m.Calls
}

// Failure document, original fixtures and these tests precede default_arena.go.
func TestDefaultArenaOriginalCompleteRules(t *testing.T) {
	for _, name := range []string{"default-arena-oracle", "default-arena-boundaries"} {
		t.Run(name, func(t *testing.T) {
			count, refs, draws, deletes, spawns := 0, 0, 0, 0, 0
			expected, references, _ := walkDefaultArenaFixture(t, name, func(d *json.Decoder, lib ArenaRuntimeLibrary) {
				for d.More() {
					var row defaultArenaCall
					if err := d.Decode(&row); err != nil {
						t.Fatal(err)
					}
					got, out, err := runDefaultArena(t, row.Before, row.Kind, lib)
					if row.ReferenceOnly != "" {
						refs++
						if err == nil || !reflect.DeepEqual(got, row.Before) || out != (ArenaSkyOutcome{}) {
							t.Fatal("reference-only out-of-domain call did not fail atomically")
						}
					} else if err != nil || !reflect.DeepEqual(got, row.After) || out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || out.Spawned != row.Spawned {
						t.Fatalf("arena call%d %s/%s differs:err=%v outcome=%+v", count, row.Kind, row.Name, err, out)
					}
					if row.Draw {
						draws++
					}
					if row.Deleted {
						deletes++
					}
					if row.Spawned {
						spawns++
					}
					count++
				}
			})
			if count != expected || refs != references {
				t.Fatal("missing arena complete-call coverage")
			}
			t.Logf("calls=%d reference-only=%d draw=%d deletes=%d spawn=%d", count, refs, draws, deletes, spawns)
		})
	}
}
func TestDefaultArenaSingleInitialStreams(t *testing.T) {
	streams, steps, deletes := 0, 0, 0
	expected, _, expectedSteps := walkDefaultArenaFixture(t, "default-arena-sequences", func(d *json.Decoder, lib ArenaRuntimeLibrary) {
		for d.More() {
			var stream struct {
				Kind    string
				Initial defaultArenaSnapshot
				Calls   []defaultArenaCall
			}
			if err := d.Decode(&stream); err != nil {
				t.Fatal(err)
			}
			live := stream.Initial
			for i, row := range stream.Calls {
				if row.External != nil {
					c := decodeArenaCamera(t, live)
					if len(row.External.SamplesX) != len(row.External.SamplesY) || len(row.External.SamplesX) > 16 {
						t.Fatal("invalid external position samples")
					}
					c.Used = uint16(len(row.External.SamplesX) * 4)
					copy(c.SamplesX[:], row.External.SamplesX)
					copy(c.SamplesY[:], row.External.SamplesY)
					live = encodeArenaCamera(live, c)
				}
				var out ArenaSkyOutcome
				var err error
				live, out, err = runDefaultArena(t, live, row.Kind, lib)
				if err != nil || !reflect.DeepEqual(live, row.After) || out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes {
					t.Fatalf("arena stream %s step%d differs:%v", stream.Kind, i, err)
				}
				if out.ClearedBytes != 0 {
					deletes++
				}
				steps++
			}
			streams++
		}
	})
	if streams != expected || streams != 6 || steps != expectedSteps || deletes < 2 {
		t.Fatal("missing full arena/palette stream coverage", streams, steps, deletes)
	}
}
func TestDefaultArenaInvalidDataIsAtomic(t *testing.T) {
	c := DefaultArenaCamera{Used: 2, X: 864}
	next, err := StepDefaultArenaCamera(c)
	if err == nil || next != c {
		t.Fatal("bad sample count changed camera")
	}
	s := DefaultArenaState{Stage: 2, Camera: DefaultArenaCamera{X: 864, Y: 16}}
	v := DefaultArenaView{}
	got, err := StepDefaultArena(s, v, ArenaRuntimeLibrary{})
	if err == nil || got != s {
		t.Fatal("missing arena table leaked camera/scroll state")
	}
	v.PauseFlags = 128
	got, err = StepDefaultArena(s, v, ArenaRuntimeLibrary{})
	if err != nil || got != s {
		t.Fatal("paused arena unexpectedly required tables")
	}
	o := ArenaSkyState{Kind: ArenaSpark, ObjectID: 0x9d80, Size: 128}
	w := ArenaSkyWorld{RandomSeed: 1}
	goObj, goWorld, out, err := StepArenaSky(o, w, ArenaRuntimeLibrary{})
	if err == nil || goObj != o || goWorld != w || out != (ArenaSkyOutcome{}) {
		t.Fatal("missing particle sequence leaked initialization/RNG")
	}
	p := ArenaPaletteState{Mode: 0x0200, Target: 0x0fff}
	p.Primary[0], p.Secondary[0] = 2, 14
	np, err := ApplyPaletteFade(p, ArenaRuntimeLibrary{})
	if err == nil || np != p {
		t.Fatal("missing fade table leaked palette")
	}
}
