package starfort

// Failure inventory before implementation: late-wave full maps resend redundant
// target snapshots/coordinates and controls; dense splash multiplies wire bytes
// and painted particles; rendering starves buttons and RTT timers; a reduction
// drops damage/skills or advances HP before impact; reconnect replays old effects.
// Real layouts and a separately labelled synthetic overlap ceiling are measured.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func densityState(t *testing.T, kind string) *State {
	players := 4
	if kind == "observed" {
		players = 2
	}
	s := centralCase(t, players)
	if kind == "coop" {
		s.initWorld(37)
		for _, wave := range []int{6, 11, 16} {
			s.Wave = wave
			s.CompletedWaves = wave - 1
			s.describeWave()
			s.worldPrepare(s.ServerAt)
			s.finishChoice("route", s.ServerAt)
			s.finishChoice("boon", s.ServerAt)
		}
	}
	s.World.Map = cloneMap(s.World.Map)
	s.Wave = 45
	s.CompletedWaves = 44
	s.World.PreparedWave = 45
	s.World.Choice = nil
	s.World.BoonChoice = nil
	s.World.Event = eventFor(1, 37)
	s.World.Event.Wave = 45
	s.Phase = "battle"
	s.NextAt = 0
	s.Events = []Event{}
	s.EventSerial = 0
	s.World.Objectives = []Objective{}
	s.WaveInfo = WaveInfo{Title: "第 45 波 · 满塔密集验收", Total: 64, Spawned: 64}
	if kind == "ceiling" {
		s.World.Layout = ""
		s.World.Map.Path = []Point{{300, 480}, s.World.Map.Base}
		s.World.Map.Routes = []Route{{ID: "main", Sector: 1, Active: true, Points: s.World.Map.Path}}
		for i := range s.World.Map.Pads {
			s.World.Map.Pads[i].X = 500 + float64(i%5)
			s.World.Map.Pads[i].Y = 480 + float64(i/5)
		}
	}
	for i := range s.World.Map.Pads {
		towerKind := []string{"cannon", "arrow", "frost", "arcane"}[i%4]
		branch := "speed"
		if towerKind == "arrow" {
			branch = "volley"
		}
		if kind == "ceiling" {
			towerKind = "cannon"
			branch = "speed"
		}
		s.Towers = append(s.Towers, Tower{ID: i + 1, Pad: i, Owner: i % 4, Kind: towerKind, Level: 80, Branch: branch, Priority: "first", Spent: 10000})
	}
	if kind == "ceiling" {
		for i := range s.Towers {
			s.Towers[i].Level = 10000
		}
	}
	if kind == "observed" {
		raw, err := os.ReadFile(filepath.Join("..", "tests", "testdata", "late-wave-build.json"))
		if err != nil {
			t.Fatal(err)
		}
		var config struct{ Towers []Tower }
		if err = json.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		s.Towers = config.Towers
	}
	s.TowerSerial = 0
	for _, tower := range s.Towers {
		s.TowerSerial = max(s.TowerSerial, tower.ID)
	}
	for i := 0; i < 64; i++ {
		route := s.World.Map.Routes[i%len(s.World.Map.Routes)]
		progress := pathDistance(route.Points)*.7 + float64(i/len(s.World.Map.Routes))*2
		if kind == "ceiling" {
			progress = 200 + float64(i)/10
		}
		p := pathPosition(route.Points, progress)
		s.Enemies = append(s.Enemies, Enemy{ID: 2201 + i, Kind: "armored", Level: 45, Route: route.ID, Progress: progress, X: p.X, Y: p.Y, HP: 1e11, MaxHP: 1e11, Speed: .001, Leak: 1})
	}
	s.EnemySerial = 2264
	for i := range s.Players {
		s.Players[i].Gold = 100000
	}
	return s
}
func TestLateWaveDensityProfile(t *testing.T) {
	dir := filepath.Join("..", "..", "..", ".local", "starfort-density")
	if e := os.MkdirAll(filepath.Join(dir, "starts"), 0755); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"central", "coop", "observed", "ceiling"} {
		t.Run(kind, func(t *testing.T) {
			s := densityState(t, kind)
			m, err := s.Envelope()
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "starts", kind+".json"), m.Data, 0644); err != nil {
				t.Fatal(err)
			}
			samples := []map[string]any{}
			maxBytes := 0
			for second := 1; second <= 12; second++ {
				now := time.UnixMilli(s.ServerAt + 1000)
				begin := time.Now()
				m, err = Module().OnTimer(gameapi.Context{Players: len(s.Players), Actor: -1, Now: now}, m)
				timerMS := float64(time.Since(begin).Microseconds()) / 1000
				if err != nil {
					t.Fatal(err)
				}
				s, err = decode(m)
				if err != nil {
					t.Fatal(err)
				}
				begin = time.Now()
				v, err := Module().View(m, 0)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				viewMS := float64(time.Since(begin).Microseconds()) / 1000
				maxBytes = max(maxBytes, len(raw))
				samples = append(samples, map[string]any{"second": second, "wireBytes": len(raw), "saveBytes": len(m.Data), "timerMS": timerMS, "viewMS": viewMS, "towers": len(s.Towers), "enemies": len(s.Enemies)})
				if second == 12 {
					os.WriteFile(filepath.Join(dir, kind+"-view.json"), raw, 0644)
				}
			}
			report, _ := json.MarshalIndent(samples, "", "  ")
			os.WriteFile(filepath.Join(dir, kind+"-profile.json"), report, 0644)
			t.Logf("%s: towers=%d enemies=%d peak wire=%d bytes", kind, len(s.Towers), len(s.Enemies), maxBytes)
			if maxBytes > 192*1024 && os.Getenv("STARFORT_DENSITY_BASELINE") != "1" {
				t.Errorf("dense presentation exceeds 192 KiB: %d", maxBytes)
			}
		})
	}
}
