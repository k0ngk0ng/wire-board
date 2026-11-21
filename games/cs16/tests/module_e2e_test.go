package cs16_test

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

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func encode(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
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

func artifact(t *testing.T, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "server")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, strings.ReplaceAll(t.Name(), "/", "-")+".json"), encode(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationAndPreparationTenSeats(t *testing.T) {
	module := game.Module()
	if module.MinPlayers != 2 || module.MaxPlayers != 10 || !module.Realtime || !module.RealtimeDeadlineFrames {
		t.Fatal("CS room or clock contract incomplete")
	}
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 1, 11, 32} {
		if _, err := module.Configure(count, nil); err == nil {
			t.Fatalf("accepted unsupported capacity %d", count)
		}
	}
	for _, raw := range []string{`{"map":"missing"}`, `{"roundSeconds":1}`, `{"roundSeconds":120.5}`, `{"maxRounds":0}`, `{"teams":["t","t"]}`, `{"teams":["t","ct"],"characters":["urban","terror"]}`, `{"health":1000}`, `{"map":null}`, `{"map":"de_dust2","map":"de_aztec"}`, `{} {}`} {
		if _, err := module.Configure(2, json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid configuration accepted: %s", raw)
		}
	}
	options, err := module.Configure(10, nil)
	if err != nil {
		t.Fatal(err)
	}
	var configured struct {
		Teams      []string `json:"teams"`
		Characters []string `json:"characters"`
	}
	if json.Unmarshal(options, &configured) != nil || len(configured.Teams) != 10 || len(configured.Characters) != 10 {
		t.Fatal("ten seats not configured", string(options))
	}
	counts := map[string]int{}
	for _, team := range configured.Teams {
		counts[team]++
	}
	if counts["t"] != 5 || counts["ct"] != 5 {
		t.Fatal("default teams not 5 versus 5", counts)
	}
	ctx := gameapi.PreparationContext{Actor: 1, Capacity: 10, Bots: []bool{false, false, true, true, true, true, true, true, true, true}}
	for _, action := range []any{map[string]any{"type": "map", "map": "cs_assault"}, map[string]any{"type": "character", "seat": 0, "character": "leet"}, map[string]any{"type": "team", "seat": 2, "team": "ct"}} {
		if _, err = module.Prepare(ctx, options, encode(action)); err == nil {
			t.Fatalf("unauthorized preparation accepted: %v", action)
		}
	}
	options, err = module.Prepare(ctx, options, encode(map[string]any{"type": "character", "character": "sas"}))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor, ctx.Host = 0, true
	options, err = module.Prepare(ctx, options, encode(map[string]any{"type": "map", "map": "cs_assault"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
		if _, err = module.Configure(10, encode(map[string]any{"map": id})); err != nil {
			t.Fatal(id, err)
		}
	}
	if _, err = module.Configure(10, encode(map[string]any{"roundSeconds": 300})); err != nil {
		t.Fatal("public five-minute option rejected", err)
	}
	artifact(t, map[string]any{"passed": true, "options": json.RawMessage(options), "teams": counts, "maps": []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"}})
}

func TestInputAuthorityProjectionAndReclaim(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 10, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := view(t, module, match, 0)
	player := before["players"].([]any)[0].(map[string]any)
	for _, field := range []string{"input", "previous", "remX", "remY", "remZ", "vx", "vy", "vz", "waterLevel"} {
		if player[field] == nil {
			t.Fatalf("self prediction state missing %s", field)
		}
	}
	action := map[string]any{"type": "input", "epoch": before["epoch"], "seq": 1, "life": player["life"], "moveX": 1000, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false, "viewFrame": before["frame"]}
	if !module.AcceptStale(ctx, match, encode(action)) {
		t.Fatal("valid independent realtime input rejected")
	}
	match, err = module.Apply(ctx, match, encode(action))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]any{"epoch": "old", "moveX": 1001, "yaw": 65536, "pitch": 16001, "life": 999, "seq": int64(999999999999999), "health": 1000, "viewFrame": int64(9999999)} {
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
		t.Fatal("spectator controlled seat")
	}
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	ctx.Actor = -1
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = 0
	afterApplied := view(t, module, match, 0)
	if integer(afterApplied["players"].([]any)[0].(map[string]any)["appliedSeq"]) != 1 {
		t.Fatal("input never applied")
	}
	match, err = module.Reclaim(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, module, match, 0)
	if after["epoch"] != before["epoch"] || after["startAt"] != before["startAt"] || after["frame"] != afterApplied["frame"] {
		t.Fatal("handoff reset match identity or clock")
	}
	public := view(t, module, match, 1)
	raw := string(encode(public))
	for _, field := range []string{`"pending"`, `"history"`, `"seed"`, `"planes"`, `"clipnodes"`, `"botGoal"`} {
		if strings.Contains(raw, field) {
			t.Fatalf("projection leaks %s", field)
		}
	}
	other := public["players"].([]any)[0].(map[string]any)
	if other["inventory"] != nil || other["money"] != nil {
		t.Fatal("other player's private inventory or money leaked")
	}
	if other["input"] != nil || other["previous"] != nil || other["remX"] != nil {
		t.Fatal("other player's prediction state leaked")
	}
	ctx.Automated = []bool{true, false, false, false, false, false, false, false, false, false}
	match, err = module.Reclaim(ctx, match)
	if err != nil || !match.HasComputerPlayers {
		t.Fatal("autoplay participation not latched", err)
	}
	ctx.Automated[0] = false
	match, err = module.Restore(ctx, match)
	if err != nil || !match.HasComputerPlayers {
		t.Fatal("restore lost computer participation", err)
	}
	artifact(t, map[string]any{"passed": true, "before": before, "after": after, "public": public, "computerParticipation": match.HasComputerPlayers})
}

func TestStreamCursorIsolationAndRecovery(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 10, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := module.StreamView(match, 0, "")
	if err != nil || first.Cursor == "" {
		t.Fatal("initial cursor missing", err)
	}
	before := sha256.Sum256(match.Data)
	repeated, err := module.StreamView(match, 0, first.Cursor)
	if err != nil || before != sha256.Sum256(match.Data) || !repeated.Unchanged || repeated.Data != nil || repeated.Cursor != first.Cursor {
		t.Fatal("projection mutated match", err)
	}
	broken, err := module.StreamView(match, 0, "other-epoch:99:999")
	if err != nil {
		t.Fatal(err)
	}
	var recovered map[string]any
	json.Unmarshal(encode(broken.Data), &recovered)
	if recovered["recover"] != true {
		t.Fatal("cursor gap did not request recovery")
	}
	snapshot, err := module.SyncSnapshot(match, 0)
	if err != nil {
		t.Fatal(err)
	}
	artifact(t, map[string]any{"passed": true, "initialCursor": first.Cursor, "repeatUnchanged": repeated.Unchanged, "repeat": repeated.Data, "gap": broken.Data, "snapshot": snapshot})
}

func TestPurchaseRejectsOldLifeAndRound(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 10, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	initial := view(t, module, match, 0)
	player := initial["players"].([]any)[0].(map[string]any)
	base := map[string]any{"type": "buy", "epoch": initial["epoch"], "life": player["life"], "round": initial["round"], "item": "secammo"}
	for _, field := range []string{"life", "round"} {
		missing := map[string]any{}
		for key, value := range base {
			if key != field {
				missing[key] = value
			}
		}
		if _, err = module.Apply(ctx, match, encode(missing)); err == nil {
			t.Fatal("purchase without generation accepted", field)
		}
		old := map[string]any{}
		for key, value := range base {
			old[key] = value
		}
		old[field] = 999
		if _, err = module.Apply(ctx, match, encode(old)); err == nil {
			t.Fatal("purchase from old generation accepted", field)
		}
	}
	match, err = module.Apply(ctx, match, encode(base))
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, module, match, 0)
	if integer(after["players"].([]any)[0].(map[string]any)["money"]) >= integer(player["money"]) {
		t.Fatal("purchase did not spend authoritative funds")
	}
	artifact(t, map[string]any{"passed": true, "initial": initial, "after": after})
}

func TestLongClockPauseUsesBoundedCatchup(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 10, Actor: -1, Now: time.Unix(100000, 0)}
	match, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	initial := view(t, module, match, 0)
	ctx.Now = ctx.Now.Add(11 * time.Second)
	ctx.Actor = 0
	bot, botErr := module.BotAction(ctx, match)
	if botErr != nil || len(bot) != 0 {
		t.Fatal("bot blocked clock catchup with an obsolete action", botErr, string(bot))
	}
	ctx.Actor = -1
	match, err = module.OnTimer(ctx, match)
	if err != nil || match.SystemDeadline <= ctx.Now.UnixMilli() {
		t.Fatal("late callback left deadline in the past", err, match.SystemDeadline)
	}
	lagged := view(t, module, match, 0)
	if integer(lagged["frame"]) > 1000 || lagged["catchingUp"] != true || lagged["canAct"] != false {
		t.Fatal("long catchup was not bounded", lagged["frame"])
	}
	player := lagged["players"].([]any)[0].(map[string]any)
	ctx.Actor = 0
	action := map[string]any{"type": "input", "epoch": lagged["epoch"], "seq": 1, "life": player["life"], "moveY": 1000, "yaw": 0, "pitch": 0, "viewFrame": lagged["frame"]}
	if _, err = module.Apply(ctx, match, encode(action)); err == nil {
		t.Fatal("active input entered obsolete world during catchup")
	}
	ctx.Actor = -1
	for step := 0; step < 32; step++ {
		ctx.Now = ctx.Now.Add(10 * time.Millisecond)
		match, err = module.OnTimer(ctx, match)
		if err != nil {
			t.Fatal(err)
		}
		if err = module.ValidateMatch(match, 10); err != nil {
			t.Fatal(err)
		}
		if view(t, module, match, 0)["catchingUp"] == false {
			break
		}
	}
	after := view(t, module, match, 0)
	if after["catchingUp"] != false || after["epoch"] != initial["epoch"] || after["startAt"] != initial["startAt"] {
		t.Fatal("catchup failed or reset match identity")
	}
	artifact(t, map[string]any{"passed": true, "initial": initial, "bounded": lagged, "after": after})
}

func TestNaturalTenSeatClassicMapMatches(t *testing.T) {
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
		t.Run(mapID, func(t *testing.T) {
			module := game.Module()
			ctx := gameapi.Context{Players: 10, Now: time.Unix(100000, 0), Automated: []bool{true, true, true, true, true, true, true, true, true, true}}
			match, err := module.Start(ctx, encode(map[string]any{"map": mapID, "roundSeconds": 60, "maxRounds": 2}))
			if err != nil {
				t.Fatal(err)
			}
			initial := json.RawMessage(append([]byte(nil), match.Data...))
			base := ctx.Now
			inputs, buys, peakBytes := 0, 0, 0
			projectionCount, totalProjectionBytes, peakStreamProjectionBytes, initialProjectionBytes := 0, 0, 0, 0
			type projectionSecond struct {
				Second int64 `json:"second"`
				Bytes  int   `json:"bytes"`
				Shots  int   `json:"shots"`
			}
			seconds := map[int64]*projectionSecond{}
			cursor := ""
			privateEventCursor := int64(0)
			for step := 1; step <= 9000 && match.Result == nil; step++ {
				ctx.Now = base.Add(time.Duration(step*30) * time.Millisecond)
				ctx.InputReceivedAt = ctx.Now
				for seat := range 10 {
					ctx.Actor = seat
					action, botErr := module.BotAction(ctx, match)
					if botErr != nil {
						t.Fatal(botErr)
					}
					if len(action) == 0 {
						continue
					}
					var payload struct {
						Type string `json:"type"`
					}
					if json.Unmarshal(action, &payload) != nil || payload.Type != "input" && payload.Type != "buy" {
						t.Fatal("bot bypassed public action", string(action))
					}
					match, err = module.Apply(ctx, match, action)
					if err != nil {
						t.Fatal(err, string(action))
					}
					if payload.Type != "buy" {
						inputs++
					}
				}
				ctx.Actor = -1
				match, err = module.OnTimer(ctx, match)
				if err != nil {
					t.Fatal(err)
				}
				packet, streamErr := module.StreamView(match, -1, cursor)
				if streamErr != nil {
					t.Fatal(streamErr)
				}
				cursor = packet.Cursor
				projected := packet.Data.(game.View)
				var persisted struct {
					World struct {
						EventID int64 `json:"eventId"`
						Events  []struct {
							ID   int64  `json:"id"`
							Kind string `json:"type"`
						} `json:"events"`
					} `json:"world"`
				}
				if json.Unmarshal(match.Data, &persisted) != nil {
					t.Fatal("persisted rule audit invalid")
				}
				for _, event := range persisted.World.Events {
					if event.ID > privateEventCursor && event.Kind == "buy" {
						buys++
					}
				}
				privateEventCursor = persisted.World.EventID
				projectionBytes := len(encode(projected))
				if projectionCount == 0 {
					initialProjectionBytes = projectionBytes
				}
				projectionCount++
				totalProjectionBytes += projectionBytes
				peakStreamProjectionBytes = max(peakStreamProjectionBytes, projectionBytes)
				second := (projected.ServerAt - projected.StartAt) / 1000
				if seconds[second] == nil {
					seconds[second] = &projectionSecond{Second: second}
				}
				seconds[second].Bytes += projectionBytes
				for _, event := range projected.Events {
					if event.Kind == "shot" {
						seconds[second].Shots++
					}
				}
				if step%100 == 0 {
					if err = module.ValidateMatch(match, 10); err != nil {
						t.Fatal(err)
					}
					peakBytes = max(peakBytes, len(encode(view(t, module, match, -1))))
				}
			}
			inputs = 0
			for seat := range 10 {
				own := view(t, module, match, seat)["players"].([]any)[seat].(map[string]any)
				inputs += int(integer(own["receivedSeq"]))
			}
			if match.Result == nil || inputs < 100 || buys < 10 || !match.Result.Team || len(match.Result.Ranks) != 10 || !match.HasComputerPlayers || peakBytes > 32000 {
				t.Fatal("incomplete ten-seat match", inputs, buys, peakBytes, match.Result)
			}
			final := view(t, module, match, -1)
			simulatedMs := integer(final["serverAt"]) - integer(final["startAt"])
			recovery, recoveryErr := module.SyncSnapshot(match, 0)
			if recoveryErr != nil {
				t.Fatal(recoveryErr)
			}
			secondRecords := []projectionSecond{}
			peakSecondBytes, fireSecondBytes, fireSeconds, shotEvents := 0, 0, 0, 0
			for second := int64(0); second <= simulatedMs/1000; second++ {
				if record := seconds[second]; record != nil {
					secondRecords = append(secondRecords, *record)
					peakSecondBytes = max(peakSecondBytes, record.Bytes)
					shotEvents += record.Shots
					if record.Shots > 0 {
						fireSecondBytes += record.Bytes
						fireSeconds++
					}
				}
			}
			kills := int64(0)
			for _, player := range final["players"].([]any) {
				kills += integer(player.(map[string]any)["kills"])
			}
			if match.Result.Draw && len(match.Result.Winners) != 0 {
				t.Fatal("draw represented as shared victory")
			}
			artifact(t, map[string]any{"passed": kills > 0, "scope": "production module full ten-seat match through ordinary QueueInput and Buy rules with accelerated wall clock only; projection byte metrics are JSON game-data bodies excluding platform envelope, compression and browser transport", "initial": initial, "inputs": inputs, "buys": buys, "kills": kills, "peakProjectionBytes": peakBytes, "streamProjectionCount": projectionCount, "streamProjectionBytes": totalProjectionBytes, "averageProjectionBytesPerSecond": int64(totalProjectionBytes) * 1000 / max(int64(1), simulatedMs), "peakStreamProjectionBytes": peakStreamProjectionBytes, "peakOneSecondProjectionBytes": peakSecondBytes, "initialProjectionBytes": initialProjectionBytes, "recoveryBytes": len(encode(recovery)), "shotEvents": shotEvents, "firingSecondCount": fireSeconds, "averageFiringSecondProjectionBytes": fireSecondBytes / max(1, fireSeconds), "projectionSeconds": secondRecords, "final": final, "result": match.Result})
			if kills == 0 {
				t.Fatal("computers completed clock without genuine combat")
			}
		})
	}
}

func TestAutomatedFrameEquivalentToOrdinaryActions(t *testing.T) {
	module := game.Module()
	ctx := gameapi.Context{Players: 10, Actor: -1, Now: time.Unix(100000, 0), Automated: []bool{true, true, true, true, true, true, true, true, true, true}}
	batched, err := module.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	ordinary := batched
	base := ctx.Now
	inputs, buys := 0, 0
	for step := 1; step <= 24; step++ {
		ctx.Now = base.Add(time.Duration(step*30) * time.Millisecond)
		manual := ctx
		manual.Automated = nil
		for seat := range 10 {
			manual.Actor = seat
			action, actionErr := module.BotAction(manual, ordinary)
			if actionErr != nil {
				t.Fatal(actionErr)
			}
			if len(action) > 0 {
				var payload struct {
					Type string `json:"type"`
				}
				if json.Unmarshal(action, &payload) != nil {
					t.Fatal("bot action not JSON")
				}
				if payload.Type == "buy" {
					buys++
				} else if payload.Type == "input" {
					inputs++
				} else {
					t.Fatal("bot bypassed public input or buy", payload.Type)
				}
				ordinary, err = module.Apply(manual, ordinary, action)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx.Actor = seat
			delegated, delegateErr := module.BotAction(ctx, batched)
			if delegateErr != nil || len(delegated) > 0 {
				t.Fatal("host automation still repeatedly decodes each seat", delegateErr, string(delegated))
			}
		}
		manual.Actor = -1
		ordinary, err = module.OnTimer(manual, ordinary)
		if err != nil {
			t.Fatal(err)
		}
		ctx.Actor = -1
		batched, err = module.OnTimer(ctx, batched)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(ordinary.Data, batched.Data) {
			t.Fatalf("ordinary and batched rule state differ at step %d", step)
		}
	}
	if inputs < 10 || buys < 10 {
		t.Fatal("equivalence scenario did not exercise inputs and purchases", inputs, buys)
	}
	ctx.Actor = 0
	batched, err = module.Reclaim(ctx, batched)
	if err != nil {
		t.Fatal(err)
	}
	before := view(t, module, batched, 0)["players"].([]any)[0].(map[string]any)
	ctx.Automated[0] = false
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(30 * time.Millisecond)
	batched, err = module.OnTimer(ctx, batched)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, module, batched, 0)["players"].([]any)[0].(map[string]any)
	if after["receivedSeq"] != before["receivedSeq"] || after["appliedSeq"] != before["appliedSeq"] {
		t.Fatal("batch advanced a reclaimed manual seat")
	}
	artifact(t, map[string]any{"passed": true, "scope": "24 production timer frames byte-identical to ordinary BotAction and Apply path, followed by manual handoff", "inputs": inputs, "buys": buys, "final": view(t, module, batched, 0)})
}
