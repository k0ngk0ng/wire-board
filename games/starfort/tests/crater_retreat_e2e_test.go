package starfort_test

// Failure cases precede implementation in crater-retreat-failures.md.
// Public module/timer/serialization and real HTTP/SQLite replay. Only the
// platform identity and log fields were removed from the production fixture.
import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func stalledCraterMatch(t *testing.T) gameapi.Match {
	t.Helper()
	raw, err := os.ReadFile("testdata/fog-crater-stalled-match.json")
	if err != nil {
		t.Fatal(err)
	}
	var match gameapi.Match
	if err = json.Unmarshal(raw, &match); err != nil {
		t.Fatal(err)
	}
	return match
}
func TestStarfortCraterRetreatProductionReplay(t *testing.T) {
	m := starfort.Module()
	match := stalledCraterMatch(t)
	var initial starfort.State
	_ = json.Unmarshal(match.Data, &initial)
	ctx := gameapi.Context{Actor: -1, Players: len(initial.Players), Now: time.UnixMilli(initial.ServerAt)}
	before := string(match.Data)
	if err := m.ValidateMatch(match, len(initial.Players)); err != nil {
		t.Fatal("last committed production state must be valid", err)
	}
	if _, err := m.View(match, 0); err != nil {
		t.Fatal(err)
	}
	if string(match.Data) != before {
		t.Fatal("read changed production state")
	}
	path := filepath.Join("..", "..", "..", ".local", "starfort-stall", "production-replay.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var after starfort.State
	for second := 1; second <= 180; second++ {
		ctx.Now = time.UnixMilli(initial.ServerAt).Add(time.Duration(second) * time.Second)
		match, err = m.OnTimer(ctx, match)
		if err != nil {
			t.Fatalf("production tick %d: %v", initial.Tick+int64(second*5), err)
		}
		if err = m.ValidateMatch(match, len(initial.Players)); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(match.Data, &after)
		_ = json.NewEncoder(f).Encode(map[string]any{"second": second, "tick": after.Tick, "wave": after.Wave, "phase": after.Phase, "enemies": len(after.Enemies), "baseHP": after.BaseHP})
		if after.CompletedWaves >= initial.Wave || after.Phase == "lost" {
			break
		}
	}
	if after.Tick <= initial.Tick+50 {
		t.Fatal("saved battle did not resume")
	}
	if after.CompletedWaves < initial.Wave && after.Phase != "lost" {
		t.Fatal("production battle failed to reach its next phase", after.Phase, after.Tick)
	}
	t.Logf("production wave %d resumed from tick %d to %d; completed=%d", initial.Wave, initial.Tick, after.Tick, after.CompletedWaves)
}
func TestStarfortCraterRetreatControlOverlap(t *testing.T) {
	for _, version := range []int{2, 3} {
		for _, control := range []string{"freeze", "sleep", "recovery", "freeze-stationary", "sleep-stationary"} {
			t.Run(fmt.Sprint(version)+"/"+control, func(t *testing.T) {
				b := v3Boundary(t, "frost", "freeze", 4, "slime")
				s := cadenceState(b)
				s.WeatherRules = version
				s.Towers = nil
				s.World.Event.ID = "fog"
				s.World.Objectives = nil
				e := &s.Enemies[0]
				e.Speed = 40
				e.ConfusedUntil = s.Tick + 12
				stationary := strings.HasSuffix(control, "-stationary")
				if stationary {
					e.ConfusedUntil = 0
				}
				e.WeatherCheckAt = s.Tick + 1000
				e.WeatherReadyAt = s.Tick + 1000
				e.Repairing = 1
				e.RepairSinceMS = s.ElapsedMS
				e.CraterGuards = []starfort.KnockbackGuard{{TowerID: 1, Until: s.Tick + 20}}
				if strings.HasPrefix(control, "sleep") {
					e.SleepingUntil = s.Tick + 3
					if stationary {
						e.SleepingUntil = s.Tick + 6
					}
				} else {
					e.FrozenUntil = s.Tick + 3
					if stationary {
						e.FrozenUntil = s.Tick + 6
					}
					e.RecoveryUntil = e.FrozenUntil + 40
					if control == "recovery" {
						e.FrozenUntil = s.Tick + 1
						e.RecoveryUntil = e.FrozenUntil + 40
					}
				}
				s.EnemySerial = 2
				s.CraterSerial = 1
				s.Craters = []starfort.Crater{{ID: 1, TowerID: 1, Owner: 0, Route: e.Route, Progress: e.Progress, X: e.X, Y: e.Y, Radius: 28, Total: 5, Remaining: 5, Workers: []int{}, ExemptEnemy: 2, CreatedMS: s.ElapsedMS}}
				before := *e
				cadenceSave(b, s)
				b.timer(time.Second)
				after := cadenceState(b)
				if stationary {
					if after.Enemies[0].Repairing != 1 || after.Enemies[0].Progress != before.Progress {
						t.Fatal("immobilized enemy without retreat must stay in crater", after.Enemies[0])
					}
				} else if after.Enemies[0].Repairing != 0 || after.Enemies[0].RepairSinceMS != 0 || after.Enemies[0].Progress >= before.Progress {
					t.Fatal("retreat must release crater after immobilization", after.Enemies[0])
				}
				if after.Craters[0].Remaining != 5 || len(after.Craters[0].Workers) != 0 {
					t.Fatal("retreating enemy repaired crater")
				}
				if !reflect.DeepEqual(before.CraterGuards, after.Enemies[0].CraterGuards) || after.Enemies[0].ConfusedUntil != before.ConfusedUntil {
					t.Fatal("release must not change immunity/status deadline")
				}
				for i := 0; i < 5; i++ {
					b.timer(time.Second)
				}
				b.validate()
				if stationary && cadenceState(b).Craters[0].Remaining != 0 {
					t.Fatal("normal worker did not finish crater after thaw/wake")
				}
			})
		}
	}
}
func craterReplayModule(t *testing.T) gameapi.Module {
	t.Helper()
	m := starfort.Module()
	m.Start = func(ctx gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		match := stalledCraterMatch(t)
		var state starfort.State
		if err := json.Unmarshal(match.Data, &state); err != nil {
			return match, err
		}
		state.ServerAt = ctx.Now.UnixMilli()
		return state.Envelope()
	}
	return m
}
func TestStarfortCraterRetreatHTTPRestart(t *testing.T) {
	tb := newHost(t, 2, craterReplayModule(t))
	tb.start(nil)
	a := tb.people[0]
	deadline := time.Now().Add(5 * time.Second)
	advanced := false
	for time.Now().Before(deadline) {
		if data(a.room())["tick"].(float64) >= 6036 {
			advanced = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !advanced {
		t.Fatal("real HTTP host is still stuck at production tick 6031")
	}
	before := data(a.room())
	tb.http.Close()
	tb.host.Close()
	host, err := server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Starfort HTTP")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.host = host
	tb.http = httptest.NewServer(host.Handler())
	for _, person := range append(tb.people, tb.spectator) {
		person.base = tb.http.URL
	}
	restored := data(a.room())
	if restored["tick"].(float64) < before["tick"].(float64) || restored["wave"] != before["wave"] {
		t.Fatal("SQLite restart reset battle")
	}
	beforeTowers, restoredTowers := before["towers"].([]any), restored["towers"].([]any)
	if len(beforeTowers) != len(restoredTowers) {
		t.Fatal("restart removed towers")
	}
	for i, raw := range beforeTowers {
		previous, next := raw.(map[string]any), restoredTowers[i].(map[string]any)
		for _, key := range []string{"id", "pad", "owner", "kind", "level", "branch", "spent"} {
			if !reflect.DeepEqual(previous[key], next[key]) {
				t.Fatal("restart changed tower investment", key)
			}
		}
	}
	for i, raw := range before["players"].([]any) {
		if restored["players"].([]any)[i].(map[string]any)["gold"].(float64) < raw.(map[string]any)["gold"].(float64) {
			t.Fatal("restart lost player gold")
		}
	}
	tick := data(a.room())["tick"].(float64)
	deadline = time.Now().Add(5 * time.Second)
	advanced = false
	for time.Now().Before(deadline) {
		if data(a.room())["tick"].(float64) > tick {
			advanced = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !advanced {
		t.Fatal("restart stalled the restored battle")
	}
	tb.record("production wave 25: timer resumes through frozen/confused crater; live SQLite restart, resources and continuation preserved")
}
