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
	ID    int       `json:"id"`
	Name  string    `json:"name"`
	X     float64   `json:"x"`
	Y     float64   `json:"y"`
	Label []float64 `json:"label,omitempty"`
}
type Route struct {
	ID       int            `json:"id"`
	A        int            `json:"a"`
	B        int            `json:"b"`
	Length   int            `json:"length"`
	Color    int            `json:"color"`
	Segments []RouteSegment `json:"segments,omitempty"`
}
type RouteSegment struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Angle float64 `json:"angle"`
}
type Ticket struct {
	ID       int  `json:"id"`
	A        int  `json:"a"`
	B        int  `json:"b"`
	Points   int  `json:"points"`
	Complete bool `json:"complete"`
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
	Eliminated  bool     `json:"eliminated,omitempty"`
	Hand        []int    `json:"hand"`
	Tickets     []Ticket `json:"tickets"`
	Trains      int      `json:"trains"`
	Score       int      `json:"score"`
	RouteScore  int      `json:"routeScore"`
	TicketScore int      `json:"ticketScore"`
	Longest     int      `json:"longest"`
	Bonus       int      `json:"bonus"`
	Completed   int      `json:"completed"`
}
type Rail struct {
	SetupPending  [][]Ticket   `json:"setupPending,omitempty"`
	Deck          []int        `json:"deck"`
	Discard       []int        `json:"discard"`
	Face          []int        `json:"face"`
	FaceVersion   [5]uint64    `json:"faceVersion"`
	TicketDeck    []Ticket     `json:"ticketDeck"`
	Pending       []Ticket     `json:"pending"`
	Players       []RailPlayer `json:"players"`
	Owners        map[int]int  `json:"owners"`
	Setup         bool         `json:"setup"`
	Drawn         int          `json:"drawn"`
	LastRemaining int          `json:"lastRemaining"`
	Passes        int          `json:"passes"`
}

func (s *State) initRail(n int) {
	g := &Rail{Players: make([]RailPlayer, n), Owners: map[int]int{}, Setup: true, LastRemaining: -1, TicketDeck: MapData().Tickets, Face: []int{}, Discard: []int{}}
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
	for i := range g.Players {
		g.Players[i] = RailPlayer{Hand: make([]int, 9), Tickets: []Ticket{}, Trains: 45}
		for j := 0; j < 4; j++ {
			c, _ := g.draw()
			g.Players[i].Hand[c]++
		}
	}
	g.refill()
	g.SetupPending = make([][]Ticket, n)
	for i := range g.Players {
		g.SetupPending[i] = append([]Ticket{}, g.TicketDeck[:3]...)
		g.TicketDeck = g.TicketDeck[3:]
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
func (g *Rail) refill() {
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
			return
		}
		for _, pile := range [][]int{g.Deck, g.Discard} {
			for _, c := range pile {
				if c != 8 {
					nonWild++
				}
			}
		}
		if nonWild < 3 {
			return
		} // A finite market when too few ordinary cards exist; draws remain legal.
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
			return
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
			} else {
				g.TicketDeck = append(g.TicketDeck, t)
			}
		}
		g.Pending = []Ticket{}
		g.Passes = 0
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 保留了 %d 张目的地任务", s.Turn+1, len(a.Keep)))
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
			if c == 8 && g.Drawn > 0 {
				return errors.New("第二张不能拿公开的万能列车牌")
			}
			g.Face[a.Slot] = -1
			g.FaceVersion[a.Slot]++
		}
		p.Hand[c]++
		g.Drawn++
		g.Passes = 0
		g.refill()
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 摸取了一张列车牌", s.Turn+1))
		if g.Drawn == 2 || (a.Slot >= 0 && c == 8) || !g.canDrawSecond() {
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
	case "claim":
		var route Route
		for _, r := range MapData().Routes {
			if r.ID == a.Route {
				route = r
				break
			}
		}
		if route.ID == 0 {
			return errors.New("不存在这条路线")
		}
		if _, ok := g.Owners[a.Route]; ok {
			return errors.New("路线已被占领")
		}
		for _, r := range MapData().Routes {
			if (r.A == route.A && r.B == route.B) || (r.A == route.B && r.B == route.A) {
				if owner, ok := g.Owners[r.ID]; ok && (len(g.Players) < 4 || owner == s.Turn) {
					return errors.New("2–3 人局只能使用双线中的一条；同一人不能占领双线")
				}
			}
		}
		if p.Trains < route.Length {
			return errors.New("剩余车厢不足")
		}
		if a.Color < 0 || a.Color > 7 || a.Wild < 0 || a.Wild > route.Length {
			return errors.New("请选择合法的支付组合")
		}
		if route.Color >= 0 && a.Color != route.Color {
			return errors.New("列车牌颜色必须与路线相同")
		}
		if p.Hand[a.Color] < route.Length-a.Wild || p.Hand[8] < a.Wild {
			return errors.New("列车牌不足")
		}
		p.Hand[a.Color] -= route.Length - a.Wild
		p.Hand[8] -= a.Wild
		for i := 0; i < route.Length-a.Wild; i++ {
			g.Discard = append(g.Discard, a.Color)
		}
		for i := 0; i < a.Wild; i++ {
			g.Discard = append(g.Discard, 8)
		}
		p.Trains -= route.Length
		p.RouteScore += []int{0, 1, 2, 4, 7, 10, 15}[route.Length]
		p.Score = p.RouteScore
		g.Owners[route.ID] = s.Turn
		g.Passes = 0
		g.refill()
		d := MapData()
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 铺设了 %s → %s", s.Turn+1, d.Cities[route.A].Name, d.Cities[route.B].Name))
		s.railNext()
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
		if c >= 0 && c != 8 {
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
	p := g.Players[s.Turn]
	for _, r := range MapData().Routes {
		if _, ok := g.Owners[r.ID]; ok || p.Trains < r.Length {
			continue
		}
		for color := 0; color < 8; color++ {
			if r.Color >= 0 && color != r.Color {
				continue
			}
			for wild := 0; wild <= r.Length; wild++ {
				if p.Hand[color] >= r.Length-wild && p.Hand[8] >= wild {
					copy := clone(*s)
					if copy.applyRail(Action{Type: "claim", Route: r.ID, Color: color, Wild: wild}) == nil {
						return true
					}
				}
			}
		}
	}
	return false
}
func Connected(owners map[int]int, player, a, b int) bool {
	adj := map[int][]int{}
	for _, r := range MapData().Routes {
		if p, ok := owners[r.ID]; ok && p == player {
			adj[r.A] = append(adj[r.A], r.B)
			adj[r.B] = append(adj[r.B], r.A)
		}
	}
	seen := map[int]bool{a: true}
	q := []int{a}
	for len(q) > 0 {
		v := q[0]
		q = q[1:]
		if v == b {
			return true
		}
		for _, n := range adj[v] {
			if !seen[n] {
				seen[n] = true
				q = append(q, n)
			}
		}
	}
	return false
}
func Longest(owners map[int]int, player int) int {
	edges := []Route{}
	for _, r := range MapData().Routes {
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
func (s *State) railFinish() {
	g := s.Rail
	longest := 0
	for i := range g.Players {
		p := &g.Players[i]
		if p.Eliminated {
			continue
		}
		p.Longest = Longest(g.Owners, i)
		longest = max(longest, p.Longest)
		for j, t := range p.Tickets {
			ok := Connected(g.Owners, i, t.A, t.B)
			p.Tickets[j].Complete = ok
			if ok {
				p.TicketScore += t.Points
				p.Completed++
			} else {
				p.TicketScore -= t.Points
			}
		}
	}
	best, completed, bonus := -999, -1, -1
	for i := range g.Players {
		p := &g.Players[i]
		if p.Eliminated {
			continue
		}
		if p.Longest == longest && longest > 0 {
			p.Bonus = 10
		}
		p.Score = p.RouteScore + p.TicketScore + p.Bonus
		if p.Score > best || (p.Score == best && p.Completed > completed) || (p.Score == best && p.Completed == completed && p.Bonus > bonus) {
			best = p.Score
			completed = p.Completed
			bonus = p.Bonus
			s.Winners = []int{i}
		} else if p.Score == best && p.Completed == completed && p.Bonus == bonus {
			s.Winners = append(s.Winners, i)
		}
	}
	s.Finished = true
	s.Phase = "finished"
}
