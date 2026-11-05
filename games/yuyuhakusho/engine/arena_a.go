package engine

import "errors"

// StepArenaA recovers 21126 and the exact writes that create its 9e00 child.
// Rows, camera and feedback are transient views of the caller's owned storage;
// all arena scratch words remain in the canonical Object.
func StepArenaA(state ArenaSceneState, child WorldObject, view DefaultArenaView, library ArenaSceneLibrary) (ArenaSceneState, WorldObject, error) {
	if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
		return state, child, nil
	}
	original, originalChild := state, child
	if state.Object.Header.Stage == 0 {
		if child.ObjectID != 0x9e00 {
			return original, originalChild, errors.New("arena A child slot is unavailable")
		}
		child.Header = MatchObjectHeader{Behavior: 0x21258, Size: 0x40}
		child.Aux18 = int32(uint32(child.Aux18)&0xffff0000 | uint32(state.Object.ObjectID))
		state.Mode, state.BackdropColor = 3, 0
		state.Object.Header.Stage += 2
	}
	var err error
	state.Camera, err = StepDefaultArenaCamera(state.Camera)
	if err != nil {
		return original, originalChild, err
	}
	row := library.LinearRows[state.Camera.X]
	if len(row) != 128 {
		return original, originalChild, errors.New("arena A linear row is unavailable")
	}
	dy := state.Camera.Y - view.Shake
	state.Vertical = [2]int16{dy, (dy >> 1) + 16}
	double := dy + dy
	state.SourceA = int32(double - 0x1800)
	half := (double >> 1) &^ int16(1)
	state.SourceB = int32(half - 0x15e0)

	base := row[40]
	state.Object.ScreenX = base
	address := uint32(int32(0x24000) + int32(state.Camera.X-0x2e0)*256 + 0x50)
	state.Object.ScreenY, state.Object.Aux0E = int16(address>>16), uint16(address)
	state.Object.Body.VelocityX = int32(uint32(uint16(base>>1))<<16 | uint32(state.Object.Body.VelocityX)&0xffff)
	near := -(state.Camera.X - 0x360)
	near += near
	near -= 0x60
	state.Object.Body.VerticalVelocity = int32(uint32(uint16(near))<<16 | uint32(state.Object.Body.VerticalVelocity)&0xffff)
	for i := 80; i < 184; i++ {
		state.RowsA[i] = base
	}
	copy(state.RowsA[184:248], row[40:104])
	for i := 16; i < 176; i++ {
		state.RowsB[i] = base >> 1
	}
	for i := 176; i < 240; i++ {
		state.RowsB[i] = near
	}
	state, err = ApplyArenaSceneFeedback(state, view)
	if err != nil {
		return original, originalChild, err
	}
	return state, child, nil
}
