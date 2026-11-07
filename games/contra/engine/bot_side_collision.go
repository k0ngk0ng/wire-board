package contra

// Match the known body-side envelope against visible terrain only. The
// original samples X±8 and scans across tile rows; a platform backed by solid
// is also a side barrier. Unknown artwork is not converted into hidden walls.
func botVisibleAirSideBlocked(o BotObservation, x, y, direction int) bool {
	top := 10
	if y >= 27 {
		top = y - 11
	}
	height := 21
	if o.terrainAt(x, y+16) == TerrainEmpty {
		height = 27
	}
	probeX := x + direction*8
	distance := 16 - ((top + int(o.ViewY)) & 15)
	probeY := top
	for {
		kind := o.terrainAt(probeX, probeY)
		if kind == TerrainSolid || kind == TerrainPlatform && o.terrainAt(probeX, probeY+16) == TerrainSolid {
			return true
		}
		if distance >= height {
			return false
		}
		probeY = top + distance
		distance += 16
	}
}
