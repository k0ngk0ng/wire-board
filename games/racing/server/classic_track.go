package racing

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

// Numeric server rule data only. Client models, textures and audio are excluded.
//
//go:embed data/index.json data/car-bodies.json data/rules/*.json
var classicRuleFiles embed.FS

type RoadSample struct {
	Height  float64 `json:"height"`
	Left    float64 `json:"left"`
	Right   float64 `json:"right"`
	NX      float64 `json:"nx"`
	NY      float64 `json:"ny"`
	NZ      float64 `json:"nz"`
	FX      float64 `json:"fx"`
	FY      float64 `json:"fy"`
	FZ      float64 `json:"fz"`
	RX      float64 `json:"rx"`
	RY      float64 `json:"ry"`
	RZ      float64 `json:"rz"`
	Surface int     `json:"surface"`
	Segment int     `json:"segment"`
}
type RoadBarrier struct {
	Kind    string  `json:"kind"`
	A       Point   `json:"a"`
	B       Point   `json:"b"`
	Bottom  float64 `json:"bottom"`
	Top     float64 `json:"top"`
	Segment int     `json:"segment"`
}
type ClassicRoadContact struct {
	Index    int     `json:"index"`
	Fraction float64 `json:"fraction"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Height   float64 `json:"height"`
	Side     float64 `json:"side"`
	Left     float64 `json:"left"`
	Right    float64 `json:"right"`
	NX       float64 `json:"nx"`
	NY       float64 `json:"ny"`
	NZ       float64 `json:"nz"`
	RX       float64 `json:"rx"`
	RY       float64 `json:"ry"`
	RZ       float64 `json:"rz"`
	Surface  int     `json:"surface"`
	Segment  int     `json:"segment"`
}
type ClassicBody struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
	Height float64 `json:"height"`
}
type classicRuleIndexEntry struct {
	ID         string `json:"id"`
	Geometry   string `json:"geometry"`
	RuleSHA256 string `json:"ruleSha256"`
}
type classicTrackInfo struct {
	track  Track
	arcs   []float64
	length float64
	grid   map[[2]int][]int
}
type classicRuleRegistry struct {
	current    []string
	byID       map[string]*classicTrackInfo
	byGeometry map[string]*classicTrackInfo
	bodies     map[string]ClassicBody
}

var classicRules = loadClassicRules()

func loadClassicRules() classicRuleRegistry {
	result := classicRuleRegistry{byID: map[string]*classicTrackInfo{}, byGeometry: map[string]*classicTrackInfo{}, bodies: map[string]ClassicBody{}}
	bodyBytes, err := classicRuleFiles.ReadFile("data/car-bodies.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(bodyBytes, &result.bodies); err != nil {
		panic(err)
	}
	indexBytes, err := classicRuleFiles.ReadFile("data/index.json")
	if err != nil {
		panic(err)
	}
	var index []classicRuleIndexEntry
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		panic(err)
	}
	// Retained hash-named versions remain available to old v3 saved matches.
	entries, err := classicRuleFiles.ReadDir("data/rules")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		payload, err := classicRuleFiles.ReadFile("data/rules/" + entry.Name())
		if err != nil {
			panic(err)
		}
		var track Track
		if err := json.Unmarshal(payload, &track); err != nil {
			panic(err)
		}
		if track.Version != 3 || len(track.Geometry) != 64 || len(track.Points) < 300 || len(track.Road) != len(track.Points) {
			panic("invalid classic road data " + entry.Name())
		}
		result.byGeometry[track.Geometry] = buildClassicTrackInfo(track)
	}
	for _, entry := range index {
		info := result.byGeometry[entry.Geometry]
		if info == nil || info.track.ID != entry.ID {
			panic("classic road index does not match immutable rules")
		}
		payload, _ := classicRuleFiles.ReadFile("data/rules/" + entry.Geometry + ".json")
		digest := sha256.Sum256(payload)
		if hex.EncodeToString(digest[:]) != entry.RuleSHA256 {
			panic("classic rule data digest mismatch " + entry.ID)
		}
		result.byID[entry.ID] = info
		result.current = append(result.current, entry.ID)
	}
	return result
}
func buildClassicTrackInfo(track Track) *classicTrackInfo {
	info := &classicTrackInfo{track: track, arcs: make([]float64, len(track.Points)), grid: map[[2]int][]int{}}
	for i, p := range track.Points {
		info.arcs[i] = info.length
		j := (i + 1) % len(track.Points)
		dx, dy, dh := track.Points[j].X-p.X, track.Points[j].Y-p.Y, track.Road[j].Height-track.Road[i].Height
		info.length += math.Sqrt(dx*dx + dy*dy + dh*dh)
	}
	for i, b := range track.Barriers {
		left, right := int(math.Floor((math.Min(b.A.X, b.B.X)-30)/100)), int(math.Floor((math.Max(b.A.X, b.B.X)+30)/100))
		top, bottom := int(math.Floor((math.Min(b.A.Y, b.B.Y)-30)/100)), int(math.Floor((math.Max(b.A.Y, b.B.Y)+30)/100))
		for x := left; x <= right; x++ {
			for y := top; y <= bottom; y++ {
				key := [2]int{x, y}
				info.grid[key] = append(info.grid[key], i)
			}
		}
	}
	return info
}
func classicTrackInfoFor(t Track) *classicTrackInfo { return classicRules.byGeometry[t.Geometry] }
func classicTrackByID(id string) (Track, bool) {
	info := classicRules.byID[id]
	if info == nil {
		return Track{}, false
	}
	return info.track, true
}
func classicTrackByGeometry(hash string) (Track, bool) {
	info := classicRules.byGeometry[hash]
	if info == nil {
		return Track{}, false
	}
	return info.track, true
}
func compactTrack(t Track) Track {
	t.Points = nil
	t.Road = nil
	t.Barriers = nil
	t.Props = nil
	return t
}

// Tracks includes the delivered original routes and explicit legacy-ID routes.
// Legacy IDs are compatibility branches, not additional delivered v3 courses.
func Tracks() []Track {
	result := make([]Track, 0, len(classicRules.current)+5)
	for _, id := range classicRules.current {
		t := classicRules.byID[id].track
		t.Points = append([]Point{}, t.Points...)
		t.Road = append([]RoadSample{}, t.Road...)
		t.Barriers = append([]RoadBarrier{}, t.Barriers...)
		result = append(result, t)
	}
	return append(result, legacyTracks()...)
}
func ClassicVehicleBody(id string) ClassicBody {
	body, ok := classicRules.bodies[id]
	if !ok {
		return classicRules.bodies["p911"]
	}
	return body
}

func ClassicRoadAt(t Track, x, y, height float64, hint int) ClassicRoadContact {
	n := len(t.Points)
	if n == 0 || len(t.Road) != n {
		return ClassicRoadContact{NY: 1, Left: t.RoadWidth / 2, Right: t.RoadWidth / 2}
	}
	best := math.Inf(1)
	bestHorizontal := math.Inf(1)
	out := ClassicRoadContact{}
	consider := func(i int) {
		j := (i + 1) % n
		a, b := t.Points[i], t.Points[j]
		dx, dy := b.X-a.X, b.Y-a.Y
		length := dx*dx + dy*dy
		f := 0.
		if length > 1e-12 {
			f = clamp(((x-a.X)*dx+(y-a.Y)*dy)/length, 0, 1)
		}
		qx, qy := a.X+dx*f, a.Y+dy*f
		r, rr := t.Road[i], t.Road[j]
		lerp := func(a, b float64) float64 { return a + (b-a)*f }
		nx, ny, nz := lerp(r.NX, rr.NX), lerp(r.NY, rr.NY), lerp(r.NZ, rr.NZ)
		if math.Abs(ny) < .1 {
			ny = 1
		}
		h := lerp(r.Height, rr.Height) - (nx*(x-qx)+nz*(y-qy))/ny
		horizontal := (x-qx)*(x-qx) + (y-qy)*(y-qy)
		vertical := height - h
		score := horizontal + vertical*vertical*4
		if score >= best {
			return
		}
		best = score
		bestHorizontal = horizontal
		rx, ry, rz := lerp(r.RX, rr.RX), lerp(r.RY, rr.RY), lerp(r.RZ, rr.RZ)
		rl := math.Hypot(rx, rz)
		if rl < 1e-8 {
			rx, rz = -dy, dx
			rl = math.Hypot(rx, rz)
		}
		side := 0.
		if rl > 1e-8 {
			side = ((x-qx)*rx + (y-qy)*rz) / rl
		}
		out = ClassicRoadContact{Index: i, Fraction: f, X: qx, Y: qy, Height: h, Side: side, Left: lerp(r.Left, rr.Left), Right: lerp(r.Right, rr.Right), NX: nx, NY: ny, NZ: nz, RX: rx, RY: ry, RZ: rz, Surface: r.Surface, Segment: r.Segment}
	}
	if hint >= 0 && hint < n {
		for k := -40; k <= 40; k++ {
			consider((hint + k + n) % n)
		}
	} else {
		for i := 0; i < n; i++ {
			consider(i)
		}
	}
	if bestHorizontal > 1200*1200 {
		for i := 0; i < n; i++ {
			consider(i)
		}
	}
	return out
}

func ClassicSurfaceFor(road ClassicRoadContact, heading float64) ClassicRoadSurface {
	excess := math.Max(0, math.Max(road.Side-road.Right, -road.Side-road.Left))
	shoulder := clamp(excess/30, 0, 1)
	ny := road.NY
	if math.Abs(ny) < .1 {
		ny = 1
	}
	grade := math.Atan(-(road.NX*math.Cos(heading) + road.NZ*math.Sin(heading)) / ny)
	return ClassicRoadSurface{Grip: 1 - .6*shoulder, Resistance: 5 * shoulder, NormalY: math.Abs(ny), Grade: grade}
}

func classicSegmentClosest(a, b, c, d Point) (Point, Point) {
	ux, uy, vx, vy, wx, wy := b.X-a.X, b.Y-a.Y, d.X-c.X, d.Y-c.Y, a.X-c.X, a.Y-c.Y
	aa, bb, ab, aw, bw := ux*ux+uy*uy, vx*vx+vy*vy, ux*vx+uy*vy, ux*wx+uy*wy, vx*wx+vy*wy
	s, q := 0., 0.
	if aa < 1e-10 && bb < 1e-10 {
		return a, c
	}
	if aa < 1e-10 {
		q = clamp(bw/bb, 0, 1)
	} else if bb < 1e-10 {
		s = clamp(-aw/aa, 0, 1)
	} else {
		den := aa*bb - ab*ab
		if den > 1e-10 {
			s = clamp((ab*bw-aw*bb)/den, 0, 1)
		}
		q = (ab*s + bw) / bb
		if q < 0 {
			q = 0
			s = clamp(-aw/aa, 0, 1)
		} else if q > 1 {
			q = 1
			s = clamp((ab-aw)/aa, 0, 1)
		}
	}
	return Point{a.X + ux*s, a.Y + uy*s}, Point{c.X + vx*q, c.Y + vy*q}
}
func classicCapsule(x, y, heading float64, body ClassicBody) (Point, Point, float64) {
	radius := body.Width * 5
	half := (body.Length - body.Width) * 5
	dx, dy := math.Cos(heading)*half, math.Sin(heading)*half
	return Point{x - dx, y - dy}, Point{x + dx, y + dy}, radius
}

func CollideClassicRoad(t Track, s ClassicMotionState, vehicle string, height float64) ClassicMotionState {
	info := classicTrackInfoFor(t)
	if info == nil {
		return s
	}
	body := ClassicVehicleBody(vehicle)
	x, y := s.X*10, s.Z*10
	for pass := 0; pass < 2; pass++ {
		candidates := info.grid[[2]int{int(math.Floor(x / 100)), int(math.Floor(y / 100))}]
		for _, idx := range candidates {
			wall := t.Barriers[idx]
			if height+body.Height*10 < wall.Bottom+.05 || height > wall.Top-.05 {
				continue
			}
			a, b, r := classicCapsule(x, y, s.Angle, body)
			p, q := classicSegmentClosest(a, b, wall.A, wall.B)
			dx, dy := p.X-q.X, p.Y-q.Y
			distance := math.Hypot(dx, dy)
			if distance >= r {
				continue
			}
			nx, ny := dx, dy
			if distance > 1e-7 {
				nx /= distance
				ny /= distance
			} else {
				nx, ny = -(wall.B.Y - wall.A.Y), wall.B.X-wall.A.X
				l := math.Hypot(nx, ny)
				if l < 1e-8 {
					nx, ny = -math.Cos(s.Angle), -math.Sin(s.Angle)
				} else {
					nx /= l
					ny /= l
				}
				if (x-(wall.A.X+wall.B.X)*.5)*nx+(y-(wall.A.Y+wall.B.Y)*.5)*ny < 0 {
					nx = -nx
					ny = -ny
				}
			}
			depth := r - distance + .001
			x += nx * depth
			y += ny * depth
			inward := s.VX*nx + s.VZ*ny
			if inward < 0 {
				s.VX -= inward * nx * 1.1
				s.VZ -= inward * ny * 1.1
			}
		}
	}
	s.X = x / 10
	s.Z = y / 10
	return s
}

func classicPose(c *Car, t Track) ClassicRoadContact {
	road := ClassicRoadAt(t, c.X, c.Y, c.Height, c.RouteIndex)
	c.RouteIndex = road.Index
	c.Height = road.Height
	c.Pitch = math.Atan(-(road.NX*math.Cos(c.Angle) + road.NZ*math.Sin(c.Angle)) / math.Max(.1, road.NY))
	c.Roll = math.Atan(-(road.NX*math.Sin(c.Angle) - road.NZ*math.Cos(c.Angle)) / math.Max(.1, road.NY))
	c.Offroad = road.Side > road.Right || road.Side < -road.Left
	return road
}

func validClassicVehicleID(id string) bool {
	switch id {
	case "p911", "fm50", "ldia", "mcf1", "ccor":
		return true
	}
	return false
}

func classicGeometryError(id string) error { return fmt.Errorf("赛车赛道版本不可用：%s", id) }
