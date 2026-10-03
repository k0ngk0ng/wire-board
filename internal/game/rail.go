package game

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed rail_data.json
var railJSON []byte

type City struct {
	ID      int       `json:"id"`
	Name    string    `json:"name"`
	X       float64   `json:"x"`
	Y       float64   `json:"y"`
	Label   []float64 `json:"label,omitempty"`
	Kind    string    `json:"kind,omitempty"`
	Country string    `json:"country,omitempty"`
}
type Route struct {
	ID         int            `json:"id"`
	A          int            `json:"a"`
	B          int            `json:"b"`
	Length     int            `json:"length"`
	Color      int            `json:"color"`
	Segments   []RouteSegment `json:"segments,omitempty"`
	Tunnel     bool           `json:"tunnel,omitempty"`
	Ferry      int            `json:"ferry,omitempty"`
	Mountain   int            `json:"mountain,omitempty"`
	Substitute int            `json:"substitute,omitempty"`
}
type RouteSegment struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Angle float64 `json:"angle"`
}
type Ticket struct {
	ID       int            `json:"id"`
	A        int            `json:"a"`
	B        int            `json:"b"`
	Points   int            `json:"points"`
	Complete bool           `json:"complete"`
	Art      int            `json:"art,omitempty"`
	Long     bool           `json:"long,omitempty"`
	Options  []TicketOption `json:"options,omitempty"`
	Value    int            `json:"value,omitempty"`
	Mandala  bool           `json:"mandala,omitempty"`
}
type TicketOption struct {
	To     int `json:"to"`
	Points int `json:"points"`
}
type RailData struct {
	Cities  []City   `json:"cities"`
	Routes  []Route  `json:"routes"`
	Tickets []Ticket `json:"tickets"`
}

var baseRailData = func() RailData {
	var d RailData
	if e := json.Unmarshal(railJSON, &d); e != nil {
		panic(e)
	}
	return d
}()

// Return owned slices: shuffling a new game's tickets must never mutate another game.
func MapData() RailData {
	return RailData{
		Cities:  append([]City{}, baseRailData.Cities...),
		Routes:  append([]Route{}, baseRailData.Routes...),
		Tickets: append([]Ticket{}, baseRailData.Tickets...),
	}
}

type RailPlayer struct {
	Eliminated     bool     `json:"eliminated,omitempty"`
	Hand           []int    `json:"hand"`
	Tickets        []Ticket `json:"tickets"`
	Trains         int      `json:"trains"`
	Score          int      `json:"score"`
	RouteScore     int      `json:"routeScore"`
	TicketScore    int      `json:"ticketScore"`
	Longest        int      `json:"longest"`
	Bonus          int      `json:"bonus"`
	Completed      int      `json:"completed"`
	Stations       []int    `json:"stations,omitempty"`
	StationRoutes  []int    `json:"stationRoutes,omitempty"`
	StationScore   int      `json:"stationScore,omitempty"`
	MandalaCount   int      `json:"mandalaCount,omitempty"`
	MandalaScore   int      `json:"mandalaScore,omitempty"`
	Network        int      `json:"network,omitempty"`
	MountainTrains int      `json:"mountainTrains,omitempty"`
	MountainRoutes int      `json:"mountainRoutes,omitempty"`
}
type RailDrawEvent struct {
	ID     uint64 `json:"id"`
	Player int    `json:"player"`
	Slot   int    `json:"slot"`
	Color  *int   `json:"color,omitempty"`
}

type HiddenDrawEvent struct {
	ID     uint64 `json:"id"`
	Player int    `json:"player"`
}

type Rail struct {
	Map              string            `json:"map,omitempty"`
	Tunnel           *RailTunnel       `json:"tunnel,omitempty"`
	DrawID           uint64            `json:"drawId,omitempty"`
	DrawEvents       []RailDrawEvent   `json:"drawEvents,omitempty"`
	HiddenDrawID     uint64            `json:"hiddenDrawId,omitempty"`
	HiddenDrawEvents []HiddenDrawEvent `json:"hiddenDrawEvents,omitempty"`
	SetupPending     [][]Ticket        `json:"setupPending,omitempty"`
	Deck             []int             `json:"deck"`
	Discard          []int             `json:"discard"`
	Face             []int             `json:"face"`
	FaceVersion      [5]uint64         `json:"faceVersion"`
	TicketDeck       []Ticket          `json:"ticketDeck"`
	Pending          []Ticket          `json:"pending"`
	Players          []RailPlayer      `json:"players"`
	Owners           map[int]int       `json:"owners"`
	Setup            bool              `json:"setup"`
	Drawn            int               `json:"drawn"`
	LastRemaining    int               `json:"lastRemaining"`
	Passes           int               `json:"passes"`
}

func (s *State) initRail(n int) {
	s.initRailMap(n, "usa")
}
func (s *State) initRailMap(n int, mapID string) {
	info, _ := RailMapInfo(mapID)
	g := &Rail{Map: info.ID, Players: make([]RailPlayer, n), Owners: map[int]int{}, Setup: true, LastRemaining: -1, Face: []int{}, Discard: []int{}}
	long := []Ticket{}
	for _, t := range g.data().Tickets {
		if t.Long {
			long = append(long, t)
		} else {
			g.TicketDeck = append(g.TicketDeck, t)
		}
	}
	for color := 0; color < 9; color++ {
		count := 12
		if color == 8 {
			count = 14
		}
		for j := 0; j < count; j++ {
			g.Deck = append(g.Deck, color)
		}
	}
	shuffle(g.Deck)
	shuffle(g.TicketDeck)
	shuffle(long)
	for i := range g.Players {
		g.Players[i] = RailPlayer{Hand: make([]int, 9), Tickets: []Ticket{}, Trains: info.Trains}
		for j := 0; j < 4; j++ {
			c, _ := g.draw()
			g.Players[i].Hand[c]++
		}
	}
	g.refill()
	g.SetupPending = make([][]Ticket, n)
	for i := range g.Players {
		count := info.SetupTickets
		if info.LongTickets {
			g.SetupPending[i] = append(g.SetupPending[i], long[i])
			count--
		}
		g.SetupPending[i] = append(g.SetupPending[i], g.TicketDeck[:count]...)
		g.TicketDeck = g.TicketDeck[count:]
	}
	s.Rail = g
	s.Phase = "tickets"
}
func (g *Rail) draw() (int, bool) {
	if len(g.Deck) == 0 {
		if len(g.Discard) == 0 {
			return 0, false
		}
		g.Deck = append([]int{}, g.Discard...)
		g.Discard = []int{}
		shuffle(g.Deck)
	}
	c := g.Deck[0]
	g.Deck = g.Deck[1:]
	return c, true
}
func (g *Rail) refill() (resets int) {
	for attempt := 0; ; attempt++ {
		for i, c := range g.Face {
			if c < 0 {
				if replacement, ok := g.draw(); ok {
					g.Face[i] = replacement
					g.FaceVersion[i]++
				}
			}
		}
		for len(g.Face) < 5 {
			c, ok := g.draw()
			if !ok {
				break
			}
			g.FaceVersion[len(g.Face)]++
			g.Face = append(g.Face, c)
		}
		wild := 0
		nonWild := 0
		for _, c := range g.Face {
			if c == 8 {
				wild++
			} else if c >= 0 {
				nonWild++
			}
		}
		if wild < 3 {
			return resets
		}
		for _, pile := range [][]int{g.Deck, g.Discard} {
			for _, c := range pile {
				if c != 8 {
					nonWild++
				}
			}
		}
		if nonWild < 3 {
			return resets
		} // A finite market when too few ordinary cards exist; draws remain legal.
		resets++
		// All slots change when the market is reset, including identical colors.
		for i := range g.FaceVersion {
			g.FaceVersion[i]++
		}
		if attempt >= 100 {
			pool := append(append(append([]int{}, g.Face...), g.Deck...), g.Discard...)
			shuffle(pool)
			g.Face = []int{}
			rest := []int{}
			for _, c := range pool {
				if c < 0 {
					continue
				}
				if c != 8 && len(g.Face) < 3 {
					g.Face = append(g.Face, c)
				} else {
					rest = append(rest, c)
				}
			}
			g.Deck = rest
			g.Discard = []int{}
			for len(g.Face) < 5 {
				c, ok := g.draw()
				if !ok {
					break
				}
				g.Face = append(g.Face, c)
			}
			return resets
		}
		for _, c := range g.Face {
			if c >= 0 {
				g.Discard = append(g.Discard, c)
			}
		}
		g.Face = []int{}
	}
}
func (s *State) applyRail(a Action) error {
	g := s.Rail
	p := &g.Players[s.Turn]
	if g.Tunnel != nil {
		return s.resolveRailTunnel(a)
	}
	if s.Phase == "tickets" {
		if a.Type != "keep" {
			return errors.New("请先选择保留的目的地任务")
		}
		minimum := 1
		if g.Setup {
			minimum = 2
		}
		minimum = min(minimum, len(g.Pending))
		if len(a.Keep) < minimum {
			return fmt.Errorf("至少保留 %d 张目的地任务", minimum)
		}
		selected := map[int]bool{}
		for _, id := range a.Keep {
			if selected[id] {
				return errors.New("不能重复选择任务")
			}
			found := false
			for _, t := range g.Pending {
				if t.ID == id {
					found = true
				}
			}
			if !found {
				return errors.New("无效的目的地任务")
			}
			selected[id] = true
		}
		for _, t := range g.Pending {
			if selected[t.ID] {
				p.Tickets = append(p.Tickets, t)
			} else if g.info().AdditionalReturn {
				g.TicketDeck = append(g.TicketDeck, t)
			}
		}
		returned := len(g.Pending) - len(a.Keep)
		g.Pending = []Ticket{}
		g.Passes = 0
		verb := "移出游戏"
		if g.info().AdditionalReturn {
			verb = "放回"
		}
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 保留了 %d 张目的地任务，"+verb+" %d 张", s.Turn+1, len(a.Keep), returned))
		if g.Setup {
			s.Turn++
			if s.Turn == len(g.Players) {
				s.Turn = 0
				g.Setup = false
				s.Phase = "turn"
			} else {
				g.Pending = append([]Ticket{}, g.TicketDeck[:3]...)
				g.TicketDeck = g.TicketDeck[3:]
			}
		} else {
			s.railNext()
		}
		return nil
	}
	if g.Drawn > 0 && a.Type != "draw" {
		return errors.New("请完成第二次摸牌")
	}
	switch a.Type {
	case "draw":
		var c int
		if a.Slot == -1 {
			var ok bool
			c, ok = g.draw()
			if !ok {
				return errors.New("牌堆和弃牌堆均已空")
			}
		} else {
			if a.Slot < 0 || a.Slot >= len(g.Face) || g.Face[a.Slot] < 0 {
				return errors.New("这张公开牌已不存在")
			}
			c = g.Face[a.Slot]
			if c == 8 && g.Drawn > 0 && !g.info().WildSingle {
				return errors.New("第二张不能拿公开的万能列车牌")
			}
			g.Face[a.Slot] = -1
			g.FaceVersion[a.Slot]++
		}
		g.DrawID++
		event := RailDrawEvent{ID: g.DrawID, Player: s.Turn, Slot: a.Slot}
		if a.Slot >= 0 {
			color := c
			event.Color = &color
		}
		g.DrawEvents = append(g.DrawEvents, event)
		if len(g.DrawEvents) > 10 {
			g.DrawEvents = g.DrawEvents[len(g.DrawEvents)-10:]
		}
		p.Hand[c]++
		g.Drawn++
		g.Passes = 0
		if a.Slot == -1 {
			g.HiddenDrawID++
			g.HiddenDrawEvents = append(g.HiddenDrawEvents, HiddenDrawEvent{ID: g.HiddenDrawID, Player: s.Turn})
			if len(g.HiddenDrawEvents) > 10 {
				g.HiddenDrawEvents = g.HiddenDrawEvents[len(g.HiddenDrawEvents)-10:]
			}
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 从牌堆摸取 1 张暗牌（第 %d 次摸牌）", s.Turn+1, g.Drawn))
		} else {
			detail := fmt.Sprintf("第 %d 次摸牌", g.Drawn)
			if c == 8 && !g.info().WildSingle {
				detail = "公开万能牌，本回合摸牌结束"
			}
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 拿取公开%s×1（市场第 %d 格，%s）", s.Turn+1, railCardName(c), a.Slot+1, detail))
		}
		s.refillRailMarket()
		if g.Drawn == 2 || (a.Slot >= 0 && c == 8 && !g.info().WildSingle) || !g.canDrawSecond() {
			s.railNext()
		} else {
			s.Phase = "draw"
		}
	case "tickets":
		if len(g.TicketDeck) == 0 {
			return errors.New("目的地牌堆已空")
		}
		n := min(3, len(g.TicketDeck))
		g.Pending = append([]Ticket{}, g.TicketDeck[:n]...)
		g.TicketDeck = g.TicketDeck[n:]
		s.Phase = "tickets"
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 抽取了 %d 张目的地任务，等待选择（至少保留 1 张）", s.Turn+1, n))
	case "claim":
		return s.claimRail(a)
	case "station":
		return s.buildRailStation(a)
	case "pass":
		if s.railHasMove() {
			return errors.New("仍有合法行动，不能跳过")
		}
		g.Passes++
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 无可用行动，跳过", s.Turn+1))
		if g.Passes >= g.activePlayers() {
			s.railFinish()
		} else {
			s.railNext()
		}
	default:
		return errors.New("未知操作")
	}
	return nil
}
func (g *Rail) canDrawSecond() bool {
	if len(g.Deck)+len(g.Discard) > 0 {
		return true
	}
	for _, c := range g.Face {
		if c >= 0 && (c != 8 || g.info().WildSingle) {
			return true
		}
	}
	return false
}
func (s *State) railNext() {
	g := s.Rail
	if g.LastRemaining >= 0 {
		g.LastRemaining--
	} else if !g.Players[s.Turn].Eliminated && g.Players[s.Turn].Trains <= 2 {
		g.LastRemaining = len(g.Players)
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 剩余 %d 节车厢，触发最后一轮（每位玩家再行动一次）", s.Turn+1, g.Players[s.Turn].Trains))
	}
	if g.LastRemaining == 0 {
		s.railFinish()
		return
	}
	for range g.Players {
		s.Turn = (s.Turn + 1) % len(g.Players)
		if s.Turn == 0 {
			s.Round++
		}
		if !g.Players[s.Turn].Eliminated {
			break
		}
		if g.LastRemaining > 0 {
			g.LastRemaining--
			if g.LastRemaining == 0 {
				s.railFinish()
				return
			}
		}
	}
	s.Phase = "turn"
	g.Drawn = 0
}
func (s *State) railHasMove() bool {
	g := s.Rail
	if len(g.Deck)+len(g.Discard)+len(g.TicketDeck) > 0 {
		return true
	}
	for _, c := range g.Face {
		if c >= 0 {
			return true
		}
	}
	for _, r := range g.data().Routes {
		if len(g.paymentOptions(s.Turn, r)) > 0 {
			return true
		}
	}
	p := g.Players[s.Turn]
	if g.info().Stations > len(p.Stations) {
		free := false
		for _, city := range g.data().Cities {
			occupied := false
			for _, other := range g.Players {
				for _, v := range other.Stations {
					occupied = occupied || v == city.ID
				}
			}
			if !occupied {
				free = true
				break
			}
		}
		if free {
			for c := 0; c < 8; c++ {
				if p.Hand[c]+p.Hand[8] >= len(p.Stations)+1 {
					return true
				}
			}
		}
	}
	return false
}
func Connected(owners map[int]int, player, a, b int) bool {
	g := &Rail{Owners: owners}
	return g.connected(player, a, b)
}
func Longest(owners map[int]int, player int) int {
	return longestRailRoutes(baseRailData.Routes, owners, player)
}
func longestRailRoutes(routes []Route, owners map[int]int, player int) int {
	edges := []Route{}
	for _, r := range routes {
		if p, ok := owners[r.ID]; ok && p == player {
			edges = append(edges, r)
		}
	}
	adj := map[int][]int{}
	for i, r := range edges {
		adj[r.A] = append(adj[r.A], i)
		adj[r.B] = append(adj[r.B], i)
	}
	type key struct {
		V    int
		Mask uint64
	}
	memo := map[key]int{}
	var dfs func(int, uint64) int
	dfs = func(v int, mask uint64) int {
		k := key{v, mask}
		if n, ok := memo[k]; ok {
			return n
		}
		best := 0
		for _, i := range adj[v] {
			bit := uint64(1) << i
			if mask&bit != 0 {
				continue
			}
			e := edges[i]
			to := e.A
			if v == e.A {
				to = e.B
			}
			best = max(best, e.Length+dfs(to, mask|bit))
		}
		memo[k] = best
		return best
	}
	best := 0
	for v := range adj {
		best = max(best, dfs(v, 0))
	}
	return best
}
func railCardName(color int) string {
	if color == 8 {
		return "万能牌"
	}
	return [...]string{"紫色", "白色", "蓝色", "黄色", "橙色", "黑色", "红色", "绿色"}[color] + "列车牌"
}

func (s *State) refillRailMarket() {
	if resets := s.Rail.refill(); resets > 0 {
		s.Log = append(s.Log, fmt.Sprintf("公开市场出现至少 3 张万能牌，已重置市场（%d 次）", resets))
	}
}
