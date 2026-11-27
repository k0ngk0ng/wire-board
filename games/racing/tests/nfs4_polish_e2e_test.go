package racing_test

import (
	"encoding/json"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicCanonicalTrackNamesConsistentAcrossAllFiveTracks(t *testing.T) {
	expected := map[string]string{
		"france":  "法国乡间公路",
		"coastal": "海豚湾",
		"uk":      "凯尔特遗迹",
		"park":    "金迪亚克公园",
		"germany": "德国乡间公路",
	}
	tracks := game.Tracks()
	seen := map[string]bool{}
	mod := game.Module()
	now := time.Unix(1_700_000_000, 0)
	for _, track := range tracks {
		if track.Version != 3 {
			continue
		}
		want, ok := expected[track.ID]
		if !ok {
			t.Fatalf("unexpected v3 track id %q", track.ID)
		}
		seen[track.ID] = true
		if track.Name != want {
			t.Fatalf("Tracks() %s name = %q, want %q", track.ID, track.Name, want)
		}
		m, err := mod.Start(gameapi.Context{Players: 2, Now: now}, raw(map[string]any{"track": track.ID, "laps": 1}))
		if err != nil {
			t.Fatalf("start %s: %v", track.ID, err)
		}
		v := view(t, mod, m, 0)
		if v.Track.Name != want {
			t.Fatalf("View %s name = %q, want %q", track.ID, v.Track.Name, want)
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("expected %d v3 tracks, saw %d", len(expected), len(seen))
	}
}

func TestRacingEightPlayersAndNumericLapsOneToTen(t *testing.T) {
	mod := game.Module()
	if mod.MinPlayers != 2 || mod.MaxPlayers != 8 {
		t.Fatalf("module players = %d..%d, want 2..8", mod.MinPlayers, mod.MaxPlayers)
	}
	for _, laps := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		b, err := mod.Configure(8, raw(map[string]any{"track": "park", "laps": laps}))
		if err != nil {
			t.Fatalf("Configure 8 players laps=%d failed: %v", laps, err)
		}
		var opt game.Options
		if err := json.Unmarshal(b, &opt); err != nil {
			t.Fatal(err)
		}
		if opt.Laps != laps || len(opt.Vehicles) != 8 {
			t.Fatalf("unexpected options %+v", opt)
		}
	}
	for _, invalidLaps := range []int{-1, 0, 11, 99} {
		if _, err := mod.Configure(4, raw(map[string]any{"track": "park", "laps": invalidLaps})); err == nil {
			t.Fatalf("Configure accepted invalid laps=%d", invalidLaps)
		}
	}
	for _, invalidSeats := range []int{0, 1, 9, 16} {
		if _, err := mod.Configure(invalidSeats, raw(map[string]any{"track": "park", "laps": 3})); err == nil {
			t.Fatalf("Configure accepted invalid seats=%d", invalidSeats)
		}
	}
	options, err := mod.Configure(8, raw(map[string]any{"track": "park", "laps": 2}))
	if err != nil {
		t.Fatal(err)
	}
	bots := []bool{false, true, true, true, true, true, true, true}
	options, err = mod.Prepare(gameapi.PreparationContext{Actor: 7, Capacity: 8, Bots: bots}, options, raw(map[string]any{"type": "vehicle", "vehicle": "mclk"}))
	if err != nil {
		t.Fatalf("seat 7 prepare failed: %v", err)
	}
	ctx := gameapi.Context{Players: 8, Now: time.Unix(1_700_000_000, 0)}
	m, err := mod.Start(ctx, options)
	if err != nil {
		t.Fatalf("start 8 players failed: %v", err)
	}
	vRaw, err := mod.View(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(vRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 8192 {
		t.Fatalf("8-player dynamic view size %d exceeds 8192 bytes", len(encoded))
	}
	v := view(t, mod, m, 0)
	if len(v.Cars) != 8 {
		t.Fatalf("expected 8 cars, got %d", len(v.Cars))
	}
	if v.Cars[7].Vehicle != "mclk" {
		t.Fatalf("seat 7 vehicle = %q, want mclk", v.Cars[7].Vehicle)
	}
	for i := 0; i < 70; i++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		for seat := 0; seat < 8; seat++ {
			ctx.Actor = seat
			act, err := mod.BotAction(ctx, m)
			if err != nil {
				t.Fatalf("bot seat %d step %d: %v", seat, i, err)
			}
			if len(act) > 0 {
				m, err = mod.Apply(ctx, m, act)
				if err != nil {
					t.Fatalf("apply seat %d step %d: %v", seat, i, err)
				}
			}
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatalf("timer step %d: %v", i, err)
		}
	}
	after := view(t, mod, m, 0)
	if after.Phase != "racing" {
		t.Fatalf("phase = %q, want racing", after.Phase)
	}
	moved := 0
	for _, car := range after.Cars {
		if car.Speed > 5 {
			moved++
		}
	}
	if moved < 8 {
		t.Fatalf("expected all 8 cars accelerating, got %d", moved)
	}
}
