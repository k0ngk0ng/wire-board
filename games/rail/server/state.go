package rail

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
)

type Action struct {
	Vertex int    `json:"vertex"`
	Type   string `json:"type"`
	Tokens []int  `json:"tokens"`
	Route  int    `json:"route"`
	Color  int    `json:"color"`
	Wild   int    `json:"wild"`
	Slot   int    `json:"slot"`
	Keep   []int  `json:"keep"`
}
type State struct {
	Kind     string   `json:"kind"`
	Turn     int      `json:"turn"`
	Phase    string   `json:"phase"`
	Round    int      `json:"round"`
	Finished bool     `json:"finished"`
	Winners  []int    `json:"winners"`
	Log      []string `json:"log"`
	Rail     *Rail    `json:"rail,omitempty"`
}

func New(kind string, n int) (*State, error) {
	if kind != "rail" {
		return nil, errors.New("未知游戏")
	}
	if n < 2 || n > 5 {
		return nil, errors.New("铁路环游需要 2–5 位玩家")
	}
	s := &State{Kind: kind, Phase: "turn", Round: 1, Log: []string{}}
	s.initRail(n)
	return s, nil
}
func (s *State) Apply(player int, a Action) error {
	if s.Finished {
		return errors.New("本局已结束")
	}
	if s.Rail.Setup {
		return s.applyRailSetup(player, a)
	}
	if player != s.Turn {
		return errors.New("还没有轮到你")
	}
	err := s.applyRail(a)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
	return err
}
func (s *State) View(player int) map[string]any {
	b, _ := json.Marshal(s)
	var v map[string]any
	_ = json.Unmarshal(b, &v)

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
			delete(m, "stationRoutes")
			delete(m, "mandalaCount")
			delete(m, "mandalaScore")
			delete(m, "stationScore")
			if i == player {
				tickets, _, completed, stationRoutes := s.Rail.evaluateTickets(i)
				if s.Rail.Map == "india" {
					components := s.Rail.mandalaComponents(i)
					for j, t := range tickets {
						tickets[j].Mandala = t.Complete && t.A != t.B && components[t.A] == components[t.B]
					}
				}
				m["tickets"] = tickets
				m["completed"] = completed
				m["stationRoutes"] = stationRoutes
			}
		}
		if i != player {
			delete(m, "hand")
			if !s.Finished {
				delete(m, "tickets")
			}
		}
	}
	info := s.Rail.info()
	g["mapInfo"] = info
	if player >= 0 && player < len(s.Rail.Players) && !s.Finished {
		payments := map[int][]RailPayment{}
		for _, r := range s.Rail.data().Routes {
			if options := s.Rail.paymentOptions(player, r); len(options) > 0 {
				payments[r.ID] = options
			}
		}
		g["payments"] = payments
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
