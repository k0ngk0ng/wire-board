package engine

import "errors"

// StepArena4Palette recovers 20dca. The caller owns pause/slow gating; this
// helper gates only on the original F501 byte and F572 word. TilesBase is
// the object's canonical +20/+22 phase/timer pair, not a graphics duplicate.
func StepArena4Palette(state ArenaSceneState, view DefaultArenaView, palette ArenaPaletteState, library ArenaSceneLibrary) (ArenaSceneState, ArenaPaletteState, error) {
	if palette.Status != 0 || view.Pan != 0 {
		return state, palette, nil
	}
	phase, timer := uint16(state.Object.TilesBase>>16), uint16(state.Object.TilesBase)-1
	state.Object.TilesBase = uint32(phase)<<16 | uint32(timer)
	if int16(timer) >= 0 {
		return state, palette, nil
	}
	phase = (phase + 4) & 12
	state.Object.TilesBase = uint32(phase)<<16 | 5
	copy(palette.Primary[9:12], library.Arena4Colors[phase/4][:])
	return state, palette, nil
}

// StepArena4 recovers 20cb2, including its three distinct initialization
// bands, reversed/forward row copies, and three-color helper.
func StepArena4(state ArenaSceneState, view DefaultArenaView, palette ArenaPaletteState, library ArenaSceneLibrary) (ArenaSceneState, ArenaPaletteState, error) {
	original, originalPalette := state, palette
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, palette, nil
	}
	if state.Object.Header.Stage == 0 {
		state.Mode, state.Object.TilesBase = 3, 0
		for i := 80; i < 112; i++ {
			state.RowsA[i] = 0x1a0
		}
		for i := 176; i < 240; i++ {
			state.RowsA[i] = 0x1a0
		}
		for i := 120; i < 168; i++ {
			state.RowsB[i] = 0x1a0
		}
		state.Object.Header.Stage += 2
	}
	state, palette, _ = StepArena4Palette(state, view, palette, library)
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, originalPalette, err
	}
	row, ok := library.Arena4Rows[uint8(state.Camera.X)&63]
	if !ok || len(row) != 112 {
		return original, originalPalette, errors.New("arena4 scrolling row is unavailable")
	}
	dy := state.Camera.Y - view.Shake
	doubled := dy * 2
	state.Vertical = [2]int16{dy, dy}
	state.SourceA, state.SourceB = int32(doubled-0x1800), int32(doubled-0x1600)
	state.Object.ScreenX = row[8]
	for i := 112; i < 176; i++ {
		state.RowsA[i] = state.Object.ScreenX
	}
	for i, value := range row {
		state.RowsB[119-i] = value
	}
	copy(state.RowsB[168:248], row[:80])
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, originalPalette, err
	}
	return state, palette, nil
}

// arenaCEVertical retains the original doubled-word overflow before ASR and
// even alignment. Reducing this expression to dy changes extreme inputs.
func arenaCEVertical(state *ArenaSceneState, view DefaultArenaView) {
	dy := state.Camera.Y - view.Shake
	doubled := dy * 2
	state.Vertical = [2]int16{dy, (dy >> 1) + 16}
	state.SourceA = int32(doubled - 0x1800)
	state.SourceB = int32(((doubled >> 1) & -2) - 0x15e0)
}

// StepArenaC recovers 212f8. Its +10/+14 longs and +1c phase remain aliases
// of the object's existing velocity/graphics fields throughout the call.
func StepArenaC(state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	original := state
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, nil
	}
	if state.Object.Header.Stage == 0 {
		state.Mode, state.BackdropColor = 3, 0
		state.Object.GraphicsSet = 0
		state.Object.Body.VelocityX, state.Object.Body.VerticalVelocity = 0, 0
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, err
	}
	if len(library.WaveX) != 1024 {
		return original, errors.New("arenaC waveform is unavailable")
	}
	arenaCEVertical(&state, view)
	if view.Pan == 0 {
		state.Object.Body.VelocityX += 0x10000
		state.Object.Body.VerticalVelocity -= 0x30000
		phase := uint16(state.Object.GraphicsSet>>16) + 16
		state.Object.GraphicsSet = uint32(phase)<<16 | state.Object.GraphicsSet&0xffff
	}
	base := -(state.Camera.X - 0x360) + 0x60
	state.Object.ScreenX = base
	phase := uint16(state.Object.GraphicsSet >> 16)
	for i := 0; i < 80; i++ {
		value := base + (int16(library.WaveX[(phase&0x7fe)/2]) >> 3)
		state.RowsA[80+i*2], state.RowsA[81+i*2] = value, value
		phase += 0x20
	}
	first := int16(state.Object.Body.VelocityX>>16) + base
	second := int16(state.Object.Body.VerticalVelocity>>16) + base
	if view.FrameCounter&1 == 0 {
		first, second = second, first
	}
	for i := 16; i < 240; i += 2 {
		state.RowsB[i], state.RowsB[i+1] = first, second
	}
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, err
	}
	return state, nil
}

// StepArenaE recovers 21412. ScreenX/Aux0A own the +08 long accumulator;
// writes to +14/+18 preserve the low halves of their canonical long fields.
func StepArenaE(state ArenaSceneState, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, error) {
	original := state
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, nil
	}
	if state.Object.Header.Stage == 0 {
		state.Mode, state.BackdropColor = 3, 0
		state.Object.ScreenX, state.Object.Aux0A = 0, 0
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, err
	}
	row, ok := library.LinearRows[state.Camera.X]
	if !ok || len(row) != 128 {
		return original, errors.New("arenaE scrolling row is unavailable")
	}
	arenaCEVertical(&state, view)
	if view.Pan == 0 {
		drift := uint32(uint16(state.Object.ScreenX))<<16 | uint32(state.Object.Aux0A)
		drift += 0x8000
		state.Object.ScreenX, state.Object.Aux0A = int16(drift>>16), uint16(drift)
	}
	state.Object.ScreenY = row[40]
	state.Object.Body.VelocityX = 0x24000 + (int32(state.Camera.X-0x2e0) << 8) + 0x50
	state.Object.Body.VerticalVelocity = worldObjectHighWord(state.Object.Body.VerticalVelocity, uint16(state.Object.ScreenY>>1))
	offset := -(state.Camera.X-0x360)*2 - 0x60
	state.Object.Aux18 = worldObjectHighWord(state.Object.Aux18, uint16(offset))
	for i := 80; i < 144; i++ {
		state.RowsA[i] = state.Object.ScreenX
	}
	for i := 144; i < 184; i++ {
		state.RowsA[i] = state.Object.ScreenY
	}
	copy(state.RowsA[184:248], row[40:104])
	for i := 16; i < 200; i++ {
		state.RowsB[i] = state.Object.ScreenY >> 1
	}
	for i := 200; i < 240; i++ {
		state.RowsB[i] = offset
	}
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, err
	}
	return state, nil
}
