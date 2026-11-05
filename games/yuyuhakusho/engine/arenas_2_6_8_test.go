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
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ARENAS_2_6_8_FAILURES.md and the pinned CPU captures precede the production
// callbacks. Raw byte layout exists only in this original-program adapter.
type arena268Snapshot struct {
	Arena, Camera, Samples, Prefix string
	ScrollA                        string `json:"scroll_a"`
	ScrollB                        string `json:"scroll_b"`
	PanA                           string `json:"pan_a"`
	PanB                           string `json:"pan_b"`
	SourceA                        int32  `json:"source_a"`
	SourceB                        int32  `json:"source_b"`
	Vertical                       string
	ScrollMode                     uint8 `json:"scroll_mode"`
	BackdropColor                  uint8 `json:"backdrop_color"`
	Pause, Slow                    uint8
	Shake, Pan                     int16
}

type arena268Delta struct {
	Offset        int
	Before, After uint8
	Owner         string
}

type arena268Call struct {
	ArenaID       uint16 `json:"arena_id"`
	Name          string
	Frame         int
	EntrySP       uint32 `json:"entry_sp"`
	Before, After arena268Snapshot
	ReferenceOnly string          `json:"reference_only"`
	RAMChanges    []arena268Delta `json:"ram_changes"`
	Timeline      []struct {
		Order   int
		Owner   string
		Event   string
		SP      uint32
		Address int
		Before  uint8
		After   uint8
		Changes []arena268Delta
	}
	ExternalInput *struct {
		SamplesX []int32 `json:"samples_x"`
		SamplesY []int32 `json:"samples_y"`
		Used     uint16
		Pause    uint8
		Slow     uint8
		Pan      int16
		Shake    int16
	} `json:"external_input"`
}

type arena268Stream struct {
	Name    string
	Initial arena268Snapshot
	Calls   []arena268Call
}

// These digests are filled from the original-core-checked captures before
// recording the missing-own-symbol red run and before writing implementation.
var arena268Frozen = map[string][2]string{
	"arena2-oracle":     {"3b3f8fc7e95837abe7a03aa9c933c9743f8606d1783c8f09de610fee5dda4c26", "ec28bdc9c9a92dad0856fffe6ec32a54c8abcc2af6fa51d119e5a0391323072e"},
	"arena2-boundaries": {"a58a83191bd3740c98e6f9e0b5717982f684a9c7f6a8a5a71983f223579f0bef", "451a5dd3fc2d300e4f69763d4897c9c2f4b220a9a68dcbde20fb056e34077b61"},
	"arena2-sequences":  {"daef8f2544cceea21520bcad5870a22f994c5688e560a6f242c9f5ed7f7a6b0d", "056ab8402c6cc62da37c792658508c5caa95a6ba9ef40c42df839adfd599dc33"},
	"arena6-oracle":     {"abbc8b21ee9056e295bbc286ce5bfb3c8adeca70e22e71f32196edb06d906998", "0a7be1613251d015b0786189484d4cb4f21607606f52fb7454d88ee697d4b23f"},
	"arena6-boundaries": {"f8be7924ca6c0a56ae6954a073b1c83d76d68d1df8eea8a0a571035d07260e3e", "cb4a4f91f80426907b257c4f59764846ef1ebf49e48f314842ed087930a4ceb5"},
	"arena6-sequences":  {"e7791cf3647e2c497b5ffc55e890658e6747d41d375ea25f4bdf8d8d933098be", "999874e8463b51dc585087e516b731a1d52547d42fd06d96fc55b3121617fc26"},
	"arena8-oracle":     {"7776828287baafe06b86a9c5364b3246ecb8368f05927f18d08c45d4187fe7c7", "9a479c24d44cab753d36c99a2d0a1d192b9ce9329979bdbe18672fb870f05e1d"},
	"arena8-boundaries": {"a1de2037e4ed468bf5876b6007a7a6222b5ac7bee36675e01c974c34835e8693", "e57424265547f5ac2e4a5fce0ea9822ab3c981974902d87aa2aea4368e8781df"},
	"arena8-sequences":  {"568ef778576628999f31e5dd380aee65a234d4498bee54c4d0ac2100a9faa4a5", "58686a1ae52931a7923d7beac2bc5810b36dbb0dcee02f44da3bdb96e8c80ecb"},
}

func arena268Bytes(t *testing.T, text string, count int) []byte {
	t.Helper()
	b, err := hex.DecodeString(text)
	if err != nil || len(b) != count {
		t.Fatalf("invalid arena original bytes: %v, %d/%d", err, len(b), count)
	}
	return b
}

func arena268Words(t *testing.T, text string, out []int16) {
	t.Helper()
	b := arena268Bytes(t, text, len(out)*2)
	for i := range out {
		out[i] = int16(binary.BigEndian.Uint16(b[i*2:]))
	}
}

func arena268WordText(words []int16) string {
	b := make([]byte, len(words)*2)
	for i, value := range words {
		binary.BigEndian.PutUint16(b[i*2:], uint16(value))
	}
	return hex.EncodeToString(b)
}

func decodeArena268(t *testing.T, raw arena268Snapshot) (ArenaSceneState, DefaultArenaView) {
	t.Helper()
	b := arena268Bytes(t, raw.Arena, 128)
	w := func(off int) uint16 { return binary.BigEndian.Uint16(b[off:]) }
	l := func(off int) uint32 { return binary.BigEndian.Uint32(b[off:]) }
	o := WorldObject{ObjectID: 0x8180, Header: MatchObjectHeader{Behavior: l(0), Size: w(4), Stage: w(6)},
		ScreenX: int16(w(8)), Aux0A: w(10), ScreenY: int16(w(12)), Aux0E: w(14),
		Aux18: int32(l(0x18)), GraphicsSet: l(0x1c), TilesBase: l(0x20), SkipGraphics: b[0x27]}
	o.Body.VelocityX, o.Body.VerticalVelocity = int32(l(0x10)), int32(l(0x14))
	o.Body.Attributes, o.Body.Layer = w(0x24), b[0x26]
	s := ArenaSceneState{Object: o, Mode: raw.ScrollMode, BackdropColor: raw.BackdropColor, SourceA: raw.SourceA, SourceB: raw.SourceB}
	c := arena268Bytes(t, raw.Camera, 14)
	s.Camera = DefaultArenaCamera{Used: binary.BigEndian.Uint16(c), Left: int16(binary.BigEndian.Uint16(c[2:])),
		Right: int16(binary.BigEndian.Uint16(c[4:])), X: int16(binary.BigEndian.Uint16(c[6:])), Y: int16(binary.BigEndian.Uint16(c[8:])),
		TargetX: int16(binary.BigEndian.Uint16(c[10:])), TargetY: int16(binary.BigEndian.Uint16(c[12:]))}
	p := arena268Bytes(t, raw.Samples, 128)
	for i := range s.Camera.SamplesX {
		s.Camera.SamplesX[i] = int32(binary.BigEndian.Uint32(p[i*4:]))
		s.Camera.SamplesY[i] = int32(binary.BigEndian.Uint32(p[64+i*4:]))
	}
	arena268Words(t, raw.ScrollA, s.RowsA[:])
	arena268Words(t, raw.ScrollB, s.RowsB[:])
	arena268Words(t, raw.PanA, s.PanA[:])
	arena268Words(t, raw.PanB, s.PanB[:])
	v := arena268Bytes(t, raw.Vertical, 64)
	s.Vertical = [2]int16{int16(binary.BigEndian.Uint16(v)), int16(binary.BigEndian.Uint16(v[40:]))}
	view := DefaultArenaView{PauseFlags: raw.Pause, SlowFlag: raw.Slow, Shake: raw.Shake, Pan: raw.Pan}
	arena268Words(t, raw.Prefix, view.HUDPrefix[:])
	return s, view
}

func encodeArena268(t *testing.T, old arena268Snapshot, s ArenaSceneState) arena268Snapshot {
	t.Helper()
	b := arena268Bytes(t, old.Arena, 128)
	w := func(off int, value uint16) { binary.BigEndian.PutUint16(b[off:], value) }
	l := func(off int, value uint32) { binary.BigEndian.PutUint32(b[off:], value) }
	o := s.Object
	l(0, o.Header.Behavior)
	w(4, o.Header.Size)
	w(6, o.Header.Stage)
	w(8, uint16(o.ScreenX))
	w(10, o.Aux0A)
	w(12, uint16(o.ScreenY))
	w(14, o.Aux0E)
	l(0x10, uint32(o.Body.VelocityX))
	l(0x14, uint32(o.Body.VerticalVelocity))
	l(0x18, uint32(o.Aux18))
	l(0x1c, o.GraphicsSet)
	l(0x20, o.TilesBase)
	w(0x24, o.Body.Attributes)
	b[0x26], b[0x27] = o.Body.Layer, o.SkipGraphics
	old.Arena = hex.EncodeToString(b)
	b = make([]byte, 14)
	c := s.Camera
	for i, value := range []uint16{c.Used, uint16(c.Left), uint16(c.Right), uint16(c.X), uint16(c.Y), uint16(c.TargetX), uint16(c.TargetY)} {
		binary.BigEndian.PutUint16(b[i*2:], value)
	}
	old.Camera = hex.EncodeToString(b)
	b = make([]byte, 128)
	for i := range c.SamplesX {
		binary.BigEndian.PutUint32(b[i*4:], uint32(c.SamplesX[i]))
		binary.BigEndian.PutUint32(b[64+i*4:], uint32(c.SamplesY[i]))
	}
	old.Samples = hex.EncodeToString(b)
	old.ScrollA, old.ScrollB = arena268WordText(s.RowsA[:]), arena268WordText(s.RowsB[:])
	old.PanA, old.PanB = arena268WordText(s.PanA[:]), arena268WordText(s.PanB[:])
	old.SourceA, old.SourceB, old.ScrollMode, old.BackdropColor = s.SourceA, s.SourceB, s.Mode, s.BackdropColor
	b = arena268Bytes(t, old.Vertical, 64)
	binary.BigEndian.PutUint16(b, uint16(s.Vertical[0]))
	binary.BigEndian.PutUint16(b[40:], uint16(s.Vertical[1]))
	old.Vertical = hex.EncodeToString(b)
	return old
}

func stepArena268(id uint16, state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	switch id {
	case 2:
		return StepArena2(state, view, library)
	case 6:
		return StepArena6(state, view, library)
	case 8:
		return StepArena8(state, view, library)
	default:
		return state, fmt.Errorf("invalid original arena id %x", id)
	}
}

func arena268Difference(got, want ArenaSceneState) string {
	for _, row := range []struct {
		name      string
		got, want []int16
	}{{"RowsA", got.RowsA[:], want.RowsA[:]}, {"RowsB", got.RowsB[:], want.RowsB[:]},
		{"PanA", got.PanA[:], want.PanA[:]}, {"PanB", got.PanB[:], want.PanB[:]}} {
		for i := range row.got {
			if row.got[i] != row.want[i] {
				return fmt.Sprintf("%s[%d]: got %d want %d", row.name, i, row.got[i], row.want[i])
			}
		}
	}
	return fmt.Sprintf("object/metadata differ: object equal=%t camera equal=%t vertical=%v/%v sources=%d,%d/%d,%d modes=%d,%d/%d,%d",
		reflect.DeepEqual(got.Object, want.Object), reflect.DeepEqual(got.Camera, want.Camera), got.Vertical, want.Vertical,
		got.SourceA, got.SourceB, want.SourceA, want.SourceB, got.Mode, got.BackdropColor, want.Mode, want.BackdropColor)
}

func arena268Tracked(offset int) bool {
	return offset >= 0x8180 && offset < 0x8200 || offset >= 0xa00e && offset < 0xa01c ||
		offset >= 0xa800 && offset < 0xa880 || offset >= 0xe500 && offset < 0xe540 ||
		offset >= 0xe600 && offset < 0xec00 || offset >= 0x6000 && offset < 0x61c0 ||
		offset >= 0x6200 && offset < 0x63c0 || offset >= 0xf598 && offset < 0xf5a0 ||
		offset == 0xf6e7 || offset == 0xf6df
}

func verifyArena268Delta(t *testing.T, row arena268Call) {
	t.Helper()
	type cell struct{ first, current uint8 }
	seen := make(map[int]cell)
	irqActive := false
	var audio []struct {
		address int
		before  uint8
	}
	for i, part := range row.Timeline {
		if part.Order != i {
			t.Fatalf("%s: unordered CPU ownership timeline", row.Name)
		}
		switch part.Event {
		case "":
			if part.Owner != "arena" && part.Owner != "irq" || (part.Owner == "irq") != irqActive {
				t.Fatalf("%s: incorrect IRQ segment ownership", row.Name)
			}
		case "irq-entry":
			if irqActive {
				t.Fatal("nested original IRQ without return")
			}
			irqActive = true
		case "irq-exit":
			if !irqActive || len(audio) != 0 {
				t.Fatal("unmatched original IRQ return")
			}
			irqActive = false
		case "audio-clear-entry":
			if !irqActive {
				t.Fatal("audio consumer outside its actual IRQ owner")
			}
			audio = append(audio, struct {
				address int
				before  uint8
			}{part.Address, part.Before})
		case "audio-clear-exit":
			if len(audio) == 0 || audio[len(audio)-1].address != part.Address ||
				audio[len(audio)-1].before != part.Before || part.After != 0 {
				t.Fatal("reordered original audio queue consumer")
			}
			audio = audio[:len(audio)-1]
		default:
			t.Fatalf("unknown CPU ownership event %q", part.Event)
		}
		for _, d := range part.Changes {
			if d.Offset < 0 || d.Offset >= 65536 || d.Before == d.After {
				t.Fatalf("%s: invalid full RAM delta", row.Name)
			}
			switch d.Owner {
			case "arena":
				if !arena268Tracked(d.Offset) {
					t.Fatalf("%s: unexplained arena write %04x", row.Name, d.Offset)
				}
			case "arena-call-stack":
				distance := (int(row.EntrySP) - d.Offset) & 65535
				if distance < 1 || distance > 8 || arena268Tracked(d.Offset) {
					t.Fatalf("%s: invalid arena call-stack ownership", row.Name)
				}
			case "irq":
				if !irqActive || arena268Tracked(d.Offset) {
					t.Fatalf("%s: external IRQ changed the modeled arena view at %04x", row.Name, d.Offset)
				}
			case "irq-exception-stack":
				if i+1 >= len(row.Timeline) || row.Timeline[i+1].Event != "irq-entry" {
					t.Fatal("missing actual IRQ entry for exception stack write")
				}
				distance := (d.Offset - int(row.Timeline[i+1].SP)) & 65535
				if distance >= 6 {
					t.Fatal("exception stack write exceeds original CPU frame")
				}
				if arena268Tracked(d.Offset) {
					t.Fatalf("%s: external IRQ changed the modeled arena view at %04x", row.Name, d.Offset)
				}
			default:
				t.Fatalf("%s: unknown RAM delta owner %q", row.Name, d.Owner)
			}
			previous, ok := seen[d.Offset]
			if !ok {
				previous = cell{d.Before, d.Before}
			}
			if previous.current != d.Before {
				t.Fatalf("%s: reordered RAM write %04x", row.Name, d.Offset)
			}
			previous.current = d.After
			seen[d.Offset] = previous
		}
	}
	if irqActive || len(audio) != 0 {
		t.Fatal("incomplete original IRQ/audio owner")
	}
	var offsets []int
	for offset, c := range seen {
		if c.first != c.current {
			offsets = append(offsets, offset)
		}
	}
	sort.Ints(offsets)
	if len(offsets) != len(row.RAMChanges) {
		t.Fatalf("%s: full64KiB delta has missing writes", row.Name)
	}
	for i, offset := range offsets {
		c, actual := seen[offset], row.RAMChanges[i]
		if actual.Offset != offset || actual.Before != c.first || actual.After != c.current {
			t.Fatalf("%s: complete RAM delta differs at %04x", row.Name, offset)
		}
	}
}

func walkArena268(t *testing.T, name string, onCall func(ArenaSceneLibrary, arena268Call), onStream func(ArenaSceneLibrary, arena268Stream)) int {
	t.Helper()
	frozen, ok := arena268Frozen[name]
	if !ok {
		t.Fatalf("original arena fixture is not frozen: %s", name)
	}
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	var manifest struct {
		Pass     bool
		Identity struct {
			ROMSHA  string `json:"rom_sha256"`
			CoreSHA string `json:"core_sha256"`
		}
		Baseline struct {
			Steps int  `json:"video_steps"`
			Match bool `json:"all_ram_video_match"`
		}
		Complete64K bool   `json:"complete_64k_delta"`
		Replay64K   bool   `json:"ordered_timeline_replay_matches_64k"`
		Unknown     int    `json:"unexplained_changes"`
		Incomplete  int    `json:"incomplete_calls"`
		GzipSHA     string `json:"gzip_sha256"`
		RawSHA      string `json:"fixture_sha256"`
	}
	if err != nil || json.Unmarshal(manifestBytes, &manifest) != nil || !manifest.Pass ||
		manifest.Identity.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" ||
		manifest.Identity.CoreSHA != "e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306" ||
		manifest.Baseline.Steps != 3602 || !manifest.Baseline.Match || !manifest.Complete64K || !manifest.Replay64K ||
		manifest.Unknown != 0 || manifest.Incomplete != 0 || manifest.GzipSHA != frozen[0] || manifest.RawSHA != frozen[1] {
		t.Fatalf("missing complete original control/ownership proof: %s: %v", name, err)
	}
	compressed, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(compressed)) != frozen[0] {
		t.Fatalf("changed original arena fixture %s: %v", name, err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	r := io.TeeReader(z, h)
	d := json.NewDecoder(r)
	if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
		t.Fatal("invalid original arena root")
	}
	var library ArenaSceneLibrary
	count := 0
	for d.More() {
		key, err := d.Token()
		if err != nil {
			t.Fatal(err)
		}
		switch key {
		case "library":
			if err := d.Decode(&library); err != nil {
				t.Fatal(err)
			}
		case "calls":
			if tok, err := d.Token(); err != nil || tok != json.Delim('[') {
				t.Fatal("invalid original arena calls")
			}
			for d.More() {
				if strings.HasSuffix(name, "-sequences") {
					var stream arena268Stream
					if err := d.Decode(&stream); err != nil {
						t.Fatal(err)
					}
					count += len(stream.Calls)
					if onStream != nil {
						onStream(library, stream)
					}
				} else {
					var row arena268Call
					if err := d.Decode(&row); err != nil {
						t.Fatal(err)
					}
					count++
					if onCall != nil {
						onCall(library, row)
					}
				}
			}
			if _, err := d.Token(); err != nil {
				t.Fatal(err)
			}
		default:
			var ignored json.RawMessage
			if err := d.Decode(&ignored); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := d.Token(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, r); err != nil || fmt.Sprintf("%x", h.Sum(nil)) != frozen[1] {
		t.Fatalf("changed decompressed original arena fixture %s: %v", name, err)
	}
	if len(library.LinearRows) != 241 || count == 0 {
		t.Fatalf("missing original arena table or calls: %s", name)
	}
	return count
}

func TestArena268OriginalCallbacks(t *testing.T) {
	for _, id := range []uint16{2, 6, 8} {
		for _, suffix := range []string{"oracle", "boundaries"} {
			name := fmt.Sprintf("arena%x-%s", id, suffix)
			t.Run(name, func(t *testing.T) {
				count := walkArena268(t, name, func(library ArenaSceneLibrary, row arena268Call) {
					verifyArena268Delta(t, row)
					before, view := decodeArena268(t, row.Before)
					got, err := stepArena268(id, before, view, library)
					if row.ReferenceOnly != "" {
						if err == nil || !reflect.DeepEqual(got, before) {
							t.Fatalf("%s: unrepresented original feedback must fail atomically", row.Name)
						}
						return
					}
					want, _ := decodeArena268(t, row.After)
					if err != nil || !reflect.DeepEqual(got, want) || encodeArena268(t, row.Before, got) != row.After {
						t.Fatalf("%s frame%d: original callback differs: %v; %s", row.Name, row.Frame, err, arena268Difference(got, want))
					}
				}, nil)
				t.Logf("%s: %d complete original CPU callbacks", name, count)
			})
		}
	}
}

func TestArena268SingleInitialStreams(t *testing.T) {
	for _, id := range []uint16{2, 6, 8} {
		name := fmt.Sprintf("arena%x-sequences", id)
		t.Run(name, func(t *testing.T) {
			count := walkArena268(t, name, nil, func(library ArenaSceneLibrary, stream arena268Stream) {
				live, view := decodeArena268(t, stream.Initial)
				for _, row := range stream.Calls {
					input := row.ExternalInput
					if input == nil || len(input.SamplesX) != 4 || len(input.SamplesY) != 4 {
						t.Fatal("missing explicit stream external input")
					}
					copy(live.Camera.SamplesX[:], input.SamplesX)
					copy(live.Camera.SamplesY[:], input.SamplesY)
					live.Camera.Used = input.Used
					view.PauseFlags, view.SlowFlag, view.Pan, view.Shake = input.Pause, input.Slow, input.Pan, input.Shake
					before, _ := decodeArena268(t, row.Before)
					if !reflect.DeepEqual(live, before) {
						t.Fatalf("%s: single initial state diverged before call", row.Name)
					}
					var err error
					live, err = stepArena268(id, live, view, library)
					want, _ := decodeArena268(t, row.After)
					if err != nil || !reflect.DeepEqual(live, want) {
						t.Fatalf("%s: continuous original state differs: %v", row.Name, err)
					}
					verifyArena268Delta(t, row)
				}
			})
			t.Logf("%s: %d original steps from three initial states", name, count)
		})
	}
}

func TestArena268InvalidDataIsAtomic(t *testing.T) {
	for _, id := range []uint16{2, 6, 8} {
		var library ArenaSceneLibrary
		var original ArenaSceneState
		walkArena268(t, fmt.Sprintf("arena%x-oracle", id), func(lib ArenaSceneLibrary, row arena268Call) {
			if library.LinearRows == nil {
				library = lib
				original, _ = decodeArena268(t, row.Before)
			}
		}, nil)
		original.Object.Header.Stage, original.Camera.Used = 0, 16
		for i := 0; i < 4; i++ {
			original.Camera.SamplesX[i], original.Camera.SamplesY[i] = 1024<<16, 256<<16
		}
		check := func(name string, state ArenaSceneState, view DefaultArenaView, lib ArenaSceneLibrary) {
			t.Helper()
			got, err := stepArena268(id, state, view, lib)
			if err == nil || !reflect.DeepEqual(got, state) {
				t.Fatalf("arena%x %s: late failure changed caller state", id, name)
			}
		}
		missing := library
		missing.LinearRows = nil
		check("missing rows after init/camera", original, DefaultArenaView{}, missing)
		short := library
		short.LinearRows = make(map[int16][]int16, len(library.LinearRows))
		for x, row := range library.LinearRows {
			short.LinearRows[x] = row[:127]
		}
		check("short source row", original, DefaultArenaView{}, short)
		if id != 6 {
			missing = library
			missing.WaveX = nil
			check("missing signed-byte waveform", original, DefaultArenaView{}, missing)
		} else {
			noWave := library
			noWave.WaveX = nil
			got, err := stepArena268(id, original, DefaultArenaView{}, noWave)
			want, wantErr := stepArena268(id, original, DefaultArenaView{}, library)
			if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("arena6 inspected an unused waveform")
			}
		}
		bad := original
		bad.Camera.Used = 2
		check("invalid camera sample offset", bad, DefaultArenaView{}, library)
		bad.Camera.Used, bad.Camera.Y = 0, 36
		check("late feedback view failure", bad, DefaultArenaView{Pan: 1}, library)
		for _, view := range []DefaultArenaView{{PauseFlags: 128}, {SlowFlag: 255}} {
			got, err := stepArena268(id, bad, view, ArenaSceneLibrary{})
			if err != nil || !reflect.DeepEqual(got, bad) {
				t.Fatalf("arena%x: gated callback inspected unavailable dependencies", id)
			}
		}
	}
}
