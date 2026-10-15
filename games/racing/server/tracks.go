package racing

import "math"

// Tracks returns detached, immutable-in-practice road descriptions. Rendering,
// gates, off-road detection and AI all consume this same sampled geometry.
func Tracks() []Track {
	return []Track{
		makeTrack("woodland", "林间环道", 2400, 1700, 170, []Point{{420, 350}, {1100, 250}, {1900, 400}, {2100, 900}, {1700, 1400}, {700, 1350}, {300, 950}}),
		makeTrack("coast", "海岸曲线", 2600, 1800, 154, []Point{{450, 400}, {1050, 250}, {1900, 300}, {2250, 650}, {1700, 850}, {2170, 1180}, {1700, 1500}, {1100, 1430}, {650, 1200}, {300, 950}}),
		makeTrack("canyon", "赤岩回廊", 2800, 2000, 158, []Point{{450, 350}, {1300, 250}, {2200, 350}, {2380, 650}, {1800, 810}, {1500, 620}, {1100, 760}, {1130, 1150}, {1900, 1120}, {2350, 1340}, {2180, 1660}, {1000, 1720}, {430, 1380}, {300, 800}}),
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
	count := int(math.Ceil(length / 45))
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
				points = append(points, Point{a.X + (b.X-a.X)*u, a.Y + (b.Y-a.Y)*u})
				break
			}
			along += l
			segment++
		}
	}
	return Track{ID: id, Name: name, Width: w, Height: h, RoadWidth: r, Points: points}
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
