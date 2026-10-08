package game

import (
	"errors"
	"slices"
)

type catanAttackEndPlan struct {
	ID     int               `json:"id"`
	Player int               `json:"player"`
	Moves  []catanAttackMove `json:"moves"`
}
type catanAttackMoveChoice struct {
	From     int   `json:"from"`
	Required bool  `json:"required"`
	Normal   []int `json:"normal"`
	Wheat    []int `json:"wheat"`
	Fish     []int `json:"fish,omitempty"`
}

func (s *State) catanAttackBeginEnd() error {
	s.catanVictory()
	if s.Finished {
		return nil
	}
	a := s.Catan.Attack
	for _, k := range a.Knights {
		if s.Catan.attackCanMoveKnight(k.Player, s.Turn) {
			a.EndPlan = &catanAttackEndPlan{ID: a.EndSequence + 1, Player: s.Turn, Moves: []catanAttackMove{}}
			s.Catan.Trade = nil
			s.Phase = "catan_attack_end"
			return nil
		}
	}
	// Other players' knights still fight even when the active player has none.
	return s.catanAttackEndStep(nil, func() int { return catanRandom(6) + 1 })
}

func (s *State) catanAttackPlanPreview() (*State, error) {
	next := clone(*s)
	if err := next.catanAttackMoveKnights(next.Catan.Attack.EndPlan.Moves, false); err != nil {
		return nil, err
	}
	return &next, nil
}
func (s *State) validateCatanAttackPlan() error {
	a := s.Catan.Attack
	q := a.EndPlan
	if (q != nil) != (s.Phase == "catan_attack_end") {
		return errors.New("骑士移动计划阶段无效")
	}
	if q == nil {
		return nil
	}
	if q.ID != a.EndSequence+1 || q.ID < 1 || q.Player != s.Turn || q.Player < 0 || q.Player >= len(s.Catan.Players) || s.Catan.Players[q.Player].Eliminated || s.Finished || a.Pending != nil || s.Catan.Trade != nil || len(q.Moves) > s.Catan.attackMoveLimit() {
		return errors.New("骑士移动计划回应者或序号无效")
	}
	_, err := s.catanAttackPlanPreview()
	return err
}
func (s *State) catanAttackEndChoice(player int, action Action) error {
	a := s.Catan.Attack
	q := a.EndPlan
	if q == nil || s.Phase != "catan_attack_end" || player != q.Player || player != s.Turn || action.Type != "catan_attack_move" || action.Prompt != q.ID || action.Skill != "" {
		return errors.New("当前不能提交这项骑士移动计划")
	}
	switch action.Choice {
	case "normal", "wheat", "fish":
		q.Moves = append(q.Moves, catanAttackMove{From: action.Edge, To: action.Target, Wheat: action.Choice == "wheat", Fish: action.Choice == "fish", Tokens: slices.Clone(action.Tokens)})
		_, err := s.catanAttackPlanPreview()
		return err
	case "undo":
		if len(q.Moves) == 0 {
			return errors.New("没有可撤销的骑士移动")
		}
		q.Moves = q.Moves[:len(q.Moves)-1]
		return nil
	case "confirm":
		moves := slices.Clone(q.Moves)
		a.EndPlan = nil
		s.Phase = "catan_turn"
		return s.catanAttackEndStep(moves, func() int { return catanRandom(6) + 1 })
	default:
		return errors.New("请选择骑士移动、撤销或确认")
	}
}

// Uses only public knights/map and the actor's own available wheat.
func (s *State) catanAttackPlanChoices(preview *State) ([]catanAttackMoveChoice, bool) {
	a, g := preview.Catan.Attack, preview.Catan
	moved := map[int]bool{}
	for _, m := range a.EndPlan.Moves {
		moved[m.From] = true
	}
	choices := []catanAttackMoveChoice{}
	ready := true
	for i, k := range a.Knights {
		original := s.Catan.Attack.Knights[i].Edge
		if !g.attackCanMoveKnight(k.Player, s.Turn) {
			continue
		}
		required := a.castleEdge(g, k.Edge)
		if required {
			ready = false
		}
		if moved[original] {
			continue
		}
		neutralMoved := false
		for _, m := range a.EndPlan.Moves {
			for _, old := range s.Catan.Attack.Knights {
				if old.Edge == m.From && old.Player == catanAttackNeutral {
					neutralMoved = true
				}
			}
		}
		if g.twoAttack() && neutralMoved && k.Player != catanAttackNeutral {
			continue
		}
		if g.twoAttack() && k.Player == catanAttackNeutral {
			waiting := false
			for _, own := range a.Knights {
				waiting = waiting || own.Player == s.Turn && a.castleEdge(g, own.Edge)
			}
			if waiting {
				continue
			}
		}
		choice := catanAttackMoveChoice{From: original, Required: required, Normal: []int{}, Wheat: []int{}}
		for edge := range a.knightDestinations(g, i, 3) {
			if edge != k.Edge {
				choice.Normal = append(choice.Normal, edge)
			}
		}
		if g.Players[s.Turn].Resources[3] > 0 {
			for edge := range a.knightDestinations(g, i, 5) {
				if edge != k.Edge {
					choice.Wheat = append(choice.Wheat, edge)
				}
			}
		}
		if g.fishingAttack() && k.Player == s.Turn && g.fishPayment(s.Turn, g.fishActionCost(s.Turn, "catan_fish_knight")) != nil {
			for edge := range a.knightDestinations(g, i, 5) {
				if edge != k.Edge {
					choice.Fish = append(choice.Fish, edge)
				}
			}
		}
		slices.Sort(choice.Fish)
		slices.Sort(choice.Normal)
		slices.Sort(choice.Wheat)
		choices = append(choices, choice)
	}
	return choices, ready
}
func (s *State) catanAttackPlanView(public map[string]any, player int) {
	q := s.Catan.Attack.EndPlan
	if q == nil {
		return
	}
	// Neither unfinished destinations nor wheat choices leave the actor's view.
	public["endPlan"] = map[string]any{"id": q.ID, "player": q.Player}
	if player != q.Player {
		return
	}
	preview, err := s.catanAttackPlanPreview()
	if err != nil {
		return
	}
	choices, ready := s.catanAttackPlanChoices(preview)
	public["endPlan"] = q
	public["moveChoices"] = choices
	public["previewKnights"] = preview.Catan.Attack.Knights
	public["previewWheat"] = preview.Catan.Players[player].Resources[3]
	public["canConfirm"] = ready
	if preview.Catan.fishingAttack() {
		public["previewFish"] = preview.Catan.Fishing.Tokens.Hands[player]
		v, _ := preview.Catan.Fishing.Tokens.view(player)
		public["fishTokens"] = v.Players[player].Tokens
		public["fishCost"] = preview.Catan.fishActionCost(player, "catan_fish_knight")
	}
}

func (s *State) catanAttackEndBot(player int) (Action, error) {
	if err := s.validateCatanAttack(); err != nil {
		return Action{}, err
	}
	q := s.Catan.Attack.EndPlan
	if q == nil || player != q.Player {
		return Action{}, errors.New("当前没有己方骑士移动阶段")
	}
	preview, err := s.catanAttackPlanPreview()
	if err != nil {
		return Action{}, err
	}
	choices, ready := s.catanAttackPlanChoices(preview)
	best := Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "confirm"}
	// Complete mandatory departures first. Backtrack destinations rather than
	// choosing an exit that traps the remaining castle knights.
	if !ready {
		if moves, ok := s.catanAttackDeparturePlan(choices); ok && len(moves) > 0 {
			m := moves[0]
			best.Edge, best.Target = m.From, m.To
			best.Choice = "normal"
			if m.Wheat {
				best.Choice = "wheat"
			}
			if m.Fish {
				best.Choice = "fish"
			}
			return best, nil
		}
		if len(q.Moves) > 0 {
			best.Choice = "undo"
			return best, nil
		}
		return Action{}, errors.New("当前骑士没有可完成的城堡撤离计划")
	}
	// Improve coastal pressure without spending wheat for an equal position.
	score := func(edge int) int {
		value := 0
		for _, tile := range preview.Catan.Edges[edge].Tiles {
			if count := preview.Catan.Attack.Barbarians[tile]; count > 0 {
				value += 10
				for _, k := range preview.Catan.Attack.Knights {
					if k.Player == player && slices.Contains(preview.Catan.Edges[k.Edge].Tiles, tile) {
						value += 4
					}
				}
			}
		}
		return value
	}
	gain := 0
	for _, c := range choices {
		for _, mode := range []struct {
			edges []int
			wheat bool
			fish  bool
		}{{c.Normal, false, false}, {c.Wheat, true, false}, {c.Fish, false, true}} {
			for _, edge := range mode.edges {
				improvement := score(edge) - score(c.From)
				if mode.wheat || mode.fish {
					improvement -= 8
				}
				if improvement > gain {
					gain = improvement
					best.Edge, best.Target = c.From, edge
					best.Choice = "normal"
					if mode.wheat {
						best.Choice = "wheat"
					}
					if mode.fish {
						best.Choice = "fish"
					}
				}
			}
		}
	}
	return best, nil
}

// At most six own knights move. Castle-only search normally finds an exit
// immediately; if all such paths fail, try moving a non-castle blocker first.
func (s *State) catanAttackDeparturePlan(choices []catanAttackMoveChoice) ([]catanAttackMove, bool) {
	for _, required := range []bool{true, false} {
		for _, c := range choices {
			if c.Required != required {
				continue
			}
			for _, mode := range []struct {
				edges []int
				wheat bool
				fish  bool
			}{{c.Normal, false, false}, {c.Wheat, true, false}, {c.Fish, false, true}} {
				for _, edge := range mode.edges {
					if (mode.wheat || mode.fish) && slices.Contains(c.Normal, edge) {
						continue
					}
					move := catanAttackMove{From: c.From, To: edge, Wheat: mode.wheat, Fish: mode.fish}
					next := clone(*s)
					next.Catan.Attack.EndPlan.Moves = append(next.Catan.Attack.EndPlan.Moves, move)
					p, err := next.catanAttackPlanPreview()
					if err != nil {
						continue
					}
					options, ready := next.catanAttackPlanChoices(p)
					if ready {
						return []catanAttackMove{move}, true
					}
					if following, ok := next.catanAttackDeparturePlan(options); ok {
						return append([]catanAttackMove{move}, following...), true
					}
				}
			}
		}
	}
	return nil, false
}
