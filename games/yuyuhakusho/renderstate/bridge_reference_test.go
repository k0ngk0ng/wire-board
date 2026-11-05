package renderstate_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

type bridgeReference struct {
	Configuration struct {
		Name          string
		Arena         uint16
		Characters    []uint8
		Team          bool
		HeldFixture   string `json:"held_fixture"`
		ReferencePath string `json:"reference_path"`
	}
	Ticks      int
	Complete   bool
	PanFrames  int    `json:"pan_frames"`
	WorldTrace string `json:"world_trace_sha256"`
	SlotTrace  string `json:"slot_trace_sha256"`
	HUDTrace   string `json:"hud_trace_sha256"`
	PixelTrace string `json:"pixel_trace_sha256"`
	FinalWorld string `json:"final_world_sha256"`
	FinalSlots string `json:"final_slots_sha256"`
	FinalHUD   string `json:"final_hud_sha256"`
}

// Only this temporary pixel view materializes client catalog tiles. Persistent
// server/client state remains refs, slots, and the canonical world cache owner.
func referencePixels(t *testing.T, world engine.ChuWorldState, prior, next renderstate.State, events []engine.ChuWorldEvent, sprites presentation.Catalog, hud presentation.HUDCatalog, arena presentation.ArenaCatalog, aux *presentation.ArenaAAuxCatalog) []byte {
	t.Helper()
	frame := presentation.BeginFrame(presentation.TileSlots(prior.SpriteSlots), 2)
	for _, event := range events {
		var err error
		if event.Kind == "arena-sprite" {
			frame, err = presentation.ComposeArenaASprite(frame, *event.RawArenaSprite)
		} else if event.Kind == "draw" {
			frame, _, err = presentation.ComposeActor(frame, presentation.Actor(*event.Draw), sprites)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if frame.Slots != presentation.TileSlots(next.SpriteSlots) {
		t.Fatal("temporary pixel view disagrees with sole persistent metadata slots")
	}
	palette := presentation.CompactPalette(world.Palette.Current)
	spriteRaster, err := presentation.RasterizeSpritesForArena(frame, sprites, aux, palette, 320, 224, false)
	if err != nil {
		t.Fatal(err)
	}
	window, err := presentation.HUDWindow(pixelHUD(t, next.HUD, hud))
	if err != nil {
		t.Fatal(err)
	}
	var horizontal, vertical [224][2]uint16
	pan := [2][224]int16{world.PanRows(0), world.PanRows(1)}
	for y := range horizontal {
		vertical[y] = [2]uint16{uint16(world.Vertical[0]), uint16(world.Vertical[1])}
		for plane, source := range world.ScrollSources {
			if source == -0xa000 && plane == 0 || source == -0x9e00 && plane == 1 {
				horizontal[y][plane] = uint16(pan[plane][y])
			} else {
				if source < -0x1a00 || source > -0x15c0 || source&1 != 0 {
					t.Fatal("unowned reference scene source")
				}
				horizontal[y][plane] = world.Cells[int(source+0x1a00)/2+y]
			}
		}
	}
	rows, err := presentation.ArenaRowsForArena(world.Match.Arena, world.BackdropColor&63, palette, horizontal, vertical)
	if err != nil {
		t.Fatal(err)
	}
	background, err := presentation.RasterizeArena(arena, presentation.ArenaFrame{Width: 320, Height: 224, Rows: rows[:], Window: &window})
	if err != nil {
		t.Fatal(err)
	}
	screen, err := presentation.CompositeArenaSprites(background, spriteRaster)
	if err != nil {
		t.Fatal(err)
	}
	return screen.RGBA().Pix
}

func TestRenderFrozenBridgeTrajectories(t *testing.T) {
	var fixture struct{ Rows []bridgeReference }
	fixtureRaw := raw(t, "testdata/bridge-reference.json")
	if e := json.Unmarshal(fixtureRaw, &fixture); e != nil {
		t.Fatal(e)
	}
	var manifest struct {
		Pass  bool
		Cases int
		SHA   string `json:"fixture_sha256"`
	}
	if e := json.Unmarshal(raw(t, "testdata/bridge-reference-manifest.json"), &manifest); e != nil {
		t.Fatal(e)
	}
	if !manifest.Pass || manifest.Cases != 49 || len(fixture.Rows) != 49 || sha(fixtureRaw) != manifest.SHA {
		t.Fatal("unbound original bridge reference")
	}
	l, sprites, hud := libraries(t)
	rules, e := engine.DecodeChuWorldRules(raw(t, "../data/chu-world-rules.json.gz"))
	if e != nil {
		t.Fatal(e)
	}
	shard, count := 0, 1
	if os.Getenv("RENDERSTATE_SHARDS") != "" {
		count, e = strconv.Atoi(os.Getenv("RENDERSTATE_SHARDS"))
		if e != nil || count < 1 {
			t.Fatal("bad shard count")
		}
		shard, e = strconv.Atoi(os.Getenv("RENDERSTATE_SHARD"))
		if e != nil || shard < 0 || shard >= count {
			t.Fatal("bad shard")
		}
	}
	for index, row := range fixture.Rows {
		if index%count != shard {
			continue
		}
		t.Run(row.Configuration.Name, func(t *testing.T) {
			c := row.Configuration
			arenaName := "arena-assets-default"
			if c.Arena != 0 {
				arenaName = fmt.Sprintf("arena-assets-%x", c.Arena)
			}
			arena, e := presentation.DecodeArenaCatalogForArena(raw(t, "../data/"+arenaName+".json.gz"), c.Arena)
			if e != nil {
				t.Fatal(e)
			}
			var aux *presentation.ArenaAAuxCatalog
			if c.Arena == 10 {
				a, e := presentation.DecodeArenaAAuxCatalog(raw(t, "../data/arena-a-aux-assets.json.gz"))
				if e != nil {
					t.Fatal(e)
				}
				aux = &a
			}
			options := engine.ChuSessionOptions{Arena: c.Arena, Seed: 0x12345678, Difficulty: 6, TeamBattle: c.Team}
			slots := []uint8{0, 3}
			if len(c.Characters) == 3 {
				slots = []uint8{0, 1, 3}
			} else if len(c.Characters) == 4 {
				slots = []uint8{0, 1, 2, 3}
			}
			for i, id := range c.Characters {
				options.Players = append(options.Players, engine.ChuSessionPlayer{Slot: slots[i], CharacterID: id, Color: uint8(i), Computer: c.HeldFixture == "", HumanChannel: uint8(i)})
			}
			world, events, e := engine.NewChuSession(options, rules)
			if e != nil {
				t.Fatal(e)
			}
			state, e := renderstate.New(l)
			if e != nil {
				t.Fatal(e)
			}
			world, state, e = renderstate.Advance(world, state, events, l)
			if e != nil {
				t.Fatal(e)
			}
			resource, e := engine.ChuSessionResourceForArena(c.Arena, rules)
			if e != nil {
				t.Fatal(e)
			}
			world, e = engine.CompleteChuSessionResource(world, resource, rules)
			if e != nil {
				t.Fatal(e)
			}
			var fixed struct {
				Held [][8]engine.Buttons `json:"held"`
			}
			if c.HeldFixture != "" {
				if e = json.Unmarshal(raw(t, "../../../"+c.HeldFixture), &fixed); e != nil {
					t.Fatal(e)
				}
			}
			wt, st, ht, pt := sha256.New(), sha256.New(), sha256.New(), sha256.New()
			panFrames, restores := 0, 0
			for tick := 1; tick <= row.Ticks; tick++ {
				var held [8]engine.Buttons
				if fixed.Held != nil {
					held = fixed.Held[tick-1]
				}
				world, events, _, e = engine.StepChuSessionFrame(world, held, rules)
				if e != nil {
					t.Fatal(tick, e)
				}
				prior := state
				world, state, e = renderstate.Advance(world, state, events, l)
				if e != nil {
					t.Fatal(tick, e)
				}
				pixels := referencePixels(t, world, prior, state, events, sprites, hud, arena, aux)
				wt.Write(jsonBytes(world))
				st.Write(jsonBytes(presentation.TileSlots(state.SpriteSlots)))
				ht.Write(jsonBytes(pixelHUD(t, state.HUD, hud)))
				pt.Write(pixels)
				if world.Clock.PanOffset != 0 {
					panFrames++
				}
				if tick == 500 || tick == 1056 && world.Clock.PanOffset != 0 {
					saved := jsonBytes(struct {
						World  engine.ChuWorldState
						Render renderstate.State
					}{world, state})
					var next struct {
						World  engine.ChuWorldState
						Render renderstate.State
					}
					if e = json.Unmarshal(saved, &next); e != nil || !reflect.DeepEqual(next.World, world) || !reflect.DeepEqual(next.Render, state) {
						t.Fatal("pixel-free checkpoint lost state", tick, e)
					}
					if e = renderstate.Validate(next.World, next.Render, l); e != nil {
						t.Fatal(e)
					}
					world, state = next.World, next.Render
					restores++
				}
			}
			got := []string{hex.EncodeToString(wt.Sum(nil)), hex.EncodeToString(st.Sum(nil)), hex.EncodeToString(ht.Sum(nil)), hex.EncodeToString(pt.Sum(nil)), sha(jsonBytes(world)), sha(jsonBytes(presentation.TileSlots(state.SpriteSlots))), sha(jsonBytes(pixelHUD(t, state.HUD, hud)))}
			want := []string{row.WorldTrace, row.SlotTrace, row.HUDTrace, row.PixelTrace, row.FinalWorld, row.FinalSlots, row.FinalHUD}
			if !reflect.DeepEqual(got, want) || world.MatchComplete != row.Complete || panFrames != row.PanFrames || restores == 0 {
				t.Fatalf("frozen complete legacy bridge differs source=%s\ngot=%v\nwant=%v pan=%d/%d restores=%d", c.ReferencePath, got, want, panFrames, row.PanFrames, restores)
			}
			t.Logf("source=%s ticks=%d complete=%v pan=%d restores=%d", c.ReferencePath, row.Ticks, row.Complete, panFrames, restores)
		})
	}
}
