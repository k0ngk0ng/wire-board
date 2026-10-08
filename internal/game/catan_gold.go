package game

import "errors"

type CatanGoldClaim struct {
	Player int `json:"player"`
	Count  int `json:"count"`
}
type CatanGoldPending struct {
	AfterRoute *CatanRouteCompletion `json:"afterRoute,omitempty"`
	Claims     []CatanGoldClaim      `json:"claims"`
	Resume     string                `json:"resume"`
	// Non-nil only for production: Hilda is checked after all gold choices.
	Received []int `json:"received,omitempty"`
}

// Gold choices are serialized clockwise from the active seat. Ordinary
// production is distributed first, so every choice sees the remaining bank.
func (s *State) catanStartGold(due, received []int, resume string) {
	g := s.Catan
	q := &CatanGoldPending{Resume: resume, Received: received}
	for step := 0; step < len(g.Players); step++ {
		player := (s.Turn + step) % len(g.Players)
		if player < len(due) && due[player] > 0 && !g.Players[player].Eliminated {
			q.Claims = append(q.Claims, CatanGoldClaim{Player: player, Count: due[player]})
		}
	}
	g.GoldPending = q
	s.catanContinueGold()
}
func (s *State) catanContinueGold() {
	g := s.Catan
	q := g.GoldPending
	for len(q.Claims) > 0 {
		claim := q.Claims[0]
		if !g.Players[claim.Player].Eliminated && sum(g.Bank[:5]) > 0 {
			s.Phase = "catan_gold"
			return
		}
		if !g.Players[claim.Player].Eliminated {
			s.catanLog(claim.Player, "金矿触发，但银行没有剩余资源")
		}
		q.Claims = q.Claims[1:]
	}
	g.GoldPending = nil
	s.Phase = q.Resume
	if q.AfterRoute != nil {
		s.catanFinishRoute(*q.AfterRoute)
	}
	if q.Received != nil {
		s.catanAfterProduction(q.Received)
	}
}
func (s *State) catanChooseGold(player int, a Action) error {
	g := s.Catan
	q := g.GoldPending
	if q == nil || s.Phase != "catan_gold" || len(q.Claims) == 0 || q.Claims[0].Player != player || a.Type != "catan_gold" {
		return errors.New("请等待对应玩家选择金矿资源")
	}
	count := min(q.Claims[0].Count, sum(g.Bank[:5]))
	if !catanBundle(a.Take) || sum(a.Take) != count || !catanHas(g.Bank, a.Take) {
		return errors.New("请选择金矿应得数量的资源，且不能超出银行库存")
	}
	catanMove(g.Bank, g.Players[player].Resources, a.Take)
	if q.Received != nil {
		q.Received[player] += count
	}
	s.catanLog(player, "通过金矿领取 %s", catanText(a.Take))
	q.Claims = q.Claims[1:]
	s.catanContinueGold()
	return nil
}
func (s *State) catanAfterProduction(received []int) {
	g := s.Catan
	s.Phase = "catan_turn"
	if g.CitiesKnights != nil {
		// Cloth is awarded during production, including rolls without an
		// aqueduct response. Publish its points before returning to the turn.
		if g.cloth() != nil {
			s.catanScores()
		}
		s.catanQueueCityHelperProduction(received)
		s.catanStartAqueduct(received)
		if g.cloth() != nil && g.CitiesKnights.Pending == nil {
			s.catanVictory()
		}
		s.catanFinishCityHelperProduction()
		return
	}
	if g.cloth() != nil {
		s.catanScores()
		s.catanVictory()
		if s.Finished {
			return
		}
	}
	if g.Options.Helpers && sum(g.Bank[:5]) > 0 {
		for i, count := range received {
			if count == 0 && g.helperReady(i, 3) {
				s.catanHelperAsk(CatanHelperPending{Player: i, Kind: "resource", Resume: "catan_turn", Optional: true})
				break
			}
		}
	}
}

// CatanPendingActor identifies required out-of-turn choices without replacing
// State.Turn, which remains the owner of this production/action phase.
func (s *State) CatanPendingActor() int {
	g := s.Catan
	if g == nil || s.Finished {
		return -1
	}
	if g.Explorer != nil {
		if g.HelperPending != nil {
			return g.HelperPending.Player
		}
		if f := g.Fishing; f != nil && f.Pending != nil && len(f.Tokens.Pending) > 0 {
			return f.Tokens.Pending[0].Player
		}
		if k := g.CitiesKnights; k != nil && k.Pending != nil && len(k.Pending.Players) > 0 {
			return k.Pending.Players[0]
		}
		switch s.Phase {
		case "catan_explorer_setup", "catan_explorer_pirate_place", "catan_explorer_pirate_steal", "catan_explorer_resolve", "catan_explorer_battle":
			return s.Turn
		}
		return -1
	}
	if g.Transport != nil && (s.Phase == "catan_transport_move" || s.Phase == "catan_transport_barbarian") {
		return s.Turn
	}
	if g.attackKnights() && g.Attack.City.Treason != nil {
		q := g.Attack.City.Treason
		if q.Placement != nil {
			return q.Actor
		}
		return q.Owner
	}
	if g.attackKnights() && g.Attack.City.Plan != nil {
		if q := g.Attack.City.Plan.Pending; q != nil {
			return q.Player
		}
		return g.Attack.City.Plan.Player
	}
	if g.Attack != nil && g.Attack.EndPlan != nil {
		return g.Attack.EndPlan.Player
	}
	if g.Attack != nil && g.Attack.Pending != nil {
		return g.Attack.Pending.Player
	}
	if g.Two != nil && (g.Two.Pending != nil || g.Two.Trade != nil) {
		return s.Turn
	}
	if g.Caravans != nil && g.Caravans.Pending != nil {
		return g.Caravans.Pending.actor()
	}
	if f := g.Fishing; f != nil && f.Pending != nil && len(f.Tokens.Pending) > 0 {
		return f.Tokens.Pending[0].Player
	}
	if q := g.CardEvent; q != nil && len(q.Players) > 0 {
		return q.Players[0]
	}
	if k := g.CitiesKnights; k != nil && k.Pending != nil && len(k.Pending.Players) > 0 {
		return k.Pending.Players[0]
	}
	if p := g.pirateIslands(); p != nil && p.Raid != nil && len(p.Raid.Rewards) > 0 {
		return p.Raid.Rewards[0]
	}
	if s.Phase == "catan_world_ports" || s.Phase == "catan_world_fish" {
		return s.Turn
	}
	if s.Phase == "catan_rivers_start" || s.Phase == "catan_cloth_steal" || s.Phase == "catan_cloth_start" || s.Phase == "catan_wonders_start" {
		return s.Turn
	}
	if t := g.tribe(); t != nil && t.Pending != nil {
		return t.Pending.Player
	}
	if g.HelperPending != nil {
		return g.HelperPending.Player
	}
	if g.GoldPending != nil && len(g.GoldPending.Claims) > 0 {
		return g.GoldPending.Claims[0].Player
	}
	return -1
}
func (s *State) catanGoldBot(player int) (Action, error) {
	g := s.Catan
	q := g.GoldPending
	if q == nil || len(q.Claims) == 0 || q.Claims[0].Player != player {
		return Action{}, errors.New("inactive gold choice seat")
	}
	return Action{Type: "catan_gold", Take: g.catanResourceChoiceBot(player, q.Claims[0].Count)}, nil
}

// Only the choosing player's hand and public bank affect resource selection.
func (g *Catan) catanResourceChoiceBot(player, count int) []int {
	take := make([]int, 5)
	bank := append([]int{}, g.Bank[:5]...)
	for range min(count, sum(bank)) {
		best, priority := -1, -999
		for color, n := range bank {
			if n <= 0 {
				continue
			}
			score := 10 - (g.Players[player].Resources[color]+take[color])*3
			if color == 3 || color == 4 {
				score++
			}
			if score > priority {
				best, priority = color, score
			}
		}
		take[best]++
		bank[best]--
	}
	return take
}
