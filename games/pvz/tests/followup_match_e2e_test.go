package pvz_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"time"
)

func followupWorld(t *testing.T, scene string, n int) engine.State {
	t.Helper()
	s, e := engine.New(engine.Options{Scene: scene, PrepSeconds: 10}, n, "followup-"+t.Name())
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.Step(&s, 200); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestPVZFollowupProgressiveSpecialUnitsE2E(t *testing.T) {
	for _, scene := range []string{"day", "night", "pool", "fog", "roof", "roofnight"} {
		t.Run("default-counter-"+scene, func(t *testing.T) {
			s, e := engine.New(engine.Options{Scene: scene}, 2, "counter-default")
			if e != nil {
				t.Fatal(e)
			}
			counter := false
			for _, kind := range s.Players[0].Deck {
				if kind == "blover" || kind == "cactus" || kind == "cattail" {
					counter = true
				}
			}
			if !counter {
				t.Fatal("default defense has no aerial answer", s.Players[0].Deck)
			}
		})
	}
	for _, q := range []struct {
		kind, scene  string
		row, seconds int
	}{{"balloon", "day", 0, 90}, {"digger", "day", 0, 90}, {"bungee", "day", 0, 90}, {"snorkel", "pool", 2, 60}, {"gargantuar", "day", 0, 120}, {"zomboss", "roofnight", 0, 120}} {
		t.Run(q.kind, func(t *testing.T) {
			m := pvz.Module()
			ctx := gameapi.Context{Actor: -1, Players: 2, Now: time.Unix(1800000000, 0)}
			raw, _ := json.Marshal(map[string]any{"scene": q.scene, "prepSeconds": 10})
			match, e := m.Start(ctx, raw)
			if e != nil {
				t.Fatal(e)
			}
			var saved struct {
				Runtime engine.State `json:"runtime"`
			}
			if e = json.Unmarshal(match.Data, &saved); e != nil {
				t.Fatal(e)
			}
			s := saved.Runtime
			s.Players[1].Deck = []string{q.kind}
			s.Players[1].Money = 10000
			s.Players[1].Cooldowns[q.kind] = 0
			if e = engine.Step(&s, 200); e != nil {
				t.Fatal(e)
			}
			a := engine.Action{Type: "spawn", Kind: q.kind, Row: q.row, Epoch: s.Epoch, Seq: s.Players[1].Seq + 1}
			before := engine.Digest(s)
			if engine.Apply(&s, 1, a) == nil || engine.Digest(s) != before {
				t.Fatal("special attacker available at opening or failed deployment changed state")
			}
			if e = engine.Step(&s, q.seconds*1000/engine.TickMS-1); e != nil {
				t.Fatal(e)
			}
			s.Players[1].Cooldowns[q.kind] = 0
			if engine.Apply(&s, 1, a) == nil {
				t.Fatal("unlock opened before its promised deadline")
			}
			if e = engine.Step(&s, 1); e != nil {
				t.Fatal(e)
			}
			if e = engine.Apply(&s, 1, a); e != nil {
				t.Fatal("unit stayed locked after advertised time", e)
			}
			followupRecord(t, s)
		})
	}
}
func followupSpawn(t *testing.T, s *engine.State, actor int, kind string, row int) int {
	t.Helper()
	s.Players[actor].Deck = []string{kind}
	s.Players[actor].Money = 10000
	s.Players[actor].Cooldowns[kind] = 0
	a := engine.Action{Type: "spawn", Epoch: s.Epoch, Seq: s.Players[actor].Seq + 1, Kind: kind, Row: row}
	if e := engine.Apply(s, actor, a); e != nil {
		t.Fatal(e)
	}
	return len(s.Zombies) - 1
}
func followupRecord(t *testing.T, s engine.State) {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local/pvz/followup-match")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real deterministic engine and normal operations; explicitly controlled boundary positions", "final": s}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, fmt.Sprintf("%x.json", []byte(t.Name()))), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestPVZFollowupMowerCrossingE2E(t *testing.T) {
	for _, q := range []struct {
		kind, scene string
		row         int
	}{{"zombie", "day", 0}, {"polevaulter", "day", 0}, {"dolphin", "pool", 2}, {"imp", "day", 0}} {
		t.Run(q.kind, func(t *testing.T) {
			s := followupWorld(t, q.scene, 2)
			at := followupSpawn(t, &s, 1, q.kind, q.row)
			z := &s.Zombies[at]
			z.X = -25
			if q.kind != "zombie" {
				z.State = "vaulting"
				z.Altitude = 1
				z.VaultUntil = s.Tick + 10
				z.VaultX = -150
				z.Flags["waterEntered"] = 1
			}
			if e := engine.Step(&s, 1); e != nil {
				t.Fatal(e)
			}
			if s.Phase != "battle" || len(s.Rounds) != 0 || s.Mowers[q.row].State != "moving" || len(s.Zombies) != 0 {
				t.Fatal("available mower did not resolve a temporary jumper before house loss", s.Phase, s.Mowers[q.row], s.Zombies)
			}
			followupRecord(t, s)
		})
	}
	t.Run("surfaced-miner-moving-away-never-breaches", func(t *testing.T) {
		s := followupWorld(t, "day", 2)
		at := followupSpawn(t, &s, 1, "digger", 0)
		z := &s.Zombies[at]
		z.X = -25
		z.Digging = false
		z.State = "walking"
		z.Flags["direction"] = 1
		s.Mowers[0].State = "spent"
		s.Mowers[0].X = 10500
		if e := engine.Step(&s, 5); e != nil {
			t.Fatal(e)
		}
		if s.Phase != "battle" {
			t.Fatal("rightward miner was adjudicated as a house crossing")
		}
		followupRecord(t, s)
	})
	t.Run("balloon-remains-a-real-flying-bypass", func(t *testing.T) {
		s := followupWorld(t, "day", 2)
		at := followupSpawn(t, &s, 1, "balloon", 0)
		s.Zombies[at].X = -25
		if e := engine.Step(&s, 1); e != nil {
			t.Fatal(e)
		}
		if s.Phase != "intermission" || s.Mowers[0].State != "ready" {
			t.Fatal("permanent flight incorrectly consumed ground mower")
		}
		followupRecord(t, s)
	})
	t.Run("a-car-far-downstream-is-not-whole-lane-immunity", func(t *testing.T) {
		s := followupWorld(t, "day", 2)
		at := followupSpawn(t, &s, 1, "zombie", 0)
		s.Zombies[at].X = -25
		s.Mowers[0].State = "moving"
		s.Mowers[0].X = 4000
		if e := engine.Step(&s, 1); e != nil {
			t.Fatal(e)
		}
		if s.Phase != "intermission" {
			t.Fatal("car already downstream granted lane immunity")
		}
		followupRecord(t, s)
	})
}
func TestPVZFollowupZombieDeploymentPacingE2E(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			initial, e := engine.New(engine.Options{}, n, "opening-budget")
			if e != nil {
				t.Fatal(e)
			}
			total := 0
			for _, p := range initial.Players {
				if p.Side == "plants" {
					total += p.Money
				}
			}
			if total != 300 {
				t.Fatal("defender initial budget changed", total)
			}
			s := followupWorld(t, "day", n)
			other := 1
			if n == 4 {
				other = 3
			}
			s.Players[1].Deck = []string{"zombie", "imp", "conehead"}
			s.Players[other].Deck = []string{"zombie", "imp", "conehead"}
			s.Players[1].Money = 10000
			s.Players[other].Money = 10000
			a := engine.Action{Type: "spawn", Kind: "zombie", Row: 0, Epoch: s.Epoch, Seq: s.Players[1].Seq + 1}
			if e = engine.Apply(&s, 1, a); e != nil {
				t.Fatal(e)
			}
			before := engine.Digest(s)
			b := engine.Action{Type: "spawn", Kind: "imp", Row: 1, Epoch: s.Epoch, Seq: s.Players[other].Seq + 1}
			if engine.Apply(&s, other, b) == nil || engine.Digest(s) != before {
				t.Fatal("changing cards or teammates bypassed shared deployment interval")
			}
			if e = engine.Step(&s, 59); e != nil {
				t.Fatal(e)
			}
			b.Seq = s.Players[other].Seq + 1
			if engine.Apply(&s, other, b) == nil {
				t.Fatal("deployment interval shorter than three seconds")
			}
			if e = engine.Step(&s, 1); e != nil {
				t.Fatal(e)
			}
			if e = engine.Apply(&s, other, b); e != nil {
				t.Fatal("deployment did not reopen", e)
			}
			followupRecord(t, s)
		})
	}
}
