package game

// The 2025 Fishing + Cities & Knights combination uses the Fishing board,
// ordinary C&K setup, and C&K's thirteen-point target (+1 for the old boot).
// Still internal: no room option exposes an unaccepted expansion combination.
func NewCatanFishingCitiesKnights(n int, options CatanOptions) (*State, error) {
	s, err := NewCatanFishing(n, options)
	if err != nil {
		return nil, err
	}
	logs := append([]string{}, s.Log...)
	s.enableCitiesKnights()
	s.Log = append(logs, s.Log...)
	s.Log = append(s.Log, "捕鱼＋城市与骑士：鱼不算资源或商品；7鱼选择一种进步牌堆抽牌；持旧靴子需14分")
	return s, nil
}
