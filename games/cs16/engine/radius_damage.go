package engine

import "math/bits"

func blastOrigin(state *State, point Vec3) Vec3 {
	start, end := point, point
	start.Z += 8 * Unit
	end.Z -= 32 * Unit
	trace := TraceWorld(state, start, end, -1)
	if trace.Fraction < NormalUnit {
		return Add(trace.End, Scale(trace.Normal, 76*Unit*3/5, NormalUnit))
	}
	return point
}

func blastWaterAllowed(state *State, point Vec3, waterLevel int) bool {
	underwater := PointContents(mapHull(state.World, -1), point) == ContentsWater
	return underwater && waterLevel > 0 || !underwater && waterLevel < 3
}

func blastLengthQ32(squared uint64) uint64 {
	if squared == 0 {
		return 0
	}
	high, low := bits.Mul64(squared, 1<<48)
	length := bits.Len64(low)
	if high > 0 {
		length = 64 + bits.Len64(high)
	}
	root := uint64(1) << ((length + 1) / 2)
	for {
		quotient, _ := bits.Div64(high, low, root)
		next := (root + quotient) / 2
		if next >= root {
			return root
		}
		root = next
	}
}

func blastFloat32Q32(value uint64) uint64 {
	shift := bits.Len64(value) - 24
	if shift <= 0 {
		return value
	}
	base := value >> shift
	remainder := value & (uint64(1)<<shift - 1)
	half := uint64(1) << (shift - 1)
	if remainder > half || remainder == half && base&1 != 0 {
		base++
	}
	return base << shift
}

func blastDamageUnits(source, target Vec3, damage, radius int) int64 {
	dx, dy, dz := source.X-target.X, source.Y-target.Y, source.Z-target.Z
	if Abs(dx) > radius || Abs(dy) > radius || Abs(dz) > radius || damage <= 0 || radius <= 0 {
		return 0
	}
	squared := uint64(int64(dx)*int64(dx) + int64(dy)*int64(dy) + int64(dz)*int64(dz))
	distance := blastLengthQ32(squared)
	if distance >= uint64(radius/Unit)<<32 {
		return 0
	}
	high, low := bits.Mul64(distance, 9586981)
	loss := high<<39 | low>>25
	base := uint64(damage) << 32
	if loss >= base {
		return 0
	}
	return int64(blastFloat32Q32(base-loss) * uint64(damageUnit) >> 32)
}
