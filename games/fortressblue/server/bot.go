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
	for _, supply := range s.Supplies {
		if supply.PickupAt > 0 && supply.PickupSeat == ctx.Actor {
			return nil, nil
		}
	}
	tank := &s.Tanks[ctx.Actor]
	target := botTarget(s, ctx.Actor)
	if target < 0 {
		return nil, nil
	}
	if slot, ok := botItem(s, ctx.Actor); ok {
		return json.Marshal(Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: slot})
	}
	if dir := botSupplyMove(s, ctx.Actor, ctx.Now.UnixMilli()); dir != 0 {
		return json.Marshal(Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: dir})
	}
	if tank.PendingShotEffect == "heal" {
		target = ctx.Actor
		missing := tank.MaxHP - tank.HP
		for i, t := range s.Tanks {
			if t.Alive && t.Team == tank.Team && t.MaxHP-t.HP > missing {
				target, missing = i, t.MaxHP-t.HP
			}
		}
	}
	spec, ok := tankSpec(tank.TankID)
	if !ok {
		return nil, nil
	}
	weapon := "primary"
	if (tank.SpecialUnlimited || tank.Special > 0) && hashString(s.Epoch, "special", string(rune(ctx.Actor)), string(rune(s.Turn)))%4 == 0 {
		weapon = "special"
	}
	aimState := s
	if tank.TankID == 14 && tank.PendingShotEffect == "" {
		ally, missing := -1, 0
		for i, t := range s.Tanks {
			if t.Alive && t.Team == tank.Team && t.MaxHP-t.HP > missing {
				ally, missing = i, t.MaxHP-t.HP
			}
		}
		if ally < 0 {
			weapon = "primary"
		} else if tank.SpecialUnlimited || tank.Special > 0 {
			weapon = "special"
			target = ally
			copyState := *s
			copyState.Tanks = append([]Tank(nil), s.Tanks...)
			copyState.Tanks[ctx.Actor].PendingShotEffect = "heal"
			aimState = &copyState
		}
	}
	best := botAimFor(aimState, ctx.Actor, target, spec)
	// A blocked shot is not the only legal action. Explore safe movements through
	// the same public movement rules, and never choose a move that starts a fall.
	risk := botGroundRisk(s, ctx.Actor, ctx.Now.UnixMilli())
	if (best.Score > 60 || risk > 0) && tank.Fuel > 0 {
		preferred := best.Facing
		if tank.Fuel < float64(spec.Speed) {
			preferred = tank.Facing
		}
		bestDir := 0
		bestScore := best.Score - 3
		bestRisk := risk
		fallback := 0
		for _, dir := range []int{preferred, -preferred} {
			candidate := *aimState
			candidate.Tanks = append([]Tank(nil), aimState.Tanks...)
			candidate.Events = nil
			if !moveTank(&candidate, ctx.Actor, dir, ctx.Now.UnixMilli()) || candidate.Resolution != nil {
				continue
			}
			candidateRisk := botGroundRisk(&candidate, ctx.Actor, ctx.Now.UnixMilli())
			if candidateRisk > risk {
				continue
			}
			if risk > 0 {
				if candidateRisk < bestRisk {
					bestRisk, bestDir = candidateRisk, dir
				}
				continue
			}
			if fallback == 0 {
				fallback = dir
			}
			aim := botAimFor(&candidate, ctx.Actor, target, spec)
			if aim.Score < bestScore {
				bestScore, bestDir = aim.Score, dir
			}
		}
		if bestDir == 0 && best.Score > 120 {
			bestDir = fallback
		}
		if bestDir != 0 {
			return json.Marshal(Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: bestDir})
		}
	}
	best = botAimError(s, ctx.Actor, spec, best)
	return json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: best.Angle, Power: best.Power, Facing: best.Facing, Weapon: weapon})
}

type botAim struct {
	Angle, Power, Score float64
	Facing              int
}

func botAimFor(s *State, seat, target int, spec fbdata.Tank) botAim {
	tank, foe := s.Tanks[seat], s.Tanks[target]
	facing := 1
	if foe.X < tank.X {
		facing = -1
	}
	stage, _ := fbmaps.ByID(s.MapID)
	mask := bulletMaskFor(stage, s.Craters)
	best := botAim{Angle: float64(spec.MinAngle), Power: 70, Score: math.Inf(1), Facing: facing}
	step := 5.0
	if s.Options.BotDifficulty == "normal" {
		step = 8
	} else if s.Options.BotDifficulty == "easy" {
		step = 12
	}
	// Include the actual minimum rather than assuming every tank starts at 5°.
	for angle := float64(spec.MinAngle); angle <= float64(spec.Angle); angle += step {
		for power := 0.0; power <= 100; power += step {
			arc := simulateArc(stage, mask, s.Tanks, seat, tank.X, tank.Y, angle, power, facing, float64(spec.Range)/50, s.Wind, MaxPath)
			center := tankCenter(foe)
			score := math.Hypot(arc.Impact.X-center.X, arc.Impact.Y-center.Y)
			if arc.Hit == target {
				score -= 1000
			}
			if arc.OutOfBounds {
				score += 500
			}
			if arc.Hit == seat {
				score += 1500
			} else if arc.Hit >= 0 && s.Tanks[arc.Hit].Team == tank.Team {
				score += 500
			}
			if tank.PendingShotEffect == "heal" {
				score = math.Hypot(arc.Impact.X-center.X, arc.Impact.Y-center.Y)
				if arc.OutOfBounds {
					score += 500
				} else {
					for _, other := range s.Tanks {
						c := tankCenter(other)
						if !other.Alive || math.Hypot(c.X-arc.Impact.X, c.Y-arc.Impact.Y) > 50 {
							continue
						}
						gain := float64(min(other.MaxHP-other.HP, other.MaxHP*30/100))
						if other.Team == tank.Team {
							score -= gain * 5
						} else {
							score += gain * 5
						}
					}
				}
			}
			if score < best.Score {
				best.Angle, best.Power, best.Score = angle, power, score
			}
		}
	}
	return best
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

func botGroundRisk(s *State, seat int, at int64) float64 {
	point := tankPositionAt(s, seat, at)
	risk := 0.0
	for _, h := range s.Hazards {
		if at < h.At || at >= h.EndAt || point.Y < h.Y-24 || point.Y > h.Y+12 {
			continue
		}
		risk += math.Max(0, h.R-math.Abs(point.X-h.X)+1)
	}
	return risk
}

// Only decision precision changes; hard preserves the original hash and bounds.
func botAimError(s *State, seat int, spec fbdata.Tank, aim botAim) botAim {
	angleSpan, powerSpan := uint64(5), uint64(1)
	switch s.Options.BotDifficulty {
	case "normal":
		angleSpan, powerSpan = 11, 9
	case "easy":
		angleSpan, powerSpan = 21, 17
	}
	angle := float64(hashString(s.Epoch, "aim", string(rune(seat)), string(rune(s.Turn)))%angleSpan) - float64(angleSpan/2)
	power := float64(hashString(s.Epoch, "power-error", string(rune(seat)), string(rune(s.Turn)))%powerSpan) - float64(powerSpan/2)
	aim.Angle = clamp(aim.Angle+angle, float64(spec.MinAngle), float64(spec.Angle))
	aim.Power = clamp(aim.Power+power, 0, 100)
	return aim
}
