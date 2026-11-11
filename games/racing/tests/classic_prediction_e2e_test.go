package racing_test

// Before prediction integration: the browser must preserve the production
// vehicle's filtered inputs, gear, reverse timer and native road contact.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicPredictionProductionTrace(t *testing.T) {
	mod := game.Module()
	rows := []any{}
	for _, id := range classicVehicles {
		ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0)}
		m, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1, "vehicles": []string{id, "p911"}}))
		if err != nil {
			t.Fatal(err)
		}
		ctx.Now = ctx.Now.Add(3 * time.Second)
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		for step := 0; step < 35; step++ {
			before := view(t, mod, m, 0)
			steer := 0.0
			if step >= 10 && step < 20 {
				steer = .04
			}
			pressure := 0.0
			if step >= 22 && step < 28 {
				pressure = .3
			} else if step >= 28 {
				pressure = 1
			}
			input := map[string]any{"type": "drive", "epoch": before.Epoch, "seq": before.Cars[0].Seq + 1, "steer": steer, "throttle": .8, "brake": pressure > 0, "brakeAmount": pressure, "drift": false}
			ctx.Actor = 0
			m, err = mod.Apply(ctx, m, raw(input))
			if err != nil {
				t.Fatal(err)
			}
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			ctx.Actor = -1
			m, err = mod.OnTimer(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			after := view(t, mod, m, 0)
			rows = append(rows, map[string]any{"vehicle": id, "before": before.Cars[0], "after": after.Cars[0], "input": input, "ms": 100, "at": ctx.Now.UnixMilli()})
		}
	}
	dir := os.Getenv("WIRE_BOARD_RACING_PREDICTION_EVIDENCE")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", ".local", "racing-nfs-v3", "prediction")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(map[string]any{"scope": "production Module Start/Apply/OnTimer/View; native road and true filtered motion; no state injection", "rows": rows}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "production-trajectories.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
