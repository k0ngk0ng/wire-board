package engine

import "fmt"

// OriginalBackgroundLaunchHeight restores the two background-column bytes
// aliased by DFF8 for auxiliary owner FF. BC74's lower row19 tile is 0135;
// BCFF's upper attribute row2 is 0142. This uses original static data and the
// last GENERATED column/phase, never a rendered image or an arbitrary player.
func OriginalBackgroundLaunchHeight(court, column, phase uint8) (Fixed, error) {
	if court >= 8 || column >= 64 {
		return 0, fmt.Errorf("background launch source outside original court/column tables")
	}
	page, group := column>>5, (column&31)>>2
	current := originalLaunchColumnAttributes[court][page][group]
	attribute := current
	full := phase&127 == 0 || column&32 == 0 && column&2 == 0 || column&32 != 0 && column&2 != 0
	if !full {
		other := originalLaunchColumnAttributes[court][page^1][group]
		otherMask, currentMask := uint8(0xcc), uint8(0x33)
		if column&2 != 0 {
			otherMask, currentMask = 0x33, 0xcc
		}
		attribute = other&otherMask | current&currentMask
	}
	return Fixed(uint16(originalLaunchColumnTiles[court][column])|uint16(attribute)<<8) * Pixel, nil
}
