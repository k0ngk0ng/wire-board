package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Before nozzle-aware landing: the same visible platform offers a forward
// landing away from the nozzle, but the planner prefers its muzzle edge and
// dies before the emerging flame becomes recognizable. Do not turn a nozzle
// into a hidden timer or ban the whole platform permanently.
func TestBotJumpKeepsLandingAwayFromVisibleNozzle(t *testing.T) {
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
	b := Bot{LastX: o.Players[0].X, LastY: o.Players[0].Y, ViewStillTicks: 1000}
	x, y, _, found := b.planJump(o, false)
	if !found || x <= o.Players[0].X || y >= o.Players[0].Y {
		t.Fatal("no forward higher landing", x, y, found)
	}
	if o.terrainAt(x, y-10) == TerrainSolid {
		t.Fatal("landing body intersects visible wall", x, y)
	}
	if x > 108 && y > 120 && y < 160 {
		t.Fatal("planner lands immediately in front of nozzle", x, y)
	}
	// An unrelated background feature with the same numeric kind must not gain
	// nozzle geometry merely because the engine reuses kind numbers.
	for i := range o.Enemies {
		if o.Enemies[i].Family == "machine-flame-nozzle-left" {
			o.Enemies[i].Family = "unclassified-structure"
		}
	}
	ordinaryX, _, _, _ := b.planJump(o, false)
	if ordinaryX == x {
		t.Fatal("nozzle label failed to influence the dangerous landing", x)
	}
}

// An intermediate platform above y160 must still allow the next safe upward
// jump when continuing horizontally would approach a visible nozzle.
func TestBotNozzleAheadAllowsClimbingFromMiddlePlatform(t *testing.T) {
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
	o.Players[0].X, o.Players[0].Y, o.Players[0].Sprite = 106, 148, 13
	b := Bot{LastX: 106, LastY: 148, ViewStillTicks: 1000}
	b.TrackSeenAt[7] = o.Tick - 10
	b.PreviousEnemies[7] = BotEnemy{ID: 7, X: 120, Y: 136, Kind: 17, Family: "machine-flame"}
	if pad := b.play(o); pad&ButtonA == 0 {
		t.Fatal("middle-platform actor walks toward nozzle instead of taking safe higher landing", pad)
	}
}
