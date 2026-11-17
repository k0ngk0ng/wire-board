package racing_test

import (
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicCompetitiveThreeLapPace(t *testing.T) {
	for _, track := range classicTracks {
		t.Run(track, func(t *testing.T) {
			t.Parallel()
			mod := game.Module()
			ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0)}
			m, err := mod.Start(ctx, raw(map[string]any{"track": track, "laps": 3, "vehicles": []string{"mcf1", "mcf1"}}))
			if err != nil {
				t.Fatal(err)
			}
			initial := view(t, mod, m, -1)
			peak, distance := 0., 0.
			for _, route := range game.Tracks() {
				if route.ID == track {
					distance = route.Length / 10 * 3
				}
			}
			for steps := 0; m.Result == nil && steps < 12040; steps++ {
				m = tick(t, mod, &ctx, m, []int{0})
				v := view(t, mod, m, -1)
				for _, c := range v.Cars {
					peak = max(peak, c.Speed/10)
				}
			}
			final := view(t, mod, m, -1)
			seconds := float64(final.Cars[0].FinishAt-initial.StartAt) / 1000
			average := distance / seconds
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "track": track, "seconds": seconds, "averageMPS": average, "peakMPS": peak, "final": final, "scope": "three solo laps through production AI input/timer pipeline; the second authenticated seat remains manual and parked; no state/reset/result injection"})
			}()
			t.Logf("%s: %.2fs, mean %.2fm/s, peak %.2fm/s", track, seconds, average, peak)
			baselinePeak := map[string]float64{"france": 48.48, "coastal": 41.11, "uk": 36.10, "park": 33.95}
			if m.Result == nil || average < 29 || peak+.05 < baselinePeak[track]*1.15 {
				t.Fatal("computer failed the frozen competitive pace budget")
			}
			if final.Cars[0].Lap != 3 || final.Cars[0].FinishAt == 0 || final.Cars[0].LastReset != 0 {
				t.Fatal("computer did not finish three natural laps")
			}
			if final.Cars[1].Seq != 0 || final.Cars[1].Lap != 0 {
				t.Fatal("manual baseline seat was automatically taken over")
			}
		})
	}
}

func TestClassicCompetitiveSixCarCompletion(t *testing.T) {
	for _, track := range classicTracks {
		t.Run(track, func(t *testing.T) {
			t.Parallel()
			mod := game.Module()
			ctx := gameapi.Context{Players: 6, Now: time.Unix(100000, 0)}
			m, err := mod.Start(ctx, raw(map[string]any{"track": track, "laps": 5, "vehicles": []string{"mcf1", "mcf1", "mcf1", "mcf1", "mcf1", "mcf1"}}))
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; m.Result == nil && step < 12040; step++ {
				m = tick(t, mod, &ctx, m, []int{0, 1, 2, 3, 4, 5})
				if step%10 == 0 {
					v := view(t, mod, m, -1)
					done := true
					for _, car := range v.Cars {
						done = done && car.Lap >= 3
					}
					if done {
						break
					}
				}
			}
			final := view(t, mod, m, -1)
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "first three complete circuits of a normal five-lap six-car match; no finish deadline extension, state injection or reset", "final": final})
			}()
			for _, car := range final.Cars {
				if car.Lap < 3 || car.Lap > 4 || car.LastReset != 0 {
					t.Fatalf("six-car traffic failed natural completion: %+v", car)
				}
			}
		})
	}
}
