package racing_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicDynamicsProductionPrediction(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/racing-dynamics/evidence/prediction")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	rows := []any{}
	for _, track := range classicTracks {
		mod := game.Module()
		ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0), Actor: 0}
		m, err := mod.Start(ctx, raw(map[string]any{"track": track, "laps": 1, "vehicles": []string{"mcf1", "p911"}}))
		if err != nil {
			t.Fatal(err)
		}
		for step := 0; step < 3000; step++ {
			before := view(t, mod, m, 0)
			if before.Cars[0].FinishAt != 0 {
				break
			}
			ctx.Actor = 0
			action, err := mod.BotAction(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			m, err = mod.Apply(ctx, m, action)
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
			var input any
			if err := json.Unmarshal(action, &input); err != nil {
				t.Fatal(err)
			}
			if before.Phase == "racing" && after.Cars[0].FinishAt == 0 && before.Cars[0].Progress > 200 {
				rows = append(rows, map[string]any{"track": track, "before": before.Cars[0], "after": after.Cars[0], "props": before.Props, "input": input, "ms": 100, "at": ctx.Now.UnixMilli()})
			}
		}
	}
	fixture := filepath.Join(out, "production.json")
	data, _ := json.Marshal(rows)
	if err := os.WriteFile(fixture, data, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--experimental-strip-types", filepath.Join(root, "games/racing/tests/dynamics-prediction-parity.mjs"), fixture, filepath.Join(out, "report.json"))
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production prediction parity: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
