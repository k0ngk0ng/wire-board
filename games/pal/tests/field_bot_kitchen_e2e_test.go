package pal_test

// Failures: doors/stairs loop; an inaccessible actor captures the plan; leaving
// before a conversational partner finishes walking hides the next NPC; a
// freshly given task is ignored because its item was searched earlier; saves
// lose exploration history, task intent or shared reading. No runtime hints.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalFieldBotFindsKitchenTray(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			originalFieldBotKitchen(t, players)
		})
	}
}

func originalFieldBotKitchen(t *testing.T, players int) {
	t.Helper()
	m, c, g := originalInnGuests(t, players)
	c.Automated = make([]bool, players)
	for i := range c.Automated {
		c.Automated[i] = true
	}
	last := uint64(0)
	commands, reads, saved, taskSaved := 0, 0, false, false
	instructionFrame := -1
	pages := []string{}
	for frame := 0; frame < 4096; frame++ {
		v := view(t, m, g, 0)
		if v.Phase == "field" && v.Field != nil && v.Field.Actors[0][2] == 208 {
			if v.Scene != 2 || v.Cash != 500 || !saved || !taskSaved || instructionFrame < 0 || frame-instructionFrame > 256 {
				t.Fatal("bot changed original kitchen rules or ignored the task", v.Scene, v.Cash, saved, taskSaved, frame, instructionFrame)
			}
			text := strings.Join(pages, "\n")
			for _, required := range []string{"那个臭要饭的赶走", "没酒喝我一步也走不动", "端上楼去"} {
				if !strings.Contains(text, required) {
					t.Fatal("bot did not follow the original interactions", required)
				}
			}
			report, err := json.MarshalIndent(map[string]any{
				"passed":  true,
				"scope":   "actual original inn guest checkpoint, ordinary public-view bot exploration through aunt, drunkard and kitchen tray; fresh-runtime saves retain movement history, visible task instruction and partial shared reading; not full campaign",
				"players": players, "fieldCommands": commands, "readingCommands": reads,
				"pages": pages, "scene": v.Scene, "cash": v.Cash, "partySprite": 208,
				"explorationHistorySaved": saved, "taskInstructionSaved": taskSaved,
				"frames": frame, "framesAfterInstruction": frame - instructionFrame,
			}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/field-bot-kitchen-%d.json", players)), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if d := v.Dialogue; d != nil && d.ID != last {
			last = d.ID
			pages = append(pages, d.Text)
			t.Log("kitchen bot dialogue", v.Scene, v.X, v.Y, d.Text)
			if !taskSaved && strings.Contains(d.Text, "桌上") && strings.Contains(d.Text, "酒菜") && strings.Contains(d.Text, "端") {
				instructionFrame = frame
				var plan struct {
					FieldBot struct {
						SearchItems bool `json:"searchItems"`
					} `json:"fieldBot"`
				}
				if err := json.Unmarshal(g.Data, &plan); err != nil || !plan.FieldBot.SearchItems {
					t.Fatal("visible task instruction was not retained", err)
				}
				c.Actor = 0
				if players > 1 {
					first, err := m.BotAction(c, g)
					if err != nil || len(first) == 0 {
						t.Fatal("first owner did not confirm the task", err)
					}
					g, err = m.Apply(c, g, first)
					if err != nil {
						t.Fatal(err)
					}
					reads++
				}
				before := view(t, m, g, 0)
				var err error
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
				after := view(t, m, g, 0)
				if !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || before.Cash != after.Cash {
					t.Fatal("saved task reading or original reward changed")
				}
				plan.FieldBot.SearchItems = false
				if err = json.Unmarshal(g.Data, &plan); err != nil || !plan.FieldBot.SearchItems {
					t.Fatal("saved visible task instruction was lost", err)
				}
				taskSaved = true
			}
		}
		for _, actor := range g.Actors {
			c.Actor = actor
			before := bytes.Clone(g.Data)
			action, err := m.BotAction(c, g)
			if err != nil || !bytes.Equal(before, g.Data) {
				t.Fatal("bot mutated source or failed", err)
			}
			if len(action) != 0 {
				var proposed pal.Action
				if err = json.Unmarshal(action, &proposed); err != nil || proposed.Type != "input" && proposed.Type != "read" {
					t.Fatal("bot bypassed original inputs", string(action), err)
				}
				if proposed.Type == "input" {
					commands++
				} else {
					reads++
				}
				g, err = m.Apply(c, g, action)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if frame%100 == 0 {
			v = view(t, m, g, 0)
			t.Log("kitchen bot position", frame, v.Scene, v.X, v.Y, v.Phase)
		}
		if frame >= 100 && !saved {
			var before struct {
				FieldBot json.RawMessage `json:"fieldBot"`
			}
			if err = json.Unmarshal(g.Data, &before); err != nil {
				t.Fatal(err)
			}
			var history struct {
				Walks []json.RawMessage `json:"walks"`
			}
			if err = json.Unmarshal(before.FieldBot, &history); err != nil || len(history.Walks) == 0 {
				t.Fatal("missing public movement observations", err)
			}
			g, err = m.SaveControl(c, g, true)
			if err != nil {
				t.Fatal(err)
			}
			m, _ = nativeModule(t)
			c.Now = c.Now.Add(time.Hour)
			g, err = m.ResumeSaved(c, g)
			if err != nil {
				t.Fatal(err)
			}
			var after struct {
				FieldBot json.RawMessage `json:"fieldBot"`
			}
			if err = json.Unmarshal(g.Data, &after); err != nil || !bytes.Equal(before.FieldBot, after.FieldBot) {
				t.Fatal("saved public exploration observations changed", err)
			}
			saved = true
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("bot did not reach original tray pickup", v.Scene, v.X, v.Y)
}
