package pal_test

// Test first: the original night/morning or farewell cannot continue; bundle
// selection/use loses state, repeats its 500 cash, misses gifts or fails to
// consume the bundle; Ling'er rejoining steals Li's owner; a missing role drops
// a human reader; unauthorized/stale controls act; sailing or arrival cannot
// resume in a fresh runtime. Walkthrough scripts/coordinates stay test-only.
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

type departureProof struct {
	Night, Morning, Farewell, BundleGiven, BundleUsed, Opened, InventoryChecked, Joined, Boat, Arrived bool
	NightSaved, MorningSaved, SelectedSaved, GrantSaved, JoinedSaved, BoatSaved, SailingSaved          bool
	UnauthorizedChecked                                                                                bool
	Readings, Saves, Scans, MenuWait                                                                   int
	LastPrompt                                                                                         uint64
}

func TestOriginalCampaignFarewellBundleAndSuzhouSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalDeparture(t, players)
		})
	}
}

func originalDeparture(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalAmbush(t, players)
	return runOriginalDeparture(t, m, c, g, players, departureProof{})
}

func runOriginalDeparture(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof departureProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	startCash := view(t, m, g, 0).Cash
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	lastFieldScene := view(t, m, g, 0).Scene
	balcony, downstairs := false, false
	roomDoorOpened := false
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"departure-failure-state.json": g.Data, "departure-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), b, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	owners := func(v pal.View) {
		t.Helper()
		seen := map[int]bool{}
		for _, p := range v.Party {
			want := 0
			if p.Role == 1 && players > 1 {
				want = 1
			}
			if p.Role < 0 || p.Role > 1 || seen[p.Role] || p.Controller != want {
				t.Fatal("departure changed original role owners", v.Party)
			}
			seen[p.Role] = true
		}
		if !seen[0] || len(v.Party) > 2 {
			t.Fatal("departure invented/lost original roles", v.Party)
		}
	}
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("departure partial reading changed")
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
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Cash != v.Cash || before.Leader != v.Leader || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Menu), raw(v.Menu)) || !bytes.Equal(raw(before.Field), raw(v.Field)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("departure save changed original story/menu/resources")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("departure accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 5000; step++ {
		v := view(t, m, g, 0)
		v = view(t, m, g, v.Leader)
		owners(v)
		if v.Cash != startCash && v.Cash != startCash+500 {
			t.Fatal("bundle repeated or changed original money", startCash, v.Cash)
		}
		if v.Cash == startCash+500 {
			proof.Opened = true
			if !proof.GrantSaved {
				save(v.Dialogue != nil)
				proof.GrantSaved = true
				v = view(t, m, g, v.Leader)
			}
		}
		if len(v.Party) == 2 && proof.BundleGiven {
			proof.Joined = true
			if !proof.JoinedSaved {
				save(v.Dialogue != nil)
				proof.JoinedSaved = true
				v = view(t, m, g, v.Leader)
			}
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Night = proof.Night || strings.Contains(d.Text, "当夜")
				proof.Morning = proof.Morning || strings.Contains(d.Text, "次日")
				proof.Farewell = proof.Farewell || strings.Contains(d.Text, "带灵儿去苗疆")
				proof.BundleGiven = proof.BundleGiven || strings.Contains(d.Text, "自己不会打开来看看")
				proof.Boat = proof.Boat || strings.Contains(d.Text, "我这艘船只到苏州")
				proof.Arrived = proof.Arrived || v.Scene == 22 && strings.Contains(d.Text, "祝你们一路顺风")
				t.Log("departure reading", proof.Readings, v.Scene, d.ID, len(v.Party))
			}
			if proof.Night && !proof.NightSaved {
				save(true)
				proof.NightSaved = true
			} else if proof.Morning && !proof.MorningSaved {
				save(true)
				proof.MorningSaved = true
			} else if proof.Boat && !proof.BoatSaved {
				save(true)
				proof.BoatSaved = true
			}
			v = view(t, m, g, v.Leader)
			d = v.Dialogue
			if len(d.Read) != players {
				t.Fatal("departure lost original human readers")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if proof.Boat && !proof.SailingSaved && v.Phase == "script" && v.Scene == 6 {
			save(false)
			proof.SailingSaved = true
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			c.Actor = v.Leader
			keys := uint32(32)
			proof.Scans++
			if proof.Scans > 128 {
				t.Fatal("original bundle inventory did not settle", originalVisibleText(v, false))
			}
			if proof.Opened {
				proof.MenuWait++
				if proof.MenuWait < 3 {
					keys = 0
				} else {
					complete := !strings.Contains(originalVisibleText(v, false), "包袱")
					for name, amount := range map[string]int{"金创药": 2, "还神丹": 2, "绣花鞋": 1, "玉镯": 1, "铁剑": 1, "手卷": 1} {
						n, ok := campaignInventoryAmount(v, name)
						complete = complete && ok && n >= amount
					}
					if complete {
						proof.InventoryChecked = true
						keys = 1
					}
				}
			} else if strings.Contains(originalVisibleText(v, true), "包袱") {
				if n, ok := campaignInventoryAmount(v, "包袱"); !ok || n != 1 {
					t.Fatal("farewell bundle amount changed", n, ok)
				}
				if !proof.SelectedSaved {
					save(false)
					proof.SelectedSaved = true
					v = view(t, m, g, v.Leader)
				}
				keys, proof.BundleUsed = 2, true
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if v.Scene != lastFieldScene {
				if v.Scene == 4 {
					balcony, downstairs = false, lastFieldScene == 3 || lastFieldScene == 5
				}
				lastFieldScene = v.Scene
			}
			if players > 1 && !proof.UnauthorizedChecked {
				c.Actor = (v.Leader + 1) % players
				other := view(t, m, g, c.Actor)
				if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: other.Epoch, Seq: other.Seq + 1, ControlAt: other.ControlAt, Direction: 4, Keys: 2})); err == nil {
					t.Fatal("departure allowed another seat's exploration input")
				}
				proof.UnauthorizedChecked = true
			}
			if proof.BundleGiven && !proof.InventoryChecked {
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			} else if v.Scene == 22 && proof.Arrived {
				if !proof.NightSaved || !proof.MorningSaved || !proof.Farewell || !proof.SelectedSaved || !proof.GrantSaved || !proof.JoinedSaved || !proof.BoatSaved || !proof.SailingSaved || !proof.InventoryChecked || !proof.BundleUsed || len(v.Party) != 2 || v.Cash != startCash+500 {
					t.Fatal("original departure incomplete", proof, v.Party)
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-departure-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "actual original post-ambush night/morning, aunt farewell, bundle opening and all six displayed gifts, Ling'er rejoining and original Suzhou ferry; fresh-runtime saves and original readers/owners; not full campaign/autoplay", "scene": v.Scene, "map": v.Map, "cashBefore": startCash, "cash": v.Cash, "party": v.Party, "readings": proof.Readings, "saves": proof.Saves, "bundleConsumed": proof.InventoryChecked, "originalGiftMoneyOnce": true, "selectedBundleSaved": proof.SelectedSaved, "originalReaders": players, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-departure.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			} else {
				stage := 0
				for _, done := range []bool{proof.Night, proof.Morning, proof.BundleGiven, proof.Joined} {
					if done {
						stage++
					}
				}
				place := [4]int{v.Scene, v.X, v.Y, stage}
				visits[place]++
				if visits[place] > 48 {
					t.Fatal("departure walkthrough repeats position", place, proof)
				}
				x, y, reach := 512, 800, 0
				handled := false
				switch v.Scene {
				case 2:
					if v.X > 1000 {
						x, y = 1152, 384
					} else if v.Y < 500 {
						x, y = 416, 400 // Ling'er's room opens onto the inn balcony.
					} else if v.Y <= 640 {
						x, y = 656, 616
						if v.X == x && v.Y == y {
							c.Actor = v.Leader
							g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 1, Keys: 8 | 2})
							g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
							handled = true
						}
					}
				case 3:
					if proof.Joined {
						x, y = 1184, 784
					} else if proof.InventoryChecked {
						// Approach Ling'er directly; a broad search from the
						// northwest would collect the nearby hidden 50 cash.
						if v.X == 1456 && v.Y == 728 {
							c.Actor = v.Leader
							direction, keys := 2, uint32(16)
							if v.Field.Direction == 2 {
								direction, keys = 4, 2
							}
							g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
						} else {
							g = walker.walk(t, m, &c, g, v, 1456, 728, 0)
						}
					} else {
						g = campaignWalkTo(t, m, &c, g, v, 1440, 736, 39)
					}
				case 4:
					x, y = 1328, 1448
					if v.X == 1296 && v.Y == 1368 {
						balcony = true
					}
					if balcony && v.X >= 1200 && v.Y >= 1456 {
						downstairs = true
					}
					if !downstairs && !balcony {
						x, y = 1296, 1368
					} else if !downstairs {
						x, y = 1168, 1368
					} else if proof.Joined {
						x, y = 976, 1704
					} else if !proof.Morning {
						g = campaignWalkTo(t, m, &c, g, v, 1088, 1648, 57)
						handled = true
					} else {
						for _, actor := range v.Field.Actors {
							if actor[0] == 1 && actor[1] == 71 && actor[5] > 0 {
								roomDoorOpened = true
							}
						}
						if !roomDoorOpened {
							// The original double door opens by search input;
							// its frame changes before the entrance is passable.
							g = campaignWalkTo(t, m, &c, g, v, 1296, 1448, 71)
							handled = true
						}
					}
				case 5:
					x, y = 1824, 1328
				case 6:
					g = campaignWalkTo(t, m, &c, g, v, 1168, 1208, 126)
				default:
					t.Fatal("departure left original route", v.Scene, v.X, v.Y)
				}
				if !handled && v.Scene != 6 && (v.Scene != 3 || proof.Joined) {
					g = walker.walk(t, m, &c, g, v, x, y, reach)
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
			t.Log("departure position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	t.Fatal("original Suzhou departure never finished", proof)
	return m, c, g
}
