package dota

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	Dota    *DotaOrder `json:"dota,omitempty"`
	Targets []int      `json:"targets"`
	Choice  string     `json:"choice"`
	Prompt  int        `json:"prompt"`
	Target  int        `json:"target"`
	Type    string     `json:"type"`
	Card    int        `json:"card"`
}
type State struct {
	Kind     string   `json:"kind"`
	Turn     int      `json:"turn"`
	Phase    string   `json:"phase"`
	Round    int      `json:"round"`
	Finished bool     `json:"finished"`
	Winners  []int    `json:"winners"`
	Log      []string `json:"log"`
	Dota     *Dota    `json:"dota,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "dota" {
		return nil, errors.New("未知游戏")
	}
	if n != 2 && n != 4 && n != 6 {
		return nil, errors.New("兵线争锋需要 2、4 或 6 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initDota(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	return s.applyDota(player, a)
}
func (s *State) View(player int) map[string]any {
	return s.dotaView(player)
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
