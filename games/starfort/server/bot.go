package starfort

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Bots use public information only. A round-robin actor cursor means one bot
// cannot monopolize the host's one-action scheduling tick.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	p := ctx.Actor
	if p < 0 || p >= len(s.Players) || s.finished() {
		return nil, errors.New("当前不能托管")
	}
	a := Action{Epoch: s.Epoch}
	if s.Phase == "paused" {
		for _, v := range s.Players {
			if v.ResumeVote && !s.Players[p].ResumeVote {
				a.Type = "resume"
			}
		}
		return marshalBotAction(a)
	}
	if s.Phase == "prepare" {
		for _, v := range s.Players {
			if v.PauseVote {
				if !s.Players[p].PauseVote {
					a.Type = "pause"
				}
				return marshalBotAction(a)
			}
		}
	}
	if investment := s.investment(p); investment != nil {
		investment.Epoch = s.Epoch
		return json.Marshal(investment)
	}
	if s.Phase == "prepare" && !s.Players[p].Ready {
		a.Type = "ready"
	}
	return marshalBotAction(a)
}
func (s *State) investment(p int) *Action {
	owned := []Tower{}
	counts := map[string]int{}
	occupied := map[int]bool{}
	for _, t := range s.Towers {
		occupied[t.Pad] = true
		counts[t.Kind]++
		if t.Owner == p {
			owned = append(owned, t)
		}
	}
	// First establish a mixed central defense; later upgrades beat buying a
	// distant low-value tower. Shared frost support is limited to avoid a team
	// investing all money in slowing enemies without doing enough damage.
	desired := max(2, 10/len(s.Players))
	if len(owned) < desired && len(s.Towers) < 12 {
		kind := "arrow"
		if counts["cannon"] == 0 && len(s.Towers) > 0 {
			kind = "cannon"
		} else if counts["arcane"] == 0 && len(s.Towers) > 1 {
			kind = "arcane"
		} else if counts["frost"] == 0 && len(s.Towers) > 2 {
			kind = "frost"
		} else {
			kind = []string{"arcane", "arrow", "cannon"}[len(s.Towers)%3]
		}
		c, _ := spec(kind)
		if s.Players[p].Gold >= c.Cost {
			for _, pad := range []int{1, 5, 3, 0, 2, 4, 6, 7, 10, 14, 8, 9, 11, 12, 13, 15} {
				if !occupied[pad] {
					return &Action{Type: "build", Pad: pad, Tower: kind}
				}
			}
		}
	}
	best := -1
	value := -1.
	for i, t := range owned {
		cost := upgradeCost(t)
		if cost > s.Players[p].Gold {
			continue
		}
		damage, _, interval := stats(t)
		score := (damage / float64(interval)) / float64(cost) / math.Sqrt(float64(t.Level))
		if t.Kind == "frost" {
			score *= .45
		}
		if score > value {
			value = score
			best = i
		}
	}
	if best >= 0 {
		t := owned[best]
		a := &Action{Type: "upgrade", TowerID: t.ID}
		if t.Level == 3 {
			a.Branch = "power"
			if t.Kind == "frost" || t.Kind == "cannon" {
				a.Branch = "speed"
			}
		}
		return a
	}
	return nil
}

// A nil action tells the host to try the next computer-controlled seat without
// storing a no-op snapshot or extending room activity.
func marshalBotAction(a Action) (json.RawMessage, error) {
	if a.Type == "" {
		return nil, nil
	}
	return json.Marshal(a)
}
