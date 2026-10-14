package shengji

func orderedCards(ids []int, r Rules) []int {
	out := append([]int{}, ids...)
	sortHand(out, r)
	return out
}
func displayTrick(plays []Play, r Rules) []Play {
	out := make([]Play, len(plays))
	for i, p := range plays {
		out[i] = p
		out[i].Cards = orderedCards(p.Cards, r)
	}
	return out
}
func displayLastTrick(t *Trick, r Rules) *Trick {
	if t == nil {
		return nil
	}
	out := *t
	out.Plays = displayTrick(t.Plays, r)
	return &out
}

func playSound(ids []int, leader bool, r Rules) (string, int) {
	shape, e := leadShape(ids, r)
	if e != nil {
		return "follow", 0
	}
	if shape.Four {
		return "four_suits", 0
	}
	if len(shape.Parts) > 1 {
		if leader {
			return "throw", 0
		}
		return "follow", 0
	}
	kind := shape.Parts[0].Kind
	value := 0
	if kind == "single" || kind == "pair" {
		value = rank(ids[0])
	}
	return kind, value
}
