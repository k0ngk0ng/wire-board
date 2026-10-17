package engine

// OriginalScoreDifference restores bank 5 $9F20 before team AI assignment.
// Zero denotes a tie; low bits are the original <4, <16 and >=16 point bands.
// Bit 7 denotes a negative signed 16-bit difference (team 1 leading in the
// ordinary score domain). Preserve the original subtraction wrap at extremes.
func OriginalScoreDifference(scores [2]uint16) uint8 {
	difference := int32(int16(scores[0] - scores[1]))
	if difference == 0 {
		return 0
	}
	var direction uint8
	if difference < 0 {
		direction = 128
		difference = -difference
	}
	band := uint8(1)
	if difference >= 16 {
		band = 3
	} else if difference >= 4 {
		band = 2
	}
	return direction | band
}
