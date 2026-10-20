package spf2t

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const (
	SchemaVersion = 1

	Cols = 6
	Rows = 12

	FrameMS        int64 = 100
	SelectMS       int64 = 15000
	CountdownMS    int64 = 3000
	BreakMS        int64 = 2500
	IdleMS         int64 = 30000
	FallMS         int64 = 600
	SoftDropMS     int64 = 50
	LockDelayMS    int64 = 350
	ResolveStepMS  int64 = 240
	MaxLockResets        = 10
	MaxBatchEvents       = 16
	MaxActionBytes       = 8192
	MaxSeqJump     int64 = 1024
	MaxSeq         int64 = 1<<31 - 1
	MaxCatchUp           = 20
	MaxPending           = 144
	BestOf3CapMS   int64 = 8 * 60 * 1000
	SingleCapMS    int64 = 4 * 60 * 1000
)

const (
	ColorRed    = "r"
	ColorYellow = "y"
	ColorGreen  = "g"
	ColorBlue   = "b"
)

const (
	GemNormal  = "normal"
	GemCrash   = "crash"
	GemPower   = "power"
	GemCounter = "counter"
	GemDiamond = "diamond"
)

const (
	PhaseSelect     = "select"
	PhaseCountdown  = "countdown"
	PhasePlaying    = "playing"
	PhaseRoundBreak = "round_break"
	PhaseFinished   = "finished"
)

const (
	FormatSingle  = "single"
	FormatBestOf3 = "bestOf3"
)

type Options struct {
	Format string `json:"format"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type GemSpec struct {
	Kind  string `json:"kind"`
	Color string `json:"color,omitempty"`
}

type Pair struct {
	ID  int     `json:"id"`
	X   int     `json:"x"`
	Y   int     `json:"y"`
	Rot int     `json:"rot"`
	A   GemSpec `json:"a"`
	B   GemSpec `json:"b"`
}

type PairCell struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Kind  string `json:"kind"`
	Color string `json:"color,omitempty"`
}

// pairCellsAt returns the pivot (A) and satellite (B) cells. Rotation 0 is
// vertical with B above A, which matches the arcade spawn at the top center.
func pairCellsAt(x, y, rot int, a, b GemSpec) [2]PairCell {
	switch ((rot % 4) + 4) % 4 {
	case 1:
		return [2]PairCell{{X: x, Y: y, Kind: a.Kind, Color: a.Color}, {X: x + 1, Y: y, Kind: b.Kind, Color: b.Color}}
	case 2:
		return [2]PairCell{{X: x, Y: y, Kind: a.Kind, Color: a.Color}, {X: x, Y: y + 1, Kind: b.Kind, Color: b.Color}}
	case 3:
		return [2]PairCell{{X: x, Y: y, Kind: a.Kind, Color: a.Color}, {X: x - 1, Y: y, Kind: b.Kind, Color: b.Color}}
	default:
		return [2]PairCell{{X: x, Y: y, Kind: a.Kind, Color: a.Color}, {X: x, Y: y - 1, Kind: b.Kind, Color: b.Color}}
	}
}

func pairCells(p Pair) [2]PairCell {
	return pairCellsAt(p.X, p.Y, p.Rot, p.A, p.B)
}

type Gem struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Color string `json:"color,omitempty"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	Timer int    `json:"timer,omitempty"`
}

func (g Gem) cells() []Point {
	if g.W <= 0 || g.H <= 0 {
		return []Point{{X: g.X, Y: g.Y}}
	}
	out := make([]Point, 0, g.W*g.H)
	for y := g.Y; y < g.Y+g.H; y++ {
		for x := g.X; x < g.X+g.W; x++ {
			out = append(out, Point{X: x, Y: y})
		}
	}
	return out
}

func (g Gem) bottomCells() []Point {
	out := make([]Point, 0, g.W)
	for x := g.X; x < g.X+g.W; x++ {
		out = append(out, Point{X: x, Y: g.Y + g.H - 1})
	}
	return out
}

type CounterSpec struct {
	Color  string `json:"color"`
	Column int    `json:"column"`
	Timer  int    `json:"timer"`
}

type Resolve struct {
	Chain       int    `json:"chain"`
	Step        int    `json:"step"`
	NextAt      int64  `json:"nextAt"`
	Diamond     bool   `json:"diamond,omitempty"`
	Target      string `json:"target,omitempty"`
	DiamondX    int    `json:"diamondX,omitempty"`
	DiamondY    int    `json:"diamondY,omitempty"`
	Accum       int    `json:"accum"`
	DiamondPart int    `json:"diamondPart,omitempty"`
	Cleared     int    `json:"cleared,omitempty"`
}

type Player struct {
	Seat        int           `json:"seat"`
	Character   string        `json:"character,omitempty"`
	Selected    bool          `json:"selected"`
	RoundWins   int           `json:"roundWins"`
	Gems        []Gem         `json:"gems,omitempty"`
	NextID      int           `json:"nextId"`
	PairID      int           `json:"pairId"`
	Active      *Pair         `json:"active,omitempty"`
	Next        Pair          `json:"next"`
	Rand        uint64        `json:"rand"`
	PairCount   int           `json:"pairCount"`
	Resolve     *Resolve      `json:"resolve,omitempty"`
	Pending     []CounterSpec `json:"pending,omitempty"`
	PatternIdx  int           `json:"patternIndex"`
	Seq         int64         `json:"seq"`
	IdleAt      int64         `json:"idleAt"`
	SpawnedAt   int64         `json:"spawnedAt"`
	FallAt      int64         `json:"fallAt"`
	LockAt      int64         `json:"lockAt,omitempty"`
	LockResets  int           `json:"lockResets,omitempty"`
	Lost        bool          `json:"lost,omitempty"`
	GemsCleared int           `json:"gemsCleared,omitempty"`
	MaxChain    int           `json:"maxChain,omitempty"`
}

type Event struct {
	ID        int     `json:"id"`
	Type      string  `json:"type"`
	At        int64   `json:"at"`
	Seat      int     `json:"seat"`
	Target    int     `json:"target,omitempty"`
	Chain     int     `json:"chain,omitempty"`
	Count     int     `json:"count,omitempty"`
	Attack    int     `json:"attack,omitempty"`
	Kind      string  `json:"kind,omitempty"`
	Color     string  `json:"color,omitempty"`
	Character string  `json:"character,omitempty"`
	Cells     []Point `json:"cells,omitempty"`
}

type State struct {
	Options      Options         `json:"options"`
	Phase        string          `json:"phase"`
	Epoch        string          `json:"epoch"`
	Seed         uint64          `json:"seed"`
	ServerAt     int64           `json:"serverAt"`
	SelectUntil  int64           `json:"selectUntil,omitempty"`
	StartAt      int64           `json:"startAt,omitempty"`
	MatchEndsAt  int64           `json:"matchEndsAt,omitempty"`
	BreakUntil   int64           `json:"breakUntil,omitempty"`
	Round        int             `json:"round"`
	Players      []Player        `json:"players"`
	Events       []Event         `json:"events"`
	EventID      int             `json:"eventId"`
	Log          []string        `json:"log"`
	Result       *gameapi.Result `json:"result,omitempty"`
	FinishReason string          `json:"finishReason,omitempty"`
}

type CharacterView struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Pattern []string `json:"pattern"`
}

type GemView struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Color string `json:"color,omitempty"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	Timer int    `json:"timer,omitempty"`
}

type PairView struct {
	ID    int        `json:"id"`
	X     int        `json:"x"`
	Y     int        `json:"y"`
	Rot   int        `json:"rot"`
	Cells []PairCell `json:"cells"`
	Ghost []Point    `json:"ghost,omitempty"`
}

type PairSpecView struct {
	A GemSpec `json:"a"`
	B GemSpec `json:"b"`
}

type PlayerView struct {
	Seat        int          `json:"seat"`
	Character   string       `json:"character,omitempty"`
	Selected    bool         `json:"selected"`
	RoundWins   int          `json:"roundWins"`
	Gems        []GemView    `json:"gems"`
	Active      *PairView    `json:"active,omitempty"`
	Next        PairSpecView `json:"next"`
	Pending     int          `json:"pendingCount"`
	Warning     string       `json:"warning,omitempty"`
	Resolving   bool         `json:"resolving"`
	Lost        bool         `json:"lost"`
	Pattern     []string     `json:"pattern,omitempty"`
	GemsCleared int          `json:"gemsCleared"`
	MaxChain    int          `json:"maxChain"`
	Seq         int64        `json:"seq,omitempty"`
}

type Controls struct {
	CanPlay   bool `json:"canPlay"`
	CanSelect bool `json:"canSelect"`
}

type View struct {
	Options     Options         `json:"options"`
	Phase       string          `json:"phase"`
	Epoch       string          `json:"epoch"`
	ServerAt    int64           `json:"serverAt"`
	StartAt     int64           `json:"startAt"`
	MatchEndsAt int64           `json:"matchEndsAt"`
	SelectUntil int64           `json:"selectUntil"`
	BreakUntil  int64           `json:"breakUntil"`
	Round       int             `json:"round"`
	RoundsToWin int             `json:"roundsToWin"`
	Players     []PlayerView    `json:"players"`
	Events      []Event         `json:"events"`
	Characters  []CharacterView `json:"characters"`
	Controls    Controls        `json:"controls"`
}

func roundsToWin(format string) int {
	if format == FormatSingle {
		return 1
	}
	return 2
}

func matchCap(format string) int64 {
	if format == FormatSingle {
		return SingleCapMS
	}
	return BestOf3CapMS
}

func validColor(color string) bool {
	return color == ColorRed || color == ColorYellow || color == ColorGreen || color == ColorBlue
}

func validGemKind(kind string) bool {
	switch kind {
	case GemNormal, GemCrash, GemPower, GemCounter, GemDiamond:
		return true
	}
	return false
}

func validSpec(spec GemSpec) bool {
	if spec.Kind == GemDiamond {
		return spec.Color == ""
	}
	if spec.Kind != GemNormal && spec.Kind != GemCrash {
		return false
	}
	return validColor(spec.Color)
}

// gemGrid builds the occupancy grid from the authoritative gem entities. The
// caller must have validated that cells are in bounds and do not overlap.
func gemGrid(gems []Gem) ([Rows][Cols]int, error) {
	var grid [Rows][Cols]int
	for _, g := range gems {
		for _, c := range g.cells() {
			if c.X < 0 || c.X >= Cols || c.Y < 0 || c.Y >= Rows {
				return grid, fmt.Errorf("宝石越界")
			}
			if grid[c.Y][c.X] != 0 {
				return grid, fmt.Errorf("宝石重叠")
			}
			grid[c.Y][c.X] = g.ID
		}
	}
	return grid, nil
}

func (s *State) decode(m gameapi.Match) error {
	if m.Schema != SchemaVersion {
		return errors.New("不支持的街霸方块存档版本")
	}
	var st State
	if err := json.Unmarshal(m.Data, &st); err != nil {
		return err
	}
	st.Result = m.Result
	if err := st.validate(); err != nil {
		return err
	}
	*s = st
	return nil
}

func (s *State) validate() error {
	if s.Options.Format != "" && s.Options.Format != FormatSingle && s.Options.Format != FormatBestOf3 {
		return errors.New("赛制无效")
	}
	if s.Options.Format == "" {
		s.Options.Format = FormatBestOf3
	}
	if s.Epoch == "" {
		return errors.New("对局标识缺失")
	}
	switch s.Phase {
	case PhaseSelect, PhaseCountdown, PhasePlaying, PhaseRoundBreak, PhaseFinished:
	default:
		return errors.New("对局阶段无效")
	}
	if len(s.Players) != 2 {
		return errors.New("街霸方块固定 2 人")
	}
	if s.Round < 1 || s.Round > 1000 {
		return errors.New("回合数无效")
	}
	if s.SelectUntil < 0 || s.StartAt < 0 || s.MatchEndsAt < 0 || s.BreakUntil < 0 || s.ServerAt < 0 {
		return errors.New("计时无效")
	}
	if s.EventID < 0 || len(s.Events) > 64 {
		return errors.New("事件记录无效")
	}
	lastEvent := 0
	for _, e := range s.Events {
		if e.ID <= lastEvent || e.ID > s.EventID || e.At < 0 || e.Seat < -1 || e.Seat > 1 {
			return errors.New("事件记录无效")
		}
		lastEvent = e.ID
	}
	if s.Result != nil && s.Phase != PhaseFinished {
		return errors.New("结果与阶段不一致")
	}
	if s.Phase == PhaseFinished && s.Result == nil {
		return errors.New("结束阶段缺少结果")
	}
	if s.Phase == PhaseSelect {
		if s.SelectUntil <= 0 {
			return errors.New("选角期限缺失")
		}
	} else if s.SelectUntil != 0 {
		return errors.New("选角期限残留")
	}
	if s.Phase == PhaseCountdown {
		if s.StartAt <= 0 {
			return errors.New("倒计时期限缺失")
		}
	}
	if s.Phase == PhasePlaying {
		if s.StartAt <= 0 || s.MatchEndsAt <= s.StartAt {
			return errors.New("比赛期限无效")
		}
	}
	if s.Phase == PhaseRoundBreak && s.BreakUntil <= 0 {
		return errors.New("回合休息期限缺失")
	}
	if s.Phase != PhaseRoundBreak && s.BreakUntil != 0 {
		return errors.New("回合休息期限残留")
	}
	if s.Phase == PhaseFinished && (s.StartAt != 0 || s.MatchEndsAt != 0) {
		// Keeping the final timestamps is useful for history, but the platform
		// requires no live deadlines; the envelope clears them.
	}
	winTarget := roundsToWin(s.Options.Format)
	for i := range s.Players {
		p := &s.Players[i]
		if p.Seat != i {
			return fmt.Errorf("席位 %d 编号无效", i)
		}
		if p.RoundWins < 0 || p.RoundWins > winTarget {
			return fmt.Errorf("席位 %d 回合胜场无效", i)
		}
		if p.Selected {
			if !validCharacter(p.Character) {
				return fmt.Errorf("席位 %d 角色无效", i)
			}
		} else if p.Character != "" {
			return fmt.Errorf("席位 %d 未选角却有角色", i)
		}
		if p.PatternIdx < 0 || p.PatternIdx >= 72 {
			return fmt.Errorf("席位 %d 图案索引无效", i)
		}
		if p.Seq < 0 || p.Seq > MaxSeq || p.LockResets < 0 || p.LockResets > MaxLockResets {
			return fmt.Errorf("席位 %d 输入序号或锁定次数无效", i)
		}
		if p.IdleAt < 0 || p.SpawnedAt < 0 || p.FallAt < 0 || p.LockAt < 0 {
			return fmt.Errorf("席位 %d 计时无效", i)
		}
		if p.GemsCleared < 0 || p.MaxChain < 0 || p.MaxChain > 100 {
			return fmt.Errorf("席位 %d 统计无效", i)
		}
		if err := validateGems(p.Gems, i); err != nil {
			return err
		}
		if p.Active != nil {
			a := p.Active
			if a.ID <= 0 || a.X < 0 || a.X >= Cols || a.Y < 0 || a.Y >= Rows || a.Rot < 0 || a.Rot > 3 {
				return fmt.Errorf("席位 %d 活动对子无效", i)
			}
			if !validSpec(a.A) || !validSpec(a.B) {
				return fmt.Errorf("席位 %d 活动对子宝石无效", i)
			}
			grid, err := gemGrid(p.Gems)
			if err != nil {
				return fmt.Errorf("席位 %d：%v", i, err)
			}
			for _, c := range pairCells(*a) {
				if c.X < 0 || c.X >= Cols || c.Y < 0 || c.Y >= Rows || grid[c.Y][c.X] != 0 {
					return fmt.Errorf("席位 %d 活动对子越界或重叠", i)
				}
			}
		}
		if p.Next.A.Kind != "" || p.Next.B.Kind != "" {
			if !validSpec(p.Next.A) || !validSpec(p.Next.B) {
				return fmt.Errorf("席位 %d 下一对宝石无效", i)
			}
		}
		if p.Resolve != nil {
			r := p.Resolve
			if r.Chain < 1 || r.Chain > 100 || r.Step < 1 || r.Step > 100 || r.NextAt < 0 || r.Accum < 0 || r.DiamondPart < 0 || r.DiamondPart > r.Accum || r.Cleared < 0 {
				return fmt.Errorf("席位 %d 结算状态无效", i)
			}
			if r.Diamond {
				if !validColor(r.Target) || r.DiamondX < 0 || r.DiamondX >= Cols || r.DiamondY < 0 || r.DiamondY >= Rows {
					return fmt.Errorf("席位 %d 钻石结算无效", i)
				}
			} else if r.Target != "" {
				return fmt.Errorf("席位 %d 非钻石结算带有目标色", i)
			}
		}
		if len(p.Pending) > MaxPending {
			return fmt.Errorf("席位 %d 待落反击宝石过多", i)
		}
		for _, c := range p.Pending {
			if !validColor(c.Color) || c.Column < 0 || c.Column >= Cols || c.Timer < 1 || c.Timer > 5 {
				return fmt.Errorf("席位 %d 待落反击宝石无效", i)
			}
		}
	}
	switch s.Phase {
	case PhaseSelect:
		for i := range s.Players {
			if s.Players[i].Active != nil || s.Players[i].Resolve != nil {
				return errors.New("选角阶段存在活动对子")
			}
		}
	case PhaseCountdown:
		for i := range s.Players {
			if !s.Players[i].Selected {
				return errors.New("倒计时阶段仍有席位未选角")
			}
			if s.Players[i].Active != nil || s.Players[i].Resolve != nil {
				return errors.New("倒计时阶段存在活动对子")
			}
		}
	case PhasePlaying:
		for i := range s.Players {
			p := &s.Players[i]
			if !p.Selected {
				return errors.New("对局中仍有席位未选角")
			}
			if !p.Lost && p.Active == nil && p.Resolve == nil {
				return fmt.Errorf("席位 %d 对局中缺少活动对子或结算", i)
			}
		}
	case PhaseRoundBreak:
		lost := 0
		for i := range s.Players {
			if s.Players[i].Lost {
				lost++
			}
			if s.Players[i].Active != nil || s.Players[i].Resolve != nil {
				return errors.New("回合休息阶段存在活动对子")
			}
		}
		if lost == 0 {
			return errors.New("回合休息阶段没有失败席位")
		}
	case PhaseFinished:
		for i := range s.Players {
			if s.Players[i].Active != nil || s.Players[i].Resolve != nil || len(s.Players[i].Pending) != 0 {
				return errors.New("结束阶段存在未清理状态")
			}
		}
	}
	return nil
}

func validateGems(gems []Gem, seat int) error {
	grid := make(map[[2]int]int, len(gems))
	ids := make(map[int]bool, len(gems))
	for _, g := range gems {
		if g.ID <= 0 || ids[g.ID] {
			return fmt.Errorf("席位 %d 宝石编号重复", seat)
		}
		ids[g.ID] = true
		if !validGemKind(g.Kind) {
			return fmt.Errorf("席位 %d 宝石类型无效", seat)
		}
		if g.Kind != GemDiamond && !validColor(g.Color) {
			return fmt.Errorf("席位 %d 宝石颜色无效", seat)
		}
		if g.Kind == GemDiamond && g.Color != "" {
			return fmt.Errorf("席位 %d 钻石颜色无效", seat)
		}
		if g.X < 0 || g.X >= Cols || g.Y < 0 || g.Y >= Rows {
			return fmt.Errorf("席位 %d 宝石位置无效", seat)
		}
		switch g.Kind {
		case GemNormal, GemCrash, GemCounter, GemDiamond:
			if g.W != 1 || g.H != 1 {
				return fmt.Errorf("席位 %d 单格宝石尺寸无效", seat)
			}
			if g.Kind == GemCounter {
				if g.Timer < 1 || g.Timer > 5 {
					return fmt.Errorf("席位 %d 反击宝石计时无效", seat)
				}
			} else if g.Timer != 0 {
				return fmt.Errorf("席位 %d 非反击宝石带有计时", seat)
			}
		case GemPower:
			if g.W < 2 || g.H < 2 || g.W > Cols || g.H > Rows || g.X+g.W > Cols || g.Y+g.H > Rows {
				return fmt.Errorf("席位 %d 强化宝石尺寸无效", seat)
			}
			if g.Timer != 0 {
				return fmt.Errorf("席位 %d 强化宝石带有计时", seat)
			}
		}
		for _, c := range g.cells() {
			key := [2]int{c.X, c.Y}
			if other, ok := grid[key]; ok && other != g.ID {
				return fmt.Errorf("席位 %d 宝石重叠", seat)
			}
			grid[key] = g.ID
		}
	}
	return nil
}

func (s *State) emit(kind string, seat int, at int64) {
	s.EventID++
	s.Events = append(s.Events, Event{ID: s.EventID, Type: kind, Seat: seat, At: at})
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
}

func (s *State) emitEvent(e Event) {
	s.EventID++
	e.ID = s.EventID
	s.Events = append(s.Events, e)
	if len(s.Events) > 64 {
		s.Events = s.Events[len(s.Events)-64:]
	}
}

func (s *State) log(message string) {
	s.Log = append(s.Log, message)
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
}

func (s *State) envelope() (gameapi.Match, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	m := gameapi.Match{CheckpointKey: s.Phase + ":" + strconv.Itoa(s.Round),
		Schema:    SchemaVersion,
		Data:      data,
		Actors:    []int{},
		Attention: []int{},
		Log:       append([]string{}, s.Log...),
		Result:    s.Result,
	}
	if s.Result != nil {
		if err := m.Validate(len(s.Players)); err != nil {
			return gameapi.Match{}, err
		}
		return m, nil
	}
	now := s.ServerAt
	switch s.Phase {
	case PhaseSelect:
		for i := range s.Players {
			if !s.Players[i].Selected {
				m.Actors = append(m.Actors, i)
			}
		}
		m.Attention = append([]int{}, m.Actors...)
		m.SystemDeadline = now + FrameMS
		if s.SelectUntil > 0 && s.SelectUntil < m.SystemDeadline {
			m.SystemDeadline = s.SelectUntil
		}
		if m.SystemDeadline <= now {
			m.SystemDeadline = now + FrameMS
		}
	case PhaseCountdown:
		m.SystemDeadline = s.StartAt
		if m.SystemDeadline <= now {
			m.SystemDeadline = now + FrameMS
		}
	case PhasePlaying:
		deadline := int64(0)
		for i := range s.Players {
			p := &s.Players[i]
			if p.Lost || p.Resolve != nil {
				continue
			}
			m.Actors = append(m.Actors, i)
			if deadline == 0 || p.IdleAt < deadline {
				deadline = p.IdleAt
			}
		}
		m.Attention = append([]int{}, m.Actors...)
		if deadline > 0 {
			m.Deadline = deadline
			for _, seat := range m.Actors {
				if s.Players[seat].IdleAt == deadline {
					m.TimeoutActors = append(m.TimeoutActors, seat)
				}
			}
		}
		m.SystemDeadline = now + FrameMS
	case PhaseRoundBreak:
		m.SystemDeadline = s.BreakUntil
		if m.SystemDeadline <= now {
			m.SystemDeadline = now + FrameMS
		}
	default:
		return gameapi.Match{}, errors.New("对局阶段无法封装")
	}
	if err := m.Validate(len(s.Players)); err != nil {
		return gameapi.Match{}, err
	}
	return m, nil
}

func (s *State) view(viewer int) (View, error) {
	v := View{
		Options:     s.Options,
		Phase:       s.Phase,
		Epoch:       s.Epoch,
		ServerAt:    s.ServerAt,
		StartAt:     s.StartAt,
		MatchEndsAt: s.MatchEndsAt,
		SelectUntil: s.SelectUntil,
		BreakUntil:  s.BreakUntil,
		Round:       s.Round,
		RoundsToWin: roundsToWin(s.Options.Format),
		Players:     make([]PlayerView, 0, len(s.Players)),
		Events:      append([]Event{}, s.Events...),
		Characters:  characterViews(),
	}
	own := viewer >= 0 && viewer < len(s.Players)
	for i := range s.Players {
		p := &s.Players[i]
		pv := PlayerView{
			Seat:        i,
			Character:   p.Character,
			Selected:    p.Selected,
			RoundWins:   p.RoundWins,
			Pending:     len(p.Pending),
			Warning:     warningFor(len(p.Pending)),
			Resolving:   p.Resolve != nil,
			Lost:        p.Lost,
			GemsCleared: p.GemsCleared,
			MaxChain:    p.MaxChain,
			Gems:        make([]GemView, 0, len(p.Gems)),
		}
		if p.Selected {
			pv.Pattern = patternFor(p.Character)
		}
		for _, g := range p.Gems {
			pv.Gems = append(pv.Gems, GemView{ID: g.ID, Kind: g.Kind, Color: g.Color, X: g.X, Y: g.Y, W: g.W, H: g.H, Timer: g.Timer})
		}
		if p.Active != nil {
			cells := pairCells(*p.Active)
			av := &PairView{ID: p.Active.ID, X: p.Active.X, Y: p.Active.Y, Rot: p.Active.Rot, Cells: cells[:]}
			if own && viewer == i && s.Phase == PhasePlaying && !p.Lost && p.Resolve == nil {
				y := dropYFor(p.Gems, *p.Active)
				ghost := pairCellsAt(p.Active.X, y, p.Active.Rot, p.Active.A, p.Active.B)
				av.Ghost = []Point{{X: ghost[0].X, Y: ghost[0].Y}, {X: ghost[1].X, Y: ghost[1].Y}}
			}
			pv.Active = av
		}
		pv.Next = PairSpecView{A: p.Next.A, B: p.Next.B}
		if own && viewer == i {
			pv.Seq = p.Seq
		}
		v.Players = append(v.Players, pv)
	}
	if own && viewer < len(s.Players) {
		p := &s.Players[viewer]
		v.Controls.CanPlay = s.Phase == PhasePlaying && !p.Lost && p.Resolve == nil && p.Active != nil
		v.Controls.CanSelect = s.Phase == PhaseSelect && !p.Selected
	}
	return v, nil
}

func warningFor(pending int) string {
	switch {
	case pending <= 0:
		return ""
	case pending <= 10:
		return "注意"
	case pending <= 30:
		return "警告"
	default:
		return "危险"
	}
}

func (s *State) finish(now int64, reason string) error {
	if s.Result != nil {
		return nil
	}
	wins := [2]int{s.Players[0].RoundWins, s.Players[1].RoundWins}
	r := &gameapi.Result{
		Ranks:   []int{2, 2},
		Winners: []int{},
		Scores:  []int{wins[0], wins[1]},
	}
	switch {
	case wins[0] > wins[1]:
		r.Ranks = []int{1, 2}
		r.Winners = []int{0}
	case wins[1] > wins[0]:
		r.Ranks = []int{2, 1}
		r.Winners = []int{1}
	default:
		r.Draw = true
		r.Ranks = []int{1, 1}
		r.Winners = []int{}
	}
	players := make([]map[string]any, 0, len(s.Players))
	for i := range s.Players {
		p := &s.Players[i]
		players = append(players, map[string]any{
			"seat": i, "character": p.Character, "roundWins": p.RoundWins,
			"gemsCleared": p.GemsCleared, "maxChain": p.MaxChain,
		})
	}
	details, _ := json.Marshal(map[string]any{
		"format": s.Options.Format, "reason": reason, "round": s.Round, "players": players,
	})
	r.Details = details
	s.Result = r
	s.FinishReason = reason
	s.Phase = PhaseFinished
	s.ServerAt = now
	s.SelectUntil = 0
	s.BreakUntil = 0
	for i := range s.Players {
		p := &s.Players[i]
		p.Active = nil
		p.Resolve = nil
		p.Pending = nil
		p.IdleAt = 0
		p.FallAt = 0
		p.LockAt = 0
	}
	s.emitEvent(Event{Type: "finish", Seat: -1, At: now})
	if r.Draw {
		s.log("街霸方块结束：和局")
	} else {
		s.log(fmt.Sprintf("街霸方块结束：玩家 %d 获胜", r.Winners[0]+1))
	}
	return nil
}

func (s *State) spawn(p *Player, now int64) bool {
	if !p.Selected {
		return false
	}
	grid, err := gemGrid(p.Gems)
	if err != nil {
		p.Lost = true
		return false
	}
	for _, c := range [2]Point{{X: 2, Y: 0}, {X: 2, Y: 1}} {
		if c.X < 0 || c.X >= Cols || c.Y < 0 || c.Y >= Rows || grid[c.Y][c.X] != 0 {
			p.Lost = true
			s.emitEvent(Event{Type: "topout", Seat: p.Seat, At: now})
			return false
		}
	}
	next := p.Next
	next.X = 2
	next.Y = 1
	next.Rot = 0
	p.Active = &next
	p.Next = generatePair(p)
	p.SpawnedAt = now
	p.FallAt = now + FallMS
	p.LockAt = 0
	p.LockResets = 0
	return true
}

func (s *State) resetRound(now int64) {
	s.Round++
	s.SelectUntil = 0
	s.BreakUntil = 0
	for i := range s.Players {
		p := &s.Players[i]
		p.Gems = nil
		p.NextID = 0
		p.PairID = 0
		p.Active = nil
		p.Resolve = nil
		p.Pending = nil
		p.PairCount = 0
		p.PatternIdx = 0
		p.Lost = false
		p.LockAt = 0
		p.LockResets = 0
		p.Next = generatePair(p)
		s.spawn(p, now)
		p.IdleAt = now + IdleMS
	}
	s.emitEvent(Event{Type: "round_start", Seat: -1, At: now, Count: s.Round})
	s.log(fmt.Sprintf("第 %d 回合开始", s.Round))
}

func (s *State) beginMatch(now int64) {
	s.Phase = PhasePlaying
	s.ServerAt = now
	s.SelectUntil = 0
	s.BreakUntil = 0
	s.MatchEndsAt = now + matchCap(s.Options.Format)
	s.Round = 0
	s.resetRound(now)
	s.emitEvent(Event{Type: "start", Seat: -1, At: now})
	s.log("街霸方块开始")
}

func (s *State) moveToCountdown(now int64) {
	s.Phase = PhaseCountdown
	s.SelectUntil = 0
	s.StartAt = now + CountdownMS
	s.ServerAt = now
	s.emitEvent(Event{Type: "countdown", Seat: -1, At: now})
	s.log("双方已选角，准备开始")
}

func (s *State) autoSelect(now int64) {
	for i := range s.Players {
		p := &s.Players[i]
		if p.Selected {
			continue
		}
		p.Character = randomCharacter(p)
		p.Selected = true
		s.emitEvent(Event{Type: "select_timeout", Seat: i, At: now, Character: p.Character})
	}
}

func (s *State) allSelected() bool {
	return !slices.ContainsFunc(s.Players, func(p Player) bool { return !p.Selected })
}

// dropYFor returns the pivot Y where the pair comes to rest. It is also used by
// the display-only ghost and by the bot's placement search.
func dropYFor(gems []Gem, pair Pair) int {
	grid, err := gemGrid(gems)
	if err != nil {
		return pair.Y
	}
	y := pair.Y
	for {
		if !pairFits(grid, pair, pair.X, y+1, pair.Rot) {
			return y
		}
		y++
	}
}

func pairFits(grid [Rows][Cols]int, pair Pair, x, y, rot int) bool {
	for _, c := range pairCellsAt(x, y, rot, pair.A, pair.B) {
		if c.X < 0 || c.X >= Cols || c.Y < 0 || c.Y >= Rows || grid[c.Y][c.X] != 0 {
			return false
		}
	}
	return true
}

// gridHasColor reports whether any cell of the given color remains.
func gridHasColor(gems []Gem, color string) bool {
	for _, g := range gems {
		if g.Color == color && (g.Kind == GemNormal || g.Kind == GemCrash || g.Kind == GemPower || g.Kind == GemCounter) {
			return true
		}
	}
	return false
}
