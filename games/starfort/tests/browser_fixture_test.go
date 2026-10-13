package starfort_test

// Scenarios exist only in this opt-in test binary. Unknown fields still reach
// production Configure and are rejected; scenario is stripped before Configure
// and Start. Terminal scenarios seed the last battle, then the real timer
// resolves the win/loss and host archive. No production action advances time.
import (
	"encoding/json"
	"errors"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func scenarioModule() gameapi.Module {
	m := starfort.Module()
	configure, start := m.Configure, m.Start
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
		case "natural", "battle", "victory-ready", "defeat-ready", "advanced", "zero-gold":
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
		switch scenario {
		case "battle":
			s.Phase = "battle"
			s.NextAt = 0
			s.Wave = 4
			s.CompletedWaves = 3
			s.WaveInfo = starfort.WaveInfo{Title: "防线演练", Description: "测试夹具：四种塔与移动敌人", Rule: "shield", Total: 8, Spawned: 8}
			kinds := []string{"arrow", "cannon", "frost", "arcane"}
			for i, kind := range kinds {
				s.TowerSerial++
				s.Towers = append(s.Towers, starfort.Tower{ID: s.TowerSerial, Pad: i, Owner: i % ctx.Players, Kind: kind, Level: 1, Spent: 150})
			}
			for i := 0; i < 8; i++ {
				s.EnemySerial++
				kind := "slime"
				if i%3 == 1 {
					kind = "shield"
				}
				s.Enemies = append(s.Enemies, starfort.Enemy{ID: s.EnemySerial, Kind: kind, X: 310 + float64(i)*24, Y: 300, Progress: 520 + float64(i)*24, HP: 2000, MaxHP: 2000, Shield: 300, Speed: 15, Reward: 10, Leak: 1})
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
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "boss", X: 999, Y: 500, Progress: 1409, HP: 1, MaxHP: 1, Speed: 60, Reward: 1, Leak: 30}}
		case "advanced":
			s.Options.Mode = "endless"
			s.Options.Waves = 10
			s.Wave = 21
			s.CompletedWaves = 20
			s.WaveInfo = starfort.WaveInfo{Title: "第 21 波 · 无尽挑战", Description: "测试夹具：已守过 20 波", Rule: "shield", Total: 32}
			s.NextAt = ctx.Now.Add(45 * time.Second).UnixMilli()
			for i := range s.Players {
				s.Players[i].Gold = 3500
			}
			s.TowerSerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arcane", Level: 5, Branch: "power", Spent: 1000}}
		case "zero-gold":
			for i := range s.Players {
				s.Players[i].Gold = 0
			}
			s.NextAt = ctx.Now.Add(45 * time.Second).UnixMilli()
		}
		return s.Envelope()
	}
	return m
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
