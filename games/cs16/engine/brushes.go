package engine

func brushDynamic(state *State, id int) *BrushState {
	for i := range state.Brushes {
		if state.Brushes[i].ID == id {
			return &state.Brushes[i]
		}
	}
	return nil
}

func brushCenter(brush Brush) Vec3 {
	return Vec3{(brush.Bounds.Min.X + brush.Bounds.Max.X) / 2, (brush.Bounds.Min.Y + brush.Bounds.Max.Y) / 2, (brush.Bounds.Min.Z + brush.Bounds.Max.Z) / 2}
}

func doorSign(positive bool) int {
	if positive {
		return 1
	}
	return -1
}

func doorOpeningDirection(brush Brush, position Vec3) int {
	lo, hi := brush.Bounds.Min, brush.Bounds.Max
	mx, my := position.X-brush.Origin.X, position.Y-brush.Origin.Y
	yDominates := Abs(my/Unit) > Abs(mx/Unit)
	if position.X < lo.X {
		if position.Y < lo.Y {
			if yDominates {
				return doorSign(my < 0)
			}
			return doorSign(mx > 0)
		}
		if position.Y > hi.Y {
			if yDominates {
				return doorSign(my < 0)
			}
			return doorSign(mx < 0)
		}
		return doorSign(my < 0)
	}
	if position.X <= hi.X {
		if position.Y < lo.Y {
			return doorSign(mx > 0)
		}
		if position.Y > hi.Y {
			return doorSign(mx < 0)
		}
		return 1
	}
	if position.Y < lo.Y {
		if yDominates {
			return doorSign(my > 0)
		}
		return doorSign(mx > 0)
	}
	if position.Y > hi.Y {
		if yDominates {
			return doorSign(my > 0)
		}
		return doorSign(mx < 0)
	}
	return doorSign(my > 0)
}

func activateBrush(state *State, index int, activator *Player) {
	brush := state.World.Brushes[index]
	dynamic := brushDynamic(state, brush.ID)
	if dynamic == nil || dynamic.Progress > 0 {
		return
	}
	dynamic.Direction = 1
	if brush.Kind == "func_door_rotating" && brush.Flags&16 == 0 && activator != nil {
		dynamic.Direction = doorOpeningDirection(brush, Position(*activator))
	}
	dynamic.Opening = true
	Emit(state, Event{Kind: "door", Seat: -1, Target: brush.ID, Value: 1})
	if brush.Target != "" {
		for target, other := range state.World.Brushes {
			if target != index && other.TargetName == brush.Target {
				activateBrush(state, target, activator)
			}
		}
	}
}

func stepBrushes(state *State) {
	if state.Phase != "live" {
		return
	}
	for index, brush := range state.World.Brushes {
		if brush.Kind != "func_door" && brush.Kind != "func_door_rotating" && brush.Kind != "func_button" {
			continue
		}
		dynamic := brushDynamic(state, brush.ID)
		if dynamic == nil {
			continue
		}
		for _, player := range state.Players {
			if !player.Alive {
				continue
			}
			p := Position(player)
			closest := Vec3{Clamp(p.X, brush.Bounds.Min.X, brush.Bounds.Max.X), Clamp(p.Y, brush.Bounds.Min.Y, brush.Bounds.Max.Y), Clamp(p.Z, brush.Bounds.Min.Z, brush.Bounds.Max.Z)}
			touch := brush.Kind != "func_button" && brush.TargetName == "" && Distance(p, closest) <= 24*Unit
			use := player.Input.Use && !player.Previous.Use && Distance(Eye(player), closest) <= 80*Unit
			if touch || use {
				activateBrush(state, index, &player)
			}
		}
		distance := Distance(Vec3{}, brush.Move)
		if brush.Kind == "func_door_rotating" {
			angle := Abs(brush.Angle)
			if angle == 0 {
				angle = 16384
			}
			distance = angle * 360 * Unit / 65536
		}
		if distance <= 0 {
			distance = 64 * Unit
		}
		speed := brush.Speed
		if speed <= 0 {
			speed = 100 * Unit
		}
		step := max(1, speed*1000/(FPS*distance))
		if dynamic.Opening {
			dynamic.Progress = min(1000, dynamic.Progress+step)
			if dynamic.Progress == 1000 {
				dynamic.Open = true
				dynamic.Opening = false
				dynamic.OpenedAt = state.Frame
			}
		} else if dynamic.Open && brush.Wait >= 0 {
			wait := brush.Wait
			if wait == 0 {
				wait = 3 * FPS
			}
			if state.Frame >= dynamic.OpenedAt+wait {
				dynamic.Open = false
			}
		} else if !dynamic.Open && dynamic.Progress > 0 {
			blocked := false
			for _, player := range state.Players {
				if !player.Alive {
					continue
				}
				expanded := brush.Bounds
				expanded.Min = Sub(expanded.Min, Vec3{16 * Unit, 16 * Unit, 36 * Unit})
				expanded.Max = Add(expanded.Max, Vec3{16 * Unit, 16 * Unit, 36 * Unit})
				if Contains(expanded, Position(player)) {
					blocked = true
					break
				}
			}
			if blocked {
				dynamic.Opening = true
			} else {
				dynamic.Progress = max(0, dynamic.Progress-step)
			}
		}
	}
}

func rotateZ(vector Vec3, angle int) Vec3 {
	sine, cosine := SinCos(angle)
	return Vec3{int((int64(vector.X)*int64(cosine) - int64(vector.Y)*int64(sine)) / NormalUnit), int((int64(vector.X)*int64(sine) + int64(vector.Y)*int64(cosine)) / NormalUnit), vector.Z}
}
