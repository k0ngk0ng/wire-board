package engine

import (
	"errors"
	"slices"
)

// plantStep contains the autonomous behavior of every standard PC seed. Board
// placement, layers, upgrades, coffee and imitater morphs belong to world/actions.
func (s *State) plantStep() {
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.HP <= 0 {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int64{}
		}
		if p.Asleep {
			p.State = "asleep"
			continue
		}
		if p.State == "asleep" {
			p.State = "ready"
		}
		switch p.Kind {
		case "sunflower", "twinsunflower", "sunshroom", "marigold":
			s.productionPlant(p)
		}
		if s.Phase != "battle" {
			continue
		}
		switch p.Kind {
		case "sunflower", "twinsunflower", "sunshroom", "marigold":
		case "peashooter", "snowpea", "repeater", "gatlingpea", "threepeater", "splitpea", "starfruit", "puffshroom", "seashroom", "scaredyshroom", "fumeshroom", "gloomshroom", "cactus", "cattail", "cabbagepult", "kernelpult", "melonpult", "wintermelon":
			s.shooterPlant(p)
		case "cherrybomb", "potatomine", "jalapeno", "iceshroom", "doomshroom", "blover":
			s.instantPlant(p)
		case "chomper":
			s.chomperPlant(p)
		case "squash":
			s.squashPlant(p)
		case "tanglekelp":
			s.kelpPlant(p)
		case "gravebuster":
			s.gravebusterPlant(p)
		case "spikeweed", "spikerock":
			s.spikePlant(p)
		case "magnetshroom":
			s.magnetPlant(p)
		case "goldmagnet":
			s.goldMagnetPlant(p)
		case "cobcannon":
			if p.ChargeMS <= s.Tick*TickMS {
				p.State = "ready"
			} else {
				p.State = "charging"
			}
		case "plantern":
			p.State = "glowing" // world visibility consumes live lantern positions.
		case "umbrellaleaf":
			if p.EffectUntil <= s.Tick {
				p.State = "ready"
			}
		case "wallnut", "tallnut", "lilypad", "flowerpot", "pumpkin", "torchwood", "hypnoshroom", "garlic": // passive collision, bite, layer or projectile effects.
		case "coffeebean", "imitater": // their finite transform/wake timing is world-owned.
		}
	}
	s.removeDead()
}

func (s *State) productionPlant(p *Plant) {
	if p.Kind == "sunshroom" {
		if p.Flags["grownAtMS"] == 0 {
			p.Flags["grownAtMS"] = p.BornAt*TickMS + 120000
		}
		if s.Tick*TickMS >= p.Flags["grownAtMS"] {
			p.State = "grown"
		} else {
			p.State = "small"
		}
	}
	if s.Tick < p.ReadyAt {
		return
	}
	amount := 25
	kind := "sun"
	if p.Kind == "sunshroom" && p.State != "grown" {
		amount = 15
	}
	if p.Kind == "marigold" {
		kind = "coin"
		amount = 1
		if s.random(100) < 10 {
			amount = 5
			kind = "goldcoin"
		}
	}
	s.addPickup(kind, p.Owner, p.Row, PlantX(*p), amount)
	if p.Kind == "twinsunflower" {
		s.addPickup("sun", p.Owner, p.Row, PlantX(*p)+200, 25)
	}
	p.Flags["attackAt"] = s.Tick
	s.emit("sun", p.Row, PlantX(*p), p.Owner, p.Kind, p.ID, amount)
	base := p.ReadyMS
	if base < s.Tick*TickMS-TickMS {
		base = s.Tick * TickMS
	}
	p.ReadyMS = base + s.productionIntervalMS(p.Kind)
	p.ReadyAt = counterTick(p.ReadyMS)
}

func plantEnemy(z Zombie) bool { return z.HP > 0 && z.Allegiance != "plants" && CanDamageZombie(z) }
func plantGroundTarget(z Zombie) bool {
	return plantEnemy(z) && z.Altitude == 0 && !z.Digging && !z.Diving && z.State != "vaulting" && z.State != "descending" && z.State != "leaving"
}

// Target categories are recovered masks, not a global ground-only filter.
// Bit 8 is retained in the native masks but has no observed eligibility branch.
func plantTargetMask(kind string) int {
	switch kind {
	case "cherrybomb", "jalapeno", "cobcannon", "doomshroom":
		return 0x7f
	case "cabbagepult", "kernelpult", "melonpult", "wintermelon", "squash":
		return 0x0d
	case "potatomine":
		return 0x4d
	case "puffshroom", "seashroom", "fumeshroom", "gloomshroom", "chomper":
		return 9
	case "cattail":
		return 11
	case "tanglekelp":
		return 5
	default:
		return 1
	}
}
func (s *State) plantTargetCategory(kind string, z Zombie, air bool) bool {
	if !plantEnemy(z) {
		return false
	}
	mask := plantTargetMask(kind)
	if kind == "cactus" && air && z.Altitude > 0 {
		mask = 2
	}
	immobilized := z.Flags["frozenUntil"] > s.Tick || z.Flags["butterUntil"] > s.Tick
	if kind == "chomper" && z.Kind == "pogo" && z.Flags["pogo"] > 0 && !immobilized {
		return false
	}
	if z.Digging {
		return mask&64 != 0
	}
	if z.Diving {
		return mask&4 != 0
	}
	if (z.State == "vaulting" || z.State == "climbing") && !immobilized {
		return mask&16 != 0
	}
	if z.Altitude > 0 && !(kind == "chomper" && z.Kind == "pogo" && immobilized) {
		return mask&2 != 0
	}
	return mask&1 != 0
}

// Return original pixel distance in thousandths. Columns use 80px and rows
// use 100px; the logical protocol uses 1000 units for either grid direction.
func plantPixelDistance(ax, ay, bx, by int) int {
	dx, dy := int64(ax-bx)*80, int64(ay-by)*100
	return projectileSqrt(dx*dx + dy*dy)
}
func (s *State) plantTarget(p Plant, row, direction, reach int, air bool) *Zombie {
	best := -1
	score := int(^uint(0) >> 1)
	x := PlantX(p)
	for i, z := range s.Zombies {
		if !s.plantTargetCategory(p.Kind, z, air) {
			continue
		}
		if row >= 0 && z.Row != row && z.Kind != "zomboss" {
			continue
		}

		dx := z.X - x
		if direction > 0 && dx < -200 || direction < 0 && dx > 200 {
			continue
		}
		if reach > 0 && abs(dx) > reach {
			continue
		}
		rank := abs(dx)
		if p.Kind == "cattail" {
			rank = plantPixelDistance(x, p.Row*1000+500, z.X, z.Row*1000+500)
			if z.Altitude > 0 {
				rank -= 10000000
			}
		} else if air && z.Altitude > 0 {
			rank -= 20000
		}
		if rank < score {
			best = i
			score = rank
		}
	}
	if best < 0 {
		return nil
	}
	return &s.Zombies[best]
}
func (s *State) plantHasTarget(p Plant) bool {
	switch p.Kind {
	case "threepeater":
		for row := max(0, p.Row-1); row <= min(s.rowCount()-1, p.Row+1); row++ {
			if s.plantTarget(p, row, 1, 0, false) != nil {
				return true
			}
		}
		return false
	case "splitpea":
		return s.plantTarget(p, p.Row, 1, 0, false) != nil || s.plantTarget(p, p.Row, -1, 0, false) != nil
	case "starfruit":
		return s.plantTarget(p, -1, 0, 0, false) != nil
	case "cattail":
		return s.plantTarget(p, -1, 0, 0, true) != nil
	case "gloomshroom":
		for _, z := range s.Zombies {
			if plantGroundTarget(z) && abs(z.Row-p.Row) <= 1 && abs(z.X-PlantX(p)) <= 1500 {
				return true
			}
		}
		return false
	case "fumeshroom":
		return s.plantTarget(p, p.Row, 1, 4250, false) != nil
	case "puffshroom", "seashroom":
		return s.plantTarget(p, p.Row, 1, 3000, false) != nil
	case "cactus":
		return s.plantTarget(p, p.Row, 1, 0, true) != nil
	default:
		return s.plantTarget(p, p.Row, 1, 0, false) != nil
	}
}
func (s *State) shooterPlant(p *Plant) {
	if p.Kind == "scaredyshroom" {
		for _, z := range s.Zombies {
			if plantEnemy(z) && !z.Digging && abs(z.Row-p.Row) <= 1 && abs(z.X-PlantX(*p)) <= 1500 {
				p.State = "scared"
				p.Flags["firePending"] = 0
				p.Flags["burstRemaining"] = 0
				return
			}
		}
		p.State = "ready"
	}
	if p.Kind == "cactus" {
		p.State = "low"
		for _, z := range s.Zombies {
			if plantEnemy(z) && z.Row == p.Row && z.Altitude > 0 {
				p.State = "high"
				break
			}
		}
	}
	s.shooterBurst(p)
	if p.Flags["firePending"] > 0 && s.Tick*TickMS >= p.Flags["fireAtMS"] {
		p.Flags["firePending"] = 0
		s.firePlantCycle(p)
	}
	if s.Tick < p.AttackAt || !s.plantHasTarget(*p) {
		return
	}
	delay := int64(0)
	switch p.Kind {
	case "peashooter", "snowpea":
		delay = 320
	case "threepeater":
		delay = 340
	case "starfruit":
		delay = 390
	case "puffshroom", "seashroom":
		delay = 280
	case "scaredyshroom":
		delay = 240
	case "fumeshroom":
		delay = 490
	case "gloomshroom":
		delay = 740
	case "gatlingpea":
		delay = 320
	case "cactus":
		delay = 220
		if p.State == "low" {
			delay = 340
		}
	case "cabbagepult":
		delay = 310
	case "kernelpult":
		delay = 290
	case "melonpult", "wintermelon":
		delay = 350
	}
	p.Flags["firePending"] = 1
	p.Flags["fireAtMS"] = s.Tick*TickMS + delay
	rate := 1500
	spec, _ := PlantSpec(p.Kind)
	if spec.LaunchMS > 0 {
		rate = spec.LaunchMS
	}
	interval := rate - s.random(15)*10
	base := p.AttackMS
	if base < s.Tick*TickMS-TickMS {
		base = s.Tick * TickMS
	}
	p.AttackMS = base + int64(interval)
	p.AttackAt = counterTick(p.AttackMS)
	p.Flags["attackAt"] = s.Tick
	if delay == 0 {
		p.Flags["firePending"] = 0
		s.firePlantCycle(p)
	}
}
func (s *State) schedulePlantBurst(p *Plant, count int, delayMS int64) {
	p.Flags["burstRemaining"] = int64(count)
	p.Flags["burstNextMS"] = s.Tick*TickMS + delayMS
	p.Flags["burstDelayMS"] = delayMS
	p.BurstAt = counterTick(p.Flags["burstNextMS"])
}
func (s *State) shooterBurst(p *Plant) {
	if p.Flags["burstRemaining"] <= 0 || s.Tick*TickMS < p.Flags["burstNextMS"] {
		return
	}
	switch p.Kind {
	case "repeater", "gatlingpea":
		s.launchPlantProjectile(*p, p.Row, 1, nil)
	case "splitpea":
		s.launchPlantProjectile(*p, p.Row, -1, nil)
	case "cattail":
		if z := s.plantTarget(*p, -1, 0, 0, true); z != nil {
			s.launchPlantProjectile(*p, z.Row, 1, z)
		}
	case "gloomshroom":
		s.plantAreaDamage(*p, 1500, 1, 20, "gloom")
	}
	p.Flags["burstRemaining"]--
	p.Flags["burstNextMS"] += p.Flags["burstDelayMS"]
	p.BurstAt = counterTick(p.Flags["burstNextMS"])
	if p.Flags["burstRemaining"] == 0 {
		p.BurstAt = 0
	}
}
func (s *State) firePlantCycle(p *Plant) {
	switch p.Kind {
	case "threepeater":
		for row := max(0, p.Row-1); row <= min(s.rowCount()-1, p.Row+1); row++ {
			s.launchPlantProjectile(*p, row, 1, nil)
		}
	case "splitpea":
		if s.plantTarget(*p, p.Row, 1, 0, false) != nil {
			s.launchPlantProjectile(*p, p.Row, 1, nil)
		}
		if s.plantTarget(*p, p.Row, -1, 0, false) != nil {
			s.launchPlantProjectile(*p, p.Row, -1, nil)
			s.schedulePlantBurst(p, 1, 250)
		}
	case "starfruit":
		s.launchStarProjectiles(*p)
	case "fumeshroom":
		s.plantAreaDamage(*p, 4250, 0, 20, "fume")
	case "gloomshroom":
		s.plantAreaDamage(*p, 1500, 1, 20, "gloom")
		s.schedulePlantBurst(p, 3, 280)
	case "repeater":
		s.launchPlantProjectile(*p, p.Row, 1, nil)
		s.schedulePlantBurst(p, 1, 250)
	case "gatlingpea":
		s.launchPlantProjectile(*p, p.Row, 1, nil)
		s.schedulePlantBurst(p, 3, 170)
	case "cattail":
		if z := s.plantTarget(*p, -1, 0, 0, true); z != nil {
			s.launchPlantProjectile(*p, z.Row, 1, z)
			s.schedulePlantBurst(p, 1, 500)
		}
	case "cabbagepult", "kernelpult", "melonpult", "wintermelon":
		if z := s.plantTarget(*p, p.Row, 1, 0, false); z != nil {
			s.launchPlantProjectile(*p, p.Row, 1, z)
		}
	case "cactus":
		if z := s.plantTarget(*p, p.Row, 1, 0, true); z != nil {
			s.launchPlantProjectile(*p, p.Row, 1, z)
		}
	default:
		s.launchPlantProjectile(*p, p.Row, 1, nil)
	}
}
func (s *State) plantAreaDamage(p Plant, reach, rows, damage int, source string) {
	ids := []int{}
	x := PlantX(p)
	for _, z := range s.Zombies {
		if !plantGroundTarget(z) || abs(z.Row-p.Row) > rows {
			continue
		}
		dx := z.X - x
		if source == "fume" && (dx < -200 || dx > reach) || source != "fume" && abs(dx) > reach {
			continue
		}
		ids = append(ids, z.ID)
	}
	for _, id := range ids {
		s.DamageZombie(id, damage, source, p.Owner)
	}
	s.emit("shoot", p.Row, x, p.Owner, source, p.ID, damage)
}
func (s *State) instantPlant(p *Plant) {
	if s.Tick < p.ReadyAt {
		return
	}
	x := PlantX(*p)
	switch p.Kind {
	case "potatomine":
		p.State = "armed"
		for _, z := range s.Zombies {
			if s.plantTargetCategory(p.Kind, z, false) && z.Row == p.Row && abs(z.X-x) <= 550 {
				s.plantExplosion(*p, x, 750, 0, 1800, "potatomine")
				p.HP = 0
				return
			}
		}
	case "cherrybomb":
		s.plantExplosion(*p, x, 1438, 1, 1800, "cherrybomb")
		p.HP = 0
	case "doomshroom":
		s.plantExplosion(*p, x, 3125, 3, 1800, "doomshroom")
		s.addCrater(p.Row, p.Col, 3600)
		for i := range s.Plants {
			q := &s.Plants[i]
			if q.ID != p.ID && q.Row == p.Row && q.Col == p.Col {
				q.HP = 0
			}
		}
		p.HP = 0
	case "jalapeno":
		ids := []int{}
		for _, z := range s.Zombies {
			if plantEnemy(z) && z.Row == p.Row {
				ids = append(ids, z.ID)
			}
		}
		for _, id := range ids {
			s.DamageZombie(id, 1800, "jalapeno", p.Owner)
		}
		terrain := s.Terrain[:0]
		for _, v := range s.Terrain {
			if v.Row != p.Row || v.Kind != "ice" {
				terrain = append(terrain, v)
			}
		}
		s.Terrain = terrain
		ladders := s.Ladders[:0]
		for _, v := range s.Ladders {
			if v.Row != p.Row {
				ladders = append(ladders, v)
			}
		}
		s.Ladders = ladders
		projectiles := s.Projectiles[:0]
		for _, v := range s.Projectiles {
			if v.Row != p.Row || v.Kind != "bossice" {
				projectiles = append(projectiles, v)
			}
		}
		s.Projectiles = projectiles
		s.emit("explosion", p.Row, x, p.Owner, p.Kind, p.ID, 1800)
		p.HP = 0
	case "iceshroom":
		ids := []int{}
		for _, z := range s.Zombies {
			if plantEnemy(z) {
				ids = append(ids, z.ID)
			}
		}
		for _, id := range ids {
			s.DamageZombie(id, 20, "iceshroom", p.Owner)
		}
		projectiles := s.Projectiles[:0]
		for _, v := range s.Projectiles {
			if v.Kind != "bossfire" {
				projectiles = append(projectiles, v)
			}
		}
		s.Projectiles = projectiles
		s.emit("freeze", p.Row, x, p.Owner, p.Kind, p.ID, 20)
		p.HP = 0
	case "blover":
		s.FogClearedUntil = max(s.FogClearedUntil, s.Tick+800)
		for i := range s.Zombies {
			if plantEnemy(s.Zombies[i]) && s.Zombies[i].Kind == "balloon" && s.Zombies[i].Altitude > 0 {
				s.Zombies[i].HP = 0
				s.emit("blow", s.Zombies[i].Row, s.Zombies[i].X, p.Owner, p.Kind, s.Zombies[i].ID, 0)
			}
		}
		s.emit("fog-clear", p.Row, x, p.Owner, p.Kind, p.ID, 0)
		p.HP = 0
	}
}
func (s *State) plantExplosion(p Plant, x, radius, rows, damage int, source string) {
	ids := []int{}
	for _, z := range s.Zombies {
		if s.plantTargetCategory(p.Kind, z, false) && abs(z.Row-p.Row) <= rows && abs(z.X-x) <= radius {
			ids = append(ids, z.ID)
		}
	}
	for _, id := range ids {
		s.DamageZombie(id, damage, source, p.Owner)
	}
	s.emit("explosion", p.Row, x, p.Owner, source, p.ID, damage)
}
func (s *State) chomperPlant(p *Plant) {
	switch p.State {
	case "biting":
		if s.Tick < p.EffectUntil {
			return
		}
		z := s.plantZombieByID(p.TargetID)
		if z == nil || !s.plantTargetCategory(p.Kind, *z, false) || abs(z.X-PlantX(*p)) > 1600 {
			p.State = "recovering"
			p.EffectUntil = s.Tick + 20
			return
		}
		if slices.Contains([]string{"gargantuar", "redeye", "zomboss"}, z.Kind) {
			s.DamageZombie(z.ID, 40, "chomper", p.Owner)
			p.State = "recovering"
			p.EffectUntil = s.Tick + 20
		} else {
			z.HP = 0
			p.State = "digesting"
			p.EffectUntil = s.Tick + 800
			s.emit("swallow", p.Row, z.X, p.Owner, p.Kind, z.ID, 0)
		}
		return
	case "digesting":
		if s.Tick >= p.EffectUntil {
			p.State = "recovering"
			p.EffectUntil = s.Tick + 20
		}
		return
	case "recovering":
		if s.Tick < p.EffectUntil {
			return
		}
		p.State = "ready"
	}
	if z := s.plantTarget(*p, p.Row, 1, 1200, false); z != nil {
		p.TargetID = z.ID
		p.State = "biting"
		p.EffectUntil = s.Tick + 14
		s.emit("chomp", p.Row, PlantX(*p), p.Owner, p.Kind, z.ID, 0)
	}
}
func (s *State) squashPlant(p *Plant) {
	if p.State == "jumping" {
		if s.Tick < p.EffectUntil {
			return
		}
		x := int(p.Flags["targetX"])
		s.plantExplosion(*p, x, 700, 0, 1800, "squash")
		p.HP = 0
		return
	}
	if z := s.plantTarget(*p, p.Row, 0, 1500, false); z != nil {
		p.State = "jumping"
		p.TargetID = z.ID
		p.Flags["targetX"] = int64(z.X)
		p.EffectUntil = s.Tick + 20
		s.emit("squash-jump", p.Row, z.X, p.Owner, p.Kind, z.ID, 0)
	}
}
func (s *State) kelpPlant(p *Plant) {
	if p.State == "grabbing" {
		if s.Tick < p.EffectUntil {
			return
		}
		if z := s.plantZombieByID(p.TargetID); z != nil {
			z.HP = 0
			s.emit("drown", z.Row, z.X, p.Owner, p.Kind, z.ID, 0)
		}
		p.HP = 0
		return
	}
	for i := range s.Zombies {
		z := &s.Zombies[i]
		if plantEnemy(*z) && z.Row == p.Row && z.Altitude == 0 && !z.Digging && abs(z.X-PlantX(*p)) <= 1000 {
			p.State = "grabbing"
			p.TargetID = z.ID
			p.EffectUntil = s.Tick + 20
			if z.Flags == nil {
				z.Flags = map[string]int64{}
			}
			z.Flags["frozenUntil"] = max(z.Flags["frozenUntil"], p.EffectUntil)
			s.emit("kelp-grab", z.Row, z.X, p.Owner, p.Kind, z.ID, 0)
			return
		}
	}
}
func (s *State) gravebusterPlant(p *Plant) {
	if p.State != "eating" {
		for _, g := range s.Graves {
			if g.Row == p.Row && g.Col == p.Col {
				p.TargetID = g.ID
				p.State = "eating"
				p.EffectUntil = s.Tick + 90
				return
			}
		}
		p.HP = 0
		return
	}
	if s.Tick >= p.EffectUntil {
		if s.removeGrave(p.TargetID) {
			s.addPickup("coin", p.Owner, p.Row, PlantX(*p), 1)
		}
		p.HP = 0
		s.emit("grave-eaten", p.Row, PlantX(*p), p.Owner, p.Kind, p.TargetID, 0)
	}
}
func (s *State) spikePlant(p *Plant) {
	now := s.Tick * TickMS
	start := p.Flags["spikeCycleMS"]
	if start == 0 {
		near := false
		for _, z := range s.Zombies {
			if plantGroundTarget(z) && z.Row == p.Row && abs(z.X-PlantX(*p)) <= 650 {
				near = true
				break
			}
		}
		if !near {
			return
		}
		start = now
		p.Flags["spikeCycleMS"] = start
		p.Flags["spikeHits"] = 0
		p.State = "attacking"
	}
	age := now - start
	hit := int64(0)
	if p.Kind == "spikerock" {
		if age >= 300 {
			hit = 1
		}
		if age >= 680 {
			hit = 2
		}
	} else if age >= 250 {
		hit = 1
	}
	for p.Flags["spikeHits"] < hit {
		p.Flags["spikeHits"]++
		ids := []int{}
		for _, z := range s.Zombies {
			if plantGroundTarget(z) && z.Row == p.Row && abs(z.X-PlantX(*p)) <= 650 {
				ids = append(ids, z.ID)
			}
		}
		for _, id := range ids {
			z := s.plantZombieByID(id)
			if z == nil {
				continue
			}
			vehicle := z.Kind == "zomboni" || z.Kind == "catapult"
			damage := 20
			if vehicle {
				damage = 1800
			}
			s.DamageZombie(id, damage, "spike", p.Owner)
			if vehicle {
				if p.Kind == "spikerock" {
					if p.Uses == 0 {
						p.Uses = 9
					}
					p.Uses--
					p.HP -= 50
					if p.Uses == 0 {
						p.HP = 0
					}
				} else {
					p.HP = 0
				}
			}
		}
		s.emit("spike", p.Row, PlantX(*p), p.Owner, p.Kind, p.ID, 20)
	}
	if age >= 1000 {
		p.Flags["spikeCycleMS"] = 0
		p.State = "ready"
	}
}
func (s *State) magnetPlant(p *Plant) {
	if p.EffectUntil > s.Tick {
		p.State = "charging"
		return
	}
	p.State = "ready"
	best := -1
	distance := int(^uint(0) >> 1)
	for i, z := range s.Zombies {
		if !plantEnemy(z) || z.Altitude > 0 || z.Flags["magnetized"] > 0 || abs(z.Row-p.Row) > 2 {
			continue
		}
		if !slices.Contains([]string{"buckethead", "football", "screendoor", "digger", "pogo", "ladder", "jackinthebox"}, z.Kind) {
			continue
		}
		if (z.Kind == "buckethead" || z.Kind == "football" || z.Kind == "screendoor" || z.Kind == "ladder") && z.Armor <= 0 {
			continue
		}
		d := plantPixelDistance(PlantX(*p), p.Row*1000+500, z.X, z.Row*1000+500)
		radius := 270000
		if z.State == "eating" {
			radius = 320000
		}
		if d > radius {
			continue
		}
		d += abs(z.Row-p.Row) * 80000
		if d < distance {
			best = i
			distance = d
		}
	}
	if best >= 0 {
		z := &s.Zombies[best]
		if z.Flags == nil {
			z.Flags = map[string]int64{}
		}
		z.Flags["magnetized"] = 1
		switch z.Kind {
		case "buckethead", "football", "screendoor", "digger", "ladder":
			z.Armor = 0
		}
		p.TargetID = z.ID
		p.Flags["attackAt"] = s.Tick
		p.EffectUntil = s.Tick + 300
		p.State = "charging"
		s.emit("magnet", z.Row, z.X, p.Owner, p.Kind, z.ID, 0)
		return
	}
	bestLadder, score := -1, int(^uint(0)>>1)
	for i, v := range s.Ladders {
		dx, dy := abs(v.Col-p.Col), abs(v.Row-p.Row)
		cheb := max(dx, dy)
		rank := cheb*100 + dy*5
		if cheb <= 2 && rank < score {
			bestLadder = i
			score = rank
		}
	}
	if bestLadder >= 0 {
		v := s.Ladders[bestLadder]
		s.Ladders = append(s.Ladders[:bestLadder], s.Ladders[bestLadder+1:]...)
		p.EffectUntil = s.Tick + 300
		p.State = "charging"
		p.Flags["attackAt"] = s.Tick
		s.emit("magnet", v.Row, v.Col*1000+500, p.Owner, p.Kind, v.PlantID, 0)
	}
}
func (s *State) goldMagnetPlant(p *Plant) {
	if p.EffectUntil > s.Tick {
		return
	}
	ids := []int{}
	for _, v := range s.Pickups {
		if v.Owner == p.Owner && (v.Kind == "coin" || v.Kind == "goldcoin" || v.Kind == "diamond") {
			ids = append(ids, v.ID)
			if len(ids) == 5 {
				break
			}
		}
	}
	for _, id := range ids {
		s.collectPickup(id)
	}
	if len(ids) > 0 {
		p.EffectUntil = s.Tick + 20
		p.Flags["attackAt"] = s.Tick
		s.emit("magnet-coins", p.Row, PlantX(*p), p.Owner, p.Kind, p.ID, len(ids))
	}
}
func (s *State) plantZombieByID(id int) *Zombie {
	for i := range s.Zombies {
		if s.Zombies[i].ID == id && s.Zombies[i].HP > 0 {
			return &s.Zombies[i]
		}
	}
	return nil
}

// OnPlantEaten is called on each actual bite by the shared plant-damage path.
// Hypnosis applies only to the final bite, while garlic reacts to contact.
func (s *State) OnPlantEaten(p Plant, z *Zombie) {
	if z == nil || !plantEnemy(*z) || p.Asleep {
		return
	}
	if z.Flags == nil {
		z.Flags = map[string]int64{}
	}
	if p.Kind == "hypnoshroom" && p.HP <= 0 && !slices.Contains([]string{"gargantuar", "redeye", "zomboss", "zomboni", "catapult", "bobsled"}, z.Kind) {
		z.Allegiance = "plants"
		z.State = "walking"
		z.TargetID = 0
		z.Flags["hypnotized"] = s.Tick
		s.OnZombieHypnotized(z)
		s.emit("hypnotize", z.Row, z.X, p.Owner, p.Kind, z.ID, 0)
	}
	if p.Kind == "garlic" && z.Flags["garlicUntil"] <= s.Tick {
		rows := []int{}
		for _, r := range []int{z.Row - 1, z.Row + 1} {
			if r >= 0 && r < s.rowCount() && s.isWater(r) == s.isWater(z.Row) {
				rows = append(rows, r)
			}
		}
		if len(rows) > 0 {
			z.Flags["garlicFromRow"] = int64(z.Row)
			z.Row = rows[s.random(len(rows))]
			z.Flags["garlicUntil"] = s.Tick + 40
			z.State = "walking"
			z.TargetID = 0
			s.emit("garlic", z.Row, z.X, p.Owner, p.Kind, z.ID, 0)
		}
	}
}
func (s *State) UsePlantAbility(actor, plantID, row, col int) error {
	if actor < 0 || actor >= len(s.Players) || s.Players[actor].Side != "plants" || s.Phase != "battle" || row < 0 || row >= s.rowCount() || col < 0 || col >= Columns {
		return errors.New("当前不能使用植物技能")
	}
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.ID != plantID {
			continue
		}
		if p.Owner != actor || p.Kind != "cobcannon" || p.Asleep || p.HP <= 0 {
			return errors.New("只能操作自己的玉米加农炮")
		}
		if p.ChargeMS > s.Tick*TickMS || p.ReadyAt > s.Tick {
			return errors.New("玉米炮尚未装填")
		}
		if len(s.Projectiles) >= MaxProjectiles {
			return errors.New("场上弹丸已满")
		}
		s.Projectiles = append(s.Projectiles, Projectile{ID: s.id(), Owner: actor, Row: row, X: PlantX(*p), Y: p.Row*1000 + 500, Kind: "cob", Damage: 1800, TargetX: col*1000 + 500, TargetY: row*1000 + 500, LandAt: s.Tick + 80, Height: 2500, Radius: 1438, BornAt: s.Tick})
		p.ChargeMS = s.Tick*TickMS + 30000
		p.ReadyAt = counterTick(p.ChargeMS)
		p.State = "charging"
		p.Uses++
		if p.Flags == nil {
			p.Flags = map[string]int64{}
		}
		p.Flags["attackAt"] = s.Tick
		s.emit("shoot", p.Row, PlantX(*p), actor, "cob", p.ID, 1800)
		return nil
	}
	return errors.New("找不到这株植物")
}
