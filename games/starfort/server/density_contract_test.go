package starfort

// Before the compact protocol: duplicate target state per shot, per-victim XY,
// snapshot size overflow, invalid grouped victims, or early/missing damage.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func TestMaximumDensityRemainsLoadableAndEveryShotSurvives(t *testing.T) {
	s := densityState(t, "ceiling")
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	for second := 0; second < 10; second++ {
		m, err = Module().OnTimer(gameapi.Context{Players: 4, Actor: -1, Now: time.UnixMilli(s.ServerAt + 1000)}, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Data) > maxSnapshotBytes {
			t.Fatalf("unloadable maximum-density save: %d", len(m.Data))
		}
		s, err = decode(m)
		if err != nil {
			t.Fatal(err)
		}
		shots := 0
		for _, e := range s.Events {
			if e.Type == "shot" && e.AtMS != nil && *e.AtMS > s.ElapsedMS-1000 {
				shots++
			}
		}
		if shots != 200 {
			t.Fatalf("lost shot at second %d: %d", second, shots)
		}
	}
	v, err := Module().View(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	var wire struct {
		Events []map[string]json.RawMessage `json:"events"`
	}
	json.Unmarshal(raw, &wire)
	snapshots, vitals, groups := 0, 0, 0
	for _, e := range wire.Events {
		if _, ok := e["target"]; ok {
			snapshots++
		}
		if _, ok := e["vitals"]; ok {
			vitals++
		}
		if _, ok := e["damageGroups"]; ok {
			groups++
		}
	}
	if snapshots > 64 || vitals == 0 || groups == 0 {
		t.Fatalf("not compact: target=%d vitals=%d groups=%d", snapshots, vitals, groups)
	}
	if len(raw) > 192*1024 {
		t.Fatalf("maximum wire %d exceeds budget", len(raw))
	}
}
func TestGroupedDamageExactlyMatchesVictimDamage(t *testing.T) {
	s := densityState(t, "ceiling")
	before := make([]float64, len(s.Enemies))
	for i, e := range s.Enemies {
		before[i] = e.HP + e.Shield
	}
	s.fireTower(0)
	raw, _ := json.Marshal(s.Events)
	var events []struct {
		Type         string
		TargetID     int
		Amount       float64
		DamageGroups []struct {
			Amount  float64
			Targets []int
		}
	}
	json.Unmarshal(raw, &events)
	sums := map[int]float64{}
	for _, e := range events {
		if e.Type != "hit" {
			continue
		}
		sums[e.TargetID] += e.Amount
		for _, g := range e.DamageGroups {
			for _, id := range g.Targets {
				sums[id] += g.Amount
			}
		}
	}
	for i, e := range s.Enemies {
		if math.Abs((before[i]-e.HP-e.Shield)-sums[e.ID]) > 1e-5 {
			t.Fatalf("victim %d damaged incorrectly", e.ID)
		}
	}
	// Corrupt save values must never enter the playback protocol.
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	json.Unmarshal(m.Data, &saved)
	for _, e := range saved["events"].([]any) {
		ev := e.(map[string]any)
		if ev["type"] == "hit" {
			ev["damageGroups"] = []any{map[string]any{"amount": 1, "targets": []int{int(ev["targetId"].(float64))}}}
			break
		}
	}
	m.Data, _ = json.Marshal(saved)
	if _, err = decode(m); err == nil {
		t.Fatal("duplicate primary grouped victim accepted")
	}
}

// Late-wave difficulty must not increase the regular wave count or bypass the
// global entity cap through bosses and their guards.
func TestEndlessEnemyCountsStayCapped(t *testing.T) {
	s := densityState(t, "central")
	for _, wave := range []int{18, 40, 100, 1000} {
		s.Wave = wave
		s.describeWave()
		if s.WaveInfo.Total != 44 {
			t.Fatalf("wave %d grows entity count: %d", wave, s.WaveInfo.Total)
		}
	}
	s.Towers = nil
	s.WaveInfo.Spawned = 0
	s.SpawnClock = 0
	s.step(s.ServerAt + StepMS)
	if len(s.Enemies) != 64 || s.WaveInfo.Spawned != 0 {
		t.Fatal("regular spawn exceeded cap")
	}
	s.Enemies[0].Kind = "boss"
	s.Enemies[0].Phase = 1
	s.Enemies[0].HP = s.Enemies[0].MaxHP * .4
	s.enemyAbilities()
	if len(s.Enemies) != 64 {
		t.Fatal("boss exceeded cap")
	}
	s.Enemies = s.Enemies[:63]
	s.Enemies[0].Phase = 1
	s.enemyAbilities()
	if len(s.Enemies) != 64 {
		t.Fatal("boss guards did not share remaining capacity")
	}
}
