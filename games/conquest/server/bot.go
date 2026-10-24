package conquest

// The policy is an independent, deterministic heuristic. It consumes only the
// same projected view as a human seat; it does not reproduce the original AI.
import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/k0ngk0ng/wire-board/games/conquest/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type botProvince struct {
	ID         string `json:"id"`
	Owner      int    `json:"owner"`
	Troops     int    `json:"troops"`
	Mobile     int    `json:"mobile"`
	Town       bool   `json:"town"`
	Fort       bool   `json:"fort"`
	Tower      bool   `json:"tower"`
	Population int    `json:"population"`
}
type botPlayer struct {
	Seat       int     `json:"seat"`
	Gold       float64 `json:"gold"`
	Income     float64 `json:"income"`
	Upkeep     float64 `json:"upkeep"`
	King       string  `json:"king"`
	Submitted  bool    `json:"submitted"`
	Eliminated bool    `json:"eliminated"`
}
type botOrder struct {
	Type     string `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	Province string `json:"province"`
	Amount   int    `json:"amount"`
	King     bool   `json:"king"`
}
type botView struct {
	Session string `json:"session"`
	Epoch   int    `json:"epoch"`
	Round   int    `json:"round"`
	Phase   string `json:"phase"`
	MapID   string `json:"mapId"`
	Options struct {
		Difficulty string `json:"difficulty"`
		Fog        bool   `json:"fog"`
	} `json:"options"`
	Provinces []botProvince `json:"provinces"`
	Projected []botProvince `json:"projected"`
	Players   []botPlayer   `json:"players"`
	Orders    []botOrder    `json:"orders"`
	Events    []struct {
		Type     string       `json:"type"`
		Seat     int          `json:"seat"`
		To       string       `json:"to"`
		Amount   int          `json:"amount"`
		Captured bool         `json:"captured"`
		Before   *botProvince `json:"before"`
		After    *botProvince `json:"after"`
	} `json:"events"`
	AvailableGold     float64 `json:"availableGold"`
	AvailableMovement float64 `json:"availableMovement"`
	Controls          struct {
		CanPlan         bool    `json:"canPlan"`
		CanSubmit       bool    `json:"canSubmit"`
		MaxOrders       int     `json:"maxOrders"`
		RecruitCost     float64 `json:"recruitCost"`
		MoveCost        float64 `json:"moveCost"`
		RecruitMovement float64 `json:"recruitMovement"`
		TowerCost       float64 `json:"towerCost"`
		TowerMovement   float64 `json:"towerMovement"`
	} `json:"controls"`
}
type botLink struct {
	to  string
	sea bool
}
type botGeography struct {
	links map[string][]botLink
	ids   []string
}

var botGeographies sync.Map

func botMap(id string) (*botGeography, error) {
	if old, ok := botGeographies.Load(id); ok {
		return old.(*botGeography), nil
	}
	m, err := maps.Load(id)
	if err != nil {
		return nil, err
	}
	g := &botGeography{links: map[string][]botLink{}}
	for _, p := range m.Provinces {
		g.ids = append(g.ids, p.ID)
		for _, n := range p.Neighbors {
			g.links[p.ID] = append(g.links[p.ID], botLink{to: n})
		}
		for _, n := range p.SeaNeighbors {
			g.links[p.ID] = append(g.links[p.ID], botLink{to: n, sea: true})
		}
		sort.Slice(g.links[p.ID], func(i, j int) bool { return g.links[p.ID][i].to < g.links[p.ID][j].to })
	}
	sort.Strings(g.ids)
	actual, _ := botGeographies.LoadOrStore(id, g)
	return actual.(*botGeography), nil
}

// bot deliberately crosses the view serialization boundary before considering
// a move. Future orders, random state and timer queues are inaccessible here.
func bot(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	if c.Actor < 0 || c.Actor >= c.Players || m.Result != nil {
		return nil, nil
	}
	projected, err := Module().View(m, c.Actor)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(projected)
	if err != nil {
		return nil, err
	}
	var v botView
	if err = json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	if v.Phase != "planning" || c.Actor >= len(v.Players) || v.Players[c.Actor].Eliminated || v.Players[c.Actor].Submitted {
		return nil, nil
	}
	action := func(a map[string]any) (json.RawMessage, error) {
		a["session"] = v.Session
		a["epoch"] = v.Epoch
		return json.Marshal(a)
	}
	submit := func() (json.RawMessage, error) {
		if !v.Controls.CanSubmit {
			return nil, nil
		}
		return action(map[string]any{"type": "submit"})
	}
	if !v.Controls.CanPlan || (v.Controls.MaxOrders > 0 && len(v.Orders) >= v.Controls.MaxOrders) {
		return submit()
	}
	g, err := botMap(v.MapID)
	if err != nil {
		return nil, err
	}
	provinces := map[string]botProvince{}
	for _, p := range v.Provinces {
		provinces[p.ID] = p
	}
	for _, p := range v.Projected {
		if p.Owner == c.Actor {
			provinces[p.ID] = p
		}
	}
	kings := map[string]int{}
	for _, p := range v.Players {
		if !p.Eliminated && p.King != "" {
			kings[p.King] = p.Seat
		}
	}
	// Once the king is committed to a march, it cannot lead another planned
	// march in the same round, even if the destination is already friendly.
	ownKing := v.Players[c.Actor].King
	moved, recruited, targeted := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, o := range v.Orders {
		if o.Type == "move" {
			moved[o.From] = true
			targeted[o.To] = true
			if o.King {
				ownKing = ""
			}
		}
		if o.Type == "recruit" {
			recruited[o.Province] = true
		}
	}
	// Distances depend only on public ownership. Rival land is preferred over
	// aimless expansion; neutral regions form corridors, not artificial goals.
	rivalDistance := map[string]int{}
	queue := []string{}
	for _, id := range g.ids {
		p := provinces[id]
		if p.Owner >= 0 && p.Owner != c.Actor {
			rivalDistance[id] = 0
			queue = append(queue, id)
		}
	}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		for _, l := range g.links[id] {
			if _, ok := rivalDistance[l.to]; !ok {
				rivalDistance[l.to] = rivalDistance[id] + 1
				queue = append(queue, l.to)
			}
		}
	}
	if len(queue) == 0 {
		return submit()
	}
	// Past failed expeditions are part of this seat's own battle reports.
	// They raise a scouting estimate without consulting hidden enemy armies.
	failedExpeditions := map[string]int{}
	visibleReplacement := map[string]int{}
	for _, e := range v.Events {
		if e.Type == "move" && e.Seat == c.Actor && !e.Captured && provinces[e.To].Owner != c.Actor {
			failedExpeditions[e.To] = max(failedExpeditions[e.To], e.Amount)
			// A failed attack can expose reinforcements that were absent from
			// the earlier planning view. Learn only from this seat's visible
			// battle report, never from the opponent's private orders or gold.
			if e.Before != nil && e.After != nil && e.Before.Owner >= 0 && e.Before.Owner == provinces[e.To].Owner && e.After.Owner == e.Before.Owner && e.Before.Troops >= 0 && e.After.Troops >= 0 {
				visibleReplacement[e.To] = max(visibleReplacement[e.To], e.Before.Troops-e.After.Troops)
			}
		}
	}
	defenseEstimate := func(p botProvince) float64 {
		if p.Troops < 0 {
			if p.Owner < 0 {
				return 500
			} // Public scenario rule: neutral initial garrison.
			return math.Max(3250, float64(failedExpeditions[p.ID])*1.8)
		}
		factor := 1.0
		if p.Town {
			factor += .5
		}
		if p.Fort {
			factor += .8
		}
		if owner, ok := kings[p.ID]; ok && owner == p.Owner {
			factor += .3
		}
		replacements := 0
		if p.Owner >= 0 {
			replacements = min(visibleReplacement[p.ID], max(0, p.Population/2))
		}
		return math.Round(float64(p.Troops+replacements) * factor)
	}
	frontline := map[string]bool{}
	for _, id := range g.ids {
		if provinces[id].Owner == c.Actor {
			for _, l := range g.links[id] {
				if provinces[l.to].Owner != c.Actor {
					frontline[id] = true
					break
				}
			}
		}
	}
	frontlineDistance := map[string]int{}
	queue = queue[:0]
	for _, id := range g.ids {
		if frontline[id] {
			frontlineDistance[id] = 0
			queue = append(queue, id)
		}
	}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		for _, l := range g.links[id] {
			if provinces[l.to].Owner == c.Actor {
				if _, ok := frontlineDistance[l.to]; !ok {
					frontlineDistance[l.to] = frontlineDistance[id] + 1
					queue = append(queue, l.to)
				}
			}
		}
	}
	type choice struct {
		score  float64
		key    string
		action map[string]any
	}
	best := choice{score: math.Inf(-1)}
	choose := func(score float64, key string, a map[string]any) {
		if score > best.score || score == best.score && key < best.key {
			best = choice{score, key, a}
		}
	}
	difficulty := v.Options.Difficulty
	aggression := 1.15
	if difficulty == "easy" {
		aggression = 1.4
	}
	if difficulty == "hard" {
		aggression = 1.06
	}
	recruitCost := v.Controls.RecruitCost
	if recruitCost <= 0 {
		recruitCost = 1
	}
	moveCost := v.Controls.MoveCost
	if moveCost <= 0 {
		moveCost = 1
	}
	recruitMovement := v.Controls.RecruitMovement
	if recruitMovement <= 0 {
		recruitMovement = 1
	}
	gold := math.Max(0, v.AvailableGold)
	minReserve := math.Max(0, v.Players[c.Actor].Upkeep-v.Players[c.Actor].Income)
	recruitBudget := math.Max(0, gold-minReserve)
	for _, id := range g.ids {
		from := provinces[id]
		if from.Owner != c.Actor {
			continue
		}
		mobile := from.Mobile
		if mobile < 0 {
			mobile = 0
		}
		attackFactor := 1.0
		bringKing := ownKing == id
		if bringKing {
			attackFactor = 1.3
		}
		// Unobserved garrisons are not zero-strength targets. A watchtower
		// obtains real next-round information through a normal legal order.
		if v.Options.Fog && !from.Tower && v.Controls.TowerCost > 0 && gold >= v.Controls.TowerCost && v.AvailableMovement+1e-8 >= v.Controls.TowerMovement {
			for _, link := range g.links[id] {
				if !link.sea && provinces[link.to].Owner != c.Actor && provinces[link.to].Troops < 0 {
					choose(45000-float64(rivalDistance[id])*100, "scout:"+id, map[string]any{"type": "build", "province": id, "building": "tower"})
					break
				}
			}
		}
		// Preserve strategic intent across successive callback invocations: one
		// forward transfer per source avoids route cycles and split micro-orders.
		if mobile > 0 && !moved[id] && v.AvailableMovement+1e-8 >= moveCost {
			for _, link := range g.links[id] {
				to := provinces[link.to]
				amount := mobile
				if link.sea {
					amount = min(amount, int(gold/2))
				}
				if amount <= 0 {
					continue
				}
				if to.Owner == c.Actor {
					// Rear garrisons join the closest border. An idle frontier stack may
					// join a stronger adjacent front when it cannot break any enemy itself.
					currentDistance, hereOK := frontlineDistance[id]
					nextDistance, nextOK := frontlineDistance[to.ID]
					if hereOK && nextOK && nextDistance < currentDistance {
						score := 11000.0 - float64(rivalDistance[to.ID])*90 + float64(amount)*.02
						if link.sea {
							score -= 600
						}
						choose(score, id+">"+to.ID, map[string]any{"type": "move", "from": id, "to": to.ID, "amount": amount, "king": bringKing})
					}
					continue
				}
				if targeted[to.ID] {
					continue
				}
				defended := defenseEstimate(to)
				if float64(amount)*attackFactor <= defended {
					continue
				}
				// A decisive margin avoids successful but useless one-soldier captures.
				// Zero-garrison targets never require an arbitrary minimum force.
				if defended > 0 && float64(amount)*attackFactor < defended*aggression+1 {
					continue
				}
				score := 40000.0 - float64(rivalDistance[to.ID])*1000 + math.Min(float64(amount), 100000)*.03
				if to.Owner >= 0 {
					score += 7000
				}
				if owner, ok := kings[to.ID]; ok && owner != c.Actor {
					score += 15000
				}
				if link.sea {
					score -= 750
				}
				if from.Town && bringKing && to.Owner == -1 {
					score -= 300
				}
				choose(score, id+">"+to.ID, map[string]any{"type": "move", "from": id, "to": to.ID, "amount": amount, "king": bringKing})
			}
		}
		if !frontline[id] || recruited[id] || moved[id] || v.AvailableMovement+1e-8 < recruitMovement {
			continue
		}
		// Recruit where the additional force can create a useful next attack.
		// One recruitment per province per plan keeps movement points available.
		required := math.Inf(1)
		desiredSea := false
		danger := 0.0
		for _, link := range g.links[id] {
			enemy := provinces[link.to]
			if enemy.Owner == c.Actor || targeted[enemy.ID] {
				continue
			}
			want := math.Ceil(defenseEstimate(enemy)/attackFactor*aggression) + 100
			if want < required {
				required = want
				desiredSea = link.sea
			}
			if enemy.Owner >= 0 {
				danger = math.Max(danger, float64(enemy.Troops)*1.3)
			}
		}
		if math.IsInf(required, 1) {
			continue
		}
		// At quiet frontiers maintain a modest expeditionary group. Avoid raising
		// population-sized armies merely because gold is available.
		required = math.Max(required, 600)
		if difficulty == "hard" {
			required = math.Max(required, 1000)
		}
		need := int(required) - mobile
		if need <= 0 {
			continue
		}
		budget := recruitBudget / recruitCost
		if desiredSea {
			budget = math.Max(0, (gold-2*float64(mobile))/(recruitCost+2))
		}
		amount := min(need, int(budget), from.Population/2, maxTroops-from.Troops)
		if amount <= 0 {
			continue
		}
		score := 18000.0 - float64(rivalDistance[id])*120 + float64(amount)*.05
		if bringKing && danger > float64(from.Troops)*1.5 {
			score = 65000 + float64(amount)*.1
		}
		choose(score, "recruit:"+id, map[string]any{"type": "recruit", "province": id, "amount": amount})
	}
	if best.action != nil {
		return action(best.action)
	}
	// Stuck adjacent fronts concentrate instead of forever matching the opposing
	// garrison. Move toward the stronger friendly stack, with a stable ID tie
	// break so two stacks never exchange places within the same round.
	if v.AvailableMovement+1e-8 >= moveCost {
		for _, id := range g.ids {
			from := provinces[id]
			if from.Owner != c.Actor || from.Mobile <= 0 || moved[id] || !frontline[id] {
				continue
			}
			for _, l := range g.links[id] {
				to := provinces[l.to]
				if to.Owner != c.Actor || !frontline[to.ID] || moved[to.ID] {
					continue
				}
				if to.Mobile < from.Mobile || to.Mobile == from.Mobile && to.ID > id {
					continue
				}
				amount := from.Mobile
				if l.sea {
					amount = min(amount, int(gold/2))
				}
				if amount <= 0 {
					continue
				}
				choose(float64(to.Mobile)+float64(amount)*.1, "merge:"+id+">"+to.ID, map[string]any{"type": "move", "from": id, "to": to.ID, "amount": amount, "king": ownKing == id})
			}
		}
	}
	if best.action != nil {
		return action(best.action)
	}
	if !v.Controls.CanSubmit {
		return nil, fmt.Errorf("电脑没有合法行动且无法提交")
	}
	return submit()
}
