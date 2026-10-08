package game

import (
	"errors"
	"slices"
)

type catanAttackCityMoveChoice struct {
	From     int   `json:"from"`
	Required bool  `json:"required"`
	Move     []int `json:"move"`
	Displace []int `json:"displace"`
	Fish     []int `json:"fish,omitempty"`
}

func (s *State) catanAttackCityMoveChoices(player int) []catanAttackCityMoveChoice {
	out := []catanAttackCityMoveChoice{}
	g := s.Catan
	if g == nil || !g.attackKnights() || s.Finished {
		return out
	}
	c := g.Attack.City
	q := c.Plan
	if q == nil || q.Player != player || q.Pending != nil || s.validateAttackCityPlan() != nil {
		return out
	}
	preview, err := g.attackCityFishPreview()
	if err != nil {
		return out
	}
	used := map[int]bool{}
	arrivals := map[int]bool{}
	displaced := false
	for _, o := range q.Orders {
		used[o.From] = true
		arrivals[o.To] = true
		displaced = displaced || o.Retreat >= 0
	}
	for _, k := range q.Before {
		if !g.attackCityCanMove(k.Owner, player) || used[k.Edge] || arrivals[k.Edge] {
			continue
		}
		i := c.at(k.Edge)
		if i < 0 || !g.attackCityCanMove(c.Knights[i].Owner, player) {
			continue
		}
		choice := catanAttackCityMoveChoice{From: k.Edge, Move: c.destinations(g, k.Edge), Displace: []int{}}
		if !displaced {
			choice.Displace = c.displacementTargets(g, k.Edge)
		}
		if preview.fishingAttack() && preview.fishPayment(player, preview.fishActionCost(player, "catan_fish_knight")) != nil {
			choice.Fish = c.fishDestinations(g, player, k.Edge)
		}
		choice.Required = g.Attack.castleEdge(g, k.Edge) && (len(choice.Move) > 0 || len(choice.Displace) > 0 || len(choice.Fish) > 0)
		out = append(out, choice)
	}
	return out
}
func (s *State) catanAttackCityPlanView(player int) map[string]any {
	g := s.Catan
	c := g.Attack.City
	q := c.Plan
	result := map[string]any{"rules": c.Rules, "knights": slices.Clone(c.Knights), "issued": c.Issued, "end": clone(c.End)}
	if end, ok := result["end"].(*catanAttackCityEnd); ok && end != nil {
		end.Orders = publicAttackCityOrders(end.Orders)
	}
	if player >= 0 && player < len(g.Players) && player == s.Turn && !s.Finished && !g.Players[player].Eliminated && s.Phase == "catan_turn" && q == nil && c.Treason == nil && g.CardEvent == nil && g.CitiesKnights.Pending == nil && g.CitiesKnights.Event == nil {
		recruit, activate, promote := []int{}, []int{}, []int{}
		if catanHas(g.Players[player].Resources, []int{0, 0, 1, 0, 1}) {
			recruit = c.recruitEdges(g, player)
		}
		for _, knight := range c.Knights {
			if knight.Owner != player {
				continue
			}
			if !knight.Active && catanHas(g.Players[player].Resources, []int{0, 0, 0, 1, 0}) {
				activate = append(activate, knight.Edge)
			}
			if g.attackCityCanPromote(player, knight) && catanHas(g.Players[player].Resources, []int{0, 0, 1, 0, 1}) {
				promote = append(promote, knight.Edge)
			}
		}
		result["choices"] = map[string]any{"recruit": recruit, "activate": activate, "promote": promote, "capture": g.Attack.captureTargets()}
	}
	if tq := c.Treason; tq != nil && !s.Finished {
		actor := g.knightResponseActor(tq.Owner, tq.Actor)
		if tq.Placement != nil {
			actor = tq.Actor
		}
		result["treason"] = map[string]any{"id": tq.ID, "actor": actor, "caster": tq.Actor, "owner": tq.Owner}
		if player == actor {
			choices := map[string]any{}
			if tq.Placement != nil {
				choices["ranks"] = slices.Clone(tq.Placement.Ranks)
				choices["edge"] = tq.Placement.Removed.Edge
			} else {
				edges := []int{}
				for _, n := range c.Knights {
					if n.Owner == tq.Owner && g.attackCityTreasonRemovable(tq.Owner, n.Edge) {
						edges = append(edges, n.Edge)
					}
				}
				choices["remove"] = edges
			}
			result["choices"] = choices
		}
	}
	if q == nil || s.Finished {
		return result
	}
	actor := q.Player
	if q.Pending != nil {
		actor = q.Pending.Player
	}
	result["plan"] = map[string]any{"id": q.ID, "player": q.Player, "actor": actor, "orders": publicAttackCityOrders(q.Orders), "awaitingRetreat": q.Pending != nil}
	if player != actor {
		return result
	}
	choices := map[string]any{}
	if q.Pending != nil {
		choices["retreat"] = slices.Clone(q.Pending.Targets)
	} else {
		moves := s.catanAttackCityMoveChoices(player)
		ready := true
		for _, m := range moves {
			ready = ready && !m.Required
		}
		if g.fishingAttack() {
			if preview, err := g.attackCityFishPreview(); err == nil {
				v, _ := preview.Fishing.Tokens.view(player)
				choices["fishTokens"] = v.Players[player].Tokens
				choices["fishCost"] = g.fishActionCost(player, "catan_fish_knight")
			}
		}
		choices["moves"], choices["canConfirm"], choices["canUndo"] = moves, ready, len(q.Orders) > q.Locked
	}
	result["choices"] = choices
	return result
}
func (s *State) catanAttackCityPlanBot(player int) (Action, error) {
	if err := s.validateAttackCityPlan(); err != nil {
		return Action{}, err
	}
	g := s.Catan
	if g == nil || !g.attackKnights() || g.Attack.City.Plan == nil || s.Finished {
		return Action{}, errors.New("没有待处理的道路骑士计划")
	}
	c := g.Attack.City
	q := c.Plan
	a := Action{Type: "catan_attack_city_move", Prompt: q.ID}
	if q.Pending != nil {
		if q.Pending.Player != player {
			return Action{}, errors.New("不是退让玩家")
		}
		if len(q.Pending.Targets) == 0 {
			return Action{}, errors.New("退让没有合法目标")
		}
		a.Choice = "retreat"
		a.Edge = q.Pending.Targets[0]
		best := -1
		for _, edge := range q.Pending.Targets {
			value := g.attackCityEdgeValue(player, edge)
			if value > best {
				a.Edge = edge
				best = value
			}
		}
		return a, nil
	}
	if q.Player != player {
		return Action{}, errors.New("不是移动玩家")
	}
	choices := s.catanAttackCityMoveChoices(player)
	for _, choice := range choices {
		// Standing active defenders keep fighting. Prioritize compulsory castle
		// departure; otherwise move only inactive knights toward useful coasts.
		i := c.at(choice.From)
		if i < 0 || !choice.Required && c.Knights[i].Active {
			continue
		}
		best := g.attackCityEdgeValue(player, choice.From)
		target := -1
		for _, edge := range choice.Move {
			value := g.attackCityEdgeValue(player, edge)
			if target < 0 && choice.Required || value > best {
				target = edge
				best = value
			}
		}
		mode := "move"
		for _, edge := range choice.Fish {
			value := g.attackCityEdgeValue(player, edge) - 8
			if target < 0 && choice.Required || value > best {
				target, best, mode = edge, value, "fish"
			}
		}
		if target >= 0 {
			a.Choice = mode
			if mode == "fish" {
				preview, err := g.attackCityFishPreview()
				if err != nil {
					return Action{}, err
				}
				a.Tokens = preview.fishPayment(player, g.fishActionCost(player, "catan_fish_knight"))
			}
			a.Edge = choice.From
			a.Target = target
			return a, nil
		}
		if choice.Required {
			a.Choice = "displace"
			a.Edge = choice.From
			a.Target = choice.Displace[0]
			return a, nil
		}
	}
	a.Choice = "confirm"
	return a, nil
}
func (g *Catan) attackCityEdgeValue(player, edge int) int {
	value := 0
	for _, tile := range g.Edges[edge].Tiles {
		if slices.Contains(g.Attack.Map.Coast, tile) {
			value += 10 + g.Attack.Barbarians[tile]*5 + g.attackTileInterest(player, tile)
		}
	}
	return value
}

func (s *State) catanAttackCityTreasonBot(player int) (Action, error) {
	if err := s.validateAttackCityTreason(); err != nil {
		return Action{}, err
	}
	g := s.Catan
	if g == nil || !g.attackKnights() || g.Attack.City.Treason == nil {
		return Action{}, errors.New("没有叛变回应")
	}
	c := g.Attack.City
	q := c.Treason
	a := Action{Type: "catan_attack_city_treason", Prompt: q.ID}
	if q.Placement == nil {
		if player != g.knightResponseActor(q.Owner, q.Actor) {
			return a, errors.New("不是移除玩家")
		}
		a.Choice = "remove"
		best := 1000
		found := false
		for _, n := range c.Knights {
			if n.Owner == q.Owner && n.Strength < best {
				best = n.Strength
				a.Edge = n.Edge
				found = true
			}
		}
		if !found {
			return a, errors.New("没有可移除骑士")
		}
	} else {
		if player != q.Actor {
			return a, errors.New("不是放置玩家")
		}
		a.Choice = "place"
		a.Edge = q.Placement.Removed.Edge
		a.Color = q.Placement.Ranks[len(q.Placement.Ranks)-1]
	}
	return a, nil
}
func (s *State) catanAttackCityBotChoices(player int) []botChoice {
	g := s.Catan
	c := g.Attack.City
	k := g.CitiesKnights
	out := []botChoice{}
	for _, edge := range c.recruitEdges(g, player) {
		out = append(out, botChoice{Action{Type: "catan_attack_knight_recruit", Edge: edge}, 510})
	}
	for _, n := range c.Knights {
		if n.Owner != player {
			continue
		}
		if !n.Active && !g.Attack.castleEdge(g, n.Edge) {
			out = append(out, botChoice{Action{Type: "catan_attack_knight_activate", Edge: n.Edge}, 770})
		}
		if n.Strength < 3 {
			out = append(out, botChoice{Action{Type: "catan_attack_knight_promote", Edge: n.Edge}, 530})
		}
	}
	for _, id := range k.Players[player].Progress {
		a := Action{Type: "catan_progress", Card: id}
		score := 850
		switch id {
		case 3:
			tiles := g.attackCityInventionTiles()
			if len(tiles) < 2 {
				continue
			}
			a.Tile, a.Target = tiles[0], tiles[1]
		case 8:
			for _, n := range c.Knights {
				if n.Owner == player && n.Strength < 3 && n.PromotedAt != k.ActionSerial && c.count(player, n.Strength+1) < 2 && (n.Strength < 2 || k.Players[player].Improvements[CatanPolitics] >= 3) {
					a.Targets = []int{n.Edge}
					break
				}
			}
			if len(a.Targets) == 0 {
				continue
			}
		case 17:
			found := false
			for _, n := range c.Knights {
				found = found || n.Owner == player && !n.Active
			}
			if !found {
				continue
			}
		case 19:
			targets := g.Attack.captureTargets()
			if len(targets) == 0 {
				continue
			}
			a.Tile = targets[0]
			for _, tile := range targets {
				if g.attackTileInterest(player, tile) > g.attackTileInterest(player, a.Tile) {
					a.Tile = tile
				}
			}
		case 21:
			a.Tile = 0
			value := -1
			for _, tile := range g.Tiles {
				v := 0
				seen := map[int]bool{}
				for _, vertex := range tile.Vertices {
					seat := g.Vertices[vertex].Owner
					if seat >= 0 && seat != player && !seen[seat] && (!g.Attack.conqueredBuilding(g, vertex) || g.attackCityMetropolis(vertex)) {
						seen[seat] = true
						v += sum(g.Players[seat].Resources)
					}
				}
				if v > value {
					a.Tile = tile.ID
					value = v
				}
			}
		case 22:
			a.Target = -1
			for _, n := range c.Knights {
				if n.Owner != player && g.attackCityKnightOwner(n.Owner) {
					a.Target = n.Owner
					break
				}
			}
			if a.Target == -1 {
				continue
			}
		default:
			continue
		}
		out = append(out, botChoice{a, score})
	}
	return out
}

func (g *Catan) attackCityCanPromote(player int, knight catanAttackCityKnight) bool {
	return player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && knight.Owner == player && knight.Strength < 3 && knight.PromotedAt != g.CitiesKnights.ActionSerial && g.Attack.City.count(player, knight.Strength+1) < 2 && (knight.Strength < 2 || g.CitiesKnights.Players[player].Improvements[CatanPolitics] >= 3)
}
