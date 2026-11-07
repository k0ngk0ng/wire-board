//go:build ignore

// Replays an explicitly life-assisted diagnostic; never used by a bot policy.
// Recorded pads and declared life grants are the only replay interventions.
package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	c "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func main() {
	input, out := os.Args[1], os.Args[2]
	start, e := strconv.Atoi(os.Args[3])
	must(e)
	end, e := strconv.Atoi(os.Args[4])
	must(e)
	must(os.MkdirAll(out, 0755))
	f, e := os.Open(filepath.Join(input, "pads.bin.gz"))
	must(e)
	z, e := gzip.NewReader(f)
	must(e)
	pads, e := io.ReadAll(z)
	must(e)
	must(z.Close())
	must(f.Close())
	if end < start || start < 1 || end*2 > len(pads) {
		panic("invalid replay bounds")
	}
	var grants []struct {
		Frame         uint64
		Actor         int
		Before, After uint8
	}
	raw, e := os.ReadFile(filepath.Join(input, "test-life-refills.json"))
	must(e)
	must(json.Unmarshal(raw, &grants))
	grant := 0
	f, e = os.Open("games/contra/engine/testdata/bot-visual-catalog.json.gz")
	must(e)
	z, e = gzip.NewReader(f)
	must(e)
	var art struct {
		Catalog c.BotVisualCatalogManifest
		RGB     []byte
		Pages   map[uint8][]byte
	}
	must(json.NewDecoder(z).Decode(&art))
	must(z.Close())
	must(f.Close())
	catalog, e := c.NewVerifiedBotVisualCatalog(art.Catalog, art.RGB, func(id uint8) ([]uint8, error) { return art.Pages[id], nil })
	must(e)
	bundle, e := c.BuiltinLevelBundle()
	must(e)
	vision, e := c.NewBuiltinBotVision(catalog, bundle)
	must(e)
	session, e := c.NewSession(42, 1, "bot-standard")
	must(e)
	applyGrants := func() {
		for grant < len(grants) && grants[grant].Frame == session.Random.Frames {
			g := grants[grant]
			if g.Actor != 0 || session.World.Active.Combat.Players[g.Actor].Lives != g.Before {
				panic("declared life intervention no longer matches")
			}
			session.World.Active.Combat.Players[g.Actor].Lives = g.After
			if g.Frame == 0 {
				session.PresentedLives[g.Actor] = g.After
			}
			grant++
		}
	}
	applyGrants()
	var observations []any
	for n := 1; n <= end; n++ {
		if n >= start {
			hud := c.BotPublicHUD{Seats: 1, Paused: c.BotPublicBool{Known: true, Value: session.World.Campaign.Paused}, Players: [2]c.BotPublicPlayer{{Occupied: true, Lives: c.BotPublicNumber{Known: true, Value: session.PresentedLives[0]}}}}
			frame, e := c.NewBotVisualFrame(session.World.Active.Presentation.Snapshot(), hud, catalog, 0, uint64(n))
			must(e)
			observation, e := c.ObserveBotFrame(frame, vision)
			must(e)
			observations = append(observations, map[string]any{"frame": n, "observation": observation})
			f, e := os.Create(filepath.Join(out, fmt.Sprintf("frame-%d.png", n)))
			must(e)
			must(png.Encode(f, &image.RGBA{Pix: frame.Pixels(), Stride: 1024, Rect: image.Rect(0, 0, 256, 240)}))
			must(f.Close())
		}
		stage, e := bundle.Stage(session.World.Active.Combat.World.Level)
		must(e)
		_, e = session.StepPads(uint64(n), [2]c.Buttons{c.Buttons(pads[(n-1)*2]), c.Buttons(pads[(n-1)*2+1])}, stage)
		must(e)
		applyGrants()
	}
	raw, e = json.MarshalIndent(observations, "", "  ")
	must(e)
	must(os.WriteFile(filepath.Join(out, "observations.json"), raw, 0644))
	raw, e = json.Marshal(session)
	must(e)
	must(os.WriteFile(filepath.Join(out, "session.json"), raw, 0644))
	fmt.Printf("Recorded-pad replay %d..%d; %d declared life grants applied; no policy or image-derived state changes.\n", start, end, grant)
}
