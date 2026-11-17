package racing_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicExpandedContentConfiguration(t *testing.T) {
	mod := game.Module()
	vehicles := []string{"mclk", "bmw5", "jxkr", "p911", "fm50", "mcf1"}
	for n := 2; n <= 6; n++ {
		options, err := mod.Configure(n, raw(map[string]any{"track": "germany", "laps": 3, "vehicles": vehicles[:n]}))
		if err != nil {
			t.Fatal(err)
		}
		for seat := 0; seat < n; seat++ {
			for _, id := range []string{"bmw5", "jxkr", "mclk"} {
				prepared, err := mod.Prepare(gameapi.PreparationContext{Actor: seat, Capacity: n}, options, raw(map[string]any{"type": "vehicle", "vehicle": id}))
				if err != nil {
					t.Fatal(err)
				}
				var after game.Options
				if err := json.Unmarshal(prepared, &after); err != nil || after.Vehicles[seat] != id {
					t.Fatal("new vehicle selection was not retained", err)
				}
			}
		}
		m, err := mod.Start(gameapi.Context{Players: n, Now: time.Unix(100000, 0)}, options)
		if err != nil {
			t.Fatal(err)
		}
		v := view(t, mod, m, -1)
		if v.Track.ID != "germany" || v.Physics != 3 || len(v.Cars) != n {
			t.Fatal("expanded choices did not enter the real race")
		}
		classicTrace(t, map[string]any{"passed": true, "scope": "production Configure/Prepare/Start/View for expanded content", "players": n, "view": v})
	}
}

func TestClassicGermanyCompleteRaceModes(t *testing.T) {
	for players := 2; players <= 6; players++ {
		for _, laps := range []int{1, 3, 5} {
			t.Run(fmt.Sprintf("%d-seats-%d-laps", players, laps), func(t *testing.T) {
				t.Parallel()
				mod, ctx, m := begin(t, players, laps, "germany")
				bots := make([]int, players)
				for i := range bots {
					bots[i] = i
				}
				for step := 0; m.Result == nil && step < 12040; step++ {
					m = tick(t, mod, &ctx, m, bots)
				}
				v := view(t, mod, m, -1)
				defer func() {
					classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "new original Germany course, all public seat/lap modes, ordinary computer inputs only", "players": players, "laps": laps, "final": v, "result": m.Result})
				}()
				if m.Result == nil || v.ServerAt >= v.MaxRaceAt {
					t.Fatal("new course failed natural race completion")
				}
				finished := 0
				for _, car := range v.Cars {
					if car.LastReset != 0 || car.Progress <= 0 {
						t.Fatal("new course required a recovery shortcut", car)
					}
					if car.FinishAt > 0 {
						finished++
					}
				}
				if finished == 0 {
					t.Fatal("no car finished the new original course")
				}
			})
		}
	}
}

func TestClassicExpandedFleetGermanyRestoreAndFinish(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 6, Now: time.Unix(100000, 0)}
	m, err := mod.Start(ctx, raw(map[string]any{"track": "germany", "laps": 3, "vehicles": []string{"bmw5", "jxkr", "mclk", "mcf1", "fm50", "p911"}}))
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; m.Result == nil && step < 12040; step++ {
		m = tick(t, mod, &ctx, m, []int{0, 1, 2, 3, 4, 5})
		if step == 500 {
			before := append([]byte{}, m.Data...)
			m, err = mod.Restore(ctx, m)
			if err != nil || string(before) != string(m.Data) {
				t.Fatal("expanded fleet restore changed the race", err)
			}
		}
	}
	v := view(t, mod, m, -1)
	defer func() {
		classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "three new and three existing models, natural Germany race, byte-stable restore", "final": v, "result": m.Result})
	}()
	if m.Result == nil || v.ServerAt >= v.MaxRaceAt {
		t.Fatal("expanded fleet never finished")
	}
	for _, car := range v.Cars {
		if car.LastReset != 0 || car.Lap < 2 {
			t.Fatal("expanded fleet stopped making normal progress", car)
		}
	}
}

func TestClassicNewVehiclesCompleteOriginalTracks(t *testing.T) {
	for _, track := range []string{"france", "germany"} {
		for _, vehicle := range []string{"bmw5", "jxkr", "mclk"} {
			t.Run(track+"-"+vehicle, func(t *testing.T) {
				t.Parallel()
				mod := game.Module()
				ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0)}
				m, err := mod.Start(ctx, raw(map[string]any{"track": track, "laps": 1, "vehicles": []string{vehicle, "p911"}}))
				if err != nil {
					t.Fatal(err)
				}
				for step := 0; m.Result == nil && step < 12040; step++ {
					m = tick(t, mod, &ctx, m, []int{0})
				}
				v := view(t, mod, m, -1)
				defer func() {
					classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "ordinary drive-only new vehicle on original track; parked manual peer; no reset or injected result", "final": v})
				}()
				if v.Cars[0].Vehicle != vehicle || v.Cars[0].Lap != 1 || v.Cars[0].FinishAt == 0 || v.Cars[0].LastReset != 0 || v.ServerAt >= v.MaxRaceAt {
					t.Fatal("new vehicle did not naturally finish", v.Cars[0])
				}
			})
		}
	}
}
