package score

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

type Stereo struct {
	Left  bool `json:"left"`
	Right bool `json:"right"`
}
type Pan struct {
	Left  bool  `json:"left"`
	Right bool  `json:"right"`
	AMS   uint8 `json:"ams"`
	FMS   uint8 `json:"fms"`
}

func (p Pan) bits() uint8 {
	var b uint8
	if p.Left {
		b |= 128
	}
	if p.Right {
		b |= 64
	}
	return b | p.AMS<<4 | p.FMS
}

type Owner struct {
	ScoreID uint8 `json:"score_id"`
	VoiceID int   `json:"voice_id"`
}
type PitchControl struct {
	EnvelopeID        uint16 `json:"envelope_id"`
	ModulationEnabled bool   `json:"modulation_enabled"`
}
type Modulation struct {
	Delay uint8 `json:"delay"`
	Rate  uint8 `json:"rate"`
	Delta int8  `json:"delta"`
	Steps uint8 `json:"steps"`
}

// VoiceState owns parameters and advancing counters, not source flow or RAM.
// NoteWait is an 8-bit cyclic countdown; zero encodes the recovered 256 interval.
type VoiceState struct {
	Owned                              bool
	Owner                              Owner
	Channel                            string
	Active, NoteOff, Claimed, Tie      bool
	Transpose                          int8
	BaseLevel                          uint8
	BasePitch                          uint16
	DACDescriptor                      string
	PitchScale                         int8
	PitchControl                       PitchControl
	Tone                               string
	VolumeEnvelope                     uint16
	VolumeCursor, PitchCursor          param.EnvelopeCursor
	NoteWait, GateRemaining, GateTicks uint8
	HasModulation                      bool
	Modulation                         Modulation
	ModDelay, ModRate, ModSteps        uint8
	ModDelta                           int8
	ModOffset                          int16
	Detune                             int8
	Pan                                Pan
	PanDefinition                      param.PanDefinition
	PanState                           param.PanState
	LoadedAlgorithm, NoiseControl      uint8
	// One semantic event cursor replaces source PC, loop counters and call stack.
	Cycle      bool
	EventIndex int
}
type State struct {
	TempoRemaining, TempoPeriod uint8
	Voices                      [18]VoiceState
	Ticks                       uint64
}
type DACFeedback struct {
	Blocked  bool
	Priority uint8
	// PanRedirect is the independent nonzero mailbox test used by channel-six
	// pan writes; a pending low bit is sufficient even without Blocked's bit7.
	PanRedirect bool
}

// SoundID is an accepted root-dispatch command, not a queue insertion. Ordinary
// PCM and group2 commands belong to their respective owner and are rejected.
type Input struct {
	SoundID uint8
	DAC     DACFeedback
}
type Intent struct {
	Order        int     `json:"order"`
	Kind         string  `json:"kind"`
	Bank         uint8   `json:"bank,omitempty"`
	Register     uint8   `json:"register,omitempty"`
	Value        uint8   `json:"value,omitempty"`
	DescriptorID string  `json:"descriptor_id,omitempty"`
	Pan          *Stereo `json:"pan,omitempty"`
	Action       string  `json:"action,omitempty"`
	SoundID      uint8   `json:"sound_id,omitempty"`
}
type Output struct {
	Intents              []Intent
	ClearPrimaryPriority bool
}
type Driver struct {
	catalog *Catalog
	state   State
	lease   *TickTransaction
}

func New(c *Catalog) (*Driver, error) {
	if c == nil || len(c.scores) != 23 {
		return nil, fmt.Errorf("invalid catalog")
	}
	return &Driver{catalog: c}, nil
}
func (d *Driver) Snapshot() State { return d.state }
func Restore(c *Catalog, s State) (*Driver, error) {
	d, err := New(c)
	if err != nil {
		return nil, err
	}
	if s.TempoPeriod == 0 && s.TempoRemaining != 0 || s.TempoPeriod != 0 && (s.TempoRemaining == 0 || s.TempoRemaining > s.TempoPeriod) {
		return nil, fmt.Errorf("invalid tempo phase")
	}
	var musicID uint8
	for i, v := range s.Voices {
		if v.Pan.AMS > 3 || v.Pan.FMS > 7 || v.LoadedAlgorithm > 7 || v.PitchControl.EnvelopeID > 8 || v.VolumeEnvelope > 11 || v.EventIndex < 0 {
			return nil, fmt.Errorf("invalid voice parameters")
		}
		if v.Active && !v.Owned {
			return nil, fmt.Errorf("active voice lacks an owner")
		}
		if v.Owned {
			def, ok := c.scores[v.Owner.ScoreID]
			if !ok || v.Owner.VoiceID < 0 || v.Owner.VoiceID >= len(def.Voices) {
				return nil, fmt.Errorf("unknown voice owner")
			}
			voice := def.Voices[v.Owner.VoiceID]
			if v.Channel != voice.Channel && !(isPSG(i) && v.Channel == "psg-noise") {
				return nil, fmt.Errorf("voice channel differs from owner")
			}
			if i < 10 {
				if musicID != 0 && musicID != v.Owner.ScoreID {
					return nil, fmt.Errorf("conflicting music instances")
				}
				musicID = v.Owner.ScoreID
			}
			slot, err := slotFor(voice.Channel, def.Kind == "music")
			if err != nil || slot != i {
				return nil, fmt.Errorf("owner slot differs")
			}
			events := voice.Timeline.Prefix
			if v.Cycle {
				if voice.Timeline.Ended {
					return nil, fmt.Errorf("finite track cannot enter cycle")
				}
				events = voice.Timeline.Cycle
			}
			if v.EventIndex > len(events) {
				return nil, fmt.Errorf("invalid event cursor")
			}
			if isFM(i) {
				if _, ok := c.patches[v.Tone]; !ok {
					return nil, fmt.Errorf("unknown selected tone")
				}
			}
		}
		if v.DACDescriptor != "" {
			if i != 0 {
				return nil, fmt.Errorf("DAC pitch in tonal voice")
			}
			if _, ok := c.descriptors[v.DACDescriptor]; !ok {
				return nil, fmt.Errorf("unknown DAC descriptor")
			}
		}
		if v.Channel == "psg-noise" && v.NoiseControl&0xf8 != 0xe0 {
			return nil, fmt.Errorf("invalid noise control")
		}
		if v.PanState.Step > 4 {
			return nil, fmt.Errorf("invalid pan phase")
		}
		if v.PitchControl.ModulationEnabled && !v.HasModulation {
			return nil, fmt.Errorf("modulation lacks definition")
		}
		if v.HasModulation && (v.ModDelay > v.Modulation.Delay || v.ModSteps > v.Modulation.Steps || v.Modulation.Rate != 0 && v.ModRate > v.Modulation.Rate || v.ModDelta != v.Modulation.Delta && v.ModDelta != -v.Modulation.Delta) {
			return nil, fmt.Errorf("invalid modulation phase")
		}
		if id := v.VolumeEnvelope; id != 0 {
			envelope, ok := c.volume[id]
			if !ok || !validEnvelopeCursor(v.VolumeCursor, len(envelope.Prefix), len(envelope.Cycle), false) {
				return nil, fmt.Errorf("invalid volume phase")
			}
		}
		if id := v.PitchControl.EnvelopeID; id != 0 {
			envelope, ok := c.pitch[id]
			if !ok || !validEnvelopeCursor(v.PitchCursor, len(envelope.Prefix), len(envelope.Cycle), envelope.Ended) {
				return nil, fmt.Errorf("invalid pitch phase")
			}
		}
		if err := v.PanDefinition.Validate(c.patterns); err != nil && v.PanDefinition.Mode != "" {
			return nil, err
		}
	}
	d.state = s
	return d, nil
}

func validEnvelopeCursor(c param.EnvelopeCursor, prefix, cycle int, ended bool) bool {
	if c.Cycle {
		return int(c.Index) < cycle
	}
	return int(c.Index) < prefix || int(c.Index) == prefix && (cycle > 0 || ended)
}

var slots = [18]string{"music.dac", "music.fm1", "music.fm2", "music.fm3", "music.fm4", "music.fm5", "music.fm6", "music.psg1", "music.psg2", "music.psg3", "primary.fm3", "primary.fm4", "primary.fm5", "primary.psg1", "primary.psg2", "primary.psg3", "secondary.fm4", "secondary.psg3"}

func domain(i int) string {
	if i < 10 {
		return "music"
	}
	if i < 16 {
		return "effect-primary"
	}
	return "effect-secondary"
}
func envelopeCursor(c param.EnvelopeCursor, id uint16) map[string]any {
	phase := "prefix"
	if id == 0 {
		phase = "disabled"
	} else if c.Cycle {
		phase = "cycle"
	}
	return map[string]any{"phase": phase, "index": int(c.Index)}
}

// View is a derived, canonical observation of score-owned parameters. It does
// not create an additional mutable state representation.
func (d *Driver) View() map[string]any {
	voices := make([]any, 18)
	for i, v := range d.state.Voices {
		var owner any
		if v.Owned {
			owner = v.Owner
		}
		var hardware any
		if v.Owned {
			hardware = v.Channel
		}
		if i == 0 {
			hardware = "dac"
		}
		var tone any
		if isPSG(i) {
			tone = v.VolumeEnvelope
		} else if v.Owned && i != 0 {
			tone = v.Tone
		}
		var base any = v.BasePitch
		if i == 0 {
			base = nil
			if v.DACDescriptor != "" {
				base = v.DACDescriptor
			}
		}
		var mod any
		if v.HasModulation {
			mod = v.Modulation
		}
		var algorithm, noise any
		if isPSG(i) {
			noise = v.NoiseControl
		} else {
			algorithm = v.LoadedAlgorithm
		}
		mode := 0
		pattern := 0
		step, limit, elapsed, period := uint8(0), uint8(0), uint8(0), uint8(0)
		switch v.PanDefinition.Mode {
		case param.PanAdvanceOnNote:
			mode = 1
		case param.PanRestartOnNoteOnce:
			mode = 2
		case param.PanRestartOnNoteLoop:
			mode = 3
		case param.PanRestartEveryDriverTick:
			mode = 4
		}
		if mode != 0 && !isPSG(i) {
			for j, p := range d.catalog.patterns {
				if p.ID == v.PanDefinition.Pattern {
					pattern = j + 1
				}
			}
			step = v.PanState.Step
			elapsed = v.PanState.Elapsed
			limit = v.PanDefinition.StepLimit
			period = v.PanDefinition.Period
		}
		voices[i] = map[string]any{"slot": slots[i], "domain": domain(i), "owner": owner, "active": v.Active, "note_off": v.NoteOff, "claimed": v.Claimed, "tie": v.Tie, "hardware_voice": hardware, "pitch_envelope_scale": v.PitchScale, "transpose": v.Transpose, "base_level": v.BaseLevel, "pitch_control": v.PitchControl, "tone": tone, "volume_envelope_cursor": envelopeCursor(v.VolumeCursor, v.VolumeEnvelope), "note_wait": v.NoteWait, "base_pitch": base, "gate_remaining": v.GateRemaining, "gate_ticks": v.GateTicks, "modulation_definition": mod, "modulation_delay_remaining": v.ModDelay, "modulation_rate_remaining": v.ModRate, "modulation_delta": v.ModDelta, "modulation_steps_remaining": v.ModSteps, "modulation_offset": v.ModOffset, "detune": v.Detune, "pan_automation": map[string]any{"mode": mode, "pattern_id": pattern, "step": step, "step_limit": limit, "elapsed": elapsed, "period": period}, "loaded_algorithm": algorithm, "noise_control": noise, "pitch_envelope_cursor": envelopeCursor(v.PitchCursor, v.PitchControl.EnvelopeID), "pan": v.Pan}
	}
	return map[string]any{"tempo_remaining": d.state.TempoRemaining, "tempo_period": d.state.TempoPeriod, "control_signal": 0, "fade_steps": 0, "fade_period": 0, "fade_remaining": 0, "pause_state": 0, "fm3_mode": 0, "music_fm3_offsets": [4]uint16{}, "effect_fm3_offsets": [4]uint16{}, "voices": voices}
}
