package game

import (
	"errors"
	"slices"
)

type catanAttackWonderLanding struct {
	ID     int      `json:"id"`
	Player int      `json:"player"`
	Dice   [][2]int `json:"dice"`
	Cursor int      `json:"cursor"`
}

func (g *Catan) wonderLandingChoices() ([]int, []int) {
	a := g.Attack
	q := a.WonderLanding
	if q == nil || q.Cursor < 0 || q.Cursor >= len(q.Dice) {
		return nil, nil
	}
	sources, targets := []int{}, []int{}
	high, low := 0, 4
	for _, id := range a.Map.Reserves {
		n := a.Barbarians[id]
		if n > high {
			high = n
			sources = nil
		}
		if n == high && n > 0 {
			sources = append(sources, id)
		}
	}
	total := q.Dice[q.Cursor][0] + q.Dice[q.Cursor][1]
	for _, id := range a.Map.Coast {
		if !a.Map.landingNumber(g, id, total) || a.Barbarians[id] >= 3 {
			continue
		}
		n := a.Barbarians[id]
		if n < low {
			low = n
			targets = nil
		}
		if n == low {
			targets = append(targets, id)
		}
	}
	return sources, targets
}

func (s *State) startWonderLandings(groups int, roll func() [2]int) error {
	g := s.Catan
	a := g.Attack
	if !g.attackWonders() || g.setup() || s.Phase != "catan_turn" || a.WonderLanding != nil || groups < 1 || groups > 2 {
		return errors.New("当前不能开始蛮族奇迹登陆")
	}
	dice := [][2]int{}
	for group := 0; group < groups; group++ {
		used := map[int]bool{}
		for len(used) < 3 {
			d := roll()
			n := d[0] + d[1]
			if d[0] < 1 || d[0] > 6 || d[1] < 1 || d[1] > 6 {
				return errors.New("登陆骰子无效")
			}
			if n == 7 || used[n] {
				continue
			}
			used[n] = true
			dice = append(dice, d)
		}
	}
	a.WonderLanding = &catanAttackWonderLanding{ID: a.Sequence + 1, Player: s.Turn, Dice: dice}
	g.Trade = nil
	s.Phase = "catan_attack_landing"
	return s.continueWonderLandings()
}

func (s *State) continueWonderLandings() error {
	g := s.Catan
	a := g.Attack
	q := a.WonderLanding
	for q != nil && q.Cursor < len(q.Dice) {
		if q.Cursor%3 == 0 {
			a.Sequence = q.ID + q.Cursor/3
			a.Landing = &catanAttackLandingRecord{ID: a.Sequence, Player: q.Player, Rolls: []catanAttackLandingRoll{}}
		}
		sources, targets := g.wonderLandingChoices()
		if len(sources) == 0 {
			s.Log = append(s.Log, "沙漠蛮族已全部调出，本次不再登陆")
			a.WonderLanding = nil
			s.Phase = "catan_turn"
			s.catanScores()
			s.catanVictory()
			return nil
		}
		if len(targets) == 0 {
			s.recordWonderLanding(-1, -1)
			continue
		}
		if len(sources) == 1 && len(targets) == 1 {
			s.recordWonderLanding(sources[0], targets[0])
			continue
		}
		return nil
	}
	a.WonderLanding = nil
	s.Phase = "catan_turn"
	s.catanScores()
	s.catanVictory()
	return nil
}

func (s *State) recordWonderLanding(source, target int) {
	a := s.Catan.Attack
	q := a.WonderLanding
	d := q.Dice[q.Cursor]
	tiles := []int{}
	if target >= 0 {
		a.Barbarians[source]--
		a.Barbarians[target]++
		tiles = append(tiles, target)
		s.catanLog(q.Player, "蛮族登陆 %d+%d：从沙漠 #%d 调出至地块 #%d（%d/3）", d[0], d[1], source+1, target+1, a.Barbarians[target])
	} else {
		s.catanLog(q.Player, "蛮族登陆 %d+%d：没有未被征服的对应地块，不调出蛮族", d[0], d[1])
	}
	a.Landing.Rolls = append(a.Landing.Rolls, catanAttackLandingRoll{Dice: d, Tiles: tiles})
	q.Cursor++
	s.catanScores()
}

func (s *State) catanWonderLandingChoice(player int, action Action) error {
	a := s.Catan.Attack
	q := a.WonderLanding
	if q == nil || s.Phase != "catan_attack_landing" || player != q.Player || player != s.Turn || action.Type != "catan_attack_landing" || action.Prompt != q.ID || action.Slot != q.Cursor {
		return errors.New("请由当前玩家完成最新登陆选择")
	}
	sources, targets := s.Catan.wonderLandingChoices()
	if !slices.Contains(sources, action.Tile) || !slices.Contains(targets, action.Target) {
		return errors.New("选择蛮族最多的沙漠及同点数蛮族最少的地块")
	}
	s.recordWonderLanding(action.Tile, action.Target)
	return s.continueWonderLandings()
}
func (s *State) catanWonderLandingBot(player int) (Action, error) {
	q := s.Catan.Attack.WonderLanding
	if q == nil || player != q.Player {
		return Action{}, errors.New("没有登陆回应")
	}
	sources, targets := s.Catan.wonderLandingChoices()
	if len(sources) == 0 || len(targets) == 0 {
		return Action{}, errors.New("登陆缺少选择")
	}
	return Action{Type: "catan_attack_landing", Prompt: q.ID, Slot: q.Cursor, Tile: sources[0], Target: targets[0]}, nil
}
func (s *State) validateWonderLanding() error {
	g := s.Catan
	a := g.Attack
	q := a.WonderLanding
	if q == nil {
		if s.Phase == "catan_attack_landing" {
			return errors.New("登陆阶段缺少骰子")
		}
		return nil
	}
	if !g.attackWonders() || s.Finished || g.setup() || s.Phase != "catan_attack_landing" || q.Player != s.Turn || q.Player < 0 || q.Player >= len(g.Players) || q.ID < 1 || len(q.Dice) != 3 && !(g.twoAttackSea() && len(q.Dice) == 6) || q.Cursor < 0 || q.Cursor >= len(q.Dice) || a.Pending != nil || a.EndPlan != nil || g.Trade != nil || g.Two != nil && (g.Two.Pending != nil || g.Two.Trade != nil) {
		return errors.New("蛮族奇迹登陆回应无效")
	}
	for group := 0; group < len(q.Dice); group += 3 {
		seen := map[int]bool{}
		for _, d := range q.Dice[group : group+3] {
			n := d[0] + d[1]
			if d[0] < 1 || d[0] > 6 || d[1] < 1 || d[1] > 6 || n == 7 || seen[n] {
				return errors.New("蛮族奇迹登陆骰子无效")
			}
			seen[n] = true
		}
	}
	if a.Landing == nil || a.Sequence != q.ID+q.Cursor/3 || a.Landing.ID != a.Sequence || len(a.Landing.Rolls) != q.Cursor%3 {
		return errors.New("蛮族奇迹登陆进度无效")
	}
	for i, r := range a.Landing.Rolls {
		if r.Dice != q.Dice[q.Cursor-q.Cursor%3+i] {
			return errors.New("登陆记录与已掷骰子不匹配")
		}
	}
	sources, targets := g.wonderLandingChoices()
	if len(sources) == 0 || len(targets) == 0 {
		return errors.New("登陆回应不能完成")
	}
	return nil
}
