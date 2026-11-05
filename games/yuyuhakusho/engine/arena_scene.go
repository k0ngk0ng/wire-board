package engine

import "errors"

// ArenaSceneState is a temporary view of one arena callback. Object owns every
// scratch word/long; row and pan arrays are views of the world's Cells/Stream.
type ArenaSceneState struct {
	Object              WorldObject
	Camera              DefaultArenaCamera
	Mode, BackdropColor uint8
	Vertical            [2]int16
	RowsA, RowsB        [256]int16
	PanA, PanB          [224]int16
	SourceA, SourceB    int32
}

type ArenaSceneLibrary struct {
	WaveX        []int8            `json:"wave_x"`
	LinearRows   map[int16][]int16 `json:"linear_rows"`
	Arena4Rows   map[uint8][]int16 `json:"arena4_rows"`
	Arena4Colors [4][3]uint16      `json:"arena4_colors"`
}

// ApplyArenaSceneFeedback copies only the proved HUD/scroll row window. The
// caller's exact source pointers select its scene-specific parallax formula.
func ApplyArenaSceneFeedback(state ArenaSceneState, view DefaultArenaView) (ArenaSceneState, error) {
	if view.Pan == 0 {
		return state, nil
	}
	original := state
	read := func(source int32) (rows [224]int16, err error) {
		if source < -0x1a00 || source > -0x15c0 || source&1 != 0 {
			return rows, errors.New("arena feedback source is outside owned row views")
		}
		start := int(source+0x1a00) / 2
		for i := range rows {
			index := start + i
			switch {
			case index < 256:
				rows[i] = view.HUDPrefix[index]
			case index < 512:
				rows[i] = state.RowsA[index-256]
			default:
				rows[i] = state.RowsB[index-512]
			}
			rows[i] += view.Pan
		}
		return rows, nil
	}
	a, err := read(state.SourceA)
	if err != nil {
		return original, err
	}
	b, err := read(state.SourceB)
	if err != nil {
		return original, err
	}
	state.PanA, state.PanB, state.SourceA, state.SourceB = a, b, -0xa000, -0x9e00
	return state, nil
}
