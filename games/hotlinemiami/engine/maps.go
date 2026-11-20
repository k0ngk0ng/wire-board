package engine

var recoveredMapIndex = func() map[string]Map {
	index := make(map[string]Map, len(recoveredMaps))
	for _, world := range recoveredMaps {
		index[world.ID] = world
	}
	return index
}()

func Maps() []Map {
	return recoveredMaps
}

func MapByID(id string) Map {
	return recoveredMapIndex[id]
}
