package engine

import "math/bits"

const smokeCoordinateLimit = 1 << 26

func SmokeRadius(grenade Grenade, frame int64) int {
	if grenade.Kind != "smokegrenade" || !grenade.Active || frame < 0 || grenade.ExpiresAt <= 0 || frame >= grenade.ExpiresAt {
		return 0
	}
	age := frame - (grenade.ExpiresAt - 18*FPS)
	if age <= 0 {
		return 0
	}
	return 180 * Unit * int(min(age, int64(FPS))) / FPS
}

func SmokeBlocks(state *State, start, end Vec3) bool {
	if state == nil {
		return false
	}
	if len(state.Grenades) > 32 {
		return true
	}
	for _, grenade := range state.Grenades {
		radius := SmokeRadius(grenade, state.Frame)
		if radius <= 0 {
			continue
		}
		center := Vec3{grenade.X, grenade.Y, grenade.Z + 70*Unit}
		if !smokeCoordinateValid(start) || !smokeCoordinateValid(end) || !smokeCoordinateValid(center) {
			return true
		}
		if smokeSegmentIntersects(start, end, center, radius) {
			return true
		}
	}
	return false
}

func smokeCoordinateValid(point Vec3) bool {
	return point.X >= -smokeCoordinateLimit && point.X <= smokeCoordinateLimit && point.Y >= -smokeCoordinateLimit && point.Y <= smokeCoordinateLimit && point.Z >= -smokeCoordinateLimit && point.Z <= smokeCoordinateLimit
}

func smokeSegmentIntersects(start, end, center Vec3, radius int) bool {
	dx, dy, dz := int64(end.X-start.X), int64(end.Y-start.Y), int64(end.Z-start.Z)
	fx, fy, fz := int64(center.X-start.X), int64(center.Y-start.Y), int64(center.Z-start.Z)
	radiusSquared := int64(radius) * int64(radius)
	lengthSquared := dx*dx + dy*dy + dz*dz
	projection := fx*dx + fy*dy + fz*dz
	if projection <= 0 || lengthSquared == 0 {
		return fx*fx+fy*fy+fz*fz <= radiusSquared
	}
	if projection >= lengthSquared {
		x, y, z := int64(center.X-end.X), int64(center.Y-end.Y), int64(center.Z-end.Z)
		return x*x+y*y+z*z <= radiusSquared
	}
	cx, cy, cz := fy*dz-fz*dy, fz*dx-fx*dz, fx*dy-fy*dx
	high, low := smokeSquare128(cx)
	for _, component := range []int64{cy, cz} {
		componentHigh, componentLow := smokeSquare128(component)
		var carry uint64
		low, carry = bits.Add64(low, componentLow, 0)
		high, _ = bits.Add64(high, componentHigh, carry)
	}
	limitHigh, limitLow := bits.Mul64(uint64(radiusSquared), uint64(lengthSquared))
	return high < limitHigh || high == limitHigh && low <= limitLow
}

func smokeSquare128(value int64) (uint64, uint64) {
	if value < 0 {
		value = -value
	}
	return bits.Mul64(uint64(value), uint64(value))
}
