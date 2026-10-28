package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// A visible wall above a one-way platform requires the original Down+A drop
// when no forward higher jump exists. Solid support and missing lower landings
// must never be treated as a legal drop-through route.
func TestBotDropsThroughPlatformWhenHighWallBlocksProgress(t *testing.T) {
	f, e := os.Open("testdata/bot-energy-drop-wall.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var base BotObservation
	if e = json.NewDecoder(z).Decode(&base); e != nil {
		t.Fatal(e)
	}
	base.Players[0].X, base.Players[0].Y, base.Players[0].Sprite = 128, 68, 13
	clean := base.Enemies[:0:0]
	for _, enemy := range base.Enemies {
		if enemy.Family != "machine-flame" {
			clean = append(clean, enemy)
		}
	}
	base.Enemies = clean
	for _, mode := range []string{"platform", "solid", "no-landing"} {
		t.Run(mode, func(t *testing.T) {
			o := base
			if mode == "solid" {
				o.Terrain[(68+16+int(o.ViewY))/16][(128+int(o.ViewX))/16] = TerrainSolid
			}
			if mode == "no-landing" {
				for y := 6; y < 15; y++ {
					clear(o.Terrain[y][:])
					clear(o.TerrainKnown[y][:])
				}
			}
			b := Bot{LastX: 128, LastY: 68, ViewStillTicks: 1000}
			b.TrackSeenAt[7] = o.Tick - 10
			b.PreviousEnemies[7] = BotEnemy{ID: 7, X: 120, Y: 104, Kind: 17, Family: "machine-flame"}
			x, y, ok := b.planDropThrough(o)
			jx, jy, _, jok := b.planJump(o, false)
			t.Log("routes", x, y, ok, jx, jy, jok, "support", o.terrainAt(128, 84), o.terrainKnownAt(128, 84), "wall", o.terrainAt(144, 64))
			pad := b.play(o)
			drop := pad&(ButtonDown|ButtonA) == ButtonDown|ButtonA
			if mode == "platform" && (pad&(ButtonLeft|ButtonRight) != 0 || absBot(x-128) > 12) {
				t.Fatal("drop ignores original neutral input / initial fall steering lock", pad, x)
			}
			if mode == "platform" && !drop {
				t.Fatal("blocked actor cannot use visible lower platform", pad)
			}
			if mode != "platform" && drop {
				t.Fatal("invented drop-through route", mode, pad)
			}
		})
	}
}
