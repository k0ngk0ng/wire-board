package game

import "errors"

// The 2025 Fishing + Cities & Knights combination uses the Fishing board,
// ordinary C&K setup, and C&K's thirteen-point target (+1 for the old boot).
// Public rooms support three to six players; extended games use paired turns.
func NewCatanFishingCitiesKnights(n int, options CatanOptions) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("渔夫、助手和城市骑士三重组合尚未接入")
	}
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
