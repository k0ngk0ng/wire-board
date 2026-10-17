package engine

import "fmt"

const maximumCourtPatchTiles = 1728 // plane 0 rows 6..29, plane 1 rows 0..29.

func validateCourtPatch(p CourtTilePatch) error {
	if p.Plane > 1 || p.Row >= 30 || p.Plane == 0 && p.Row < 6 || p.Column >= 32 || len(p.Tiles) == 0 || int(p.Column)+len(p.Tiles) > 32 {
		return fmt.Errorf("invalid court tile patch extent")
	}
	for _, tile := range p.Tiles {
		if tile < 0 || tile > 255 {
			return fmt.Errorf("invalid court tile %d", tile)
		}
	}
	return nil
}
func validateCourtPatchBatch(patches []CourtTilePatch, unique bool) error {
	total := 0
	seen := map[int]bool{}
	for _, p := range patches {
		if err := validateCourtPatch(p); err != nil {
			return err
		}
		if unique && len(p.Tiles) != 1 {
			return fmt.Errorf("court history must contain individual final tiles")
		}
		total += len(p.Tiles)
		if total > maximumCourtPatchTiles {
			return fmt.Errorf("court patch history exceeds physical tile capacity")
		}
		for i := range p.Tiles {
			key := int(p.Plane)*960 + int(p.Row)*32 + int(p.Column) + i
			if unique && seen[key] {
				return fmt.Errorf("duplicate court history tile")
			}
			seen[key] = true
		}
	}
	return nil
}
func validateRuntimeCourtAnimationDisplay(s DisplayState) error {
	if err := validateCourtPatchBatch(s.CourtPatches, true); err != nil {
		return err
	}
	return validateCourtPatchBatch(s.PendingPatches, false)
}

// QueueDisplayPatches stages one complete animation command batch. It does not
// publish tiles before video commit or retain caller-owned slices. An empty
// producer result leaves an already pending video batch untouched.
func QueueDisplayPatches(original DisplayState, patches []CourtTilePatch) (DisplayState, error) {
	if err := validateCourtPatchBatch(patches, false); err != nil {
		return original, err
	}
	if len(patches) == 0 {
		return original, nil
	}
	s := original
	s.PendingPatches = make([]CourtTilePatch, len(patches))
	for i, p := range patches {
		s.PendingPatches[i] = p
		s.PendingPatches[i].Tiles = append([]int(nil), p.Tiles...)
	}
	return s, nil
}

func commitDisplayCourtPatches(s *DisplayState, column *CourtColumnCommit) error {
	if err := validateRuntimeCourtAnimationDisplay(*s); err != nil {
		return err
	}
	if len(s.CourtPatches) == 0 && len(s.PendingPatches) == 0 {
		return nil
	}
	if column == nil && len(s.PendingPatches) == 0 {
		return nil
	}
	tiles := make(map[int]int, len(s.CourtPatches)+len(s.PendingPatches))
	for _, p := range s.CourtPatches {
		if column == nil || p.Column != column.Column&31 {
			tiles[int(p.Plane)*960+int(p.Row)*32+int(p.Column)] = p.Tiles[0]
		}
	}
	// Native CCxx submits all animation queues after the CBxx column stream.
	for _, p := range s.PendingPatches {
		for i, tile := range p.Tiles {
			tiles[int(p.Plane)*960+int(p.Row)*32+int(p.Column)+i] = tile
		}
	}
	history := make([]CourtTilePatch, 0, len(tiles))
	for key := 0; key < 1920; key++ {
		if tile, ok := tiles[key]; ok {
			history = append(history, CourtTilePatch{Plane: uint8(key / 960), Row: uint8((key % 960) / 32), Column: uint8(key % 32), Tiles: []int{tile}})
		}
	}
	if len(history) == 0 {
		history = nil
	}
	s.CourtPatches = history
	if len(s.PendingPatches) != 0 {
		s.CourtSerial++
	}
	s.PendingPatches = nil
	return nil
}
