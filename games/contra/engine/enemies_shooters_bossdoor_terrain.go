package contra

import "errors"

// setBossDoorCollision follows original E98B: the low two pattern bits apply
// across the top pair of 16×16 cells; the next two bits apply to the lower pair.
// Code4 thus creates a floor under an empty tunnel rather than a solid square.
func setBossDoorCollision(window *CollisionWindow, address uint16, pattern uint8) error {
	index := int((address>>8)&7)<<4 | int((address>>4)&12) | int((address>>3)&3)
	column := int(address>>1) & 3
	if window == nil || index+4+(column+1)/4 >= len(window) {
		return errors.New("boss-door collision address outside mapped windows")
	}
	for row := 0; row < 2; row++ {
		code := (pattern >> (row * 2)) & 3
		for col := 0; col < 2; col++ {
			cell := column + col
			at := index + row*4 + cell/4
			shift := 6 - 2*(cell&3)
			window[at] = window[at]&^(uint8(3)<<shift) | code<<shift
		}
	}
	return nil
}
