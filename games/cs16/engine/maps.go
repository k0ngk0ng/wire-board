package engine

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/cs16/data"
	"sync"
)

var originalMaps []Map
var mapsOnce sync.Once

func loadMaps() []Map {
	var maps []Map
	if err := json.Unmarshal([]byte(data.MapsJSON), &maps); err != nil {
		panic(err)
	}
	for i := range maps {
		for h := range maps[i].Hulls {
			maps[i].Hulls[h].Planes = maps[i].Planes
		}
		for b := range maps[i].Brushes {
			for h := range maps[i].Brushes[b].Hulls {
				maps[i].Brushes[b].Hulls[h].Planes = maps[i].Planes
			}
		}
		prepareNavigation(&maps[i])
		maps[i].SurfaceIndex = make(map[int][]int)
		for face, surface := range maps[i].Surfaces {
			maps[i].SurfaceIndex[surface.Plane] = append(maps[i].SurfaceIndex[surface.Plane], face)
		}
	}
	return maps
}

func Maps() []Map {
	mapsOnce.Do(func() { originalMaps = loadMaps() })
	return append([]Map(nil), originalMaps...)
}
func MapByID(id string) Map {
	mapsOnce.Do(func() { originalMaps = loadMaps() })
	for _, world := range originalMaps {
		if world.ID == id {
			return world
		}
	}
	return Map{}
}

func Characters(team string) []string {
	if team == "ct" {
		return []string{"urban", "gsg9", "sas", "gign"}
	}
	return []string{"terror", "leet", "arctic", "guerilla"}
}
