package carcassonne

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Rotation int    `json:"rotation"`
	Feature  int    `json:"feature"`
	Tile     int    `json:"tile"`
	Type     string `json:"type"`
}
type State struct {
	Kind        string       `json:"kind"`
	Turn        int          `json:"turn"`
	Phase       string       `json:"phase"`
	Round       int          `json:"round"`
	Finished    bool         `json:"finished"`
	Winners     []int        `json:"winners"`
	Log         []string     `json:"log"`
	Carcassonne *Carcassonne `json:"carcassonne,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "carcassonne" {
		return nil, errors.New("未知游戏")
	}
	if n < 2 || n > 5 {
		return nil, errors.New("卡卡颂需要 2–5 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initCarcassonne(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	err := s.applyCarcassonne(player, a)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	return err
}
func (s *State) View(player int) map[string]any {
	b, _ := json.Marshal(s)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	s.carView(v, player)
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
