package runtimebridge

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

type PresentationProjectionRequest struct {
	Surface engine.OriginalRasterSurface   `json:"surface"`
	Plan    engine.OriginalRasterFramePlan `json:"plan"`
}

// Row bytes stay local to browser Wasm. Network snapshots can retain compact
// committed surfaces and plans without serializing thousands of row objects.
type PresentationProjection struct {
	Format  uint8     `json:"format"`
	Palette [32]uint8 `json:"palette"`
	Rows    []byte    `json:"rows"`
}

func projectPresentation(input PresentationProjectionRequest) (PresentationProjection, error) {
	rows, err := engine.ProjectOriginalPostResultRasterRows(input.Surface, input.Plan)
	if err != nil {
		return PresentationProjection{}, err
	}
	result := PresentationProjection{Format: 1, Palette: rows.Palette, Rows: make([]byte, 0, 68000)}
	for y, row := range rows.Rows {
		if row.Y != uint16(y) || len(row.Segments) > 256 {
			return PresentationProjection{}, fmt.Errorf("invalid projected row")
		}
		result.Rows = append(result.Rows, row.SpriteCHR[:]...)
		result.Rows = append(result.Rows, row.Mask, uint8(len(row.Segments)), uint8(len(row.Segments)>>8))
		for _, segment := range row.Segments {
			if segment.X > 255 {
				return PresentationProjection{}, fmt.Errorf("projected segment outside frame")
			}
			result.Rows = append(result.Rows, uint8(segment.X), segment.Length, segment.Page, segment.Tile, segment.Row, segment.Column, segment.Palette, segment.Mask)
		}
	}
	return result, nil
}
