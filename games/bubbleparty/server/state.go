// Package bubbleparty implements a server-authoritative real-time team battle
// with water balloons, destructible scenery and rescue-based team combat.
package bubbleparty

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	Cols = 15
	Rows = 13

	FrameMS    = int64(100)
	StepMS     = int64(20)
	Countdown  = int64(3000)
	BombFuseMS = int64(2200)
	BlastMS    = int64(420)
	ShrinkMS   = int64(3000)
	IdleMS     = int64(60000)
	LostMS     = int64(700)
	DefaultMS  = int64(300000)

	// Provisional tuning pending runtime evidence from the original game.
	TrapMS             = int64(5000)
	RescueProtectionMS = int64(1000)

	ClassicRulesVersion = 2
	MaxPlayers          = 8
	MaxBombs            = 64
	MaxItems            = 40
	MaxEvents           = 64
	MaxLog              = 80
	MaxPending          = 32
	MaxActionSize       = 8 * 1024
	MaxSeq              = int64(1<<31 - 1)
	MaxSeqJump          = int64(2048)
	MaxBombCapacity     = 5
	MaxBombRange        = 6
	MinMoveMS           = 70
	BaseMoveMS          = 150
	BaseBombRange       = 2
	PlayerRadius        = 0.28
)

const (
	PhaseCountdown = "countdown"
	PhasePlaying   = "playing"
	PhaseFinished  = "finished"
)

const (
	CellEmpty = iota
	CellWall
	CellCrate
	CellBomb
)

const (
	ItemSpeed  = "speed"
	ItemBomb   = "bomb"
	ItemRange  = "range"
	ItemShield = "shield"
)

var directions = [][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

var directionNames = map[string]string{
	"up": "up", "down": "down", "left": "left", "right": "right", "none": "none",
}

type Options struct {
	Map          string `json:"map"`
	RoundSeconds int    `json:"roundSeconds"`
}

type Input struct {
	At   int64  `json:"at"`
	Move string `json:"move"`
	Bomb bool   `json:"bomb"`
}

const AvatarCount = 4

type Player struct {
	Avatar            int     `json:"avatar,omitempty"`
	Seat              int     `json:"seat"`
	Team              int     `json:"team"`
	X                 int     `json:"x"`
	Y                 int     `json:"y"`
	OffsetX           float64 `json:"offsetX,omitempty"`
	OffsetY           float64 `json:"offsetY,omitempty"`
	Dir               string  `json:"dir"`
	Alive             bool    `json:"alive"`
	TrappedUntil      int64   `json:"trappedUntil"`
	InvulnerableUntil int64   `json:"invulnerableUntil"`
	TrappedBy         int     `json:"trappedBy"`
	Shield            bool    `json:"shield"`
	SpeedMS           int     `json:"speedMs"`
	MaxBombs          int     `json:"maxBombs"`
	Range             int     `json:"range"`
	Eliminations      int     `json:"eliminations"`
	Crates            int     `json:"crates"`
	Seq               int64   `json:"seq"`
	LastInputAt       int64   `json:"lastInputAt"`
	IdleAt            int64   `json:"idleAt"`
	NextMoveAt        int64   `json:"nextMoveAt"`
	NextBombAt        int64   `json:"nextBombAt"`
	BombReq           bool    `json:"bombReq"`
	Applied           Input   `json:"applied"`
	Pending           []Input `json:"pending,omitempty"`
}

type Bomb struct {
	ID        int   `json:"id"`
	Seat      int   `json:"seat"`
	Team      int   `json:"team"`
	X         int   `json:"x"`
	Y         int   `json:"y"`
	Range     int   `json:"range"`
	At        int64 `json:"at"`
	ExplodeAt int64 `json:"explodeAt"`
}

type Blast struct {
	X     int   `json:"x"`
	Y     int   `json:"y"`
	Seat  int   `json:"seat"`
	At    int64 `json:"at"`
	EndAt int64 `json:"endAt"`
}

type Item struct {
	ID   int    `json:"id"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Type string `json:"type"`
	At   int64  `json:"at"`
}

type Event struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	At    int64  `json:"at"`
	Seat  int    `json:"seat"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Kind  string `json:"kind,omitempty"`
	Value int    `json:"value,omitempty"`
}

type State struct {
	RulesVersion int             `json:"rulesVersion,omitempty"`
	Schema       int             `json:"schema"`
	Options      Options         `json:"options"`
	Phase        string          `json:"phase"`
	Epoch        string          `json:"epoch"`
	Seed         uint64          `json:"seed"`
	ServerAt     int64           `json:"serverAt"`
	StartAt      int64           `json:"startAt"`
	MatchEndsAt  int64           `json:"matchEndsAt"`
	SuddenAt     int64           `json:"suddenAt"`
	Sudden       bool            `json:"sudden"`
	SafeInset    int             `json:"safeInset"`
	MapID        string          `json:"mapId"`
	Players      []Player        `json:"players"`
	TeamSize     int             `json:"teamSize"`
	Cells        []byte          `json:"cells"`
	Bombs        []Bomb          `json:"bombs"`
	Blasts       []Blast         `json:"blasts"`
	Items        []Item          `json:"items"`
	Events       []Event         `json:"events"`
	EventID      int             `json:"eventId"`
	BombID       int             `json:"bombId"`
	ItemID       int             `json:"itemId"`
	Log          []string        `json:"log"`
	Result       *gameapi.Result `json:"result,omitempty"`
}

type PlayerView struct {
	Avatar            int     `json:"avatar"`
	Seat              int     `json:"seat"`
	Seq               int64   `json:"seq"`
	Team              int     `json:"team"`
	X                 int     `json:"x"`
	Y                 int     `json:"y"`
	PX                float64 `json:"px"`
	PY                float64 `json:"py"`
	SpeedMS           int     `json:"speedMs"`
	Dir               string  `json:"dir"`
	Alive             bool    `json:"alive"`
	TrappedUntil      int64   `json:"trappedUntil"`
	InvulnerableUntil int64   `json:"invulnerableUntil"`
	Shield            bool    `json:"shield"`
	MaxBombs          int     `json:"maxBombs"`
	Range             int     `json:"range"`
	Eliminations      int     `json:"eliminations"`
	Crates            int     `json:"crates"`
	Applied           Input   `json:"applied"`
}

type View struct {
	RulesVersion int          `json:"rulesVersion"`
	Schema       int          `json:"schema"`
	Options      Options      `json:"options"`
	Phase        string       `json:"phase"`
	Epoch        string       `json:"epoch"`
	ServerAt     int64        `json:"serverAt"`
	StartAt      int64        `json:"startAt"`
	MatchEndsAt  int64        `json:"matchEndsAt"`
	SuddenAt     int64        `json:"suddenAt"`
	Sudden       bool         `json:"sudden"`
	SafeInset    int          `json:"safeInset"`
	MapID        string       `json:"mapId"`
	Players      []PlayerView `json:"players"`
	TeamSize     int          `json:"teamSize"`
	Cells        []int        `json:"cells"`
	Tiles        []int        `json:"tiles"`
	Bombs        []Bomb       `json:"bombs"`
	Blasts       []Blast      `json:"blasts"`
	Items        []Item       `json:"items"`
	Events       []Event      `json:"events"`
	Log          []string     `json:"log"`
	You          int          `json:"you"`
	CanAct       bool         `json:"canAct"`
	Spectator    bool         `json:"spectator"`
}

func strict(raw json.RawMessage, v any) error {
	if len(raw) > MaxActionSize || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players != 2 && players != 4 && players != 6 && players != 8 {
		return nil, errors.New("泡泡堂支持 2、4、6 或 8 人")
	}
	o := Options{Map: "community10", RoundSeconds: 300}
	if len(raw) > 0 {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效对局设置")
		}
	}
	if !validMap(o.Map) {
		return nil, errors.New("请选择有效地图")
	}
	if o.RoundSeconds < 180 || o.RoundSeconds > 300 {
		return nil, errors.New("单局时限需为 180～300 秒")
	}
	return json.Marshal(o)
}

func decode(m gameapi.Match) (*State, error) {
	var s State
	if m.Schema != 1 {
		return nil, errors.New("不支持的泡泡堂存档版本")
	}
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return nil, err
	}
	if s.RulesVersion != 0 && s.RulesVersion != ClassicRulesVersion {
		return nil, errors.New("不支持的泡泡堂规则版本")
	}
	if s.Schema != 1 || len(s.Players) == 0 || len(s.Players) > MaxPlayers || len(s.Players)%2 != 0 {
		return nil, errors.New("无效对局人数")
	}
	if s.TeamSize != len(s.Players)/2 || len(s.Players) != s.TeamSize*2 {
		return nil, errors.New("队伍数量无效")
	}
	if _, err := configure(len(s.Players), mustJSON(s.Options)); err != nil {
		return nil, err
	}
	if s.Phase != PhaseCountdown && s.Phase != PhasePlaying && s.Phase != PhaseFinished {
		return nil, errors.New("对局阶段无效")
	}
	if s.MapID == "" || !validMap(s.MapID) || s.Epoch == "" {
		return nil, errors.New("地图或 epoch 无效")
	}
	if len(s.Cells) != Cols*Rows {
		return nil, errors.New("地图状态长度无效")
	}
	for _, c := range s.Cells {
		if c > CellBomb {
			return nil, errors.New("地图状态无效")
		}
	}
	if len(s.Bombs) > MaxBombs || len(s.Items) > MaxItems || len(s.Events) > MaxEvents || len(s.Blasts) > MaxBombs*4 {
		return nil, errors.New("实体数量超出上限")
	}
	for i := range s.Players {
		p := &s.Players[i]
		if p.Avatar < 0 || p.Avatar >= AvatarCount {
			return nil, errors.New("人物造型无效")
		}
		if p.Seat != i || p.Team != i/s.TeamSize {
			return nil, errors.New("席位或队伍无效")
		}
		if p.X < 0 || p.X >= Cols || p.Y < 0 || p.Y >= Rows || math.Abs(p.OffsetX) > 0.500001 || math.Abs(p.OffsetY) > 0.500001 {
			return nil, errors.New("玩家位置无效")
		}
		if p.SpeedMS < MinMoveMS || p.SpeedMS > BaseMoveMS || p.MaxBombs < 1 || p.MaxBombs > MaxBombCapacity || p.Range < 1 || p.Range > MaxBombRange {
			return nil, errors.New("玩家属性无效")
		}
		if p.Seq < 0 || p.Seq > MaxSeq || len(p.Pending) > MaxPending {
			return nil, errors.New("输入状态无效")
		}
		if !p.Alive && s.Phase == PhasePlaying && s.Result == nil {
			// eliminated players are valid; the envelope carries them explicitly.
		}
	}
	if s.Phase == PhasePlaying {
		if s.StartAt <= 0 || s.MatchEndsAt <= s.StartAt || (s.RulesVersion != ClassicRulesVersion && (s.SuddenAt < s.StartAt || s.SuddenAt > s.MatchEndsAt)) || s.ServerAt < s.StartAt {
			return nil, errors.New("对局时钟无效")
		}
	}
	return &s, nil
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func validMap(id string) bool {
	if id == "random" {
		return true
	}
	for _, v := range mapIDs() {
		if v == id {
			return true
		}
	}
	return false
}

func randomMap(seed uint64) string {
	ids := mapIDs()
	return ids[seed%uint64(len(ids))]
}

func hash64(seed uint64, text string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	var buf [8]byte
	for i := range buf {
		buf[i] = byte(seed >> (8 * i))
	}
	_, _ = h.Write(buf[:])
	return h.Sum64()
}

func (s *State) match() (gameapi.Match, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	var appearance uint32
	for i, p := range s.Players {
		appearance |= uint32(p.Avatar) << uint(i*2)
	}
	m := gameapi.Match{CheckpointKey: fmt.Sprintf("%s:%x", s.Phase, appearance), Schema: 1, Data: data, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, Log: append([]string(nil), s.Log...), Result: s.Result}
	if s.Result != nil {
		return m, nil
	}
	switch s.Phase {
	case PhaseCountdown:
		for i := range s.Players {
			m.Actors = append(m.Actors, i)
		}
		m.SystemDeadline = s.StartAt
	case PhasePlaying:
		for i := range s.Players {
			p := &s.Players[i]
			if !p.Alive {
				m.Eliminated = append(m.Eliminated, i)
				continue
			}
			m.Actors = append(m.Actors, i)
			if m.Deadline == 0 || p.IdleAt < m.Deadline {
				m.Deadline = p.IdleAt
			}
		}
		m.Attention = []int{}
		for i := range s.Players {
			if s.Players[i].Alive && s.Players[i].IdleAt == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, i)
			}
		}
		m.SystemDeadline = s.ServerAt + FrameMS
		if s.SuddenAt > s.ServerAt && s.SuddenAt < m.SystemDeadline {
			m.SystemDeadline = s.SuddenAt
		}
		if s.MatchEndsAt < m.SystemDeadline {
			m.SystemDeadline = s.MatchEndsAt
		}
	}
	return m, nil
}

func (s *State) project(seat int) View {
	v := View{
		RulesVersion: s.RulesVersion, Schema: 1, Options: s.Options, Phase: s.Phase, Epoch: s.Epoch,
		ServerAt: s.ServerAt, StartAt: s.StartAt, MatchEndsAt: s.MatchEndsAt,
		SuddenAt: s.SuddenAt, Sudden: s.Sudden, SafeInset: s.SafeInset,
		MapID: s.MapID, TeamSize: s.TeamSize, Cells: make([]int, len(s.Cells)), Tiles: mapTiles(s.MapID, s.Cells),
		Bombs: append([]Bomb{}, s.Bombs...), Blasts: append([]Blast{}, s.Blasts...),
		Items: append([]Item{}, s.Items...), Events: append([]Event{}, s.Events...),
		Log: append([]string{}, s.Log...), Players: []PlayerView{}, You: seat, Spectator: seat < 0 || seat >= len(s.Players),
	}
	for i, c := range s.Cells {
		v.Cells[i] = int(c)
	}
	for i := range s.Players {
		p := &s.Players[i]
		v.Players = append(v.Players, PlayerView{
			Avatar: p.Avatar, Seat: p.Seat, Seq: p.Seq, Team: p.Team, X: p.X, Y: p.Y, Dir: p.Dir, Alive: p.Alive,
			PX: float64(p.X) + p.OffsetX, PY: float64(p.Y) + p.OffsetY, SpeedMS: p.SpeedMS,
			TrappedUntil: p.TrappedUntil, InvulnerableUntil: p.InvulnerableUntil,
			Shield: p.Shield, MaxBombs: p.MaxBombs, Range: p.Range,
			Eliminations: p.Eliminations, Crates: p.Crates, Applied: p.Applied,
		})
	}
	v.CanAct = s.Phase == PhasePlaying && seat >= 0 && seat < len(s.Players) && s.Players[seat].Alive && s.Result == nil
	return v
}

func (s *State) log(text string) {
	s.Log = append(s.Log, text)
	if len(s.Log) > MaxLog {
		s.Log = append([]string(nil), s.Log[len(s.Log)-MaxLog:]...)
	}
}

func (s *State) emit(kind string, seat, x, y int, extra string, value int, at int64) {
	s.EventID++
	s.Events = append(s.Events, Event{ID: s.EventID, Type: kind, At: at, Seat: seat, X: x, Y: y, Kind: extra, Value: value})
	if len(s.Events) > MaxEvents {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-MaxEvents:]...)
	}
}

func (s *State) cell(x, y int) byte {
	if x < 0 || x >= Cols || y < 0 || y >= Rows {
		return CellWall
	}
	return s.Cells[y*Cols+x]
}

func (s *State) setCell(x, y int, v byte) {
	if x < 0 || x >= Cols || y < 0 || y >= Rows {
		return
	}
	s.Cells[y*Cols+x] = v
}

func (s *State) player(seat int) *Player {
	if seat < 0 || seat >= len(s.Players) {
		return nil
	}
	return &s.Players[seat]
}

func (s *State) bombAt(x, y int) *Bomb {
	for i := range s.Bombs {
		if s.Bombs[i].X == x && s.Bombs[i].Y == y {
			return &s.Bombs[i]
		}
	}
	return nil
}

func (s *State) canWalk(p *Player, x, y int) bool {
	if x < 0 || x >= Cols || y < 0 || y >= Rows {
		return false
	}
	c := s.cell(x, y)
	if c == CellWall || c == CellCrate {
		return false
	}
	if c == CellBomb && !(p.X == x && p.Y == y) {
		return false
	}
	return true
}

func (s *State) placeBomb(p *Player, at int64) bool {
	if p == nil || !p.Alive || p.TrappedUntil > 0 || p.MaxBombs <= 0 {
		return false
	}
	if s.cell(p.X, p.Y) == CellWall || s.cell(p.X, p.Y) == CellCrate || s.cell(p.X, p.Y) == CellBomb {
		return false
	}
	n := 0
	for i := range s.Bombs {
		if s.Bombs[i].Seat == p.Seat {
			n++
		}
	}
	if n >= p.MaxBombs {
		return false
	}
	s.BombID++
	b := Bomb{ID: s.BombID, Seat: p.Seat, Team: p.Team, X: p.X, Y: p.Y, Range: p.Range, At: at, ExplodeAt: at + BombFuseMS}
	s.Bombs = append(s.Bombs, b)
	s.setCell(p.X, p.Y, CellBomb)
	p.BombReq = false
	p.NextBombAt = at + 280
	s.emit("place", p.Seat, p.X, p.Y, "", b.ID, at)
	return true
}

func (s *State) pickupItems(at int64) {
	for i := 0; i < len(s.Players); i++ {
		p := &s.Players[i]
		if !p.Alive || p.TrappedUntil > 0 {
			continue
		}
		for j := 0; j < len(s.Items); j++ {
			it := s.Items[j]
			if it.X != p.X || it.Y != p.Y {
				continue
			}
			switch it.Type {
			case ItemSpeed:
				p.SpeedMS -= 12
				if p.SpeedMS < MinMoveMS {
					p.SpeedMS = MinMoveMS
				}
			case ItemBomb:
				if p.MaxBombs < MaxBombCapacity {
					p.MaxBombs++
				}
			case ItemRange:
				if p.Range < MaxBombRange {
					p.Range++
				}
			case ItemShield:
				p.Shield = true
			}
			s.Items = append(s.Items[:j], s.Items[j+1:]...)
			j--
			s.emit("item", p.Seat, p.X, p.Y, it.Type, it.ID, at)
		}
	}
}

func (s *State) eliminate(p *Player, at int64, by int) {
	if p == nil || !p.Alive {
		return
	}
	if s.RulesVersion == 0 && p.Shield {
		p.Shield = false
		s.emit("shield", p.Seat, p.X, p.Y, "", by, at)
		return
	}
	p.Alive = false
	p.TrappedUntil = 0
	p.Applied = Input{At: at, Move: "none"}
	p.Pending = nil
	p.BombReq = false
	s.emit("eliminate", p.Seat, p.X, p.Y, "", by, at)
	if killer := s.player(by); killer != nil && killer.Team != p.Team {
		killer.Eliminations++
	}
	s.log(fmt.Sprintf("%d 号被水泡淘汰", p.Seat+1))
}

func stopPlayerControls(p *Player, at int64) {
	p.Applied = Input{At: at, Move: "none"}
	p.Pending = nil
	p.BombReq = false
}

// Classic water traps first. Existing version-0 saves retain direct damage.
func (s *State) hitWater(p *Player, at int64, by int) {
	if s.RulesVersion == 0 {
		s.eliminate(p, at, by)
		return
	}
	if p == nil || !p.Alive || p.TrappedUntil > 0 || p.InvulnerableUntil > at {
		return
	}
	if p.Shield {
		p.Shield = false
		p.InvulnerableUntil = at + RescueProtectionMS
		s.emit("shield", p.Seat, p.X, p.Y, "", by, at)
		return
	}
	p.TrappedUntil = at + TrapMS
	p.TrappedBy = by
	stopPlayerControls(p, at)
	s.emit("trap", p.Seat, p.X, p.Y, "", by, at)
}

func (s *State) resolveWaterAndContacts(at int64) {
	// Legacy damage is applied once by blastCell when the water is created.
	// Rechecking it every substep would consume an old shield twice.
	if s.RulesVersion == 0 {
		return
	}
	for i := range s.Players {
		p := &s.Players[i]
		if !p.Alive {
			continue
		}
		if p.TrappedUntil > 0 && p.TrappedUntil <= at {
			s.eliminate(p, at, p.TrappedBy)
			continue
		}
		for _, b := range s.Blasts {
			if b.EndAt > at && p.X == b.X && p.Y == b.Y {
				s.hitWater(p, at, b.Seat)
			}
		}
	}
	for i := range s.Players {
		p := &s.Players[i]
		if !p.Alive || p.TrappedUntil == 0 {
			continue
		}
		for j := range s.Players {
			q := &s.Players[j]
			if i == j || !q.Alive || q.TrappedUntil > 0 || math.Hypot(float64(q.X-p.X)+q.OffsetX-p.OffsetX, float64(q.Y-p.Y)+q.OffsetY-p.OffsetY) > 2*PlayerRadius {
				continue
			}
			if q.Team != p.Team {
				s.eliminate(p, at, q.Seat)
			} else {
				p.TrappedUntil = 0
				p.InvulnerableUntil = at + RescueProtectionMS
				stopPlayerControls(p, at)
				s.emit("rescue", p.Seat, p.X, p.Y, "", q.Seat, at)
			}
			break
		}
	}
}

func (s *State) blastCell(x, y int, seat int, at int64) {
	for i := range s.Blasts {
		if s.Blasts[i].X == x && s.Blasts[i].Y == y && s.Blasts[i].EndAt > at {
			return
		}
	}
	s.Blasts = append(s.Blasts, Blast{X: x, Y: y, Seat: seat, At: at, EndAt: at + BlastMS})
	for i := range s.Players {
		p := &s.Players[i]
		if p.Alive && p.X == x && p.Y == y {
			s.hitWater(p, at, seat)
		}
	}
}

func (s *State) explodeBombs(at int64) {
	for {
		idx := -1
		for i := range s.Bombs {
			if s.Bombs[i].ExplodeAt <= at {
				idx = i
				break
			}
		}
		if idx < 0 {
			return
		}
		b := s.Bombs[idx]
		s.Bombs = append(s.Bombs[:idx], s.Bombs[idx+1:]...)
		if s.cell(b.X, b.Y) == CellBomb {
			s.setCell(b.X, b.Y, CellEmpty)
		}
		s.emit("explode", b.Seat, b.X, b.Y, "", b.Range, at)
		s.blastCell(b.X, b.Y, b.Seat, at)
		for _, d := range directions {
			for step := 1; step <= b.Range; step++ {
				x, y := b.X+d[0]*step, b.Y+d[1]*step
				c := s.cell(x, y)
				if c == CellWall {
					break
				}
				if c == CellCrate {
					s.setCell(x, y, CellEmpty)
					if p := s.player(b.Seat); p != nil {
						p.Crates++
					}
					if hash64(s.Seed+uint64(at), fmt.Sprintf("drop:%d:%d:%d", b.X, b.Y, step))%100 < 58 {
						s.spawnItem(x, y, at, b.Seat)
					}
					break
				}
				if c == CellBomb {
					if other := s.bombAt(x, y); other != nil {
						other.ExplodeAt = at
					}
					break
				}
				s.blastCell(x, y, b.Seat, at)
			}
		}
	}
}

func (s *State) spawnItem(x, y int, at int64, seat int) {
	if len(s.Items) >= MaxItems {
		return
	}
	types := []string{ItemSpeed, ItemBomb, ItemRange, ItemShield}
	s.ItemID++
	t := types[int(hash64(s.Seed+uint64(at)+uint64(seat), fmt.Sprintf("item:%d:%d:%d", x, y, s.ItemID))%uint64(len(types)))]
	s.Items = append(s.Items, Item{ID: s.ItemID, X: x, Y: y, Type: t, At: at})
	s.emit("drop", seat, x, y, t, s.ItemID, at)
}

func (s *State) stepPlayer(p *Player, at int64) {
	if !p.Alive {
		return
	}
	for len(p.Pending) > 0 && p.Pending[0].At <= at {
		p.Applied = p.Pending[0]
		p.Pending = p.Pending[1:]
	}
	if at-p.LastInputAt > LostMS {
		p.Applied.Move = "none"
	}
	if p.TrappedUntil > 0 {
		stopPlayerControls(p, at)
		return
	}
	if p.Applied.Bomb {
		p.BombReq = true
		p.Applied.Bomb = false
	}
	if p.BombReq {
		// Bomb is an edge, not a promise to place later when capacity/cooldown
		// eventually permits it. Every edge gets exactly one attempt.
		p.BombReq = false
		if at >= p.NextBombAt {
			s.placeBomb(p, at)
		}
	}
	move := p.Applied.Move
	if move == "" {
		move = "none"
	}
	if move != "none" {
		dx, dy := 0, 0
		switch move {
		case "up":
			dy = -1
		case "down":
			dy = 1
		case "left":
			dx = -1
		case "right":
			dx = 1
		}
		if dx != 0 || dy != 0 {
			if s.RulesVersion == 0 {
				s.movePlayerLegacy(p, dx, dy, at, move)
			} else {
				s.movePlayer(p, dx, dy, float64(StepMS)/float64(p.SpeedMS), at, move)
			}
		}
	}

}

func (s *State) movePlayerLegacy(p *Player, dx, dy int, at int64, dir string) {
	if at < p.NextMoveAt {
		return
	}
	x, y := p.X+dx, p.Y+dy
	if s.canWalk(p, x, y) {
		// Preserve a balloon's occupancy after departure (common collision fix).
		p.X, p.Y = x, y
		p.Dir = dir
		p.NextMoveAt = at + int64(p.SpeedMS)
		s.emit("move", p.Seat, x, y, dir, 0, at)
	} else {
		p.NextMoveAt = at + 50
	}
}

// Move in grid units, using relative offsets so schema-1 saves lacking them
// naturally restore at the old integer cell center.
func (s *State) movePlayer(p *Player, dx, dy int, distance float64, at int64, dir string) {
	px, py := float64(p.X)+p.OffsetX, float64(p.Y)+p.OffsetY
	oldX, oldY := p.X, p.Y
	// A turn first reaches the current row/column center; this keeps a round
	// body from cutting a solid corner and preserves the distance budget.
	if dx != 0 && math.Abs(p.OffsetY) > 0.000001 {
		target := py + math.Copysign(math.Min(distance, math.Abs(p.OffsetY)), -p.OffsetY)
		next := s.moveAxis(p, px, py, target, false)
		distance -= math.Abs(next - py)
		py = next
		if math.Abs(py-float64(p.Y)) > 0.000001 {
			distance = 0
		}
	}
	if dy != 0 && math.Abs(p.OffsetX) > 0.000001 {
		target := px + math.Copysign(math.Min(distance, math.Abs(p.OffsetX)), -p.OffsetX)
		next := s.moveAxis(p, px, py, target, true)
		distance -= math.Abs(next - px)
		px = next
		if math.Abs(px-float64(p.X)) > 0.000001 {
			distance = 0
		}
	}
	if distance > 0 {
		if dx != 0 {
			px = s.moveAxis(p, px, py, px+float64(dx)*distance, true)
		}
		if dy != 0 {
			py = s.moveAxis(p, px, py, py+float64(dy)*distance, false)
		}
	}
	p.X, p.Y = int(math.Floor(px+0.5)), int(math.Floor(py+0.5))
	p.OffsetX, p.OffsetY = px-float64(p.X), py-float64(p.Y)
	if math.Abs(p.OffsetX) < 0.000001 {
		p.OffsetX = 0
	}
	if math.Abs(p.OffsetY) < 0.000001 {
		p.OffsetY = 0
	}
	p.Dir = dir
	if p.X != oldX || p.Y != oldY {
		s.emit("move", p.Seat, p.X, p.Y, dir, 0, at)
	}
}

func (s *State) moveAxis(p *Player, px, py, target float64, horizontal bool) float64 {
	old, across := px, py
	if !horizontal {
		old, across = py, px
	}
	if math.Abs(target-old) < 0.000001 {
		return old
	}
	positive := target > old
	for y := p.Y - 1; y <= p.Y+1; y++ {
		for x := p.X - 1; x <= p.X+1; x++ {
			c := s.cell(x, y)
			if c != CellWall && c != CellCrate && c != CellBomb {
				continue
			}
			center, cross := float64(x), float64(y)
			if !horizontal {
				center, cross = float64(y), float64(x)
			}
			if math.Abs(across-cross) >= 0.5+PlayerRadius-0.000001 {
				continue
			}
			low, high := center-0.5-PlayerRadius, center+0.5+PlayerRadius
			if old > low+0.000001 && old < high-0.000001 {
				// Players may choose any exit while still in the original balloon
				// cell. After crossing its cell boundary they may only move away.
				if c == CellBomb && (p.X == x && p.Y == y || positive && old >= center || !positive && old <= center) {
					continue
				}
				if target > low && target < high {
					target = old
				}
				continue
			}
			if positive && old <= low+0.000001 && target > low {
				target = low
			}
			if !positive && old >= high-0.000001 && target < high {
				target = high
			}
		}
	}
	return target
}

func (s *State) shrinkZone(at int64) {
	if s.RulesVersion == ClassicRulesVersion {
		return
	}
	if !s.Sudden && at >= s.SuddenAt {
		s.Sudden = true
		s.emit("sudden", -1, 0, 0, "", 0, at)
		s.log("突然死亡：安全区开始收缩")
	}
	if !s.Sudden {
		return
	}
	want := 1 + int((at-s.SuddenAt)/ShrinkMS)
	maxInset := (minInt(Cols, Rows)-1)/2 + 1
	if want > maxInset {
		want = maxInset
	}
	if want <= s.SafeInset {
		return
	}
	s.SafeInset = want
	for i := range s.Players {
		p := &s.Players[i]
		if !p.Alive {
			continue
		}
		if p.X < s.SafeInset || p.Y < s.SafeInset || p.X >= Cols-s.SafeInset || p.Y >= Rows-s.SafeInset {
			s.eliminate(p, at, -1)
		}
	}
	s.emit("shrink", -1, 0, 0, "", s.SafeInset, at)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *State) step(at int64) {
	for i := range s.Players {
		s.stepPlayer(&s.Players[i], at)
	}
	s.explodeBombs(at)
	for i := 0; i < len(s.Blasts); {
		if s.Blasts[i].EndAt <= at {
			s.Blasts = append(s.Blasts[:i], s.Blasts[i+1:]...)
			continue
		}
		i++
	}
	s.resolveWaterAndContacts(at)
	s.pickupItems(at)
	s.shrinkZone(at)
}

func (s *State) aliveCount(team int) int {
	n := 0
	for i := range s.Players {
		if s.Players[i].Alive && s.Players[i].Team == team {
			n++
		}
	}
	return n
}

func (s *State) evaluate(at int64) {
	if s.Result != nil {
		return
	}
	a, b := s.aliveCount(0), s.aliveCount(1)
	if a == 0 && b == 0 {
		s.finish(true, -1, "双方同时全灭")
		return
	}
	if a == 0 {
		s.finish(false, 1, "红队全灭")
		return
	}
	if b == 0 {
		s.finish(false, 0, "蓝队全灭")
		return
	}
	if at >= s.MatchEndsAt {
		if s.RulesVersion == ClassicRulesVersion {
			s.finish(true, -1, "时间到，双方仍有队员存活")
			return
		}
		switch {
		case a > b:
			s.finish(false, 0, "时间到，红队存活人数更多")
		case b > a:
			s.finish(false, 1, "时间到，蓝队存活人数更多")
		default:
			s.finish(true, -1, "时间到，双方存活人数相同")
		}
	}
}

func (s *State) finish(draw bool, winner int, reason string) {
	if s.Result != nil {
		return
	}
	s.Phase = PhaseFinished
	r := &gameapi.Result{Team: true, Ranks: make([]int, len(s.Players)), Winners: []int{}, Scores: make([]int, len(s.Players))}
	for i := range s.Players {
		r.Scores[i] = s.Players[i].Eliminations*100 + s.Players[i].Crates*10
	}
	if draw {
		r.Draw = true
		for i := range r.Ranks {
			r.Ranks[i] = 1
		}
	} else {
		for i := range s.Players {
			if s.Players[i].Team == winner {
				r.Ranks[i] = 1
				r.Winners = append(r.Winners, i)
			} else {
				r.Ranks[i] = 2
			}
		}
	}
	r.Details, _ = json.Marshal(map[string]any{
		"map": s.MapID, "reason": reason, "winner": winner, "draw": draw,
		"players": s.Players,
	})
	s.Result = r
	s.Bombs = nil
	s.Blasts = nil
	s.emit("finish", -1, 0, 0, reason, winner, s.ServerAt)
	s.log("对局结束：" + reason)
}

func (s *State) beginPlaying(at int64) {
	if s.Phase != PhaseCountdown {
		return
	}
	s.Phase = PhasePlaying
	s.StartAt = at
	s.ServerAt = at
	seconds := int64(s.Options.RoundSeconds)
	if seconds <= 0 {
		seconds = 300
	}
	s.MatchEndsAt = at + seconds*1000
	s.SuddenAt = at + seconds*600
	if s.RulesVersion == ClassicRulesVersion {
		s.SuddenAt = 0
	}
	for i := range s.Players {
		p := &s.Players[i]
		p.Alive = true
		p.Applied = Input{At: at, Move: "none"}
		p.Pending = nil
		p.LastInputAt = at
		p.IdleAt = at + IdleMS
		p.NextMoveAt = at
		p.NextBombAt = at
		p.BombReq = false
	}
	s.emit("start", -1, 0, 0, "", 0, at)
	s.log("水泡大战开始")
}

func (s *State) advance(until int64) {
	if s.Result != nil {
		return
	}
	if s.Phase == PhaseCountdown {
		if until < s.StartAt {
			return
		}
		s.beginPlaying(s.StartAt)
	}
	if s.Phase != PhasePlaying {
		return
	}
	if until > s.MatchEndsAt {
		until = s.MatchEndsAt
	}
	if s.RulesVersion == 0 {
		// Keep the old frame cadence and saved NextMoveAt contract intact.
		for s.Result == nil && s.ServerAt+FrameMS <= until {
			for step := int64(0); step < FrameMS/StepMS; step++ {
				at := s.ServerAt + StepMS
				s.step(at)
				s.ServerAt = at
			}
			s.evaluate(s.ServerAt)
		}
	} else {
		// Classic inputs may arrive between snapshot deadlines. Advancing each
		// complete substep prevents a short press/release collapsing to no motion.
		for s.Result == nil && s.ServerAt+StepMS <= until {
			at := s.ServerAt + StepMS
			s.step(at)
			s.ServerAt = at
			s.evaluate(at)
		}
	}

	if s.Result == nil && (s.ServerAt >= s.MatchEndsAt || until >= s.MatchEndsAt) {
		s.evaluate(s.MatchEndsAt)
	}
}

func (s *State) onTimer(now int64) {
	if s.Result != nil || s.Phase == PhaseFinished {
		return
	}
	switch s.Phase {
	case PhaseCountdown:
		if now >= s.StartAt {
			s.beginPlaying(s.StartAt)
			s.advance(now)
		}
	case PhasePlaying:
		target := now
		if target > s.MatchEndsAt {
			target = s.MatchEndsAt
		}
		if target > s.ServerAt {
			s.advance(target)
		}
		if s.Result == nil && now >= s.MatchEndsAt {
			s.evaluate(s.MatchEndsAt)
		}
	}
}

func (s *State) applyInput(p *Player, a Action, now int64) {
	p.Seq = a.Seq
	if p.TrappedUntil > 0 {
		p.LastInputAt = now
		p.IdleAt = now + IdleMS
		stopPlayerControls(p, now)
		return
	}
	in := Input{At: now, Move: a.Move, Bomb: a.Bomb}
	if len(p.Pending) > 0 && p.Pending[len(p.Pending)-1].At == now {
		p.Pending[len(p.Pending)-1] = in
	} else {
		p.Pending = append(p.Pending, in)
	}
	if len(p.Pending) > MaxPending {
		p.Pending = p.Pending[len(p.Pending)-MaxPending:]
	}
	p.LastInputAt = now
	p.IdleAt = now + IdleMS
	if a.Bomb {
		p.BombReq = true
	}
}
