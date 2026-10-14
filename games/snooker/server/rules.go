package snooker

import (
	"errors"
	"math"
	"slices"
	"sort"
)

type RuleContext struct {
	Balls    []Ball `json:"balls"`
	Target   string `json:"target"`
	Clear    int    `json:"clear"`
	FreeBall bool   `json:"freeBall"`
	InHand   bool   `json:"inHand"`
	Scores   [2]int `json:"scores"`
	Team     int    `json:"team"`
}
type Ruling struct {
	Points   int      `json:"points"`
	Foul     int      `json:"foul"`
	Reasons  []string `json:"reasons"`
	Respot   []int    `json:"respot"`
	Target   string   `json:"target"`
	Clear    int      `json:"clear"`
	KeepTurn bool     `json:"keepTurn"`
	InHand   bool     `json:"inHand"`
	Miss     bool     `json:"miss"`
	FullBall bool     `json:"fullBall"`
}

func findBall(bs []Ball, id int) (Ball, bool) {
	for _, b := range bs {
		if b.ID == id {
			return b, true
		}
	}
	return Ball{}, false
}
func redsLeft(bs []Ball) int {
	n := 0
	for _, b := range bs {
		if b.ID >= 1 && b.ID <= 15 && !b.Down {
			n++
		}
	}
	return n
}
func Remaining(c RuleContext) int {
	n := redsLeft(c.Balls)
	if c.Target == "red" {
		return n*8 + 27
	}
	if c.Target == "colour" {
		return n*8 + 34
	}
	total := 0
	for v := max(2, c.Clear); v <= 7; v++ {
		total += v
	}
	return total
}
func onBalls(c RuleContext, nominate int) []int {
	ids := []int{}
	for _, b := range c.Balls {
		if b.Down || b.ID == 0 {
			continue
		}
		v := BallValue(b.ID)
		if c.Target == "red" && v == 1 || c.Target == "colour" && v >= 2 && (nominate == 0 || v == nominate) || c.Target == "clear" && v == c.Clear {
			ids = append(ids, b.ID)
		}
	}
	return ids
}
func lineOpen(origin Vec, target Ball, angle, distance float64, bs []Ball, ignore []int) bool {
	dir := Vec{math.Cos(angle), math.Sin(angle)}
	for _, b := range bs {
		if b.Down || b.ID == 0 || b.ID == target.ID || slices.Contains(ignore, b.ID) {
			continue
		}
		if t, ok := circleTime(origin.sub(b.Pos), dir, 2*BallRadius, distance); ok && t < distance-1e-7 {
			return false
		}
	}
	return true
}

// Snookers are made by balls not on, and both extreme edges of at least one
// ball on must be available. Cushion restrictions are not called a snooker.
func IsSnookered(bs []Ball, on []int) bool {
	cue, ok := findBall(bs, 0)
	if !ok || cue.Down || len(on) == 0 {
		return false
	}
	for _, id := range on {
		ball, ok := findBall(bs, id)
		if !ok || ball.Down {
			continue
		}
		delta := ball.Pos.sub(cue.Pos)
		d := delta.length()
		if d <= 2*BallRadius+1e-6 {
			return false
		}
		angle := math.Atan2(delta.Y, delta.X)
		spread := math.Asin(math.Min(1, 2*BallRadius/d))
		distance := math.Sqrt(d*d - 4*BallRadius*BallRadius)
		if lineOpen(cue.Pos, ball, angle-spread, distance, bs, on) && lineOpen(cue.Pos, ball, angle+spread, distance, bs, on) {
			return false
		}
	}
	return true
}
func directContact(bs []Ball, on []int) (any, full bool) {
	cue, ok := findBall(bs, 0)
	if !ok || cue.Down {
		return
	}
	for _, id := range on {
		ball, ok := findBall(bs, id)
		if !ok || ball.Down {
			continue
		}
		d := ball.Pos.sub(cue.Pos)
		length := d.length()
		if length <= 2*BallRadius+1e-6 {
			return true, true
		}
		center := math.Atan2(d.Y, d.X)
		spread := math.Asin(2 * BallRadius / length)
		if lineOpen(cue.Pos, ball, center, length-2*BallRadius, bs, nil) {
			full = true
			any = true
		}
		// Testing interval boundaries and their midpoints covers every visibility
		// interval, including narrow openings that fixed angular samples would miss.
		angles := []float64{-spread, spread, 0}
		for _, b := range bs {
			if b.Down || b.ID == 0 || b.ID == id {
				continue
			}
			v := b.Pos.sub(cue.Pos)
			dist := v.length()
			if dist <= 2*BallRadius {
				continue
			}
			phi := math.Remainder(math.Atan2(v.Y, v.X)-center, 2*math.Pi)
			arc := math.Asin(2 * BallRadius / dist)
			for _, a := range []float64{phi - arc, phi + arc} {
				if a > -spread && a < spread {
					angles = append(angles, a)
				}
			}
		}
		sort.Float64s(angles)
		candidates := append([]float64{}, angles...)
		for i := 1; i < len(angles); i++ {
			candidates = append(candidates, (angles[i]+angles[i-1])/2)
		}
		for _, a := range candidates {
			dir := Vec{math.Cos(center + a), math.Sin(center + a)}
			if t, hit := circleTime(cue.Pos.sub(ball.Pos), dir, 2*BallRadius, TableWidth*2); hit && lineOpen(cue.Pos, ball, center+a, t, bs, nil) {
				any = true
			}
		}
	}
	return
}
func Judge(c RuleContext, shot Strike, m Motion) Ruling {
	if c.Target == "colour" && shot.Nominate == 0 {
		// Resolve the implicitly chosen colour from authoritative contacts, never
		// a UI default. On a wrong first contact, a potted colour still determines
		// the applicable penalty. No contact/pot retains all available colours.
		for _, id := range m.First {
			if BallValue(id) >= 2 {
				shot.Nominate = BallValue(id)
				break
			}
		}
		if shot.Nominate == 0 {
			for _, id := range m.Potted {
				shot.Nominate = max(shot.Nominate, BallValue(id))
			}
			if shot.Nominate < 2 {
				shot.Nominate = 0
			}
		}
	}
	r := Ruling{Target: c.Target, Clear: c.Clear, Reasons: []string{}, Respot: []int{}}
	actual := onBalls(c, shot.Nominate)
	allowed := append([]int{}, actual...)
	value := 1
	if c.Target == "colour" {
		value = max(2, shot.Nominate)
	} else if c.Target == "clear" {
		value = c.Clear
	}
	foul := func(v int, why string) {
		r.Foul = max(r.Foul, 4, value, v)
		if !slices.Contains(r.Reasons, why) {
			r.Reasons = append(r.Reasons, why)
		}
	}
	free := shot.FreeBall > 0 && c.FreeBall
	if shot.FreeBall > 0 {
		b, exists := findBall(c.Balls, shot.FreeBall)
		if !free || !exists || b.Down || slices.Contains(actual, b.ID) {
			foul(BallValue(shot.FreeBall), "无效自由球提名")
			free = false
		} else {
			allowed = []int{shot.FreeBall}
		}
	}
	if len(actual) == 0 {
		foul(4, "没有提名合法目标球")
	}
	cue, _ := findBall(c.Balls, 0)
	touchingOn := false
	for _, id := range allowed {
		b, ok := findBall(c.Balls, id)
		if ok && b.Pos.sub(cue.Pos).length() <= 2*BallRadius+1e-6 {
			after, _ := findBall(m.Balls, id)
			toward := Vec{math.Cos(shot.Angle), math.Sin(shot.Angle)}.dot(b.Pos.sub(cue.Pos)) > 1e-9
			immediate := false
			for _, event := range m.Events {
				if event.Kind == "ball" && event.Time <= .001 && ((event.Ball == 0 && event.Other == id) || (event.Ball == id && event.Other == 0)) {
					immediate = true
				}
			}
			if immediate || (toward && (after.Pos.sub(b.Pos).length() > .00002 || after.Down)) {
				foul(BallValue(id), "推移贴住的目标球")
			} else {
				// Later movement after a cushion or another ball is not a push.
				touchingOn = true
			}
		}
	}
	contactWrong := len(m.First) == 0 && !touchingOn
	if contactWrong {
		foul(0, "未先击中目标球")
	}
	if len(m.First) > 0 && !(touchingOn && r.Foul == 0) {
		hasFree := free && slices.Contains(m.First, shot.FreeBall)
		for _, id := range m.First {
			valid := slices.Contains(allowed, id) || (hasFree && slices.Contains(actual, id))
			if !valid {
				foul(BallValue(id), "先击中非目标球")
				contactWrong = true
			}
		}
		if len(m.First) > 1 && !hasFree && c.Target != "red" {
			foul(0, "同时先碰多个不同目标球")
			contactWrong = true
		}
	}
	scored := 0
	actualPots := 0
	for _, id := range m.Potted {
		if id == 0 {
			r.InHand = true
			foul(0, "白球落袋")
			continue
		}
		if slices.Contains(actual, id) {
			scored += value
			actualPots++
		} else if free && id == shot.FreeBall {
			scored += value
		} else {
			foul(BallValue(id), "非目标球落袋")
		}
	}
	for _, id := range m.Off {
		foul(BallValue(id), "球离开台面")
		if id == 0 {
			r.InHand = true
		}
	}
	if free && c.Target != "red" && actualPots > 0 {
		scored = value
	}
	// Section 3 Rule 12 refers to the effective snookering ball, even when
	// another, farther colour would continue to obstruct the line.
	if free && scored == 0 && r.Foul == 0 {
		objects := []int{}
		for _, b := range m.Balls {
			if b.ID > 0 && !b.Down {
				objects = append(objects, b.ID)
			}
		}
		exception := len(objects) == 2 && slices.Contains(objects, 21) && objects[0] >= 16 && objects[1] >= 16
		if !exception && slices.Contains(effectiveSnookeringBalls(m.Balls, actual), shot.FreeBall) {
			foul(value, "自由球遮挡下一手目标球")
		}
	}
	for _, id := range append(append([]int{}, m.Potted...), m.Off...) {
		if id >= 16 && (r.Foul > 0 || c.Target != "clear" || free && id == shot.FreeBall) {
			if !slices.Contains(r.Respot, id) {
				r.Respot = append(r.Respot, id)
			}
		}
	}
	slices.Sort(r.Respot)
	if r.Foul > 0 {
		r.Target = "red"
		if redsLeft(m.Balls) == 0 {
			r.Target = "clear"
			if c.Target != "clear" {
				r.Clear = 2
			}
		}
		visible, full, possible := missContact(c, shot, allowed)
		r.FullBall = full
		scores := c.Scores
		scores[1-c.Team] += r.Foul
		neededBefore := absInt(c.Scores[0]-c.Scores[1]) > Remaining(c)
		afterContext := c
		afterContext.Balls = m.Balls
		afterContext.Target = r.Target
		afterContext.Clear = r.Clear
		neededAfter := absInt(scores[0]-scores[1]) > Remaining(afterContext)
		r.Miss = contactWrong && !neededBefore && !neededAfter && (visible || cushionEscape(c.Balls, possible))
		return r
	}
	r.Points = scored
	r.KeepTurn = scored > 0
	switch c.Target {
	case "red":
		if scored > 0 {
			r.Target = "colour"
		} else if redsLeft(m.Balls) == 0 {
			r.Target = "clear"
			r.Clear = 2
		}
	case "colour":
		if redsLeft(m.Balls) > 0 {
			r.Target = "red"
		} else {
			r.Target = "clear"
			r.Clear = 2
		}
	case "clear":
		if actualPots > 0 {
			r.Clear++
		}
	}
	return r
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func clearSpot(pos Vec, bs []Ball, id int) bool {
	if pos.X < BallRadius || pos.X > TableWidth-BallRadius || pos.Y < BallRadius || pos.Y > TableHeight-BallRadius {
		return false
	}
	for _, b := range bs {
		if !b.Down && b.ID != id && b.Pos.sub(pos).length() < 2*BallRadius+.000001 {
			return false
		}
	}
	return true
}
func nearSpot(start Vec, bs []Ball, id int, direction float64) (Vec, bool) {
	// Move only to interval boundaries of the forbidden ball-contact regions.
	x := start.X
	gap := 2*BallRadius + .000002
	for tries := 0; tries <= len(bs); tries++ {
		moved := false
		for _, b := range bs {
			if b.Down || b.ID == id {
				continue
			}
			dy := math.Abs(b.Pos.Y - start.Y)
			if dy >= gap {
				continue
			}
			dx := math.Sqrt(gap*gap - dy*dy)
			lo, hi := b.Pos.X-dx, b.Pos.X+dx
			if x >= lo && x <= hi {
				if direction > 0 {
					x = hi + .000001
				} else {
					x = lo - .000001
				}
				moved = true
			}
		}
		if !moved {
			p := Vec{x, start.Y}
			return p, clearSpot(p, bs, id)
		}
	}
	return Vec{}, false
}
func SpotColours(balls []Ball, ids []int) ([]Ball, error) {
	out := append([]Ball{}, balls...)
	ids = append([]int{}, ids...)
	slices.Sort(ids)
	slices.Reverse(ids)
	for _, id := range ids {
		if id < 16 || id > 21 {
			return nil, errors.New("只能复位彩球")
		}
		at := -1
		for i := range out {
			if out[i].ID == id {
				at = i
				break
			}
		}
		if at < 0 {
			return nil, errors.New("缺少需要复位的彩球")
		}
		target := ColourSpot(BallValue(id))
		found := clearSpot(target, out, id)
		if !found {
			for v := 7; v >= 2; v-- {
				p := ColourSpot(v)
				if clearSpot(p, out, id) {
					target = p
					found = true
					break
				}
			}
		}
		if !found {
			target, found = nearSpot(ColourSpot(BallValue(id)), out, id, 1)
		}
		if !found && (id == 20 || id == 21) {
			target, found = nearSpot(ColourSpot(BallValue(id)), out, id, -1)
		}
		if !found {
			return nil, errors.New("没有合法的彩球复位点")
		}
		out[at].Pos = target
		out[at].Down = false
	}
	return out, nil
}
