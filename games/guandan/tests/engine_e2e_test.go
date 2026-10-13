package guandan_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/guandan/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func view(t *testing.T, m gameapi.Match, p int) map[string]any {
	t.Helper()
	v, e := game.Module().View(m, p)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
func TestJiangsuNaturalMatchesE2E(t *testing.T) {
	for _, rounds := range []int{3, 0} {
		t.Run(fmt.Sprint(rounds), func(t *testing.T) {
			module := game.Module()
			o, e := module.Configure(4, json.RawMessage(fmt.Sprintf(`{"rounds":%d}`, rounds)))
			if e != nil {
				t.Fatal(e)
			}
			ctx := gameapi.Context{Players: 4, Actor: -1, Now: time.Now()}
			m, e := module.Start(ctx, o)
			if e != nil {
				t.Fatal(e)
			}
			trace := []any{map[string]any{"initialTestSnapshot": m.Data}}
			phases := map[string]int{}
			steps := 0
			for ; m.Result == nil && steps < 12000; steps++ {
				if e := module.ValidateMatch(m, 4); e != nil {
					t.Fatal(e)
				}
				obs := view(t, m, -1)
				if len(obs["hand"].([]any)) != 0 {
					t.Fatal("spectator hand leak")
				}
				if _, ok := obs["hands"]; ok {
					t.Fatal("all hands leaked")
				}
				phases[obs["phase"].(string)]++
				if len(m.Actors) == 0 {
					t.Fatal("live match stalled", string(m.Data))
				}
				ctx.Actor = m.Actors[0]
				ctx.Now = ctx.Now.Add(200 * time.Millisecond)
				ctx.Version++
				action, e := module.BotAction(ctx, m)
				if e != nil || len(action) == 0 {
					t.Fatal("bot stalled", e, string(m.Data))
				}
				next, e := module.Apply(ctx, m, action)
				if e != nil {
					t.Fatalf("illegal bot %s: %v at %s", action, e, m.Data)
				}
				trace = append(trace, map[string]any{"actor": ctx.Actor, "action": json.RawMessage(action)})
				m = next
			}
			if m.Result == nil {
				t.Fatal("full Jiangsu match did not finish")
			}
			if e := m.Validate(4); e != nil {
				t.Fatal(e)
			}
			if len(m.Result.Winners) != 2 || !m.Result.Team {
				t.Fatal("fixed partnership outcome incorrect")
			}
			// Anti-tribute may legitimately skip both phases. The deterministic
			// HTTP fixture separately verifies mandatory tribute and return.
			if rounds == 0 {
				v := view(t, m, 0)
				if v["finishReason"] != "passed_ace" {
					t.Fatal("long match was shortened instead of passing A", v["finishReason"])
				}
			}
			out := filepath.Join("..", "..", "..", ".local", "poker", "guandan")
			_ = os.MkdirAll(out, 0755)
			b, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "roundLimit": rounds, "steps": steps, "phases": phases, "trace": trace, "result": m.Result}, "", "  ")
			if e = os.WriteFile(filepath.Join(out, fmt.Sprintf("natural-%d.json", rounds)), b, 0600); e != nil {
				t.Fatal(e)
			}
		})
	}
}
