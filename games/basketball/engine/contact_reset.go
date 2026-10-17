package engine

// ContactPhaseState retains the original per-step search/response bookkeeping.
// Marks are response marks, distinct from Candidates and the object update bits.
// Negative candidates use the engine's -1 sentinel rather than a ROM byte value.
type ContactPhaseState struct {
	Rotation           uint8
	Marks              [7]uint8
	Candidates         [7]int8
	Links              [7]uint8
	StartingBallAction BallAction
	Parity             uint8
}

// BeginOriginalContacts restores bank 5 $8469 before search or interaction.
// It clears only the processed bit in links, retaining attachment identity.
func BeginOriginalContacts(original ContactPhaseState, ballAction BallAction) ContactPhaseState {
	s := original
	s.Rotation++
	s.Marks = [7]uint8{}
	s.Candidates = [7]int8{-1, -1, -1, -1, -1, -1, -1}
	for i := range s.Links {
		s.Links[i] &^= 0x40
	}
	s.StartingBallAction = ballAction & 127
	s.Parity ^= 1
	return s
}
