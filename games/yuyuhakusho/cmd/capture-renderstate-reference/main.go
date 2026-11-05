// Command capture-renderstate-reference freezes the existing client bridge.
// It supplies only new configuration/physical inputs, never original state.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/runtimebridge"
)

type configuration struct {
	Name          string  `json:"name"`
	Arena         uint16  `json:"arena"`
	Characters    []uint8 `json:"characters"`
	Team          bool    `json:"team"`
	HeldFixture   string  `json:"held_fixture,omitempty"`
	ReferencePath string  `json:"reference_path"`
}

func read(name string) []byte {
	b, e := os.ReadFile("games/yuyuhakusho/data/" + name + ".json.gz")
	if e != nil {
		panic(e)
	}
	z, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		panic(e)
	}
	defer z.Close()
	r, e := io.ReadAll(z)
	if e != nil {
		panic(e)
	}
	return r
}
func encoded(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func main() {
	path := flag.String("config", "", "new configuration JSON inside the worktree")
	flag.Parse()
	raw, e := os.ReadFile(*path)
	if e != nil {
		panic(e)
	}
	var c configuration
	if e = json.Unmarshal(raw, &c); e != nil {
		panic(e)
	}
	rules, sprites, hud := read("chu-world-rules"), read("world-sprites"), read("hud-assets")
	arenaName := "arena-assets-default"
	if c.Arena != 0 {
		arenaName = fmt.Sprintf("arena-assets-%x", c.Arena)
	}
	arena := read(arenaName)
	var aux []byte
	if c.Arena == 10 {
		aux = read("arena-a-aux-assets")
	}
	a, e := runtimebridge.DecodeAssetsForArena(rules, sprites, arena, hud, aux, c.Arena)
	if e != nil {
		panic(e)
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
	var fixed struct {
		Held [][8]engine.Buttons `json:"held"`
	}
	if c.HeldFixture != "" {
		b, e := os.ReadFile(c.HeldFixture)
		if e != nil {
			panic(e)
		}
		if e = json.Unmarshal(b, &fixed); e != nil {
			panic(e)
		}
	}
	s, e := runtimebridge.NewSession(options, a)
	if e != nil {
		panic(e)
	}
	if e = s.Ready(); e != nil {
		panic(e)
	}
	worldTrace, slotTrace, hudTrace, pixelTrace := sha256.New(), sha256.New(), sha256.New(), sha256.New()
	ticks, panFrames := 0, 0
	var final struct {
		World engine.ChuWorldState   `json:"world"`
		Slots presentation.TileSlots `json:"tile_slots"`
		HUD   presentation.HUDState  `json:"hud"`
	}
	for tick := 1; tick <= 20000; tick++ {
		var held [8]engine.Buttons
		if fixed.Held != nil {
			if tick > len(fixed.Held) {
				break
			}
			held = fixed.Held[tick-1]
		}
		f, e := s.Step(held)
		if e != nil {
			panic(fmt.Sprintf("%s tick%d: %v", c.Name, tick, e))
		}
		b, e := s.Save()
		if e != nil {
			panic(e)
		}
		if e = json.Unmarshal(b, &final); e != nil {
			panic(e)
		}
		worldTrace.Write(encoded(final.World))
		slotTrace.Write(encoded(final.Slots))
		hudTrace.Write(encoded(final.HUD))
		pixelTrace.Write(f.Pixels)
		ticks = tick
		if f.View.PanOffset != 0 {
			panFrames++
		}
		if f.View.Completed {
			break
		}
	}
	result := map[string]any{"configuration": c, "ticks": ticks, "complete": s.View().Completed, "pan_frames": panFrames,
		"world_trace_sha256": hex.EncodeToString(worldTrace.Sum(nil)), "slot_trace_sha256": hex.EncodeToString(slotTrace.Sum(nil)),
		"hud_trace_sha256": hex.EncodeToString(hudTrace.Sum(nil)), "pixel_trace_sha256": hex.EncodeToString(pixelTrace.Sum(nil)),
		"final_world_sha256": digest(encoded(final.World)), "final_slots_sha256": digest(encoded(final.Slots)), "final_hud_sha256": digest(encoded(final.HUD)),
		"reference_state_loaded": false, "pass": ticks > 0 && (s.View().Completed || fixed.Held != nil && ticks == len(fixed.Held))}
	if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
		panic(e)
	}
}
