package fortressblue

import (
	"encoding/json"
	"math"

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
		class := classOrder[(ctx.Actor+int(s.Seed%uint64(len(classOrder))))%len(classOrder)]
		return json.Marshal(Action{Type: "select", Epoch: s.Epoch, Class: class})
	}
	if !s.canAct(ctx.Actor) {
		return nil, nil
	}
	tank := &s.Tanks[ctx.Actor]
	target := botTarget(s, ctx.Actor)
	if target < 0 {
		return nil, nil
	}
	foe := &s.Tanks[target]
	facing := 1
	if foe.X < tank.X {
		facing = -1
	}
	weapon := "primary"
	spec, _ := classByID(tank.Class)
	useSpecial := tank.Special > 0 && spec.Special.Shots > 0 && hashString(s.Epoch, "special", string(rune(ctx.Actor)), string(rune(s.Turn)))%5 == 0
	if useSpecial {
		weapon = "special"
	}
	bestAngle, bestPower := 45.0, 70.0
	bestDistance := math.Inf(1)
	bestHit := false
	for angle := 18.0; angle <= 82; angle += 6 {
		for power := 32.0; power <= 100; power += 6 {
			arc := simulateArc(s.Heights, s.Tanks, ctx.Actor, tank.X, tank.Y, angle, power, facing, s.Wind, MaxPath)
			distance := math.Hypot(arc.Impact.X-foe.X, arc.Impact.Y-(foe.Y-7))
			hit := arc.Hit == target
			if hit {
				distance -= 100
			}
			if distance < bestDistance {
				bestDistance, bestAngle, bestPower, bestHit = distance, angle, power, hit
			}
		}
	}
	if !bestHit {
		bestAngle, bestPower = 45, 82
	}
	// Deterministic tiny spread makes repeated bot matches less sterile without
	// ever changing legality.
	jitter := float64(hashString(s.Epoch, "aim", string(rune(ctx.Actor)), string(rune(s.Turn)))%5) - 2
	bestAngle = math.Max(5, math.Min(85, bestAngle+jitter))
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
