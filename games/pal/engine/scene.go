package engine

import "errors"

// Scene command 2 preserves PAL_MakeScene's map-only waving stage. A/B are
// the applied level/phase, C is the original signed per-render progression.
func validateSceneDraws(draws []DrawOp) error {
	if len(draws) > 8192 {
		return errors.New("PAL scene commands exceed limit")
	}
	for i, d := range draws {
		if d.Kind < 0 || d.Kind > 2 {
			return errors.New("invalid PAL scene command")
		}
		if d.Kind == 2 && (i != 0 || d.A < 1 || d.A > 255 || d.B < 0 || d.B > 31 || d.C < -32768 || d.C > 32767 || d.D != 0 || d.X != 0 || d.BaseY != 0 || d.SortY != -2147483648) {
			return errors.New("invalid PAL map wave command")
		}
	}
	return nil
}
