package engine

type TraceResult struct {
	Fraction   int  `json:"fraction"`
	End        Vec3 `json:"end"`
	Normal     Vec3 `json:"normal"`
	StartSolid bool `json:"startSolid"`
	AllSolid   bool `json:"allSolid"`
	Brush      int  `json:"brush"`
	Plane      int  `json:"plane"`
}

func planeDistance(plane Plane, p Vec3) int {
	return int((int64(plane.X)*int64(p.X)+int64(plane.Y)*int64(p.Y)+int64(plane.Z)*int64(p.Z))/NormalUnit) - plane.Dist
}

func PointContents(hull Hull, p Vec3) int {
	node := hull.Root
	for depth := 0; node >= 0 && depth < 256; depth++ {
		if node >= len(hull.Nodes) {
			return ContentsSolid
		}
		current := hull.Nodes[node]
		if current.Plane < 0 || current.Plane >= len(hull.Planes) {
			return ContentsSolid
		}
		if planeDistance(hull.Planes[current.Plane], p) >= 0 {
			node = current.Front
		} else {
			node = current.Back
		}
	}
	if node >= 0 {
		return ContentsSolid
	}
	return node
}

func TraceHull(hull Hull, start, end Vec3) TraceResult {
	result := TraceResult{Fraction: NormalUnit, End: end, AllSolid: true, Brush: -1, Plane: -1}
	result.StartSolid = PointContents(hull, start) == ContentsSolid
	var visit func(int, int, int, Vec3, Vec3, Vec3, int, int) bool
	visit = func(index, startFraction, endFraction int, a, b, normal Vec3, depth, hitPlane int) bool {
		if depth > 256 {
			return false
		}
		if index < 0 {
			if index == ContentsSolid {
				if startFraction < result.Fraction {
					result.Fraction = startFraction
					result.End = a
					result.Normal = normal
					result.Plane = hitPlane
				}
				return false
			}
			result.AllSolid = false
			return true
		}
		if index >= len(hull.Nodes) {
			return false
		}
		node := hull.Nodes[index]
		if node.Plane < 0 || node.Plane >= len(hull.Planes) {
			return false
		}
		plane := hull.Planes[node.Plane]
		d1, d2 := planeDistance(plane, a), planeDistance(plane, b)
		if d1 >= 0 && d2 >= 0 {
			return visit(node.Front, startFraction, endFraction, a, b, normal, depth+1, hitPlane)
		}
		if d1 < 0 && d2 < 0 {
			return visit(node.Back, startFraction, endFraction, a, b, normal, depth+1, hitPlane)
		}
		near, far := node.Front, node.Back
		outward := Vec3{plane.X, plane.Y, plane.Z}
		numerator := d1 - Unit/32
		if d1 < 0 {
			near, far = node.Back, node.Front
			outward = Vec3{-plane.X, -plane.Y, -plane.Z}
			numerator = d1 + Unit/32
		}
		fraction := Clamp(int(int64(numerator)*NormalUnit/int64(d1-d2)), 0, NormalUnit)
		mid := Add(a, Scale(Sub(b, a), fraction, NormalUnit))
		middleFraction := startFraction + int(int64(endFraction-startFraction)*int64(fraction)/NormalUnit)
		if !visit(near, startFraction, middleFraction, a, mid, normal, depth+1, hitPlane) {
			return false
		}
		if PointContents(Hull{Root: far, Nodes: hull.Nodes, Planes: hull.Planes}, mid) != ContentsSolid {
			return visit(far, middleFraction, endFraction, mid, b, outward, depth+1, node.Plane)
		}
		if middleFraction < result.Fraction {
			result.Fraction = middleFraction
			result.End = mid
			result.Normal = outward
			result.Plane = node.Plane
		}
		return false
	}
	visit(hull.Root, 0, NormalUnit, start, end, Vec3{}, 0, -1)
	if result.StartSolid && result.AllSolid {
		result.Fraction = 0
		result.End = start
	}
	return result
}

func mapHull(world Map, index int) Hull {
	if len(world.Planes) == 0 {
		return Hull{Root: ContentsEmpty}
	}
	if index < 0 {
		return Hull{Root: world.Root, Nodes: world.Nodes, Planes: world.Planes}
	}
	if index >= len(world.Hulls) {
		return Hull{Root: ContentsEmpty}
	}
	hull := world.Hulls[index]
	hull.Planes = world.Planes
	return hull
}

func TraceWorld(state *State, start, end Vec3, hullIndex int) TraceResult {
	result := TraceHull(mapHull(state.World, hullIndex), start, end)
	for index, brush := range state.World.Brushes {
		if brush.Kind == "func_ladder" || brush.Kind == "func_illusionary" || brush.Kind == "water" {
			continue
		}
		open, health, progress, direction := brush.Open, brush.Health, 0, 1
		for _, dynamic := range state.Brushes {
			if dynamic.ID == brush.ID {
				open, health, progress = dynamic.Open, dynamic.Health, dynamic.Progress
				if dynamic.Direction < 0 {
					direction = -1
				}
				break
			}
		}
		if open && progress == 0 || brush.Kind == "func_breakable" && health <= 0 {
			continue
		}
		var trace TraceResult
		origin := Add(brush.Origin, Scale(brush.Move, progress, 1000))
		angle := 0
		if brush.Kind == "func_door_rotating" {
			angle = brush.Angle
			if angle == 0 {
				angle = 16384
			}
			angle = angle * progress / 1000
			angle *= direction
			if brush.Flags&2 != 0 {
				angle = -angle
			}
		}
		brushHull := hullIndex + 1
		if brushHull >= 0 && brushHull < len(brush.Hulls) {
			hull := brush.Hulls[brushHull]
			hull.Planes = state.World.Planes
			localStart, localEnd := Sub(start, origin), Sub(end, origin)
			if angle != 0 {
				localStart, localEnd = rotateZ(localStart, -angle), rotateZ(localEnd, -angle)
			}
			trace = TraceHull(hull, localStart, localEnd)
			if angle != 0 {
				trace.End, trace.Normal = rotateZ(trace.End, angle), rotateZ(trace.Normal, angle)
			}
			trace.End = Add(trace.End, origin)
		} else {
			bounds := brush.Bounds
			shift := Scale(brush.Move, progress, 1000)
			bounds.Min, bounds.Max = Add(bounds.Min, shift), Add(bounds.Max, shift)
			if hullIndex >= 0 {
				radius, height := 16*Unit, 36*Unit
				if hullIndex == 1 {
					height = 18 * Unit
				}
				bounds.Min = Sub(bounds.Min, Vec3{radius, radius, height})
				bounds.Max = Add(bounds.Max, Vec3{radius, radius, height})
			}
			trace = TraceBounds(bounds, start, end)
		}
		if trace.Fraction < result.Fraction {
			trace.Brush = index
			result = trace
		}
	}
	return result
}

func TraceBounds(bounds Bounds, start, end Vec3) TraceResult {
	result := TraceResult{Fraction: NormalUnit, End: end, Brush: -1, Plane: -1}
	if Contains(bounds, start) {
		result.Fraction = 0
		result.End = start
		result.StartSolid = true
		result.AllSolid = Contains(bounds, end)
		return result
	}
	entry, exit := 0, NormalUnit
	normal := Vec3{}
	a, b, low, high := []int{start.X, start.Y, start.Z}, []int{end.X, end.Y, end.Z}, []int{bounds.Min.X, bounds.Min.Y, bounds.Min.Z}, []int{bounds.Max.X, bounds.Max.Y, bounds.Max.Z}
	for axis := range 3 {
		delta := b[axis] - a[axis]
		if delta == 0 {
			if a[axis] < low[axis] || a[axis] > high[axis] {
				return result
			}
			continue
		}
		n1 := int(int64(low[axis]-a[axis]) * NormalUnit / int64(delta))
		n2 := int(int64(high[axis]-a[axis]) * NormalUnit / int64(delta))
		sign := -NormalUnit
		if n1 > n2 {
			n1, n2 = n2, n1
			sign = NormalUnit
		}
		if n1 > entry {
			entry = n1
			normal = Vec3{}
			switch axis {
			case 0:
				normal.X = sign
			case 1:
				normal.Y = sign
			case 2:
				normal.Z = sign
			}
		}
		exit = min(exit, n2)
		if entry > exit {
			return result
		}
	}
	if entry < 0 || entry > NormalUnit {
		return result
	}
	result.Fraction = max(0, entry-1)
	result.End = Add(start, Scale(Sub(end, start), result.Fraction, NormalUnit))
	result.Normal = normal
	return result
}

func Visible(state *State, seat, target int) bool {
	if seat < 0 || target < 0 || seat >= len(state.Players) || target >= len(state.Players) {
		return false
	}
	a, b := state.Players[seat], state.Players[target]
	if !a.Alive || !b.Alive {
		return false
	}
	start := Eye(a)
	for _, end := range []Vec3{Eye(b), Position(b), {b.X, b.Y, b.Z - 16*Unit}} {
		if TraceWorld(state, start, end, -1).Fraction == NormalUnit && !SmokeBlocks(state, start, end) {
			return true
		}
	}
	return false
}
