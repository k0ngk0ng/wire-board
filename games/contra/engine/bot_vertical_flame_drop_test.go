package contra

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Failure inventory: both seats walk into a visible/recent vertical fire
// column; no lower landing causes a blind drop; an accepted jump becomes
// Down+A; missing pixels immediately erase observed fire; stale scene history
// invents a current hazard. Fixtures contain RGBA-derived observations and
// complete public Bot memory, never native World fields.
func TestBotVerticalFlameUsesVisibleLowerRoute(t *testing.T) {
	f, err := os.Open("testdata/bot-coop-vertical-flame.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Cases []struct {
			Observation BotObservation
			Bot         Bot
			RecordedPad Buttons
		}
	}
	if err := json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	for _, c := range data.Cases {
		for _, mode := range []string{"visible", "recent-missing", "no-lower-landing", "jumping", "stale-missing"} {
			t.Run(fmt.Sprintf("seat%d/%s", c.Observation.Actor, mode), func(t *testing.T) {
				o, b := c.Observation, c.Bot
				o.Enemies = append([]BotEnemy(nil), o.Enemies...)
				p := o.Players[o.Actor]
				if mode == "recent-missing" || mode == "stale-missing" {
					kept := o.Enemies[:0]
					for _, e := range o.Enemies {
						if e.Family != "machine-flame" || e.Kind != 16 {
							kept = append(kept, e)
						}
					}
					o.Enemies = kept
					if mode == "stale-missing" {
						for i := range b.TrackSeenAt {
							b.TrackSeenAt[i] = o.Tick - 200
						}
					}
				}
				if mode == "no-lower-landing" {
					footRow := (p.Y + 16 + int(o.ViewY&15)) / 16
					for row := footRow + 1; row < len(o.Terrain); row++ {
						o.Terrain[row] = [17]TerrainKind{}
						o.TerrainKnown[row] = [17]bool{}
					}
				}
				if mode == "jumping" {
					o.Players[o.Actor].Sprite = 8
					b.JumpTargetActive, b.JumpTargetX, b.JumpCooldown = true, p.X+32, 9
				}
				if mode == "visible" || mode == "recent-missing" {
					probe := b
					known := probe.publicMemory(o)
					_, y, found := probe.planDropThrough(known)
					if !found || y <= p.Y {
						t.Fatal("fixture no longer has a visible lower landing")
					}
				}
				pad := b.Decide(o)
				drop := pad&(ButtonDown|ButtonA) == ButtonDown|ButtonA
				if mode == "visible" || mode == "recent-missing" {
					if !drop || pad&(ButtonLeft|ButtonRight) != 0 {
						t.Fatalf("vertical fire did not select neutral Down+A: got=%d recorded=%d", pad, c.RecordedPad)
					}
				} else if drop {
					t.Fatalf("unsafe Down+A in %s: %d", mode, pad)
				}
			})
		}
	}
}
