package pokepath_test

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

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestPokepathBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_POKEPATH_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "pokepath", "native-browser")
	if configured := os.Getenv("WIRE_BOARD_POKEPATH_BROWSER_OUT"); configured != "" {
		absolute, pathErr := filepath.Abs(configured)
		local, relativeErr := filepath.Rel(root, absolute)
		if pathErr != nil || relativeErr != nil || local == ".." || strings.HasPrefix(local, ".."+string(filepath.Separator)) {
			t.Fatal("browser acceptance output must remain inside the workspace")
		}
		out = absolute
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	web := filepath.Join(root, "web", "dist")
	if os.Getenv("WIRE_BOARD_POKEPATH_SCOPED_WEB") == "1" {
		web = filepath.Join(root, ".local", "pokepath", "native-web")
	}
	module := pokepath.Module()
	start := module.Start
	module.Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		match, startErr := start(ctx, options)
		if startErr != nil {
			return match, startErr
		}
		fixture := ""
		for _, name := range []string{"editor-owner", "challenge-ready", "result-victory", "result-defeat", "map-ready", "range-ready"} {
			if _, markerErr := os.Stat(filepath.Join(out, name)); markerErr == nil {
				fixture = name
				break
			} else if !os.IsNotExist(markerErr) {
				return match, markerErr
			}
		}
		if fixture == "" {
			return match, nil
		}
		if markerErr := os.Remove(filepath.Join(out, fixture)); markerErr != nil {
			return match, markerErr
		}
		var saved map[string]json.RawMessage
		if decodeErr := json.Unmarshal(match.Data, &saved); decodeErr != nil {
			return match, decodeErr
		}
		world, decodeErr := engine.Unmarshal(saved["snapshot"])
		if decodeErr != nil {
			return match, decodeErr
		}
		if fixture == "challenge-ready" {
			world.Gold = 1000000000
			world.Stars = 540
			world.TeamSlots = 10
			for route := 0; route < 5; route++ {
				world.Records[route] = 100
			}
			world.Records[5] = 40
			world.Progression.Ribbons[world.Route] = map[string]int{"levelCap": 1, "mirror": 1}
			world.Ribbons = 2
			for len(world.Team)+len(world.Box) < 30 {
				if applyErr := engine.Apply(world, 0, engine.Action{Type: "egg", Epoch: world.Epoch}); applyErr != nil {
					return match, applyErr
				}
			}
		} else if fixture == "map-ready" {
			world.Stars = 200
			world.Records[0], world.Records[1] = 100, 100
			world.Progression.Settings.MapEffectsMode = 0
			world.Progression.Settings.MapEffects = false
			if applyErr := engine.Apply(world, 0, engine.Action{Type: "route", Route: 6, Epoch: world.Epoch}); applyErr != nil {
				return match, applyErr
			}
		} else if fixture == "range-ready" {
			world.Progression.Settings.MapEffectsMode, world.Progression.Settings.MapEffects = 0, false
			for _, key := range []string{"pidgey", "patrat"} {
				spec := engine.CatalogData().Species[key]
				world.Team = append(world.Team, engine.Pokemon{ID: world.NextID, Species: key, Level: 1, TileID: -1, Target: "first", Sprite: spec.Sprite, Stats: engine.Stats{Power: spec.PowerBase + spec.PowerPerLevel, AttackMS: spec.AttackMSBase + spec.AttackMSPerLevel, Range: spec.RangeBase + spec.RangePerLevel, CriticalPct: spec.CriticalBase + spec.CriticalPerLevel}})
				world.NextID++
			}
			if applyErr := engine.Apply(world, 0, engine.Action{Type: "deploy", PokemonID: world.Team[2].ID, TileID: 15, Epoch: world.Epoch}); applyErr != nil {
				return match, applyErr
			}
		} else if fixture == "editor-owner" {
			world.Inventory = append(world.Inventory, engine.OwnedItem{ID: "revysBook"})
		} else {
			id := world.Team[0].ID
			if err := engine.Apply(world, 0, engine.Action{Type: "deploy", PokemonID: id, TileID: 1, Epoch: world.Epoch}); err != nil {
				return match, err
			}
			world.Wave, world.Health = 100, 4
			if fixture == "result-defeat" {
				world.Wave, world.Health = 28, 1
			}
			world.RouteWaves[world.Route] = world.Wave
			if err := engine.Apply(world, 0, engine.Action{Type: "wave", Epoch: world.Epoch}); err != nil {
				return match, err
			}
			limit, expect := int64(80), "route_complete"
			if fixture == "result-victory" {
				world.Enemies = []engine.Enemy{}
			} else {
				limit, expect = 150000, "defeat"
			}
			if err := engine.Advance(world, limit); err != nil {
				return match, err
			}
			if world.Phase != expect {
				return match, fmt.Errorf("result fixture expected %s, got %s", expect, world.Phase)
			}
		}
		snapshot, encodeErr := engine.Marshal(world)
		if encodeErr != nil {
			return match, encodeErr
		}
		saved["snapshot"] = snapshot
		match.Data, encodeErr = json.Marshal(saved)
		if encodeErr != nil {
			return match, encodeErr
		}
		return match, os.WriteFile(filepath.Join(out, fixture+"-receipt.json"), snapshot, 0600)
	}
	app, err := server.New(server.Config{DataDir: filepath.Join(out, "database"), InviteCode: "pokepath-e2e", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(web))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer host.Close()
	go host.Serve(listener)
	if err := os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 40*time.Minute; {
		if _, err := os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
