package engine

import "strconv"

func routeFor(id int) (RouteSpec, bool) {
	catalog := CatalogData()
	if catalog == nil {
		return RouteSpec{}, false
	}
	route, ok := catalog.Routes[strconv.Itoa(id)]
	return route, ok
}

func waveFor(route RouteSpec, number int) (WaveSpec, bool) {
	if number < 1 {
		return WaveSpec{}, false
	}
	wave, ok := route.Waves[strconv.Itoa((number-1)%100+1)]
	if !ok {
		wave, ok = route.Waves[strconv.Itoa(number)]
	}
	return wave, ok
}
