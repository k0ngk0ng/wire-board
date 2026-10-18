// Package campaign contains original story-mode preparation rules. It is a
// leaf package: it does not run matches, choose random values, or control UI.
package campaign

import (
	"fmt"
	"math/bits"
)

type Item int8

const Empty Item = -1

type Slot uint8

const (
	Shirt Slot = iota
	Pants
	Shoes
)

// Progress is the canonical preparation state. Equipment belongs to the four
// Nekketsu roster characters, not to platform seats. JSON saves retain the
// complete ordered inventory and equipped items; original passwords do not.
type Progress struct {
	Mode      uint8      `json:"mode"`
	Period    uint8      `json:"period"`
	WinBits   [2]uint8   `json:"winBits"`
	RewardSet uint8      `json:"rewardSet"`
	Inventory [16]Item   `json:"inventory"`
	Equipment [4][3]Item `json:"equipment"`
}

// New accepts the reward set already selected by the caller's original RNG.
// It never chooses a reward set, consumes randomness, or awards an item.
func New(mode, rewardSet uint8) (Progress, error) {
	p := Progress{Mode: mode, RewardSet: rewardSet}
	for i := range p.Inventory {
		p.Inventory[i] = Empty
	}
	for i := range p.Equipment {
		for j := range p.Equipment[i] {
			p.Equipment[i][j] = Empty
		}
	}
	if err := p.Validate(); err != nil {
		return Progress{}, err
	}
	return p, nil
}

// Validate checks the source preparation domain and structural consistency.
// It does not attest that arbitrary externally constructed arrays were earned
// through matches. The caller owns persisted-state provenance and authorization.
func (p Progress) Validate() error {
	if p.Mode > 1 {
		return fmt.Errorf("unsupported campaign mode %d", p.Mode)
	}
	if p.RewardSet >= 4 {
		return fmt.Errorf("unsupported original reward selector %d", p.RewardSet)
	}
	if p.WinBits[0]&128 != 0 || p.WinBits[1]&128 != 0 {
		return fmt.Errorf("campaign win mask outside seven opponents")
	}
	first, second := bits.OnesCount8(p.WinBits[0]), bits.OnesCount8(p.WinBits[1])
	if first-second < 0 || first-second > 1 {
		return fmt.Errorf("campaign home/away win counts differ from original progression")
	}
	if p.Period != uint8(2*(first+second)) {
		return fmt.Errorf("campaign period %d does not match win masks", p.Period)
	}
	ended := false
	for _, item := range p.Inventory {
		if item == Empty {
			ended = true
			continue
		}
		if _, err := SlotForItem(item); err != nil {
			return err
		}
		if ended {
			return fmt.Errorf("campaign inventory must be packed before empty slots")
		}
	}
	for character, slots := range p.Equipment {
		for slot, item := range slots {
			if item == Empty {
				continue
			}
			kind, err := SlotForItem(item)
			if err != nil {
				return err
			}
			if int(kind) != slot {
				return fmt.Errorf("item %d is not equipment slot %d for character %d", item, slot, character)
			}
		}
	}
	return nil
}

func SlotForItem(item Item) (Slot, error) {
	if item < 0 || item >= 32 {
		return 0, fmt.Errorf("unknown original equipment %d", item)
	}
	if item < 10 {
		return Shirt, nil
	}
	if item < 21 {
		return Pants, nil
	}
	return Shoes, nil
}

// AvailableOpponents preserves original map order and the mask selected by
// the next match's half-period counter. A completed campaign has no opponents.
func AvailableOpponents(p Progress) ([]uint8, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	remaining := make([]uint8, 0, 7)
	mask := p.WinBits[(p.Period>>1)&1]
	for _, opponent := range [7]uint8{1, 4, 7, 3, 5, 2, 6} {
		if mask&(1<<(opponent-1)) == 0 {
			remaining = append(remaining, opponent)
		}
	}
	return remaining, nil
}

// CourtForOpponent derives the selected away court or original home court0.
// It never replaces the selected opponent's team with the court's team ID.
func CourtForOpponent(p Progress, opponent uint8) (uint8, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	if opponent < 1 || opponent > 7 {
		return 0, fmt.Errorf("campaign opponent %d is not selectable", opponent)
	}
	if p.WinBits[(p.Period>>1)&1]&(1<<(opponent-1)) != 0 {
		return 0, fmt.Errorf("campaign opponent %d already defeated in this round", opponent)
	}
	if p.Period&2 != 0 {
		return 0, nil
	}
	return opponent, nil
}
