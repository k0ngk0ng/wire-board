package game

import "errors"

type CatanNumberToken struct {
	Tile int `json:"tile"`
	Slot int `json:"slot"` // 0 is the main disc; 1 is the fishing extra disc.
}
type CatanNumberChoice struct {
	CatanNumberToken
	Number int `json:"number"`
}
type CatanNumberSwap struct {
	Left   CatanNumberToken `json:"left"`
	Right  CatanNumberToken `json:"right"`
	Before [2]int           `json:"before"`
}

func inventionNumber(number int) bool {
	return number == 3 || number == 4 || number == 5 || number == 9 || number == 10 || number == 11
}
func (g *Catan) inventionNumbers() []CatanNumberChoice {
	result := []CatanNumberChoice{}
	for _, tile := range g.Tiles {
		if inventionNumber(tile.Number) {
			result = append(result, CatanNumberChoice{CatanNumberToken{tile.ID, 0}, tile.Number})
		}
	}
	if g.Fishing != nil {
		for _, extra := range g.Fishing.Map.ExtraNumbers {
			if inventionNumber(extra.Number) {
				result = append(result, CatanNumberChoice{CatanNumberToken{extra.Tile, 1}, extra.Number})
			}
		}
	}
	return result
}
func (g *Catan) numberToken(ref CatanNumberToken) *int {
	if ref.Tile < 0 || ref.Tile >= len(g.Tiles) {
		return nil
	}
	if ref.Slot == 0 {
		return &g.Tiles[ref.Tile].Number
	}
	if ref.Slot == 1 && g.Fishing != nil {
		for i := range g.Fishing.Map.ExtraNumbers {
			if g.Fishing.Map.ExtraNumbers[i].Tile == ref.Tile {
				return &g.Fishing.Map.ExtraNumbers[i].Number
			}
		}
	}
	return nil
}
func (s *State) catanInvention(a Action) error {
	g := s.Catan
	slots := []int{0, 0}
	if len(a.Tokens) > 0 {
		if len(a.Tokens) != 2 {
			return errors.New("请选择两枚数字圆片")
		}
		slots = a.Tokens
	}
	left, right := CatanNumberToken{a.Tile, slots[0]}, CatanNumberToken{a.Target, slots[1]}
	lp, rp := g.numberToken(left), g.numberToken(right)
	if left == right || lp == nil || rp == nil || !inventionNumber(*lp) || !inventionNumber(*rp) {
		return errors.New("请选择两枚不同的数字圆片，不能交换2、6、8、12")
	}
	before := [2]int{*lp, *rp}
	if g.Explorer != nil {
		if err := g.Explorer.Board.swapNumbers(g, a.Tile, a.Target); err != nil {
			return err
		}
	} else {
		if g.Fishing != nil && len(g.Fishing.Map.ExtraNumbers) > 0 {
			g.Fishing.Map.NumberSwaps = append(g.Fishing.Map.NumberSwaps, CatanNumberSwap{left, right, before})
		}
		*lp, *rp = before[1], before[0]
	}
	s.catanLog(s.Turn, "交换地块 #%d 的%d和地块 #%d 的%d，强盗位置不变", a.Tile+1, before[0], a.Target+1, before[1])
	return nil
}

// Rewind public Invention swaps before checking the fixed fishing recipe.
// Lake positions and the original extra-disc identities remain strict. This
// avoids freezing a legitimately movable disc or accepting arbitrary changes.
func (f catanFishingMap) numbersBeforeInvention(g *Catan) (map[CatanNumberToken]int, error) {
	numbers := map[CatanNumberToken]int{}
	for _, tile := range g.Tiles {
		numbers[CatanNumberToken{tile.ID, 0}] = tile.Number
	}
	for _, extra := range f.ExtraNumbers {
		numbers[CatanNumberToken{extra.Tile, 1}] = extra.Number
	}
	if len(f.NumberSwaps) > 0 && (g.CitiesKnights == nil || g.setup()) {
		return nil, errors.New("数字交换必须发生在城市骑士正式行动中")
	}
	for i := len(f.NumberSwaps) - 1; i >= 0; i-- {
		swap := f.NumberSwaps[i]
		l, lok := numbers[swap.Left]
		r, rok := numbers[swap.Right]
		if !lok || !rok || swap.Left == swap.Right || !inventionNumber(swap.Before[0]) || !inventionNumber(swap.Before[1]) || l != swap.Before[1] || r != swap.Before[0] {
			return nil, errors.New("捕鱼地图的发明交换记录与数字不符")
		}
		numbers[swap.Left], numbers[swap.Right] = swap.Before[0], swap.Before[1]
	}
	return numbers, nil
}
