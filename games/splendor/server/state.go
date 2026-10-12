package splendor

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	Cards  []int  `json:"cards"`
	Choice string `json:"choice"`
	Tile   int    `json:"tile"`
	Target int    `json:"target"`
	Take   []int  `json:"take"`
	Type   string `json:"type"`
	Card   int    `json:"card"`
	Tier   int    `json:"tier"`
	Tokens []int  `json:"tokens"`
	Noble  int    `json:"noble"`
	Color  int    `json:"color"`
	Slot   int    `json:"slot"`
}
type State struct {
	Kind     string    `json:"kind"`
	Turn     int       `json:"turn"`
	Phase    string    `json:"phase"`
	Round    int       `json:"round"`
	Finished bool      `json:"finished"`
	Winners  []int     `json:"winners"`
	Log      []string  `json:"log"`
	Splendor *Splendor `json:"splendor,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "splendor" {
		return nil, errors.New("未知游戏")
	}
	if n < 2 || n > 4 {
		return nil, errors.New("璀璨宝石需要 2–4 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initSplendor(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	if player != s.Turn {
		return errors.New("还没有轮到你")
	}
	err := s.applySplendor(a)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	return err
}
func (s *State) View(player int) map[string]any {
	b, _ := json.Marshal(s)
	var v map[string]any
	_ = json.Unmarshal(b, &v)

	g := v["splendor"].(map[string]any)
	delete(g, "decks")
	if player != s.Turn {
		delete(g, "reserveChoice")
	}
	if s.Splendor.Options.TradingPosts {
		g["tradingPostRules"] = GemTradingPosts()
	}
	if player == s.Turn && !s.Finished {
		if s.Phase == "gem_post" {
			g["postChoices"] = s.gemPostOptions()
		}
		if s.Phase == "gem_stronghold" {
			g["strongholdActions"] = s.gemStrongholdActions()
		}
		if s.Phase == "gem_conquest" {
			g["conquestCard"] = s.gemConquestCard()
		}
	}
	remaining := make([]int, len(s.Splendor.Decks))
	for i, deck := range s.Splendor.Decks {
		remaining[i] = len(deck)
	}
	g["remaining"] = remaining
	if s.Splendor.Options.Cities {
		eligible := make([][]int, len(s.Splendor.Players))
		for i, p := range s.Splendor.Players {
			eligible[i] = s.Splendor.gemCitiesFor(p)
		}
		g["cityEligibility"] = eligible
	}
	if player == s.Turn && s.Phase == "gem_free_card" {
		g["freeCardChoices"] = s.gemFreeCards(s.Splendor.Effects[0].Tier)
	}
	for i, p := range g["players"].([]any) {
		if i != player && !s.Finished {
			m := p.(map[string]any)
			m["reservedCount"] = len(s.Splendor.Players[i].Reserved)
			delete(m, "reserved")
		}
	}
	return v
}
func shuffle[T any](v []T) {
	for i := len(v) - 1; i > 0; i-- {
		j, e := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if e != nil {
			panic(e)
		}
		v[i], v[int(j.Int64())] = v[int(j.Int64())], v[i]
	}
}
func sum(v []int) int {
	n := 0
	for _, x := range v {
		n += x
	}
	return n
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var c T; _ = json.Unmarshal(b, &c); return c }
