package stoneage

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// The following primitives follow the classic 2.5 source's active
// _BATTLE_NEWPOWER branch: char.c CHAR_initcharWorkInt and battle_event.c
// BATTLE_adjustRidePet3A, BATTLE_DamageCalc, BATTLE_AttrCalc, BATTLE_GuardAdjust.
// Arena budgets, usable content and finite consumables are explicit below.
func unitFromBuild(b [4]int) Unit {
	v, a, d, q := b[0], b[1], b[2], b[3]
	hp := v*4 + a + d + q
	return Unit{Loyalty: 100, HP: hp, MaxHP: hp, MP: 120, MaxMP: 120, Attack: int(float64(a) + float64(d+v)*.1 + float64(q)*.05), Defense: int(float64(d) + float64(a+v)*.1 + float64(q)*.05), Speed: q}
}
func budget(weights [4]int, total int) [4]int {
	sum := 0
	for _, v := range weights {
		sum += max(1, v)
	}
	out := [4]int{}
	used := 0
	for i, v := range weights {
		out[i] = max(1, v) * total / sum
		used += out[i]
	}
	out[0] += total - used
	return out
}
func rideStats(body, ride Unit, throw bool) (int, int, int) {
	if ride.HP <= 0 {
		return body.Attack, body.Defense, body.Speed
	}
	a := int(float64(body.Attack+ride.Attack) * .8)
	q := int(float64(body.Speed)*.2 + float64(ride.Speed)*.8)
	if throw {
		a = body.Attack + int(float64(ride.Attack)*.4)
		q = int(float64(body.Speed)*.8 + float64(ride.Speed)*.2)
	}
	return a, int(float64(body.Defense+ride.Defense) * .7), q
}
func physicalBase(attack, defense float64, roll int) int {
	fraction := float64(roll) / 10000
	if attack <= 0 {
		return 0
	}
	if defense > attack {
		return int(fraction * 2)
	}
	if attack < defense*8/7 {
		return int(fraction * (attack/16 + 1))
	}
	variance := float64(int(fraction*(attack/8+1))) - attack/16
	return max(0, int((attack-defense)*2+variance))
}
func elementMultiplier(a, d [4]int) float64 {
	// Earth > Water > Fire > Wind > Earth. Neutral is disadvantaged
	// against an elemental attacker, matching AJ_UP/AJ_DOWN in the source.
	aa, dd := [5]int{}, [5]int{}
	an, dn := 100, 100
	for i := range a {
		aa[i] = max(0, a[i])
		dd[i] = max(0, d[i])
		an -= aa[i]
		dn -= dd[i]
	}
	aa[4] = max(0, an)
	dd[4] = max(0, dn)
	total := float64(0)
	for i, av := range aa {
		for j, dv := range dd {
			factor := 1.0
			if i < 4 && j == 4 || i < 4 && j < 4 && (i+1)%4 == j {
				factor = 1.5
			} else if i == 4 && j < 4 || i < 4 && j < 4 && (j+1)%4 == i {
				factor = .6
			}
			total += float64(av*dv) * factor
		}
	}
	return total / 10000
}
func guardDamage(damage, roll int) int {
	factor := 0.
	switch {
	case roll <= 25:
		factor = 0
	case roll <= 50:
		factor = .1
	case roll <= 70:
		factor = .2
	case roll <= 85:
		factor = .3
	case roll <= 95:
		factor = .4
	default:
		factor = .5
	}
	return int(float64(damage) * factor)
}
func (s *State) initializePlayer(i int) {
	p := &s.Players[i]
	l := *p.Loadout
	var allocated [4]int
	copy(allocated[:], l.Points)
	p.Body = unitFromBuild(allocated)
	p.Body.ID = i * 2
	p.Body.Name = "人物"
	p.Body.Graphic = l.Avatar
	pet, _ := petByID(s.Pool.Pets, l.Pet)
	p.Pet = unitFromBuild(budget(pet.Weights, 640))
	p.Pet.ID = i*2 + 1
	p.Pet.Name = pet.Name
	p.Pet.Loyalty = pet.Loyalty
	p.Pet.Graphic = pet.Graphic
	p.Pet.Elements = pet.Elements
	ride, _ := petByID(s.Pool.Rides, l.Ride)
	p.Ride = unitFromBuild(budget(ride.Weights, 400))
	p.Ride.ID = 100 + i
	p.Ride.Name = ride.Name
	p.Ride.Loyalty = ride.Loyalty
	p.Ride.Graphic = ride.Graphic
	p.Ride.Elements = ride.Elements
	p.Riding = true
	p.Body.Elements = [4]int{25, 25, 25, 25}
	p.Magic = []Magic{}
	for _, g := range s.Pool.gear(l) {
		p.Body.Attack += g.Attack
		p.Body.Defense += g.Defense
		p.Body.Speed += g.Speed
		p.Body.MaxHP += g.HP
		p.Body.MaxMP += g.MP
		if g.Magic != nil && !slices.ContainsFunc(p.Magic, func(m Magic) bool { return m.ID == g.Magic.ID }) {
			p.Magic = append(p.Magic, *g.Magic)
		}
	}
	weapon, _ := gearByID(s.Pool.Weapons, l.Weapon)
	if weapon.WeaponType >= 0 && weapon.WeaponType <= 4 {
		p.Body.Graphic = l.Avatar + weapon.WeaponType
	}
	p.Body.Attack = max(1, p.Body.Attack)
	p.Body.Defense = max(1, p.Body.Defense)
	p.Body.Speed = max(1, p.Body.Speed)
	p.Body.HP = p.Body.MaxHP
	p.Body.MP = p.Body.MaxMP
	p.Inventory = []InventoryItem{}
	for _, entry := range []struct {
		rows      []Item
		id, count int
	}{{s.Pool.HPItems, l.HPItem, 3}, {s.Pool.MPItems, l.MPItem, 2}, {s.Pool.CureItems, l.CureItem, 1}} {
		item, _ := itemByID(entry.rows, entry.id)
		p.Inventory = append(p.Inventory, InventoryItem{Item: item, Count: entry.count})
	}
	for _, id := range l.Tactics {
		item, _ := itemByID(s.Pool.Tactics, id)
		p.Inventory = append(p.Inventory, InventoryItem{Item: item, Count: 1})
	}
}
func (s *State) unit(id int) *Unit {
	if id < 0 || id >= len(s.Players)*2 {
		return nil
	}
	p := &s.Players[id/2]
	if id%2 == 0 {
		return &p.Body
	}
	return &p.Pet
}
func (s *State) active(id int) bool {
	u := s.unit(id)
	return u != nil && u.HP > 0 && !u.Fled && s.Players[id/2].Body.HP > 0 && !s.Players[id/2].Surrendered
}
func (s *State) friendly(a, b int) bool {
	return a >= 0 && b >= 0 && a/2 < len(s.Players) && b/2 < len(s.Players) && s.Players[a/2].Team == s.Players[b/2].Team
}
func findMagic(p Player, id int) (Magic, bool) {
	for _, m := range p.Magic {
		if m.ID == id {
			return m, true
		}
	}
	return Magic{}, false
}
func findInventory(p Player, id int) (int, Item, bool) {
	for i, row := range p.Inventory {
		if row.Item.ID == id && row.Count > 0 {
			return i, row.Item, true
		}
	}
	return 0, Item{}, false
}
func (s *State) validatePlan(seat int, plan Plan) error {
	for pet, c := range []Command{plan.Player, plan.Pet} {
		actor := seat*2 + pet
		alive := s.active(actor)
		if !alive {
			if c.Kind != "wait" {
				return errors.New("退场的宠物只能等待")
			}
			continue
		}
		switch c.Kind {
		case "wait":
			if pet == 0 {
				return errors.New("人物请选择攻击、防御、精灵或物品")
			}
		case "guard":
			if c.Target != actor {
				return errors.New("防御目标必须是自己")
			}
		case "attack":
			if !s.active(c.Target) || s.friendly(actor, c.Target) {
				return errors.New("请选择存活的对手")
			}
		case "skill":
			if pet != 1 || !hasSkill(s.Players[seat].Loadout.Skills, c.ID) {
				return errors.New("宠物尚未学习这项技能")
			}
			if c.ID == 2 {
				if c.Target != actor {
					return errors.New("防御目标必须是自己")
				}
			} else if !s.active(c.Target) || s.friendly(actor, c.Target) {
				return errors.New("技能目标不合法")
			}
		case "magic":
			magic, ok := findMagic(s.Players[seat], c.ID)
			if pet != 0 || !ok || magic.MP > s.Players[seat].Body.MP {
				return errors.New("没有这项装备精灵或气力不足")
			}
			if !s.active(c.Target) || s.friendly(actor, c.Target) == (magic.Kind == "damage") {
				return errors.New("精灵目标不合法")
			}
		case "item":
			_, item, ok := findInventory(s.Players[seat], c.ID)
			if pet != 0 || !ok {
				return errors.New("没有可使用的物品")
			}
			if !s.active(c.Target) || s.friendly(actor, c.Target) == (item.Kind == "dispel") {
				return errors.New("物品目标不合法")
			}
		default:
			return errors.New("无效战斗指令")
		}
	}
	return nil
}
func clonePlayers(players []Player) []Player {
	var out []Player
	_ = jsonUnmarshal(mustJSON(players), &out)
	return out
}

// Keep the JSON clone at the persistence boundary, never retain mutable slices
// from the prior round inside an event or client projection.
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func (s *State) event(kind string, actor, target int, name string, amount, rideAmount int, critical bool) {
	at := 0
	if len(s.Events) > 0 {
		last := s.Events[len(s.Events)-1]
		at = last.At + last.Duration
	}
	duration := 480
	if kind == "guard" || kind == "wait" || kind == "status" {
		duration = 220
	}
	s.EventID++
	s.Events = append(s.Events, Event{ID: s.EventID, Kind: kind, Actor: actor, Target: target, Name: name, Amount: amount, RideAmount: rideAmount, Critical: critical, At: at, Duration: duration, Players: clonePlayers(s.Players)})
}
func (s *State) offense(id int) (int, int, int) {
	u := *s.unit(id)
	p := s.Players[id/2]
	a, d, q := u.Attack, u.Defense, u.Speed
	if id%2 == 0 && p.Riding {
		g, _ := gearByID(s.Pool.Weapons, p.Loadout.Weapon)
		a, d, q = rideStats(u, p.Ride, isThrow(g.WeaponType))
	}
	if u.Status.Rage > 0 {
		a = a * 125 / 100
	}
	if u.TurnDefense > 0 {
		d = d * u.TurnDefense / 100
	}
	if u.Status.Haste > 0 {
		q = q * 130 / 100
	}
	if u.Status.Drunk > 0 {
		q = max(1, q/2)
	}
	return a, d, q
}
func isThrow(kind int) bool { return kind == 4 || kind >= 17 }
func (s *State) targets(actor, target int) []int {
	if s.active(target) && !s.friendly(actor, target) {
		return []int{target}
	}
	for _, p := range s.Players {
		if p.Team != s.Players[actor/2].Team && s.active(p.Body.ID) {
			return []int{p.Body.ID}
		}
	}
	return nil
}
func (s *State) resolve(now time.Time) {
	before := clonePlayers(s.Players)
	s.Events = []Event{}
	s.Playback = &Playback{StartedAt: now.UnixMilli(), Before: before}
	s.Phase = "resolving"
	s.Window++
	type turn struct {
		id, speed, tie int
		command        Command
		loyaltyNote    string
		betrayal       bool
	}
	order := []turn{}
	guardians := map[int]int{}
	counterStyle := map[int]bool{}
	for i := range s.Players {
		s.Players[i].Body.TurnDefense = 100
		s.Players[i].Pet.TurnDefense = 100
	}
	for i, p := range s.Players {
		if p.Body.HP <= 0 || p.Surrendered || s.Plans[i] == nil {
			continue
		}
		for pet, c := range []Command{s.Plans[i].Player, s.Plans[i].Pet} {
			id := i*2 + pet
			if !s.active(id) {
				continue
			}
			u := s.unit(id)
			c, loyaltyNote, betrayal := s.loyalCommand(id, c)
			u.Guard = false
			if (c.Kind == "guard" || c.Kind == "skill" && c.ID == 2) && u.Status.Stone == 0 && u.Status.Sleep == 0 && u.Status.Confusion == 0 {
				u.Guard = true
			}
			if c.Kind == "skill" && c.ID == 20 && u.Status.Stone == 0 && u.Status.Sleep == 0 && u.Status.Confusion == 0 {
				guardians[i*2] = id
			}
			if c.Kind == "skill" && c.ID == 150 && u.Status.Stone == 0 && u.Status.Sleep == 0 && u.Status.Confusion == 0 {
				counterStyle[id] = true
			}
			_, _, speed := s.offense(id)
			work := speed + 20
			speed = work - s.randomN(work*30/100+1)
			if c.Kind == "item" {
				speed += work * 15 / 100
			}
			order = append(order, turn{id: id, speed: speed, tie: s.randomN(1 << 30), command: c, loyaltyNote: loyaltyNote, betrayal: betrayal})
		}
	}
	slices.SortFunc(order, func(a, b turn) int {
		if a.speed != b.speed {
			return b.speed - a.speed
		}
		return a.tie - b.tie
	})
	for _, t := range order {
		if s.outcome() != nil {
			break
		}
		if !s.active(t.id) {
			continue
		}
		u := s.unit(t.id)
		c := t.command
		if u.Status.Stone > 0 || u.Status.Sleep > 0 {
			s.event("wait", t.id, t.id, "无法行动", 0, 0, false)
			continue
		}
		if u.Status.Confusion > 0 {
			targets := []int{}
			for id := 0; id < len(s.Players)*2; id++ {
				if id != t.id && s.active(id) {
					targets = append(targets, id)
				}
			}
			if len(targets) > 0 {
				c = Command{Kind: "attack", Target: targets[s.randomN(len(targets))]}
				s.attack(t.id, c, guardians, counterStyle, true)
				continue
			}
		}
		if t.loyaltyNote != "" && c.Kind != "flee" {
			s.event("status", t.id, t.id, t.loyaltyNote, 0, 0, false)
		}
		switch c.Kind {
		case "flee":
			u.Fled = true
			u.Guard = false
			s.event("flee", t.id, t.id, "不服从：逃跑", 0, 0, false)
		case "guard":
			s.event("guard", t.id, t.id, "防御", 0, 0, false)
		case "wait":
			s.event("wait", t.id, t.id, "等待", 0, 0, false)
		case "attack", "skill":
			if c.Kind == "skill" && c.ID == 2 {
				s.event("guard", t.id, t.id, "防御", 0, 0, false)
			} else {
				s.attack(t.id, c, guardians, counterStyle, t.betrayal)
			}
		case "item":
			s.useItem(t.id, c)
		case "magic":
			s.useMagic(t.id, c)
		}
	}
	s.tickStatuses(before)
	s.PendingResult = s.outcome()
	if s.PendingResult != nil {
		s.Reason = "elimination"
	}
	if s.PendingResult == nil && s.Round >= s.Options.MaxRounds {
		s.PendingResult = s.result(-1)
		s.Reason = "round_limit"
	}
	if len(s.Events) == 0 {
		s.event("wait", -1, -1, "本回合结束", 0, 0, false)
	}
	last := s.Events[len(s.Events)-1]
	s.Playback.Duration = last.At + last.Duration + 250
	s.ResolveAt = now.UnixMilli() + int64(s.Playback.Duration)
	s.Log = append(s.Log, fmt.Sprintf("第 %d 回合：双方指令已结算。", s.Round))
	if len(s.Log) > 100 {
		s.Log = s.Log[len(s.Log)-100:]
	}
}
func (s *State) attack(actor int, c Command, guardians map[int]int, counterStyle map[int]bool, confused bool) {
	skill := allSkills[0]
	if c.Kind == "skill" {
		var ok bool
		skill, ok = skillByID(s.Pool.Skills, c.ID)
		if !ok {
			return
		}
	}
	if skill.ID == 50 {
		s.unit(actor).TurnDefense = 65
	}
	if skill.ID == 51 {
		s.unit(actor).TurnDefense = 45
	}
	targets := s.targets(actor, c.Target)
	if confused && s.active(c.Target) {
		targets = []int{c.Target}
	}
	hits := skill.Hits
	weapon := Equipment{WeaponType: -1}
	if actor%2 == 0 {
		weapon, _ = gearByID(s.Pool.Weapons, s.Players[actor/2].Loadout.Weapon)
		switch weapon.WeaponType {
		case 0:
			hits = 2
		case 4:
			candidates := []int{}
			for id := 0; id < len(s.Players)*2; id++ {
				if !s.friendly(actor, id) && s.active(id) {
					candidates = append(candidates, id)
				}
			}
			hits = 1 + s.randomN(3)
			targets = sample(s, candidates, min(hits, len(candidates)))
			hits = 1
		case 17:
			targets = nil
			for id := 0; id < len(s.Players)*2; id++ {
				if id%2 == c.Target%2 && !s.friendly(actor, id) && s.active(id) {
					targets = append(targets, id)
				}
			}
		}
	}
	for _, target := range targets {
		for hit := 0; hit < max(1, hits); hit++ {
			if !s.active(actor) {
				return
			}
			if !s.active(target) {
				break
			}
			actual := target
			if pet, ok := guardians[target]; ok && s.active(pet) && actor != pet && s.friendly(target, pet) && (actor%2 == 1 || !isThrow(weapon.WeaponType)) && s.unit(pet).Status.Stone+s.unit(pet).Status.Sleep+s.unit(pet).Status.Confusion == 0 {
				actual = pet
				delete(guardians, target)
			}
			a, _, speed := s.offense(actor)
			_, d, defSpeed := s.offense(actual)
			defender := s.unit(actual)
			power := skill.Power
			if skill.ID == 3 {
				if defender.Guard {
					power = 250
				} else {
					power = 65
				}
			}
			if actor%2 == 0 && weapon.WeaponType == 0 {
				power = 55
			}
			if actor%2 == 0 && weapon.WeaponType == 4 {
				power = 60
			}
			if actor%2 == 0 && weapon.WeaponType == 17 {
				power = 55
			}
			if defender.Status.Stone > 0 {
				d *= 2
			}
			evade := max(0, min(35, (defSpeed-speed)/5))
			if counterStyle[actual] {
				evade += 30
			}
			if skill.ID == 40 {
				evade += 30
			}
			if defender.Status.Stone > 0 || defender.Status.Sleep > 0 {
				evade = 0
			}
			if s.randomN(100) < evade {
				s.event("miss", actor, actual, skill.Name, 0, 0, false)
				continue
			}
			critical := s.randomN(100) < max(3, min(18, 5+(speed-defSpeed)/15))
			damage := physicalBase(float64(a*power)/100, float64(d)*.7, s.randomN(10000))
			if critical {
				damage = max(damage, a*power/100)
			}
			damage = int(float64(damage) * elementMultiplier(s.unit(actor).Elements, defender.Elements))
			if defender.Guard && skill.ID != 3 {
				damage = guardDamage(damage, 1+s.randomN(100))
			}
			if defender.Status.Shield > 0 {
				damage = damage * 65 / 100
			}
			lost, rideLost := s.damage(actor, actual, damage)
			if lost+rideLost > 0 && skill.Status != "" {
				s.applyStatus(actor, actual, skill.Status, skill.Chance)
			}
			if lost+rideLost > 0 && skill.ID == 210 && actual%2 == 0 && s.Players[actual/2].Riding && s.randomN(100) < skill.Chance {
				s.Players[actual/2].Riding = false
			}
			s.event("attack", actor, actual, skill.Name, lost, rideLost, critical)
			if skill.ID == 30 || skill.ID == 31 {
				s.unit(actor).TurnDefense = 80
			}
			if s.active(actual) && s.active(actor) && lost > 0 && skill.Status == "" && weapon.WeaponType < 4 {
				chance := 5
				if counterStyle[actual] {
					chance = 50
				}
				if s.unit(actual).Status.Stone == 0 && s.unit(actual).Status.Sleep == 0 && s.randomN(100) < chance {
					ca, _, _ := s.offense(actual)
					_, cd, _ := s.offense(actor)
					counter := physicalBase(float64(ca)*.65, float64(cd)*.7, s.randomN(10000))
					n, r := s.damage(actual, actor, counter)
					s.event("counter", actual, actor, "反击", n, r, false)
				}
			}
		}
	}
}
func (s *State) damage(source, target, amount int) (int, int) {
	u := s.unit(target)
	if u == nil || u.HP <= 0 || amount <= 0 {
		return 0, 0
	}
	p := &s.Players[target/2]
	rideLost := 0
	bodyDamage := amount
	if target%2 == 0 && p.Riding && p.Ride.HP > 0 {
		// The classic partition has a +1 rounding term on both recipients.
		bodyDamage = amount*max(1, p.Ride.Defense)/max(1, u.Defense+p.Ride.Defense) + 1
		rideDamage := max(0, amount-bodyDamage+1)
		rideLost = min(p.Ride.HP, rideDamage)
		p.Ride.HP -= rideLost
		if p.Ride.HP == 0 {
			p.Riding = false
		}
	}
	lost := min(u.HP, bodyDamage)
	u.HP -= lost
	if lost > 0 && u.Status.Sleep > 0 {
		u.Status.Sleep = 0
		u.Status.Immune = max(u.Status.Immune, 1)
	}
	if source >= 0 && source/2 < len(s.Players) {
		s.Players[source/2].Damage += lost + rideLost
		if u.HP == 0 {
			s.Players[source/2].Kills++
		}
	}
	return lost, rideLost
}
func (s *State) applyStatus(source, target int, kind string, chance int) {
	u := s.unit(target)
	if u == nil || u.HP == 0 || u.Status.Immune > 0 {
		return
	}
	if target%2 == 0 {
		for _, g := range s.Pool.gear(*s.Players[target/2].Loadout) {
			if g.Resist == kind {
				chance -= 25
			}
		}
	}
	if s.randomN(100) >= max(0, chance) {
		return
	}
	switch kind {
	case "poison":
		u.Status.Poison = max(u.Status.Poison, 3)
		u.Status.PoisonSource = source
	case "stone":
		u.Status.Stone = 2
	case "sleep":
		u.Status.Sleep = 2
	case "confusion":
		u.Status.Confusion = 2
	case "drunk":
		u.Status.Drunk = 3
	}
}
func clearBad(u *Unit) {
	u.Status.Poison = 0
	u.Status.Stone = 0
	u.Status.Sleep = 0
	u.Status.Confusion = 0
	u.Status.Drunk = 0
	u.Status.Immune = max(u.Status.Immune, 1)
}
func (s *State) effect(actor, target int, kind string, power int) (int, int) {
	u := s.unit(target)
	if u == nil || u.HP <= 0 {
		return 0, 0
	}
	amount := 0
	switch kind {
	case "hp", "heal":
		amount = min(power, u.MaxHP-u.HP)
		u.HP += amount
		s.Players[actor/2].Healing += amount
	case "mp":
		amount = min(power, u.MaxMP-u.MP)
		u.MP += amount
	case "cure":
		clearBad(u)
		amount = min(power, u.MaxHP-u.HP)
		u.HP += amount
		s.Players[actor/2].Healing += amount
	case "shield":
		u.Status.Shield = power
	case "haste":
		u.Status.Haste = power
	case "rage":
		u.Status.Rage = power
	case "immune":
		u.Status.Immune = power
	case "dispel":
		u.Status.Shield = 0
		u.Status.Haste = 0
		u.Status.Rage = 0
		u.Status.Immune = 0
	case "damage":
		return s.damage(actor, target, power)
	}
	return amount, 0
}
func (s *State) useItem(actor int, c Command) {
	p := &s.Players[actor/2]
	index, item, ok := findInventory(*p, c.ID)
	if !ok {
		return
	}
	if !s.active(c.Target) {
		s.event("wait", actor, c.Target, item.Name+"：目标已退场", 0, 0, false)
		return
	}
	p.Inventory[index].Count--
	amount, _ := s.effect(actor, c.Target, item.Kind, item.Power)
	s.event("item", actor, c.Target, item.Name, amount, 0, false)
	s.Events[len(s.Events)-1].Effect = item.Kind
}
func (s *State) useMagic(actor int, c Command) {
	p := &s.Players[actor/2]
	magic, ok := findMagic(*p, c.ID)
	if !ok || p.Body.MP < magic.MP {
		return
	}
	targets := []int{c.Target}
	if magic.Group {
		targets = nil
		for id := 0; id < len(s.Players)*2; id++ {
			if s.active(id) && s.friendly(actor, id) {
				targets = append(targets, id)
			}
		}
	}
	if len(targets) == 0 || !s.active(c.Target) {
		s.event("wait", actor, c.Target, magic.Name+"：目标已退场", 0, 0, false)
		return
	}
	p.Body.MP -= magic.MP
	for _, target := range targets {
		power := magic.Power
		if magic.Kind == "damage" {
			element := [4]int{}
			index := map[string]int{"earth": 0, "water": 1, "fire": 2, "wind": 3}[magic.Status]
			element[index] = 100
			power = int(float64(power) * elementMultiplier(element, s.unit(target).Elements))
		}
		amount, r := s.effect(actor, target, magic.Kind, power)
		s.event("magic", actor, target, magic.Name, amount, r, false)
		s.Events[len(s.Events)-1].Effect = magic.Kind
	}
}
func decrement(v *int) { *v = max(0, *v-1) }
func (s *State) tickStatuses(before []Player) {
	for id := 0; id < len(s.Players)*2; id++ {
		if !s.active(id) {
			continue
		}
		u := s.unit(id)
		old := before[id/2].Body.Status
		if id%2 == 1 {
			old = before[id/2].Pet.Status
		}
		if u.Status.Poison > 0 {
			source := u.Status.PoisonSource
			n, r := s.damage(source, id, max(1, u.MaxHP/16))
			s.event("poison", source, id, "中毒", n, r, false)
		}
		controlExpired := false
		for _, pair := range []struct {
			old int
			now *int
		}{{old.Poison, &u.Status.Poison}, {old.Stone, &u.Status.Stone}, {old.Sleep, &u.Status.Sleep}, {old.Confusion, &u.Status.Confusion}, {old.Drunk, &u.Status.Drunk}, {old.Shield, &u.Status.Shield}, {old.Haste, &u.Status.Haste}, {old.Rage, &u.Status.Rage}, {old.Immune, &u.Status.Immune}} {
			if pair.old > 0 {
				previous := *pair.now
				decrement(pair.now)
				if previous > 0 && *pair.now == 0 && (pair.now == &u.Status.Stone || pair.now == &u.Status.Sleep || pair.now == &u.Status.Confusion) {
					controlExpired = true
				}
			}
		}
		if controlExpired {
			u.Status.Immune = max(u.Status.Immune, 1)
		}
		u.Guard = false
	}
	// Include status expirations in the final frame even when there is no tick.
	s.event("status", -1, -1, "回合结束", 0, 0, false)
}
func (s *State) outcome() *gameapi.Result {
	alive := [2]int{}
	for _, p := range s.Players {
		if p.Body.HP > 0 && !p.Surrendered {
			alive[p.Team]++
		}
	}
	if alive[0] > 0 && alive[1] > 0 {
		return nil
	}
	if alive[0] == 0 && alive[1] == 0 {
		return s.result(-1)
	}
	if alive[0] > 0 {
		return s.result(0)
	}
	return s.result(1)
}
func (s *State) result(team int) *gameapi.Result {
	r := &gameapi.Result{Team: true, Draw: team < 0, Ranks: make([]int, len(s.Players)), Winners: []int{}, Scores: make([]int, len(s.Players))}
	for i, p := range s.Players {
		r.Ranks[i] = 1
		if team >= 0 && p.Team != team {
			r.Ranks[i] = 2
		}
		if team >= 0 && p.Team == team {
			r.Winners = append(r.Winners, i)
		}
		r.Scores[i] = p.Damage
	}
	return r
}
