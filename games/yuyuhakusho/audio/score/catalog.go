// Package score advances typed musical events and produces ordered chip intents.
// It does not execute source CPUs, own the root queue, or synthesize audio.
package score

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

type pitch struct {
	Kind         string `json:"kind"`
	Semitone     int    `json:"semitone,omitempty"`
	DescriptorID string `json:"descriptor_id,omitempty"`
}
type event struct {
	At                   int                  `json:"at"`
	Kind                 string               `json:"kind"`
	Pitch                *pitch               `json:"pitch,omitempty"`
	Duration             int                  `json:"duration,omitempty"`
	Left                 bool                 `json:"left,omitempty"`
	Right                bool                 `json:"right,omitempty"`
	Value                int8                 `json:"value,omitempty"`
	Delta                int8                 `json:"delta,omitempty"`
	Definition           *param.PanDefinition `json:"definition,omitempty"`
	ImmediateFM          bool                 `json:"immediate_fm,omitempty"`
	Ticks                uint8                `json:"ticks,omitempty"`
	Period               uint8                `json:"period,omitempty"`
	SoundID              uint8                `json:"sound_id,omitempty"`
	PatchID              string               `json:"patch_id,omitempty"`
	Delay                uint8                `json:"delay,omitempty"`
	Rate                 uint8                `json:"rate,omitempty"`
	Steps                uint8                `json:"steps,omitempty"`
	White                bool                 `json:"white,omitempty"`
	Clock                uint8                `json:"clock,omitempty"`
	EnvelopeID           uint16               `json:"envelope_id,omitempty"`
	ModulationEnabled    bool                 `json:"modulation_enabled,omitempty"`
	Semitones            int8                 `json:"semitones,omitempty"`
	Enabled              bool                 `json:"enabled,omitempty"`
	ReleasePolicy        string               `json:"release_policy,omitempty"`
	ClearPrimaryPriority bool                 `json:"clear_primary_priority,omitempty"`
	EndsDriverTick       bool                 `json:"ends_driver_tick,omitempty"`
}
type timeline struct {
	PrefixQuanta int     `json:"prefix_quanta"`
	CycleQuanta  int     `json:"cycle_quanta"`
	Prefix       []event `json:"prefix"`
	Cycle        []event `json:"cycle"`
	Ended        bool    `json:"ended"`
}
type initial struct {
	Transpose      int8   `json:"transpose"`
	Level          uint8  `json:"level"`
	PitchControl   uint8  `json:"pitch_control"`
	VolumeEnvelope uint16 `json:"volume_envelope"`
	PatchReference string `json:"patch_reference,omitempty"`
}
type voice struct {
	ID       int      `json:"id"`
	Channel  string   `json:"channel"`
	Initial  initial  `json:"initial"`
	Timeline timeline `json:"timeline"`
}
type definition struct {
	ID           uint8   `json:"id"`
	Kind         string  `json:"kind"`
	InitialTempo uint8   `json:"initial_tempo"`
	StartPhase   string  `json:"start_phase"`
	Voices       []voice `json:"voices"`
}

// DACDescriptor identifies a sample dependency. Playback and compressed sample
// bytes have their own owner; the score retains only this immutable descriptor.
type DACDescriptor struct {
	ID         string `json:"id"`
	SampleID   string `json:"sample_id"`
	NativeRate uint8  `json:"native_rate"`
	Flags      uint8  `json:"flags"`
	Routing    string `json:"routing"`
}
type rawCatalog struct {
	Schema          int                    `json:"schema"`
	Format          string                 `json:"format"`
	DSPImplemented  bool                   `json:"dsp_implemented"`
	FallbackPatchID string                 `json:"fallback_patch_id"`
	Scores          []definition           `json:"scores"`
	Patches         []param.FMPatch        `json:"fm_patches"`
	PitchEnvelopes  []param.PitchEnvelope  `json:"pitch_envelopes"`
	VolumeEnvelopes []param.VolumeEnvelope `json:"psg_volume_envelopes"`
	Patterns        []param.PanPattern     `json:"pan_patterns"`
	PitchTables     struct {
		FM       []uint16 `json:"fm_fnums"`
		PSG      []uint16 `json:"psg_dividers"`
		Extended []struct {
			Index   int    `json:"index"`
			Divider uint16 `json:"divider"`
		} `json:"psg_extended_dividers"`
	} `json:"pitch_tables"`
	Descriptors []DACDescriptor `json:"dac_descriptors"`
	Payloads    []struct {
		ID    string `json:"id"`
		Codec string `json:"codec"`
		Data  string `json:"data_base64"`
	} `json:"dac_payloads"`
}

// Catalog is immutable after decoding. Its maps and definitions are private so
// callers cannot mutate another driver's definitions through a shared catalog.
type Catalog struct {
	jsonSHA         string
	scores          map[uint8]definition
	patches         map[string]param.FMPatch
	pitch           map[uint16]param.PitchEnvelope
	volume          map[uint16]param.VolumeEnvelope
	patterns        []param.PanPattern
	fm              [12]uint16
	psg             map[int]uint16
	descriptors     map[string]DACDescriptor
	fallbackPatchID string
}

func DecodeCatalog(r io.Reader) (*Catalog, error) {
	var raw rawCatalog
	source, err := io.ReadAll(io.LimitReader(r, (16<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(source) > 16<<20 {
		return nil, fmt.Errorf("catalog resource limit exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(source))
	d.DisallowUnknownFields()
	if err := d.Decode(&raw); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing catalog data")
	}
	if raw.Schema != 2 || raw.Format != "typed-audio-score-v2" || raw.DSPImplemented || len(raw.Scores) != 23 || len(raw.Patches) > 512 || len(raw.PitchTables.FM) != 12 || len(raw.PitchTables.PSG) != 70 {
		return nil, fmt.Errorf("unsupported catalog profile")
	}
	c := &Catalog{scores: map[uint8]definition{}, patches: map[string]param.FMPatch{}, pitch: map[uint16]param.PitchEnvelope{}, volume: map[uint16]param.VolumeEnvelope{}, psg: map[int]uint16{}, descriptors: map[string]DACDescriptor{}, patterns: raw.Patterns}
	identity := sha256.Sum256(source)
	c.jsonSHA = hex.EncodeToString(identity[:])
	copy(c.fm[:], raw.PitchTables.FM)
	for i, v := range raw.PitchTables.PSG {
		c.psg[i] = v
	}
	for _, entry := range raw.PitchTables.Extended {
		if entry.Index < 70 || entry.Index > 127 {
			return nil, fmt.Errorf("invalid extended PSG index")
		}
		if _, ok := c.psg[entry.Index]; ok {
			return nil, fmt.Errorf("duplicate PSG index")
		}
		c.psg[entry.Index] = entry.Divider
	}
	for _, p := range raw.Patches {
		if p.ID == "" {
			return nil, fmt.Errorf("empty patch ID")
		}
		if _, ok := c.patches[p.ID]; ok {
			return nil, fmt.Errorf("duplicate patch")
		}
		if err := p.Validate(); err != nil {
			return nil, err
		}
		c.patches[p.ID] = p
	}
	if _, ok := c.patches[raw.FallbackPatchID]; !ok {
		return nil, fmt.Errorf("missing uninitialized restore dependency")
	}
	c.fallbackPatchID = raw.FallbackPatchID
	for _, e := range raw.PitchEnvelopes {
		if _, ok := c.pitch[e.ID]; ok {
			return nil, fmt.Errorf("duplicate pitch envelope")
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		c.pitch[e.ID] = e
	}
	for _, e := range raw.VolumeEnvelopes {
		if _, ok := c.volume[e.ID]; ok {
			return nil, fmt.Errorf("duplicate volume envelope")
		}
		if err := e.Validate(); err != nil {
			return nil, err
		}
		c.volume[e.ID] = e
	}
	if len(c.pitch) != 8 || len(c.volume) != 11 || len(c.patterns) != 3 {
		return nil, fmt.Errorf("incomplete parameter profile")
	}
	for id := uint16(1); id <= 8; id++ {
		if _, ok := c.pitch[id]; !ok {
			return nil, fmt.Errorf("missing required pitch definition")
		}
	}
	for id := uint16(1); id <= 11; id++ {
		if _, ok := c.volume[id]; !ok {
			return nil, fmt.Errorf("missing required volume definition")
		}
	}
	dependencies := map[string]bool{}
	for _, p := range raw.Payloads {
		if dependencies[p.ID] || p.Codec != "dpcm4" {
			return nil, fmt.Errorf("invalid audio dependency")
		}
		data, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil || len(data) > 32768 || len(data) == 0 {
			return nil, fmt.Errorf("invalid DPCM dependency")
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != p.ID {
			return nil, fmt.Errorf("sample digest differs")
		}
		dependencies[p.ID] = true
	}
	for _, entry := range raw.Descriptors {
		if _, ok := c.descriptors[entry.ID]; ok || entry.ID == "" || !dependencies[entry.SampleID] || entry.Routing != "track-pan" {
			return nil, fmt.Errorf("invalid DAC descriptor")
		}
		c.descriptors[entry.ID] = entry
	}
	for _, s := range raw.Scores {
		if _, ok := c.scores[s.ID]; ok || !supported(s.ID) {
			return nil, fmt.Errorf("unsupported or duplicate score")
		}
		music := s.ID < 0xa0
		if music != (s.Kind == "music") || (!music && s.Kind != "effect-primary") || s.StartPhase != map[bool]string{true: "next-driver-tick", false: "current-driver-tick"}[music] || len(s.Voices) == 0 || len(s.Voices) > 9 {
			return nil, fmt.Errorf("invalid score header")
		}
		occupied := map[int]bool{}
		for ordinal, v := range s.Voices {
			slot, err := slotFor(v.Channel, music)
			if err != nil || occupied[slot] || v.ID != ordinal {
				return nil, fmt.Errorf("invalid declared voice")
			}
			occupied[slot] = true
			if v.Initial.PitchControl&127 > 8 || v.Initial.VolumeEnvelope > 11 {
				return nil, fmt.Errorf("initial parameter outside profile")
			}
			if isFM(slot) {
				if _, ok := c.patches[v.Initial.PatchReference]; !ok {
					return nil, fmt.Errorf("missing initial patch")
				}
			}
			for part, events := range [][]event{v.Timeline.Prefix, v.Timeline.Cycle} {
				at := 0
				for i, e := range events {
					if e.At != at {
						return nil, fmt.Errorf("noncanonical event timing")
					}
					if err := c.validateEvent(e, slot); err != nil {
						return nil, err
					}
					if e.Kind == "note" {
						at += e.Duration
					}
					if e.Kind == "track-end" && (part != 0 || !v.Timeline.Ended || i != len(events)-1) {
						return nil, fmt.Errorf("misplaced track end")
					}
				}
				want := v.Timeline.PrefixQuanta
				if part == 1 {
					want = v.Timeline.CycleQuanta
				}
				if at != want {
					return nil, fmt.Errorf("event durations disagree with timeline")
				}
			}
			if len(v.Timeline.Prefix)+len(v.Timeline.Cycle) > 100000 || v.Timeline.PrefixQuanta < 0 || v.Timeline.CycleQuanta < 0 || v.Timeline.Ended != (len(v.Timeline.Cycle) == 0) || (!v.Timeline.Ended && v.Timeline.CycleQuanta == 0) {
				return nil, fmt.Errorf("invalid timeline termination")
			}
			if v.Timeline.Ended && (len(v.Timeline.Prefix) == 0 || v.Timeline.Prefix[len(v.Timeline.Prefix)-1].Kind != "track-end") {
				return nil, fmt.Errorf("finite track lacks end")
			}
		}
		c.scores[s.ID] = s
	}
	return c, nil
}

func supported(id uint8) bool {
	switch id {
	case 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x8a, 0x8d, 0x95, 0x8f:
		return true
	}
	return id >= 0xa0 && id <= 0xab
}
func slotFor(channel string, music bool) (int, error) {
	m := map[string]int{"dac": 0, "fm1": 1, "fm2": 2, "fm3": 3, "fm4": 4, "fm5": 5, "fm6": 6, "psg1": 7, "psg2": 8, "psg3": 9, "psg-noise": 9}
	if !music {
		m = map[string]int{"fm3": 10, "fm4": 11, "fm5": 12, "psg1": 13, "psg2": 14, "psg3": 15, "psg-noise": 15}
	}
	if i, ok := m[channel]; ok {
		return i, nil
	}
	return 0, fmt.Errorf("unsupported channel %q", channel)
}
func isPSG(slot int) bool { return slot >= 7 && slot <= 9 || slot >= 13 && slot <= 15 || slot == 17 }
func isFM(slot int) bool  { return slot != 0 && !isPSG(slot) }
func (c *Catalog) validateEvent(e event, slot int) error {
	switch e.Kind {
	case "note":
		if e.Pitch == nil || e.Duration < 1 || e.Duration > 256 {
			return fmt.Errorf("invalid note")
		}
		switch e.Pitch.Kind {
		case "previous", "rest":
		case "chromatic":
			if slot == 0 || e.Pitch.Semitone < 0 || e.Pitch.Semitone > 94 {
				return fmt.Errorf("invalid chromatic note")
			}
		case "dac":
			if slot != 0 {
				return fmt.Errorf("DAC note in tonal voice")
			}
			if _, ok := c.descriptors[e.Pitch.DescriptorID]; !ok {
				return fmt.Errorf("unknown DAC dependency")
			}
		default:
			return fmt.Errorf("unknown pitch kind")
		}
	case "fm-patch", "fm-patch-select":
		if !isFM(slot) {
			return fmt.Errorf("FM patch in non-FM voice")
		}
		if _, ok := c.patches[e.PatchID]; !ok {
			return fmt.Errorf("unknown patch")
		}
	case "pan-automation":
		if e.Definition == nil {
			return fmt.Errorf("missing pan definition")
		}
		return e.Definition.Validate(c.patterns)
	case "volume-envelope":
		if !isPSG(slot) || e.EnvelopeID > 11 {
			return fmt.Errorf("invalid volume selection")
		}
	case "pitch-control":
		if e.EnvelopeID > 8 {
			return fmt.Errorf("invalid pitch selection")
		}
	case "psg-noise":
		if !isPSG(slot) || e.Clock > 3 {
			return fmt.Errorf("invalid PSG noise")
		}
	case "track-end":
		if e.EndsDriverTick != (slot == 0) {
			return fmt.Errorf("invalid stop scope")
		}
	case "pan", "detune", "level-add", "tie", "gate", "tempo", "enqueue-replace-first", "modulation", "modulation-enable", "transpose-add", "all-key-off":
	default:
		return fmt.Errorf("unknown score event %q", e.Kind)
	}
	return nil
}

func (c *Catalog) Descriptor(id string) (DACDescriptor, bool) {
	entry, ok := c.descriptors[id]
	return entry, ok
}
