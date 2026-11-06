package rootcontrol

import "errors"

// These accesses occur at the caller's supplied semantic stage. Root owns no
// second clock; the timing service accounts for the actual read/write cost.
func (s *State) ReadPCMChoice(_ uint64) (uint8, error) {
	if s == nil || s.ChoiceBits > 1 {
		return 0, errors.New("unsupported PCM choice owner")
	}
	return s.ChoiceBits, nil
}

func (s *State) WritePCMChoice(_ uint64, value uint8) error {
	if s == nil || s.ChoiceBits > 1 || value > 1 {
		return errors.New("unsupported PCM choice write")
	}
	s.ChoiceBits = value
	return nil
}
