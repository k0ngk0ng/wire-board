package engine

import "errors"

func arena268LinearRow(library ArenaSceneLibrary, x int16) ([]int16, error) {
	row, ok := library.LinearRows[x]
	if !ok || len(row) != 128 {
		return nil, errors.New("arena scene linear row is unavailable")
	}
	return row, nil
}

func arena268Vertical(state *ArenaSceneState, view DefaultArenaView, halfB bool) {
	dy := state.Camera.Y - view.Shake
	double := dy + dy
	state.SourceA = int32(double - 0x1800)
	if halfB {
		state.Vertical = [2]int16{dy, (dy >> 1) + 0x10}
		b := (double >> 1) &^ 1
		state.SourceB = int32(b - 0x15e0)
	} else {
		state.Vertical = [2]int16{dy + 8, dy + 8}
		state.SourceB = int32(double - 0x1600)
	}
}

func arena268Fill(rows []int16, value int16) {
	for i := range rows {
		rows[i] = value
	}
}

func arena268Wave(rows []int16, base int16, phase, stride uint16, shift uint, wave []int8) {
	for i := range rows {
		rows[i] = base + (int16(wave[(phase&0x7fe)/2]) >> shift)
		phase += stride
	}
}

// StepArena2 recovers20ae0: three independent long drifts and word phases,
// signed-byte waves and the scene's own88-word perspective slice. Object is
// the sole scratch owner; high-word writes preserve their actual low words.
func StepArena2(state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, nil
	}
	original := state
	if state.Object.Header.Stage == 0 {
		state.Mode, state.BackdropColor = 3, 0
		state.Object.ScreenX, state.Object.Aux0A = 0, 0
		state.Object.Body.VerticalVelocity = 0
		state.Object.GraphicsSet, state.Object.TilesBase, state.Object.Body.Attributes = 0, 0, 0
		arena268Fill(state.RowsB[:120], 0x1a0)
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, err
	}
	row, err := arena268LinearRow(library, state.Camera.X)
	if err != nil {
		return original, err
	}
	if len(library.WaveX) != 1024 {
		return original, errors.New("arena scene signed-byte waveform is unavailable")
	}
	arena268Vertical(&state, view, false)
	if view.Pan == 0 {
		drift := int32(uint32(uint16(state.Object.ScreenX))<<16 | uint32(state.Object.Aux0A))
		drift += 0x4000
		state.Object.ScreenX, state.Object.Aux0A = int16(drift>>16), uint16(drift)
		state.Object.Body.VerticalVelocity += 0x6000
		state.Object.GraphicsSet += 0x8000
		a, b := uint16(state.Object.TilesBase>>16)+8, uint16(state.Object.TilesBase)+0x10
		state.Object.TilesBase = uint32(a)<<16 | uint32(b)
		state.Object.Body.Attributes += 0x10
	}
	first := row[16]
	state.Object.Aux18 = worldObjectHighWord(state.Object.Aux18, uint16(first))
	half := first + 0x60
	half >>= 1
	state.Object.ScreenY = first - half
	arena268Wave(state.RowsA[80:104], state.Object.ScreenX, uint16(state.Object.TilesBase>>16), 0x300, 3, library.WaveX)
	arena268Fill(state.RowsA[104:160], state.Object.ScreenY)
	copy(state.RowsA[160:248], row[16:104])
	arena268Wave(state.RowsB[120:152], int16(state.Object.Body.VerticalVelocity>>16)-state.Camera.X,
		uint16(state.Object.TilesBase), 0x80, 5, library.WaveX)
	arena268Fill(state.RowsB[152:192], first)
	arena268Wave(state.RowsB[192:240], int16(state.Object.GraphicsSet>>16)-state.Camera.X,
		state.Object.Body.Attributes, 0x300, 4, library.WaveX)
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, err
	}
	return state, nil
}

// StepArena6 recovers20e28, including MULS/SWAP's signed product high word
// and the half-speed B source's word-double/ASR/even-address sequence.
func StepArena6(state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, nil
	}
	original := state
	if state.Object.Header.Stage == 0 {
		state.Mode, state.BackdropColor = 3, 0
		arena268Fill(state.RowsA[16:112], 0x1a0)
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, err
	}
	row, err := arena268LinearRow(library, state.Camera.X)
	if err != nil {
		return original, err
	}
	arena268Vertical(&state, view, true)
	x := state.Camera.X
	delta := -(x - 0x360)
	product := int32(delta) * 0x7000
	state.Object.ScreenX = int16(product>>16) - 0x60
	state.Object.ScreenY = x - 0x3c0
	state.Object.Body.VelocityX = worldObjectHighWord(state.Object.Body.VelocityX, uint16((x>>1)-0x210))
	state.Object.Body.VerticalVelocity = worldObjectHighWord(state.Object.Body.VerticalVelocity, uint16((x>>2)-0x138))
	state.Object.Aux18 = worldObjectHighWord(state.Object.Aux18, uint16((x>>3)-0xcc))
	arena268Fill(state.RowsA[112:176], state.Object.ScreenX)
	copy(state.RowsA[176:248], row[32:104])
	arena268Fill(state.RowsB[16:136], state.Object.ScreenY)
	arena268Fill(state.RowsB[136:152], int16(state.Object.Body.VelocityX>>16))
	arena268Fill(state.RowsB[152:168], int16(state.Object.Body.VerticalVelocity>>16))
	arena268Fill(state.RowsB[168:208], int16(state.Object.Aux18>>16))
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, err
	}
	return state, nil
}

// StepArena8 recovers20fb4. Its saved perspective pointer occupies the actual
// ScreenY/Aux0E long; GraphicsSet's high word alone owns the wave phase.
func StepArena8(state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, nil
	}
	original := state
	if state.Object.Header.Stage == 0 {
		state.Mode, state.BackdropColor = 3, 0
		state.Object.GraphicsSet &= 0xffff
		arena268Fill(state.RowsB[16:144], 0x1a0)
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, err
	}
	row, err := arena268LinearRow(library, state.Camera.X)
	if err != nil {
		return original, err
	}
	if len(library.WaveX) != 1024 {
		return original, errors.New("arena scene signed-byte waveform is unavailable")
	}
	arena268Vertical(&state, view, true)
	phase := uint16(state.Object.GraphicsSet >> 16)
	if view.Pan == 0 {
		phase += 0x20
		state.Object.GraphicsSet = uint32(phase)<<16 | state.Object.GraphicsSet&0xffff
	}
	x := state.Camera.X
	pointer := uint32(int32(x-0x2e0)<<8) + 0x24000
	state.Object.ScreenX = row[0]
	state.Object.ScreenY, state.Object.Aux0E = int16(pointer>>16), uint16(pointer)
	state.Object.Body.VerticalVelocity = worldObjectHighWord(state.Object.Body.VerticalVelocity, uint16(row[0]))
	delta := x - 0x360
	state.Object.Body.VelocityX = worldObjectHighWord(state.Object.Body.VelocityX, uint16((delta>>2)-0x60))
	back := -delta
	back += back
	back -= 0x60
	state.Object.Aux18 = worldObjectHighWord(state.Object.Aux18, uint16(back))
	arena268Fill(state.RowsA[80:144], state.Object.ScreenX)
	copy(state.RowsA[144:248], row[:104])
	arena268Fill(state.RowsB[16:144], int16(state.Object.Body.VelocityX>>16))
	arena268Wave(state.RowsB[144:200], row[0], phase, 0x200, 5, library.WaveX)
	arena268Fill(state.RowsB[200:240], int16(state.Object.Aux18>>16))
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, err
	}
	return state, nil
}
