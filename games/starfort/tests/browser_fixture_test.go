package starfort_test

// Scenarios exist only in this opt-in test binary. Unknown fields still reach
// production Configure and are rejected; scenario is stripped before Configure
// and Start. Terminal scenarios seed the last battle, then the real timer
// resolves the win/loss and host archive. No production action advances time.
import (
	"encoding/json"
	"errors"
	"fmt"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func scenarioModule() gameapi.Module {
	m := starfort.Module()
	configure, start, apply := m.Configure, m.Start, m.Apply
	split := func(raw json.RawMessage) (string, json.RawMessage, error) {
		v := map[string]json.RawMessage{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &v); err != nil {
				return "", nil, err
			}
		}
		scenario := "natural"
		if value, ok := v["scenario"]; ok {
			if err := json.Unmarshal(value, &scenario); err != nil {
				return "", nil, err
			}
			delete(v, "scenario")
		}
		switch scenario {
		case "impact", "natural", "battle", "victory-ready", "defeat-ready", "advanced", "zero-gold", "growth-vote", "cleared-5", "cleared-10", "cleared-15", "stage2", "stage3", "stage4", "endless-rotation", "objectives", "objectives-clear", "objectives-fail", "legacy-save":
		default:
			return "", nil, errors.New("unknown test scenario")
		}
		body, err := json.Marshal(v)
		return scenario, body, err
	}
	m.Configure = func(n int, raw json.RawMessage) (json.RawMessage, error) {
		scenario, body, err := split(raw)
		if err != nil {
			return nil, err
		}
		value, err := configure(n, body)
		if err != nil {
			return nil, err
		}
		var options map[string]any
		_ = json.Unmarshal(value, &options)
		options["scenario"] = scenario
		return json.Marshal(options)
	}
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		scenario, body, err := split(raw)
		if err != nil {
			return gameapi.Match{}, err
		}
		match, err := start(ctx, body)
		if err != nil {
			return match, err
		}
		var s starfort.State
		if err = json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		s.Log = append(s.Log, "浏览器／HTTP 验收夹具："+scenario)
		// Stage construction resolves real production choices using a virtual
		// clock, solely inside this test binary. The returned room then runs its
		// usual real host timer. Clients cannot request these in production.
		stage := 1
		switch scenario {
		case "stage2", "cleared-10":
			stage = 2
		case "stage3", "cleared-15":
			stage = 3
		case "stage4", "endless-rotation", "advanced", "objectives", "objectives-clear", "objectives-fail":
			stage = 4
		}
		if stage > 1 {
			s.Options.Mode = "endless"
			if err = fixtureWorldStage(ctx, &s, stage); err != nil {
				return match, err
			}
		}
		switch scenario {
		case "impact":
			s.Phase = "battle"
			s.NextAt = 0
			s.WaveInfo = starfort.WaveInfo{Title: "命中时序验收", Total: 1, Spawned: 1}
			pad := s.World.Map.Pads[0]
			route := s.World.Map.Routes[0]
			best, progress := 1e9, 0.0
			for d := 0.0; d < 2000; d += 5 {
				x, y := fixturePathPosition(route.Points, d)
				dist := math.Hypot(x-pad.X, y-pad.Y)
				if dist < best {
					best, progress = dist, d
				}
			}
			x, y := fixturePathPosition(route.Points, progress)
			s.TowerSerial = 1
			s.EnemySerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arrow", Level: 1, Spent: 70, Priority: "first"}}
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "shield", X: x, Y: y, Progress: progress, HP: 80, MaxHP: 80, Shield: 10, Speed: 0.01, Reward: 10, Leak: 1, Route: route.ID}}
		case "battle":
			s.Phase = "battle"
			s.NextAt = 0
			s.Wave = 4
			s.CompletedWaves = 3
			s.WaveInfo = starfort.WaveInfo{Title: "防线演练", Description: "测试夹具：四种塔与移动敌人", Rule: "shield", Total: 8, Spawned: 8}
			kinds := []string{"arrow", "cannon", "frost", "arcane"}
			for i, kind := range kinds {
				s.TowerSerial++
				s.Towers = append(s.Towers, starfort.Tower{ID: s.TowerSerial, Pad: i, Owner: i % ctx.Players, Kind: kind, Level: 1, Spent: 150, Priority: "first"})
			}
			for i := 0; i < 8; i++ {
				s.EnemySerial++
				kind := "slime"
				if i%3 == 1 {
					kind = "shield"
				}
				x, y := fixturePathPosition(s.World.Map.Routes[0].Points, 520+float64(i)*24)
				s.Enemies = append(s.Enemies, starfort.Enemy{ID: s.EnemySerial, Kind: kind, X: x, Y: y, Progress: 520 + float64(i)*24, HP: 2000, MaxHP: 2000, Shield: 300, Speed: 15, Reward: 10, Leak: 1, Route: s.World.Map.Routes[0].ID})
			}
			for i := range s.Players {
				s.Players[i].Gold = 2500
			}
		case "victory-ready":
			s.Phase = "battle"
			s.NextAt = 0
			s.Wave = s.Options.Waves
			s.CompletedWaves = s.Wave - 1
			s.WaveInfo = starfort.WaveInfo{Title: "最后一波", Description: "测试夹具：已消灭最终首领", Rule: "boss", Total: 1, Spawned: 1, Boss: true}
		case "defeat-ready":
			s.Phase = "battle"
			s.NextAt = 0
			s.WaveInfo = starfort.WaveInfo{Title: "基地危急", Description: "测试夹具：敌人即将抵达基地", Rule: "boss", Total: 1, Spawned: 1, Boss: true}
			s.EnemySerial = 1
			route := s.World.Map.Routes[0]
			distance := fixturePathLength(route.Points) - 1
			x, y := fixturePathPosition(route.Points, distance)
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "boss", X: x, Y: y, Progress: distance, HP: 1, MaxHP: 1, Speed: 60, Reward: 1, Leak: 30, Route: route.ID}}
		case "advanced":
			s.Options.Mode = "endless"
			s.Options.Waves = 10
			if err = fixtureClearedWave(ctx, &s, 20, true); err != nil {
				return match, err
			}
			s.WaveInfo = starfort.WaveInfo{Title: "第 21 波 · 无尽挑战", Description: "测试夹具：已守过 20 波", Rule: "shield", Total: 32}
			s.NextAt = ctx.Now.Add(45 * time.Second).UnixMilli()
			for i := range s.Players {
				s.Players[i].Gold = 3500
			}
			s.TowerSerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arcane", Level: 5, Branch: "power", Spent: 1000, Priority: "first"}}
		case "zero-gold":
			for i := range s.Players {
				s.Players[i].Gold = 0
			}
			s.NextAt = ctx.Now.Add(45 * time.Second).UnixMilli()
		case "growth-vote", "cleared-5", "cleared-10", "cleared-15":
			wave := 5
			if scenario == "cleared-10" {
				wave = 10
			}
			if scenario == "cleared-15" {
				wave = 15
			}
			for i := range s.Players {
				s.Players[i].Gold = 1500
			}
			s.TowerSerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arrow", Level: 2, Spent: 153, Priority: "first"}}
			if err = fixtureClearedWave(ctx, &s, wave, false); err != nil {
				return match, err
			}
		case "stage2", "stage3", "stage4", "endless-rotation":
			if scenario == "endless-rotation" {
				if err = fixtureClearedWave(ctx, &s, 25, true); err != nil {
					return match, err
				}
			}
			s.Phase = "battle"
			s.NextAt = 0
			s.ServerAt = ctx.Now.UnixMilli()
			s.WaveInfo = starfort.WaveInfo{Title: fmt.Sprintf("第 %d 波 · 多路防守演练", s.Wave), Description: "测试夹具：持续多入口交通与功能型敌人", Rule: "mixed", Total: 24, Spawned: 24}
			kinds := []string{"slime", "healer", "banner", "splitter", "engineer", "boss"}
			s.Enemies = []starfort.Enemy{}
			s.EnemySerial = 0
			for ri, route := range s.World.Map.Routes {
				if !route.Active {
					continue
				}
				for i := 0; i < 6; i++ {
					distance := 80 + float64(i)*42
					x, y := fixturePathPosition(route.Points, distance)
					s.EnemySerial++
					s.Enemies = append(s.Enemies, starfort.Enemy{ID: s.EnemySerial, Kind: kinds[(i+ri)%len(kinds)], Route: route.ID, X: x, Y: y, Progress: distance, HP: 5000, MaxHP: 5000, Speed: 12, Reward: 10, Leak: 1})
				}
			}
			s.Towers = []starfort.Tower{}
			s.TowerSerial = 0
			for i, pad := range s.World.Map.Pads {
				if i%4 != 0 {
					continue
				}
				s.TowerSerial++
				s.Towers = append(s.Towers, starfort.Tower{ID: s.TowerSerial, Pad: pad.ID, Owner: (i / 4) % ctx.Players, Kind: []string{"arrow", "cannon", "frost", "arcane"}[(i/4)%4], Level: 2, Spent: 250, Priority: "first"})
			}
			for i := range s.Players {
				s.Players[i].Gold = 3500
			}
		case "objectives", "objectives-clear", "objectives-fail":
			for i := range s.Players {
				s.Players[i].Gold = 3500
			}
			s.BaseHP = 20
			s.NextAt = ctx.Now.Add(45 * time.Second).UnixMilli()
		case "legacy-save":
			// Deliberately emit a frozen schema-1 battle, exactly as an old
			// persisted room would look before its first v2 read/timer.
			s.Phase = "battle"
			s.NextAt = 0
			s.Wave = 4
			s.CompletedWaves = 3
			s.WaveInfo = starfort.WaveInfo{Title: "旧版存档", Description: "测试夹具：迁移中的进行中战斗", Rule: "shield", Total: 1, Spawned: 1}
			s.EnemySerial = 1
			s.TowerSerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arrow", Level: 2, Spent: 153}}
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "shield", X: 370, Y: 300, Progress: 580, HP: 5000, MaxHP: 5000, Speed: 10, Reward: 10, Leak: 1}}
			raw, _ := json.Marshal(s)
			var old map[string]any
			_ = json.Unmarshal(raw, &old)
			delete(old, "world")
			for _, key := range []string{"towers", "enemies"} {
				for _, entry := range old[key].([]any) {
					v := entry.(map[string]any)
					delete(v, "priority")
					delete(v, "route")
					delete(v, "phase")
					delete(v, "abilityTick")
				}
			}
			match.Schema = 1
			match.Data, _ = json.Marshal(old)
			match.SystemDeadline = ctx.Now.Add(time.Second).UnixMilli()
			return match, nil
		}
		return s.Envelope()
	}
	m.Apply = func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		next, err := apply(ctx, match, raw)
		if err != nil {
			return next, err
		}
		var before, after starfort.State
		if json.Unmarshal(match.Data, &before) != nil || json.Unmarshal(next.Data, &after) != nil || before.Phase != "prepare" || after.Phase != "battle" {
			return next, nil
		}
		fixture := ""
		for _, line := range after.Log {
			if strings.Contains(line, "验收夹具：objectives-clear") {
				fixture = "clear"
			}
			if strings.Contains(line, "验收夹具：objectives-fail") {
				fixture = "fail"
			}
		}
		if fixture == "" {
			return next, nil
		}
		// Explicitly skipped combat. Mission acceptance, readiness, progression,
		// rewards and persistence remain production code and real host timers.
		after.Enemies = []starfort.Enemy{}
		after.WaveInfo.Spawned = after.WaveInfo.Total
		for i := range after.World.Objectives {
			o := &after.World.Objectives[i]
			if o.Kind != "convoy" || o.Status != "active" || after.Wave <= o.StartWave {
				continue
			}
			for _, route := range after.World.Map.Routes {
				if route.ID != o.Route {
					continue
				}
				path := append([]starfort.Point{}, route.Points...)
				path[len(path)-1] = after.World.Map.Base
				// On the second fixture wave skip the long safe journey. The
				// real timer still moves the final 10 units and delivers rewards.
				o.Progress = fixturePathLength(path) - 10
				o.X, o.Y = fixturePathPosition(path, o.Progress)
			}
		}
		if fixture == "fail" {
			for i := range after.World.Objectives {
				o := &after.World.Objectives[i]
				if o.Status != "active" {
					continue
				}
				o.HP = .1
				for _, route := range after.World.Map.Routes {
					if route.ID != o.Route {
						continue
					}
					distance := 90.
					if o.Kind == "convoy" {
						distance = o.Progress
					}
					x, y := fixturePathPosition(route.Points, distance)
					after.EnemySerial++
					after.Enemies = append(after.Enemies, starfort.Enemy{ID: after.EnemySerial, Kind: "slime", Route: route.ID, X: x, Y: y, Progress: distance, HP: 5000, MaxHP: 5000, Speed: 1, Reward: 10, Leak: 1})
				}
			}
		}
		return after.Envelope()
	}
	return m
}

func fixturePathLength(points []starfort.Point) float64 {
	d := 0.0
	for i := 1; i < len(points); i++ {
		d += math.Hypot(points[i].X-points[i-1].X, points[i].Y-points[i-1].Y)
	}
	return d
}
func fixturePathPosition(points []starfort.Point, d float64) (float64, float64) {
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		length := math.Hypot(b.X-a.X, b.Y-a.Y)
		if d <= length {
			r := d / length
			return a.X + (b.X-a.X)*r, a.Y + (b.Y-a.Y)*r
		}
		d -= length
	}
	p := points[len(points)-1]
	return p.X, p.Y
}
func fixtureWorldStage(ctx gameapi.Context, s *starfort.State, target int) error {
	for s.World.Stage < target {
		if err := fixtureClearedWave(ctx, s, s.World.Stage*5, true); err != nil {
			return err
		}
	}
	return nil
}
func fixtureClearedWave(ctx gameapi.Context, s *starfort.State, wave int, resolve bool) error {
	s.Phase = "battle"
	s.NextAt = 0
	s.Wave = wave
	s.CompletedWaves = wave - 1
	s.Enemies = []starfort.Enemy{}
	s.WaveInfo = starfort.WaveInfo{Title: fmt.Sprintf("第 %d 波已清场", wave), Description: "测试夹具：跳过前序战斗，后续结算由真实系统计时器完成", Rule: "elite", Total: 1, Spawned: 1}
	s.ServerAt = ctx.Now.UnixMilli()
	if !resolve {
		return nil
	}
	m := starfort.Module()
	match, err := s.Envelope()
	if err != nil {
		return err
	}
	next := ctx
	next.Actor = -1
	next.Now = ctx.Now.Add(time.Second)
	match, err = m.OnTimer(next, match)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(match.Data, s); err != nil {
		return err
	}
	for _, kind := range []string{"route_vote", "boon_vote"} {
		for actor := 0; actor < ctx.Players; actor++ {
			choice := s.World.Choice
			if kind == "boon_vote" {
				choice = s.World.BoonChoice
			}
			if choice == nil {
				break
			}
			action, _ := json.Marshal(map[string]any{"type": kind, "choice": choice.Options[0].ID, "epoch": s.Epoch})
			next.Actor = actor
			match, err = m.Apply(next, match, action)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(match.Data, s); err != nil {
				return err
			}
		}
	}
	s.ServerAt = ctx.Now.UnixMilli()
	s.NextAt = ctx.Now.Add(time.Duration(s.Options.PrepSeconds) * time.Second).UnixMilli()
	return nil
}

func TestStarfortBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_STARFORT_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "starfort", "browser")
	if override := os.Getenv("WIRE_BOARD_STARFORT_BROWSER_OUT"); override != "" {
		if !filepath.IsAbs(override) {
			override = filepath.Join(root, override)
		}
		rel, e := filepath.Rel(root, override)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("browser output must stay inside repository")
		}
		out = override
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "starfort-browser", AdminUsername: "上架管理员", AdminPassword: "starfort-browser-test", AssetsBaseURL: os.Getenv("WIRE_BOARD_STARFORT_ASSETS"), Modules: []gameapi.Module{scenarioModule()}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hs := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer hs.Close()
	go hs.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 20*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
