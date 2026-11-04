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

type matchStartSnapshot struct {
	World, Globals, Colors, System, Registers string
	Queue                                     [4]uint8
	Options                                   [3]uint8
	ResetWords                                [4]uint16 `json:"reset_words"`
}

var matchStartObjectIDs = [10]uint16{0x8100, 0x8140, 0x8180, 0x81c0, 0x8200, 0x8240, 0x8280, 0x9c00, 0x9c80, 0x9d00}

func matchStartBytes(t *testing.T, s string, size int) []byte {
	t.Helper()
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != size {
		t.Fatal("bad original match-start bytes", e, len(b), size)
	}
	return b
}

func decodeMatchStart(t *testing.T, r matchStartSnapshot) MatchStartState {
	w := matchStartBytes(t, r.World, 0x1fc0)
	g := matchStartBytes(t, r.Globals, 512)
	sys := matchStartBytes(t, r.System, 178)
	regs := matchStartBytes(t, r.Registers, 38)
	colors := matchStartBytes(t, r.Colors, 128)
	word := func(b []byte, o int) uint16 { return binary.BigEndian.Uint16(b[o:]) }
	long := func(b []byte, o int) uint32 { return binary.BigEndian.Uint32(b[o:]) }
	s := MatchStartState{Manager: MatchObjectHeader{long(w, 0), word(w, 4), word(w, 6)}, PauseFlags: sys[0x50], ModeFlag: g[0x96], ResourceBusy: sys[0x4e], ResourceList: long(sys, 0x30), VideoMode: word(sys, 0x12),
		Arena: gword(g, 0x60), Rule: gword(g, 0x62), Layout: gword(g, 0x70), Round: gword(g, 0x7e), RoundPair: gword(g, 0x72), Options: r.Options, OptionA097: g[0x97],
		Disabled: g[0x32], SlowGate: sys[0x5b], SlowFlag: sys[0x5a], SlowTicks: word(sys, 0x58), PendingSound: g[0x30], OpeningTicks: word(g, 0x3c), ResultTicks: word(g, 0x3a), WinnerCode: word(g, 0x74), AttributeMask: word(g, 0x3e), Selected: word(g, 0x64), SoundQueue: r.Queue,
		Camera: [4]int16{int16(word(g, 0x14)), int16(word(g, 0x16)), int16(word(g, 0x18)), int16(word(g, 0x1a))}, Bounds: [2]int16{int16(word(g, 0x10)), int16(word(g, 0x12))}, PlaneSize: [2]uint16{word(sys, 8), word(sys, 10)}, MaxSprites: word(sys, 0x7a), ScrollSources: [2]uint32{long(sys, 0x98), long(sys, 0x9c)}, ResetWords: r.ResetWords,
		VideoBytes: [5]uint8{regs[0xf], regs[0x17], regs[0x19], regs[0x23], regs[0x25]}}
	for i := range s.PlaneBases {
		s.PlaneBases[i] = word(sys, 0x20+i*2)
	}
	for i := range s.Feedback {
		s.Feedback[i] = word(sys, 0x66+i*2)
	}
	for i := 0; i < 4; i++ {
		s.Characters[i], s.Controllers[i], s.Colors[i], s.BaseTeams[i] = g[0x100+i], g[0x104+i], g[0x108+i], g[0x10c+i]
	}
	for i := range s.PrimaryColors {
		s.PrimaryColors[i] = word(colors, i*2)
	}
	for i, id := range matchStartObjectIDs {
		b := w[int(id)-0x8040:]
		s.Objects[i] = MatchStartObject{Header: MatchObjectHeader{long(b, 0), word(b, 4), word(b, 6)}, Parameter: word(b, 0x18)}
	}
	for i, id := range [4]uint16{0x8400, 0x8800, 0x8c00, 0x9000} {
		b := w[int(id)-0x8040:]
		s.Fighters[i] = MatchStartFighter{Header: MatchObjectHeader{long(b, 0), word(b, 4), word(b, 6)}, TargetBit: b[0x70], Attributes: word(b, 0x24), X: int32(long(b, 0x40)), Ground: int32(long(b, 0x9c)), Depth: word(b, 0x3e), Facing: b[0x61], Layer: b[0x26], CharacterID: b[0x58], Channel: b[0x76], BaseTeam: b[0x6f], InputMode: b[0xc9], StreamPosition: long(b, 0xca), HUDOffset: word(b, 0x8a)}
	}
	return s
}
func gword(b []byte, o int) uint16 { return binary.BigEndian.Uint16(b[o:]) }

func encodeMatchStart(t *testing.T, s MatchStartState, before matchStartSnapshot) matchStartSnapshot {
	r := before
	w := matchStartBytes(t, r.World, 0x1fc0)
	g := matchStartBytes(t, r.Globals, 512)
	sys := matchStartBytes(t, r.System, 178)
	regs := matchStartBytes(t, r.Registers, 38)
	colors := make([]byte, 128)
	w16 := func(b []byte, o int, v uint16) { binary.BigEndian.PutUint16(b[o:], v) }
	w32 := func(b []byte, o int, v uint32) { binary.BigEndian.PutUint32(b[o:], v) }
	header := func(b []byte, h MatchObjectHeader) { w32(b, 0, h.Behavior); w16(b, 4, h.Size); w16(b, 6, h.Stage) }
	header(w, s.Manager)
	for i, id := range matchStartObjectIDs {
		b := w[int(id)-0x8040:]
		header(b, s.Objects[i].Header)
		w16(b, 0x18, s.Objects[i].Parameter)
	}
	for i, id := range [4]uint16{0x8400, 0x8800, 0x8c00, 0x9000} {
		b := w[int(id)-0x8040:]
		f := s.Fighters[i]
		header(b, f.Header)
		b[0x70], b[0x61], b[0x26], b[0x58], b[0x76], b[0x6f], b[0xc9] = f.TargetBit, f.Facing, f.Layer, f.CharacterID, f.Channel, f.BaseTeam, f.InputMode
		w16(b, 0x24, f.Attributes)
		w16(b, 0x3e, f.Depth)
		w32(b, 0x40, uint32(f.X))
		w32(b, 0x9c, uint32(f.Ground))
		w32(b, 0xca, f.StreamPosition)
		w16(b, 0x8a, f.HUDOffset)
	}
	for _, f := range []struct {
		o int
		v uint16
	}{{0x60, s.Arena}, {0x62, s.Rule}, {0x70, s.Layout}, {0x7e, s.Round}, {0x72, s.RoundPair}, {0x3c, s.OpeningTicks}, {0x3a, s.ResultTicks}, {0x74, s.WinnerCode}, {0x3e, s.AttributeMask}, {0x64, s.Selected}, {0x10, uint16(s.Bounds[0])}, {0x12, uint16(s.Bounds[1])}} {
		w16(g, f.o, f.v)
	}
	for i := range s.Camera {
		w16(g, 0x14+i*2, uint16(s.Camera[i]))
	}
	g[0x96], g[0x97], g[0x32], g[0x30] = s.ModeFlag, s.OptionA097, s.Disabled, s.PendingSound
	for i := 0; i < 4; i++ {
		g[0x100+i], g[0x104+i], g[0x108+i], g[0x10c+i] = s.Characters[i], s.Controllers[i], s.Colors[i], s.BaseTeams[i]
	}
	sys[0x50], sys[0x4e], sys[0x5a], sys[0x5b] = s.PauseFlags, s.ResourceBusy, s.SlowFlag, s.SlowGate
	w32(sys, 0x30, s.ResourceList)
	w16(sys, 0x12, s.VideoMode)
	w16(sys, 0x58, s.SlowTicks)
	w16(sys, 8, s.PlaneSize[0])
	w16(sys, 10, s.PlaneSize[1])
	w16(sys, 0x7a, s.MaxSprites)
	w32(sys, 0x98, s.ScrollSources[0])
	w32(sys, 0x9c, s.ScrollSources[1])
	for i := range s.PlaneBases {
		w16(sys, 0x20+i*2, s.PlaneBases[i])
	}
	for i := range s.Feedback {
		w16(sys, 0x66+i*2, s.Feedback[i])
	}
	for i, o := range []int{0xf, 0x17, 0x19, 0x23, 0x25} {
		regs[o] = s.VideoBytes[i]
	}
	for i, v := range s.PrimaryColors {
		w16(colors, i*2, v)
	}
	r.World, r.Globals, r.Colors, r.System, r.Registers = hex.EncodeToString(w), hex.EncodeToString(g), hex.EncodeToString(colors), hex.EncodeToString(sys), hex.EncodeToString(regs)
	r.Queue, r.Options, r.ResetWords = s.SoundQueue, s.Options, s.ResetWords
	return r
}

// Hardware portrait cache, text raster/DMA and VDP buffers belong to emitted
// presentation operations. Rules preserve complete fighter/global regions.
func projectMatchStartRules(t *testing.T, r matchStartSnapshot) matchStartSnapshot {
	w := matchStartBytes(t, r.World, 0x1fc0)
	for i := 8; i < 64; i++ {
		w[i] = 0
	}
	r.World = hex.EncodeToString(w)
	sys := matchStartBytes(t, r.System, 178)
	keep := map[int]bool{}
	for _, span := range [][2]int{{8, 12}, {0x12, 0x14}, {0x20, 0x2a}, {0x30, 0x34}, {0x4e, 0x4f}, {0x50, 0x51}, {0x58, 0x5c}, {0x66, 0x74}, {0x7a, 0x7c}, {0x98, 0xa0}} {
		for i := span[0]; i < span[1]; i++ {
			keep[i] = true
		}
	}
	for i := range sys {
		if !keep[i] {
			sys[i] = 0
		}
	}
	r.System = hex.EncodeToString(sys)
	regs := matchStartBytes(t, r.Registers, 38)
	for i := range regs {
		if i != 0xf && i != 0x17 && i != 0x19 && i != 0x23 && i != 0x25 {
			regs[i] = 0
		}
	}
	r.Registers = hex.EncodeToString(regs)
	return r
}

func readMatchStart(t *testing.T, name string, target any) int {
	t.Helper()
	zipped, e := os.ReadFile("testdata/" + name + ".json.gz")
	if e != nil {
		t.Fatal(e)
	}
	z, e := gzip.NewReader(bytes.NewReader(zipped))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	raw, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	mb, e := os.ReadFile("testdata/" + name + "-manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Pass     bool
		Records  int
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		Library  string `json:"library_sha256"`
	}
	if e = json.Unmarshal(mb, &m); e != nil {
		t.Fatal(e)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	lb, e := os.ReadFile("testdata/match-start-library.json")
	if e != nil {
		t.Fatal(e)
	}
	lh := sha256.Sum256(lb)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip || hex.EncodeToString(lh[:]) != m.Library {
		t.Fatal("unverified match-start evidence")
	}
	if e = json.Unmarshal(raw, target); e != nil {
		t.Fatal(e)
	}
	return m.Records
}
func matchStartLibrary(t *testing.T) MatchStartLibrary {
	t.Helper()
	b, e := os.ReadFile("testdata/match-start-library.json")
	if e != nil {
		t.Fatal(e)
	}
	var lib MatchStartLibrary
	if e = json.Unmarshal(b, &lib); e != nil {
		t.Fatal(e)
	}
	return lib
}

type matchStartCall struct {
	Name             string
	Before, After    matchStartSnapshot
	Events           []MatchStartEvent
	ResourceComplete bool `json:"resource_complete"`
}

func equalMatchStartEvents(a, b []MatchStartEvent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Original CPU fixtures and these tests precede match_start.go.
func TestMatchStartOriginalRules(t *testing.T) {
	lib := matchStartLibrary(t)
	beforeLib, _ := json.Marshal(lib)
	for _, name := range []string{"match-start-oracle", "match-start-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []matchStartCall
			count := readMatchStart(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original match-start calls")
			}
			for i, row := range rows {
				input := decodeMatchStart(t, row.Before)
				got, out, e := StepMatchStart(input, lib)
				want := decodeMatchStart(t, row.After)
				if e != nil || got != want || !equalMatchStartEvents(out.Events, row.Events) || !reflect.DeepEqual(projectMatchStartRules(t, encodeMatchStart(t, got, row.Before)), projectMatchStartRules(t, row.After)) {
					t.Fatalf("match start %d %s differs: %v\ngot=%+v\nwant=%+v\nevents=%+v/%+v", i, row.Name, e, got, want, out.Events, row.Events)
				}
			}
		})
	}
	afterLib, _ := json.Marshal(lib)
	if !bytes.Equal(beforeLib, afterLib) {
		t.Fatal("immutable match-start library changed")
	}
}
func TestMatchStartSingleInitialState(t *testing.T) {
	lib := matchStartLibrary(t)
	var streams []struct {
		Initial matchStartSnapshot
		Calls   []matchStartCall
	}
	count := readMatchStart(t, "match-start-sequences", &streams)
	if len(streams) != count || count != 4 {
		t.Fatal("missing start streams")
	}
	steps := 0
	for _, stream := range streams {
		s := decodeMatchStart(t, stream.Initial)
		encoded := stream.Initial
		for i, c := range stream.Calls {
			if c.ResourceComplete {
				s.ResourceBusy = 0
			}
			next, out, e := StepMatchStart(s, lib)
			s = next
			encoded = encodeMatchStart(t, s, encoded)
			if e != nil || s != decodeMatchStart(t, c.After) || !equalMatchStartEvents(out.Events, c.Events) || !reflect.DeepEqual(projectMatchStartRules(t, encoded), projectMatchStartRules(t, c.After)) {
				t.Fatal("match-start stream", i, e)
			}
			steps++
		}
	}
	if steps != 16 {
		t.Fatal("missing continuous starts")
	}
}
func TestMatchStartAtomicErrors(t *testing.T) {
	lib := matchStartLibrary(t)
	var rows []matchStartCall
	readMatchStart(t, "match-start-boundaries", &rows)
	base := decodeMatchStart(t, rows[0].Before)
	base.Manager.Stage = 2
	base.ResourceBusy = 0
	base.PauseFlags = 0
	base.ModeFlag = 0
	base.Characters = [4]uint8{1, 1, 1, 12}
	base.Layout = 2
	got, out, e := StepMatchStart(base, lib)
	if e == nil || got != base || len(out.Events) != 0 {
		t.Fatal("late invalid character caused partial start")
	}
	base.ResourceBusy = 3
	got, _, e = StepMatchStart(base, lib)
	if e != nil || got != base {
		t.Fatal("busy loader inspected unused invalid configuration")
	}
	base.ResourceBusy = 0
	base.Characters = [4]uint8{}
	base.Rule = 4
	base.Round = 0xffff
	got, out, e = StepMatchStart(base, lib)
	if e == nil || got != base || len(out.Events) != 0 {
		t.Fatal("undecoded round table caused partial start")
	}
}
