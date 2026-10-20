package tests

// Failure cases are specified first in BROWSER_REFACTOR_SCENARIOS.md. This is
// production HTTP/auth/SQLite and wall clock; wrappers only arrange a labelled
// opening position. No browser state or live rule/timer behavior is replaced.
import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func fortressBrowserOpening(ctx gameapi.Context, module gameapi.Module, m gameapi.Match, out, scenario string) (gameapi.Match, error) {
	var s fb.State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return m, err
	}
	if s.MapID != "stage-16" || len(s.Tanks) != 2 {
		return m, fmt.Errorf("timeline fixture requires stage-16 and two seats")
	}
	for i := range s.Tanks {
		tankID := 1
		if scenario == "poison" && i == 0 {
			tankID = 2
		}
		a, _ := json.Marshal(fb.Action{Type: "select", Epoch: s.Epoch, Tank: tankID})
		c := ctx
		c.Actor = i
		var err error
		m, err = module.Apply(c, m, a)
		if err != nil {
			return m, err
		}
	}
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return m, err
	}
	s.Wind = 0
	s.Tanks[0].X, s.Tanks[0].Y = 300, 808
	s.Tanks[1].X, s.Tanks[1].Y = 430, 816
	if scenario == "heal" {
		s.Tanks[0].Items = []int{2}
		s.Tanks[1].HP = 400
	}
	if scenario == "lethal" {
		s.Tanks[1].HP = 1
	}
	if scenario == "cancel" || scenario == "windup-hit" {
		s.Tanks[0].HP = 1
		if scenario == "windup-hit" {
			s.Tanks[0].HP = 100
		}
		s.Sequence++
		s.Hazards = []fb.Hazard{{ID: s.Sequence, Kind: "fire", X: s.Tanks[0].X, Y: s.Tanks[0].Y, R: 40, At: ctx.Now.UnixMilli(), EndAt: ctx.Now.Add(15 * time.Second).UnixMilli(), NextTickAt: ctx.Now.Add(7 * time.Second).UnixMilli(), Source: 1}}
		m.Data, _ = json.Marshal(s)
		evidence, _ := json.MarshalIndent(map[string]any{"scenario": scenario, "initial": s, "source": "opening fixture only; real hazard deadline kills shooter during native-key windup"}, "", "  ")
		if e := os.WriteFile(filepath.Join(out, "calibration-"+scenario+".json"), append(evidence, '\n'), 0600); e != nil {
			return m, e
		}
		return module.Restore(ctx, m)
	}
	s.Log = append(s.Log, "浏览器验收定点：仅调整开局位置；射击、计时、伤害和落地均由生产规则驱动。")
	m.Data, _ = json.Marshal(s)
	calibrationBase := m
	if scenario == "heal" {
		raw, _ := json.Marshal(fb.Action{Type: "item", Epoch: s.Epoch, Turn: s.Turn, Slot: 0})
		c := ctx
		c.Actor = s.Current
		var err error
		calibrationBase, err = module.Apply(c, m, raw)
		if err != nil {
			return m, err
		}
	}
	// Search through the public module interface, not a second trajectory model.
	// Calibration is detached from the live match; its timer calls never alter
	// the server clock or the match returned below.
	var best fb.Action
	var bestShot *fb.Shot
	var bestFalls []fb.Fall
	var bestMatch gameapi.Match
	bestDuration := int64(0)
	for angle := 15; angle <= 80; angle += 5 {
		for power := 0; power <= 100; power += 2 {
			a := fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: float64(angle), Power: float64(power), Facing: 1, Weapon: "primary"}
			if (scenario == "fire" || scenario == "fire-crater") || scenario == "poison" {
				a.Weapon = "special"
			}
			raw, _ := json.Marshal(a)
			c := ctx
			c.Actor = s.Current
			shot, err := module.Apply(c, calibrationBase, raw)
			if err != nil {
				continue
			}
			var flying fb.State
			if json.Unmarshal(shot.Data, &flying) != nil || flying.Resolution == nil || flying.LastShot == nil {
				continue
			}
			duration := flying.Resolution.ImpactAt - ctx.Now.UnixMilli()
			if duration <= bestDuration || duration < 900 {
				continue
			}
			c.Actor = -1
			c.Now = time.UnixMilli(flying.Resolution.ImpactAt)
			hit, err := module.OnTimer(c, shot)
			if err != nil {
				continue
			}
			var impact fb.State
			if json.Unmarshal(hit.Data, &impact) != nil || impact.Resolution == nil {
				continue
			}
			if scenario == "heal" {
				if impact.Tanks[1].HP <= s.Tanks[1].HP {
					continue
				}
			} else if impact.Tanks[1].HP >= s.Tanks[1].HP {
				continue
			}
			if (scenario == "fire" || scenario == "fire-crater") || scenario == "poison" {
				if len(impact.Hazards) == 0 {
					continue
				}
				c.Now = time.UnixMilli(flying.Resolution.ImpactAt + 2500)
				later, err := module.OnTimer(c, hit)
				if err != nil {
					continue
				}
				var landed fb.State
				if json.Unmarshal(later.Data, &landed) != nil {
					continue
				}
				found := false
				for _, e := range landed.Events {
					if e.Type == "status" && e.Target == 1 {
						found = true
					}
				}
				if !found {
					continue
				}
			}
			if scenario == "timeline" && len(impact.Resolution.Falls) == 0 {
				continue
			}
			best, bestShot, bestFalls, bestDuration = a, flying.LastShot, impact.Resolution.Falls, duration
			bestMatch = shot
		}
	}
	if bestShot == nil {
		return m, fmt.Errorf("could not calibrate a legal >=900ms damaging/falling shot")
	}
	var followup *fb.Action
	if scenario == "fire-crater" {
		c := ctx
		c.Actor = -1
		c.Now = time.UnixMilli(bestShot.ImpactAt + 3000)
		settled, e := module.OnTimer(c, bestMatch)
		if e == nil {
			var live fb.State
			_ = json.Unmarshal(settled.Data, &live)
			if live.Resolution == nil && len(live.Hazards) > 0 {
				hazard := live.Hazards[0]
				for angle := 15; angle <= 80 && followup == nil; angle += 5 {
					for power := 0; power <= 100 && followup == nil; power += 2 {
						for _, facing := range []int{-1, 1} {
							a := fb.Action{Type: "fire", Epoch: live.Epoch, Turn: live.Turn, Angle: float64(angle), Power: float64(power), Facing: facing, Weapon: "primary"}
							raw, _ := json.Marshal(a)
							shotCtx := c
							shotCtx.Actor = live.Current
							next, e := module.Apply(shotCtx, settled, raw)
							if e != nil {
								continue
							}
							var flying fb.State
							_ = json.Unmarshal(next.Data, &flying)
							if flying.Resolution == nil || flying.Resolution.ImpactAt >= hazard.EndAt-500 {
								continue
							}
							shotCtx.Actor = -1
							shotCtx.Now = time.UnixMilli(flying.Resolution.ImpactAt)
							hit, e := module.OnTimer(shotCtx, next)
							if e != nil {
								continue
							}
							var after fb.State
							_ = json.Unmarshal(hit.Data, &after)
							present := false
							for _, h := range after.Hazards {
								if h.ID == hazard.ID {
									present = true
								}
							}
							if !present && len(after.Craters) > len(live.Craters) {
								copy := a
								followup = &copy
								break
							}
						}
					}
				}
			}
		}
		if followup == nil {
			return m, fmt.Errorf("could not calibrate support-removing followup shot")
		}
	}
	evidence, _ := json.MarshalIndent(map[string]any{"followupAction": followup, "scenario": scenario, "initial": s, "action": best, "flightMS": bestDuration, "predictedShot": bestShot, "predictedFalls": bestFalls, "source": "production Module.Apply/OnTimer; detached calibration only"}, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "calibration-"+scenario+".json"), append(evidence, '\n'), 0600); err != nil {
		return m, err
	}
	return m, nil
}

func TestFortressBlueBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_FORTRESSBLUE_BROWSER") != "1" {
		t.Skip("explicit real browser acceptance only")
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out, err := filepath.Abs(os.Getenv("WIRE_BOARD_FORTRESSBLUE_BROWSER_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, out)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Fatal("output must be a dedicated repository directory")
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	modules := games.Modules()
	for i := range modules {
		if modules[i].ID == "fortressblue" {
			original := modules[i]
			modules[i].Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
				m, e := original.Start(ctx, raw)
				if e != nil {
					return m, e
				}
				data, e := os.ReadFile(filepath.Join(out, "scenario"))
				if e != nil || strings.TrimSpace(string(data)) == "normal" {
					return m, nil
				}
				scenario := strings.TrimSpace(string(data))
				if scenario != "timeline" && scenario != "lethal" && scenario != "heal" && scenario != "fire" && scenario != "poison" && scenario != "fire-crater" && scenario != "cancel" && scenario != "windup-hit" {
					return m, fmt.Errorf("unknown browser scenario %q", scenario)
				}
				return fortressBrowserOpening(ctx, original, m, out, scenario)
			}
		}
	}
	s, err := server.New(server.Config{DataDir: filepath.Join(out, "temporary/data"), InviteCode: "fortress-browser", AdminUsername: "坦克验收管理员", AdminPassword: "fortress-browser-test-only", Modules: modules}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for start := time.Now(); time.Since(start) < 25*time.Minute; {
		if _, e := os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture exceeded 25 minutes")
}
