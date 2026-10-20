package fortressblue

import (
	"encoding/json"
	"math"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) || s.Finished {
		return nil, nil
	}
	if s.Phase == PhaseSelect {
		if s.Tanks[ctx.Actor].Selected {
			return nil, nil
		}
		ids := fbdata.IDs()
		if len(ids) == 0 {
			return nil, nil
		}
		id := ids[(ctx.Actor+int(hashString(s.Epoch, "select", string(rune(ctx.Actor)))%uint64(len(ids))))%len(ids)]
		return json.Marshal(Action{Type: "select", Epoch: s.Epoch, Tank: id})
	}
	if !s.canAct(ctx.Actor) {
		return nil, nil
	}
	tank := &s.Tanks[ctx.Actor]
	target := botTarget(s, ctx.Actor)
	if target < 0 {
		return nil, nil
	}
	if slot, ok := botItem(s, ctx.Actor); ok {
		return json.Marshal(Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: slot})
	}
	foe := &s.Tanks[target]
	facing := 1
	if foe.X < tank.X {
		facing = -1
	}
	spec, ok := tankSpec(tank.TankID)
	if !ok {
		return nil, nil
	}
	weapon := "primary"
	if tank.Special > 0 && hashString(s.Epoch, "special", string(rune(ctx.Actor)), string(rune(s.Turn)))%4 == 0 {
		weapon = "special"
	}
	stage, _ := fbmaps.ByID(s.MapID)
	mask := maskFor(stage, s.Craters)
	bestAngle, bestPower := 45.0, 70.0
	bestScore := math.Inf(1)
	maxAngle := float64(spec.Angle)
	if maxAngle < 15 {
		maxAngle = 15
	}
	for angle := 5.0; angle <= maxAngle; angle += 5 {
		for power := 25.0; power <= 100; power += 5 {
			rangeScale := float64(spec.Range) / 50
			arc := simulateArc(stage, mask, s.Tanks, ctx.Actor, tank.X, tank.Y, angle, power, facing, rangeScale, s.Wind, MaxPath)
			c := tankCenter(*foe)
			score := math.Hypot(arc.Impact.X-c.X, arc.Impact.Y-c.Y)
			if arc.Hit == target {
				score -= 1000
			}
			// Prefer shots that land on the target's side and stay inside the map.
			if arc.Impact.X < 0 || arc.Impact.X > float64(stage.Width) {
				score += 500
			}
			if score < bestScore {
				bestScore, bestAngle, bestPower = score, angle, power
			}
		}
	}
	jitter := float64(hashString(s.Epoch, "aim", string(rune(ctx.Actor)), string(rune(s.Turn)))%5) - 2
	bestAngle = clamp(bestAngle+jitter, 1, maxAngle)
	bestPower = clamp(bestPower, 20, 100)
	return json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: bestAngle, Power: bestPower, Facing: facing, Weapon: weapon})
}

func botTarget(s *State, seat int) int {
	best, bestDistance := -1, math.Inf(1)
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive || t.Team == s.Tanks[seat].Team {
			continue
		}
		distance := math.Hypot(t.X-s.Tanks[seat].X, t.Y-s.Tanks[seat].Y)
		if distance < bestDistance {
			best, bestDistance = i, distance
		}
	}
	return best
}
