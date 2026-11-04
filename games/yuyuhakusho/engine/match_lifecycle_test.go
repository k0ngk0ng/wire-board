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

type matchOriginalSnapshot struct {
	World            string
	Globals          string
	PaletteRegisters string `json:"palette_registers"`
	PaletteCurrent   string `json:"palette_current"`
	PaletteSecondary string `json:"palette_secondary"`
	PauseFlags       uint8  `json:"pause_flags"`
	PauseMask        uint8  `json:"pause_mask"`
	Pressed          [4]uint8
	Queue            [4]uint8
	Stats            string
	OptionFF14       uint8 `json:"option_ff14"`
	OptionFF16       uint8 `json:"option_ff16"`
}

func matchBytes(t *testing.T, s string, count int) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != count {
		t.Fatalf("invalid original match bytes: %v %d/%d", err, len(b), count)
	}
	return b
}

func decodeMatchLifecycle(t *testing.T, raw matchOriginalSnapshot) MatchLifecycleState {
	t.Helper()
	w := matchBytes(t, raw.World, 0x1fc0)
	g := matchBytes(t, raw.Globals, 512)
	p := matchBytes(t, raw.PaletteRegisters, 20)
	stats := matchBytes(t, raw.Stats, 160)
	word := func(b []byte, i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	header := func(b []byte) MatchObjectHeader {
		return MatchObjectHeader{Behavior: binary.BigEndian.Uint32(b), Size: word(b, 4), Stage: word(b, 6)}
	}
	s := MatchLifecycleState{Manager: header(w), PauseFlags: raw.PauseFlags, PauseMask: raw.PauseMask, Pressed: raw.Pressed, SoundQueue: raw.Queue,
		Disabled: g[0x32], PendingSound: g[0x30], ResultTicks: word(g, 0x3a), OpeningTicks: word(g, 0x3c), Rule: word(g, 0x62), RoundIndex: int16(word(g, 0x6a)),
		WinnerCode: int16(word(g, 0x74)), TeamResult: g[0x7a], EligibleChannels: g[0x7c], Grouping: word(g, 0x88), ModeFlag: g[0x96], OptionA097: g[0x97],
		OptionFF14: raw.OptionFF14, OptionFF16: raw.OptionFF16, PaletteFlags: p[0], PaletteStatus: p[1], PaletteMode: word(p, 2), PaletteStep: word(p, 4), PaletteTarget: word(p, 6), VideoMode: word(p, 18)}
	copy(s.Characters[:], g[0x100:0x104])
	copy(s.Controllers[:], g[0x104:0x108])
	copy(s.HeadToHead[:], stats[:16])
	copy(s.WinsByCharacter[:], stats[32:96])
	copy(s.LossesByCharacter[:], stats[96:160])
	for i := range s.Fighters {
		b := w[0x3c0+i*0x400:]
		s.Fighters[i] = MatchFighter{Behavior: binary.BigEndian.Uint32(b), Health: int16(word(b, 0x78)), BaseTeam: b[0x6f]}
	}
	for i := range s.Presentation {
		s.Presentation[i] = header(w[0x2c0+i*64:])
	}
	current := matchBytes(t, raw.PaletteCurrent, 128)
	secondary := matchBytes(t, raw.PaletteSecondary, 128)
	for i := range s.CurrentColors {
		s.CurrentColors[i], s.SecondaryColors[i] = word(current, i*2), word(secondary, i*2)
	}
	return s
}

func encodeMatchLifecycle(t *testing.T, s MatchLifecycleState, result MatchLifecycleOutcome, previous matchOriginalSnapshot) matchOriginalSnapshot {
	t.Helper()
	raw := previous
	w := matchBytes(t, previous.World, 0x1fc0)
	if result.ClearWorldFrom != 0 {
		if result.ClearWorldFrom != 0x8040 {
			t.Fatal("unexpected clear-world start")
		}
		clear(w) // Actual semantic clear request, never an expected-output mask.
	}
	putWord := func(b []byte, i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	header := func(b []byte, h MatchObjectHeader) {
		binary.BigEndian.PutUint32(b, h.Behavior)
		putWord(b, 4, h.Size)
		putWord(b, 6, h.Stage)
	}
	header(w, s.Manager)
	for i, f := range s.Fighters {
		b := w[0x3c0+i*0x400:]
		binary.BigEndian.PutUint32(b, f.Behavior)
		putWord(b, 0x78, uint16(f.Health))
		b[0x6f] = f.BaseTeam
	}
	for i, h := range s.Presentation {
		header(w[0x2c0+i*64:], h)
	}
	raw.World = hex.EncodeToString(w)
	g := matchBytes(t, previous.Globals, 512)
	g[0x32], g[0x30], g[0x7a], g[0x7c], g[0x96], g[0x97] = s.Disabled, s.PendingSound, s.TeamResult, s.EligibleChannels, s.ModeFlag, s.OptionA097
	putWord(g, 0x3a, s.ResultTicks)
	putWord(g, 0x3c, s.OpeningTicks)
	putWord(g, 0x62, s.Rule)
	putWord(g, 0x6a, uint16(s.RoundIndex))
	putWord(g, 0x74, uint16(s.WinnerCode))
	putWord(g, 0x88, s.Grouping)
	copy(g[0x100:0x104], s.Characters[:])
	copy(g[0x104:0x108], s.Controllers[:])
	raw.Globals = hex.EncodeToString(g)
	p := matchBytes(t, previous.PaletteRegisters, 20)
	p[0], p[1] = s.PaletteFlags, s.PaletteStatus
	putWord(p, 2, s.PaletteMode)
	putWord(p, 4, s.PaletteStep)
	putWord(p, 6, s.PaletteTarget)
	putWord(p, 18, s.VideoMode)
	raw.PaletteRegisters = hex.EncodeToString(p)
	current, secondary := make([]byte, 128), make([]byte, 128)
	for i := range s.CurrentColors {
		putWord(current, i*2, s.CurrentColors[i])
		putWord(secondary, i*2, s.SecondaryColors[i])
	}
	raw.PaletteCurrent, raw.PaletteSecondary = hex.EncodeToString(current), hex.EncodeToString(secondary)
	stats := matchBytes(t, previous.Stats, 160)
	copy(stats[:16], s.HeadToHead[:])
	copy(stats[32:96], s.WinsByCharacter[:])
	copy(stats[96:160], s.LossesByCharacter[:])
	raw.Stats = hex.EncodeToString(stats)
	raw.PauseFlags, raw.PauseMask, raw.Pressed, raw.Queue, raw.OptionFF14, raw.OptionFF16 = s.PauseFlags, s.PauseMask, s.Pressed, s.SoundQueue, s.OptionFF14, s.OptionFF16
	return raw
}

type matchOriginalCall struct {
	Name          string
	Frame, Stage  int
	Before, After matchOriginalSnapshot
	Events        []string
	ReferenceOnly string `json:"reference_only_reason"`
	External      struct {
		Health       map[int]int16
		PaletteFlags *uint8 `json:"palette_flags"`
	} `json:"external_input"`
}

// Stream big original snapshots instead of duplicating hundreds of MB in the
// test heap. Both the complete compressed and decompressed sources are bound.
func visitMatchFixture(t *testing.T, name string, visit func(*json.Decoder)) int {
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
		Baseline bool   `json:"frames_match_original_core"`
		External int    `json:"external_queue_calls"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	zh := sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || m.External != 0 || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified match fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	reader := io.TeeReader(z, h)
	d := json.NewDecoder(reader)
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		t.Fatal("invalid match array", err)
	}
	count := 0
	for d.More() {
		visit(d)
		count++
	}
	if token, err = d.Token(); err != nil || token != json.Delim(']') {
		t.Fatal("incomplete match array", err)
	}
	if _, err = io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if count != m.Records || hex.EncodeToString(h.Sum(nil)) != m.SHA {
		t.Fatal("unbound or missing complete match records")
	}
	return count
}

func matchEvents(outcome MatchLifecycleOutcome) []string {
	events := make([]string, 0, 3)
	if outcome.Intro {
		events = append(events, "intro")
	}
	if outcome.Results {
		events = append(events, "results")
	}
	if outcome.ClearWorldFrom != 0 {
		events = append(events, "clear-world")
	}
	return events
}

// Original complete-stage fixtures and tests precede match_lifecycle.go.
func TestMatchLifecycleOriginalStages(t *testing.T) {
	for _, name := range []string{"match-lifecycle-oracle", "match-lifecycle-boundaries"} {
		t.Run(name, func(t *testing.T) {
			index := 0
			rejected := 0
			count := visitMatchFixture(t, name, func(d *json.Decoder) {
				var row matchOriginalCall
				if err := d.Decode(&row); err != nil {
					t.Fatal(err)
				}
				input := decodeMatchLifecycle(t, row.Before)
				got, outcome, err := StepMatchLifecycle(input)
				if row.ReferenceOnly != "" {
					invalidTeam := false
					for _, fighter := range input.Fighters {
						invalidTeam = invalidTeam || fighter.Behavior == 0x12668 && fighter.Health != -1024 && fighter.BaseTeam > 4
					}
					if input.Manager.Stage != 20 || !invalidTeam || err == nil || got != input || outcome != (MatchLifecycleOutcome{}) {
						t.Fatal("out-of-game reference input was not rejected atomically")
					}
					rejected++
					index++
					return
				}
				want := decodeMatchLifecycle(t, row.After)
				if err != nil || got != want || !reflect.DeepEqual(encodeMatchLifecycle(t, got, outcome, row.Before), row.After) || !reflect.DeepEqual(matchEvents(outcome), row.Events) {
					t.Fatalf("manager %d stage=%d frame=%d %s differs: err=%v got stage=%d winner=%d ticks=%d outcome=%+v want stage=%d winner=%d ticks=%d events=%v", index, row.Stage, row.Frame, row.Name, err, got.Manager.Stage, got.WinnerCode, got.ResultTicks, outcome, want.Manager.Stage, want.WinnerCode, want.ResultTicks, row.Events)
				}
				index++
			})
			t.Logf("complete original manager records=%d semantic matches=%d reference-only rejected=%d", count, count-rejected, rejected)
		})
	}
}

func TestMatchLifecycleSingleInitialState(t *testing.T) {
	steps := 0
	clears := 0
	count := visitMatchFixture(t, "match-lifecycle-sequences", func(d *json.Decoder) {
		var stream struct {
			Initial matchOriginalSnapshot
			Calls   []matchOriginalCall
		}
		if err := d.Decode(&stream); err != nil {
			t.Fatal(err)
		}
		state := decodeMatchLifecycle(t, stream.Initial)
		encoded := stream.Initial
		for index, call := range stream.Calls {
			for slot, health := range call.External.Health {
				state.Fighters[slot].Health = health
			}
			if call.External.PaletteFlags != nil {
				state.PaletteStatus = *call.External.PaletteFlags
			}
			var outcome MatchLifecycleOutcome
			var err error
			state, outcome, err = StepMatchLifecycle(state)
			encoded = encodeMatchLifecycle(t, state, outcome, encoded)
			if err != nil || state != decodeMatchLifecycle(t, call.After) || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(matchEvents(outcome), call.Events) {
				t.Fatalf("lifecycle stream step %d differs: %v", index, err)
			}
			if outcome.ClearWorldFrom != 0 {
				clears++
			}
			steps++
		}
	})
	if count != 4 || steps != 3112 || clears != 4 {
		t.Fatal("missing complete lifecycle streams", count, steps, clears)
	}
}

func TestMatchLifecycleBoundariesAreAtomic(t *testing.T) {
	for _, state := range []MatchLifecycleState{{Manager: MatchObjectHeader{Stage: 0}}, {Manager: MatchObjectHeader{Stage: 2}}, {Manager: MatchObjectHeader{Stage: 255}},
		{Manager: MatchObjectHeader{Stage: 22}, ResultTicks: 319, WinnerCode: 1, Grouping: 2}} {
		got, outcome, err := StepMatchLifecycle(state)
		if err == nil || got != state || outcome != (MatchLifecycleOutcome{}) {
			t.Fatal("unsupported initialization or invalid taunt data leaked partial stage")
		}
	}
	for _, state := range []MatchLifecycleState{{Manager: MatchObjectHeader{Stage: 0}, PauseFlags: 128, PendingSound: 151}, {Manager: MatchObjectHeader{Stage: 255}, ModeFlag: 2, ResultTicks: 319}} {
		got, outcome, err := StepMatchLifecycle(state)
		if err != nil || got != state || outcome != (MatchLifecycleOutcome{}) {
			t.Fatal("entry bypass did not precede stage validation")
		}
	}
}
