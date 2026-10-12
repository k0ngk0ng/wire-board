// Package monopoly owns all Monopoly rules, data, views and saved state.
package monopoly

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

//go:embed board.json
var boardJSON []byte

type Space struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Color        string `json:"color"`
	Price        int    `json:"price"`
	Group        int    `json:"group"`
	Rents        [6]int `json:"rents"`
	BuildingCost int    `json:"buildingCost"`
	Tax          int    `json:"tax"`
}

var board = func() []Space {
	var v []Space
	if err := json.Unmarshal(boardJSON, &v); err != nil {
		panic(err)
	}
	return v
}()

type Options struct {
	AuctionSeconds int `json:"auctionSeconds"`
	TurnSeconds    int `json:"turnSeconds"`
	TradeSeconds   int `json:"tradeSeconds"`
}
type Player struct {
	Cash      int      `json:"cash"`
	Position  int      `json:"position"`
	Bankrupt  bool     `json:"bankrupt"`
	Jailed    bool     `json:"jailed"`
	JailTurns int      `json:"jailTurns"`
	JailCards []string `json:"jailCards"`
}
type Property struct {
	Owner     int  `json:"owner"`
	Mortgaged bool `json:"mortgaged"`
	Buildings int  `json:"buildings"` // 0..4 houses, 5 = hotel.
}
type Debt struct {
	Debtor   int    `json:"debtor"`
	Creditor int    `json:"creditor"` // -1 = Bank.
	Amount   int    `json:"amount"`
	Reason   string `json:"reason"`
}
type Auction struct {
	Kind      string `json:"kind"` // property, house or hotel
	Property  int    `json:"property"`
	Bid       int    `json:"bid"`
	Bidder    int    `json:"bidder"`
	Expires   int64  `json:"expires"`
	Withdrawn []bool `json:"withdrawn"`
	Resume    *Pause `json:"resume,omitempty"`
}
type Pause struct {
	Phase    string `json:"phase"`
	TimeLeft int64  `json:"timeLeft"`
}
type Transfer struct {
	Player   int `json:"player"`
	Property int `json:"property"`
}

// Effects keep interrupted obligations durable, including sequential estates.
type Effect struct {
	Kind     string `json:"kind"`
	Actor    int    `json:"actor"`
	Target   int    `json:"target"`
	Property int    `json:"property"`
	Amount   int    `json:"amount"`
	Reason   string `json:"reason"`
}
type Random struct {
	Key     [32]byte `json:"key"`
	Counter uint64   `json:"counter"`
}

func (r *Random) pick(n int) int {
	// Rejection sampling avoids modulo bias; the secret key and counter are
	// persisted and never returned in views or inspected by bot policy.
	for {
		r.Counter++
		var input [40]byte
		copy(input[:32], r.Key[:])
		binary.BigEndian.PutUint64(input[32:], r.Counter)
		hash := sha256.Sum256(input[:])
		v := binary.BigEndian.Uint64(hash[:8])
		limit := ^uint64(0) - (^uint64(0) % uint64(n))
		if v < limit {
			return int(v % uint64(n))
		}
	}
}

type State struct {
	MotionSerial int        `json:"motionSerial"`
	Motion       []Motion   `json:"motion,omitempty"`
	RollSerial   int        `json:"rollSerial"`
	Options      Options    `json:"options"`
	Players      []Player   `json:"players"`
	Properties   []Property `json:"properties"`
	Turn         int        `json:"turn"`
	Phase        string     `json:"phase"`
	NextPhase    string     `json:"nextPhase"`
	Window       int        `json:"window"`
	Deadline     int64      `json:"deadline"`
	Dice         [2]int     `json:"dice"`
	UtilityDice  [2]int     `json:"utilityDice"`
	Doubles      int        `json:"doubles"`
	Houses       int        `json:"houses"`
	Hotels       int        `json:"hotels"`
	BankNet      int        `json:"bankNet"` // Signed bank receipts; player cash + BankNet = 0.
	Auction      *Auction   `json:"auction,omitempty"`
	Debt         *Debt      `json:"debt,omitempty"`
	Transfer     *Transfer  `json:"transfer,omitempty"`
	Effects      []Effect   `json:"effects,omitempty"`
	Eliminated   []int      `json:"eliminated"`
	Waiting      []bool     `json:"waiting"`
	Random       Random     `json:"random"`
	Decks        [2][]int   `json:"decks"`
	Log          []string   `json:"log"`
	Trades       []Trade    `json:"trades"`
	TradeSerial  int        `json:"tradeSerial"`
	LastCard     *CardEvent `json:"lastCard,omitempty"`
	CardSerial   int        `json:"cardSerial"`
	TurnSerial   int        `json:"turnSerial"`
	TradeSent    []int      `json:"tradeSent"`
}

func newState(n int, options Options, now time.Time) (*State, error) {
	s := &State{Options: options, Players: make([]Player, n), Properties: make([]Property, 40), Houses: 32, Hotels: 12, BankNet: -1500 * n, NextPhase: "end"}
	s.TradeSent = make([]int, n)
	for i := range s.TradeSent {
		s.TradeSent[i] = -1
	}
	if _, err := rand.Read(s.Random.Key[:]); err != nil {
		return nil, err
	}
	for i := range s.Players {
		s.Players[i].Cash = 1500
		s.Players[i].JailCards = []string{}
	}
	for i := range s.Properties {
		s.Properties[i].Owner = -1
	}
	for d := range s.Decks {
		s.Decks[d] = make([]int, 16)
		for i := range s.Decks[d] {
			s.Decks[d][i] = i
		}
		for i := 15; i > 0; i-- {
			j := s.Random.pick(i + 1)
			s.Decks[d][i], s.Decks[d][j] = s.Decks[d][j], s.Decks[d][i]
		}
	}
	candidates := make([]int, n)
	for i := range candidates {
		candidates[i] = i
	}
	for len(candidates) > 1 {
		best := 0
		next := []int{}
		for _, i := range candidates {
			a, b := s.Random.pick(6)+1, s.Random.pick(6)+1
			s.note("玩家 %d 的先手骰为 %d、%d", i+1, a, b)
			if a+b > best {
				best = a + b
				next = []int{i}
			} else if a+b == best {
				next = append(next, i)
			}
		}
		candidates = next
	}
	s.Turn = candidates[0]
	s.enter("roll", now)
	return s, nil
}
func (s *State) note(format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
	if len(s.Log) > 100 {
		s.Log = s.Log[len(s.Log)-100:]
	}
}
func (s *State) enter(phase string, now time.Time) {
	s.Phase = phase
	if phase == "finished" {
		s.Trades = nil
	}
	s.Window++
	s.Waiting = make([]bool, len(s.Players))
	s.Deadline = now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
}
func (s *State) active() int {
	n := 0
	for _, p := range s.Players {
		if !p.Bankrupt {
			n++
		}
	}
	return n
}
func (s *State) nextTurn() {
	s.TurnSerial++
	for range s.Players {
		s.Turn = (s.Turn + 1) % len(s.Players)
		if !s.Players[s.Turn].Bankrupt {
			break
		}
	}
	s.Doubles = 0
}
func (s *State) dueActor() int {
	switch s.Phase {
	case "roll", "end", "purchase":
		return s.Turn
	case "debt":
		if s.Debt != nil {
			return s.Debt.Debtor
		}
	case "mortgage_transfer":
		if s.Transfer != nil {
			return s.Transfer.Player
		}
	}
	return -1
}
func (s *State) auctionMinimum() int {
	if s.Auction.Bid == 0 {
		return 10
	}
	return s.Auction.Bid + 1
}
func (s *State) auctionActors() []int {
	out := []int{}
	if s.Auction == nil {
		return out
	}
	for i, p := range s.Players {
		if p.Bankrupt || i == s.Auction.Bidder || s.Auction.Withdrawn[i] || s.liquidation(i) < s.auctionMinimum() {
			continue
		}
		if s.buildingAuction() && !s.canFundBuild(i, s.Auction.Kind, s.auctionMinimum()) {
			continue
		}
		out = append(out, i)
	}
	return out
}
func (s *State) actors() []int {
	if s.Phase == "finished" {
		return []int{}
	}
	due := s.dueActor()
	out := []int{}
	if due >= 0 && !s.Players[due].Bankrupt {
		out = append(out, due)
	}
	for i, p := range s.Players {
		if i != due && !p.Bankrupt && ((len(s.Waiting) <= i || !s.Waiting[i]) || slices.Contains(s.auctionActors(), i) || s.hasIncomingTrade(i)) {
			out = append(out, i)
		}
	}
	return out
}

func (s *State) validate() error {
	if len(s.Players) < 2 || len(s.Players) > 6 || len(s.Properties) != 40 || s.Turn < 0 || s.Turn >= len(s.Players) || s.Options.TurnSeconds <= 0 || s.Options.AuctionSeconds <= 0 {
		return errors.New("无效强手棋存档")
	}
	if !slices.Contains([]string{"roll", "end", "purchase", "auction", "debt", "mortgage_transfer", "finished"}, s.Phase) {
		return errors.New("未知强手棋阶段")
	}
	total := s.BankNet
	houses, hotels := s.Houses, s.Hotels
	for _, p := range s.Players {
		if p.Cash < 0 || p.Position < 0 || p.Position >= 40 {
			return errors.New("无效玩家资产")
		}
		total += p.Cash
	}
	if total != 0 {
		return errors.New("资金账目不平衡")
	}
	for i, p := range s.Properties {
		if p.Owner < -1 || p.Owner >= len(s.Players) || p.Buildings < 0 || p.Buildings > 5 {
			return errors.New("无效地产状态")
		}
		if p.Owner >= 0 && (board[i].Price == 0 || s.Players[p.Owner].Bankrupt) {
			return errors.New("无效地产归属")
		}
		if p.Buildings == 5 {
			hotels++
		} else {
			houses += p.Buildings
		}
		if p.Buildings > 0 && (p.Owner < 0 || p.Mortgaged || board[i].Kind != "street") {
			return errors.New("无效建筑状态")
		}
	}
	if s.Houses < 0 || s.Hotels < 0 || houses != 32 || hotels != 12 {
		return errors.New("建筑库存不平衡")
	}
	if s.Phase == "debt" && (s.Debt == nil || s.Debt.Debtor < 0 || s.Debt.Debtor >= len(s.Players) || s.Debt.Creditor < -1 || s.Debt.Creditor >= len(s.Players) || s.Debt.Amount <= 0) {
		return errors.New("无效欠款")
	}
	if s.Phase == "auction" && (s.Auction == nil || s.Auction.Property < 0 || s.Auction.Property >= 40 || len(s.Auction.Withdrawn) != len(s.Players)) {
		return errors.New("无效拍卖")
	}
	if s.Phase == "mortgage_transfer" && (s.Transfer == nil || s.Transfer.Player < 0 || s.Transfer.Player >= len(s.Players) || s.Transfer.Property < 0 || s.Transfer.Property >= 40) {
		return errors.New("无效抵押转让")
	}
	return s.validateDecks()
}

// Envelope serializes the game-owned state and projects only host capabilities.
func (s *State) Envelope() (gameapi.Match, error) {
	if err := s.validate(); err != nil {
		return gameapi.Match{}, err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{Schema: 1, Data: raw, Actors: s.actors(), Attention: []int{}, Eliminated: s.Eliminated, Log: append([]string{}, s.Log...)}
	if s.Phase == "finished" {
		ranks := make([]int, len(s.Players))
		scores := make([]int, len(s.Players))
		winners := []int{}
		for i, p := range s.Players {
			scores[i] = p.Cash
			if !p.Bankrupt {
				ranks[i] = 1
				winners = append(winners, i)
			}
		}
		for i, p := range s.Eliminated {
			ranks[p] = len(s.Players) - i
		}
		m.Result = &gameapi.Result{Ranks: ranks, Winners: winners, Scores: scores}
		return m, m.Validate(len(s.Players))
	}
	if s.Phase == "auction" {
		m.SystemDeadline = s.Auction.Expires
	} else if actor := s.dueActor(); actor >= 0 {
		m.Attention = []int{actor}
		m.TimeoutActors = []int{actor}
		m.Deadline = s.Deadline
	}
	for _, offer := range s.Trades {
		if m.SystemDeadline == 0 || offer.Expires < m.SystemDeadline {
			m.SystemDeadline = offer.Expires
		}
	}
	return m, m.Validate(len(s.Players))
}

func decode(match gameapi.Match) (*State, error) {
	if match.Schema != 1 {
		return nil, errors.New("不支持的强手棋存档版本")
	}
	var s State
	if err := json.Unmarshal(match.Data, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}
