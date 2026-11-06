package engine

import "strings"

// CanChillZombie / CanFreezeZombie follow the original eligibility gates at
// 0x531990 and 0x531a10. Whole-board cold still respects phase immunity.
func CanChillZombie(z Zombie) bool {
	if z.HP <= 0 || z.Allegiance == "plants" || z.Kind == "zomboni" {
		return false
	}
	if z.Kind == "bobsled" && (z.Flags["sled"] > 0 || z.State == "sledding" || z.State == "sled-crashing") {
		return false
	}
	if z.Digging || z.State == "digging" || z.State == "emerging" || z.State == "summoning" {
		return false
	}
	if z.Kind == "zomboss" {
		return z.State == "boss-head-idle-before" || z.State == "boss-head-idle-after" || z.State == "boss-head-spit"
	}
	return true
}
func CanFreezeZombie(z Zombie) bool {
	if !CanChillZombie(z) {
		return false
	}
	switch z.State {
	case "vaulting", "landing", "sled-crashing", "jackbox-popping", "squash-rising", "squash-falling", "summoning", "emerging", "entering-water":
		return false
	}
	if z.Kind == "balloon" && z.Altitude > 0 || z.Kind == "pogo" && z.Flags["pogo"] > 0 {
		return false
	}
	return z.Kind != "bungee" || z.State == "grabbing"
}

// The original state-machine entry points and measured health/clock values are
// retained in docs/research/pvz/evidence/full. Timed events use the shared 50ms
// versus clock; _ground motion remains the documented cycle-mean adaptation.
// All children retain their purchasing seat and current allegiance.
func (s *State) initializeZombie(z *Zombie) {
	if z.Flags == nil {
		z.Flags = map[string]int64{}
	}
	if z.Allegiance == "" {
		z.Allegiance = "zombies"
	}
	z.Flags["initialized"] = 1
	z.Flags["direction"] = -1
	z.Flags["phaseAt"] = s.Tick
	if z.SpeedMilli <= 0 {
		z.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
	}
	if z.State == "" {
		z.State = "walking"
	}
	switch z.Kind {
	case "flag":
		z.SpeedMilli = 13218
	case "polevaulter":
		z.SpecialSpeedMilli = (660 + s.random(21)) * 29375 / 1000
	case "newspaper":
		z.SpecialSpeedMilli = (890 + s.random(21)) * 29375 / 1000
	case "dancing":
		z.State = "dancing"
		z.SpeedMilli = 13218
		z.Flags["summonAt"] = s.Tick + 60
	case "backupdancer":
		z.SpeedMilli = 13218
	case "duckytube":
		z.Flags["swimmer"] = 1
	case "snorkel":
		z.Flags["swimmer"] = 1
		z.SpeedMilli = (660 + s.random(21)) * 29375 / 1000
	case "dolphin":
		z.Flags["swimmer"] = 1
		velocity := 890 + s.random(21)
		z.SpecialSpeedMilli = velocity * 62500 / 1000
		z.SpeedMilli = velocity * 29375 / 1000
	case "zomboni":
		z.State = "driving"
		z.SpeedMilli = 15625
	case "bobsled":
		z.State = "sledding"
		z.SpeedMilli = 37500
		z.Armor = 300
		z.MaxArmor = 300
	case "jackinthebox":
		velocity := 660 + s.random(21)
		distance := 450 + s.random(300)
		z.Flags["velocityMilli"] = int64(velocity)
		z.SpeedMilli = velocity * 29375 / 1000
		if s.random(20) == 0 {
			distance /= 3
		}
		z.Flags["fuseDistance"] = int64(distance)
		z.Flags["explodeAt"] = s.Tick + counterTick(int64(2*(distance*1000/velocity)*10))
	case "balloon":
		z.Altitude = 1
		z.SpeedMilli = (230 + s.random(141)) * 62500 / 1000
		z.State = "flying"
		z.Flags["balloon"] = 1
		z.Flags["flyingHP"] = 20
	case "digger":
		z.Digging = true
		z.State = "digging"
		z.SpeedMilli = (660 + s.random(21)) * 62500 / 1000
	case "pogo":
		z.Flags["pogo"] = 1
		z.State = "bouncing"
		z.SpeedMilli = 13218
		z.SpecialSpeedMilli = 28125
	case "yeti":
		z.SpeedMilli = 11750
		z.Flags["retreatAt"] = s.Tick + int64(300+s.random(101))
	case "bungee":
		z.State = "descending"
		z.Altitude = 3000 + s.random(151)
		z.AbilityUntil = s.Tick + counterTick(int64(z.Altitude*10/8))
	case "catapult":
		z.SpeedMilli = (230 + s.random(141)) * 62500 / 1000
		z.Flags["ammo"] = 20
		z.Flags["shootAt"] = s.Tick + 1
	case "gargantuar", "redeye":
		z.Flags["hasImp"] = 1
	case "peahead":
		z.Flags["shootAt"] = s.Tick + 30
	case "gatlinghead":
		z.Flags["shootAt"] = s.Tick + 30
	case "jalapenohead":
		z.Flags["explodeAt"] = s.Tick + int64(300+s.random(101))
	case "ladder", "squashhead":
		z.SpeedMilli = (790 + s.random(21)) * 29375 / 1000
	case "zomboss":
		z.X = 9200
		z.Row = min(2, s.rowCount()-1)
		z.State = "boss-enter"
		z.AbilityUntil = s.Tick + 67 // 40 source frames at 12fps.
		z.Flags["summonAt"] = s.Tick + 100
		z.Flags["headAt"] = s.Tick + 1000
		z.Flags["bungeeAt"] = s.Tick + 800
		z.Flags["stompAt"] = s.Tick + 600
		z.Flags["rvAt"] = s.Tick + 1000
	}
}

func (s *State) zombieStep() {
	// Newly summoned units begin next step. Copying the active zombie keeps a
	// child append from invalidating its pointer when the slice grows.
	count := len(s.Zombies)
	for i := 0; i < count; i++ {
		z := s.Zombies[i]
		if z.HP <= 0 {
			continue
		}
		if z.Flags == nil {
			s.initializeZombie(&z)
		}
		if z.Flags["frozenUntil"] > s.Tick || z.Flags["butterUntil"] > s.Tick {
			s.pauseZombieTimers(&z)
			s.commitZombie(i, z)
			continue
		}
		if z.Flags["activeAt"] > s.Tick {
			s.commitZombie(i, z)
			continue
		}
		if z.Kind == "jackinthebox" && z.Flags["magnetized"] > 0 && z.Flags["disarmed"] == 0 {
			z.Flags["disarmed"] = 1
			z.Flags["explodeAt"] = 0
			if z.State == "jackbox-popping" {
				z.State = "walking"
				z.AbilityUntil = 0
			}
			s.emit("jack-disarm", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
		}
		if z.Allegiance == "plants" && z.Flags["hypnosisDetached"] == 0 {
			s.OnZombieHypnotized(&z)
		}
		if z.Kind == "zomboss" {
			s.bossStep(&z)
			s.commitZombie(i, z)
			continue
		}
		if z.Kind == "bungee" {
			s.bungeeStep(&z)
			s.commitZombie(i, z)
			continue
		}
		if s.zombieAirAndUnderground(&z) {
			s.commitZombie(i, z)
			continue
		}
		if s.zombieSpecialPhase(&z) {
			s.commitZombie(i, z)
			continue
		}
		s.zombieRanged(&z)
		if z.HP <= 0 {
			s.commitZombie(i, z)
			continue
		}
		if z.Kind == "dancing" {
			s.dancerStep(&z)
		}
		if z.Kind == "bobsled" {
			s.bobsledStep(&z)
			if z.State == "sled-crashing" {
				s.commitZombie(i, z)
				continue
			}
			if z.Flags["sled"] > 0 {
				if p := s.zombieFindContact(&z, z.X-40, -1); p != nil {
					s.bobsledCrash(&z)
				} else {
					s.zombieMove(&z, -1)
					z.State = "sledding"
					s.zombieMower(&z)
				}
				s.commitZombie(i, z)
				continue
			}
		}
		if z.Kind == "zomboni" {
			s.zomboniTrack(&z)
		}
		if z.Kind == "yeti" && z.Allegiance != "plants" && s.Tick >= z.Flags["retreatAt"] {
			z.State = "escaping"
			z.Flags["direction"] = 1
		}
		if z.Kind == "gargantuar" || z.Kind == "redeye" {
			space := z.X > 1000
			if z.Allegiance == "plants" {
				space = z.X < 9000
			}
			if z.Flags["hasImp"] == 1 && z.HP < z.MaxHP/2 && space {
				z.Flags["hasImp"] = 0
				z.State = "throwing"
				// Original anim_throw: 34 frames at 24fps, contact at .74.
				z.Flags["throwAt"] = s.Tick + 21
				z.AbilityUntil = s.Tick + 29
				s.emit("imp-windup", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
				s.commitZombie(i, z)
				continue
			}
		}
		if z.Allegiance == "plants" {
			s.hypnotizedStep(&z)
			s.commitZombie(i, z)
			continue
		}
		s.walkAndAttack(&z)
		s.commitZombie(i, z)
	}
}

func (s *State) commitZombie(index int, z Zombie) {
	if z.Allegiance == "plants" && s.Zombies[index].Allegiance != "plants" {
		s.OnZombieHypnotized(&z)
	}
	if z.State != s.Zombies[index].State {
		z.Flags["phaseAt"] = s.Tick
	}
	s.Zombies[index] = z
}

// Original StartMindControlled detaches dancer relationships. Existing backups
// keep their allegiance; only future summons inherit the converted leader.
func (s *State) OnZombieHypnotized(z *Zombie) {
	if z.Flags == nil {
		z.Flags = map[string]int64{}
	}
	z.Flags["hypnosisDetached"] = 1
	z.Flags["direction"] = 1
	// The original hypnoshroom chew caller (0x52b940), immediately after
	// StartMindControlled, sets velX=.17 and updates its animation rate. Apply
	// that same value to both cached movement profiles; do not clear existing
	// cold or stun deadlines, since neither source function clears them.
	z.SpeedMilli = 170 * 29375 / 1000
	z.SpecialSpeedMilli = z.SpeedMilli
	if z.Kind == "backupdancer" {
		if parent := s.zombieByID(int(z.Flags["leader"])); parent != nil {
			for slot := 0; slot < 4; slot++ {
				key := "f" + string(rune('0'+slot))
				if parent.Flags[key] == int64(z.ID) {
					parent.Flags[key] = 0
				}
			}
		}
		z.Flags["leader"] = 0
	}
	if z.Kind == "dancing" {
		for slot := 0; slot < 4; slot++ {
			key := "f" + string(rune('0'+slot))
			if child := s.zombieByID(int(z.Flags[key])); child != nil {
				child.Flags["leader"] = 0
			}
			z.Flags[key] = 0
		}
	}
}

func (s *State) pauseZombieTimers(z *Zombie) {
	if z.AbilityUntil > 0 {
		z.AbilityUntil++
	}
	if z.AbilityMS > 0 {
		z.AbilityMS += TickMS
	}
	if z.VaultUntil > 0 {
		z.VaultUntil++
	}
	for key, value := range z.Flags {
		if value > 0 && (strings.HasSuffix(key, "At") || strings.HasSuffix(key, "Until")) && key != "frozenUntil" && key != "butterUntil" {
			z.Flags[key] = value + 1
		}
	}
}
func (s *State) zombieByID(id int) *Zombie {
	for i := range s.Zombies {
		if s.Zombies[i].ID == id {
			return &s.Zombies[i]
		}
	}
	return nil
}
func (s *State) zombiePlant(id int) *Plant {
	for i := range s.Plants {
		if s.Plants[i].ID == id && s.Plants[i].HP > 0 {
			return &s.Plants[i]
		}
	}
	return nil
}
func (s *State) zombieUmbrella(row, col int) bool {
	for _, p := range s.Plants {
		if p.Kind == "umbrellaleaf" && p.HP > 0 && !p.Asleep && abs(p.Row-row) <= 1 && abs(p.Col-col) <= 1 {
			return true
		}
	}
	return false
}
func (s *State) hasIce(row, x int) bool {
	for _, t := range s.Terrain {
		if t.Kind == "ice" && t.Row == row && t.Until > s.Tick && x >= t.FromX && x <= t.ToX {
			return true
		}
	}
	return false
}
func (s *State) hasLadder(row, col int) bool {
	for _, l := range s.Ladders {
		if l.Row == row && l.Col == col && s.zombiePlant(l.PlantID) != nil {
			return true
		}
	}
	return false
}

// Air, water and underground eligibility are real rule states, not just art.
func (s *State) zombieAirAndUnderground(z *Zombie) bool {
	if z.State == "entering-water" {
		// The native dolphin entry clip has a constant _ground track; only
		// the snorkel's explicit .2 velocity advances during this animation.
		if z.Kind == "snorkel" {
			s.zombieMove(z, -1)
		}
		if s.Tick >= z.AbilityUntil {
			z.Flags["waterEntered"] = 1
			z.Flags["waterEntering"] = 0
			if z.Kind == "snorkel" {
				z.Diving = true
				z.State = "diving"
				z.SpeedMilli = 12500
			} else {
				z.State = "riding"
				z.X -= 875
			}
			s.emit("splash", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
		}
		return true
	}
	if (z.Kind == "snorkel" || z.Kind == "dolphin") && s.isWater(z.Row) && z.Flags["waterEntered"] == 0 && z.X <= 8500 {
		z.State = "entering-water"
		z.Flags["waterEntering"] = 1
		z.AbilityUntil = s.Tick + 57
		if z.Kind == "snorkel" {
			z.AbilityUntil = s.Tick + 24
			z.SpeedMilli = 12500
		}
		return true
	}
	if z.State == "landing" || z.State == "emerging" {
		if s.Tick < z.AbilityUntil {
			return true
		}
		z.State = "walking"
		z.Altitude = 0
		z.Digging = false
		if z.Kind == "digger" {
			z.SpeedMilli = 3525 // Native surfaced phase0x25 velX=.12.
			if z.Flags["magnetized"] == 0 {
				z.Flags["direction"] = 1
			} else {
				z.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
			}
		}
	}
	if z.Kind == "pogo" && z.Flags["magnetized"] > 0 {
		z.Flags["pogo"] = 0
		z.Altitude = 0
		z.Vaulted = true
		z.State = "walking"
	}
	if z.State == "vaulting" || z.State == "climbing" {
		if s.Tick < z.VaultUntil {
			return true
		}
		z.X = z.VaultX
		z.State = "walking"
		z.Altitude = 0
		if z.Kind == "dolphin" {
			z.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
		}
		return false
	}
	if z.Kind == "balloon" {
		if z.Altitude > 0 {
			z.State = "flying"
			s.zombieMove(z, -1)
			return true
		}
		if z.Flags["balloon"] == 1 {
			z.Flags["balloon"] = 0
			z.State = "landing"
			z.AbilityUntil = s.Tick + 10
			if s.isWater(z.Row) {
				z.HP = 0
				s.emit("drown", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
			}
			return true
		}
	}
	if z.Kind == "digger" && z.Digging {
		if z.Flags["magnetized"] > 0 || z.X <= 350 {
			z.Digging = false
			z.State = "emerging"
			z.AbilityUntil = s.Tick + int64(26)
			if z.Flags["magnetized"] > 0 {
				z.AbilityUntil += 40
			}
			s.emit("emerge", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
			return true
		}
		s.zombieMove(z, -1)
		return true
	}
	if z.Kind == "snorkel" {
		if z.State == "snorkel-surfacing" {
			if s.Tick >= z.AbilityUntil {
				z.State = "eating"
				z.Diving = false
				z.Flags["snorkelSurfaced"] = 1
			}
			return true
		}
		if z.State == "snorkel-submerging" {
			if s.Tick >= z.AbilityUntil {
				z.State = "diving"
				z.Diving = true
				z.Flags["snorkelSurfaced"] = 0
				z.SpeedMilli = 18750
			}
			return true
		}
		z.Diving = s.isWater(z.Row) && z.Flags["waterEntered"] > 0 && z.Flags["snorkelSurfaced"] == 0
		if z.Diving {
			z.State = "diving"
		}
	}
	if z.Kind == "pogo" && z.Flags["magnetized"] > 0 {
		z.Flags["pogo"] = 0
		z.Altitude = 0
		z.Vaulted = true
		z.State = "walking"
	}
	return false
}

func (s *State) zombieSpecialPhase(z *Zombie) bool {
	switch z.State {
	case "throwing":
		if at := z.Flags["throwAt"]; at > 0 && s.Tick >= at {
			z.Flags["throwAt"] = 0
			destination := max(500, z.X-4500-s.random(1251))
			if s.Map.Roof {
				destination = max(500, destination-1000)
			}
			if z.Allegiance == "plants" {
				destination = min(9500, z.X+4500+s.random(1251))
			}
			id := s.spawnZombie("imp", z.Row, max(500, z.X-1000), z.Owner)
			if child := s.zombieByID(id); child != nil {
				child.Allegiance = z.Allegiance
				child.State = "vaulting"
				child.Altitude = 1
				child.VaultX = destination
				child.VaultUntil = s.Tick + 25
				child.SlowUntil = z.SlowUntil
				s.emit("imp-throw", z.Row, z.X, z.Owner, z.Kind, id, destination)
			}
		}
		if s.Tick >= z.AbilityUntil {
			z.State = "walking"
		}
		return true
	case "smashing":
		if at := z.Flags["smashAt"]; at > 0 && s.Tick >= at {
			z.Flags["smashAt"] = 0
			if p := s.zombiePlant(z.TargetID); p != nil && p.Row == z.Row && abs(zombieContactX(*p, -1)-z.X) <= 1300 {
				if p.Kind == "spikerock" {
					s.damagePlant(p.ID, 50, nil)
					s.damage(z, 20, false)
				} else {
					col := p.Col
					if p.Kind == "cobcannon" {
						col++
					}
					s.zombieCrushCell(p.Row, col)
				}
				s.emit("smash", z.Row, z.X, z.Owner, z.Kind, z.TargetID, 100000)
			}
		}
		if s.Tick >= z.AbilityUntil {
			z.State = "walking"
		}
		return true
	case "squash-rising":
		if s.Tick >= z.AbilityUntil {
			if p := s.zombiePlant(z.TargetID); p != nil {
				col := p.Col
				if p.Kind == "cobcannon" {
					col++
				}
				s.zombieCrushCell(p.Row, col)
			}
			z.HP = 0
			s.emit("squash", z.Row, z.X, z.Owner, z.Kind, z.TargetID, 100000)
		}
		return true
	case "ladder-placing":
		if s.Tick >= z.AbilityUntil {
			if p := s.zombiePlant(z.TargetID); p != nil && z.Flags["magnetized"] == 0 {
				if !s.hasLadder(p.Row, p.Col) {
					s.Ladders = append(s.Ladders, Ladder{Row: p.Row, Col: p.Col, PlantID: p.ID, Owner: z.Owner})
				}
				z.Armor = 0
				z.Flags["ladderPlaced"] = 1
				z.Vaulted = true
				z.State = "climbing"
				z.VaultX = PlantX(*p) - 650
				z.VaultUntil = s.Tick + 18
				s.emit("ladder", p.Row, PlantX(*p), z.Owner, z.Kind, p.ID, 0)
			} else {
				z.State = "walking"
			}
		}
		return true
	case "sled-crashing":
		if s.Tick >= z.AbilityUntil {
			z.State = "walking"
			z.Flags["sled"] = 0
			z.Armor = 0
			z.MaxArmor = 0
			z.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
		}
		return true
	case "jackbox-popping":
		if s.Tick >= z.AbilityUntil {
			s.jackboxExplode(z)
		}
		return true
	}
	if z.Kind == "jackinthebox" && z.Flags["disarmed"] == 0 && s.Tick >= z.Flags["explodeAt"] && z.Flags["exploded"] == 0 {
		z.State = "jackbox-popping"
		z.AbilityUntil = s.Tick + 22 // Original opening phase is 110 updates.
		s.emit("jack-windup", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
		return true
	}
	if z.Kind == "jalapenohead" && s.Tick >= z.Flags["explodeAt"] && z.Flags["exploded"] == 0 {
		z.Flags["exploded"] = 1
		z.HP = 0
		if z.Allegiance == "plants" {
			for i := range s.Zombies {
				other := &s.Zombies[i]
				if other.Row == z.Row && other.Allegiance != "plants" {
					s.damage(other, 1800, false)
				}
			}
		} else {
			for _, p := range s.Plants {
				if p.Row == z.Row && p.HP > 0 {
					s.damagePlant(p.ID, 100000, nil)
				}
			}
		}
		s.emit("jalapeno", z.Row, z.X, z.Owner, z.Kind, z.ID, 100000)
		return true
	}
	return false
}

func (s *State) jackboxExplode(z *Zombie) {
	z.Flags["exploded"] = 1
	z.HP = 0
	z.Flags["noLoot"] = 1
	if z.Allegiance == "plants" {
		for i := range s.Zombies {
			other := &s.Zombies[i]
			if other.ID != z.ID && other.Allegiance != "plants" && abs(other.Row-z.Row) <= 1 && abs(other.X-z.X) <= 1450 {
				s.damage(other, 1800, false)
			}
		}
	} else {
		for _, p := range s.Plants {
			if p.HP > 0 && abs(p.Row-z.Row) <= 1 && abs(PlantX(p)-z.X) <= 1450 {
				s.damagePlant(p.ID, 100000, nil)
			}
		}
	}
	s.emit("explosion", z.Row, z.X, z.Owner, z.Kind, z.ID, 100000)
}

func (s *State) zombieRanged(z *Zombie) {
	if z.Kind == "catapult" {
		if z.Flags["flatTires"] > 0 {
			return
		}
		target := s.catapultPlant(z.Row)
		if z.X <= 7600 && z.Flags["ammo"] > 0 && target != nil {
			z.State = "catapult-shooting"
			if s.Tick >= z.Flags["shootAt"] {
				if s.zombieProjectile(z, "basketball", 75, target) {
					z.Flags["ammo"]--
				}
				z.Flags["shootAt"] = s.Tick + 60
			}
		} else if z.State == "catapult-shooting" {
			z.State = "walking"
		}
		return
	}
	if z.Kind != "peahead" && z.Kind != "gatlinghead" {
		return
	}
	if z.Kind == "peahead" {
		if s.Tick >= z.Flags["shootAt"] {
			s.zombieProjectile(z, "zombiepea", 20, nil)
			z.Flags["shootAt"] = s.Tick + 30
		}
		return
	}
	if z.Flags["burstLeft"] == 0 && s.Tick >= z.Flags["shootAt"] {
		z.Flags["burstLeft"] = 4
		z.Flags["nextShotMS"] = s.Tick * TickMS
		z.Flags["shootAt"] = s.Tick + 30
	}
	if z.Flags["burstLeft"] > 0 && s.Tick*TickMS >= z.Flags["nextShotMS"] {
		s.zombieProjectile(z, "zombiepea", 20, nil)
		z.Flags["burstLeft"]--
		z.Flags["nextShotMS"] += 170
	}
}
func (s *State) zombieProjectile(z *Zombie, kind string, damage int, target *Plant) bool {
	if len(s.Projectiles) >= MaxProjectiles {
		return false
	}
	direction := -1
	payload := "hostile"
	if z.Allegiance == "plants" {
		direction = 1
		payload = "friendly"
	}
	p := Projectile{ID: s.id(), Owner: z.Owner, Kind: kind, Row: z.Row, X: z.X + direction*200, Y: z.Row*1000 + 500, VX: direction * 250, Damage: damage, Payload: payload, BornAt: s.Tick}
	if target != nil {
		p.TargetID = target.ID
		p.TargetX = PlantX(*target)
		p.TargetY = target.Row*1000 + 500
		p.LandAt = s.Tick + 24
		p.Height = 900
		p.VX = 0
	}
	if kind == "bossfire" || kind == "bossice" {
		p.X = 6125 // Original ball collision center: 455px + 75px.
		p.VX = -23
		if kind == "bossice" {
			p.VX = -22
		}
		p.Radius = 500
		p.Pierce = true
	}
	s.Projectiles = append(s.Projectiles, p)
	if z.Kind == "peahead" || z.Kind == "gatlinghead" || z.Kind == "catapult" {
		z.Flags["phaseAt"] = s.Tick
	}
	s.emit("shoot", z.Row, z.X, z.Owner, kind, z.ID, damage)
	return true
}
func (s *State) catapultPlant(row int) *Plant {
	var target *Plant
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.Row != row || p.HP <= 0 || p.Layer == "support" || p.Kind == "spikeweed" || p.Kind == "spikerock" {
			continue
		}
		if target == nil || p.Col < target.Col || p.Col == target.Col && p.Layer == "shell" {
			target = p
		}
	}
	return target
}

func (s *State) dancerStep(z *Zombie) {
	if s.Tick < z.Flags["summonAt"] {
		return
	}
	z.Flags["summonAt"] = s.Tick + 200
	positions := [4][2]int{{z.Row - 1, z.X}, {z.Row + 1, z.X}, {z.Row, z.X - 1100}, {z.Row, z.X + 1100}}
	for slot, pos := range positions {
		key := "f" + string(rune('0'+slot))
		if old := s.zombieByID(int(z.Flags[key])); old != nil && old.HP > 0 {
			continue
		}
		if pos[0] < 0 || pos[0] >= s.rowCount() || s.isWater(pos[0]) {
			continue
		}
		id := s.spawnZombie("backupdancer", pos[0], max(300, min(9800, pos[1])), z.Owner)
		if child := s.zombieByID(id); child != nil {
			child.Allegiance = z.Allegiance
			if z.Allegiance == "plants" {
				child.Flags["hypnosisDetached"] = 1
				child.Flags["direction"] = 1
			}
			child.Flags["leader"] = int64(z.ID)
			child.Flags["slot"] = int64(slot)
			child.Flags["activeAt"] = s.Tick + 30
			child.State = "summoning"
			z.Flags[key] = int64(id)
		}
	}
	z.State = "dancing"
	s.emit("summon", z.Row, z.X, z.Owner, z.Kind, z.ID, 4)
}
func (s *State) zomboniTrack(z *Zombie) {
	z.State = "driving"
	if z.Flags["flatTires"] > 0 {
		z.SpeedMilli = max(0, z.SpeedMilli-150)
		if z.SpeedMilli == 0 {
			z.HP = 0
		}
		return
	}
	z.SpeedMilli = 3125 + max(0, min(5000, z.X-3500))*12500/5000
	left := max(0, z.X-200)
	if s.Map.Roof {
		left = max(5000, left)
	}
	found := false
	for i := range s.Terrain {
		track := &s.Terrain[i]
		if track.Kind == "ice" && track.Row == z.Row && track.Owner == z.Owner {
			track.FromX = min(track.FromX, left)
			track.ToX = 10000
			track.Until = s.Tick + 600
			found = true
			break
		}
	}
	if !found {
		s.Terrain = append(s.Terrain, Terrain{ID: s.id(), Row: z.Row, FromX: left, ToX: 10000, Owner: z.Owner, Kind: "ice", Until: s.Tick + 600})
	}
}
func (s *State) bobsledStep(z *Zombie) {
	if z.Flags["sledStarted"] == 0 && z.Flags["leader"] == 0 {
		z.Flags["sledStarted"] = 1
		z.Flags["sled"] = 1
		z.Flags["slot"] = 0
		z.Flags["sledLeader"] = int64(z.ID)
		for n := 1; n <= 3; n++ {
			id := s.spawnZombie("bobsled", z.Row, min(9900, z.X+n*200), z.Owner)
			if child := s.zombieByID(id); child != nil {
				child.Allegiance = z.Allegiance
				child.Armor = 0
				child.MaxArmor = 0
				child.Flags["leader"] = int64(z.ID)
				child.Flags["sledLeader"] = int64(z.ID)
				child.Flags["slot"] = int64(n)
				child.Flags["sled"] = 1
				child.Flags["sledStarted"] = 1
			}
		}
	}
	if z.Flags["sled"] == 0 {
		return
	}
	if parentID := z.Flags["leader"]; parentID > 0 {
		if parent := s.zombieByID(int(parentID)); parent == nil || parent.HP <= 0 || parent.Flags["sled"] == 0 {
			s.bobsledCrash(z)
			return
		}
	}
	if !s.hasIce(z.Row, z.X) || z.Flags["slot"] == 0 && z.Armor <= 0 {
		s.bobsledCrash(z)
		return
	}
	z.State = "sledding"
	z.SpeedMilli = 37500
}

func (s *State) bobsledCrash(z *Zombie) {
	z.State = "sled-crashing"
	z.AbilityUntil = s.Tick + 20
	if z.Flags["slot"] == 0 {
		for i := range s.Zombies {
			member := &s.Zombies[i]
			if member.Flags["leader"] == int64(z.ID) {
				member.State = "sled-crashing"
				member.AbilityUntil = z.AbilityUntil
				member.Flags["leader"] = 0
			}
		}
	}
	z.Flags["leader"] = 0
	s.emit("bobsled-crash", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
}

func (s *State) bungeeStep(z *Zombie) {
	switch z.State {
	case "descending":
		z.Altitude = max(0, z.Altitude-40)
		if s.Tick >= z.AbilityUntil {
			z.Altitude = 0
			z.State = "grabbing"
			z.AbilityUntil = s.Tick + 60
			s.emit("bungee-land", z.Row, z.X, z.Owner, z.Kind, z.TargetID, 0)
		}
	case "grabbing":
		col := max(0, min(Columns-1, z.X/1000))
		if s.zombieUmbrella(z.Row, col) {
			z.State = "rising"
			z.AbilityUntil = s.Tick + 30
			s.emit("umbrella", z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
			return
		}
		if s.Tick >= z.AbilityUntil {
			z.State = "bungee-grab"
			z.AbilityUntil = s.Tick + 13 // 15 source frames at 24fps.
			s.emit("bungee-grab", z.Row, z.X, z.Owner, z.Kind, z.TargetID, 0)
		}
	case "bungee-grab":
		col := max(0, min(Columns-1, z.X/1000))
		if s.zombieUmbrella(z.Row, col) {
			z.State = "rising"
			z.AbilityUntil = s.Tick + 15
			return
		}
		if s.Tick >= z.AbilityUntil {
			p := s.zombiePlant(z.TargetID)
			if p == nil {
				p = s.zombiePlant(s.topPlantID(z.Row, col))
			}
			if p != nil && p.Kind != "cobcannon" && p.Kind != "gravebuster" {
				id := p.ID
				if spec, ok := PlantSpec(p.Kind); ok {
					z.Flags["stolenPlant"] = int64(spec.OriginalID + 1)
					z.Flags["stolenOwner"] = int64(p.Owner + 1)
				}
				s.damagePlant(id, 100000, nil)
				s.emit("steal", z.Row, z.X, z.Owner, z.Kind, id, 0)
			}
			z.State = "rising"
			z.AbilityUntil = s.Tick + 15
		}
	case "rising":
		z.Altitude += 40
		if s.Tick >= z.AbilityUntil {
			z.HP = 0
			z.Flags["noLoot"] = 1
		}
	default:
		z.State = "descending"
		z.Altitude = 3000
		z.AbilityUntil = s.Tick + 75
	}
}

func (s *State) zombieMove(z *Zombie, direction int) {
	moveMilli := z.SpeedMilli
	if z.Allegiance != "plants" && (z.Kind == "polevaulter" && !z.Vaulted || z.Kind == "newspaper" && z.Armor == 0 || z.Kind == "dolphin" && !z.Vaulted && z.Flags["waterEntered"] > 0 || z.Kind == "pogo" && z.Flags["pogo"] == 1) {
		if z.SpecialSpeedMilli > 0 {
			moveMilli = z.SpecialSpeedMilli
		}
	}
	if z.State == "escaping" && z.Allegiance != "plants" {
		moveMilli = 23500
	}
	if z.SlowUntil > s.Tick {
		direct := z.Kind == "balloon" && z.Altitude > 0 || z.Kind == "pogo" && z.Flags["pogo"] > 0 || z.Kind == "dolphin" && !z.Vaulted && z.Flags["waterEntered"] > 0 || z.Kind == "snorkel" && z.Diving || z.Kind == "catapult"
		if direct {
			moveMilli = moveMilli * 2 / 5
		} else if z.Kind != "zomboni" && !z.Digging && !(z.Kind == "bobsled" && z.Flags["sled"] > 0) && !(z.Kind == "snorkel" && z.State == "entering-water") {
			moveMilli /= 2
		}
	}
	z.MoveRemainder += max(0, moveMilli)
	distance := z.MoveRemainder / 1000
	z.MoveRemainder %= 1000
	z.X += direction * distance
	if direction > 0 && z.X > 10200 {
		z.HP = 0
		z.Flags["noLoot"] = 1
	}
}
func (s *State) walkAndAttack(z *Zombie) {
	direction := int(z.Flags["direction"])
	if direction == 0 {
		direction = -1
	}
	if z.State == "escaping" {
		s.zombieMove(z, 1)
		return
	}
	if z.Kind == "catapult" && z.State == "catapult-shooting" {
		return
	}
	distance := max(1, z.SpeedMilli/1000)
	if z.SlowUntil > s.Tick {
		distance = max(1, distance/2)
	}
	next := z.X + direction*distance
	target := s.zombieFindContact(z, next, direction)
	if target == nil {
		if z.Kind == "snorkel" && z.Flags["snorkelSurfaced"] == 1 {
			z.State = "snorkel-submerging"
			z.AbilityUntil = s.Tick + 10
			return
		}
		s.zombieMove(z, direction)
		z.State = "walking"
		if z.Kind == "newspaper" && z.Armor == 0 {
			z.State = "angry"
		}
		if z.Kind == "snorkel" && z.Diving {
			z.State = "diving"
		}
		s.zombieMower(z)
		return
	}
	id, row, col, kind := target.ID, target.Row, target.Col, target.Kind
	if kind == "cobcannon" && direction < 0 {
		col++
	}
	if z.Kind == "snorkel" && z.Diving && z.Flags["snorkelSurfaced"] == 0 {
		z.State = "snorkel-surfacing"
		z.AbilityUntil = s.Tick + 10
		z.TargetID = id
		return
	}
	if s.hasLadder(row, col) && direction < 0 {
		z.State = "climbing"
		z.VaultX = PlantX(*target) - 650
		z.VaultUntil = s.Tick + 20
		z.Altitude = 1
		s.emit("climb", row, z.X, z.Owner, z.Kind, id, 0)
		return
	}
	if z.Kind == "polevaulter" && !z.Vaulted || z.Kind == "dolphin" && !z.Vaulted || z.Kind == "pogo" && z.Flags["pogo"] == 1 {
		if z.Kind != "pogo" {
			z.Vaulted = true
		}
		if kind != "tallnut" {
			z.State = "vaulting"
			z.Altitude = 1
			z.VaultUntil = s.Tick + 36 // 43-frame pole jump at 24fps.
			if z.Kind == "dolphin" {
				z.VaultUntil = s.Tick + 46
			} // 23 frames at 10fps.
			if z.Kind == "pogo" {
				z.VaultUntil = s.Tick + 16
			}
			z.VaultX = PlantX(*target) + direction*650
			s.emit("vault", row, z.X, z.Owner, z.Kind, id, 0)
			return
		}
		if z.Kind == "pogo" {
			z.Flags["pogo"] = 0
			z.Vaulted = true
			z.Altitude = 0
			s.emit("pogo-break", row, z.X, z.Owner, z.Kind, id, 0)
		}
	}
	if z.Kind == "zomboni" || z.Kind == "catapult" {
		if kind == "spikeweed" || kind == "spikerock" {
			z.Flags["flatTires"] = 1
			z.HP = 0
			if kind == "spikeweed" {
				s.damagePlant(id, 100000, nil)
			} else if rock := s.zombiePlant(id); rock != nil {
				// This contact can precede the plant's timed spike pulse. The
				// vehicle is already dead, so that later path cannot consume its
				// durability; settle this one collision here exactly once.
				if rock.Uses == 0 {
					rock.Uses = 9
				}
				rock.Uses--
				rock.HP -= 50
				if rock.Uses == 0 {
					rock.HP = 0
				}
			}
		} else {
			s.zombieCrushCell(row, col)
		}
		s.zombieMove(z, direction)
		s.emit("crush", row, z.X, z.Owner, z.Kind, id, 100000)
		return
	}
	if z.Kind == "gargantuar" || z.Kind == "redeye" {
		z.State = "smashing"
		z.TargetID = id
		// Original anim_smash: 33 frames at 16fps, contact at .64.
		z.Flags["smashAt"] = s.Tick + 27
		z.AbilityUntil = s.Tick + 42
		return
	}
	if z.Kind == "squashhead" && z.Flags["squashed"] == 0 {
		z.Flags["squashed"] = 1
		z.State = "squash-rising"
		z.TargetID = id
		z.AbilityUntil = s.Tick + 22
		return
	}
	if z.Kind == "ladder" && z.Armor > 0 && z.Flags["ladderPlaced"] == 0 && z.Flags["magnetized"] == 0 && (kind == "wallnut" || kind == "tallnut" || kind == "pumpkin") {
		z.State = "ladder-placing"
		z.TargetID = id
		z.AbilityUntil = s.Tick + 10 // 12 source frames at 24fps.
		return
	}
	if direction < 0 {
		z.X = max(z.X, zombieContactX(*target, direction)+400)
	} else {
		z.X = min(z.X, zombieContactX(*target, direction)-400)
	}
	z.State = "eating"
	z.Diving = false
	interval := 40
	if z.SlowUntil > s.Tick {
		interval = 80
	}
	z.BiteCarryMS += TickMS
	amount := (z.BiteCarryMS / interval) * 4
	z.BiteCarryMS %= interval
	z.BiteAt = s.Tick + counterTick(int64(interval-z.BiteCarryMS))
	if amount > 0 {
		s.damagePlant(id, amount, z)
		s.emit("bite", row, z.X, z.Owner, z.Kind, id, amount)
	}
}
func zombieContactX(p Plant, direction int) int {
	x := PlantX(p)
	if direction < 0 && PlantOccupies(p, p.Row, p.Col+1) {
		x += 1000
	}
	return x
}
func (s *State) zombieFindContact(z *Zombie, next, direction int) *Plant {
	var best *Plant
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.HP <= 0 || p.Row != z.Row {
			continue
		}
		if (p.Kind == "spikeweed" || p.Kind == "spikerock") && z.Kind != "gargantuar" && z.Kind != "redeye" && z.Kind != "zomboni" && z.Kind != "catapult" {
			continue
		}
		x := zombieContactX(*p, direction)
		if direction < 0 {
			if z.X < x-250 || next > x+450 {
				continue
			}
			if best != nil && x < zombieContactX(*best, direction) {
				continue
			}
		} else {
			if z.X > x+250 || next < x-450 {
				continue
			}
			if best != nil && x > zombieContactX(*best, direction) {
				continue
			}
		}
		if best != nil && x == zombieContactX(*best, direction) {
			priority := func(p *Plant) int {
				if p.Layer == "shell" {
					return 3
				}
				if p.Layer == "support" {
					return 1
				}
				return 2
			}
			if priority(p) <= priority(best) {
				continue
			}
		}
		best = p
	}
	return best
}
func (s *State) zombieMower(z *Zombie) {
	if z.Allegiance != "plants" && z.Altitude == 0 && !z.Digging && z.Kind != "bungee" && z.Kind != "zomboss" && z.X <= 150 && z.Row >= 0 && z.Row < len(s.Mowers) && s.Mowers[z.Row].State == "ready" {
		s.Mowers[z.Row].State = "moving"
		s.Mowers[z.Row].X = -150
		s.emit("mower", z.Row, -150, -1, "", 0, 0)
	}
}
func (s *State) zombieCrushCell(row, col int) {
	for tries := 0; tries < 4; tries++ {
		p := s.zombiePlant(s.topPlantID(row, col))
		if p == nil {
			return
		}
		s.damagePlant(p.ID, 100000, nil)
	}
}
func (s *State) hypnotizedStep(z *Zombie) {
	var target *Zombie
	for i := range s.Zombies {
		other := &s.Zombies[i]
		if other.ID == z.ID || other.HP <= 0 || other.Row != z.Row || other.Allegiance == "plants" || other.Digging || other.Altitude > 0 {
			continue
		}
		if other.X < z.X-250 || other.X > z.X+650 {
			continue
		}
		if target == nil || other.X < target.X {
			target = other
		}
	}
	if target == nil {
		z.State = "walking"
		s.zombieMove(z, 1)
		return
	}
	z.State = "eating"
	z.Diving = false
	interval := 40
	if z.SlowUntil > s.Tick {
		interval = 80
	}
	z.BiteCarryMS += TickMS
	damage := z.BiteCarryMS / interval * 4
	z.BiteCarryMS %= interval
	if z.Kind == "gargantuar" || z.Kind == "redeye" {
		damage = 0
		if s.Tick >= z.Flags["smashAt"] {
			damage = 500
			if z.Kind == "redeye" {
				damage = 1000
			}
			z.Flags["smashAt"] = s.Tick + 32
		}
	}
	if damage > 0 {
		s.damage(target, damage, false)
		s.emit("bite", z.Row, target.X, z.Owner, z.Kind, target.ID, damage)
	}
}

// Zomboss keeps independent ability deadlines and an exposed-head phase. Each
// animation contact is a scheduled authoritative event, never an instant UI hit.
func (s *State) bossStep(z *Zombie) {
	if z.State == "boss-enter" {
		if s.Tick >= z.AbilityUntil {
			z.State = "boss-idle"
		}
		return
	}
	if strings.HasPrefix(z.State, "boss-head-") {
		s.bossHeadStep(z)
		return
	}
	if z.State != "boss-idle" {
		if at := z.Flags["contactAt"]; at > 0 && s.Tick >= at {
			z.Flags["contactAt"] = 0
			s.bossContact(z)
		}
		if s.Tick >= z.AbilityUntil {
			z.Flags["headOpen"] = 0
			z.State = "boss-idle"
			z.Flags["idleAt"] = s.Tick + int64(20+s.random(21))
		}
		return
	}
	if s.Tick < z.Flags["idleAt"] {
		return
	}
	phase := 0
	if z.HP*3 < z.MaxHP*2 {
		phase = 1
	}
	if z.HP*3 < z.MaxHP {
		phase = 2
	}
	if int(z.Flags["damagePhase"]) < phase {
		z.Flags["damagePhase"] = int64(phase)
		if phase == 1 {
			z.Flags["bungeeAt"] = s.Tick
		} else {
			z.Flags["rvAt"] = s.Tick
		}
	}
	state, key, duration, contact := "", "", int64(0), int64(0)
	switch {
	case s.Tick >= z.Flags["headAt"]:
		state, key, duration, contact = "boss-head-enter", "headAt", 95, 0
		z.Flags[key] = s.Tick + int64(800+s.random(201))
		z.Flags["headOpen"] = 0
		z.Flags["headStage"] = 1
		s.emit("boss-head", z.Row, z.X, z.Owner, z.Kind, z.ID, int(z.Flags["element"]))
	case s.Tick >= z.Flags["stompAt"]:
		rows := []int{}
		for row := 0; row < s.rowCount()-1; row++ {
			for _, p := range s.Plants {
				if p.HP > 0 && p.Row >= row && p.Row <= row+1 && p.Col >= 5 {
					rows = append(rows, row)
					break
				}
			}
		}
		z.Flags["stompAt"] = s.Tick + int64(1100+s.random(201))
		if len(rows) == 0 {
			return
		}
		z.Row = rows[s.random(len(rows))]
		frames, trigger := 31, 50
		if z.Row >= 2 {
			frames, trigger = 29, 55
		}
		state, key, duration, contact = "boss-stomp", "stompAt", counterTick(int64(frames*1000/12)), counterTick(int64(frames*1000*trigger/(12*100)))
		z.Flags["targetCol"] = 5
	case s.Tick >= z.Flags["rvAt"]:
		state, key, duration, contact = "boss-rv", "rvAt", 48, 31
		z.Flags[key] = s.Tick + 1000
		z.Row = s.random(max(1, s.rowCount()-1))
		z.Flags["targetCol"] = int64(s.random(3))
	case s.Tick >= z.Flags["bungeeAt"]:
		state, key, duration, contact = "boss-bungee", "bungeeAt", 210, 17
		z.Flags[key] = s.Tick + int64(800+s.random(201))
		z.Flags["targetCol"] = int64(s.random(3))
	case s.Tick >= z.Flags["summonAt"]:
		state, key, duration, contact = "boss-summon", "summonAt", 42, 26
		period := 90
		if phase == 1 {
			period = 70
		}
		if phase == 2 {
			period = 30
		}
		z.Flags[key] = s.Tick + int64(period+s.random(21))
		z.Row = s.random(s.rowCount())
		z.Flags["targetRow"] = int64(z.Row)
	}
	if state != "" {
		z.State = state
		z.AbilityUntil = s.Tick + duration
		z.Flags["contactAt"] = 0
		if contact > 0 {
			z.Flags["contactAt"] = s.Tick + contact
		}
		s.emit(state, z.Row, z.X, z.Owner, z.Kind, z.ID, 0)
	}
}

func (s *State) bossHeadStep(z *Zombie) {
	if z.State == "boss-head-spit" {
		if at := z.Flags["contactAt"]; at > 0 && s.Tick >= at {
			z.Flags["contactAt"] = 0
			s.bossContact(z)
		}
	}
	if s.Tick < z.AbilityUntil {
		return
	}
	switch z.State {
	case "boss-head-enter":
		z.State = "boss-head-idle-before"
		z.Flags["headOpen"] = 1
		z.Flags["headStage"] = 2
		z.AbilityUntil = s.Tick + 100
	case "boss-head-idle-before":
		z.State = "boss-head-spit"
		z.Flags["headStage"] = 3
		z.Row = s.random(min(5, s.rowCount()))
		z.Flags["element"] = int64(s.random(2))
		frames := 40
		if z.Row == 4 {
			frames = 43
		}
		z.AbilityUntil = s.Tick + counterTick(int64(frames*1000/12))
		z.Flags["contactAt"] = s.Tick + counterTick(int64(frames*1000*42/(12*100)))
	case "boss-head-spit":
		z.State = "boss-head-idle-after"
		z.Flags["headStage"] = 4
		z.AbilityUntil = s.Tick + 60
	case "boss-head-idle-after":
		z.State = "boss-head-leave"
		z.Flags["headStage"] = 5
		z.Flags["headOpen"] = 0
		z.AbilityUntil = s.Tick + 75
	case "boss-head-leave":
		z.State = "boss-idle"
		z.Flags["headStage"] = 0
		z.Flags["headOpen"] = 0
		z.Flags["idleAt"] = s.Tick + int64(20+s.random(21))
	}
}
func (s *State) bossContact(z *Zombie) {
	switch z.State {
	case "boss-head-spit":
		kind := "bossfire"
		if z.Flags["element"] == 1 {
			kind = "bossice"
		}
		s.zombieProjectile(z, kind, 100000, nil)
		s.emit("boss-ball", z.Row, z.X, z.Owner, kind, z.ID, 0)
	case "boss-stomp":
		for row := z.Row; row < min(s.rowCount(), z.Row+2); row++ {
			for col := 5; col < Columns; col++ {
				s.zombieCrushCell(row, col)
			}
		}
		s.emit("boss-impact", z.Row, 5500, z.Owner, z.Kind, z.ID, 100000)
	case "boss-rv":
		col := int(z.Flags["targetCol"])
		for row := z.Row; row < min(s.rowCount(), z.Row+2); row++ {
			for c := col; c < min(Columns, col+3); c++ {
				s.zombieCrushCell(row, c)
			}
		}
		s.emit("boss-rv-impact", z.Row, col*1000+500, z.Owner, z.Kind, z.ID, 100000)
	case "boss-bungee":
		for n := 0; n < 3; n++ {
			row := s.random(s.rowCount())
			col := int(z.Flags["targetCol"]) + n
			id := s.spawnZombie("bungee", row, col*1000+500, z.Owner)
			if child := s.zombieByID(id); child != nil {
				child.Allegiance = z.Allegiance
				child.X = col*1000 + 500
			}
		}
	case "boss-summon":
		choices := []string{"zombie", "conehead", "buckethead", "football", "ladder", "catapult", "gargantuar"}
		limit := 4 + int(z.Flags["damagePhase"])
		kind := choices[s.random(min(len(choices), limit))]
		row := z.Row
		id := s.spawnZombie(kind, row, 9000, z.Owner)
		if child := s.zombieByID(id); child != nil {
			child.Allegiance = z.Allegiance
		}
		s.emit("boss-spawn", row, 9000, z.Owner, kind, id, 0)
	}
}
