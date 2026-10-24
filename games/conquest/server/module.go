package conquest

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	conquestmaps "github.com/k0ngk0ng/wire-board/games/conquest/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"math"
	"slices"
	"sync"
	"time"
)

type geography struct {
	data  conquestmaps.Map
	index map[string]int
}

var mapOnce sync.Once
var geographyMaps map[string]*geography

func mapFor(id string) (*geography, error) {
	mapOnce.Do(func() {
		geographyMaps = map[string]*geography{}
		for _, entry := range conquestmaps.Catalog() {
			id := entry.ID
			m, e := conquestmaps.Load(id)
			if e != nil {
				continue
			}
			g := &geography{data: m, index: map[string]int{}}
			for i, p := range m.Provinces {
				g.index[p.ID] = i
			}
			geographyMaps[id] = g
		}
	})
	g, ok := geographyMaps[id]
	if !ok {
		return nil, errors.New("请选择世界、欧洲、美国或中国地图")
	}
	return g, nil
}
func strict(b []byte, v any) error {
	if len(b) == 0 || len(b) > 16384 || bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, b json.RawMessage) (json.RawMessage, error) {
	if n < 2 || n > 8 {
		return nil, errors.New("征服时代支持2～8席")
	}
	o := Options{MapID: "world", Difficulty: "normal", TurnSeconds: 120}
	if len(b) > 0 {
		if e := strict(b, &o); e != nil {
			return nil, e
		}
	}
	if _, e := mapFor(o.MapID); e != nil {
		return nil, e
	}
	if !slices.Contains([]string{"easy", "normal", "hard"}, o.Difficulty) || !slices.Contains([]int{60, 120, 180}, o.TurnSeconds) {
		return nil, errors.New("无效难度或行动时限")
	}
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "conquest", Name: "征服时代", MinPlayers: 2, MaxPlayers: 8, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim, ReclaimOnCancel: true, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer, PreciseTimers: true}
}
func start(c gameapi.Context, b json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, b)
	if e != nil {
		return gameapi.Match{}, e
	}
	var options Options
	_ = json.Unmarshal(o, &options)
	g, _ := mapFor(options.MapID)
	sum := sha256.Sum256([]byte(fmt.Sprintf("conquest/%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Players)))
	s := State{Options: options, Session: fmt.Sprintf("%x", sum[:12]), Epoch: 1, Round: 1, Phase: "planning", MapID: g.data.ID, MapVersion: g.data.Version, MapSHA256: g.data.SHA256, Seed: binary.LittleEndian.Uint64(sum[:8]), Players: make([]Player, c.Players), Provinces: make([]Province, len(g.data.Provinces)), Plans: make([][]Order, c.Players), NextOrder: make([]int, c.Players), PauseVotes: make([]bool, c.Players), ResumeVotes: make([]bool, c.Players), Remaining: make([]int64, c.Players), Events: []Event{}, Pending: []queued{}, Winner: -1, Log: []string{"招募、建设与调兵形成计划，所有势力提交后统一执行。"}}
	for i, p := range g.data.Provinces {
		s.Provinces[i] = Province{ID: p.ID, Owner: -1, Troops: 500, Mobile: 500, Population: 25000, Economy: 50000, Morale: 90}
	}
	starts, e := startingRegions(g, c.Players, s.Seed)
	if e != nil {
		return gameapi.Match{}, e
	}
	for i, id := range starts {
		p := &s.Provinces[g.index[id]]
		p.Owner = i
		p.Town = true
		p.Tower = true
		s.Players[i] = Player{Seat: i, Color: colors[i], Gold: 5000, MP10: 32, King: id, Tax: 10, Morale: 90, EconomyRate: 80, MilitaryRate: 80, Deadline: c.Now.Add(time.Duration(options.TurnSeconds) * time.Second).UnixMilli()}
		s.Plans[i] = []Order{}
	}
	s.refresh()
	return s.envelope(), nil
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != schema || len(m.Data) > 4<<20 {
		return nil, errors.New("不支持的征服时代存档")
	}
	var s State
	if e := json.Unmarshal(m.Data, &s); e != nil {
		return nil, e
	}
	g, e := mapFor(s.MapID)
	if e != nil {
		return nil, e
	}
	n := len(s.Players)
	if _, e = configure(n, raw(s.Options)); e != nil {
		return nil, e
	}
	if s.Options.MapID != s.MapID || s.MapVersion != g.data.Version || s.MapSHA256 != g.data.SHA256 {
		return nil, errors.New("地图版本与存档不一致")
	}
	if s.Session == "" || s.Epoch < 1 || s.Round < 1 || !slices.Contains([]string{"planning", "resolving", "paused", "finished"}, s.Phase) || len(s.Provinces) != len(g.data.Provinces) || len(s.Plans) != n || len(s.NextOrder) != n || len(s.PauseVotes) != n || len(s.ResumeVotes) != n || len(s.Remaining) != n || s.Cursor < 0 || s.Cursor > len(s.Pending) || len(s.Events) > 64 || len(s.Pending) > maxOrders*n {
		return nil, errors.New("征服时代存档结构无效")
	}
	for i, p := range s.Provinces {
		if p.ID != g.data.Provinces[i].ID || p.Owner < -1 || p.Owner >= n || p.Troops < 0 || p.Troops > maxTroops || p.Population < 0 || p.Population > maxCivil || p.Economy < 0 || p.Economy > maxCivil {
			return nil, errors.New("无效地区状态")
		}
	}
	for i, p := range s.Players {
		if p.Seat != i || p.MP10 < 0 || p.MP10 > 10000 || p.Tax < 0 || p.Tax > 100 || p.Morale < 0 || p.Morale > 100 || p.Gold < -1000000000000 || p.Gold > 1000000000000 || p.Deadline < 0 || len(s.Plans[i]) > maxOrders {
			return nil, errors.New("无效势力状态")
		}
		if !p.Eliminated {
			j, ok := g.index[p.King]
			if !ok || s.Provinces[j].Owner != i {
				return nil, errors.New("君主与领土不一致")
			}
		}
		for _, o := range s.Plans[i] {
			if o.ID < 1 || o.ID > s.NextOrder[i] {
				return nil, errors.New("无效指令编号")
			}
		}
	}
	if s.Phase == "resolving" && s.ResolveAt <= 0 {
		return nil, errors.New("缺少结算时间")
	}
	if s.Phase == "finished" && (s.Winner < 0 || s.Winner >= n) {
		return nil, errors.New("无效胜利结果")
	}
	if err := s.validateExecution(m, g); err != nil {
		return nil, err
	}
	return &s, nil
}
func (s *State) refresh() {
	for i := range s.Players {
		p := &s.Players[i]
		p.Territories, p.Troops = 0, 0
		p.Movement = float64(p.MP10) / 10
	}
	for i := range s.Provinces {
		p := &s.Provinces[i]
		p.Mobile = p.Troops
		if p.Owner >= 0 {
			s.Players[p.Owner].Territories++
			s.Players[p.Owner].Troops += p.Troops
			p.Morale = s.Players[p.Owner].Morale
		}
	}
	for i := range s.Players {
		p := &s.Players[i]
		p.Income = income(s, i)
		p.Upkeep = int64(p.Troops/5) * int64(p.MilitaryRate) / 80
		p.EconomySpending = p.Income / 10 * int64(p.EconomyRate) / 80
	}
}
func (s *State) envelope() gameapi.Match {
	s.refresh()
	m := gameapi.Match{Schema: schema, Data: raw(s), Log: append([]string(nil), s.Log...), Attention: []int{}}
	for i, p := range s.Players {
		if p.Eliminated {
			m.Eliminated = append(m.Eliminated, i)
			continue
		}
		if s.Phase == "planning" || s.Phase == "paused" {
			m.Actors = append(m.Actors, i)
		}
		if s.Phase == "planning" && !p.Submitted {
			m.Attention = append(m.Attention, i)
			if m.Deadline == 0 || p.Deadline < m.Deadline {
				m.Deadline = p.Deadline
				m.TimeoutActors = []int{i}
			} else if p.Deadline == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, i)
			}
		}
	}
	if s.Phase == "resolving" {
		m.SystemDeadline = s.ResolveAt
	}
	if s.Phase == "paused" {
		m.Paused = true
	}
	if s.Phase == "finished" {
		ranks := make([]int, len(s.Players))
		scores := make([]int, len(s.Players))
		for i, p := range s.Players {
			ranks[i] = 2
			scores[i] = p.Territories
			if i == s.Winner {
				ranks[i] = 1
			} else {
				for j, q := range s.Players {
					if j != i && j != s.Winner && q.LostRound > p.LostRound {
						ranks[i]++
					}
				}
			}
		}
		m.Result = &gameapi.Result{Ranks: ranks, Winners: []int{s.Winner}, Scores: scores, Details: raw(map[string]any{"mapId": s.MapID, "rounds": s.Round, "reason": "last_empire"})}
	}
	return m
}
func (s *State) note(t string) {
	s.Log = append(s.Log, t)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
}
func view(m gameapi.Match, seat int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	s.refresh()
	v := View{Options: s.Options, Session: s.Session, Epoch: s.Epoch, Round: s.Round, Phase: s.Phase, MapID: s.MapID, MapVersion: s.MapVersion, MapSHA256: s.MapSHA256, Provinces: append([]Province(nil), s.Provinces...), Players: append([]Player(nil), s.Players...), Orders: []Order{}, Projected: append([]Province(nil), s.Provinces...), PauseVotes: s.PauseVotes, ResumeVotes: s.ResumeVotes, Events: s.Events, Winner: s.Winner, Log: s.Log, Controls: Controls{CanBuildTower: s.Options.Fog, RecruitCost: 1, FortCost: 5000, TowerCost: 2500, TownCost: 75000, SeaCost: 2, MoveCost: 1, RecruitMovement: 1, FortMovement: .5, TowerMovement: .3, TownMovement: 5, MaxOrders: maxOrders}}
	if seat >= 0 && seat < len(s.Players) && !s.Players[seat].Eliminated {
		p := s.Players[seat]
		v.AvailableGold = p.Gold
		v.AvailableMovement = float64(p.MP10) / 10
		if s.Phase == "planning" || s.Phase == "paused" {
			v.Orders = append([]Order{}, s.Plans[seat]...)
			projected, gold, mp, _, err := s.project(seat, s.Plans[seat])
			if err != nil {
				return nil, fmt.Errorf("存档计划无效: %w", err)
			}
			v.Projected = projected
			v.AvailableGold = gold
			v.AvailableMovement = float64(mp) / 10
		}
		v.Controls.CanPlan = s.Phase == "planning" && !p.Submitted
		v.Controls.CanSubmit = v.Controls.CanPlan
		v.Controls.CanUnsubmit = s.Phase == "planning" && p.Submitted
		v.Controls.CanPause = s.Phase == "planning"
		v.Controls.CanResume = s.Phase == "paused"
		v.Controls.CanResign = s.Phase == "planning"
	}
	s.mask(&v, seat)
	return v, nil
}
func restore(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Players != len(s.Players) {
		return m, errors.New("席位数与存档不一致")
	}
	if err := s.validateRestoredPlans(); err != nil {
		return m, err
	}
	return s.envelope(), nil
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor < 0 || c.Actor >= len(s.Players) {
		return m, errors.New("无效席位")
	}
	p := &s.Players[c.Actor]
	if s.Phase == "planning" && !p.Eliminated && !p.Submitted && p.Deadline <= c.Now.UnixMilli() {
		p.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		return s.envelope(), nil
	}
	return m, nil
}
func acceptStale(c gameapi.Context, m gameapi.Match, b json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	var a Action
	return strict(b, &a) == nil && a.Session == s.Session && a.Epoch == s.Epoch && c.Actor >= 0 && c.Actor < len(s.Players)
}
func income(s *State, seat int) int64 {
	production := int64(0)
	for _, p := range s.Provinces {
		if p.Owner == seat {
			production += int64(p.Economy) * int64(p.Population) / 50000
		}
	}
	p := s.Players[seat]
	return int64(math.Round(math.Pow(float64(production)*.08, .96) * float64(p.Tax) / 10 * float64(p.Morale) / 100))
}
