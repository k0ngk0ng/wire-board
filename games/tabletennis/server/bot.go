package tabletennis

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"hash/fnv"
)

// Decisions consume exactly the public seat view. A bot never changes state,
// ball coordinates, contact windows or outcomes and submits ordinary swings.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	projected, e := project(m, ctx.Actor)
	if e != nil {
		return nil, e
	}
	v := projected.(View)
	if !v.Controls.CanSwing {
		return nil, nil
	}
	now := ctx.Now.UnixMilli()
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s/%d/%d/%d", v.Epoch, v.Rally, v.Shot, ctx.Actor)
	seed := h.Sum64()
	rnd := func(shift uint) float64 { return float64((seed>>shift)&1023) / 1023 }
	a := Action{Type: "swing", Epoch: v.Epoch, Rally: v.Rally, Shot: v.Shot, Seq: v.Controls.Seq + 1, Kind: "drive", Aim: (rnd(10)*2 - 1) * .85, Power: .5 + .4*rnd(20), At: now}
	if v.Phase == "serve" {
		if now < v.PhaseUntil+450 {
			return nil, nil
		}
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
		return nil, nil
	}
	contactAt := now
	if contactAt < f.ContactStart {
		contactAt = f.ContactStart
	}
	point, _ := f.at(contactAt)
	positionalError := map[string]float64{"easy": .19, "normal": .12, "hard": .06}[v.Options.Difficulty]
	a.RacketX = clamp(point.X*sign(v.Options.Teams[ctx.Actor]^v.Ends)/.9+(rnd(50)*2-1)*positionalError, -1, 1)
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
