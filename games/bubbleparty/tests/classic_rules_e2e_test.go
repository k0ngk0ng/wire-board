package bubbleparty_test

// Versioning failure scenarios precede implementation in REFACTOR_SCENARIOS.md.
import (
	"encoding/json"
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func classicState(t *testing.T, m gameapi.Match) map[string]json.RawMessage {
	t.Helper()
	var v map[string]json.RawMessage
	if err := json.Unmarshal(m.Data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func classicLegacy(t *testing.T, m gameapi.Match) gameapi.Match {
	t.Helper()
	raw := classicState(t, m)
	delete(raw, "rulesVersion")
	// A historical fixture must contain historical player attributes, not the
	// native profiles now created by Start.
	var players []bubbleparty.Player
	if err := json.Unmarshal(raw["players"], &players); err != nil {
		t.Fatal(err)
	}
	for i := range players {
		players[i].NativeSpeed = 0
		players[i].SpeedMS = bubbleparty.BaseMoveMS
		players[i].Mount = ""
		players[i].MaxBombs = 1
		players[i].Range = bubbleparty.BaseBombRange
	}
	raw["players"], _ = json.Marshal(players)
	var err error
	m.Data, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestClassicNewMatchDoesNotShrink(t *testing.T) {
	m := refactorStart(t)
	raw := classicState(t, m)
	if string(raw["rulesVersion"]) != "3" {
		t.Fatalf("new match missing classic version: %s", raw["rulesVersion"])
	}
	start := refactorRead(t, m, 0).StartAt
	m = refactorTick(t, m, start+200000)
	viewAny, err := bubbleparty.Module().View(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	v := viewAny.(bubbleparty.View)
	if v.Sudden || v.SafeInset != 0 || v.SuddenAt != 0 || m.Result != nil {
		t.Fatalf("classic match gained sudden-death mechanics: sudden=%v inset=%d at=%d result=%+v", v.Sudden, v.SafeInset, v.SuddenAt, m.Result)
	}
	for _, p := range v.Players {
		if !p.Alive {
			t.Fatalf("standing boundary player %d was eliminated without water", p.Seat)
		}
	}
}

func TestClassicUnequalSurvivorsDrawAtTimeLimit(t *testing.T) {
	m := refactorArena(t, func(s *bubbleparty.State) { s.Players[2].Alive = false })
	var s bubbleparty.State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		t.Fatal(err)
	}
	m = refactorTick(t, m, s.MatchEndsAt)
	if m.Result == nil || !m.Result.Draw || len(m.Result.Winners) != 0 {
		t.Fatalf("2 vs 1 survivors must draw at time limit: %+v", m.Result)
	}
}

func TestClassicRestorePreservesLegacyRules(t *testing.T) {
	for _, scenario := range []string{"shrink", "population"} {
		t.Run(scenario, func(t *testing.T) {
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.SuddenAt = s.ServerAt + 100
				if scenario == "population" {
					s.MatchEndsAt = s.ServerAt + 100
					s.Players[2].Alive = false
				}
			})
			m = classicLegacy(t, m)
			before := classicState(t, m)
			restored, err := bubbleparty.Module().Restore(gameapi.Context{Now: time.UnixMilli(refactorRead(t, m, 0).ServerAt)}, m)
			if err != nil {
				t.Fatal(err)
			}
			after := classicState(t, restored)
			if string(after["rulesVersion"]) != "" && string(after["rulesVersion"]) != "0" {
				t.Fatalf("legacy save upgraded during restore: %s", after["rulesVersion"])
			}
			for _, key := range []string{"serverAt", "suddenAt", "matchEndsAt", "players"} {
				if string(before[key]) != string(after[key]) {
					t.Fatalf("restore changed legacy %s", key)
				}
			}
			restored = refactorTick(t, restored, refactorRead(t, restored, 0).ServerAt+100)
			viewAny, err := bubbleparty.Module().View(restored, 0)
			if err != nil {
				t.Fatal(err)
			}
			v := viewAny.(bubbleparty.View)
			if !v.Sudden || v.SafeInset != 1 {
				t.Fatal("legacy sudden-death rule disappeared")
			}
			if scenario == "population" && (restored.Result == nil || restored.Result.Draw || len(restored.Result.Winners) != 2 || restored.Result.Winners[0] != 0) {
				t.Fatalf("legacy population result changed: %+v", restored.Result)
			}
		})
	}
}
