package pal_test

// Before continuous autoplay: plans work only from hand-prepared checkpoints,
// an earlier empty item probe suppresses a later reward, conversation history
// captures the wrong actor, doors loop, or saved partial reading/held movement
// loses an earlier quest. No walkthrough positions or script IDs drive play.
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
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalContinuousAutoplayThroughWineQuest(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			m, _ := nativeModule(t)
			c := gameapi.Context{Players: players, Now: time.Unix(1800000000, 0), Automated: make([]bool, players)}
			for i := range c.Automated {
				c.Automated[i] = true
			}
			g, err := m.Start(c, raw(map[string]any{}))
			if err != nil {
				t.Fatal(err)
			}
			pages := []string{}
			last := uint64(0)
			moves, reads, saves := 0, 0, 0
			tray, delivered, wine, promised := false, false, false, false
			for frame := 0; frame < 8192; frame++ {
				v := view(t, m, g, 0)
				if v.Field != nil && v.Field.Actors[0][2] == 208 {
					tray = true
				}
				if d := v.Dialogue; d != nil && d.ID != last {
					last = d.ID
					pages = append(pages, d.Text)
					t.Log("continuous dialogue", frame, v.Scene, v.X, v.Y, d.Text)
					delivered = delivered || strings.Contains(d.Text, "酒菜送来了")
					wine = wine || strings.Contains(d.Text, "桂花酒") && strings.Contains(d.Text, "收在怀里")
					promised = promised || strings.Contains(d.Text, "山神庙")
				}
				if promised && v.Phase == "field" {
					if !tray || !delivered || !wine || v.Cash != 500 || saves < 2 {
						t.Fatal("continuous bot skipped original plot", tray, delivered, wine, v.Cash, saves)
					}
					for _, actor := range v.Field.Actors {
						if actor[0] == 1 && actor[1] == 63 {
							t.Fatal("original drunkard did not leave")
						}
					}
					report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "continuous original GameMain from Start through opening, inn guest reward, tray pickup/delivery, wine receipt/use and temple promise; only ordinary bot actions and timers, with fresh-runtime saves; not full campaign", "players": players, "frames": frame, "fieldCommands": moves, "readingCommands": reads, "saves": saves, "pages": pages, "cash": v.Cash, "trayPickedUp": tray, "trayDelivered": delivered, "wineReceived": wine, "templePromised": promised}, "", "  ")
					if err = os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/continuous-wine-%d.json", players)), report, 0600); err != nil {
						t.Fatal(err)
					}
					return
				}
				if (moves >= 24 && saves == 0) || (wine && v.Dialogue != nil && saves == 1) {
					if saves == 1 && players > 1 {
						c.Actor = 0
						g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
						partial := view(t, m, g, 0).Dialogue
						if partial == nil || !partial.Read[0] || partial.Read[1] {
							t.Fatal("continuous save did not contain partial shared reading")
						}
					}
					before := view(t, m, g, 0)
					g, err = m.SaveControl(c, g, true)
					if err != nil {
						t.Fatal(err)
					}
					c.Now = c.Now.Add(24 * time.Hour)
					m, _ = nativeModule(t)
					g, err = m.ResumeSaved(c, g)
					if err != nil {
						t.Fatal(err)
					}
					after := view(t, m, g, 0)
					if before.Scene != after.Scene || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) {
						t.Fatal("continuous checkpoint changed on save")
					}
					saves++
				}
				for _, actor := range g.Actors {
					c.Actor = actor
					before := bytes.Clone(g.Data)
					a, err := m.BotAction(c, g)
					if err != nil || !bytes.Equal(before, g.Data) {
						t.Fatal("continuous bot mutated source or failed", err)
					}
					if len(a) == 0 {
						continue
					}
					var action pal.Action
					if err = json.Unmarshal(a, &action); err != nil || action.Type != "read" && action.Type != "input" {
						t.Fatal("continuous bot bypassed normal controls", string(a), err)
					}
					if action.Type == "read" {
						reads++
					} else {
						moves++
					}
					g, err = m.Apply(c, g, a)
					if err != nil {
						t.Fatal(err)
					}
				}
				c.Now = time.UnixMilli(g.SystemDeadline)
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if frame%250 == 0 {
					v = view(t, m, g, 0)
					t.Log("continuous position", frame, v.Scene, v.X, v.Y, v.Phase, moves, reads)
				}
			}
			if err = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/continuous-wine-failure-state.json"), g.Data, 0600); err != nil {
				t.Fatal(err)
			}
			v := view(t, m, g, 0)
			t.Fatal("continuous autoplay failed", v.Scene, v.X, v.Y, tray, delivered, wine, promised)
		})
	}
}
