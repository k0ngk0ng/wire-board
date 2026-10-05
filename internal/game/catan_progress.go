package game

import (
	"errors"
	"slices"
)

type CatanProgressRule struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Track   int    `json:"track"`
	Count   int    `json:"count"`
	Victory bool   `json:"victory"`
}

// 2025 rulebook pp.13–15. Repeated kinds represent interchangeable physical
// cards, and each kind's total is conserved across decks, hands and public VPs.
var catanProgressRules = []CatanProgressRule{
	{0, "Alchemy", 0, 2, false}, {1, "Crane", 0, 2, false}, {2, "Engineering", 0, 1, false}, {3, "Invention", 0, 2, false}, {4, "Irrigation", 0, 2, false}, {5, "Medicine", 0, 2, false}, {6, "Mining", 0, 2, false}, {7, "Road Building", 0, 2, false}, {8, "Smithing", 0, 2, false}, {9, "Printing", 0, 1, true},
	{10, "Commercial Harbor", 1, 2, false}, {11, "Guild Dues", 1, 2, false}, {12, "Merchant", 1, 6, false}, {13, "Merchant Fleet", 1, 2, false}, {14, "Resource Monopoly", 1, 4, false}, {15, "Trade Monopoly", 1, 2, false},
	{16, "Diplomacy", 2, 2, false}, {17, "Encouragement", 2, 2, false}, {18, "Espionage", 2, 3, false}, {19, "Intrigue", 2, 2, false}, {20, "Sabotage", 2, 2, false}, {21, "Taxation", 2, 2, false}, {22, "Treason", 2, 2, false}, {23, "Constitution", 2, 1, true}, {24, "Wedding", 2, 2, false},
}

func CatanProgressRules() []CatanProgressRule {
	return append([]CatanProgressRule{}, catanProgressRules...)
}
func (k *CatanCitiesKnights) initProgress() {
	for _, rule := range catanProgressRules {
		for range rule.Count {
			k.ProgressDecks[rule.Track] = append(k.ProgressDecks[rule.Track], rule.ID)
		}
	}
	for track := range 3 {
		shuffle(k.ProgressDecks[track])
	}
	for i := range k.Players {
		k.Players[i].Progress = []int{}
		k.Players[i].PublicProgress = []int{}
	}
}
func (s *State) catanDrawProgress(player, track int) {
	k := s.Catan.CitiesKnights
	deck := k.ProgressDecks[track]
	if len(deck) == 0 {
		s.catanLog(player, "%s进步牌堆已空，本次不抽牌", catanCityTracks[track])
		return
	}
	card := deck[len(deck)-1]
	k.ProgressDecks[track] = deck[:len(deck)-1]
	rule := catanProgressRules[card]
	p := &k.Players[player]
	if rule.Victory {
		p.PublicProgress = append(p.PublicProgress, card)
		p.ProgressPoints++
		s.catanLog(player, "抽到并立即公开胜利点牌「%s」，获得1分", rule.Name)
		s.catanScores()
		s.catanVictory()
	} else {
		p.Progress = append(p.Progress, card)
		s.catanLog(player, "抽取一张%s进步牌", catanCityTracks[track])
	}
	if !s.Finished && player != s.Turn && len(p.Progress) > 4 {
		k.Pending = &CatanCityPending{Kind: "progress_discard", Players: []int{player}}
		s.Phase = "catan_progress_discard"
	}
}
func (k *CatanCitiesKnights) returnProgress(cards []int) {
	for _, card := range cards {
		track := catanProgressRules[card].Track
		k.ProgressDecks[track] = append([]int{card}, k.ProgressDecks[track]...)
	}
}
func (s *State) catanDiscardProgress(player int, a Action) error {
	k := s.Catan.CitiesKnights
	if a.Type != "catan_progress_discard" || (s.Phase != "catan_progress_discard" && s.Phase != "catan_progress_end") {
		return errors.New("当前不需要弃置进步牌")
	}
	hand := append([]int{}, k.Players[player].Progress...)
	if len(a.Cards) != len(hand)-4 || len(a.Cards) == 0 {
		return errors.New("请选择超出四张上限的进步牌")
	}
	for _, card := range a.Cards {
		at := slices.Index(hand, card)
		if at < 0 {
			return errors.New("不能弃置不在手中的进步牌")
		}
		hand = slices.Delete(hand, at, at+1)
	}
	k.Players[player].Progress = hand
	k.returnProgress(a.Cards)
	s.catanLog(player, "将%d张进步牌放回对应牌堆底部", len(a.Cards))
	return nil
}
