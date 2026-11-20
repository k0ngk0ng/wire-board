package hotlinemiami_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/hotlinemiami/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func encode(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func view(t *testing.T, module gameapi.Module, match gameapi.Match, seat int) map[string]any {
	t.Helper()
	value, err := module.View(match, seat)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(encode(value)))
	decoder.UseNumber()
	if err = decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func integer(value any) int64 {
	number, ok := value.(json.Number)
	if !ok {
		panic(fmt.Sprintf("expected JSON integer, got %v", value))
	}
	result, err := number.Int64()
	if err != nil {
		panic(err)
	}
	return result
}

func artifact(t *testing.T, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", ".local", "hotlinemiami", "verification", "module")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	name := strings.ReplaceAll(t.Name(), "/", "-") + ".json"
	if err := os.WriteFile(filepath.Join(directory, name), encode(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationAndPreparation(t *testing.T) {
	module := game.Module()
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 5, 33} {
		if _, err := module.Configure(count, nil); err == nil {
			t.Fatalf("accepted unsupported capacity %d", count)
		}
	}
	for _, raw := range []string{`{"map":"missing"}`, `{"roundSeconds":1}`, `{"roundSeconds":180.5}`, `{"characters":["missing","jacket"]}`, `{"weapons":["laser","pistol"]}`, `{"score":100}`, `{"map":null}`, `{"map":"no-talk","map":"push-it"}`} {
		if _, err := module.Configure(2, json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid configuration accepted: %s", raw)
		}
	}
	options, err := module.Configure(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := gameapi.PreparationContext{Actor: 1, Capacity: 3, Bots: []bool{false, false, true}}
	for _, action := range []any{map[string]any{"type": "map", "map": "push-it"}, map[string]any{"type": "character", "seat": 0, "character": "biker"}, map[string]any{"type": "weapon", "seat": 2, "weapon": "uzi"}} {
		if _, err = module.Prepare(ctx, options, encode(action)); err == nil {
			t.Fatalf("unauthorized preparation accepted: %v", action)
		}
	}
	options, err = module.Prepare(ctx, options, encode(map[string]any{"type": "character", "seat": 1, "character": "biker"}))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor, ctx.Host = 0, true
	options, err = module.Prepare(ctx, options, encode(map[string]any{"type": "weapon", "seat": 2, "weapon": "katana"}))
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		Characters []string `json:"characters"`
		Weapons    []string `json:"weapons"`
	}
	if err = json.Unmarshal(options, &prepared); err != nil || prepared.Characters[1] != "biker" || prepared.Weapons[2] != "katana" {
		t.Fatal("preparation lost seat ownership", string(options), err)
	}
	artifact(t, map[string]any{"passed": true, "options": json.RawMessage(options)})
}

func TestNaturalCompleteMatches(t *testing.T) {
	for _, mapID := range []string{"no-talk", "full-house", "push-it"} {
		for _, count := range []int{2, 3, 4} {
			t.Run(fmt.Sprintf("%s-%d", mapID, count), func(t *testing.T) {
				module := game.Module()
				ctx := gameapi.Context{Players: count, Now: time.Unix(100000, 0), Automated: make([]bool, count)}
				for i := range ctx.Automated {
					ctx.Automated[i] = true
				}
				match, err := module.Start(ctx, encode(map[string]any{"map": mapID, "roundSeconds": 180}))
				if err != nil {
					t.Fatal(err)
				}
				initial := json.RawMessage(append([]byte(nil), match.Data...))
				base := ctx.Now
				inputs, peakBytes := 0, 0
				for step := 1; step <= 10950 && match.Result == nil; step++ {
					ctx.Now = base.Add(time.Duration((int64(step)*1000+59)/60) * time.Millisecond)
					ctx.InputReceivedAt = ctx.Now
					for seat := 0; seat < count; seat++ {
						ctx.Actor = seat
						before := sha256.Sum256(match.Data)
						action, botErr := module.BotAction(ctx, match)
						if botErr != nil || sha256.Sum256(match.Data) != before {
							t.Fatal("bot error or mutation", botErr)
						}
						if len(action) > 0 {
							var payload struct {
								Type string `json:"type"`
							}
							if json.Unmarshal(action, &payload) != nil || payload.Type != "input" {
								t.Fatal("bot bypassed ordinary input", string(action))
							}
							match, err = module.Apply(ctx, match, action)
							if err != nil {
								t.Fatal(err, string(action))
							}
							inputs++
						}
					}
					ctx.Actor = -1
					match, err = module.OnTimer(ctx, match)
					if err != nil {
						t.Fatal(err)
					}
					if step%120 == 0 {
						if err = module.ValidateMatch(match, count); err != nil {
							t.Fatal(err)
						}
						projection := encode(view(t, module, match, -1))
						if len(projection) > peakBytes {
							peakBytes = len(projection)
						}
						if strings.Contains(string(projection), `"pending"`) || strings.Contains(string(projection), `"walls"`) {
							t.Fatal("frequent projection contains private queue or static map")
						}
					}
				}
				if match.Result == nil || inputs < 100 || len(match.Result.Scores) != count || peakBytes > 24000 {
					t.Fatal("incomplete natural match or unbounded projection", inputs, peakBytes)
				}
				final := view(t, module, match, -1)
				kills := 0
				for _, player := range final["players"].([]any) {
					kills += int(integer(player.(map[string]any)["kills"]))
				}
				if kills == 0 {
					t.Fatal("bots completed time without genuine combat")
				}
				artifact(t, map[string]any{"passed": true, "scope": "production module natural full match, accelerated clock only", "initial": initial, "inputs": inputs, "peakProjectionBytes": peakBytes, "kills": kills, "final": final, "result": match.Result})
			})
		}
	}
}

func TestInputEpochOwnershipAndReclaim(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := view(t, module, match, 0)
	player := before["players"].([]any)[0].(map[string]any)
	action := map[string]any{"type": "input", "epoch": before["epoch"], "seq": 1, "life": player["life"], "moveX": 1000, "moveY": 0, "aim": 0, "fire": false}
	if !module.AcceptStale(ctx, match, encode(action)) {
		t.Fatal("valid independent realtime input rejected")
	}
	match, err = module.Apply(ctx, match, encode(action))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"epoch": "old", "moveX": 1001, "aim": 1024, "life": 999, "seq": 99999999999, "score": 100} {
		bad := map[string]any{}
		for k, v := range action {
			bad[k] = v
		}
		bad["seq"] = 2
		bad[key] = value
		if _, err = module.Apply(ctx, match, encode(bad)); err == nil {
			t.Fatalf("invalid input %s accepted", key)
		}
	}
	ctx.Actor = -1
	if _, err = module.Apply(ctx, match, encode(action)); err == nil {
		t.Fatal("spectator controlled a seat")
	}
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	active := view(t, module, match, 0)
	match, err = module.Reclaim(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, module, match, 0)
	if after["epoch"] != before["epoch"] || after["startAt"] != before["startAt"] || after["frame"] != active["frame"] {
		t.Fatal("manual handoff reset match identity or clock")
	}
	artifact(t, map[string]any{"passed": true, "before": before, "active": active, "after": after})
}

func TestAutoplayLatchesComputerParticipation(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0), Automated: []bool{false, false}}
	match, err := module.Start(ctx, nil)
	if err != nil || match.HasComputerPlayers {
		t.Fatal("manual match incorrectly marked computerized", err)
	}
	ctx.Automated[0] = true
	action, err := module.BotAction(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	match, err = module.Apply(ctx, match, action)
	if err != nil || !match.HasComputerPlayers {
		t.Fatal("autoplay did not latch computer participation", err)
	}
	ctx.Automated[0] = false
	match, err = module.Reclaim(ctx, match)
	if err != nil || !match.HasComputerPlayers {
		t.Fatal("canceling autoplay restored rated eligibility", err)
	}
	match, err = module.Restore(ctx, match)
	if err != nil || !match.HasComputerPlayers {
		t.Fatal("restart restored rated eligibility", err)
	}
	artifact(t, map[string]any{"passed": true, "scope": "production module autoplay input, manual handoff and restore retain computer participation", "hasComputerPlayers": match.HasComputerPlayers})
}
