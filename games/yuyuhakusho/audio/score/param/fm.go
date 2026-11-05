// Package param implements typed sound-driver parameter arithmetic. It consumes
// no source addresses, CPU instructions, RAM mirrors or raw score controls.
package param

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// FMOperator uses ascending hardware register-slot order: 0,4,8,12.
type FMOperator struct {
	Detune          uint8 `json:"detune"`
	Multiple        uint8 `json:"multiple"`
	TotalLevel      uint8 `json:"total_level"`
	RateScale       uint8 `json:"rate_scale"`
	AttackRate      uint8 `json:"attack_rate"`
	AMEnabled       bool  `json:"am_enabled"`
	DecayRate       uint8 `json:"decay_rate"`
	SustainRate     uint8 `json:"sustain_rate"`
	SustainLevel    uint8 `json:"sustain_level"`
	ReleaseRate     uint8 `json:"release_rate"`
	Carrier         bool  `json:"carrier"`
	VolumeCarryBias bool  `json:"volume_carry_bias"`
}

type FMPatch struct {
	ID        string        `json:"id,omitempty"`
	Algorithm uint8         `json:"algorithm"`
	Feedback  uint8         `json:"feedback"`
	Operators [4]FMOperator `json:"operators"`
}

// UnmarshalJSON checks the exact operator count rather than letting the standard
// fixed-array decoder silently discard additional slots. Failure is atomic.
func (p *FMPatch) UnmarshalJSON(raw []byte) error {
	var decoded struct {
		ID        string       `json:"id"`
		Algorithm uint8        `json:"algorithm"`
		Feedback  uint8        `json:"feedback"`
		Operators []FMOperator `json:"operators"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&decoded); err != nil {
		return err
	}
	if len(decoded.Operators) != 4 {
		return fmt.Errorf("FM patch requires exactly four operators")
	}
	next := FMPatch{ID: decoded.ID, Algorithm: decoded.Algorithm, Feedback: decoded.Feedback}
	copy(next.Operators[:], decoded.Operators)
	if err := next.Validate(); err != nil {
		return err
	}
	*p = next
	return nil
}

var sourceSlotOrder = [4]uint8{0, 2, 1, 3}

func (p FMPatch) Validate() error {
	if p.Algorithm > 7 || p.Feedback > 7 {
		return fmt.Errorf("FM algorithm and feedback must be in 0..7")
	}
	mask := [8]uint8{8, 8, 8, 8, 10, 14, 14, 15}[p.Algorithm]
	for sourceSlot, slot := range sourceSlotOrder {
		o := p.Operators[slot]
		if o.Detune > 7 || o.Multiple > 15 || o.TotalLevel > 127 || o.RateScale > 3 || o.AttackRate > 31 || o.DecayRate > 31 || o.SustainRate > 31 || o.SustainLevel > 15 || o.ReleaseRate > 15 {
			return fmt.Errorf("FM operator %d field out of range", slot)
		}
		if o.Carrier != (mask&(1<<sourceSlot) != 0) {
			return fmt.Errorf("FM operator %d carrier differs from algorithm", slot)
		}
	}
	return nil
}

type FMLevelUpdate struct {
	Slot       uint8 `json:"slot"`
	TotalLevel uint8 `json:"total_level"`
}

// FMLevelUpdates returns writes in the original source order. The driver's TL
// high arithmetic bit has an independent semantic field even though YM ignores
// it. Ordinary signed attenuation and carry suppression differ from patch load.
func FMLevelUpdates(p FMPatch, base uint8, initial bool) ([]FMLevelUpdate, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if !initial && base >= 128 {
		return nil, nil
	}
	var out []FMLevelUpdate
	for _, slot := range sourceSlotOrder {
		o := p.Operators[slot]
		if initial {
			level := o.TotalLevel
			if o.Carrier {
				level += base
			}
			out = append(out, FMLevelUpdate{slot, level & 127})
		} else if o.Carrier {
			driverLevel := uint16(o.TotalLevel)
			if o.VolumeCarryBias {
				driverLevel += 128
			}
			if driverLevel+uint16(base) <= 255 {
				out = append(out, FMLevelUpdate{slot, (o.TotalLevel + base) & 127})
			}
		}
	}
	return out, nil
}

// FMRegisterWrite is channel-independent and chip-effective. The caller routes
// the register to a hardware channel/port and owns pan/AMS/FMS separately.
type FMRegisterWrite struct {
	Register uint8 `json:"register"`
	Value    uint8 `json:"value"`
}

// PatchWrites emits B0, five four-operator parameter groups, then four TL writes.
// Reserved hardware-ignored bits absent from typed parameters are canonical zero.
func PatchWrites(p FMPatch, base uint8) ([]FMRegisterWrite, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out := make([]FMRegisterWrite, 0, 25)
	out = append(out, FMRegisterWrite{0xb0, p.Feedback<<3 | p.Algorithm})
	for _, group := range [...]uint8{0x30, 0x50, 0x60, 0x70, 0x80} {
		for _, slot := range sourceSlotOrder {
			o := p.Operators[slot]
			var value uint8
			switch group {
			case 0x30:
				value = o.Detune<<4 | o.Multiple
			case 0x50:
				value = o.RateScale<<6 | o.AttackRate
			case 0x60:
				value = o.DecayRate
				if o.AMEnabled {
					value |= 128
				}
			case 0x70:
				value = o.SustainRate
			case 0x80:
				value = o.SustainLevel<<4 | o.ReleaseRate
			}
			out = append(out, FMRegisterWrite{group + slot*4, value})
		}
	}
	levels, _ := FMLevelUpdates(p, base, true)
	for _, level := range levels {
		out = append(out, FMRegisterWrite{0x40 + level.Slot*4, level.TotalLevel})
	}
	return out, nil
}
