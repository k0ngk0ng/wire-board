package catan

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	SpiceLoad   []int  `json:"spiceLoad,omitempty"`
	SpiceUnload []int  `json:"spiceUnload,omitempty"`
	GoldGive    int    `json:"goldGive,omitempty"`
	GoldTake    int    `json:"goldTake,omitempty"`
	Cards       []int  `json:"cards"`
	Targets     []int  `json:"targets"`
	Choice      string `json:"choice"`
	Skill       string `json:"skill"`
	Prompt      int    `json:"prompt"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Vertex      int    `json:"vertex"`
	Edge        int    `json:"edge"`
	Tile        int    `json:"tile"`
	Target      int    `json:"target"`
	Offer       int    `json:"offer"`
	Give        []int  `json:"give"`
	Take        []int  `json:"take"`
	Type        string `json:"type"`
	Card        int    `json:"card"`
	Tokens      []int  `json:"tokens"`
	Route       int    `json:"route"`
	Color       int    `json:"color"`
	Slot        int    `json:"slot"`
}
type State struct {
	Kind     string   `json:"kind"`
	Turn     int      `json:"turn"`
	Phase    string   `json:"phase"`
	Round    int      `json:"round"`
	Finished bool     `json:"finished"`
	Winners  []int    `json:"winners"`
	Log      []string `json:"log"`
	Catan    *Catan   `json:"catan,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "catan" {
		return nil, errors.New("未知游戏")
	}
	if n < 3 || n > 4 {
		return nil, errors.New("卡坦岛需要 3–4 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initCatan(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	err := s.applyCatan(player, a)
	if err == nil && len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	return err
}
func (s *State) View(player int) map[string]any {
	b, _ := json.Marshal(s)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	s.catanView(v, player)
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
