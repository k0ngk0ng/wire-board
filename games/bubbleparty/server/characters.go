package bubbleparty

const RegularCharacterCount = 8
const CharacterCount = 10

// CharacterProfile is keyed by the original Cxx resource ID, not the record
// position in Attr/BomberAttr. Speeds are original pixels per 30ms tick.
// Provenance: games/bubbleparty/data/classic-2002.json characterAttributes.
// Init 0x441c50 and power-ups 0x445740/0x4457a0/0x445870 establish the fields.
type CharacterProfile struct {
	ID           int
	InitialSpeed int
	MaximumSpeed int
	InitialBombs int
	MaximumBombs int
	InitialRange int
	MaximumRange int
	Hidden       bool
}

var originalCharacterProfiles = [CharacterCount]CharacterProfile{
	{0, 5, 6, 1, 10, 1, 6, false},
	{1, 4, 7, 2, 7, 1, 8, false},
	{2, 4, 7, 1, 8, 2, 7, false},
	{3, 5, 7, 1, 10, 1, 5, false},
	{4, 4, 8, 2, 9, 1, 5, false},
	{5, 5, 7, 1, 10, 1, 5, false},
	{6, 5, 9, 1, 6, 1, 7, false},
	{7, 4, 7, 1, 9, 2, 6, false},
	{8, 6, 9, 3, 9, 3, 7, true},
	{9, 5, 9, 3, 9, 3, 8, true},
}

func characterProfile(id int) (CharacterProfile, bool) {
	if id < 0 || id >= len(originalCharacterProfiles) {
		return CharacterProfile{}, false
	}
	return originalCharacterProfiles[id], true
}

// Display helper only. Simulation retains the exact 1200/nativeSpeed duration.
func characterMoveMS(nativeSpeed int) int {
	if nativeSpeed <= 0 {
		return 0
	}
	return (1200 + nativeSpeed - 1) / nativeSpeed
}
