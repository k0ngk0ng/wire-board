package battlecity

import (
	"math"

	"github.com/k0ngk0ng/wire-board/games/battlecity/maps"
)

func parseMove(move string) (int, bool) {
	switch move {
	case "up":
		return DirUp, true
	case "right":
		return DirRight, true
	case "down":
		return DirDown, true
	case "left":
		return DirLeft, true
	}
	return 0, false
}

func (s *State) baseAt(cx, cy int) (int, bool) {
	if cx == 6 && cy == 0 {
		return TeamTop, true
	}
	if cx == 6 && cy == 12 {
		return TeamBottom, true
	}
	return 0, false
}

func (s *State) isRing(team, cx, cy int) bool {
	for _, p := range maps.Ring(team) {
		if p.X == cx && p.Y == cy {
			return true
		}
	}
	return false
}

func ringIndexes(team int) []int {
	out := make([]int, 0, 20)
	for _, p := range maps.Ring(team) {
		for sy := p.Y * 2; sy < p.Y*2+2; sy++ {
			for sx := p.X * 2; sx < p.X*2+2; sx++ {
				out = append(out, sy*SubCols+sx)
			}
		}
	}
	return out
}

// subSolid reports whether an 8x8 sub-cell blocks tanks and bullets.
func (s *State) subSolid(sx, sy int, at int64) bool {
	if sx < 0 || sx >= SubCols || sy < 0 || sy >= SubRows {
		return true
	}
	cx, cy := sx/2, sy/2
	if _, ok := s.baseAt(cx, cy); ok {
		return true
	}
	idx := sy*SubCols + sx
	for team := 0; team < 2; team++ {
		if at < s.ShovelUntil[team] && s.isRing(team, cx, cy) {
			return s.Broken[idx] == 0 // shovel steel, destructible by tech 3
		}
	}
	switch s.cells[cy][cx] {
	case maps.Brick, maps.Steel:
		return s.Broken[idx] == 0
	case maps.Water:
		return true
	}
	return false
}

func (s *State) terrainAt(x, y float64, at int64) (sx, sy int, solid bool) {
	sx = int(math.Floor(x / SubSize))
	sy = int(math.Floor(y / SubSize))
	return sx, sy, s.subSolid(sx, sy, at)
}

func (s *State) tankFree(seat int, x, y float64, at int64) bool {
	if x < 0 || y < 0 || x+Cell > Cols*Cell || y+Cell > Rows*Cell {
		return false
	}
	x0 := int(math.Floor(x / SubSize))
	y0 := int(math.Floor(y / SubSize))
	x1 := int(math.Floor((x + Cell - 0.001) / SubSize))
	y1 := int(math.Floor((y + Cell - 0.001) / SubSize))
	for sy := y0; sy <= y1; sy++ {
		for sx := x0; sx <= x1; sx++ {
			if s.subSolid(sx, sy, at) {
				return false
			}
		}
	}
	for i := range s.Tanks {
		o := &s.Tanks[i]
		if i == seat || !o.Alive {
			continue
		}
		if o.X < x+Cell && x < o.X+Cell && o.Y < y+Cell && y < o.Y+Cell {
			return false
		}
	}
	return true
}

func (s *State) onIce(t *Tank) bool {
	cx := int((t.X + Cell/2) / Cell)
	cy := int((t.Y + Cell/2) / Cell)
	if cx < 0 || cx >= Cols || cy < 0 || cy >= Rows {
		return false
	}
	return s.cells[cy][cx] == maps.Ice
}

func (s *State) turn(t *Tank, dir int, at int64) {
	x, y := t.X, t.Y
	if dir == DirLeft || dir == DirRight {
		y = math.Round(y/SubSize) * SubSize
	} else {
		x = math.Round(x/SubSize) * SubSize
	}
	if s.tankFree(t.Seat, x, y, at) {
		t.X, t.Y, t.Dir = x, y, dir
	}
}

func (s *State) moveTank(t *Tank, dir int, amount float64, at int64) {
	if amount <= 0 {
		return
	}
	nx := t.X + dirDX[dir]*amount
	ny := t.Y + dirDY[dir]*amount
	nx = math.Max(0, math.Min(nx, float64(Cols*Cell-Cell)))
	ny = math.Max(0, math.Min(ny, float64(Rows*Cell-Cell)))
	if s.tankFree(t.Seat, nx, ny, at) {
		t.X, t.Y = nx, ny
		t.SlideDir = dir
	}
}

func (s *State) stepTank(t *Tank, at int64, dt float64) {
	if !t.Alive || t.Class == "" || t.HP <= 0 {
		return
	}
	if at < t.FreezeUntil {
		return
	}
	in := t.Applied
	if at-t.LastInputAt > LostMS {
		in.Move, in.Fire = "none", false
	}
	ice := s.onIce(t)
	if dir, ok := parseMove(in.Move); ok {
		if dir != t.Dir {
			s.turn(t, dir, at)
		}
		speed := s.spec(t.Class).Speed
		if ice {
			speed *= 1.1
		}
		s.moveTank(t, t.Dir, speed*dt, at)
		if ice {
			t.SlideUntil = at + 150
		} else {
			t.SlideUntil = 0
		}
	} else if t.SlideUntil > at && t.SlideDir >= 0 {
		s.moveTank(t, t.SlideDir, s.spec(t.Class).Speed*0.6*dt, at)
	}
	if in.Fire && at >= t.LastFireAt+s.spec(t.Class).ReloadMS && s.activeBullets(t.Seat) < s.maxBullets(t) {
		s.fire(t, at)
	}
}

func (s *State) fire(t *Tank, at int64) {
	if len(s.Bullets) >= MaxBullets {
		return
	}
	t.LastFireAt = at
	s.BulletID++
	cx := t.X + Cell/2 + dirDX[t.Dir]*10
	cy := t.Y + Cell/2 + dirDY[t.Dir]*10
	s.Bullets = append(s.Bullets, Bullet{
		ID: s.BulletID, Seat: t.Seat, Team: t.Team, X: cx, Y: cy,
		Dir: t.Dir, Speed: s.bulletSpeed(t), Tech: t.Tech, At: at,
	})
	s.emit("fire", t.Seat, t.Team, at, cx, cy, 0, "", 0)
}

func (s *State) damageTank(t *Tank, from int, at int64, cause string) {
	if !t.Alive {
		return
	}
	if at < t.ShieldUntil {
		s.emit("shield", from, t.Team, at, t.X, t.Y, t.Seat, "", 0)
		return
	}
	t.HP--
	s.emit("hit", from, t.Team, at, t.X, t.Y, t.Seat, cause, t.HP)
	if t.HP <= 0 {
		s.killTank(t, from, at, cause)
	}
}

func (s *State) killTank(t *Tank, killer int, at int64, cause string) {
	if !t.Alive {
		return
	}
	t.Alive = false
	t.Deaths++
	t.HP = 0
	t.Tech = 0
	t.FreezeUntil = 0
	t.ShieldUntil = 0
	t.RespawnAt = at + RespawnMS
	if t.Spare {
		t.Spare = false
		t.RespawnAt = at
		t.ShieldUntil = at + SpareMS
	}
	if killer >= 0 && killer < len(s.Tanks) && killer != t.Seat {
		s.Tanks[killer].Kills++
	}
	s.emit("kill", killer, t.Team, at, t.X, t.Y, t.Seat, cause, 0)
	if cause == "bullet" {
		s.maybeDrop(t, at)
	}
}

func (s *State) trySpawn(t *Tank, at int64, shield int64) bool {
	team := t.Team
	spawns := maps.SpawnPoints(s.TeamSize, team)
	if len(spawns) == 0 {
		return false
	}
	start := t.SpawnIndex
	if start < 0 || start >= len(spawns) {
		start = 0
	}
	for k := 0; k < len(spawns); k++ {
		p := spawns[(start+k)%len(spawns)]
		x, y := float64(p.X*Cell), float64(p.Y*Cell)
		if !s.tankFree(t.Seat, x, y, at) {
			continue
		}
		t.X, t.Y = x, y
		t.Dir = DirUp
		if team == TeamBottom {
			t.Dir = DirDown
		}
		t.Alive = true
		t.MaxHP = s.maxHP(t)
		t.HP = t.MaxHP
		t.RespawnAt = 0
		t.SpawnIndex = (start + k) % len(spawns)
		if t.ShieldUntil < at {
			t.ShieldUntil = at + shield
		}
		s.emit("respawn", t.Seat, t.Team, at, t.X, t.Y, 0, "", 0)
		return true
	}
	return false
}

func (s *State) tryRespawns(at int64) {
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Selected && !t.Alive && t.RespawnAt > 0 && at >= t.RespawnAt {
			if !s.trySpawn(t, at, ShieldMS) {
				t.RespawnAt = at + 250
			}
		}
	}
}

func (s *State) baseCellHit(x, y float64) (int, bool) {
	cx := int(math.Floor(x / Cell))
	cy := int(math.Floor(y / Cell))
	return s.baseAt(cx, cy)
}

func (s *State) bulletHitsTerrain(b *Bullet, x, y float64, at int64) (remove bool) {
	if x < 0 || y < 0 || x >= Cols*Cell || y >= Rows*Cell {
		return true
	}
	if team, ok := s.baseCellHit(x, y); ok {
		if b.Team == team {
			return false // own base is immune; let the bullet pass
		}
		base := &s.Bases[team]
		if base.Alive {
			base.Alive = false
			base.DestroyedAt = at
			base.DestroyedBy = b.Seat
			base.DestroyTick = s.Tick
			s.emit("base_destroyed", b.Seat, team, at, x, y, team, "", 0)
			s.log("大本营被摧毁")
		}
		return true
	}
	sx, sy := int(math.Floor(x/SubSize)), int(math.Floor(y/SubSize))
	if sx < 0 || sx >= SubCols || sy < 0 || sy >= SubRows {
		return true
	}
	cx, cy := sx/2, sy/2
	code := s.cells[cy][cx]
	idx := sy*SubCols + sx
	shovelSteel := false
	for team := 0; team < 2; team++ {
		if at < s.ShovelUntil[team] && s.isRing(team, cx, cy) {
			shovelSteel = true
		}
	}
	switch code {
	case maps.Brick:
		if shovelSteel {
			if b.Tech < 3 {
				s.emit("steel", b.Seat, b.Team, at, x, y, 0, "", 0)
				return true
			}
			s.Broken[idx] = 1
			s.emit("steel", b.Seat, b.Team, at, x, y, 0, "", 0)
			return true
		}
		if s.Broken[idx] == 0 {
			s.Broken[idx] = 1
			s.emit("brick", b.Seat, b.Team, at, x, y, 0, "", 0)
			return true
		}
		return false
	case maps.Steel:
		if s.Broken[idx] == 0 {
			if b.Tech >= 3 {
				s.Broken[idx] = 1
			}
			s.emit("steel", b.Seat, b.Team, at, x, y, 0, "", 0)
			return true
		}
		return false
	}
	return false
}

func (s *State) bulletHitsTank(b *Bullet, at int64) bool {
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive || t.Team == b.Team {
			continue
		}
		if b.X+3 > t.X && b.X-3 < t.X+Cell && b.Y+3 > t.Y && b.Y-3 < t.Y+Cell {
			if at < t.ShieldUntil {
				s.emit("shield", b.Seat, t.Team, at, b.X, b.Y, t.Seat, "", 0)
			} else {
				s.damageTank(t, b.Seat, at, "bullet")
			}
			return true
		}
	}
	return false
}

func (s *State) bulletHitsBullet(b *Bullet, removed map[int]bool) (bool, int) {
	for i := range s.Bullets {
		o := &s.Bullets[i]
		if o.ID == b.ID || removed[o.ID] {
			continue
		}
		if math.Abs(o.X-b.X) < 6 && math.Abs(o.Y-b.Y) < 6 {
			return true, o.ID
		}
	}
	return false, 0
}

func (s *State) stepBullets(at int64, dt float64) {
	if len(s.Bullets) == 0 {
		return
	}
	removed := map[int]bool{}
	for i := range s.Bullets {
		b := &s.Bullets[i]
		if removed[b.ID] {
			continue
		}
		dx, dy := dirDX[b.Dir]*b.Speed*dt, dirDY[b.Dir]*b.Speed*dt
		steps := int(math.Ceil(math.Hypot(dx, dy) / 2))
		if steps < 1 {
			steps = 1
		}
		for k := 1; k <= steps; k++ {
			x := b.X + dx*float64(k)/float64(steps)
			y := b.Y + dy*float64(k)/float64(steps)
			b.X, b.Y = x, y
			if hit, other := s.bulletHitsBullet(b, removed); hit {
				removed[b.ID] = true
				removed[other] = true
				s.emit("bullet", b.Seat, b.Team, at, x, y, other, "", 0)
				break
			}
			if s.bulletHitsTerrain(b, x, y, at) {
				removed[b.ID] = true
				break
			}
			if s.bulletHitsTank(b, at) {
				removed[b.ID] = true
				break
			}
		}
	}
	if len(removed) == 0 {
		return
	}
	out := s.Bullets[:0]
	for _, b := range s.Bullets {
		if !removed[b.ID] {
			out = append(out, b)
		}
	}
	s.Bullets = out
}

func (s *State) maybeDrop(v *Tank, at int64) {
	if len(s.PowerUps) >= MaxPowerUps || s.randN(100) >= 30 {
		return
	}
	kinds := []string{"star", "grenade", "helmet", "shovel", "tank", "clock"}
	kind := kinds[s.randN(len(kinds))]
	x, y, ok := s.freePowerUpPos(v.X, v.Y)
	if !ok {
		return
	}
	s.spawnPowerUp(kind, x, y, at)
}

func (s *State) freePowerUpPos(x, y float64) (float64, float64, bool) {
	cx := int(math.Floor(x / Cell))
	cy := int(math.Floor(y / Cell))
	if cx < 0 {
		cx = 0
	}
	if cy < 0 {
		cy = 0
	}
	bestX, bestY, best := 0, 0, 1<<30
	for yy := 0; yy < Rows; yy++ {
		for xx := 0; xx < Cols; xx++ {
			if s.cells[yy][xx] != maps.Empty && s.cells[yy][xx] != maps.Ice {
				continue
			}
			if _, ok := s.baseAt(xx, yy); ok {
				continue
			}
			px, py := float64(xx*Cell), float64(yy*Cell)
			blocked := false
			for _, p := range s.PowerUps {
				if math.Abs(p.X-px) < Cell && math.Abs(p.Y-py) < Cell {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			d := absInt(xx-cx) + absInt(yy-cy)
			if d < best {
				best, bestX, bestY = d, xx, yy
			}
		}
	}
	if best == 1<<30 {
		return 0, 0, false
	}
	return float64(bestX * Cell), float64(bestY * Cell), true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (s *State) spawnPowerUp(kind string, x, y float64, at int64) {
	if len(s.PowerUps) >= MaxPowerUps {
		return
	}
	s.PowerUpID++
	p := PowerUp{ID: s.PowerUpID, Type: kind, X: x, Y: y, SpawnAt: at, ExpireAt: at + DropMS}
	s.PowerUps = append(s.PowerUps, p)
	s.emit("powerup_spawn", -1, -1, at, x, y, 0, kind, 0)
}

func (s *State) pickupPowerUps(at int64) {
	if len(s.PowerUps) == 0 {
		return
	}
	used := make([]bool, len(s.PowerUps))
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if !t.Alive {
			continue
		}
		for j := range s.PowerUps {
			if used[j] {
				continue
			}
			p := &s.PowerUps[j]
			if t.X < p.X+Cell && p.X < t.X+Cell && t.Y < p.Y+Cell && p.Y < t.Y+Cell {
				used[j] = true
				s.applyPowerUp(t, p.Type, at)
				s.emit("powerup_pickup", t.Seat, t.Team, at, p.X, p.Y, 0, p.Type, 0)
			}
		}
	}
	out := s.PowerUps[:0]
	for j, p := range s.PowerUps {
		if !used[j] && at < p.ExpireAt {
			out = append(out, p)
		}
	}
	s.PowerUps = out
}

func (s *State) applyPowerUp(t *Tank, kind string, at int64) {
	switch kind {
	case "star":
		if t.Tech < 3 {
			t.Tech++
		}
	case "grenade":
		for i := range s.Tanks {
			o := &s.Tanks[i]
			if o.Team != t.Team && o.Alive {
				s.killTank(o, t.Seat, at, "grenade")
			}
		}
		s.emit("grenade", t.Seat, t.Team, at, t.X, t.Y, 0, "", 0)
	case "helmet":
		if t.ShieldUntil < at {
			t.ShieldUntil = at + 8000
		} else {
			t.ShieldUntil += 8000
		}
	case "shovel":
		s.ShovelUntil[t.Team] = at + ShovelMS
		for _, idx := range ringIndexes(t.Team) {
			s.Broken[idx] = 0
		}
		s.emit("shovel", t.Seat, t.Team, at, t.X, t.Y, t.Team, "", 0)
	case "tank":
		t.Spare = true
	case "clock":
		for i := range s.Tanks {
			o := &s.Tanks[i]
			if o.Team != t.Team && o.Alive {
				if o.FreezeUntil < at {
					o.FreezeUntil = at + FreezeMS
				} else {
					o.FreezeUntil += FreezeMS
				}
			}
		}
		s.emit("freeze", t.Seat, t.Team, at, t.X, t.Y, 0, "", 0)
	}
}

func (s *State) tickShovel(at int64) {
	for team := 0; team < 2; team++ {
		if s.ShovelUntil[team] > 0 && at >= s.ShovelUntil[team] {
			s.ShovelUntil[team] = 0
			for _, idx := range ringIndexes(team) {
				s.Broken[idx] = 0
			}
			s.emit("shovel_end", -1, team, at, 0, 0, team, "", 0)
		}
	}
}

func (s *State) damageBaseForTest(team int, at int64) {
	if team < 0 || team > 1 || !s.Bases[team].Alive {
		return
	}
	s.Bases[team].Alive = false
	s.Bases[team].DestroyedAt = at
	s.Bases[team].DestroyedBy = -1
	s.Bases[team].DestroyTick = s.Tick
	s.emit("base_destroyed", -1, team, at, 0, 0, team, "", 0)
}
