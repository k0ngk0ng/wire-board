package pal_test

// Test first against actual original joining checkpoints, never edited stats.
// Failures: physical-only bots lose the rescue battle; a bot controls another
// role; a pending command is duplicated; magic is fabricated or mana is not
// consumed; save/reclaim loses the selected role/animation or repeats rewards;
// reading after temporary party changes omits an original human. Existing
// takeover E2E separately covers unfinished human magic/item/misc cancellation.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func originalJoiningCheckpoint(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	b, err := os.ReadFile(campaignEvidencePath("first-party-joining-state.json", players))
	if err != nil {
		t.Fatal("run the original first-party E2E first", err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: players, Now: time.UnixMilli(v.ControlAt), Automated: make([]bool, players)}
	for i := range c.Automated {
		c.Automated[i] = true
	}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 1 {
		t.Fatal("checkpoint is not original first joining", v.Party)
	}
	return m, c, g
}

func TestOriginalRescueBattleBotsMagicAndSavedTakeover(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			m, c, g := originalJoiningCheckpoint(t, players)
			initial := view(t, m, g, 0)
			cash := initial.Cash
			commands := make([]int, players)
			magic, saved, reclaimed, battle := false, false, false, false
			for step := 0; step < 2048; step++ {
				v := view(t, m, g, 0)
				if v.Cash < cash {
					t.Fatal("original rescue bots lost and restarted", v.Scene, v.Cash, v.Party)
				}
				if v.Battle != nil {
					battle = true
					for _, p := range v.Party {
						for _, before := range initial.Party {
							if p.Role == before.Role && p.MP < before.MP {
								magic = true
							}
						}
					}
				}
				if magic && !saved && v.Battle != nil {
					before := v
					var err error
					g, err = m.SaveControl(c, g, true)
					if err != nil {
						t.Fatal(err)
					}
					if a, err := m.BotAction(c, g); err != nil || len(a) != 0 {
						t.Fatal("saved rescue bot kept casting", err)
					}
					c.Now = c.Now.Add(24 * time.Hour)
					m, _ = nativeModule(t)
					g, err = m.ResumeSaved(c, g)
					if err != nil {
						t.Fatal(err)
					}
					v = view(t, m, g, 0)
					if !reflect.DeepEqual(before.Battle, v.Battle) || !reflect.DeepEqual(before.Party, v.Party) || !reflect.DeepEqual(before.Raster, v.Raster) || before.Cash != v.Cash {
						t.Fatal("magic animation save changed original state")
					}
					c.Actor = v.Leader
					if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 32768})); err == nil {
						t.Fatal("saved rescue accepted old casting input")
					}
					saved = true
				}
				if v.Phase == "field" && battle {
					if !magic || !saved || !reclaimed || v.Cash != cash+96 || len(v.Party) != 2 {
						t.Fatal("rescue bot proof incomplete", magic, saved, reclaimed, v.Cash, v.Party)
					}
					for actor := 0; actor < min(players, 2); actor++ {
						if commands[actor] == 0 {
							t.Fatal("assigned seat did not act", actor)
						}
					}
					report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "production bot actions from unmodified actual original joining checkpoint through rescue victory and original permanent joining; cast mana, save during magic, manual reclaim, original shared reading and 96 cash reward; not whole campaign/autoplay", "cashBefore": cash, "cashAfter": v.Cash, "commands": commands, "magicManaSpent": magic, "magicSaveFreshRuntime": saved, "manualReclaim": reclaimed, "originalReadingParticipants": players}, "", "  ")
					if err := os.WriteFile(campaignEvidencePath("battle-bot-rescue.json", players), report, 0600); err != nil {
						t.Fatal(err)
					}
					return
				}
				if d := v.Dialogue; d != nil && len(d.Read) != players {
					t.Fatal("rescue reading lost original roster")
				}
				for actor := 0; actor < players; actor++ {
					c.Actor = actor
					before := bytes.Clone(g.Data)
					a, err := m.BotAction(c, g)
					if err != nil || !bytes.Equal(before, g.Data) {
						t.Fatal("rescue bot mutated input", err)
					}
					if len(a) == 0 {
						continue
					}
					own := view(t, m, g, actor)
					if own.Battle != nil && own.Battle.Current >= 0 && own.Battle.Current < len(own.Party) && own.Party[own.Battle.Current].Controller != actor {
						t.Fatal("rescue bot controls teammate", actor)
					}
					g, err = m.Apply(c, g, a)
					if err != nil {
						t.Fatal(err)
					}
					commands[actor]++
					if own.Battle != nil && !reclaimed && own.Battle.State == 1 && own.Battle.Menu == 0 {
						// Cancel a queued bot command before C consumes it, then
						// let the human defend through the ordinary shortcut.
						g, err = m.Reclaim(c, g)
						if err != nil {
							t.Fatal(err)
						}
						after := view(t, m, g, actor)
						if !reflect.DeepEqual(own.Battle, after.Battle) || !reflect.DeepEqual(own.Party, after.Party) || after.Seq != 0 {
							t.Fatal("manual reclaim cast the queued command")
						}
						g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: after.Epoch, Seq: 1, Direction: 4, Keys: 1024})
						reclaimed = true
						// Only subsequent production BotAction may cast magic.
					}
					if own.Battle != nil {
						if duplicate, err := m.BotAction(c, g); err != nil || len(duplicate) != 0 {
							t.Fatal("rescue bot duplicated pending battle input")
						}
					}
				}
				c.Now = time.UnixMilli(g.SystemDeadline + 200)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Fatal("original rescue battle bots did not finish")
		})
	}
}
