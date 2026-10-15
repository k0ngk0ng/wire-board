package junqi

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
)

// No private state enters choose. Opponent identities and ranks may differ
// arbitrarily behind the same visible position without changing this policy.
func choose(v View, seat int) json.RawMessage {
	a := Action{Round: v.Round, Ply: v.Ply, Revision: v.Revision}
	if v.CanConfirm {
		yes := true
		a.Type, a.Ready = "deploy_ready", &yes
		out, _ := json.Marshal(a)
		return out
	}
	if v.CanVote {
		yes := true
		a.Type, a.OfferID, a.Accept = "vote_draw", v.Offer.ID, &yes
		out, _ := json.Marshal(a)
		return out
	}
	if !v.CanMove || len(v.Legal) == 0 {
		return nil
	}
	units := map[string]PieceView{}
	positions := map[int]PieceView{}
	for _, p := range v.Pieces {
		units[p.ID] = p
		positions[p.Pos] = p
	}
	threats := map[string]bool{}
	blocking := map[int]bool{}
	if v.Options.Difficulty == "standard" {
		// Unknown opponents are conservatively treated as ordinary mobile
		// pieces. This is a public-position hypothesis, never a private lookup.
		hypothesis := append([]PieceView(nil), v.Pieces...)
		flag := -1
		for i, p := range hypothesis {
			if p.Seat == seat && p.Kind == "flag" {
				flag = p.Pos
			}
			if p.Kind == "unknown" {
				hypothesis[i].Kind = "company"
			}
		}
		for enemy, player := range v.Players {
			if !player.Alive || allied(len(v.Players), seat, enemy) {
				continue
			}
			for _, danger := range legalMoves(v.Graph, hypothesis, enemy) {
				if danger.To != flag {
					continue
				}
				threats[danger.Unit] = true
				for _, pos := range danger.Path[1 : len(danger.Path)-1] {
					blocking[pos] = true
				}
			}
		}
	}
	targets := []int{}
	for _, p := range v.Pieces {
		if !allied(len(v.Players), seat, p.Seat) && p.Kind == "flag" {
			targets = append(targets, p.Pos)
		}
	}
	if len(targets) == 0 {
		for _, node := range v.Graph.Nodes {
			if node.HQ && v.Players[node.Seat].Alive && !allied(len(v.Players), seat, node.Seat) {
				targets = append(targets, node.ID)
			}
		}
	}
	distance := func(pos int) float64 {
		best := 1e6
		p := v.Graph.Nodes[pos]
		for _, to := range targets {
			q := v.Graph.Nodes[to]
			d := math.Abs(float64(p.X-q.X)) + math.Abs(float64(p.Y-q.Y))
			if d < best {
				best = d
			}
		}
		return best
	}
	best, value := v.Legal[0], -1e12
	for _, m := range v.Legal {
		p := units[m.Unit]
		from, to := v.Graph.Nodes[m.From], v.Graph.Nodes[m.To]
		score := (distance(m.From)-distance(m.To))*18 - distance(m.To)*3
		if to.HQ {
			score -= 60
		}
		if to.Camp {
			score += 3
		}
		if p.Kind == "engineer" {
			score += 5
		}
		if p.Kind == "commander" || p.Kind == "general" {
			score += 4
		}
		if target, found := positions[m.To]; found {
			if threats[target.ID] {
				score += 3000
			}
			score += 70
			if target.Kind == "flag" {
				score += 10000
			} else if target.Kind == "mine" {
				if p.Kind == "engineer" {
					score += 100
				} else if p.Kind != "bomb" {
					score -= 180
				}
			} else if target.Kind != "unknown" {
				if p.Kind == "bomb" {
					score += float64(ranks[target.Kind]) * 8
				} else if ranks[p.Kind] >= ranks[target.Kind] {
					score += 100
				} else {
					score -= 120
				}
			} else if to.Seat >= 0 && to.Row >= 4 && p.Kind != "engineer" && p.Kind != "bomb" {
				score -= 85
			}
			if p.Kind == "bomb" {
				score += 15
			}
		}
		if blocking[m.To] {
			score += 1500
		}
		// Recent public movement is enough to discourage immediate backtracking.
		for _, last := range v.Recent {
			if last.Unit == m.Unit && last.From == m.To {
				score -= 90
			}
		}
		if from.HQ {
			score = -1e10
		}
		if score > value {
			best, value = m, score
		}
	}
	a.Type, a.Unit, a.To = "move", best.Unit, &best.To
	out, _ := json.Marshal(a)
	return out
}
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	projected, e := view(m, c.Actor)
	if e != nil {
		return nil, e
	}
	v := projected.(View)
	if c.Actor < 0 || c.Actor >= len(v.Players) {
		return nil, nil
	}
	return choose(v, c.Actor), nil
}
