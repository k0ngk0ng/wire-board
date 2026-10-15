package junqi

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"math/big"
	"sort"
	"time"
)

type Options struct {
	Mode         string `json:"mode"`
	Difficulty   string `json:"difficulty"`
	TurnSeconds  int    `json:"turnSeconds"`
	SetupSeconds int    `json:"setupSeconds"`
}
type Piece struct {
	ID       string `json:"id"`
	Seat     int    `json:"seat"`
	Kind     string `json:"kind"`
	Pos      int    `json:"pos"`
	Alive    bool   `json:"alive"`
	Revealed bool   `json:"revealed"`
}
type PieceView struct {
	ID       string `json:"id"`
	Seat     int    `json:"seat"`
	Kind     string `json:"kind"`
	Pos      int    `json:"pos"`
	Revealed bool   `json:"revealed"`
}
type PlayerView struct {
	Seat         int  `json:"seat"`
	Team         int  `json:"team"`
	Ready        bool `json:"ready"`
	Alive        bool `json:"alive"`
	Remaining    int  `json:"remaining"`
	FlagRevealed bool `json:"flagRevealed"`
}
type Event struct {
	ID           int      `json:"id"`
	Type         string   `json:"type"`
	Seat         int      `json:"seat"`
	Moves        []Move   `json:"moves"`
	Outcome      string   `json:"outcome"`
	Removed      []string `json:"removed"`
	Commanders   []int    `json:"commanders"`
	FlagCaptured bool     `json:"flagCaptured"`
	At           int64    `json:"at"`
}
type Offer struct {
	ID    int    `json:"id"`
	By    int    `json:"by"`
	Votes []bool `json:"votes"`
}
type State struct {
	Options      Options  `json:"options"`
	Players      int      `json:"players"`
	Round        string   `json:"round"`
	Phase        string   `json:"phase"`
	Turn         int      `json:"turn"`
	Ply          int      `json:"ply"`
	Pieces       []Piece  `json:"pieces"`
	Ready        []bool   `json:"ready"`
	Alive        []bool   `json:"alive"`
	Revisions    []int    `json:"revisions"`
	Deadlines    []int64  `json:"deadlines"`
	FlagRevealed []bool   `json:"flagRevealed"`
	NoCasualty   int      `json:"noCasualty"`
	Keys         []string `json:"keys"`
	Offer        *Offer   `json:"offer,omitempty"`
	OfferSerial  int      `json:"offerSerial"`
	Last         Event    `json:"last"`
	Recent       []Move   `json:"recent"`
	Winner       int      `json:"winner"`
	Reason       string   `json:"reason"`
	Log          []string `json:"log"`
}

// Do not embed State here: hidden kinds, repetition hashes and deployment
// inventory order must never become a seat's or a spectator's public payload.
type View struct {
	Options    Options      `json:"options"`
	Players    []PlayerView `json:"players"`
	Round      string       `json:"round"`
	Phase      string       `json:"phase"`
	Mode       string       `json:"mode"`
	Turn       int          `json:"turn"`
	Ply        int          `json:"ply"`
	Pieces     []PieceView  `json:"pieces"`
	Graph      Graph        `json:"graph"`
	Revision   int          `json:"revision"`
	Deadlines  []int64      `json:"deadlines"`
	Legal      []Move       `json:"legal"`
	NoCasualty int          `json:"noCasualty"`
	Offer      *Offer       `json:"offer,omitempty"`
	Last       Event        `json:"last"`
	Recent     []Move       `json:"recent"`
	Winner     int          `json:"winner"`
	Reason     string       `json:"reason"`
	Log        []string     `json:"log"`
	CanArrange bool         `json:"canArrange"`
	CanConfirm bool         `json:"canConfirm"`
	CanUnready bool         `json:"canUnready"`
	CanMove    bool         `json:"canMove"`
	CanVote    bool         `json:"canVote"`
	CanOffer   bool         `json:"canOffer"`
	CanResign  bool         `json:"canResign"`
}
type Action struct {
	Type     string `json:"type"`
	Round    string `json:"round"`
	Ply      int    `json:"ply"`
	Revision int    `json:"revision"`
	Unit     string `json:"unit,omitempty"`
	Other    string `json:"other,omitempty"`
	To       *int   `json:"to,omitempty"`
	Ready    *bool  `json:"ready,omitempty"`
	OfferID  int    `json:"offerID,omitempty"`
	Accept   *bool  `json:"accept,omitempty"`
}

func strict(raw []byte, v any) error {
	if len(raw) > 4096 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n != 2 && n != 4 {
		return nil, errors.New("军棋仅支持两人陆战棋或四国对家模式")
	}
	o := Options{Difficulty: "standard", TurnSeconds: 120, SetupSeconds: 180}
	if len(raw) > 0 && strict(raw, &o) != nil {
		return nil, errors.New("无效军棋设置")
	}
	if o.Difficulty != "easy" && o.Difficulty != "standard" || o.TurnSeconds < 30 || o.TurnSeconds > 300 || o.SetupSeconds < 30 || o.SetupSeconds > 300 {
		return nil, errors.New("请选择支持的电脑难度及 30～300 秒时限")
	}
	mode := "duel"
	if n == 4 {
		mode = "teams"
	}
	if o.Mode != "" && o.Mode != mode {
		return nil, errors.New("请坐满房间原定席位后开始对应军棋模式")
	}
	o.Mode = mode
	return json.Marshal(o)
}
func token() (string, error) {
	b := [16]byte{}
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func shuffle[T any](values []T) error {
	for i := len(values) - 1; i > 0; i-- {
		j, e := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if e != nil {
			return e
		}
		k := int(j.Int64())
		values[i], values[k] = values[k], values[i]
	}
	return nil
}
func (s *State) arrange(seat int) error {
	g := makeGraph(s.Players)
	slots := []int{}
	for _, node := range g.Nodes {
		if node.Seat == seat && !node.Camp {
			slots = append(slots, node.ID)
		}
	}
	if e := shuffle(slots); e != nil {
		return e
	}
	take := func(kind string) int {
		for i, pos := range slots {
			node := g.Nodes[pos]
			ok := kind != "flag" || node.HQ
			if kind == "mine" {
				ok = node.Row >= 4
			}
			if kind == "bomb" {
				ok = node.Row > 0 && !node.HQ
			}
			if kind != "flag" && kind != "mine" && node.HQ {
				ok = false
			}
			if ok {
				slots = append(slots[:i], slots[i+1:]...)
				return pos
			}
		}
		return -1
	}
	units := map[string][]int{}
	for i, p := range s.Pieces {
		if p.Seat == seat {
			units[p.Kind] = append(units[p.Kind], i)
		}
	}
	flag := take("flag")
	if flag < 0 {
		return errors.New("无法生成军旗位置")
	}
	s.Pieces[units["flag"][0]].Pos = flag
	// The second headquarters is a legal mine placement, so no mobile unit is
	// silently trapped there by the default computer formation.
	otherHQ := -1
	for i, pos := range slots {
		if g.Nodes[pos].HQ {
			otherHQ = pos
			slots = append(slots[:i], slots[i+1:]...)
			break
		}
	}
	if otherHQ < 0 {
		return errors.New("缺少大本营")
	}
	s.Pieces[units["mine"][0]].Pos = otherHQ
	for _, i := range units["mine"][1:] {
		s.Pieces[i].Pos = take("mine")
	}
	for _, i := range units["bomb"] {
		s.Pieces[i].Pos = take("bomb")
	}
	ordinary := []int{}
	for kind, list := range units {
		if kind != "flag" && kind != "mine" && kind != "bomb" {
			ordinary = append(ordinary, list...)
		}
	}
	sort.Ints(ordinary)
	if e := shuffle(ordinary); e != nil {
		return e
	}
	for _, i := range ordinary {
		s.Pieces[i].Pos = take(s.Pieces[i].Kind)
	}
	return s.validateFormation(seat)
}
func (s *State) validateFormation(seat int) error {
	g := makeGraph(s.Players)
	counts := map[string]int{}
	seen := map[int]bool{}
	for _, p := range s.Pieces {
		if p.Seat != seat {
			continue
		}
		if !p.Alive || p.Pos < 0 || p.Pos >= len(g.Nodes) || seen[p.Pos] {
			return errors.New("布阵棋子数量或位置不正确")
		}
		seen[p.Pos] = true
		node := g.Nodes[p.Pos]
		if node.Seat != seat || node.Camp || p.Kind == "flag" && !node.HQ || p.Kind == "mine" && node.Row < 4 || p.Kind == "bomb" && node.Row == 0 {
			return errors.New("布阵须遵守军旗、地雷、炸弹和行营限制")
		}
		counts[p.Kind]++
	}
	for k, n := range inventory {
		if counts[k] != n {
			return errors.New("布阵棋子数量不正确")
		}
	}
	return nil
}
func decode(m gameapi.Match) (*State, error) {
	var s State
	if m.Schema != 1 || json.Unmarshal(m.Data, &s) != nil || s.Round == "" || (s.Players != 2 && s.Players != 4) || s.Turn < 0 || s.Turn >= s.Players || s.Ply < 0 || s.NoCasualty < 0 || s.NoCasualty > 200 || len(s.Keys) > 201 {
		return nil, errors.New("无效军棋存档")
	}
	if s.Phase != "deploy" && s.Phase != "playing" && s.Phase != "finished" {
		return nil, errors.New("无效军棋阶段")
	}
	for _, n := range []int{len(s.Ready), len(s.Alive), len(s.Revisions), len(s.Deadlines), len(s.FlagRevealed)} {
		if n != s.Players {
			return nil, errors.New("无效席位信息")
		}
	}
	o, _ := json.Marshal(s.Options)
	if _, e := configure(s.Players, o); e != nil {
		return nil, e
	}
	g := makeGraph(s.Players)
	ids, positions := map[string]bool{}, map[int]bool{}
	for _, p := range s.Pieces {
		if p.ID == "" || ids[p.ID] || p.Seat < 0 || p.Seat >= s.Players || inventory[p.Kind] == 0 {
			return nil, errors.New("无效棋子")
		}
		ids[p.ID] = true
		if p.Alive {
			if !s.Alive[p.Seat] || p.Pos < 0 || p.Pos >= len(g.Nodes) || positions[p.Pos] {
				return nil, errors.New("棋子位置冲突")
			}
			positions[p.Pos] = true
		}
	}
	if s.Offer != nil && (s.Offer.ID <= 0 || s.Offer.By < 0 || s.Offer.By >= s.Players || len(s.Offer.Votes) != s.Players) {
		return nil, errors.New("无效和棋提议")
	}
	return &s, nil
}
func projected(s *State, actor int) []PieceView {
	pieces := []PieceView{}
	for _, p := range s.Pieces {
		if !p.Alive {
			continue
		}
		kind := "unknown"
		if p.Seat == actor || p.Revealed || s.Phase == "finished" {
			kind = p.Kind
		}
		pieces = append(pieces, PieceView{p.ID, p.Seat, kind, p.Pos, p.Revealed})
	}
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].Pos < pieces[j].Pos })
	return pieces
}
func PositionKey(s *State) string {
	type item struct {
		ID       string
		Kind     string
		Pos      int
		Revealed bool
	}
	parts := []item{}
	for _, p := range s.Pieces {
		if p.Alive {
			parts = append(parts, item{p.ID, p.Kind, p.Pos, p.Revealed})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].ID < parts[j].ID })
	raw, _ := json.Marshal([]any{s.Turn, s.Alive, parts})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func sideName(seat int) string { return []string{"红方", "蓝方", "黄方", "绿方"}[seat] }

func (s *State) note(text string) {
	s.Log = append(s.Log, text)
	if len(s.Log) > 80 {
		s.Log = append([]string(nil), s.Log[len(s.Log)-80:]...)
	}
}
func (s *State) finish(winner int, reason string) {
	s.Phase, s.Winner, s.Reason, s.Offer = "finished", winner, reason, nil
	for i := range s.Deadlines {
		s.Deadlines[i] = 0
	}
	labels := map[string]string{"last_team": "仅剩一个阵营，胜负已定", "agreement": "存活玩家一致同意和棋", "no_casualty": "连续 200 着无战损，和棋", "repetition": "局面与手番四次重复，和棋"}
	s.note(labels[reason])
}
func (s *State) envelope() (gameapi.Match, error) {
	// An army is stored in private inventory order. Clearing it must not
	// publish that ordering through an otherwise rank-free event payload.
	sort.Strings(s.Last.Removed)
	raw, e := json.Marshal(s)
	m := gameapi.Match{Schema: 1, Data: raw, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: append([]string(nil), s.Log...)}
	for i, alive := range s.Alive {
		if !alive {
			m.Eliminated = append(m.Eliminated, i)
		}
	}
	if s.Phase == "finished" {
		r := &gameapi.Result{Ranks: make([]int, s.Players), Winners: []int{}, Team: s.Players == 4}
		for i := range r.Ranks {
			r.Ranks[i] = 1
			if s.Winner >= 0 {
				if allied(s.Players, i, s.Winner) {
					r.Winners = append(r.Winners, i)
				} else {
					r.Ranks[i] = 2
					if s.Players == 4 {
						r.Ranks[i] = 3
					}
				}
			}
		}
		r.Draw = s.Winner < 0
		r.Details, _ = json.Marshal(map[string]any{"reason": s.Reason, "plies": s.Ply, "noCasualty": s.NoCasualty})
		m.Result = r
		return m, e
	}
	for i, alive := range s.Alive {
		if alive {
			m.Actors = append(m.Actors, i)
		}
	}
	if s.Phase == "deploy" {
		for i, ready := range s.Ready {
			if !ready && (m.Deadline == 0 || s.Deadlines[i] < m.Deadline) {
				m.Deadline = s.Deadlines[i]
			}
		}
		for i, ready := range s.Ready {
			if !ready {
				m.Attention = append(m.Attention, i)
				if s.Deadlines[i] == m.Deadline {
					m.TimeoutActors = append(m.TimeoutActors, i)
				}
			}
		}
	} else {
		m.Deadline = s.Deadlines[s.Turn]
		m.TimeoutActors = []int{s.Turn}
		m.Attention = []int{s.Turn}
	}
	return m, e
}
func view(m gameapi.Match, actor int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	v := View{Options: s.Options, Round: s.Round, Phase: s.Phase, Mode: "duel", Turn: s.Turn, Ply: s.Ply, Pieces: projected(s, actor), Graph: makeGraph(s.Players), Players: []PlayerView{}, Deadlines: append([]int64(nil), s.Deadlines...), Legal: []Move{}, NoCasualty: s.NoCasualty, Last: s.Last, Recent: append([]Move{}, s.Recent...), Winner: s.Winner, Reason: s.Reason, Log: append([]string(nil), s.Log...)}
	if s.Players == 4 {
		v.Mode = "teams"
	}
	if s.Offer != nil {
		v.Offer = &Offer{s.Offer.ID, s.Offer.By, append([]bool(nil), s.Offer.Votes...)}
	}
	for i := 0; i < s.Players; i++ {
		remaining := 0
		for _, p := range v.Pieces {
			if p.Seat == i {
				remaining++
			}
		}
		team := i
		if s.Players == 4 {
			team = i % 2
		}
		v.Players = append(v.Players, PlayerView{i, team, s.Ready[i], s.Alive[i], remaining, s.FlagRevealed[i]})
	}
	active := actor >= 0 && actor < s.Players && s.Alive[actor]
	if active {
		v.Revision = s.Revisions[actor]
		v.CanArrange = s.Phase == "deploy" && !s.Ready[actor]
		v.CanConfirm = v.CanArrange
		v.CanUnready = s.Phase == "deploy" && s.Ready[actor]
		v.CanMove = s.Phase == "playing" && s.Turn == actor
		v.CanResign = s.Phase == "playing"
		v.CanOffer = v.CanMove && s.Offer == nil
		v.CanVote = s.Phase == "playing" && s.Offer != nil && !s.Offer.Votes[actor]
		if v.CanMove {
			v.Legal = legalMoves(v.Graph, v.Pieces, actor)
		}
	}
	return v, nil
}
func start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var opts Options
	_ = json.Unmarshal(o, &opts)
	round, e := token()
	if e != nil {
		return gameapi.Match{}, e
	}
	s := State{Options: opts, Players: c.Players, Round: round, Phase: "deploy", Turn: 0, Winner: -1, Ready: make([]bool, c.Players), Alive: make([]bool, c.Players), Revisions: make([]int, c.Players), Deadlines: make([]int64, c.Players), FlagRevealed: make([]bool, c.Players), Pieces: []Piece{}, Keys: []string{}, Log: []string{"请在己方阵地布阵；所有玩家确认后开始"}, Last: Event{Moves: []Move{}, Removed: []string{}, Commanders: []int{}}}
	kinds := []string{}
	for k := range inventory {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for seat := 0; seat < c.Players; seat++ {
		s.Alive[seat] = true
		s.Deadlines[seat] = c.Now.Add(time.Duration(opts.SetupSeconds) * time.Second).UnixMilli()
		for _, kind := range kinds {
			for j := 0; j < inventory[kind]; j++ {
				id, e := token()
				if e != nil {
					return gameapi.Match{}, e
				}
				s.Pieces = append(s.Pieces, Piece{id, seat, kind, -1, true, false})
			}
		}
		if e = s.arrange(seat); e != nil {
			return gameapi.Match{}, e
		}
	}
	return s.envelope()
}
func (s *State) unit(id string) *Piece {
	for i := range s.Pieces {
		if s.Pieces[i].ID == id {
			return &s.Pieces[i]
		}
	}
	return nil
}
func (s *State) beginEvent(c gameapi.Context, kind string) {
	s.Last = Event{ID: s.Last.ID + 1, Type: kind, Seat: c.Actor, Moves: []Move{}, Removed: []string{}, Commanders: []int{}, At: c.Now.UnixMilli()}
}
func (s *State) remove(p *Piece, combat bool) {
	if !p.Alive {
		return
	}
	p.Alive, p.Pos = false, -1
	s.Last.Removed = append(s.Last.Removed, p.ID)
	if combat && p.Kind == "commander" {
		s.FlagRevealed[p.Seat] = true
		s.Last.Commanders = append(s.Last.Commanders, p.Seat)
		for i := range s.Pieces {
			if s.Pieces[i].Seat == p.Seat && s.Pieces[i].Kind == "flag" {
				s.Pieces[i].Revealed = true
			}
		}
		s.note(fmt.Sprintf("%s的司令阵亡，军旗公开", sideName(p.Seat)))
	}
}
func (s *State) eliminate(seat int, reason string) {
	if !s.Alive[seat] {
		return
	}
	s.Alive[seat] = false
	s.Deadlines[seat] = 0
	s.NoCasualty = 0
	s.Keys = nil
	for i := range s.Pieces {
		if s.Pieces[i].Seat == seat {
			s.remove(&s.Pieces[i], false)
		}
	}
	s.note(fmt.Sprintf("%s%s，出局并清除棋子", sideName(seat), reason))
}
func (s *State) checkWin() bool {
	winner := -1
	for i, alive := range s.Alive {
		if !alive {
			continue
		}
		if winner < 0 {
			winner = i
		} else if !allied(s.Players, i, winner) {
			return false
		}
	}
	s.finish(winner, "last_team")
	return true
}
func (s *State) advance(now time.Time, includeCurrent bool) {
	if s.checkWin() {
		return
	}
	if !includeCurrent {
		s.Turn = (s.Turn + 1) % s.Players
	}
	g := makeGraph(s.Players)
	for attempts := 0; attempts < s.Players; attempts++ {
		if s.Alive[s.Turn] {
			if len(legalMoves(g, projected(s, s.Turn), s.Turn)) > 0 {
				s.Deadlines[s.Turn] = now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
				return
			}
			s.eliminate(s.Turn, "没有合法走法")
			if s.checkWin() {
				return
			}
		}
		s.Turn = (s.Turn + 1) % s.Players
	}
}
func (s *State) remember() {
	s.Keys = append(s.Keys, PositionKey(s))
	if s.Phase != "playing" {
		return
	}
	if s.NoCasualty >= 200 {
		s.finish(-1, "no_casualty")
		return
	}
	same := 0
	for _, k := range s.Keys {
		if k == s.Keys[len(s.Keys)-1] {
			same++
		}
	}
	if same >= 4 {
		s.finish(-1, "repetition")
	}
}
