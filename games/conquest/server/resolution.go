package conquest

import (
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"sort"
	"time"
)

func (s *State) beginResolution(now time.Time) {
	s.refresh()
	s.Phase = "resolving"
	s.Epoch++
	s.Cursor = 0
	s.Pending = []queued{}
	s.PauseVotes = make([]bool, len(s.Players))
	for seat, orders := range s.Plans {
		if !s.Players[seat].Eliminated {
			for _, o := range orders {
				s.Pending = append(s.Pending, queued{seat, o})
			}
		}
		s.Players[seat].Deadline = 0
	}
	sort.SliceStable(s.Pending, func(i, j int) bool {
		a, b := s.Pending[i], s.Pending[j]
		ca, cb := category(a.Order), category(b.Order)
		if ca != cb {
			return ca < cb
		}
		if ca != 2 {
			return a.Seat < b.Seat
		}
		if a.Order.ID != b.Order.ID {
			return a.Order.ID < b.Order.ID
		}
		pa, pb := s.Players[a.Seat], s.Players[b.Seat]
		if pa.Territories != pb.Territories {
			return pa.Territories > pb.Territories
		}
		if pa.Troops != pb.Troops {
			return pa.Troops > pb.Troops
		}
		if pa.Gold != pb.Gold {
			return pa.Gold > pb.Gold
		}
		return a.Seat < b.Seat
	})
	s.nextOperation(now)
}
func (s *State) appendEvent(e Event) {
	s.NextEvent++
	e.ID = s.NextEvent
	s.Events = append(s.Events, e)
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
}
func (s *State) nextOperation(now time.Time) {
	for s.Cursor < len(s.Pending) {
		q := s.Pending[s.Cursor]
		if s.Players[q.Seat].Eliminated {
			s.Cursor++
			continue
		}
		if q.Order.Type == "move" {
			g, _ := mapFor(s.MapID)
			idx, ok := g.index[q.Order.From]
			target, valid := g.index[q.Order.To]
			if !ok || !valid || s.Provinces[idx].Owner != q.Seat || s.Provinces[idx].Troops == 0 {
				s.Cursor++
				continue
			}
			player := s.Players[q.Seat]
			allowed, sea := route(g, q.Order.From, q.Order.To)
			if !allowed || player.MP10 < 10 || (sea && player.Gold < int64(q.Order.Amount)*2) {
				s.Cursor++
				continue
			}
			before := s.Provinces[target]
			s.appendEvent(Event{Type: "move", From: q.Order.From, To: q.Order.To, Seat: q.Seat, Amount: min(q.Order.Amount, s.Provinces[idx].Troops), At: now.UnixMilli(), Until: now.UnixMilli() + moveDuration, Before: &before, Text: "部队行军"})
			s.ResolveAt = now.UnixMilli() + moveDuration
			return
		}
		s.execute(q, now, false)
		s.Cursor++
	}
	s.finishRound(now)
}
func onTimer(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Players != len(s.Players) {
		return m, errors.New("席位不匹配")
	}
	if s.Phase != "resolving" || c.Now.UnixMilli() < s.ResolveAt {
		return m, nil
	}
	if s.Cursor < len(s.Pending) {
		s.execute(s.Pending[s.Cursor], c.Now, true)
		if s.Phase != "finished" {
			s.Cursor++
		}
	}
	if s.Phase != "finished" {
		s.nextOperation(c.Now)
	}
	return s.envelope(), nil
}
func (s *State) execute(q queued, now time.Time, animated bool) {
	seat, o := q.Seat, q.Order
	p := &s.Players[seat]
	if p.Eliminated {
		return
	}
	g, _ := mapFor(s.MapID)
	i, valid := g.index[o.Province]
	if o.Type == "move" {
		i, valid = g.index[o.From]
	}
	if o.Type != "tax" && (!valid || s.Provinces[i].Owner != seat) {
		return
	}
	event := Event{Type: o.Type, Province: o.Province, From: o.From, To: o.To, Seat: seat, Amount: o.Amount, At: now.UnixMilli(), Until: now.UnixMilli(), Text: "指令已执行"}
	switch o.Type {
	case "tax":
		p.Tax = o.Value
		event.Text = fmt.Sprintf("税率调整为%d%%", o.Value)
	case "recruit":
		src := &s.Provinces[i]
		n := o.Amount
		if n <= 0 || p.MP10 < 10 || n > src.Population/2 || int64(n) > p.Gold || src.Troops+n > maxTroops {
			return
		}
		src.Troops += n
		src.Population -= n
		p.Gold -= int64(n)
		p.MP10 -= 10
		event.Amount = n
		event.Text = "招募完成"
	case "disband":
		src := &s.Provinces[i]
		n := min(o.Amount, src.Troops)
		if n <= 0 || p.MP10 < 1 {
			return
		}
		src.Troops -= n
		src.Population = min(maxCivil, src.Population+n)
		p.MP10--
		event.Amount = n
		event.Text = "部队解散并返回民间"
	case "build":
		cost, points, ok := buildCost(o.Building)
		if !ok || p.Gold < cost || p.MP10 < points {
			return
		}
		src := &s.Provinces[i]
		switch o.Building {
		case "fort":
			if src.Fort {
				return
			}
			src.Fort = true
			event.Text = "工事建成"
		case "tower":
			if src.Tower || !s.Options.Fog {
				return
			}
			src.Tower = true
			event.Text = "瞭望塔建成"
		case "town":
			if src.Town {
				return
			}
			src.Town = true
			event.Text = "城镇建成"
		}
		p.Gold -= cost
		p.MP10 -= points
	case "move":
		to, ok := g.index[o.To]
		allowed, sea := route(g, o.From, o.To)
		if !ok || !allowed || p.MP10 < 10 {
			return
		}
		src, dst := &s.Provinces[i], &s.Provinces[to]
		n := min(o.Amount, src.Troops)
		if sea && p.Gold < int64(o.Amount)*2 {
			return
		}
		if dst.Owner == seat {
			n = min(n, maxTroops-dst.Troops)
		}
		if n <= 0 {
			return
		}
		before := *dst
		event.Before = &before
		king := o.King && p.King == o.From
		p.MP10 -= 10
		if sea {
			p.Gold -= int64(n) * 2
		}
		src.Troops -= n
		event.Amount = n
		if dst.Owner == seat {
			dst.Troops += n
			if king {
				p.King = o.To
			}
			event.Text = "部队抵达"
		} else {
			oldOwner := dst.Owner
			bonus := float32(1)
			if dst.Fort {
				bonus += .8
			}
			if dst.Town {
				bonus += .5
			}
			if oldOwner >= 0 && s.Players[oldOwner].King == dst.ID {
				bonus += .3
			}
			defenseBase := math.Floor(float64(float32(dst.Troops)*bonus) + .5)
			attackBase := float32(n)
			if king {
				attackBase *= 1.3
			}
			a := float32(math.Pow(float64(attackBase), 1.019))
			d := float32(math.Pow(defenseBase, 1.019))
			if a > d {
				survivors := n
				if a != 0 {
					survivors = int(float32(n) - float32(n)*d/a)
				}
				if dst.Troops > 0 {
					dst.Fort = false
				}
				dst.Troops = max(0, survivors)
				dst.Owner = seat
				if king {
					p.King = o.To
				}
				event.Captured = true
				event.Text = "占领" + g.data.Provinces[to].Name
				s.note(event.Text)
				if oldOwner >= 0 && s.Players[oldOwner].King == o.To {
					s.captureKing(oldOwner, seat)
				}
			} else {
				survivors := dst.Troops
				if d != 0 {
					survivors = int(float32(dst.Troops) - float32(dst.Troops)*a/d)
				}
				dst.Troops = max(0, survivors)
				event.Text = "进攻受阻"
			}
		}
		after := *dst
		after.Mobile = after.Troops
		event.After = &after
		s.refresh()
		s.checkVictory()
	}
	if animated && len(s.Events) > 0 {
		previous := &s.Events[len(s.Events)-1]
		event.ID = previous.ID
		event.At = previous.At
		event.Until = previous.Until
		*previous = event
	} else {
		s.appendEvent(event)
	}
}
func (s *State) captureKing(loser, winner int) {
	p := &s.Players[loser]
	if p.Gold > 0 {
		s.Players[winner].Gold += p.Gold
		p.Gold = 0
	}
	indices := []int{}
	for i := range s.Provinces {
		v := &s.Provinces[i]
		if v.Owner == loser {
			v.Troops = v.Troops * 7 / 10
			indices = append(indices, i)
		}
	}
	keep := 0
	if len(indices) >= 3 {
		keep = max(1, len(indices)*18/100)
	}
	sort.SliceStable(indices, func(a, b int) bool {
		x, y := s.Provinces[indices[a]], s.Provinces[indices[b]]
		if x.Troops != y.Troops {
			return x.Troops > y.Troops
		}
		if x.Economy != y.Economy {
			return x.Economy > y.Economy
		}
		if x.Population != y.Population {
			return x.Population > y.Population
		}
		return indices[a] < indices[b]
	})
	for i, index := range indices {
		if i >= keep {
			s.Provinces[index].Owner = winner
		}
	}
	if keep == 0 {
		p.Eliminated = true
		p.Submitted = true
		p.King = ""
		p.Deadline = 0
		p.LostRound = s.Round
		s.Plans[loser] = nil
		s.note("君主被俘，该势力退出争霸。")
	} else {
		p.King = s.Provinces[indices[0]].ID
		s.note("君主被俘，余部退守并拥立继承者。")
	}
	s.refresh()
}
func (s *State) finishRound(now time.Time) {
	s.refresh()
	s.checkVictory()
	if s.Phase == "finished" {
		return
	}
	g, _ := mapFor(s.MapID)
	// The old engine applies bounded population growth, then 15% land-neighbour
	// migration from a shared population snapshot, economy, morale and finances.
	for i := range s.Provinces {
		p := &s.Provinces[i]
		rate := float32(1.08)
		if p.Owner >= 0 {
			rate = float32(1.01) + float32(s.Players[p.Owner].EconomyRate)*.001
		}
		town := float32(1)
		if p.Town {
			town = 1.25
		}
		p.Population = min(maxCivil, max(0, int(math.Floor(float64(float32(p.Population)*rate*town)+.5))))
	}
	populations := make([]int, len(s.Provinces))
	for i, p := range s.Provinces {
		populations[i] = p.Population
	}
	for i, p := range g.data.Provinces {
		if len(p.Neighbors) == 0 {
			continue
		}
		share := populations[i] * 15 / 100 / len(p.Neighbors)
		for _, neighbor := range p.Neighbors {
			j := g.index[neighbor]
			s.Provinces[j].Population = min(maxCivil, s.Provinces[j].Population+share)
			s.Provinces[i].Population = max(0, s.Provinces[i].Population-share)
		}
	}
	for i := range s.Provinces {
		p := &s.Provinces[i]
		ecoRate := 0
		if p.Owner >= 0 {
			ecoRate = s.Players[p.Owner].EconomyRate
		}
		rate := float32(.99)
		if p.Population > p.Economy {
			rate = float32(1.03) + float32(ecoRate-80)*.03/100
		}
		p.Economy = min(maxCivil, max(0, int(math.Floor(float64(float32(p.Economy)*rate)+.5))))
		if p.Owner >= 0 {
			owner := s.Players[p.Owner]
			change := int(math.Floor(float64(float32(p.Troops)*(float32(owner.MilitaryRate-80)*.5/100)) + .5))
			if owner.Gold < 0 {
				change = -p.Troops / 10
				if p.Troops > 0 && p.Troops < 10 {
					change = -1
				}
			}
			p.Troops = min(maxTroops, max(0, p.Troops+change))
			p.Population = min(maxCivil, max(0, p.Population-change))
		}
	}
	for i := range s.Players {
		p := &s.Players[i]
		if p.Eliminated {
			continue
		}
		delta := 10 - p.Tax
		if delta < 0 {
			delta *= 2
		}
		p.Morale = min(100, max(0, p.Morale+delta))
	}
	s.refresh()
	for i := range s.Players {
		p := &s.Players[i]
		if p.Eliminated {
			continue
		}
		p.Gold += p.Income - p.Upkeep - p.EconomySpending
		p.Gold = min(int64(1000000000000), max(int64(-1000000000000), p.Gold))
		if p.Gold < 0 {
			p.Morale = max(0, p.Morale-1)
		}
		p.MP10 = min(10000, p.MP10/2+int((2.2+math.Pow(float64(p.Territories), .63))*10))
		p.Submitted = false
		p.Deadline = now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		s.Plans[i] = []Order{}
		s.NextOrder[i] = 0
	}
	s.Round++
	s.Epoch++
	s.Phase = "planning"
	s.Pending = []queued{}
	s.Cursor = 0
	s.ResolveAt = 0
	s.PauseVotes = make([]bool, len(s.Players))
	s.ResumeVotes = make([]bool, len(s.Players))
	s.note(fmt.Sprintf("第%d回合：税收与军费已结算。", s.Round))
}
