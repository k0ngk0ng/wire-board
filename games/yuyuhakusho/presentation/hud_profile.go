package presentation

import "errors"

// DefaultArenaRows recovers the observed normal-mode default battle display:
// rows14..207 are visible, the top six tile rows use the window, and backdrop
// color0 is uploaded after row15 and cleared again at the lower blank band.
// Scrolling and all other current colors are supplied by the independent world.
func DefaultArenaRows(palette [64]uint16, horizontal, vertical [224][2]uint16) ([224]ArenaRow, error) {
	for _, color := range palette {
		if color > 511 {
			return [224]ArenaRow{}, errors.New("invalid default arena compact palette")
		}
	}
	var rows [224]ArenaRow
	for y := range rows {
		rows[y] = ArenaRow{Horizontal: horizontal[y], Vertical: vertical[y], Palette: palette, Visible: y >= 14 && y < 208, Window: y < 48}
		if y < 16 || y >= 208 {
			rows[y].Palette[0] = 0
		}
	}
	return rows, nil
}
