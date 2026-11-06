package pvz_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

// This opt-in fixture is an initial condition, not a room option or alternate
// game implementation. Every later timer, input, view, sound and frame uses
// the production module/Board. The snapshot is not a naturally played match.
func TestPvZFullDensityBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_PVZ_DENSITY") != "1" {
		t.Skip("opt-in production density fixture")
	}
	out := os.Getenv("PVZ_DENSITY_OUT")
	if out == "" {
		t.Fatal("PVZ_DENSITY_OUT required")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	dense := os.Getenv("PVZ_DENSITY_MODE") == "dense"
	module := integrationModule(t)
	start := module.Start
	module.Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, options)
		if err != nil {
			return match, err
		}
		var envelope map[string]json.RawMessage
		if err = json.Unmarshal(match.Data, &envelope); err != nil {
			return match, err
		}
		var s engine.State
		if err = json.Unmarshal(envelope["runtime"], &s); err != nil {
			return match, err
		}
		if s.Map.ID != "roofnight" {
			return match, fmt.Errorf("density fixture requires roofnight")
		}
		s.Tick = 1000
		s.PhaseEndTick = 1400
		s.NextSunTick = 1100
		apply := func(actor int, kind string, row, col int) error {
			p := &s.Players[actor]
			p.Money = engine.MaxMoney
			p.Cooldowns = map[string]int64{}
			p.Deck = []string{kind}
			s.TeamCooldowns[p.Team] = map[string]int64{}
			s.LaneCooldowns = [engine.MaxRows]int64{}
			a := engine.Action{Type: "plant", Epoch: s.Epoch, Seq: p.Seq + 1, Kind: kind, Row: row, Col: col}
			if p.Side == "zombies" {
				a.Type = "spawn"
				a.Col = 0
			}
			return engine.Apply(&s, actor, a)
		}
		bodies := []string{"cabbagepult", "kernelpult", "melonpult", "starfruit", "cactus", "magnetshroom", "umbrellaleaf", "tallnut", "repeater"}
		for row := 0; row < 5; row++ {
			for col := 0; col < 9; col++ {
				if !dense && col >= 2 {
					continue
				}
				if err = apply(0, "flowerpot", row, col); err != nil {
					return match, err
				}
				if dense && row == 4 && col >= 7 {
					continue
				}
				kind := bodies[(row*9+col)%len(bodies)]
				if err = apply(0, kind, row, col); err != nil {
					return match, err
				}
				if err = apply(0, "pumpkin", row, col); err != nil {
					return match, err
				}
			}
		}
		s.Phase = "battle"
		s.BattleStartTick = s.Tick
		s.PhaseEndTick = s.Tick + int64(s.Options.RoundSeconds*1000/engine.TickMS)
		s.NextBrainTick = s.Tick + 40
		kinds := []string{"gargantuar", "redeye", "buckethead", "football", "ladder", "pogo", "zomboni", "catapult", "balloon", "peahead", "wallnuthead", "imp", "newspaper", "zombie"}
		count := 10
		if dense {
			count = 80
		}
		for i := 0; i < count; i++ {
			kind := kinds[i%len(kinds)]
			// Most slots are authentic full-health red-eye giants so the
			// stress window remains dense without freezing HP or collisions.
			if dense && i >= len(kinds) {
				kind = "redeye"
			}
			if dense && i == count-1 {
				kind = "zomboss"
			}
			if err = apply(1, kind, i%5, 0); err != nil {
				return match, err
			}
			// Initial placement changes no speed, health, damage or immunity.
			s.Zombies[len(s.Zombies)-1].X = 8500 + (i/5)*10
		}
		for i := range s.Players {
			p := &s.Players[i]
			p.Money = 10000
			p.Seq = 0
			p.Cooldowns = map[string]int64{}
			p.Deck = engine.DefaultDeck("roofnight", p.Side)
		}
		s.TeamCooldowns = [2]map[string]int64{{}, {}}
		s.LaneCooldowns = [engine.MaxRows]int64{}
		s.Events = []engine.Event{}
		s.EventSerial = 0
		if err = engine.Validate(s); err != nil {
			return match, fmt.Errorf("initial density validation: %w", err)
		}
		envelope["runtime"], err = json.Marshal(s)
		if err != nil {
			return match, err
		}
		match.Data, err = json.Marshal(envelope)
		if err != nil {
			return match, err
		}
		match.CheckpointKey = s.Epoch + "/" + s.Phase
		match.Log = []string{"本地受控初始密度验收：真实生产规则与时钟，非自然完整对局"}
		if err = module.ValidateMatch(match, ctx.Players); err != nil {
			return match, err
		}
		for _, viewer := range []int{-1, 0, 1} {
			if _, err = module.View(match, viewer); err != nil {
				return match, err
			}
			if _, err = module.SyncSnapshot(match, viewer); err != nil {
				return match, err
			}
		}
		raw, _ := json.MarshalIndent(map[string]any{"fixture": "production-Start controlled initial condition only", "dense": dense, "plants": len(s.Plants), "zombies": len(s.Zombies), "snapshot": s, "digest": engine.Digest(s), "viewsValidated": []int{-1, 0, 1}}, "", "  ")
		if err = os.WriteFile(filepath.Join(out, "initial.json"), raw, 0644); err != nil {
			return match, err
		}
		return match, nil
	}
	s, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "pvz-density", AdminUsername: "DensityRoot", AdminPassword: "pvz-density-password", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for began := time.Now(); time.Since(began) < 10*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("density fixture timed out")
}
