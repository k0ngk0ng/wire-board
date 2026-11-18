package pal_test

// Before field strategy: bots stall after the opening, drive another seat,
// flood pending inputs, loop through the bedroom door, miss the original guest
// storyline, or lose a bounded visible-history plan on save. Opening checkpoint
// is prepared through ordinary original reading, never by editing native state.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalFieldBotsReachInnGuestReward(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			m, _ := nativeModule(t)
			c := gameapi.Context{Players: players, Now: time.Unix(1800000000, 0), Automated: make([]bool, players)}
			for i := range c.Automated {
				c.Automated[i] = true
			}
			g := originalInn(t, m, &c)
			moves, reads, saved, historySaved := 0, 0, false, false
			pages := []string{}
			lastPrompt := uint64(0)
			for frame := 0; frame < 2048; frame++ {
				v := view(t, m, g, 0)
				if v.Dialogue != nil && v.Dialogue.ID != lastPrompt {
					lastPrompt = v.Dialogue.ID
					pages = append(pages, v.Dialogue.Text)
					t.Log("bot dialogue", v.Scene, v.X, v.Y, v.Dialogue.Text)
				}
				if v.Cash == 500 && v.Phase == "field" {
					if moves == 0 || reads == 0 || !saved || !historySaved || v.Scene != 4 || len(v.Party) != 1 {
						t.Fatal("bots bypassed original cooperative exploration", moves, reads, saved, historySaved, v.Scene)
					}
					report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original inn checkpoint prepared by ordinary opening reading, public-view field bots and cooperative reading through guest reward, save/fresh-runtime continuation of movement and populated visible history; not full campaign bots", "players": players, "fieldCommands": moves, "readingCommands": reads, "pages": pages, "cash": v.Cash, "populatedHistorySaved": historySaved}, "", "  ")
					if err := os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/field-bots-inn-%d.json", players)), report, 0600); err != nil {
						t.Fatal(err)
					}
					return
				}
				for _, actor := range slices.Clone(g.Actors) {
					if !slices.Contains(g.Actors, actor) {
						continue
					}
					c.Actor = actor
					own := view(t, m, g, actor)
					before := bytes.Clone(g.Data)
					action, err := m.BotAction(c, g)
					if err != nil || !bytes.Equal(before, g.Data) {
						t.Fatal("field bot mutated source or failed", err)
					}
					if len(action) == 0 {
						if own.Phase == "field" && own.Seq == own.AppliedSeq {
							t.Fatal("field bot has no ordinary exploration action", own.Scene, own.X, own.Y)
						}
						continue
					}
					var proposed pal.Action
					if err = json.Unmarshal(action, &proposed); err != nil || proposed.Type != "read" && proposed.Type != "input" {
						t.Fatal("field bot bypassed ordinary actions", string(action), err)
					}
					g, err = m.Apply(c, g, action)
					if err != nil {
						t.Fatal("field bot action rejected", string(action), err)
					}
					if proposed.Type == "input" {
						moves++
					} else {
						reads++
					}
					if proposed.Type == "input" && !saved && moves >= 4 {
						g, err = m.SaveControl(c, g, true)
						if err != nil {
							t.Fatal(err)
						}
						if idle, err := m.BotAction(c, g); err != nil || len(idle) != 0 {
							t.Fatal("saving field bot still acts", err)
						}
						c.Now = c.Now.Add(time.Hour)
						m, _ = nativeModule(t)
						g, err = m.ResumeSaved(c, g)
						if err != nil {
							t.Fatal(err)
						}
						saved = true
					}
					if proposed.Type == "read" && saved && !historySaved {
						var before struct {
							FieldBot json.RawMessage `json:"fieldBot"`
						}
						var history struct {
							Recent []json.RawMessage `json:"recent"`
						}
						if err = json.Unmarshal(g.Data, &before); err != nil {
							t.Fatal(err)
						}
						if err = json.Unmarshal(before.FieldBot, &history); err != nil || len(history.Recent) == 0 {
							t.Fatal("bot did not retain public conversation observations", err)
						}
						g, err = m.SaveControl(c, g, true)
						if err != nil {
							t.Fatal(err)
						}
						c.Now = c.Now.Add(time.Hour)
						m, _ = nativeModule(t)
						g, err = m.ResumeSaved(c, g)
						if err != nil {
							t.Fatal(err)
						}
						var after struct {
							FieldBot json.RawMessage `json:"fieldBot"`
						}
						if err = json.Unmarshal(g.Data, &after); err != nil || !bytes.Equal(before.FieldBot, after.FieldBot) {
							t.Fatal("saved public conversation history was reset", err)
						}
						historySaved = true
					}
				}
				var err error
				c.Now = time.UnixMilli(g.SystemDeadline)
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if frame%100 == 0 {
					v := view(t, m, g, 0)
					t.Log("bot position", frame, v.Scene, v.X, v.Y, moves, reads)
				}
			}
			v := view(t, m, g, 0)
			t.Fatal("original field bots did not reach guest reward", v.Scene, v.X, v.Y, moves, reads)
		})
	}
}

func TestOriginalFieldBotMemoryRejectsMalformedSave(t *testing.T) {
	m, _ := nativeModule(t)
	c := gameapi.Context{Players: 1, Now: time.Unix(1800000000, 0)}
	g := originalInn(t, m, &c)
	tooManyRecent := make([]map[string]any, 17)
	for i := range tooManyRecent {
		tooManyRecent[i] = map[string]any{"scene": 1, "npc": i + 1, "frame": 0}
	}
	for _, recent := range []any{
		[]map[string]any{{"scene": 0, "npc": 1, "frame": 0}},
		[]map[string]any{{"scene": 2, "npc": -1, "frame": 0}},
		[]map[string]any{{"scene": 2, "npc": 1, "frame": -1}},
		[]map[string]any{{"scene": 2, "npc": 1, "frame": 0}, {"scene": 2, "npc": 1, "frame": 1}},
		tooManyRecent,
	} {
		var stored map[string]json.RawMessage
		if err := json.Unmarshal(g.Data, &stored); err != nil {
			t.Fatal(err)
		}
		stored["fieldBot"] = raw(map[string]any{"recent": recent})
		bad := g
		bad.Data = raw(stored)
		if _, err := m.View(bad, 0); err == nil {
			t.Fatal("malformed saved bot history accepted", recent)
		}
	}
	if _, err := m.View(g, 0); err != nil {
		t.Fatal("matching-build save without bot history no longer reads", err)
	}
	// New public exploration history must remain bounded and reject invalid
	// saved positions, including both ends of a previously observed transition.
	tooManyLinks := make([]map[string]int, 33)
	for i := range tooManyLinks {
		tooManyLinks[i] = map[string]int{"scene": 1, "x": i * 128, "y": 0}
	}
	tooManyWalks := make([]map[string]int, 257)
	for i := range tooManyWalks {
		tooManyWalks[i] = map[string]int{"scene": 1, "x": i * 32, "y": 0}
	}
	for _, memory := range []map[string]any{
		{"last": map[string]int{"scene": 0, "x": 0, "y": 0}},
		{"last": map[string]int{"scene": 1, "x": 131071, "y": 0}},
		{"links": []map[string]int{{"scene": 1, "x": 0, "y": -65537}}},
		{"links": tooManyLinks},
		{"walks": []map[string]int{{"scene": 65536, "x": 0, "y": 0}}},
		{"walks": tooManyWalks},
		{"searchItems": "yes"},
		{"wine": map[string]any{"wanted": true}},
		{"wine": map[string]any{"scene": 65536}},
		{"wine": map[string]any{"scene": 4, "npc": -1}},
		{"wine": map[string]any{"moves": -1}},
		{"wine": map[string]any{"moves": 513}},
		{"wine": map[string]any{"pending": 2}},
		{"wine": map[string]any{"settled": 3}},
		{"wine": map[string]any{"cursor": strings.Repeat("x", 257)}},
		{"wine": map[string]any{"menu": true}},
		{"wine": map[string]any{"available": true}},
		{"wine": map[string]any{"wanted": "yes"}},
		// Navigation is saved public history: malformed nodes/edges must not
		// turn a restored bot into an unbounded traversal or an invalid index.
		{"nav": []map[string]int{{"scene": 0, "x": 0, "y": 0}}},
		{"nav": []map[string]int{{"scene": 2, "x": 0, "y": 0}, {"scene": 2, "x": 0, "y": 0}}},
		{"edges": [][2]int{{0, 1}}},
		{"nav": []map[string]int{{"scene": 2, "x": 0, "y": 0}}, "edges": [][2]int{{0, 0}}},
		{"navLast": map[string]int{"scene": 2, "x": 0, "y": 0}},
		{"nav": []map[string]int{{"scene": 2, "x": 0, "y": 0}}, "navLast": map[string]int{"scene": 2, "x": 16, "y": 8}},
		{"still": 9},
		{"wine": map[string]any{"scene": 4, "anchor": map[string]int{"scene": 2, "x": 0, "y": 0}}},
	} {
		var stored map[string]json.RawMessage
		if err := json.Unmarshal(g.Data, &stored); err != nil {
			t.Fatal(err)
		}
		stored["fieldBot"] = raw(memory)
		bad := g
		bad.Data = raw(stored)
		if _, err := m.View(bad, 0); err == nil {
			t.Fatal("malformed saved public exploration accepted", memory)
		}
	}
}
