package game

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	Cards    []int  `json:"cards"`
	Targets  []int  `json:"targets"`
	Choice   string `json:"choice"`
	Skill    string `json:"skill"`
	Prompt   int    `json:"prompt"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Rotation int    `json:"rotation"`
	Feature  int    `json:"feature"`
	Vertex   int    `json:"vertex"`
	Edge     int    `json:"edge"`
	Tile     int    `json:"tile"`
	Target   int    `json:"target"`
	Offer    int    `json:"offer"`
	Give     []int  `json:"give"`
	Take     []int  `json:"take"`
	Type     string `json:"type"`
	Card     int    `json:"card"`
	Tier     int    `json:"tier"`
	Tokens   []int  `json:"tokens"`
	Noble    int    `json:"noble"`
	Route    int    `json:"route"`
	Color    int    `json:"color"`
	Wild     int    `json:"wild"`
	Slot     int    `json:"slot"`
	Keep     []int  `json:"keep"`
}
type State struct {
	Sanguosha   *Sanguosha   `json:"sanguosha,omitempty"`
	Carcassonne *Carcassonne `json:"carcassonne,omitempty"`
	Catan       *Catan       `json:"catan,omitempty"`
	Kind        string       `json:"kind"`
	Turn        int          `json:"turn"`
	Phase       string       `json:"phase"`
	Round       int          `json:"round"`
	Finished    bool         `json:"finished"`
	Winners     []int        `json:"winners"`
	Log         []string     `json:"log"`
	Splendor    *Splendor    `json:"splendor,omitempty"`
	Rail        *Rail        `json:"rail,omitempty"`
}

func New(kind string, n int) (*State, error) {
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	switch kind {
	case "sanguosha":
		if n < 4 || n > 8 {
			return nil, errors.New("三国杀需要 4–8 位玩家")
		}
		s.initSanguosha(n)
	case "carcassonne":
		if n < 2 || n > 5 {
			return nil, errors.New("卡卡颂需要 2–5 位玩家")
		}
		s.initCarcassonne(n)
	case "catan":
		if n < 3 || n > 4 {
			return nil, errors.New("卡坦岛需要 3–4 位玩家")
		}
		s.initCatan(n)
	case "splendor":
		if n < 2 || n > 4 {
			return nil, errors.New("璀璨宝石需要 2–4 位玩家")
		}
		s.initSplendor(n)
	case "rail":
		if n < 2 || n > 5 {
			return nil, errors.New("铁路环游需要 2–5 位玩家")
		}
		s.initRail(n)
	default:
		return nil, errors.New("未知游戏")
	}
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	if s.Sanguosha != nil {
		return s.applySanguosha(player, a)
	}
	if s.Carcassonne != nil {
		err := s.applyCarcassonne(player, a)
		if len(s.Log) > 80 {
			s.Log = s.Log[len(s.Log)-80:]
		}
		return err
	}
	if s.Catan != nil {
		err := s.applyCatan(player, a)
		if len(s.Log) > 80 {
			s.Log = s.Log[len(s.Log)-80:]
		}
		return err
	}
	if s.Rail != nil && s.Rail.Setup {
		return s.applyRailSetup(player, a)
	}
	if player != s.Turn {
		return errors.New("还没有轮到你")
	}
	var err error
	if s.Kind == "splendor" {
		err = s.applySplendor(a)
	} else {
		err = s.applyRail(a)
	}
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	return err
}
func (s *State) View(player int) map[string]any {
	if s.Sanguosha != nil {
		return s.sgView(player)
	}
	b, _ := json.Marshal(s)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	if s.Carcassonne != nil {
		s.carView(v, player)
		return v
	}
	if s.Catan != nil {
		s.catanView(v, player)
		return v
	}
	if s.Splendor != nil {
		g := v["splendor"].(map[string]any)
		delete(g, "decks")
		g["remaining"] = []int{len(s.Splendor.Decks[0]), len(s.Splendor.Decks[1]), len(s.Splendor.Decks[2])}
		for i, p := range g["players"].([]any) {
			if i != player && !s.Finished {
				m := p.(map[string]any)
				m["reservedCount"] = len(s.Splendor.Players[i].Reserved)
				delete(m, "reserved")
			}
		}
	} else {
		g := v["rail"].(map[string]any)
		delete(g, "deck")
		delete(g, "ticketDeck")
		delete(g, "discard")
		delete(g, "setupPending")
		g["remaining"] = len(s.Rail.Deck) + len(s.Rail.Discard)
		g["ticketsRemaining"] = len(s.Rail.TicketDeck)
		for i, p := range g["players"].([]any) {
			m := p.(map[string]any)
			m["handCount"] = sum(s.Rail.Players[i].Hand)
			m["ticketCount"] = len(s.Rail.Players[i].Tickets)
			if !s.Finished {
				// Progress belongs to this viewer only. Never mutate persisted scoring
				// fields: final scoring must still count each destination exactly once.
				delete(m, "completed")
				if i == player {
					tickets := append([]Ticket{}, s.Rail.Players[i].Tickets...)
					completed := 0
					for j := range tickets {
						tickets[j].Complete = Connected(s.Rail.Owners, i, tickets[j].A, tickets[j].B)
						if tickets[j].Complete {
							completed++
						}
					}
					m["tickets"] = tickets
					m["completed"] = completed
				}
			}
			if i != player {
				delete(m, "hand")
				if !s.Finished {
					delete(m, "tickets")
				}
			}
		}
		if s.Rail.Setup {
			delete(g, "pending")
			ready := make([]bool, len(s.Rail.Players))
			for i := range ready {
				ready[i] = len(s.Rail.SetupPending[i]) == 0
			}
			g["setupReady"] = ready
			if player >= 0 && player < len(ready) && !ready[player] {
				g["pending"] = clone(s.Rail.SetupPending[player])
			}
		} else if player != s.Turn {
			delete(g, "pending")
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
