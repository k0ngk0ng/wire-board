package racing

import "math"

// Tracks returns detached, immutable-in-practice road descriptions. Rendering,
// gates, off-road detection and AI all consume this same sampled geometry.
func Tracks() []Track {
	return []Track{
		makeTrack("metro", "霓虹城区", 6200, 4400, 200, []Point{{850, 650}, {2700, 450}, {5000, 650}, {5500, 1450}, {4400, 1900}, {5100, 3100}, {4200, 3750}, {2500, 3550}, {1400, 3800}, {650, 2750}, {1000, 1700}}),
		makeTrack("harbor", "工业港湾", 6600, 4700, 190, []Point{{900, 750}, {2500, 550}, {5200, 650}, {5700, 1450}, {4750, 1900}, {5500, 2800}, {5100, 3850}, {3450, 4050}, {3300, 2900}, {2200, 2650}, {1500, 3650}, {650, 2900}, {650, 1600}}),
		makeTrack("woodland", "林间山道", 5200, 3800, 174, []Point{{900, 750}, {2400, 550}, {4200, 850}, {4550, 1950}, {3700, 3100}, {1500, 3000}, {650, 2050}}),
		makeTrack("coast", "滨海公路", 5700, 4000, 186, []Point{{950, 850}, {2300, 550}, {4200, 650}, {4900, 1400}, {3750, 1850}, {4750, 2580}, {3750, 3300}, {2400, 3150}, {1400, 2650}, {650, 2050}}),
		makeTrack("canyon", "赤岩峡谷", 5500, 4000, 178, []Point{{850, 650}, {2500, 500}, {4300, 650}, {4650, 1250}, {3500, 1580}, {2900, 1200}, {2100, 1500}, {2180, 2250}, {3700, 2180}, {4600, 2600}, {4250, 3250}, {1900, 3400}, {850, 2700}, {600, 1550}}),
	}
}
func makeTrack(id, name string, w, h, r float64, knots []Point) Track {
	dense := []Point{}
	n := len(knots)
	for i, p1 := range knots {
		p0, p2, p3 := knots[(i+n-1)%n], knots[(i+1)%n], knots[(i+2)%n]
		for j := 0; j < 40; j++ {
			t := float64(j) / 40
			t2, t3 := t*t, t*t*t
			f := func(a, b, c, d float64) float64 {
				return .5 * ((2 * b) + (-a+c)*t + (2*a-5*b+4*c-d)*t2 + (-a+3*b-3*c+d)*t3)
			}
			dense = append(dense, Point{f(p0.X, p1.X, p2.X, p3.X), f(p0.Y, p1.Y, p2.Y, p3.Y)})
		}
	}
	length := 0.
	for i, p := range dense {
		length += dist(p, dense[(i+1)%len(dense)])
	}
	count := int(math.Ceil(length / 75))
	spacing := length / float64(count)
	points := make([]Point, 0, count)
	segment := 0
	along := 0.
	for i := 0; i < count; i++ {
		target := float64(i) * spacing
		for {
			a, b := dense[segment], dense[(segment+1)%len(dense)]
			l := dist(a, b)
			if along+l >= target {
				u := (target - along) / l
				points = append(points, Point{math.Round((a.X+(b.X-a.X)*u)*10) / 10, math.Round((a.Y+(b.Y-a.Y)*u)*10) / 10})
				break
			}
			along += l
			segment++
		}
	}
	t := Track{ID: id, Name: name, Width: w, Height: h, RoadWidth: r, Points: points}
	t.Props = roadside(t)
	return t
}
func trackByID(id string) (Track, bool) {
	for _, t := range Tracks() {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
}
func tangent(t Track, i int) Point {
	n := len(t.Points)
	a, b := t.Points[(i+n-1)%n], t.Points[(i+1)%n]
	d := dist(a, b)
	return Point{(b.X - a.X) / d, (b.Y - a.Y) / d}
}
func nearest(t Track, p Point) (Point, int, float64, float64) {
	best := math.Inf(1)
	out := Point{}
	index := 0
	fraction := 0.
	for i, a := range t.Points {
		b := t.Points[(i+1)%len(t.Points)]
		dx, dy := b.X-a.X, b.Y-a.Y
		f := clamp(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy), 0, 1)
		q := Point{a.X + dx*f, a.Y + dy*f}
		d := (p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y)
		if d < best {
			best = d
			out = q
			index = i
			fraction = f
		}
	}
	return out, index, math.Sqrt(best), fraction
}
func pathLengths(t Track) ([]float64, float64) {
	arcs := make([]float64, len(t.Points))
	total := 0.
	for i, p := range t.Points {
		arcs[i] = total
		total += dist(p, t.Points[(i+1)%len(t.Points)])
	}
	return arcs, total
}

// These same footprints are sent to the renderer; decorative solid structures
// never occupy road space or silently allow cars to drive through their mesh.
func roadside(t Track) []Scenery {
	out := []Scenery{}
	stride := len(t.Points) / 32
	if stride < 1 {
		stride = 1
	}
	for i := 2; i < len(t.Points); i += stride {
		p, v := t.Points[i], tangent(t, i)
		for _, side := range []float64{-1, 1} {
			kind, radius, height := "tree", 16., 65.+float64(i%9)*4
			switch t.ID {
			case "metro":
				kind, radius, height = "building", 40.+float64(i%4)*6, 120.+float64(i%7)*45
			case "harbor":
				kind, radius, height = "container", 35, 35.+float64(i%3)*22
			case "canyon":
				kind, radius, height = "rock", 28.+float64(i%5)*5, 60.+float64(i%7)*16
			case "coast":
				kind, radius, height = "palm", 14, 85
			}
			offset := t.RoadWidth/2 + radius + 65 + float64(i%5)*20
			prop := Scenery{X: math.Round(p.X - v.Y*offset*side), Y: math.Round(p.Y + v.X*offset*side), Radius: radius, Height: height, Kind: kind}
			if prop.X < radius || prop.Y < radius || prop.X > t.Width-radius || prop.Y > t.Height-radius {
				continue
			}
			_, _, distance, _ := nearest(t, Point{prop.X, prop.Y})
			if distance < t.RoadWidth/2+radius+25 {
				continue
			}
			clear := true
			for _, q := range out {
				if dist(Point{q.X, q.Y}, Point{prop.X, prop.Y}) < q.Radius+radius+15 {
					clear = false
					break
				}
			}
			if clear {
				out = append(out, prop)
			}
		}
	}
	return out
}
