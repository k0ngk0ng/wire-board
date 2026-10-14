package snooker

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"slices"
)

// Decisions use only public ball positions, bounded local physics probes and
// ordinary actions. The host owns scheduling, persistence and manual takeover.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if ctx.Actor < 0 || ctx.Actor >= s.Players || s.Phase == "rolling" || s.Phase == "finished" {
		return nil, nil
	}
	a := Action{Window: s.Window}
	if s.Phase == "frame_end" {
		if s.Ready[ctx.Actor] {
			return nil, nil
		}
		a.Type = "ready"
		return json.Marshal(a)
	}
	if ctx.Actor != s.Turn {
		return nil, nil
	}
	switch s.Phase {
	case "start_choice":
		a.Type = "break_choice"
		a.Team = s.Teams[ctx.Actor]
	case "black_choice":
		a.Type = "black_choice"
		a.Team = s.Teams[ctx.Actor]
	case "black_player":
		a.Type = "black_player"
		a.Seat = ctx.Actor
	case "foul":
		a.Type = "foul_choice"
		a.Choice = "accept"
	case "concede":
		a.Type = "concede_reply"
		a.Choice = "accept"
	case "aim":
		cue, _ := findBall(s.Balls, 0)
		if cue.Down {
			if !s.InHand {
				return nil, errors.New("白球不可摆放")
			}
			// Prefer an unblocked placement; if snookered across the D, any clear
			// legal placement permits a subsequent free-ball or cushion decision.
			nominate := 7
			if s.Target == "clear" {
				nominate = s.Clear
			}
			p, ok := dVisiblePosition(s.Balls, onBalls(s.context(), nominate))
			if !ok {
				for ix := 0; ix <= 40 && !ok; ix++ {
					for iy := -40; iy <= 40; iy++ {
						q := Vec{BaulkX - float64(ix)*DRadius/40, TableHeight/2 + float64(iy)*DRadius/40}
						if q.sub(Vec{BaulkX, TableHeight / 2}).length() <= DRadius && clearSpot(q, s.Balls, 0) {
							p = q
							ok = true
							break
						}
					}
				}
			}
			if !ok {
				return nil, errors.New("D 区没有合法白球位置")
			}
			a.Type = "place"
			a.X = p.X
			a.Y = p.Y
			break
		}
		shot, e := planShot(s)
		if e != nil {
			return nil, e
		}
		a.Type = "shoot"
		a.Angle = shot.Angle
		a.Power = shot.Power
		a.Top = shot.Top
		a.Side = shot.Side
		a.Nominate = shot.Nominate
		a.FreeBall = shot.FreeBall
	default:
		return nil, nil
	}
	return json.Marshal(a)
}

func reclaim(_ gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	// Snooker has no human shot deadline to renew. Never reset an active flight,
	// decision window, partner order or the table when control returns to a user.
	_, e := decode(m)
	return m, e
}

func botDistance(a, b Vec) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }
func botPathClear(a, b Vec, balls []Ball, ignore1, ignore2 int) bool {
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
		near := Vec{X: a.X + u*dx, Y: a.Y + u*dy}
		if botDistance(near, ball.Pos) < 2*BallRadius+.0001 {
			return false
		}
	}
	return true
}
func planShot(s *State) (Strike, error) {
	var cue Ball
	targets := []Ball{}
	for _, b := range s.Balls {
		if b.ID == 0 {
			cue = b
		}
		if b.Down || b.ID == 0 {
			continue
		}
		v := BallValue(b.ID)
		if s.Target == "red" && v == 1 || s.Target == "colour" && v >= 2 || s.Target == "clear" && v == s.Clear {
			targets = append(targets, b)
		}
	}
	if cue.Down || len(targets) == 0 {
		return Strike{}, errors.New("电脑没有可用白球或目标球")
	}
	pockets := []Vec{{X: 0, Y: 0}, {X: TableWidth / 2, Y: -.026}, {X: TableWidth, Y: 0}, {X: 0, Y: TableHeight}, {X: TableWidth / 2, Y: TableHeight + .026}, {X: TableWidth, Y: TableHeight}}
	candidates := []Strike{}
	for _, b := range targets {
		for _, p := range pockets {
			length := botDistance(b.Pos, p)
			ux, uy := (p.X-b.Pos.X)/length, (p.Y-b.Pos.Y)/length
			ghost := Vec{X: b.Pos.X - 2*BallRadius*ux, Y: b.Pos.Y - 2*BallRadius*uy}
			if ghost.X < BallRadius || ghost.X > TableWidth-BallRadius || ghost.Y < BallRadius || ghost.Y > TableHeight-BallRadius {
				continue
			}
			if !botPathClear(cue.Pos, ghost, s.Balls, 0, b.ID) || !botPathClear(b.Pos, p, s.Balls, b.ID, 0) {
				continue
			}
			travel := botDistance(cue.Pos, ghost)
			dot := (ghost.X-cue.Pos.X)*ux + (ghost.Y-cue.Pos.Y)*uy
			if dot <= travel*.3 {
				continue
			}
			power := math.Min(.7, math.Max(.10, math.Sqrt((travel+length)*.36)/4.2*1.7))
			for _, scale := range []float64{.85, 1, 1.18} {
				candidates = append(candidates, Strike{Angle: math.Atan2(ghost.Y-cue.Pos.Y, ghost.X-cue.Pos.X), Power: math.Min(.85, power*scale), Nominate: BallValue(b.ID)})
			}
		}
	}
	// A legal contact or safety is a fallback when no direct pot is available.
	for _, b := range targets {
		for _, power := range []float64{.24, .45} {
			candidates = append(candidates, Strike{Angle: math.Atan2(b.Pos.Y-cue.Pos.Y, b.Pos.X-cue.Pos.X), Power: power, Nominate: BallValue(b.ID)})
		}
	}
	best := Strike{}
	score := math.Inf(-1)
	context := s.context()
	evaluate := func(shot Strike) {
		motion, e := Simulate(s.Balls, shot)
		if e != nil {
			return
		}
		// A wrong first contact cannot win against a legal contact. Avoid the
		// referee's expensive Miss-route proof for every rejected candidate.
		allowed := onBalls(context, shot.Nominate)
		if shot.FreeBall > 0 {
			allowed = []int{shot.FreeBall}
		}
		contact := len(motion.First) > 0
		for _, id := range motion.First {
			if !slices.Contains(allowed, id) {
				contact = false
			}
		}
		touching := false
		for _, b := range s.Balls {
			if !b.Down && slices.Contains(allowed, b.ID) && b.Pos.sub(cue.Pos).length() <= 2*BallRadius+1e-6 {
				touching = true
			}
		}
		rule := Ruling{Foul: 7}
		if contact || touching {
			rule = Judge(context, shot, motion)
		}
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

	for _, shot := range candidates {
		evaluate(shot)
	}
	// Free-ball nomination is optional; include legal alternatives only when
	// an ordinary shot cannot score. Nomination still passes through Apply.
	if score < 100 && s.FreeBall {
		nominate := s.Clear
		if s.Target == "red" {
			nominate = 1
		} else if s.Target == "colour" {
			nominate = 7
		}
		on := onBalls(context, nominate)
		for _, b := range s.Balls {
			if b.Down || b.ID == 0 || slices.Contains(on, b.ID) {
				continue
			}
			for _, p := range pockets {
				delta := p.sub(b.Pos)
				length := delta.length()
				if length < 1e-9 {
					continue
				}
				ghost := b.Pos.sub(delta.mul(2 * BallRadius / length))
				if !botPathClear(cue.Pos, ghost, s.Balls, 0, b.ID) || !botPathClear(b.Pos, p, s.Balls, b.ID, 0) {
					continue
				}
				for _, power := range []float64{.24, .45} {
					evaluate(Strike{Angle: math.Atan2(ghost.Y-cue.Pos.Y, ghost.X-cue.Pos.X), Power: power, Nominate: nominate, FreeBall: b.ID})
				}
			}
			evaluate(Strike{Angle: math.Atan2(b.Pos.Y-cue.Pos.Y, b.Pos.X-cue.Pos.X), Power: .3, Nominate: nominate, FreeBall: b.ID})
		}
	}
	// Bounded cushion search for blocked tables. Screen geometric rays first
	// and simulate at most 128 escapes; it never changes the real table.
	if score < -1000 {
		tries := 0
		for i := 0; i < 360 && tries < 128; i++ {
			angle := float64(i) * math.Pi / 180
			id := rayFirst(s.Balls, angle)
			nominate := BallValue(id)
			legal := id > 0 && (s.Target == "red" && nominate == 1 || s.Target == "colour" && nominate >= 2 || s.Target == "clear" && nominate == s.Clear)
			if !legal {
				continue
			}
			for _, power := range []float64{.45, .75} {
				evaluate(Strike{Angle: angle, Power: power, Nominate: nominate})
				tries++
			}
			if score > -1000 {
				break
			}
		}
	}
	if best.Power == 0 {
		return Strike{}, errors.New("电脑无法计算击球")
	}
	return best, nil
}
