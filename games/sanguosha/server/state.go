package sanguosha

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	Cards     []int  `json:"cards"`
	Targets   []int  `json:"targets"`
	Choice    string `json:"choice"`
	Skill     string `json:"skill"`
	Direction string `json:"direction,omitempty"`
	Prompt    int    `json:"prompt"`
	Target    int    `json:"target"`
	Take      []int  `json:"take"`
	Type      string `json:"type"`
	Card      int    `json:"card"`
	Color     int    `json:"color"`
	Slot      int    `json:"slot"`
}
type State struct {
	Kind      string     `json:"kind"`
	Turn      int        `json:"turn"`
	Phase     string     `json:"phase"`
	Round     int        `json:"round"`
	Finished  bool       `json:"finished"`
	Winners   []int      `json:"winners"`
	Log       []string   `json:"log"`
	Sanguosha *Sanguosha `json:"sanguosha,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "sanguosha" {
		return nil, errors.New("未知游戏")
	}
	if n < 4 || n > 8 {
		return nil, errors.New("三国杀需要 4–8 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initSanguosha(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	return s.applySanguosha(player, a)
}
func (s *State) View(player int) map[string]any {
	return s.sgView(player)
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

func sgRandom(n int) int {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		panic(e)
	}
	return int(v.Int64())
}
