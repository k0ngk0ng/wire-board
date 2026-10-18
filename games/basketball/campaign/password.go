package campaign

import (
	"encoding/hex"
	"fmt"
	"math/bits"
	"strings"
)

// DecodePassword restores BF52's legal generated-password domain and A8C1's
// inventory reconstruction. Unsupported reward-table aliases are rejected.
// Bad passwords yield an error, never a partially reset campaign to play.
func DecodePassword(code string) (Progress, error) {
	if len(code) != 6 {
		return Progress{}, fmt.Errorf("original campaign password needs six hexadecimal characters")
	}
	raw, err := hex.DecodeString(code)
	if err != nil {
		return Progress{}, fmt.Errorf("invalid original campaign password: %w", err)
	}
	mode := raw[0] & 1
	wins := [2]uint8{raw[0] >> 1, raw[1]}
	reward := raw[2] & 15
	checksum := (wins[0] + wins[1] + reward + mode) & 15
	if raw[2]>>4 != checksum {
		return Progress{}, fmt.Errorf("original campaign password checksum mismatch")
	}
	if wins[0] == 127 && wins[1] == 127 {
		return Progress{}, fmt.Errorf("completed campaign password is not restorable")
	}
	p, err := New(mode, reward)
	if err != nil {
		return Progress{}, err
	}
	p.WinBits = wins
	p.Period = uint8(2 * (bits.OnesCount8(wins[0]) + bits.OnesCount8(wins[1])))
	if err = p.Validate(); err != nil {
		return Progress{}, err
	}
	// A8C1 reconstructs home-mask rewards first (even table offsets), then
	// away-mask rewards (odd offsets), in opponent ID order within each mask.
	next := 0
	for _, side := range [2]int{1, 0} {
		offset := 1 - side
		for opponent := uint8(1); opponent <= 7; opponent++ {
			if wins[side]&(1<<(opponent-1)) == 0 {
				continue
			}
			p.Inventory[next] = rewardTables[reward][int(opponent-1)*2+offset]
			next++
		}
	}
	return p, nil
}

// EncodePassword writes the same progress code as BECD. Equipment and inventory
// changes deliberately do not alter it. Only restorable campaign codes are
// exported; the original decoder rejects an already completed campaign.
func EncodePassword(p Progress) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if p.Period == 28 {
		return "", fmt.Errorf("completed campaign password is not restorable")
	}
	checksum := (p.WinBits[0] + p.WinBits[1] + p.RewardSet + p.Mode) & 15
	raw := [3]uint8{p.WinBits[0]<<1 | p.Mode, p.WinBits[1], checksum<<4 | p.RewardSet}
	return strings.ToUpper(hex.EncodeToString(raw[:])), nil
}
