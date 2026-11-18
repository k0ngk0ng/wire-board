package engine

import (
	"errors"
	"math"
)

func applyFlyingAim(state *State, tower *Tower, action Action) error {
	if state.Paused || state.Health <= 0 || !finiteNumber(action.X) || !finiteNumber(action.Y) || math.Abs(action.X) > 1000000 || math.Abs(action.Y) > 1000000 {
		return errors.New("请选择可控制的飞行伙伴与有效目标")
	}
	route, _ := routeForState(state)
	worldTower(state, tower).Destination = Point{X: max(0, min(float64(route.Width), action.X)), Y: max(0, min(float64(route.Height), action.Y))}
	return nil
}
