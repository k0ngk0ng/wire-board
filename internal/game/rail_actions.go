package game

import (
	"errors"
	"fmt"
	"strings"
)

type RailTunnel struct {
	Route    int   `json:"route"`
	Color    int   `json:"color"`
	Base     []int `json:"base"`
	Revealed []int `json:"revealed"`
	Extra    int   `json:"extra"`
	WildOnly bool  `json:"wildOnly"`
}
type RailPayment struct {
	Color  int   `json:"color"`
	Tokens []int `json:"tokens"`
}

func (g *Rail) route(id int) (Route, bool) {
	for _, r := range g.data().Routes {
		if r.ID == id {
			return r, true
		}
	}
	return Route{}, false
}
func (g *Rail) routeOpen(r Route, player int) bool {
	if _, ok := g.Owners[r.ID]; ok {
		return false
	}
	for _, other := range g.data().Routes {
		if (other.A == r.A && other.B == r.B) || (other.A == r.B && other.B == r.A) {
			if owner, ok := g.Owners[other.ID]; ok && (len(g.Players) < g.info().DoubleMin || owner == player) {
				return false
			}
		}
	}
	return true
}
func (g *Rail) wildAllowed(r Route) bool {
	switch g.info().WildRule {
	case "tunnel":
		return r.Tunnel
	case "special":
		return r.Tunnel || r.Ferry > 0
	default:
		return true
	}
}
func railHeld(hand, pay []int) bool {
	if len(pay) != 9 {
		return false
	}
	for c, n := range pay {
		if n < 0 || n > hand[c] {
			return false
		}
	}
	return true
}
func railSimplePayment(a Action, count int) []int {
	if len(a.Tokens) > 0 {
		return append([]int{}, a.Tokens...)
	}
	if a.Color < 0 || a.Color > 7 || a.Wild < 0 || a.Wild > count {
		return nil
	}
	pay := make([]int, 9)
	pay[a.Color] = count - a.Wild
	pay[8] = a.Wild
	return pay
}
func (g *Rail) validPayment(r Route, hand []int, color int, pay []int) bool {
	if color < 0 || color > 7 || (r.Color >= 0 && color != r.Color) || !railHeld(hand, pay) {
		return false
	}
	total := sum(pay)
	if r.Substitute == 4 {
		// The nine-space Nordic route: any four cards replace one matching card.
		for replaced := 0; replaced <= r.Length; replaced++ {
			if total == r.Length+3*replaced && pay[color] >= r.Length-replaced {
				return true
			}
		}
		return false
	}
	if r.Substitute == 3 && r.Ferry > 0 {
		// Substitution is only for compulsory ferry locomotives. Remaining ordinary
		// spaces still need the chosen color or locomotives; no card counts twice.
		for replaced := 0; replaced <= r.Ferry; replaced++ {
			if total == r.Length+2*replaced && pay[8] >= r.Ferry-replaced && pay[color]+pay[8] >= r.Length-replaced {
				return true
			}
		}
		return false
	}
	if total != r.Length || pay[color]+pay[8] != total || pay[8] < r.Ferry {
		return false
	}
	return pay[8] == 0 || g.wildAllowed(r)
}
func takeExtraCards(hand, pay []int, count int) bool {
	// Preserve locomotives when ordinary cards can cover an arbitrary-card group.
	for c := 0; c < 9; c++ {
		n := min(count, hand[c]-pay[c])
		pay[c] += n
		count -= n
	}
	return count == 0
}
func (g *Rail) paymentOptions(player int, r Route) []RailPayment {
	result := []RailPayment{}
	p := g.Players[player]
	if p.Eliminated || !g.routeOpen(r, player) || p.Trains < r.Length+r.Mountain {
		return result
	}
	for c := 0; c < 8; c++ {
		if r.Color >= 0 && r.Color != c {
			continue
		}
		var selected []int
		if r.Substitute == 4 {
			for replaced := 0; replaced <= r.Length; replaced++ {
				if p.Hand[c] < r.Length-replaced {
					continue
				}
				pay := make([]int, 9)
				pay[c] = r.Length - replaced
				if takeExtraCards(p.Hand, pay, 4*replaced) && g.validPayment(r, p.Hand, c, pay) {
					selected = pay
					break
				}
			}
		} else if r.Substitute == 3 && r.Ferry > 0 {
			for replaced := 0; replaced <= r.Ferry; replaced++ {
				wild := max(r.Ferry-replaced, r.Length-replaced-p.Hand[c])
				if wild > p.Hand[8] {
					continue
				}
				pay := make([]int, 9)
				pay[8] = wild
				pay[c] = r.Length - replaced - wild
				if takeExtraCards(p.Hand, pay, 3*replaced) && g.validPayment(r, p.Hand, c, pay) {
					selected = pay
					break
				}
			}
		} else {
			pay := make([]int, 9)
			pay[8] = max(r.Ferry, r.Length-p.Hand[c])
			pay[c] = r.Length - pay[8]
			if g.validPayment(r, p.Hand, c, pay) {
				selected = pay
			}
		}
		if selected != nil {
			result = append(result, RailPayment{c, selected})
		}
		if r.Tunnel && p.Hand[8] >= r.Length && (selected == nil || selected[8] != r.Length) {
			pay := make([]int, 9)
			pay[8] = r.Length
			result = append(result, RailPayment{c, pay})
		}
	}
	return result
}
func (g *Rail) spend(player int, pay []int) {
	for c, n := range pay {
		g.Players[player].Hand[c] -= n
		for range n {
			g.Discard = append(g.Discard, c)
		}
	}
}
func railPaymentText(pay []int) string {
	parts := []string{}
	for c, n := range pay {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s×%d", railCardName(c), n))
		}
	}
	return strings.Join(parts, "、")
}
func (s *State) claimRail(a Action) error {
	g := s.Rail
	p := &g.Players[s.Turn]
	route, ok := g.route(a.Route)
	if !ok {
		return errors.New("不存在这条路线")
	}
	if !g.routeOpen(route, s.Turn) {
		return errors.New("路线已占领，或本地图人数限制不允许使用该双线")
	}
	if p.Trains < route.Length+route.Mountain {
		return errors.New("剩余车厢不足，需同时保留山地额外消耗")
	}
	pay := railSimplePayment(a, route.Length)
	if !g.validPayment(route, p.Hand, a.Color, pay) {
		return errors.New("支付不符合本路线的颜色、渡轮或万能牌规则")
	}
	if !route.Tunnel {
		g.spend(s.Turn, pay)
		s.completeRailRoute(route, pay)
		return nil
	}
	// Keep the offered cards outside the discard pile until the tunnel resolves;
	// reshuffling a short draw pile must not reveal the offered cards themselves.
	for c, n := range pay {
		p.Hand[c] -= n
	}
	tunnel := &RailTunnel{Route: route.ID, Color: a.Color, Base: pay, Revealed: []int{}, WildOnly: pay[8] == route.Length}
	for range 3 {
		c, ok := g.draw()
		if !ok {
			break
		}
		tunnel.Revealed = append(tunnel.Revealed, c)
		if c == 8 || (!tunnel.WildOnly && c == a.Color) {
			tunnel.Extra++
		}
	}
	g.Tunnel = tunnel
	g.Passes = 0
	revealed := make([]int, 9)
	for _, c := range tunnel.Revealed {
		revealed[c]++
	}
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 尝试隧道 %s → %s；翻出 %s，追加费用 %d 张", s.Turn+1, g.data().Cities[route.A].Name, g.data().Cities[route.B].Name, railPaymentText(revealed), tunnel.Extra))
	if tunnel.Extra == 0 {
		return s.resolveRailTunnel(Action{Type: "tunnel_pay"})
	}
	s.Phase = "tunnel"
	return nil
}
func (s *State) resolveRailTunnel(a Action) error {
	g := s.Rail
	t := g.Tunnel
	if t == nil {
		return errors.New("当前没有待处理的隧道")
	}
	if a.Type != "tunnel_pay" && a.Type != "tunnel_cancel" {
		return errors.New("请先追加隧道费用或放弃本次铺路")
	}
	p := &g.Players[s.Turn]
	if a.Type == "tunnel_cancel" {
		for c, n := range t.Base {
			p.Hand[c] += n
		}
		g.Discard = append(g.Discard, t.Revealed...)
		g.Tunnel = nil
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 放弃隧道，已退回原支付牌，本回合结束", s.Turn+1))
		s.refillRailMarket()
		s.railNext()
		return nil
	}
	paymentAction := a
	paymentAction.Color = t.Color
	pay := railSimplePayment(paymentAction, t.Extra)
	if !railHeld(p.Hand, pay) || sum(pay) != t.Extra || pay[t.Color]+pay[8] != t.Extra || (t.WildOnly && pay[8] != t.Extra) {
		return errors.New("隧道追加牌不足或颜色不符")
	}
	g.spend(s.Turn, pay)
	total := append([]int{}, t.Base...)
	for c, n := range t.Base {
		for range n {
			g.Discard = append(g.Discard, c)
		}
		total[c] += pay[c]
	}
	g.Discard = append(g.Discard, t.Revealed...)
	route, _ := g.route(t.Route)
	g.Tunnel = nil
	s.completeRailRoute(route, total)
	return nil
}
func (s *State) completeRailRoute(route Route, pay []int) {
	g := s.Rail
	p := &g.Players[s.Turn]
	p.Trains -= route.Length + route.Mountain
	points := railRoutePoints(route.Length) + 2*route.Mountain
	p.RouteScore += points
	p.Score = p.RouteScore
	if route.Mountain > 0 {
		p.MountainTrains += route.Mountain
		p.MountainRoutes++
	}
	g.Owners[route.ID] = s.Turn
	g.Passes = 0
	detail := ""
	if route.Mountain > 0 {
		detail = fmt.Sprintf("，山地额外消耗 %d 节、奖励 %d 分", route.Mountain, route.Mountain*2)
	}
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 铺设了 %s → %s（%d 节%s）；支付 %s；获得 %d 分，剩余 %d 节车厢", s.Turn+1, g.data().Cities[route.A].Name, g.data().Cities[route.B].Name, route.Length, detail, railPaymentText(pay), points, p.Trains))
	s.refillRailMarket()
	s.railNext()
}
func (s *State) buildRailStation(a Action) error {
	g := s.Rail
	p := &g.Players[s.Turn]
	if g.info().Stations == 0 || len(p.Stations) >= g.info().Stations {
		return errors.New("没有可建造的车站")
	}
	if a.Vertex < 0 || a.Vertex >= len(g.data().Cities) {
		return errors.New("无效城市")
	}
	for _, other := range g.Players {
		for _, city := range other.Stations {
			if city == a.Vertex {
				return errors.New("该城市已有车站")
			}
		}
	}
	count := len(p.Stations) + 1
	pay := railSimplePayment(a, count)
	if a.Color < 0 || a.Color > 7 || !railHeld(p.Hand, pay) || sum(pay) != count || pay[a.Color]+pay[8] != count {
		return errors.New("车站需要对应数量的同色牌，可用万能牌补足")
	}
	g.spend(s.Turn, pay)
	p.Stations = append(p.Stations, a.Vertex)
	g.Passes = 0
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 在 %s 建造了第 %d 座车站；支付 %s", s.Turn+1, g.data().Cities[a.Vertex].Name, len(p.Stations), railPaymentText(pay)))
	s.refillRailMarket()
	s.railNext()
	return nil
}
