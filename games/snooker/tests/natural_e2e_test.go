package snooker_test

// A test-only shot planner operates on the same public ball positions players
// see. It sends only angle/power/nomination actions through the real module;
// it never moves balls or injects scores, pots, trajectories or referee results.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func distance(a, b game.Vec) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
func pathClear(a, b game.Vec, balls []game.Ball, ignore1, ignore2 int) bool {
	dx, dy := b.X-a.X, b.Y-a.Y
	den := dx*dx + dy*dy
	for _, ball := range balls {
		if ball.Down || ball.ID == ignore1 || ball.ID == ignore2 {
			continue
		}
		u := ((ball.Pos.X-a.X)*dx + (ball.Pos.Y-a.Y)*dy) / den
		if u <= 0 || u >= 1 {
			continue
		}
		near := game.Vec{X: a.X + u*dx, Y: a.Y + u*dy}
		if distance(near, ball.Pos) < 2*game.BallRadius+.0001 {
			return false
		}
	}
	return true
}
func plannedShot(t *testing.T, s game.State) game.Strike {
	t.Helper()
	var cue game.Ball
	targets := []game.Ball{}
	for _, b := range s.Balls {
		if b.ID == 0 {
			cue = b
		}
		if b.Down || b.ID == 0 {
			continue
		}
		v := game.BallValue(b.ID)
		if s.Target == "red" && v == 1 || s.Target == "colour" && v >= 2 || s.Target == "clear" && v == s.Clear {
			targets = append(targets, b)
		}
	}
	if cue.Down || len(targets) == 0 {
		t.Fatal("planner received no cue or targets")
	}
	pockets := []game.Vec{{X: 0, Y: 0}, {X: game.TableWidth / 2, Y: -.026}, {X: game.TableWidth, Y: 0}, {X: 0, Y: game.TableHeight}, {X: game.TableWidth / 2, Y: game.TableHeight + .026}, {X: game.TableWidth, Y: game.TableHeight}}
	candidates := []game.Strike{}
	for _, b := range targets {
		for _, p := range pockets {
			length := distance(b.Pos, p)
			ux, uy := (p.X-b.Pos.X)/length, (p.Y-b.Pos.Y)/length
			ghost := game.Vec{X: b.Pos.X - 2*game.BallRadius*ux, Y: b.Pos.Y - 2*game.BallRadius*uy}
			if ghost.X < game.BallRadius || ghost.X > game.TableWidth-game.BallRadius || ghost.Y < game.BallRadius || ghost.Y > game.TableHeight-game.BallRadius {
				continue
			}
			if !pathClear(cue.Pos, ghost, s.Balls, 0, b.ID) || !pathClear(b.Pos, p, s.Balls, b.ID, 0) {
				continue
			}
			travel := distance(cue.Pos, ghost)
			dot := (ghost.X-cue.Pos.X)*ux + (ghost.Y-cue.Pos.Y)*uy
			if dot <= travel*.3 {
				continue
			}
			power := math.Min(.7, math.Max(.10, math.Sqrt((travel+length)*.36)/4.2*1.7))
			for _, scale := range []float64{.85, 1, 1.18} {
				candidates = append(candidates, game.Strike{Angle: math.Atan2(ghost.Y-cue.Pos.Y, ghost.X-cue.Pos.X), Power: math.Min(.85, power*scale), Nominate: game.BallValue(b.ID)})
			}
		}
	}
	// A legal contact or safety is a fallback when no direct pot is available.
	for _, b := range targets {
		for _, power := range []float64{.24, .45} {
			candidates = append(candidates, game.Strike{Angle: math.Atan2(b.Pos.Y-cue.Pos.Y, b.Pos.X-cue.Pos.X), Power: power, Nominate: game.BallValue(b.ID)})
		}
	}
	best := game.Strike{}
	score := math.Inf(-1)
	context := game.RuleContext{Balls: s.Balls, Target: s.Target, Clear: s.Clear, FreeBall: s.FreeBall, InHand: s.InHand, Scores: s.Scores, Team: s.Teams[s.Turn]}
	for _, shot := range candidates {
		motion, e := game.Simulate(s.Balls, shot)
		if e != nil {
			continue
		}
		rule := game.Judge(context, shot, motion)
		value := float64(rule.Points)*1000 - float64(rule.Foul)*10000 - shot.Power*10
		if rule.KeepTurn {
			value += 100
		}
		for _, id := range motion.Potted {
			if id == 0 {
				value -= 10000
			}
		}
		if value > score {
			score = value
			best = shot
		}
	}
	if best.Power == 0 {
		t.Fatal("no computable physical shot")
	}
	return best
}
func TestSnookerNaturalRackToFinishedMatch(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(map[int]string{2: "singles", 4: "doubles"}[n], func(t *testing.T) {
			bestOf := 1
			if n == 4 {
				bestOf = 3
			}
			mod, ctx, m := begin(t, n, bestOf)
			records := []any{map[string]any{"scope": "natural rack; deterministic test planner; real module shots and system timers", "initial": state(t, m)}}
			shots := 0
			for steps := 0; steps < 850 && m.Result == nil; steps++ {
				s := state(t, m)
				ctx.Actor = s.Turn
				ctx.Now = ctx.Now.Add(time.Second)
				switch s.Phase {
				case "aim":
					for _, b := range s.Balls {
						if b.ID == 0 && b.Down {
							placed := false
							for _, p := range []game.Vec{{X: game.BaulkX - .15, Y: game.TableHeight / 2}, {X: game.BaulkX - .08, Y: game.TableHeight/2 - .15}, {X: game.BaulkX - .08, Y: game.TableHeight/2 + .15}, {X: game.BaulkX - .25, Y: game.TableHeight / 2}} {
								a := mustAction(map[string]any{"type": "place", "window": s.Window, "x": p.X, "y": p.Y})
								next, e := mod.Apply(ctx, m, a)
								if e == nil {
									m = next
									placed = true
									break
								}
							}
							if !placed {
								t.Fatal("no usable D position")
							}
							s = state(t, m)
						}
					}
					shot := plannedShot(t, s)
					before := s
					m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": shot.Angle, "power": shot.Power, "nominate": shot.Nominate})
					flight := state(t, m).Flight
					ctx.Now = time.UnixMilli(m.SystemDeadline)
					m = finishStroke(t, mod, gameapi.Context{Players: n, Now: ctx.Now.Add(-time.Millisecond)}, m)
					after := state(t, m)
					records = append(records, map[string]any{"beforeBalls": before.Balls, "turn": before.Turn, "target": before.Target, "shot": shot, "first": flight.Motion.First, "potted": flight.Motion.Potted, "scores": after.Scores, "nextPhase": after.Phase, "nextTurn": after.Turn})
					shots++
					if shots%20 == 0 {
						t.Logf("%d strokes, scores %v, target %s %d", shots, after.Scores, after.Target, after.Clear)
					}
				case "foul":
					m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "accept"})
				case "frame_end":
					for p := 0; p < n; p++ {
						ctx.Actor = p
						m = action(t, mod, ctx, m, map[string]any{"type": "ready"})
					}
				case "black_choice":
					m = action(t, mod, ctx, m, map[string]any{"type": "black_choice", "team": 0})
				case "black_player":
					m = action(t, mod, ctx, m, map[string]any{"type": "black_player", "seat": s.Turn})
				default:
					t.Fatal("unexpected phase", s.Phase)
				}
			}
			if m.Result == nil {
				trace(t, records)
				t.Fatal("natural match stalled", shots, state(t, m).Target)
			}
			final := state(t, m)
			if len(final.Results) != bestOf || final.Results[len(final.Results)-1].Reason != "black" {
				t.Fatal("finished without playing through to the black", final.Results)
			}
			for _, b := range final.Balls {
				if b.ID > 0 && b.ID < 21 && !b.Down {
					t.Fatal("object ball left before black finish", b)
				}
			}
			records = append(records, map[string]any{"shots": shots, "final": final, "result": m.Result})
			trace(t, records)
		})
	}
}

func mustAction(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
