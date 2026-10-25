package tabletennis

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"hash/fnv"
	"math"
)

func botMove(v View, seat int, now int64, x, y, z float64) (json.RawMessage, error) {
	p := v.Paddles[seat]
	if now-p.MotionAt < 80 {
		return nil, nil
	}
	x = p.X + clamp(x-p.X, -.3, .3)
	y = p.Y + clamp(y-p.Y, -.18, .18)
	z = clamp(p.Z+clamp(z-p.Z, -.2, .2), 0, ReachDepth)
	if math.Abs(x-p.X)+math.Abs(y-p.Y)+math.Abs(z-p.Z) < .008 {
		return nil, nil
	}
	return json.Marshal(Action{Type: "move", Epoch: v.Epoch, Ends: v.Ends, MotionSeq: p.MotionSeq + 1, RacketX: x, RacketY: &y, RacketZ: z, At: now})
}

// Decisions consume exactly the public seat view. A bot never changes state,
// ball coordinates, contact windows or outcomes and submits ordinary swings.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	projected, e := project(m, ctx.Actor)
	if e != nil {
		return nil, e
	}
	v := projected.(View)
	if v.Phase == "finished" {
		return nil, nil
	}
	now := ctx.Now.UnixMilli()
	if !v.Controls.CanSwing {
		if v.Controls.Armed {
			return nil, nil
		}
		for i := len(v.Events) - 1; i >= 0; i-- {
			e := v.Events[i]
			age := now - e.At
			if e.Type == "hit" && e.Seat == ctx.Actor && age >= 0 && age < 320 {
				dir := sign(v.Options.Teams[ctx.Actor] ^ v.Ends)
				k := math.Sin(float64(age) / 320 * math.Pi)
				return botMove(v, ctx.Actor, now, e.P.X*dir/.9+.12*k, e.P.Y+.16*k, e.P.Z*dir-.18*k)
			}
		}
		return nil, nil
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s/%d/%d/%d", v.Epoch, v.Rally, v.Shot, ctx.Actor)
	seed := h.Sum64()
	rnd := func(shift uint) float64 { return float64((seed>>shift)&1023) / 1023 }
	a := Action{Type: "swing", Epoch: v.Epoch, Rally: v.Rally, Shot: v.Shot, Seq: v.Controls.Seq + 1, Kind: "drive", Aim: (rnd(10)*2 - 1) * .85, Power: .5 + .4*rnd(20), At: now}
	if v.Phase == "serve" {
		if now < v.PhaseUntil+450 {
			return botMove(v, ctx.Actor, now, .34/.9, 1.1, 1.28)
		}
		y := 1.1
		a.RacketX = .34 / .9
		a.RacketY = &y
		a.RacketZ = 1.28
		a.Power = .5
		if seed%3 == 0 {
			a.Kind = "chop"
		}
		return json.Marshal(a)
	}
	f := v.Flight
	if f == nil || f.ReceiveAt == 0 || f.ContactEnd == 0 || now > f.ContactEnd {
		return nil, nil
	}
	delay := int64(80)
	switch v.Options.Difficulty {
	case "easy":
		delay = 100 + int64(rnd(30)*200)
	case "normal":
		delay = 70 + int64(rnd(30)*160)
	case "hard":
		delay = 70 + int64(rnd(30)*100)
	}
	window := f.ContactEnd - f.ContactStart
	if v.Options.Difficulty != "easy" {
		fraction := .65
		if v.Options.Difficulty == "hard" {
			fraction = .4
		}
		if limit := int64(float64(window) * fraction); delay > limit {
			delay = limit
		}
		// The same short early-input buffer is available to a human. Use it
		// for fast incoming balls instead of granting a larger reach window.
		if window < 120 {
			delay = -60
		}
	}
	if now < f.ContactStart+delay || now < f.StartAt+180 {
		when := min(f.ContactEnd, max(f.ContactStart, f.ContactStart+delay))
		point, _ := f.at(when)
		dir := sign(v.Options.Teams[ctx.Actor] ^ v.Ends)
		windup := clamp(float64(when-now)/160, 0, 1)
		return botMove(v, ctx.Actor, now, point.X*dir/.9+.15*windup, point.Y+.08*windup, point.Z*dir+.1*windup)
	}
	contactAt := now
	if contactAt < f.ContactStart {
		contactAt = f.ContactStart
	}
	point, _ := f.at(contactAt)
	positionalError := map[string]float64{"easy": .19, "normal": .12, "hard": .06}[v.Options.Difficulty]
	a.RacketX = clamp(point.X*sign(v.Options.Teams[ctx.Actor]^v.Ends)/.9+(rnd(50)*2-1)*positionalError, -1, 1)
	y := point.Y + (rnd(45)*2-1)*positionalError*.4
	a.RacketY = &y
	a.RacketZ = point.Z * sign(v.Options.Teams[ctx.Actor]^v.Ends)
	a.Lift = .8
	if own := v.Paddles[ctx.Actor]; own.Result == "miss" && now-own.At < 120 {
		return nil, nil
	}
	switch seed % 7 {
	case 0, 1:
		a.Kind = "chop"
		a.Power = .35 + .45*rnd(20)
	case 2:
		a.Kind = "block"
		a.Power = .3 + .45*rnd(20)
	}
	// Pressure creates ordinary timing/placement errors, not manufactured points.
	if rnd(40) < map[string]float64{"easy": .28, "normal": .16, "hard": .09}[v.Options.Difficulty] {
		a.Aim = .94 + .06*rnd(10)
		if rnd(10) < .5 {
			a.Aim = -a.Aim
		}
		a.Power = 1
	}
	return json.Marshal(a)
}
