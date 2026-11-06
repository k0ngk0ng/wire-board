package rootcontrol

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
)

// PauseLevels is only the information that physical mute would destroy.
// The caller owns its optional lifetime from an actual pause capture through
// successful resume/reset. It is not a persistent second YM or voice image.
type PauseLevels struct {
	Levels [6][4]uint8
}

// LevelWrite is a transient ordered effect, never a stored register cache.
type LevelWrite struct{ Bank, Register, Value uint8 }

func CapturePauseLevels(current chips.YMState) (PauseLevels, error) {
	var saved PauseLevels
	for channel, value := range current.Channels {
		for operator, op := range value.Operators {
			if op.Level > 127 {
				return PauseLevels{}, errors.New("invalid visible FM pause level")
			}
			saved.Levels[channel][operator] = op.Level
		}
	}
	return saved, nil
}

func pauseWrites(saved *PauseLevels) [24]LevelWrite {
	var result [24]LevelWrite
	index := 0
	for channel := 2; channel >= 0; channel-- {
		for physical, logical := range [4]int{0, 2, 1, 3} {
			for bank := 0; bank < 2; bank++ {
				value := uint8(127)
				if saved != nil {
					value = saved.Levels[bank*3+channel][logical]
				}
				result[index] = LevelWrite{Bank: uint8(bank), Register: uint8(0x40 + physical*4 + channel), Value: value}
				index++
			}
		}
	}
	return result
}

func PauseMuteWrites() [24]LevelWrite { return pauseWrites(nil) }

func PauseRestoreWrites(saved PauseLevels) ([24]LevelWrite, error) {
	for _, channel := range saved.Levels {
		for _, level := range channel {
			if level > 127 {
				return [24]LevelWrite{}, errors.New("invalid pending FM pause level")
			}
		}
	}
	return pauseWrites(&saved), nil
}
