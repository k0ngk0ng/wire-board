package engine

func BotInput(s *State, seat int) Input {
	if seat < 0 || seat >= len(s.Players) {
		return Input{}
	}
	p := s.Players[seat]
	in := Input{Seq: p.ReceivedSeq + 1, Life: p.Life, Aim: p.Aim}
	if !p.Alive || p.DownUntil > s.Frame || s.Finished {
		return in
	}
	world := s.world()
	weapon, _ := WeaponByID(p.Weapon)
	target := -1
	nearest := int64(ViewportWidthPixels*Unit)*int64(ViewportWidthPixels*Unit) + int64(ViewportHeightPixels*Unit)*int64(ViewportHeightPixels*Unit) + 1
	for _, other := range s.Players {
		if other.Seat == seat || !other.Alive || !insideCamera(world, p.X, p.Y, other.X, other.Y) {
			continue
		}
		d := distance2(p.X, p.Y, other.X, other.Y)
		if d < nearest && Visible(world, s.Doors, s.BrokenGlass, p.X, p.Y, other.X, other.Y) {
			nearest = d
			target = other.Seat
		}
	}
	goal := Point{}
	if len(world.Spawns) > 0 {
		goal = world.Spawns[(int(s.Frame/180)+seat+1)%len(world.Spawns)]
	}
	if target >= 0 {
		other := s.Players[target]
		goal = Point{X: other.X, Y: other.Y}
		in.Aim = AimAt(other.X-p.X, other.Y-p.Y)
		in.Finish = other.DownUntil > s.Frame && nearest < int64(39*Unit)*int64(39*Unit)
		rangeLimit := weapon.Range
		if p.Weapon == "" {
			rangeLimit = 25 * Unit
		}
		in.Fire = other.ProtectedUntil <= s.Frame && nearest <= int64(rangeLimit+Radius)*int64(rangeLimit+Radius) && (weapon.Melee || p.Ammo > 0 || p.Weapon == "")
		if !weapon.Melee && p.Ammo > 0 && nearest < int64(200*Unit)*int64(200*Unit) {
			goal.X = p.X + (p.Y-other.Y)/3
			goal.Y = p.Y - (p.X-other.X)/3
		}
	}
	if p.Weapon == "" || !weapon.Melee && p.Ammo == 0 {
		best := int64(700*Unit) * int64(700*Unit)
		for _, drop := range s.Drops {
			w, ok := WeaponByID(drop.Weapon)
			if !ok || !w.Melee && drop.Ammo == 0 {
				continue
			}
			d := distance2(p.X, p.Y, drop.X, drop.Y)
			if d < best && insideCamera(world, p.X, p.Y, drop.X, drop.Y) && Visible(world, s.Doors, s.BrokenGlass, p.X, p.Y, drop.X, drop.Y) {
				best = d
				goal = Point{X: drop.X, Y: drop.Y}
				if d <= int64(40*Unit)*int64(40*Unit) {
					in.Pickup = true
				}
			}
		}
		if best == int64(700*Unit)*int64(700*Unit) && p.Weapon != "" {
			in.Throw = true
		}
	}
	next := navigationStep(world, p.X, p.Y, goal.X, goal.Y, s.BrokenGlass)
	dx, dy := next.X-p.X, next.Y-p.Y
	norm := max(abs(dx), abs(dy), 1)
	if abs(dx)+abs(dy) > 2*Unit {
		in.MoveX = dx * 1000 / norm
		in.MoveY = dy * 1000 / norm
	}
	for _, door := range world.Doors {
		if !doorOpen(s.Doors, door.ID) && distance2(p.X, p.Y, door.X+door.W/2, door.Y+door.H/2) < int64(52*Unit)*int64(52*Unit) {
			in.Interact = true
			break
		}
	}
	return in
}

func insideCamera(world Map, x, y, tx, ty int) bool {
	width, height := ViewportWidthPixels*Unit, ViewportHeightPixels*Unit
	left := clamp(x-width/2, 0, max(0, world.Width-width))
	top := clamp(y-height/2, 0, max(0, world.Height-height))
	return tx >= left && tx <= left+width && ty >= top && ty <= top+height
}

func navigationStep(world Map, x, y, tx, ty int, broken []int) Point {
	cell := 24 * Unit
	cols, rows := (world.Width+cell-1)/cell, (world.Height+cell-1)/cell
	if cols*rows <= 0 || cols*rows > 32768 {
		return Point{X: tx, Y: ty}
	}
	startX, startY := clamp(x/cell, 0, cols-1), clamp(y/cell, 0, rows-1)
	goalX, goalY := clamp(tx/cell, 0, cols-1), clamp(ty/cell, 0, rows-1)
	start, goal := startY*cols+startX, goalY*cols+goalX
	if start == goal {
		return Point{X: tx, Y: ty}
	}
	parents := make([]int, cols*rows)
	for i := range parents {
		parents[i] = -1
	}
	parents[start] = start
	queue := make([]int, 1, min(cols*rows, 8192))
	queue[0] = start
	closest := start
	closestDistance := abs(startX-goalX) + abs(startY-goalY)
	openDoors := make([]Door, 0, len(world.Doors))
	for _, door := range world.Doors {
		openDoors = append(openDoors, Door{ID: door.ID, Open: true})
	}
	for head := 0; head < len(queue) && head < 8192; head++ {
		current := queue[head]
		cx, cy := current%cols, current/cols
		d := abs(cx-goalX) + abs(cy-goalY)
		if d < closestDistance {
			closest = current
			closestDistance = d
		}
		if current == goal {
			closest = goal
			break
		}
		for _, delta := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
			nx, ny := cx+delta[0], cy+delta[1]
			if nx < 0 || ny < 0 || nx >= cols || ny >= rows {
				continue
			}
			index := ny*cols + nx
			if parents[index] >= 0 || blocked(world, openDoors, broken, nx*cell+cell/2, ny*cell+cell/2) {
				continue
			}
			parents[index] = current
			queue = append(queue, index)
		}
	}
	if closest == start {
		return Point{X: tx, Y: ty}
	}
	next := closest
	for parents[next] != start && parents[next] >= 0 && parents[next] != next {
		next = parents[next]
	}
	return Point{X: (next%cols)*cell + cell/2, Y: (next/cols)*cell + cell/2}
}
