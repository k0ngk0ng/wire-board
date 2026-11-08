package stoneage

import (
	"encoding/json"
	"slices"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Suggest uses exactly a seat's public/private View. Neither future plans,
// opponent inventories nor the engine RNG are available to this strategy.
// The same legal actions are used by humans, bots and temporary autoplay.
func Suggest(v View) *Action {
	seat := v.Viewer
	if seat < 0 || seat >= len(v.Players) || v.Self == nil {
		return nil
	}
	p := v.Players[seat]
	a := &Action{Session: v.Session, Window: v.Window}
	if v.Phase == "arming" {
		if p.BattleReady {
			return nil
		}
		a.Type = "battle_ready"
		return a
	}
	if v.Phase == "selection" {
		if p.Locked {
			return nil
		}
		if v.Self.Loadout != nil {
			a.Type = "lock"
			return a
		}
		pool := v.Pool
		ride := pool.Rides[seat%len(pool.Rides)]
		pets := reliablePets(pool.Pets)
		l := &Loadout{Pet: pets[(seat*3)%len(pets)].ID, Ride: ride.ID, Avatar: ride.Riders[seat%len(ride.Riders)].Avatar, Points: [][]int{{140, 120, 100, 120}, {120, 180, 100, 80}, {110, 110, 80, 180}, {170, 70, 120, 120}}[seat%4], Hat: pool.Hats[seat%len(pool.Hats)].ID, Weapon: pool.Weapons[seat%len(pool.Weapons)].ID, Clothes: pool.Clothes[seat%len(pool.Clothes)].ID, Accessories: []int{pool.Accessories[seat%len(pool.Accessories)].ID, pool.Accessories[(seat+1)%len(pool.Accessories)].ID}, HPItem: pool.HPItems[seat%len(pool.HPItems)].ID, MPItem: pool.MPItems[seat%len(pool.MPItems)].ID, CureItem: pool.CureItems[seat%len(pool.CureItems)].ID, Tactics: []int{pool.Tactics[seat%len(pool.Tactics)].ID, pool.Tactics[(seat+1)%len(pool.Tactics)].ID}, Skills: []int{1, 2}}
		families := map[string]bool{"attack": true, "guard": true}
		// Different seats cover burst, control and protection. Strongest
		// variants within one family never occupy duplicate learning slots.
		candidates := append([]Skill(nil), pool.Skills...)
		slices.SortStableFunc(candidates, func(a, b Skill) int {
			score := func(s Skill) int {
				x := s.Power * max(1, s.Hits)
				if s.Status != "" {
					x += 45
				}
				if s.ID == 210 {
					x += 100
				}
				if s.ID == 20 && seat%3 == 0 {
					x += 90
				}
				return x
			}
			return score(b) - score(a)
		})
		for _, skill := range candidates {
			if len(l.Skills) == 7 {
				break
			}
			if !families[skill.Family] {
				l.Skills = append(l.Skills, skill.ID)
				families[skill.Family] = true
			}
		}
		a.Type = "loadout"
		a.Loadout = l
		return a
	}
	if v.Phase != "planning" || p.Submitted || p.Body.HP <= 0 || p.Surrendered {
		return nil
	}
	enemy := -1
	best := 1 << 30
	for _, other := range v.Players {
		if other.Team == p.Team || other.Body.HP <= 0 || other.Surrendered {
			continue
		}
		score := other.Body.HP
		if other.Riding {
			score += other.Ride.HP / 3
		}
		if score < best {
			best = score
			enemy = other.Body.ID
		}
	}
	if enemy < 0 {
		return nil
	}
	plan := &Plan{Player: Command{Kind: "attack", Target: enemy}, Pet: Command{Kind: "wait", Target: p.Pet.ID}}
	inventory := v.Self.Inventory
	itemOf := func(kind string) (Item, bool) {
		for _, row := range inventory {
			if row.Count > 0 && row.Item.Kind == kind {
				return row.Item, true
			}
		}
		return Item{}, false
	}
	badTarget := -1
	healTarget := -1
	worst := 101
	for _, other := range v.Players {
		if other.Team != p.Team || other.Body.HP <= 0 {
			continue
		}
		for _, u := range []Unit{other.Body, other.Pet} {
			if u.HP <= 0 || u.Fled {
				continue
			}
			if u.Status.Stone > 0 || u.Status.Sleep > 0 || u.Status.Confusion > 0 {
				badTarget = u.ID
			}
			ratio := 100 * u.HP / max(1, u.MaxHP)
			if ratio < worst {
				worst = ratio
				healTarget = u.ID
			}
		}
	}
	handled := false
	if item, ok := itemOf("cure"); ok && badTarget >= 0 && badTarget != p.Body.ID {
		plan.Player = Command{Kind: "item", ID: item.ID, Target: badTarget}
		handled = true
	}
	if !handled && worst < 45 {
		if item, ok := itemOf("hp"); ok {
			plan.Player = Command{Kind: "item", ID: item.ID, Target: healTarget}
			handled = true
		}
	}
	if !handled {
		for _, magic := range v.Self.Magic {
			if magic.MP > p.Body.MP {
				continue
			}
			if magic.Kind == "heal" && worst < 65 {
				plan.Player = Command{Kind: "magic", ID: magic.ID, Target: healTarget}
				handled = true
				break
			}
			if magic.Kind == "cure" && badTarget >= 0 && badTarget != p.Body.ID {
				plan.Player = Command{Kind: "magic", ID: magic.ID, Target: badTarget}
				handled = true
				break
			}
		}
	}
	if !handled && p.Body.MP < 20 && len(v.Self.Magic) > 0 {
		if item, ok := itemOf("mp"); ok {
			plan.Player = Command{Kind: "item", ID: item.ID, Target: p.Body.ID}
			handled = true
		}
	}
	if !handled && p.Body.Status.Shield == 0 && p.Body.HP < p.Body.MaxHP/2 {
		if item, ok := itemOf("shield"); ok {
			plan.Player = Command{Kind: "item", ID: item.ID, Target: p.Body.ID}
			handled = true
		}
	}
	if p.Pet.HP > 0 && !p.Pet.Fled {
		score := -1
		chosen := 1
		for _, id := range v.Self.Loadout.Skills {
			skill, ok := skillByID(v.Pool.Skills, id)
			if !ok || id == 2 {
				continue
			}
			value := skill.Power * max(1, skill.Hits)
			target := v.Players[enemy/2]
			if id == 210 && target.Riding {
				value += 150
			}
			if id == 3 && target.Body.Guard {
				value += 200
			}
			if skill.Status != "" && target.Body.Status.Immune == 0 {
				value += 40
			}
			if id == 20 && p.Body.HP < p.Body.MaxHP/3 {
				value += 200
			}
			if value > score {
				score = value
				chosen = id
			}
		}
		plan.Pet = Command{Kind: "skill", ID: chosen, Target: enemy}
	}
	a.Type = "submit"
	a.Plan = plan
	return a
}
func botAction(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	projected, e := makeView(m, c.Actor)
	if e != nil {
		return nil, e
	}
	a := Suggest(projected.(View))
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}
