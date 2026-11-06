package mixed

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
)

// ReadPauseLevels reads the actual TL owner only after acquisition. It neither
// advances synthesis nor keeps a register mirror; TLs are stable while held.
func (r *Runtime) ReadPauseLevels(at uint64) ([6][4]uint8, error) {
	var levels [6][4]uint8
	cycle, err := r.relative(at)
	if err != nil {
		return levels, err
	}
	s := r.chip.Snapshot()
	end := s.Time + chips.FrameCycles
	if !r.pcm.Status().Held || cycle < s.Cursor || (cycle > end && cycle-end > chips.MaxInputAhead) {
		return levels, errors.New("pause level read lacks held current owner")
	}
	for ch, v := range s.YM.Channels {
		for op, value := range v.Operators {
			levels[ch][op] = value.Level
		}
	}
	return levels, nil
}
