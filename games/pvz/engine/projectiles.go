package engine

import "slices"

func (s *State) launchPlantProjectile(p Plant, row, direction int, target *Zombie) {
	if len(s.Projectiles) >= MaxProjectiles || row < 0 || row >= s.rowCount() {
		return
	}
	q := Projectile{ID: s.id(), Owner: p.Owner, Row: row, X: PlantX(p) + 200*direction, Y: p.Row*1000 + 500, VX: 208 * direction, Kind: "pea", Damage: 20, BornAt: s.Tick}
	switch p.Kind {
	case "snowpea":
		q.Kind = "icepea"
	case "puffshroom", "seashroom":
		q.Kind = "spore"
		q.Remaining = 3000
	case "scaredyshroom":
		q.Kind = "spore"
	case "cactus":
		q.Kind = "needle"
		if target != nil && target.Altitude > 0 {
			q.Payload = "air"
			q.TargetID = target.ID
			q.Height = 600
		}
	case "cattail":
		q.Kind = "needle"
		q.Payload = "homing"
		q.Height = 300
		if target != nil {
			q.TargetID = target.ID
		}
		q.VX = 120
	case "cabbagepult":
		q.Kind = "cabbage"
		q.Damage = 40
	case "kernelpult":
		q.Kind = "kernel"
		q.Damage = 20
		if s.random(4) == 0 {
			q.Kind = "butter"
			q.Damage = 40
		}
	case "melonpult":
		q.Kind = "melon"
		q.Damage = 80
		q.Radius = 1250
	case "wintermelon":
		q.Kind = "wintermelon"
		q.Damage = 80
		q.Radius = 1250
	}
	if p.Kind == "threepeater" && row != p.Row {
		q.Payload = "threepeater"
		q.TargetY = row*1000 + 500
	}
	if p.Kind == "cabbagepult" || p.Kind == "kernelpult" || p.Kind == "melonpult" || p.Kind == "wintermelon" {
		if target == nil {
			return
		}
		q.LandAt = s.Tick + 24
		q.TargetID = target.ID
		q.TargetX = max(PlantX(p)+400, target.X-target.SpeedMilli*24/1000)
		q.TargetY = target.Row*1000 + 500
		q.Height = 1200
		q.VX = 0
		q.Payload = "lob"
	} else if s.Map.Roof && p.Col < 4 && q.Payload != "air" && q.Payload != "homing" {
		q.Payload = "roofblocked"
	}
	s.Projectiles = append(s.Projectiles, q)
	s.emit("shoot", p.Row, PlantX(p), p.Owner, q.Kind, p.ID, q.Damage)
}
func (s *State) launchStarProjectiles(p Plant) {
	vectors := [][2]int{{-208, 0}, {0, 208}, {0, -208}, {180, 104}, {180, -104}}
	for _, v := range vectors {
		if len(s.Projectiles) >= MaxProjectiles {
			break
		}
		s.Projectiles = append(s.Projectiles, Projectile{ID: s.id(), Owner: p.Owner, Row: p.Row, X: PlantX(p), Y: p.Row*1000 + 500, VX: v[0], VY: v[1], Kind: "star", Damage: 20, BornAt: s.Tick})
	}
	s.emit("shoot", p.Row, PlantX(p), p.Owner, "star", p.ID, 20)
}
func (s *State) shoot(p Plant) { s.launchPlantProjectile(p, p.Row, 1, nil) }

func (s *State) projectileStep() {
	live := make([]Projectile, 0, len(s.Projectiles))
	for _, q := range s.Projectiles {
		if q.Y == 0 {
			q.Y = q.Row*1000 + 500
		}
		if s.Tick-q.BornAt > 400 {
			continue
		}
		if q.Kind == "spore" && q.Remaining > 0 && s.Tick-q.BornAt >= 15 {
			continue
		}
		hostile := s.projectileHostile(q)
		if q.LandAt > 0 {
			left := max(int64(1), q.LandAt-s.Tick+1)
			q.X += (q.TargetX - q.X) / int(left)
			q.Y += (q.TargetY - q.Y) / int(left)
			duration := max(int64(1), q.LandAt-q.BornAt)
			age := min(duration, max(int64(0), s.Tick-q.BornAt))
			height := int64(1200)
			if q.Kind == "cob" {
				height = 3500
			}
			if q.Kind == "basketball" {
				height = 1800
			}
			q.Height = int(4 * height * age * (duration - age) / (duration * duration))
			if s.Tick >= q.LandAt {
				q.X = q.TargetX
				q.Y = q.TargetY
				s.projectileImpact(q, hostile, 0)
				continue
			}
			live = append(live, q)
			continue
		}
		oldX, oldY := q.X, q.Y
		if q.Kind == "bossfire" || q.Kind == "bossice" {
			speed := int64(23145)
			if q.Kind == "bossice" {
				speed = 22347
			}
			age := max(int64(1), s.Tick-q.BornAt)
			q.VX = -int(age*speed/1000 - (age-1)*speed/1000)
		}
		if q.Payload == "threepeater" {
			gap := q.TargetY - q.Y
			q.VY = gap * 141 / 1000
			if gap != 0 && q.VY == 0 {
				if gap > 0 {
					q.VY = 1
				} else {
					q.VY = -1
				}
			}
		}
		if q.Payload == "homing" {
			target := s.plantZombieByID(q.TargetID)
			if target == nil || !plantEnemy(*target) || target.Digging || target.Diving {
				target = s.homingTarget(q)
				if target != nil {
					q.TargetID = target.ID
				}
			}
			if target == nil {
				continue
			}
			dx, dy := target.X-q.X, target.Row*1000+500-q.Y
			d := max(1, projectileSqrt(int64(dx)*int64(dx)+int64(dy)*int64(dy)))
			q.VX = dx * 270 / d
			q.VY = dy * 270 / d
			if d < 270 {
				q.VX = dx
				q.VY = dy
			}
		}
		if q.VX == 0 && q.VY == 0 {
			q.VX = 208
			if hostile {
				q.VX = -208
			}
		}
		q.X += q.VX
		q.Y += q.VY
		q.Row = min(s.rowCount()-1, max(0, q.Y/1000))
		if q.Kind == "pea" || q.Kind == "icepea" {
			s.torchProjectile(&q, oldX)
		}
		if q.Payload == "roofblocked" && s.Tick-q.BornAt >= 8 {
			s.emit("impact", q.Row, q.X, q.Owner, q.Kind, 0, 0)
			continue
		}
		if q.Remaining > 0 && q.Kind != "spore" {
			q.Remaining -= abs(q.VX) + abs(q.VY)
			if q.Remaining <= 0 {
				continue
			}
		}
		if hostile {
			if s.hostileProjectileHit(&q, oldX, oldY) {
				if q.Kind != "bossfire" && q.Kind != "bossice" && !q.Pierce {
					continue
				}
			}
		} else {
			hit := s.projectileZombieTarget(q, oldX, oldY)
			if hit > 0 {
				s.projectileImpact(q, false, hit)
				if q.Pierce {
					q.Hits = append(q.Hits, hit)
				} else {
					continue
				}
			}
		}
		if q.X < -1000 || q.X > 11000 || q.Y < -1000 || q.Y > s.rowCount()*1000+1000 {
			continue
		}
		q.Row = min(s.rowCount()-1, max(0, q.Y/1000))
		live = append(live, q)
	}
	s.Projectiles = live
}
func (s *State) projectileHostile(q Projectile) bool {
	if q.Payload == "friendly" {
		return false
	}
	if q.Payload == "hostile" {
		return true
	}
	if q.Owner >= 0 && q.Owner < len(s.Players) && s.Players[q.Owner].Side == "zombies" {
		return true
	}
	return slices.Contains([]string{"zombiepea", "zombieicepea", "zombiefirepea", "basketball", "bossfire", "bossice"}, q.Kind)
}
func (s *State) homingTarget(q Projectile) *Zombie {
	best := -1
	score := int(^uint(0) >> 1)
	for i, z := range s.Zombies {
		if !plantEnemy(z) || z.Digging || z.Diving {
			continue
		}
		d := plantPixelDistance(q.X, q.Y, z.X, z.Row*1000+500)
		if z.Altitude > 0 {
			d -= 10000000
		}
		if d < score {
			best = i
			score = d
		}
	}
	if best < 0 {
		return nil
	}
	return &s.Zombies[best]
}
func (s *State) torchProjectile(q *Projectile, oldX int) {
	for _, p := range s.Plants {
		if p.HP <= 0 || p.Asleep || p.Kind != "torchwood" || p.Row != q.Row {
			continue
		}
		x := PlantX(p)
		cross := q.VX > 0 && oldX < x && q.X >= x || q.VX < 0 && oldX > x && q.X <= x
		if !cross {
			continue
		}
		if q.Kind == "icepea" {
			q.Kind = "pea"
			q.Damage = 20
		} else if q.Kind == "pea" {
			q.Kind = "firepea"
			q.Damage = 40
			q.Radius = 1250
		}
		s.emit("torch", q.Row, x, p.Owner, q.Kind, q.ID, q.Damage)
	}
}
func projectileTargetPlantKind(q Projectile) string {
	switch q.Kind {
	case "cabbage":
		return "cabbagepult"
	case "kernel", "butter":
		return "kernelpult"
	case "melon":
		return "melonpult"
	case "wintermelon":
		return "wintermelon"
	case "cob":
		return "cobcannon"
	case "spore":
		return "puffshroom"
	case "needle":
		if q.Payload == "homing" {
			return "cattail"
		}
		return "cactus"
	default:
		return "peashooter"
	}
}
func (s *State) projectileZombieTarget(q Projectile, oldX, oldY int) int {
	best, rank := 0, int(^uint(0)>>1)
	for _, z := range s.Zombies {
		if !s.plantTargetCategory(projectileTargetPlantKind(q), z, q.Payload == "air") || slices.Contains(q.Hits, z.ID) {
			continue
		}
		air := q.Payload == "air" || q.Payload == "homing"
		if q.Payload == "air" && z.Altitude == 0 {
			continue
		}
		if z.Altitude > 0 && !air {
			continue
		}
		zy := z.Row*1000 + 500
		if z.Kind == "zomboss" {
			zy = q.Y
		}
		if !projectileNearSegment(oldX, oldY, q.X, q.Y, z.X, zy, 260) {
			continue
		}
		distance := abs(z.X-oldX) + abs(zy-oldY)
		if distance < rank {
			best = z.ID
			rank = distance
		}
	}
	return best
}
func projectileNearSegment(ax, ay, bx, by, x, y, radius int) bool {
	dx, dy := int64(bx-ax), int64(by-ay)
	px, py := int64(x-ax), int64(y-ay)
	length := dx*dx + dy*dy
	if length == 0 {
		return px*px+py*py <= int64(radius*radius)
	}
	t := max(int64(0), min(length, px*dx+py*dy))
	cx, cy := int64(ax)+dx*t/length, int64(ay)+dy*t/length
	ex, ey := int64(x)-cx, int64(y)-cy
	return ex*ex+ey*ey <= int64(radius*radius)
}
func projectileSqrt(x int64) int {
	if x <= 0 {
		return 0
	}
	r := x
	next := (r + 1) / 2
	for next < r {
		r = next
		next = (r + x/r) / 2
	}
	return int(r)
}
func (s *State) projectileImpact(q Projectile, hostile bool, targetID int) {
	if hostile {
		if q.Kind == "basketball" {
			s.hostileLobImpact(q)
		} else {
			s.hostileProjectileHit(&q, q.X-q.VX, q.Y-q.VY)
		}
		s.emit("impact", q.Row, q.X, q.Owner, q.Kind, targetID, q.Damage)
		return
	}
	if q.Kind == "cob" {
		ids := []int{}
		row := min(s.rowCount()-1, max(0, q.TargetY/1000))
		for _, z := range s.Zombies {
			if plantEnemy(z) && abs(z.Row-row) <= 1 && abs(z.X-q.X) <= 1438 {
				ids = append(ids, z.ID)
			}
		}
		for _, id := range ids {
			s.DamageZombie(id, 1800, "cob", q.Owner)
		}
		s.emit("explosion", row, q.X, q.Owner, "cob", 0, 1800)
		return
	}
	if targetID == 0 {
		best := 500
		for _, z := range s.Zombies {
			if !s.plantTargetCategory(projectileTargetPlantKind(q), z, false) || z.Row != q.Row {
				continue
			}
			d := abs(z.X - q.X)
			if d < best {
				best = d
				targetID = z.ID
			}
		}
	}
	if q.Kind == "needle" && targetID > 0 {
		if z := s.plantZombieByID(targetID); z != nil && z.Kind == "balloon" && z.Altitude > 0 {
			z.Altitude = 0
			if z.Flags == nil {
				z.Flags = map[string]int64{}
			}
			z.Flags["flyingHP"] = 0
			s.emit("balloon-pop", z.Row, z.X, q.Owner, q.Kind, z.ID, 0)
			return
		}
	}
	if q.Kind == "firepea" && targetID > 0 {
		if z := s.plantZombieByID(targetID); z != nil && projectileFireResistant(*z) {
			s.DamageZombie(targetID, q.Damage, "fireresisted", q.Owner)
			s.emit("hit", q.Row, q.X, q.Owner, q.Kind, targetID, q.Damage)
			return
		}
	}
	if q.Kind == "melon" || q.Kind == "wintermelon" || q.Kind == "firepea" {
		s.projectileSplash(q, targetID)
	} else if targetID > 0 {
		source := q.Kind
		if q.Kind == "zombiepea" {
			source = "pea"
		}
		if q.VX < 0 && (q.Kind == "pea" || q.Kind == "star") {
			source = "backwards"
		}
		s.DamageZombie(targetID, q.Damage, source, q.Owner)
	}
	s.emit("hit", q.Row, q.X, q.Owner, q.Kind, targetID, q.Damage)
}
func projectileFireResistant(z Zombie) bool {
	return z.Kind == "zomboni" || z.Kind == "catapult" || (z.Kind == "screendoor" || z.Kind == "ladder") && z.Armor > 0
}
func (s *State) projectileSplash(q Projectile, targetID int) {
	ids := []int{}
	for _, z := range s.Zombies {
		if !s.plantTargetCategory(projectileTargetPlantKind(q), z, false) {
			continue
		}
		if q.Kind == "firepea" && projectileFireResistant(z) {
			continue
		}
		rows := 1
		if q.Kind == "firepea" {
			rows = 0
		}
		if abs(z.Row-q.Row) <= rows && abs(z.X-q.X) <= max(500, q.Radius) {
			ids = append(ids, z.ID)
		}
	}
	splashed := 0
	for _, id := range ids {
		if id != targetID {
			splashed++
		}
	}
	amount := q.Damage / 3
	capDamage := q.Damage * 7
	if q.Kind == "firepea" {
		capDamage = q.Damage
	}
	if amount*splashed > capDamage && splashed > 0 {
		amount = max(1, q.Damage*capDamage/(amount*splashed*3))
	}
	for _, id := range ids {
		damage := amount
		if id == targetID {
			damage = q.Damage
		}
		s.DamageZombie(id, damage, q.Kind, q.Owner)
	}
}
func (s *State) hostileProjectileHit(q *Projectile, oldX, oldY int) bool {
	best := -1
	rank := int(^uint(0) >> 1)
	for i, p := range s.Plants {
		if p.HP <= 0 || slices.Contains(q.Hits, p.ID) {
			continue
		}
		x := PlantX(p)
		if !projectileNearSegment(oldX, oldY, q.X, q.Y, x, p.Row*1000+500, 300) {
			continue
		}
		priority := 0
		switch p.Layer {
		case "shell":
			priority = 3
		case "body":
			priority = 2
		case "support":
			priority = 1
		}
		distance := abs(x-oldX)*10 - priority
		if distance < rank {
			best = i
			rank = distance
		}
	}
	if best < 0 {
		return false
	}
	p := s.Plants[best]
	s.damagePlant(p.ID, q.Damage, nil)
	q.Hits = append(q.Hits, p.ID)
	s.emit("hit", p.Row, PlantX(p), q.Owner, q.Kind, p.ID, q.Damage)
	return true
}
func (s *State) hostileLobImpact(q Projectile) {
	row := min(s.rowCount()-1, max(0, q.TargetY/1000))
	col := min(Columns-1, max(0, q.TargetX/1000))
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.HP > 0 && !p.Asleep && p.Kind == "umbrellaleaf" && abs(p.Row-row) <= 1 && abs(p.Col-col) <= 1 {
			p.State = "reflecting"
			p.EffectUntil = s.Tick + 20
			s.emit("umbrella", row, q.TargetX, p.Owner, p.Kind, p.ID, 0)
			return
		}
	}
	best := -1
	priority := 0
	for i, p := range s.Plants {
		if p.HP <= 0 || p.Row != row || p.Col != col {
			continue
		}
		rank := 1
		if p.Layer == "body" {
			rank = 2
		}
		if p.Layer == "shell" {
			rank = 3
		}
		if rank > priority {
			best = i
			priority = rank
		}
	}
	if best >= 0 {
		s.damagePlant(s.Plants[best].ID, q.Damage, nil)
	}
}
