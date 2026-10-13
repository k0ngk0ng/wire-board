package shengji_test

// Complete natural matches must traverse declaration, kitty, every trick,
// scoring, next-hand dealer and final results; own-hand-only bots may never
// stall, suggest an illegal follow, mutate their input or expose other hands.
import (
	"bytes"
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/shengji/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
func TestShanghaiNaturalMatchesE2E(t *testing.T) {
	for _, decks := range []int{1, 2} {
		for _, rounds := range []int{3, 0} {
			t.Run(fmt.Sprintf("%d-%d", decks, rounds), func(t *testing.T) {
				mod := game.Module()
				ctx := gameapi.Context{Players: 4, Actor: -1, Now: time.Unix(1700000000, 0)}
				o, e := mod.Configure(4, json.RawMessage(fmt.Sprintf(`{"decks":%d,"rounds":%d}`, decks, rounds)))
				if e != nil {
					t.Fatal(e)
				}
				m, e := mod.Start(ctx, o)
				if e != nil {
					t.Fatal(e)
				}
				trace := []any{map[string]any{"initialTestSnapshot": m.Data}}
				phases := map[string]int{}
				steps := 0
				for ; m.Result == nil && steps < 14000; steps++ {
					if e = mod.ValidateMatch(m, 4); e != nil {
						t.Fatal(e)
					}
					obs := view(t, m, -1)
					phase := obs["phase"].(string)
					phases[phase]++
					if len(obs["hand"].([]any)) != 0 {
						t.Fatal("spectator hand leak")
					}
					if _, ok := obs["hands"]; ok {
						t.Fatal("all hands exposed")
					}
					if phase != "round_end" && phase != "finished" && len(obs["bottom"].([]any)) > 0 {
						t.Fatal("spectator bottom leak")
					}
					if len(m.Actors) == 0 {
						t.Fatal("stalled", phase)
					}
					ctx.Actor = m.Actors[0]
					ctx.Now = ctx.Now.Add(100 * time.Millisecond)
					ctx.Version++
					before := append([]byte{}, m.Data...)
					a, err := mod.BotAction(ctx, m)
					if err != nil || len(a) == 0 {
						t.Fatal("bot stalled", err, phase)
					}
					next, err := mod.Apply(ctx, m, a)
					if err != nil {
						t.Fatalf("illegal bot %s in %s: %v", a, m.Data, err)
					}
					if !bytes.Equal(before, m.Data) {
						t.Fatal("mutated previous snapshot")
					}
					trace = append(trace, map[string]any{"actor": ctx.Actor, "action": json.RawMessage(a)})
					m = next
					if steps%29 == 0 {
						b, _ := json.Marshal(m)
						var restored gameapi.Match
						_ = json.Unmarshal(b, &restored)
						if !reflect.DeepEqual(view(t, m, ctx.Actor), view(t, restored, ctx.Actor)) {
							t.Fatal("restore")
						}
					}
				}
				if m.Result == nil {
					t.Fatal("whole match did not finish")
				}
				if e = m.Validate(4); e != nil {
					t.Fatal(e)
				}
				if len(m.Result.Winners) != 2 || !m.Result.Team {
					t.Fatal("bad partners result")
				}
				for _, p := range []string{"declare", "bury", "play", "round_end"} {
					if phases[p] == 0 {
						t.Fatal("missing phase", p)
					}
				}
				if rounds == 0 && view(t, m, 0)["finishReason"] != "passed_ace" {
					t.Fatal("did not play through A")
				}
				out := filepath.Join("..", "..", "..", ".local", "poker", "shengji")
				if e = os.MkdirAll(out, 0755); e != nil {
					t.Fatal(e)
				}
				b, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "decks": decks, "roundLimit": rounds, "steps": steps, "phases": phases, "trace": trace, "result": m.Result}, "", "  ")
				if e = os.WriteFile(filepath.Join(out, fmt.Sprintf("natural-%d-%d.json", decks, rounds)), b, 0600); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
