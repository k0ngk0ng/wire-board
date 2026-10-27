package starfort

// Before compact AoE events: a legal maximum-density second must retain every
// shot and every victim, stay decodable, and preserve continuous event IDs.
import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestDenseBattleRetainsCompleteAttackBatch(t *testing.T) {
	s := centralCase(t, 4)
	s.World.Layout = ""
	s.SkillRules = 0
	s.Phase = "battle"
	s.NextAt = 0
	s.Events = nil
	s.EventSerial = 0
	path := []Point{{300, 480}, s.World.Map.Base}
	s.World.Map.Path = path
	s.World.Map.Routes = []Route{{ID: "main", Sector: 1, Active: true, Points: path}}
	s.WaveInfo.Total = 64
	s.WaveInfo.Spawned = 64
	for i := range s.World.Map.Pads {
		s.World.Map.Pads[i].X = 500 + float64(i%5)
		s.World.Map.Pads[i].Y = 480 + float64(i/5)
		s.Towers = append(s.Towers, Tower{ID: i + 1, Pad: i, Owner: i % 4, Kind: "cannon", Level: 10000, Branch: "speed", Priority: "first", Spent: 1000})
	}
	s.TowerSerial = len(s.Towers)
	for i := 0; i < 64; i++ {
		p := pathPosition(path, 200+float64(i)/10)
		s.Enemies = append(s.Enemies, Enemy{ID: i + 1, Kind: "armored", Route: "main", Progress: 200 + float64(i)/10, X: p.X, Y: p.Y, HP: 1e11, MaxHP: 1e11, Speed: 1, Leak: 1})
	}
	s.EnemySerial = 64
	m, e := s.Envelope()
	if e != nil {
		t.Fatal(e)
	}
	m, e = Module().OnTimer(gameapi.Context{Actor: -1, Players: 4, Now: time.UnixMilli(s.ServerAt + 1000)}, m)
	if e != nil {
		t.Fatal(e)
	}
	after, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	shots := 0
	for i, event := range after.Events {
		if event.Type == "shot" {
			shots++
		}
		if i > 0 && event.ID != after.Events[i-1].ID+1 {
			t.Fatal("event IDs contain a gap")
		}
	}
	if shots != 40*5 {
		t.Fatalf("got %d shots; expected all 200", shots)
	}
	if len(m.Data) > 2*1024*1024 {
		t.Fatal("snapshot exceeded its fixed budget", len(m.Data))
	}
}

func TestDenseFrostPassivesKeepEveryShot(t *testing.T) {
	s := skillCase(t, "frost")
	lead := s.Enemies[0]
	lead.HP = 1e11
	lead.MaxHP = 1e11
	s.Enemies = nil
	for i := 0; i < 64; i++ {
		e := lead
		e.ID = i + 1
		s.Enemies = append(s.Enemies, e)
	}
	s.EnemySerial = 64
	s.Towers = nil
	for i := range s.World.Map.Pads {
		s.World.Map.Pads[i].X = lead.X - 15
		s.World.Map.Pads[i].Y = lead.Y
		s.Towers = append(s.Towers, Tower{ID: i + 1, Pad: i, Owner: 0, Kind: "frost", Level: 10000, Branch: "speed", Spent: 1000, Priority: "first"})
	}
	s.TowerSerial = len(s.Towers)
	for i := range s.Towers {
		if !s.fireTower(i) {
			t.Fatal("fixture cannot fire")
		}
	}
	shots, rings := 0, 0
	for _, e := range s.Events {
		if e.Type == "shot" {
			shots++
		}
		if e.Type == "frost_ring" {
			rings++
		}
	}
	if shots != len(s.Towers) || rings != len(s.Towers) {
		t.Fatalf("shots %d rings %d towers %d", shots, rings, len(s.Towers))
	}
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decode(m); err != nil {
		t.Fatal(err)
	}
}
