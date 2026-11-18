package pal_test

// Test first: the bed passage or mountain transitions fail; chasing monsters
// are skipped or kill the party; original lesson animation traps; a saved
// reading/animation changes progress; sword magic is absent after the lesson.
// All routes are E2E input, never production navigation or state changes.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type nightLessonProof struct {
	MovieSaved                         bool
	MovieFrame                         int
	BedOpened, Escaped, Named, Learned bool
	ReadingSaved, AnimationSaved       bool
	Readings, Saves                    int
	LastPrompt                         uint64
}

func TestOriginalCampaignNightSwordLessonAndSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			m, c, g := originalReturnMedicine(t, players)
			runOriginalNightLesson(t, m, c, g, players, nightLessonProof{})
		})
	}
}

func runOriginalNightLesson(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof nightLessonProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"night-lesson-failure-state.json": g.Data, "night-lesson-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("night lesson lost partial reading")
				}
			}
		}
		before := v
		var err error
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
		v = view(t, m, g, 0)
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Cash != v.Cash || before.Night != v.Night || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Menu), raw(v.Menu)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("night lesson continuation changed original state")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("night lesson accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 6000; step++ {
		v := view(t, m, g, 0)
		movie := false
		if v.Raster != nil {
			for _, n := range v.Raster.Nodes {
				if n[0] == 10 && n[5] == 1 {
					movie = true
					proof.MovieFrame = max(proof.MovieFrame, n[7])
				}
			}
		}
		if len(v.Party) != 1 || v.Party[0].Role != 0 {
			t.Fatal("night lesson invented original party roles", v.Party)
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				if v.Scene == 12 {
					proof.Named = proof.Named || strings.Contains(d.Text, "酒剑仙")
				}
				t.Log("night lesson reading", proof.Readings, v.Scene, d.ID)
			}
			if v.Scene == 12 && !proof.ReadingSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				proof.ReadingSaved = true
			}
			if len(d.Read) != players {
				t.Fatal("night lesson changed original reading roster")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if movie && !proof.MovieSaved && proof.MovieFrame >= 55 {
			save(false)
			proof.MovieSaved = true
		} else if v.Scene == 12 && v.Phase == "script" && proof.ReadingSaved && !proof.AnimationSaved {
			save(false)
			proof.AnimationSaved = true
		} else if v.Phase == "menu" && v.Seq == v.AppliedSeq {
			c.Actor = v.Leader
			keys := uint32(1)
			if v.Menu.Kind == 1 && !proof.Learned {
				keys = 2
				if v.Menu.Selected < 2 {
					keys = 4
				} else if v.Menu.Selected > 2 {
					keys = 16
				}
			} else if v.Menu.Kind == 10 && !proof.Learned {
				if !strings.Contains(originalVisibleText(v, false), "御剑术") {
					t.Fatal("original lesson did not teach sword magic", originalVisibleText(v, false))
				}
				save(false)
				v = view(t, m, g, 0)
				proof.Learned = true
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			stage := 0
			if proof.BedOpened {
				stage++
			}
			if proof.Escaped {
				stage++
			}
			if proof.Learned {
				stage++
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("night lesson walkthrough repeats position", place)
			}
			if proof.Learned && v.Scene == 5 {
				if !proof.Escaped || !proof.Named || !proof.ReadingSaved || !proof.AnimationSaved || !proof.MovieSaved || proof.MovieFrame != 409 {
					t.Fatal("night lesson evidence incomplete", proof)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "original bed passage, mountain/temple sword lesson and original magic list, partial reading/script/menu fresh-runtime saves, ordinary return to village; not rescue/party joining/full campaign", "players": players, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "readings": proof.Readings, "saves": proof.Saves, "swordMagicLearned": proof.Learned, "partialReadingSaved": players > 1, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-night-lesson.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if proof.Named && !proof.Learned {
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 1})
			} else {
				x, y, reach := 512, 800, 32
				walker.allowTouch = v.Scene == 2 || v.Scene == 4 || v.Scene == 7 || v.Scene == 8
				switch v.Scene {
				case 2:
					if v.X > 1000 {
						x, y, reach = 1376, 272, 0
						if !proof.BedOpened {
							if v.X == 1360 && v.Y == 280 {
								c.Actor = v.Leader
								direction, keys := 2, uint32(16)
								if v.Field.Direction == 2 {
									direction, keys, proof.BedOpened = 4, 2, true
								}
								g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
							} else {
								g = walker.walk(t, m, &c, g, v, 1360, 280, 0)
							}
						} else {
							g = walker.walk(t, m, &c, g, v, x, y, reach)
						}
					} else {
						proof.Escaped = true
						g = walker.walk(t, m, &c, g, v, x, y, reach)
					}
				case 4:
					x, y = 976, 1704
				case 5:
					x, y = 128, 1344
				case 7:
					x, y = 160, 144
					if proof.Learned {
						x, y = 1872, 1944
					}
				case 8:
					x, y = 1072, 680
					if proof.Learned {
						x, y = 1504, 960
					}
				case 12:
					x, y = 736, 368
					if proof.Learned {
						x, y = 416, 528
					}
				default:
					t.Fatal("night lesson left original route", v.Scene, v.X, v.Y)
				}
				if v.Scene != 2 {
					g = walker.walk(t, m, &c, g, v, x, y, reach)
				}
			}
		} else if v.Battle != nil {
			for _, actor := range g.Actors {
				c.Actor = actor
				a, err := m.BotAction(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if len(a) > 0 {
					g, err = m.Apply(campaignBotContext(c), g, a)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("night lesson position", step, v.Scene, v.X, v.Y, v.Phase)
		}
	}
	t.Fatal("original night sword lesson did not finish", proof)
	return m, c, g
}
