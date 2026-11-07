package escape_test

// Additional failure inventory before circuit/coupled-valve implementation:
// capacity bypass, remote switches, wrong coupling, unwinnable random targets,
// incorrect pump benefit, or a bot choosing a different booked escape route.
import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

type expedition struct {
	t   *testing.T
	m   gameapi.Module
	s   gameapi.Match
	now time.Time
}

func (x *expedition) view() map[string]any { return readView(x.t, x.m, x.s, 0) }
func (x *expedition) advance(ms int) {
	x.t.Helper()
	for i := 0; i < ms/100; i++ {
		x.now = x.now.Add(100 * time.Millisecond)
		var e error
		x.s, e = x.m.OnTimer(gameapi.Context{Players: 2, Now: x.now}, x.s)
		if e != nil {
			x.t.Fatal(e)
		}
	}
}
func (x *expedition) act(a map[string]any) {
	x.t.Helper()
	var e error
	x.s, e = action(x.t, x.m, x.s, 0, a, x.now)
	if e != nil {
		x.t.Fatal(a, e)
	}
}
func (x *expedition) rooms() []map[string]any {
	out := []map[string]any{}
	for _, r := range array(x.view(), "rooms") {
		out = append(out, obj(r))
	}
	return out
}
func (x *expedition) path(to string) []string {
	current := text(obj(array(x.view(), "players")[0]), "room")
	queue := [][]string{{current}}
	seen := map[string]bool{current: true}
	rooms := x.rooms()
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		last := p[len(p)-1]
		if last == to {
			return p
		}
		for _, r := range rooms {
			if text(r, "id") != last || !boolv(r, "known") {
				continue
			}
			for _, d := range array(r, "doors") {
				id := text(obj(d), "room")
				if !seen[id] {
					seen[id] = true
					queue = append(queue, append(append([]string{}, p...), id))
				}
			}
		}
	}
	return nil
}
func (x *expedition) goRoom(id string) {
	x.t.Helper()
	p := x.path(id)
	if len(p) == 0 {
		x.t.Fatal("no discovered path", id)
	}
	for _, next := range p[1:] {
		x.act(map[string]any{"type": "travel", "target": next})
		for i := 0; i < 100; i++ {
			x.advance(100)
			if text(obj(array(x.view(), "players")[0]), "room") == next {
				break
			}
		}
		if text(obj(array(x.view(), "players")[0]), "room") != next {
			x.t.Fatal("ordinary travel stalled")
		}
	}
}
func (x *expedition) find(kind string) map[string]any {
	x.t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		rooms := x.rooms()
		for _, r := range rooms {
			if text(r, "kind") == kind {
				x.goRoom(text(r, "id"))
				return r
			}
		}
		frontier := ""
		for _, r := range rooms {
			if !boolv(r, "known") && len(x.path(text(r, "id"))) > 0 {
				frontier = text(r, "id")
				break
			}
		}
		if frontier == "" {
			x.t.Fatal("unreachable room", kind)
		}
		x.goRoom(frontier)
	}
	x.t.Fatal("discovery bound exceeded")
	return nil
}
func (x *expedition) device(kind string) map[string]any {
	x.t.Helper()
	id := text(obj(array(x.view(), "players")[0]), "room")
	for _, r := range x.rooms() {
		if text(r, "id") == id {
			for _, d := range array(r, "objects") {
				if text(obj(d), "kind") == kind {
					return obj(d)
				}
			}
		}
	}
	x.t.Fatal("missing device", kind)
	return nil
}
func (x *expedition) approach(kind string) map[string]any {
	d := x.device(kind)
	x.act(map[string]any{"type": "inspect", "target": d["id"]})
	x.advance(5000)
	return x.device(kind)
}
func (x *expedition) solve(kind string) {
	x.t.Helper()
	var values []any
	for _, c := range array(x.view(), "journal") {
		if text(obj(c), "key") == kind {
			values = array(obj(c), "values")
		}
	}
	if len(values) != 3 {
		x.t.Fatal("required public clue missing", kind)
	}
	d := x.approach(kind)
	for i := 0; i < 3; i++ {
		for count := 0; count < 4; count++ {
			d = x.device(kind)
			if array(d, "dials")[i] == values[i] {
				break
			}
			x.act(map[string]any{"type": "turn", "target": d["id"], "index": i})
		}
	}
	x.act(map[string]any{"type": "interact", "target": d["id"]})
	if !boolv(x.device(kind), "done") {
		x.t.Fatal("public clue could not solve coupled device")
	}
}
func TestEscapeCooperativeCircuitAndCoupledPumps(t *testing.T) {
	for _, scene := range []string{"station", "archive"} {
		t.Run(scene, func(t *testing.T) {
			m := module(t)
			x := expedition{t, m, start(t, m, 2, scene, "easy", "cooperation-19"), epochTime}
			x.find("medical")
			x.approach("clue-pump")
			x.find("pump")
			d := x.approach("pump")
			x.act(map[string]any{"type": "turn", "target": d["id"], "index": 0})
			d = x.device("pump")
			dial := array(d, "dials")
			if dial[0] != float64(1) || dial[1] != float64(1) || dial[2] != float64(0) {
				t.Fatal("inlet must change its own and downstream pressure", dial)
			}
			x.solve("pump")
			x.find("power")
			d = x.approach("power")
			before := x.view()
			if len(array(before, "circuits")) != 2 {
				t.Fatal("two-slot shared power panel missing")
			}
			missing := "pump"
			if scene == "archive" {
				missing = "lift"
			}
			if _, e := action(t, m, x.s, 0, map[string]any{"type": "circuit", "target": d["id"], "circuit": missing}, x.now); e == nil {
				t.Fatal("three circuits exceed capacity")
			}
			x.act(map[string]any{"type": "circuit", "target": d["id"], "circuit": "lights"})
			x.act(map[string]any{"type": "circuit", "target": d["id"], "circuit": missing})
			if len(array(x.view(), "circuits")) != 2 {
				t.Fatal("power routing did not consume slots")
			}
			evidence(t, fmt.Sprint(scene, "-cooperative-power"), map[string]any{"passed": true, "view": x.view()})
		})
	}
}

func TestEscapeAlternateRouteCampaignsE2E(t *testing.T) {
	// Failure: computer partners ignore a human's route plan, keep restoring
	// the wrong circuit, or cannot complete either scenario's alternate route.
	m := module(t)
	for _, scene := range []string{"station", "archive"} {
		for _, n := range []int{2, 3, 4} {
			for _, difficulty := range []string{"easy", "normal", "hard"} {
				name := fmt.Sprintf("alternate-%s-%d-%s", scene, n, difficulty)
				t.Run(name, func(t *testing.T) {
					route := "passage"
					if scene == "archive" {
						route = "lift"
					}
					s := start(t, m, n, scene, difficulty, "alternate-23")
					now := epochTime
					var e error
					s, e = action(t, m, s, 0, map[string]any{"type": "plan", "route": route}, now)
					if e != nil {
						t.Fatal("human route plan rejected", e)
					}
					automated := make([]bool, n)
					for i := range automated {
						automated[i] = true
					}
					for step := 0; step < 16000 && s.Result == nil; step++ {
						now = now.Add(100 * time.Millisecond)
						if step%2 == 0 {
							for actor := 0; actor < n; actor++ {
								ctx := gameapi.Context{Players: n, Actor: actor, Automated: automated, Now: now}
								raw, e := m.BotAction(ctx, s)
								if e != nil {
									t.Fatal(e)
								}
								if len(raw) > 0 {
									s, e = m.Apply(ctx, s, raw)
									if e != nil {
										t.Fatal(string(raw), e)
									}
								}
							}
						}
						s, e = m.OnTimer(gameapi.Context{Players: n, Automated: automated, Now: now}, s)
						if e != nil {
							t.Fatal(e)
						}
					}
					v := readView(t, m, s, 0)
					if s.Result == nil || len(s.Result.Winners) != n || text(obj(v["stats"]), "route") != route {
						raw, _ := json.Marshal(v)
						t.Fatalf("alternate route failed: %s", raw)
					}
					evidence(t, name, map[string]any{"passed": true, "view": v, "result": s.Result})
				})
			}
		}
	}
}

func TestEscapeComputersRespondToSharedRally(t *testing.T) {
	// A teammate's visible call should redirect helpers, without discovering
	// hidden rooms or overriding rescue/transport obligations.
	m := module(t)
	s := start(t, m, 4, "station", "easy", "rally-7")
	now := epochTime
	step := func() {
		now = now.Add(100 * time.Millisecond)
		for actor := 2; actor < 4; actor++ {
			ctx := gameapi.Context{Actor: actor, Players: 4, Now: now}
			raw, e := m.BotAction(ctx, s)
			if e != nil {
				t.Fatal(e)
			}
			if len(raw) > 0 {
				s, e = m.Apply(ctx, s, raw)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		var e error
		s, e = m.OnTimer(gameapi.Context{Players: 4, Now: now}, s)
		if e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 100; i++ {
		step()
	}
	var e error
	s, e = action(t, m, s, 0, map[string]any{"type": "ping"}, now)
	if e != nil {
		t.Fatal(e)
	}
	called := readView(t, m, s, 0)
	if text(obj(called["rally"]), "room") != "r4" {
		t.Fatal("shared rally missing")
	}
	arrived := false
	for i := 0; i < 180; i++ {
		step()
		v := readView(t, m, s, 0)
		p := array(v, "players")
		if text(obj(p[2]), "room") == "r4" && text(obj(p[3]), "room") == "r4" {
			arrived = true
			break
		}
	}
	if !arrived {
		t.Fatal("computer partners ignored visible rally")
	}
	evidence(t, "shared-rally", map[string]any{"passed": true, "called": called, "arrived": readView(t, m, s, 0)})
}

func TestEscapeLateRouteSwitchAndIdempotentPlanE2E(t *testing.T) {
	// Failure inventory: confirming the same plan clears a partner's readiness;
	// or an already available old route makes bots abandon the newly selected
	// route, repeatedly trying to depart before its objectives are complete.
	for _, scene := range []string{"station", "archive"} {
		t.Run(scene, func(t *testing.T) {
			m := module(t)
			x := expedition{t, m, start(t, m, 2, scene, "easy", "late-plan-43"), epochTime}
			ready := func(route string) bool {
				for _, v := range array(x.view(), "routes") {
					r := obj(v)
					if text(r, "id") == route {
						return boolv(r, "available")
					}
				}
				return false
			}
			step := func(actors ...int) {
				x.now = x.now.Add(100 * time.Millisecond)
				for _, actor := range actors {
					ctx := gameapi.Context{Actor: actor, Players: 2, Now: x.now}
					raw, e := m.BotAction(ctx, x.s)
					if e != nil {
						t.Fatal(e)
					}
					if len(raw) > 0 {
						x.s, e = m.Apply(ctx, x.s, raw)
						if e != nil {
							t.Fatal(e)
						}
					}
				}
				var e error
				x.s, e = m.OnTimer(gameapi.Context{Players: 2, Now: x.now}, x.s)
				if e != nil {
					t.Fatal(e)
				}
			}
			primary, alternate := "lift", "passage"
			if scene == "archive" {
				primary, alternate = "boat", "lift"
			}
			for i := 0; i < 6000 && !ready(primary) && x.s.Result == nil; i++ {
				step(0, 1)
			}
			if !ready(primary) || x.s.Result != nil {
				t.Fatal("could not reach an available primary route before departure")
			}
			for i := 0; i < 150 && !boolv(obj(array(x.view(), "players")[1]), "ready"); i++ {
				step(1)
			}
			if !boolv(obj(array(x.view(), "players")[1]), "ready") {
				t.Fatal("partner did not confirm departure")
			}
			x.act(map[string]any{"type": "plan", "route": primary})
			if !boolv(obj(array(x.view(), "players")[1]), "ready") {
				t.Fatal("reaffirming the same route cleared a partner's readiness")
			}
			x.act(map[string]any{"type": "plan", "route": alternate})
			x.find("power")
			d := x.approach("power")
			x.act(map[string]any{"type": "circuit", "target": d["id"], "circuit": "lights"})
			extra := "pump"
			if scene == "archive" {
				extra = "lift"
			}
			x.act(map[string]any{"type": "circuit", "target": d["id"], "circuit": extra})
			if !ready(primary) || ready(alternate) {
				t.Fatal("late-switch setup did not retain the old route and pending new route")
			}
			for i := 0; i < 16000 && x.s.Result == nil; i++ {
				step(0, 1)
			}
			if x.s.Result == nil || len(x.s.Result.Winners) != 2 || text(obj(x.view()["stats"]), "route") != alternate {
				t.Fatal("computer partners did not finish the newly selected route")
			}
			evidence(t, scene+"-late-route-switch", map[string]any{"passed": true, "view": x.view(), "result": x.s.Result})
		})
	}
}
