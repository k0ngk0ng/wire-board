package engine

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

type arena4CESnapshot struct {
	defaultArenaSnapshot
	VideoFlags   uint8 `json:"video_flags"`
	FrameCounter uint8 `json:"frame_counter"`
}
type arena4CECall struct {
	Kind, Name    string
	ArenaID       uint16 `json:"arena_id"`
	Stack         uint32
	Before, After arena4CESnapshot
	ExternalInput *struct {
		Pan                 int16
		Pause, Slow, Status uint8
		FrameCounter        uint8    `json:"frame_counter"`
		SamplesX            [4]int32 `json:"samples_x"`
		SamplesY            [4]int32 `json:"samples_y"`
		Used                uint16
	} `json:"external_input"`
	Audit struct {
		RAMBefore string `json:"ram_before"`
		AfterHash string `json:"ram_after_sha256"`
		RAMBytes  int    `json:"ram_bytes"`
		Unknown   int    `json:"unknown_writes"`
		Trace     [][6]uint32
	} `json:"audit"`
}

func arena4CEJSON(t *testing.T, path string, target any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(target); err != nil {
		t.Fatal(err)
	}
}
func arena4CELibrary(t *testing.T) ArenaSceneLibrary {
	t.Helper()
	var four, linear ArenaSceneLibrary
	arena4CEJSON(t, "../data/arena-scene-4-tables.json.gz", &four)
	arena4CEJSON(t, "../data/arena-scene-linear-rows.json.gz", &linear)
	var base struct{ Library ArenaRuntimeLibrary }
	b, err := os.ReadFile("../data/arena-runtime-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &base); err != nil {
		t.Fatal(err)
	}
	four.LinearRows, four.WaveX = linear.LinearRows, base.Library.WaveX
	if len(four.Arena4Rows) != 64 || len(four.LinearRows) != 241 || len(four.WaveX) != 1024 {
		t.Fatal("incomplete typed arena tables")
	}
	for _, row := range four.Arena4Rows {
		if len(row) != 112 {
			t.Fatal("arena4 row length")
		}
	}
	for _, row := range four.LinearRows {
		if len(row) != 128 {
			t.Fatal("linear row length")
		}
	}
	return four
}
func decodeArena4CE(t *testing.T, s arena4CESnapshot) (ArenaSceneState, DefaultArenaView, ArenaPaletteState) {
	t.Helper()
	r := ArenaSceneState{Object: decodeWorldObjectTest(t, 0x8180, s.Arena), Camera: decodeArenaCamera(t, s.defaultArenaSnapshot),
		Mode: s.ScrollMode, BackdropColor: s.VideoFlags, SourceA: s.SourceA, SourceB: s.SourceB}
	copy(r.RowsA[:], arenaWords(t, s.ScrollA, 256))
	copy(r.RowsB[:], arenaWords(t, s.ScrollB, 256))
	copy(r.PanA[:], arenaWords(t, s.PanA, 224))
	copy(r.PanB[:], arenaWords(t, s.PanB, 224))
	b := arenaBytes(t, s.Vertical, 64)
	r.Vertical = [2]int16{int16(binary.BigEndian.Uint16(b)), int16(binary.BigEndian.Uint16(b[40:]))}
	v := DefaultArenaView{PauseFlags: s.Pause, SlowFlag: s.Slow, Shake: s.Shake, Pan: s.Pan, FrameCounter: s.FrameCounter}
	copy(v.HUDPrefix[:], arenaWords(t, s.Prefix, 256))
	return r, v, decodeArenaPalette(t, s.defaultArenaSnapshot)
}
func encodeArena4CE(t *testing.T, prior arena4CESnapshot, r ArenaSceneState, p ArenaPaletteState) arena4CESnapshot {
	t.Helper()
	s := prior
	s.Arena = encodeWorldObjectTest(t, r.Object, s.Arena, 0)
	s.defaultArenaSnapshot = encodeArenaCamera(s.defaultArenaSnapshot, r.Camera)
	s.defaultArenaSnapshot = encodeArenaPalette(s.defaultArenaSnapshot, p)
	s.ScrollMode, s.VideoFlags, s.SourceA, s.SourceB = r.Mode, r.BackdropColor, r.SourceA, r.SourceB
	s.ScrollA, s.ScrollB = arenaWordString(r.RowsA[:]), arenaWordString(r.RowsB[:])
	s.PanA, s.PanB = arenaWordString(r.PanA[:]), arenaWordString(r.PanB[:])
	b := arenaBytes(t, s.Vertical, 64)
	binary.BigEndian.PutUint16(b, uint16(r.Vertical[0]))
	binary.BigEndian.PutUint16(b[40:], uint16(r.Vertical[1]))
	s.Vertical = hex.EncodeToString(b)
	return s
}
func arena4CERAMSnapshot(ram []byte) arena4CESnapshot {
	h := func(a, b int) string { return hex.EncodeToString(ram[a:b]) }
	w := func(a int) int16 { return int16(binary.BigEndian.Uint16(ram[a:])) }
	l := func(a int) int32 { return int32(binary.BigEndian.Uint32(ram[a:])) }
	return arena4CESnapshot{defaultArenaSnapshot: defaultArenaSnapshot{
		Arena: h(0x8180, 0x8280), Camera: h(0xa00e, 0xa01c), Samples: h(0xa800, 0xa880), Prefix: h(0xe600, 0xe800),
		ScrollA: h(0xe800, 0xea00), ScrollB: h(0xea00, 0xec00), PanA: h(0x6000, 0x61c0), PanB: h(0x6200, 0x63c0),
		SourceA: l(0xf598), SourceB: l(0xf59c), Vertical: h(0xe500, 0xe540), ScrollMode: ram[0xf6e7],
		PaletteRegisters: h(0xf500, 0xf508), Palettes: h(0xe000, 0xe180), Pause: ram[0xf550], Slow: ram[0xf55a], Shake: w(0xf566), Pan: w(0xf572)},
		VideoFlags: ram[0xf6df], FrameCounter: ram[0xf589]}
}
func arena4CEAudit(t *testing.T, c arena4CECall) {
	t.Helper()
	if c.Audit.RAMBytes != 65536 || c.Audit.Unknown != 0 {
		t.Fatal("incomplete RAM audit")
	}
	ram := arenaBytes(t, c.Audit.RAMBefore, 65536)
	if !reflect.DeepEqual(arena4CERAMSnapshot(ram), c.Before) {
		t.Fatal("RAM before projection differs")
	}
	var entries, exits int
	for _, e := range c.Audit.Trace {
		if e[5] == 2 {
			entries++
		}
		if e[5] == 3 {
			exits++
		}
		if e[5] != 1 {
			continue
		}
		if e[2] != 1 && e[2] != 2 && e[2] != 4 {
			t.Fatal("invalid trace write width")
		}
		for i := uint32(0); i < e[2]; i++ {
			ram[(e[1]+i)&65535] = byte(e[3] >> (8 * (e[2] - 1 - i)))
		}
	}
	if entries != exits {
		t.Fatal("IRQ return not observed")
	}
	if fmt.Sprintf("%x", sha256.Sum256(ram)) != c.Audit.AfterHash || !reflect.DeepEqual(arena4CERAMSnapshot(ram), c.After) {
		t.Fatal("complete 64KiB write trace differs from original after state")
	}
}
func runArena4CE(t *testing.T, c arena4CECall, s ArenaSceneState, v DefaultArenaView, p ArenaPaletteState, library ArenaSceneLibrary) (ArenaSceneState, ArenaPaletteState, error) {
	t.Helper()
	switch c.Kind {
	case "palette":
		return StepArena4Palette(s, v, p, library)
	case "arena":
		switch c.ArenaID {
		case 4:
			return StepArena4(s, v, p, library)
		case 12:
			n, e := StepArenaC(s, v, library)
			return n, p, e
		case 14:
			n, e := StepArenaE(s, v, library)
			return n, p, e
		}
	}
	t.Fatal("unknown original callback")
	return s, p, nil
}
func checkArena4CE(t *testing.T, c arena4CECall, s ArenaSceneState, v DefaultArenaView, p ArenaPaletteState, library ArenaSceneLibrary) (ArenaSceneState, ArenaPaletteState) {
	t.Helper()
	n, q, err := runArena4CE(t, c, s, v, p, library)
	want, _, wantPalette := decodeArena4CE(t, c.After)
	if err != nil || !reflect.DeepEqual(n, want) || q != wantPalette || !reflect.DeepEqual(encodeArena4CE(t, c.Before, n, q), c.After) {
		t.Fatalf("arena %x %s %s differs: %v\nstate equality=%v palette equality=%v", c.ArenaID, c.Kind, c.Name, err, reflect.DeepEqual(n, want), q == wantPalette)
	}
	return n, q
}
func TestArenas4CEOriginalReturns(t *testing.T) {
	library := arena4CELibrary(t)
	for _, id := range []string{"4", "c", "e"} {
		for _, kind := range []string{"natural", "boundaries"} {
			t.Run(id+"/"+kind, func(t *testing.T) {
				name := "testdata/arenas-4-c-e-" + id + "-" + kind
				f, err := os.Open(name + ".jsonl.gz")
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				z, err := gzip.NewReader(f)
				if err != nil {
					t.Fatal(err)
				}
				defer z.Close()
				d := json.NewDecoder(z)
				count := 0
				for {
					var c arena4CECall
					err := d.Decode(&c)
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					arena4CEAudit(t, c)
					s, v, p := decodeArena4CE(t, c.Before)
					checkArena4CE(t, c, s, v, p, library)
					count++
				}
				if count < 1000 {
					t.Fatalf("truncated original corpus: %d", count)
				}
				t.Logf("%d complete original returns; exact 65536-byte trace replay", count)
			})
		}
	}
}
func TestArenas4CESingleInitialStateSequences(t *testing.T) {
	library := arena4CELibrary(t)
	for _, id := range []string{"4", "c", "e"} {
		var fixture struct {
			Streams []struct {
				Kind    string
				Initial arena4CESnapshot
				Calls   []arena4CECall
			}
		}
		arena4CEJSON(t, "testdata/arenas-4-c-e-"+id+"-sequences.json.gz", &fixture)
		for _, stream := range fixture.Streams {
			t.Run(id+"/"+stream.Kind, func(t *testing.T) {
				s, v, p := decodeArena4CE(t, stream.Initial)
				encoded := stream.Initial
				if len(stream.Calls) != 180 {
					t.Fatal("truncated continuous original sequence")
				}
				for _, c := range stream.Calls {
					x := c.ExternalInput
					if x == nil {
						t.Fatal("missing declared external input")
					}
					s.Camera.Used = x.Used
					copy(s.Camera.SamplesX[:4], x.SamplesX[:])
					copy(s.Camera.SamplesY[:4], x.SamplesY[:])
					v.Pan, v.PauseFlags, v.SlowFlag, v.FrameCounter = x.Pan, x.Pause, x.Slow, x.FrameCounter
					p.Status = x.Status
					encoded = encodeArena4CE(t, encoded, s, p)
					encoded.Pan, encoded.Pause, encoded.Slow, encoded.FrameCounter = x.Pan, x.Pause, x.Slow, x.FrameCounter
					if !reflect.DeepEqual(encoded, c.Before) {
						t.Fatalf("%s carry state differs before callback", c.Name)
					}
					arena4CEAudit(t, c)
					s, p = checkArena4CE(t, c, s, v, p, library)
					encoded = encodeArena4CE(t, encoded, s, p)
				}
			})
		}
	}
}
func TestArenas4CEAtomicErrorsAndGates(t *testing.T) {
	library := arena4CELibrary(t)
	for _, id := range []uint16{4, 12, 14} {
		s := ArenaSceneState{Object: WorldObject{Header: MatchObjectHeader{Stage: 0}}, Camera: DefaultArenaCamera{X: 864, Y: 16}}
		v := DefaultArenaView{}
		p := ArenaPaletteState{}
		call := arena4CECall{ArenaID: id, Kind: "arena"}
		bad := library
		switch id {
		case 4:
			bad.Arena4Rows = nil
		case 12:
			bad.WaveX = nil
		case 14:
			bad.LinearRows = nil
		}
		n, q, err := runArena4CE(t, call, s, v, p, bad)
		if err == nil || !reflect.DeepEqual(n, s) || q != p {
			t.Fatalf("arena %x missing table did not roll back", id)
		}
		s.Camera.Used = 2
		n, q, err = runArena4CE(t, call, s, v, p, library)
		if err == nil || !reflect.DeepEqual(n, s) || q != p {
			t.Fatalf("arena %x invalid camera did not roll back", id)
		}
		for _, gate := range []DefaultArenaView{{PauseFlags: 128}, {SlowFlag: 1}} {
			n, q, err = runArena4CE(t, call, s, gate, p, bad)
			if err != nil || !reflect.DeepEqual(n, s) || q != p {
				t.Fatalf("arena %x gated call touched unavailable state", id)
			}
		}
		s.Camera.Used = 0
		v.Pan = 1
		v.Shake = 300
		n, q, err = runArena4CE(t, call, s, v, p, library)
		if err == nil || !reflect.DeepEqual(n, s) || q != p {
			t.Fatalf("arena %x invalid feedback did not roll back", id)
		}
	}
}
