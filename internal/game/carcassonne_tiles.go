package game

// Ports run clockwise: NW,N,NE / EN,E,ES / SE,S,SW / WS,W,WN.
// Roads and cities join at edge centers. Fields join at the two flanks
// (and the center of meadow edges), so walls and roads truly partition farms.
type CarFeature struct {
	Kind    string  `json:"kind"`
	Ports   []int   `json:"ports"`
	Cities  []int   `json:"cities,omitempty"`
	Shields int     `json:"shields,omitempty"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}
type CarDefinition struct {
	Name     string       `json:"name"`
	Arts     []int        `json:"arts"`
	Features []CarFeature `json:"features"`
}

func carFeature(kind string, x, y float64, ports ...int) CarFeature {
	return CarFeature{Kind: kind, Ports: ports, X: x, Y: y}
}
func carCity(shield int, x, y float64, ports ...int) CarFeature {
	f := carFeature("city", x, y, ports...)
	f.Shields = shield
	return f
}
func carField(x, y float64, cities []int, ports ...int) CarFeature {
	f := carFeature("field", x, y, ports...)
	f.Cities = cities
	return f
}

var carDefinitions = []CarDefinition{
	{"A", []int{65, 66}, []CarFeature{carFeature("monastery", .5, .42), carFeature("road", .55, .84, 7), carField(.22, .45, nil, 0, 1, 2, 3, 4, 5, 6, 8, 9, 10, 11)}},
	{"B", []int{61, 62, 63, 64}, []CarFeature{carFeature("monastery", .5, .48), carField(.2, .25, nil, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)}},
	{"C", []int{67}, []CarFeature{carCity(1, .5, .5, 1, 4, 7, 10)}},
	{"D", []int{35, 36, 37, 38}, []CarFeature{carCity(0, .5, .15, 1), carFeature("road", .5, .58, 4, 10), carField(.5, .36, []int{0}, 3, 11), carField(.5, .83, nil, 5, 6, 7, 8, 9)}},
	{"E", []int{21, 22, 23, 24, 25}, []CarFeature{carCity(0, .5, .15, 1), carField(.5, .65, []int{0}, 3, 4, 5, 6, 7, 8, 9, 10, 11)}},
	{"F", []int{14, 15}, []CarFeature{carCity(1, .5, .5, 4, 10), carField(.5, .1, []int{0}, 0, 1, 2), carField(.5, .87, []int{0}, 6, 7, 8)}},
	{"G", []int{13}, []CarFeature{carCity(0, .5, .5, 4, 10), carField(.5, .1, []int{0}, 0, 1, 2), carField(.5, .87, []int{0}, 6, 7, 8)}},
	{"H", []int{18, 19, 20}, []CarFeature{carCity(0, .5, .12, 1), carCity(0, .5, .88, 7), carField(.5, .5, []int{0, 1}, 3, 4, 5, 9, 10, 11)}},
	{"I", []int{16, 17}, []CarFeature{carCity(0, .5, .12, 1), carCity(0, .12, .5, 10), carField(.65, .65, []int{0, 1}, 3, 4, 5, 6, 7, 8)}},
	{"J", []int{29, 30, 31}, []CarFeature{carCity(0, .5, .13, 1), carFeature("road", .53, .73, 4, 7), carField(.28, .57, []int{0}, 3, 8, 9, 10, 11), carField(.8, .83, nil, 5, 6)}},
	{"K", []int{26, 27, 28}, []CarFeature{carCity(0, .5, .13, 1), carFeature("road", .54, .74, 7, 10), carField(.75, .54, []int{0}, 3, 4, 5, 6, 11), carField(.22, .84, nil, 8, 9)}},
	{"L", []int{32, 33, 34}, []CarFeature{carCity(0, .5, .12, 1), carFeature("road", .88, .53, 4), carFeature("road", .5, .86, 7), carFeature("road", .12, .53, 10), carField(.5, .33, []int{0}, 3, 11), carField(.8, .8, nil, 5, 6), carField(.2, .8, nil, 8, 9)}},
	{"M", []int{6, 7}, []CarFeature{carCity(1, .3, .3, 1, 10), carField(.75, .72, []int{0}, 3, 4, 5, 6, 7, 8)}},
	{"N", []int{3, 4, 5}, []CarFeature{carCity(0, .3, .3, 1, 10), carField(.75, .72, []int{0}, 3, 4, 5, 6, 7, 8)}},
	{"O", []int{11, 12}, []CarFeature{carCity(1, .3, .3, 1, 10), carFeature("road", .56, .73, 4, 7), carField(.37, .72, []int{0}, 3, 8), carField(.84, .84, nil, 5, 6)}},
	{"P", []int{8, 9, 10}, []CarFeature{carCity(0, .3, .3, 1, 10), carFeature("road", .56, .73, 4, 7), carField(.37, .72, []int{0}, 3, 8), carField(.84, .84, nil, 5, 6)}},
	{"Q", []int{71}, []CarFeature{carCity(1, .5, .3, 1, 4, 10), carField(.5, .88, []int{0}, 6, 7, 8)}},
	{"R", []int{68, 69, 70}, []CarFeature{carCity(0, .5, .3, 1, 4, 10), carField(.5, .88, []int{0}, 6, 7, 8)}},
	{"S", []int{1, 2}, []CarFeature{carCity(1, .5, .3, 1, 4, 10), carFeature("road", .5, .87, 7), carField(.75, .9, []int{0}, 6), carField(.25, .9, []int{0}, 8)}},
	{"T", []int{0}, []CarFeature{carCity(0, .5, .3, 1, 4, 10), carFeature("road", .5, .87, 7), carField(.75, .9, []int{0}, 6), carField(.25, .9, []int{0}, 8)}},
	{"U", []int{39, 40, 41, 42, 43, 44, 45, 46}, []CarFeature{carFeature("road", .46, .5, 1, 7), carField(.8, .5, nil, 2, 3, 4, 5, 6), carField(.18, .5, nil, 8, 9, 10, 11, 0)}},
	{"V", []int{47, 48, 49, 50, 51, 52, 53, 54, 55}, []CarFeature{carFeature("road", .48, .67, 7, 10), carField(.73, .3, nil, 0, 1, 2, 3, 4, 5, 6, 11), carField(.2, .85, nil, 8, 9)}},
	{"W", []int{56, 57, 58, 59}, []CarFeature{carFeature("road", .86, .52, 4), carFeature("road", .5, .86, 7), carFeature("road", .13, .52, 10), carField(.5, .25, nil, 0, 1, 2, 3, 11), carField(.8, .82, nil, 5, 6), carField(.2, .82, nil, 8, 9)}},
	{"X", []int{60}, []CarFeature{carFeature("road", .5, .14, 1), carFeature("road", .86, .52, 4), carFeature("road", .5, .86, 7), carFeature("road", .13, .52, 10), carField(.2, .2, nil, 0, 11), carField(.8, .2, nil, 2, 3), carField(.8, .82, nil, 5, 6), carField(.2, .82, nil, 8, 9)}},
}
var carNames = map[string]string{"road": "道路", "city": "城市", "field": "田地", "monastery": "修道院"}

func carKind(art int) int {
	for i, d := range carDefinitions {
		for _, a := range d.Arts {
			if a == art {
				return i
			}
		}
	}
	panic("invalid Carcassonne tile")
}
func carPortFeature(t CarTile, port int) int {
	port = (port - t.Rotation*3 + 12) % 12
	for i, f := range carDefinitions[t.Kind].Features {
		for _, p := range f.Ports {
			if p == port {
				return i
			}
		}
	}
	return -1
}
