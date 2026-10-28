package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Before visible-flame ducking: standing or jumping into the current beam
// kills an otherwise grounded actor. Another row, an unidentified source and
// a beam that has visibly vanished must not lock the actor in a crouch.
func TestBotVisibleNozzleBeamCrouchesOnlyAcrossCurrentBody(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-nozzle-landing.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var base BotObservation
	if err = json.NewDecoder(z).Decode(&base); err != nil {
		t.Fatal(err)
	}
	base.Players[0].X, base.Players[0].Y, base.Players[0].Sprite = 106, 148, 13
	for _, mode := range []string{"crossing", "other-row", "no-source", "vanished"} {
		t.Run(mode, func(t *testing.T) {
			o := base
			o.Enemies = append([]BotEnemy(nil), base.Enemies...)
			flame := BotEnemy{ID: -1, X: 90, Y: 136, Kind: 17, Family: "machine-flame"}
			if mode == "other-row" {
				flame.Y = 184
			}
			if mode == "no-source" {
				o.Enemies = nil
			}
			if mode != "vanished" {
				o.Enemies = append(o.Enemies, flame)
			}
			b := Bot{LastX: 106, LastY: 148, ViewStillTicks: 1000}
			// Keep the jump-opening prerequisite satisfied; this test isolates
			// whether the current beam crosses the standing body.
			b.TrackSeenAt[7] = o.Tick - 10
			b.PreviousEnemies[7] = BotEnemy{ID: 7, X: 120, Y: 136, Kind: 17, Family: "machine-flame"}
			pad := b.play(o)
			if mode == "crossing" {
				if pad&ButtonDown == 0 || pad&(ButtonA|ButtonLeft|ButtonRight) != 0 {
					t.Fatal("did not crouch under visible overhead beam", pad)
				}
			} else if pad&ButtonDown != 0 {
				t.Fatal("non-crossing evidence locked actor in crouch", mode, pad)
			}
		})
	}
}

// A nozzle is a hazard landmark, not a shootable target. Aiming at a lower
// nozzle while jumping would aim at a non-target. Neutral Down+A would also
// select drop-through; with Left/Right held the original still selects a jump.
func TestBotNozzleLandmarksDoNotAimBelowWhileJumping(t *testing.T) {
	f, err := os.Open("testdata/bot-energy-nozzle-landing.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var o BotObservation
	if err = json.NewDecoder(z).Decode(&o); err != nil {
		t.Fatal(err)
	}
	o.Enemies = append(o.Enemies, BotEnemy{ID: -1, X: 140, Y: 184, Kind: 17, Family: "machine-flame"})
	for x := 108; x <= 114; x++ {
		o.Players[0].X, o.Players[0].Y, o.Players[0].Sprite = x, 148, 13
		b := Bot{LastX: x, LastY: 148, ViewStillTicks: 1000}
		pad := b.play(o)
		if pad&ButtonA != 0 && pad&ButtonDown != 0 {
			t.Fatal("static nozzle diverted jump aim toward a non-target", x, pad)
		}
	}
}
