package game

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"math"
	"math/bits"
	"math/rand"
	"slices"
)

// Copy only public board information. Pending enemy orders never enter the
// evaluator, its random seed or its candidate generation.
func (s *State) dotaPublicCopy() *State {
	g := *s.Dota
	g.Players = slices.Clone(g.Players)
	for i := range g.Players {
		g.Players[i].Gear = slices.Clone(g.Players[i].Gear)
	}
	g.Towers = [2][]int{slices.Clone(g.Towers[0]), slices.Clone(g.Towers[1])}
	g.Track = slices.Clone(g.Track)
	g.Pending = nil
	g.History = nil
	return &State{Kind: "dota", Phase: s.Phase, Round: s.Round, Turn: s.Turn, Dota: &g}
}
func (s *State) dotaBot(player int) (Action, error) {
	if !slices.Contains(s.DotaActors(), player) {
		return Action{}, errors.New("当前无需电脑行动")
	}
	g := s.Dota
	a := Action{Prompt: g.Sequence}
	switch s.Phase {
	case "dota_teams":
		a.Type = "dota_confirm"
	case "dota_draft":
		available := []string{}
		for _, h := range dotaCatalog.Heroes {
			used := false
			for _, p := range g.Players {
				used = used || p.Hero == h.ID
			}
			if !used {
				available = append(available, h.ID)
			}
		}
		shuffle(available)
		a.Type = "dota_pick"
		a.Choice = available[0]
	case "dota_plan":
		view := s.dotaPublicCopy()
		teamOrders := map[int]DotaOrder{}
		for i, p := range g.Pending {
			if p != nil && g.Players[i].Team == g.Players[player].Team {
				teamOrders[i] = *p
			}
		}
		data, _ := json.Marshal(struct {
			Board  *State
			Player int
			Team   map[int]DotaOrder
		}{view, player, teamOrders})
		h := fnv.New64a()
		_, _ = h.Write(data)
		rng := rand.New(rand.NewSource(int64(h.Sum64())))
		order := view.dotaSearch(player, rng, teamOrders)
		a.Type = "dota_plan"
		a.Dota = &order
	default:
		return Action{}, errors.New("无效电脑阶段")
	}
	return a, nil
}
func (s *State) dotaSearch(player int, rng *rand.Rand, teamOrders map[int]DotaOrder) DotaOrder {
	g := s.Dota
	team := g.Players[player].Team
	guesses := [][]DotaOrder{}
	for _, policy := range []string{"rush", "hunt", "economy", "heuristic"} {
		orders := make([]DotaOrder, len(g.Players))
		for i, p := range g.Players {
			if a, ok := teamOrders[i]; ok {
				orders[i] = a
				continue
			}
			style := policy
			if p.Team == team {
				style = "heuristic"
			}
			orders[i] = g.heuristic(i, style, rng)
		}
		guesses = append(guesses, orders)
	}
	best := math.Inf(-1)
	chosen := DotaOrder{}
	for _, a := range g.legal(player, true) {
		total, worst := 0.0, math.Inf(1)
		for _, guess := range guesses {
			orders := slices.Clone(guess)
			orders[player] = a
			trial := s.dotaPublicCopy()
			trial.dotaResolve(orders, false)
			v := trial.dotaValue(team)
			total += v
			worst = min(worst, v)
		}
		score := total/float64(len(guesses))*.8 + worst*.2 + rng.Float64() - .5
		if score > best {
			best = score
			chosen = a
		}
	}
	return chosen
}
func (s *State) dotaValue(team int) float64 {
	g := s.Dota
	if s.Finished {
		if slices.Contains(s.Winners, g.seats(team)[0]) {
			return 1000
		}
		return -1000
	}
	total := 0.0
	for side := 0; side < 2; side++ {
		score := float64(g.Core[side]*14 + sum(g.Towers[side])*6)
		for _, h := range g.Players {
			if h.Team != side {
				continue
			}
			score += float64(h.HP)/float64(h.maximum())*float64(dotaHero(h.Hero).HP)*.6 + float64(h.Gold)*.6 + float64(h.Charge)*.3
			for _, id := range h.Gear {
				score += float64(dotaItem(id).Price) * .85
			}
			score -= float64(bits.OnesCount(uint(h.Spent))) * .25
			score += .8 * float64(dotaBool(h.Lane >= 0))
		}
		direction := 1
		if side == 1 {
			direction = -1
		}
		score += float64(sum(g.Track) * direction)
		if side == team {
			total += score
		} else {
			total -= score
		}
	}
	return total
}
func (g *Dota) heuristic(player int, policy string, rng *rand.Rand) DotaOrder {
	h := g.Players[player]
	friends, foes := []DotaPlayer{}, []DotaPlayer{}
	for _, p := range g.Players {
		if p.Lane == h.Lane {
			if p.Team == h.Team {
				friends = append(friends, p)
			} else if p.Lane >= 0 {
				foes = append(foes, p)
			}
		}
	}
	hurt := float64(h.maximum() - h.HP)
	attackWeight, pushWeight := 1.0, 1.0
	if policy == "hunt" {
		attackWeight = 1.55
		pushWeight = .35
	}
	if policy == "rush" {
		attackWeight = .35
		pushWeight = 1.6
	}
	best := math.Inf(-1)
	chosen := DotaOrder{}
	for _, a := range g.legal(player, true) {
		score := 0.0
		switch a.Card {
		case 4:
			score = -1.6 + .8*float64(bits.OnesCount(uint(h.Spent))) + .4*hurt
			if h.HP <= 3 && len(foes) > 0 {
				score += 3
			}
			if a.Buy != "" {
				item := dotaItem(a.Buy)
				pref := "attack"
				if slices.Contains([]string{"lina", "crystal_maiden", "earthshaker"}, h.Hero) {
					pref = "spell"
				}
				if h.Hero == "axe" {
					pref = "tank"
				}
				if policy == "rush" {
					pref = "push"
				}
				if policy == "hunt" {
					pref = "attack"
				}
				score += 1.1 + float64(dotaBool(item.Role == pref)) + .35*float64(dotaBool(item.Price >= 6)) - 1.5*float64(dotaBool(len(h.Gear) >= 2))
			}
			if h.Spent == 0 && hurt == 0 {
				score -= 3
			}
		case 0:
			enemy, friendly := 0, 0
			for i, x := range g.Players {
				if x.Lane == a.Lane {
					if x.Team != h.Team {
						enemy++
					} else if i != player {
						friendly++
					}
				}
			}
			base := 2.7
			if h.Lane == a.Lane {
				base = 3.5
			}
			score = pushWeight*base + 1.3*float64(dotaBool(enemy == 0)) - .8*float64(friendly) + 1.1*float64(dotaBool(g.Towers[1-h.Team][a.Lane] == 0)) + .8*float64(dotaBool(g.Towers[h.Team][a.Lane] == 0 && enemy > 0))
			if h.Lane < 0 {
				score += 6
			}
			if h.HP <= 3 && enemy > 0 {
				score -= 2
			}
		case 3:
			score = 1.9
			if policy == "economy" {
				score = 3.6
			}
			if h.Gold >= 10 {
				score *= .3
			}
			score -= .4 * float64(len(foes))
		case 2:
			score = pushWeight*1.8 + .8*float64(len(foes)) + .25*hurt
			if h.has("blade_mail") && len(foes) > 0 {
				score += 1.3
			}
		default:
			switch {
			case h.Hero == "axe" && a.Card == 5:
				score = 2*pushWeight + 1.1*float64(len(foes)) + .4*hurt
			case h.Hero == "juggernaut" && a.Card == 6:
				score = pushWeight
				for _, f := range friends {
					score += float64(min(3, dotaHero(f.Hero).HP-f.HP)) * .8
				}
			case h.Hero == "sven" && a.Card == 6:
				score = pushWeight*2 + .6*float64(len(foes)*len(friends))
			case h.Hero == "drow" && a.Card == 6:
				score = pushWeight*2 + .8*float64(len(foes))
			default:
				score = pushWeight
				if a.Card == 6 {
					score *= .6
				}
				if a.Target >= 0 {
					target := g.Players[a.Target]
					damage := 3
					if a.Card == 7 {
						damage = 6
					}
					score += attackWeight * (float64(damage)*.6 + float64(max(0, damage+1-target.HP))*.65)
					if target.HP <= 3 && bits.OnesCount(uint(target.Spent)) >= 3 {
						score -= 1.2
					}
				}
				if slices.Contains([]string{"lina", "crystal_maiden", "juggernaut", "earthshaker"}, h.Hero) && a.Card >= 5 {
					score += .8 * float64(max(0, len(foes)-1))
				}
			}
		}
		score += rng.Float64()*1.6 - .8
		if score > best {
			best = score
			chosen = a
		}
	}
	return chosen
}
