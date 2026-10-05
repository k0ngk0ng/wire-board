package game

import (
	"errors"
	"slices"
)

const CatanCitiesKnightsRules = "catan-knights-2025"

// Cards 0–4 remain ordinary resources. Terrain IDs are a separate domain:
// desert/sea/gold must never be interpreted as commodity card types.
const (
	CatanPaper          = 5
	CatanCommodityCloth = 6
	CatanCoin           = 7
	CatanScience        = 0
	CatanCommerce       = 1
	CatanPolitics       = 2
)

var catanCommodities = []string{"纸张", "布料", "钱币"}
var catanCityTracks = []string{"科学", "贸易", "政治"}

type CatanCityPlayer struct {
	Progress       []int  `json:"progress"`
	PublicProgress []int  `json:"publicProgress"`
	Improvements   [3]int `json:"improvements"`
	DefenderPoints int    `json:"defenderPoints"`
	ProgressPoints int    `json:"progressPoints"`
}
type CatanCityPending struct {
	Target  int          `json:"target"`
	Color   int          `json:"color"`
	Knight  *CatanKnight `json:"knight,omitempty"`
	Kind    string       `json:"kind"`
	Players []int        `json:"players"`
	Track   int          `json:"track"`
}
type CatanCitiesKnights struct {
	Layout            string            `json:"layout"`
	Merchant          *CatanMerchant    `json:"merchant,omitempty"`
	TradePowers       *CatanTradePowers `json:"tradePowers,omitempty"`
	EventDie          int               `json:"eventDie"`
	ProgressDecks     [3][]int          `json:"progressDecks"`
	Event             *CatanCityEvent   `json:"event,omitempty"`
	BarbarianPosition int               `json:"barbarianPosition"`
	RobberStart       int               `json:"robberStart"`
	FallenCities      []int             `json:"fallenCities"`
	Knights           []CatanKnight     `json:"knights"`
	ActionSerial      uint64            `json:"actionSerial"`
	Rules             string            `json:"rules"`
	Players           []CatanCityPlayer `json:"players"`
	Walls             []int             `json:"walls"`
	Metropolises      [3]int            `json:"metropolises"` // vertex IDs; -1 means unclaimed
	Invasions         int               `json:"invasions"`
	Pending           *CatanCityPending `json:"pending,omitempty"`
}

// Internal construction only. Room configuration and expansion UI are not
// exposed until complete combination and end-to-end acceptance.
func NewCatanCitiesKnights(n int, options CatanOptions) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("Helpers尚无与城市与骑士组合的官方兼容规则")
	}
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.CitiesKnights = &CatanCitiesKnights{Layout: "variable", Rules: catanCitiesKnightsRules(n), Players: make([]CatanCityPlayer, n), Walls: []int{}, Metropolises: [3]int{-1, -1, -1}, Knights: []CatanKnight{}, ActionSerial: 1}
	g.CitiesKnights.initProgress()
	g.CitiesKnights.EventDie = -1
	g.CitiesKnights.RobberStart = g.Robber
	g.CitiesKnights.FallenCities = []int{}
	commodities := 12
	if n > 4 {
		commodities = 18
	}
	g.Bank = append(g.Bank, commodities, commodities, commodities)
	for i := range g.Players {
		g.Players[i].Resources = make([]int, 8)
	}
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Robber = -1
	g.StartPlayer = catanRandom(n)
	s.Turn = g.StartPlayer
	if g.Paired != nil {
		g.Paired.Primary = g.StartPlayer
		g.Paired.Secondary = (g.StartPlayer + 3) % n
	}
	s.Log = []string{"城市与骑士开局：顺序放村庄，逆序放城市；第二座建筑只领取普通起始资源，13分获胜"}
	return s, nil
}
func catanCardName(card int) string {
	if card >= 5 && card < 8 {
		return catanCommodities[card-5]
	}
	if card >= 0 && card < 5 {
		return CatanResources[card]
	}
	return "未知卡牌"
}
func (g *Catan) cardBundle(cards []int) bool {
	if len(cards) != len(g.Bank) {
		return false
	}
	for _, n := range cards {
		if n < 0 || n > 24 {
			return false
		}
	}
	return true
}
func (g *Catan) catanDiscardLimit(player int) int {
	limit := 7
	if k := g.CitiesKnights; k != nil {
		for _, v := range k.Walls {
			if g.Vertices[v].Owner == player && g.Vertices[v].Level == 2 {
				limit += 2
			}
		}
	}
	return limit
}
func (g *Catan) cityProduction(claim []int, terrain, level int) {
	claim[terrain] += level
	if g.CitiesKnights == nil || level != 2 {
		return
	}
	commodity := -1
	switch terrain {
	case 0:
		commodity = CatanPaper
	case 2:
		commodity = CatanCommodityCloth
	case 4:
		commodity = CatanCoin
	}
	if commodity >= 0 {
		claim[terrain]--
		claim[commodity]++
	}
}
func (s *State) catanAfterSevenDiscards() {
	s.Phase = "catan_robber"
	if k := s.Catan.CitiesKnights; k != nil && k.Invasions == 0 {
		s.Phase = "catan_turn"
	}
}
func (g *Catan) cityMetropolisOwner(track int) int {
	if v := g.CitiesKnights.Metropolises[track]; v >= 0 && v < len(g.Vertices) {
		return g.Vertices[v].Owner
	}
	return -1
}
func (g *Catan) cityMetropolisSites(player int) []int {
	out := []int{}
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level == 2 && !slices.Contains(g.CitiesKnights.Metropolises[:], v.ID) {
			out = append(out, v.ID)
		}
	}
	return out
}
func (s *State) catanCityAction(player int, a Action) error {
	return s.catanCityBuild(player, a, 0)
}

// Discounts come only from validated progress-card play, never Action fields.
func (s *State) catanCityBuild(player int, a Action, discount int) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || s.Phase != "catan_turn" || player != s.Turn {
		return errors.New("当前不能进行城市建设")
	}
	if a.Type == "catan_wall" {
		v := a.Vertex
		if v < 0 || v >= len(g.Vertices) || g.Vertices[v].Owner != player || g.Vertices[v].Level != 2 || slices.Contains(k.Walls, v) {
			return errors.New("只能给自己的无城墙城市建造城墙")
		}
		if g.catanDiscardLimit(player) >= 13 {
			return errors.New("每人最多三座城墙")
		}
		cost := []int{0, max(0, 2-discount), 0, 0, 0}
		if !catanHas(g.Players[player].Resources, cost) {
			return errors.New("城墙需要两张砖块")
		}
		catanMove(g.Players[player].Resources, g.Bank, cost)
		k.Walls = append(k.Walls, v)
		g.Trade = nil
		s.catanLog(player, "支付砖块×%d，为城市 #%d 建造城墙，弃牌上限为%d张", cost[1], v+1, g.catanDiscardLimit(player))
		return nil
	}
	track := a.Color
	if a.Type != "catan_improvement" || track < 0 || track > 2 {
		return errors.New("请选择科学、贸易或政治建设")
	}
	_, _, cities := g.pieces(player)
	if cities == 0 {
		return errors.New("至少拥有一座城市才能继续城市建设")
	}
	next := k.Players[player].Improvements[track] + 1
	if next > 5 {
		return errors.New("该城市建设已达最高等级")
	}
	owner := g.cityMetropolisOwner(track)
	if next >= 4 && owner != player && len(g.cityMetropolisSites(player)) == 0 {
		return errors.New("需要一座未放置大都会的城市才能购买第四或第五级建设")
	}
	card := 5 + track
	cost := max(0, next-discount)
	if g.Players[player].Resources[card] < cost {
		return errors.New("相应商品不足")
	}
	g.Players[player].Resources[card] -= cost
	g.Bank[card] += cost
	k.Players[player].Improvements[track] = next
	g.Trade = nil
	s.catanLog(player, "支付%s×%d，将%s建设提升至%d级", catanCardName(card), cost, catanCityTracks[track], next)
	if next >= 4 && owner != player && (owner < 0 || k.Players[owner].Improvements[track] < next) {
		k.Pending = &CatanCityPending{Kind: "metropolis", Players: []int{player}, Track: track}
		s.Phase = "catan_metropolis"
	}
	return nil
}
func (s *State) catanStartAqueduct(received []int) {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil {
		return
	}
	q := &CatanCityPending{Kind: "aqueduct", Players: []int{}}
	for offset := range len(g.Players) {
		p := (s.Turn + offset) % len(g.Players)
		if !g.Players[p].Eliminated && k.Players[p].Improvements[CatanScience] >= 3 && received[p] == 0 {
			q.Players = append(q.Players, p)
		}
	}
	if len(q.Players) > 0 && sum(g.Bank[:5]) > 0 {
		k.Pending = q
		s.Phase = "catan_aqueduct"
	}
}
func (s *State) catanCityChoice(player int, a Action) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || k.Pending == nil || len(k.Pending.Players) == 0 || k.Pending.Players[0] != player {
		return errors.New("请等待对应玩家完成城市选择")
	}
	q := k.Pending
	switch q.Kind {
	case "diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place":
		return s.catanPoliticsChoice(player, a)
	case "guild_dues", "commercial_harbor":
		return s.catanTradeProgressChoice(player, a)
	case "pillage", "defender_reward", "progress_discard":
		return s.catanEventChoice(player, a)
	case "knight_retreat":
		if err := s.catanKnightRetreat(player, a); err != nil {
			return err
		}
	case "metropolis":
		if s.Phase != "catan_metropolis" || a.Type != "catan_metropolis" || !slices.Contains(g.cityMetropolisSites(player), a.Vertex) {
			return errors.New("请选择自己的无大都会城市")
		}
		old := g.cityMetropolisOwner(q.Track)
		k.Metropolises[q.Track] = a.Vertex
		if old >= 0 {
			s.catanLog(old, "失去%s大都会", catanCityTracks[q.Track])
		}
		s.catanLog(player, "将%s大都会放在城市 #%d，额外获得2分", catanCityTracks[q.Track], a.Vertex+1)
	case "aqueduct":
		if s.Phase != "catan_aqueduct" || a.Type != "catan_aqueduct" {
			return errors.New("请选择引水渠补偿资源")
		}
		if a.Choice == "skip" {
			s.catanLog(player, "放弃本次引水渠补偿")
		} else {
			if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] == 0 {
				return errors.New("引水渠只能领取银行现有的一张普通资源")
			}
			g.Bank[a.Color]--
			g.Players[player].Resources[a.Color]++
			s.catanLog(player, "通过引水渠领取%s×1", catanCardName(a.Color))
		}
	default:
		return errors.New("未知城市选择")
	}
	q.Players = q.Players[1:]
	if q.Kind == "aqueduct" && sum(g.Bank[:5]) == 0 {
		q.Players = nil
	}
	if len(q.Players) == 0 {
		k.Pending = nil
		s.Phase = "catan_turn"
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
func (s *State) catanCityChoiceBot(player int) (Action, error) {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || k.Pending == nil || len(k.Pending.Players) == 0 || k.Pending.Players[0] != player {
		return Action{}, errors.New("inactive city choice seat")
	}
	if slices.Contains([]string{"diplomacy", "espionage", "sabotage", "wedding", "treason_remove", "treason_place"}, k.Pending.Kind) {
		return s.catanPoliticsChoiceBot(player)
	}
	if k.Pending.Kind == "guild_dues" || k.Pending.Kind == "commercial_harbor" {
		return s.catanTradeProgressChoiceBot(player)
	}
	if k.Pending.Kind == "metropolis" {
		sites := g.cityMetropolisSites(player)
		if len(sites) > 0 {
			return Action{Type: "catan_metropolis", Vertex: sites[0]}, nil
		}
	}
	if k.Pending.Kind == "progress_discard" {
		hand := k.Players[player].Progress
		if len(hand) > 4 {
			return Action{Type: "catan_progress_discard", Cards: append([]int{}, hand[:len(hand)-4]...)}, nil
		}
	}
	if k.Pending.Kind == "pillage" {
		sites := g.pillageSites(player)
		if len(sites) > 0 {
			best := sites[0]
			for _, site := range sites {
				if g.vertexValue(player, site) < g.vertexValue(player, best) {
					best = site
				}
			}
			return Action{Type: "catan_pillage", Vertex: best}, nil
		}
	}
	if k.Pending.Kind == "defender_reward" {
		best := -1
		for track, deck := range k.ProgressDecks {
			if len(deck) > 0 && (best < 0 || k.Players[player].Improvements[track] > k.Players[player].Improvements[best]) {
				best = track
			}
		}
		if best >= 0 {
			return Action{Type: "catan_defender_reward", Color: best}, nil
		}
	}
	if k.Pending.Kind == "knight_retreat" && k.Pending.Knight != nil {
		sites := g.knightDestinations(*k.Pending.Knight, true)
		if len(sites) > 0 {
			return Action{Type: "catan_knight_retreat", Vertex: sites[0]}, nil
		}
	}
	if k.Pending.Kind == "aqueduct" {
		best := -1
		for c, n := range g.Bank[:5] {
			if n > 0 && (best < 0 || g.Players[player].Resources[c] < g.Players[player].Resources[best]) {
				best = c
			}
		}
		if best >= 0 {
			return Action{Type: "catan_aqueduct", Color: best}, nil
		}
		return Action{Type: "catan_aqueduct", Choice: "skip"}, nil
	}
	return Action{}, errors.New("no legal city choice")
}

// Economic priorities use only public improvements/cities and this seat's
// hand. Knight priorities are separate; progress-card priorities are still pending.
func (g *Catan) cityEconomyBotChoices(player int) []botChoice {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	_, _, cities := g.pieces(player)
	if cities == 0 {
		return nil
	}
	choices := []botChoice{}
	for track, level := range k.Players[player].Improvements {
		if level == 5 || (level >= 3 && g.cityMetropolisOwner(track) != player && len(g.cityMetropolisSites(player)) == 0) {
			continue
		}
		score := 350
		if track == CatanScience && level < 3 {
			score = 650 // Secure production compensation early.
		}
		owner := g.cityMetropolisOwner(track)
		if level >= 3 && (owner < 0 || (owner != player && k.Players[owner].Improvements[track] < level+1)) {
			score = 720
		}
		choices = append(choices, botChoice{Action{Type: "catan_improvement", Color: track}, score})
	}
	if g.catanDiscardLimit(player) < 13 {
		for _, v := range g.Vertices {
			if v.Owner == player && v.Level == 2 && !slices.Contains(k.Walls, v.ID) {
				score := 140
				if sum(g.Players[player].Resources) > g.catanDiscardLimit(player) {
					score = 550
				}
				choices = append(choices, botChoice{Action{Type: "catan_wall", Vertex: v.ID}, score})
				break
			}
		}
	}
	return choices
}
