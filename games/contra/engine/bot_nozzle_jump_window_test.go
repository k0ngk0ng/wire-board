package contra

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

// Before jump-window protection: the standing position is below a visible
// beam, but the jump crosses that beam. Brief recognition gaps must not count
// as a guaranteed quiet window; stale observations must still expire.
func TestBotWaitsBeforeJumpingIntoVisibleNozzleBeam(t *testing.T) {
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
	o.Tick = 100
	o.Enemies = append(o.Enemies, BotEnemy{ID: 7, X: 128, Y: 136, Kind: 17, Family: "machine-flame"})
	b := Bot{LastX: o.Players[0].X, LastY: o.Players[0].Y, ViewStillTicks: 1000}
	if pad := b.play(o); pad&ButtonA != 0 || pad&(ButtonLeft|ButtonRight) != 0 {
		t.Fatal("jump starts through a visibly emitting nozzle", pad)
	}
	// The same publicly observed flame is briefly absent from recognition.
	b.TrackSeenAt[7] = 100
	b.PreviousEnemies[7] = o.Enemies[len(o.Enemies)-1]
	o.Enemies = o.Enemies[:len(o.Enemies)-1]
	o.Tick = 103
	if pad := b.play(o); pad&ButtonA != 0 {
		t.Fatal("brief missing recognition became a safe jump window", pad)
	}
	o.Tick = 110
	if pad := b.play(o); pad&ButtonA == 0 {
		t.Fatal("recently cleared visible beam does not release jump", pad)
	}
	b.JumpCooldown = 0
	b.JumpTargetActive = false
	o.Tick = 120
	if pad := b.play(o); pad&ButtonA != 0 {
		t.Fatal("an arbitrary late quiet frame is assumed to be a fresh opening", pad)
	}
}
