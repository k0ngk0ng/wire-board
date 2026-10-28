package contra

import (
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func tickCount(s *State, at int64) int {
	elapsed := min(int64(250), max(int64(0), at-s.ServerAt))
	total := elapsed*60 + s.Remainder
	s.ServerAt = at
	s.Remainder = total % 1000
	return int(total / 1000)
}
func (h *host) timer(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
	s, e := h.decode(g)
	if e != nil {
		return g, e
	}
	if c.Actor != -1 || g.Paused {
		return g, errors.New("invalid rule timer")
	}
	auto, e := flags(c, s.Players)
	if e != nil {
		return g, e
	}
	if e = contextError(c, &s); e != nil {
		return g, e
	}
	if g.Result != nil {
		return g, nil
	}
	at := c.Now.UnixMilli()
	g.SystemDeadline = at + frameMS
	for i, a := range auto {
		if s.Inputs[i].Automated != a {
			clearInput(&s.Inputs[i])
			s.Inputs[i].Automated = a
		}
		if s.StageLoading != nil && a {
			s.StageLoading.Ready[i] = true
		}
		if s.StageLoading != nil && !s.StageLoading.Ready[i] {
			clearInput(&s.Inputs[i])
		}
		if !a && s.Inputs[i].ReceivedAt != 0 && at-s.Inputs[i].ReceivedAt >= leaseMS {
			clearInput(&s.Inputs[i])
		}
	}
	releaseStage(&s)
	if s.Session.World.Campaign.Outcome != engine.CampaignOngoing {
		if e = h.finish(&g, &s); e != nil {
			return g, e
		}
		return store(g, &s)
	}
	count := tickCount(&s, at)
	for tick := 0; tick < count; tick++ {
		var pads [2]engine.Buttons
		var observed engine.BotObservation
		for _, automated := range auto {
			if automated {
				visual, err := h.botVisualFrame(&s)
				if err != nil {
					return g, err
				}
				observed, e = h.observeBot(visual)
				if e != nil {
					return g, e
				}
				if observed.Actor != 0 || observed.Tick != visual.Tick() || observed.Seats != s.Players {
					return g, errors.New("shared bot observer identity differs from public visual frame")
				}
				break
			}
		}
		for i, a := range auto {
			input := &s.Inputs[i]
			if a {
				// Both seats see the same final pixels and HUD. Classification is
				// shared; controller memory and actor identity remain seat-local.
				ob := policyObservation(observed, i)
				pads[i] = s.Bots[i].Decide(ob)
				input.Held = pads[i]
				input.Applied = pads[i]
			} else {
				if len(input.Queue) > 0 {
					input.Applied = input.Queue[0]
					input.Queue = input.Queue[1:]
				}
				pads[i] = input.Applied
			}
		}
		level := s.Session.World.Active.Combat.World.Level
		r, e := h.bundle.Stage(level)
		if e != nil {
			return g, e
		}
		if _, e = s.Session.StepPads(s.Session.Random.Frames+1, pads, r); e != nil {
			return g, e
		}
		if e = h.record(&s, "pads", pads); e != nil {
			return g, e
		}
		if s.Session.World.Campaign.Outcome != engine.CampaignOngoing {
			if e = h.finish(&g, &s); e != nil {
				return g, e
			}
			break
		}
		if next := s.Session.World.Active.Combat.World.Level; next != level {
			s.StageLoading = &StageLoading{Level: next, Ready: append([]bool(nil), auto...), RuntimeBuildID: h.runtimeBuild}
			for i := range s.Inputs {
				clearInput(&s.Inputs[i])
			}
			s.Remainder = 0
			critical(&g, &s, "stage")
			break
		}
		if s.Session.World.Campaign.Paused {
			g.Paused = true
			s.Remainder = 0
			for i := range s.Inputs {
				s.Inputs[i].Queue = nil
			}
			critical(&g, &s, "paused")
			break
		}
	}
	return store(g, &s)
}
func (h *host) pausedTimer(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
	s, e := h.decode(g)
	if e != nil {
		return g, e
	}
	if !g.Paused || c.Actor != -1 {
		return g, errors.New("invalid paused timer")
	}
	if _, e = flags(c, s.Players); e != nil {
		return g, e
	}
	if e = contextError(c, &s); e != nil {
		return g, e
	}
	r, e := h.bundle.Stage(s.Session.World.Active.Combat.World.Level)
	if e != nil {
		return g, e
	}
	for n := tickCount(&s, c.Now.UnixMilli()); n > 0; n-- {
		if _, e = s.Session.PresentPaused(r); e != nil {
			return g, e
		}
		if e = h.record(&s, "present-paused", [2]engine.Buttons{}); e != nil {
			return g, e
		}
	}
	g.SystemDeadline = c.Now.UnixMilli() + frameMS
	return store(g, &s)
}
