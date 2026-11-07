package starfort_test

// Failure scenarios declared before implementation: clients choose/reroll world
// seeds; map changes on view/restart/unpublish; an expansion shifts old pads or
// destroys investments; partial/duplicate/concurrent votes select a different
// branch; stale epochs or spectators vote; readiness skips unresolved choices;
// timer does not create 6/11/16-wave choices; later choices disappear on restart;
// old schema-1 battles reset roads/enemy progress during migration; another seat
// changes targeting; rewards/boons apply twice. Fixtures skip completed combat
// only. Decisions, timers, auth, transactions and persistence use real HTTP.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func worldData(t *testing.T, r map[string]any) map[string]any {
	t.Helper()
	w, ok := data(r)["world"].(map[string]any)
	if !ok {
		t.Fatal("missing persisted world in public snapshot")
	}
	return w
}
func worldEvidence(tb *table) {
	tb.t.Helper()
	tb.t.Cleanup(func() {
		out := filepath.Join("..", "..", "..", ".local", "starfort-world", "http")
		if err := os.MkdirAll(out, 0755); err != nil {
			tb.t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": tb.t.Name(), "passed": !tb.t.Failed(), "scope": "real HTTP and production timers; test-only skipped-wave fixtures labelled in trace", "command": ". scripts/env.sh && go test ./games/starfort/tests -run '^" + tb.t.Name() + "$' -count=1 -v", "trace": tb.trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(tb.t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			tb.t.Error(err)
		}
	})
}
func httpWorldChoice(t *testing.T, r map[string]any, key string) map[string]any {
	t.Helper()
	c, ok := worldData(t, r)[key].(map[string]any)
	if !ok {
		t.Fatalf("missing %s for wave %v", key, data(r)["wave"])
	}
	return c
}
func voteAll(tb *table, key, action string, index int) {
	tb.t.Helper()
	c := httpWorldChoice(tb.t, tb.people[0].room(), key)
	id := c["options"].([]any)[index].(map[string]any)["id"]
	for _, p := range tb.people {
		if worldData(tb.t, p.room())[key] == nil {
			break
		}
		p.act(map[string]any{"type": action, "choice": id}, 200)
	}
	if worldData(tb.t, tb.people[0].room())[key] != nil {
		tb.t.Fatal("all votes did not resolve", key)
	}
}
func TestStarfortWorldSeedAndPublicationHTTP(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tb := newHost(t, n, starfort.Module())
			worldEvidence(tb)
			a := tb.people[0]
			a.request("POST", "/api/admin/games/starfort", map[string]bool{"enabled": true}, 200)
			for _, opts := range []map[string]any{{"seed": 123}, {"world": map[string]any{"seed": 123}}, {"scenario": "growth-vote"}} {
				a.request("POST", "/api/rooms", map[string]any{"kind": "starfort", "name": "禁止客户端选择地形", "capacity": n, "options": opts}, 400)
			}
			tb.start(map[string]any{"mode": "endless", "prepSeconds": 45})
			initial := a.room()
			w := worldData(t, initial)
			initialMap, ok := data(initial)["map"].(map[string]any)
			if w["seed"].(float64) <= 0 || w["stage"] != float64(1) || !ok || len(initialMap["routes"].([]any)) != 1 {
				t.Fatal("invalid initial world")
			}
			if _, duplicate := w["map"]; duplicate {
				t.Fatal("network view duplicated the map")
			}
			for i := 0; i < 3; i++ {
				snapshot := a.room()
				if !reflect.DeepEqual(w, worldData(t, snapshot)) || !reflect.DeepEqual(initialMap, data(snapshot)["map"]) {
					t.Fatal("read rerolled world")
				}
			}
			for _, p := range tb.people {
				p.act(map[string]any{"type": "pause"}, 200)
			}
			paused := worldData(t, a.room())
			tb.restart()
			restored := a.room()
			if !reflect.DeepEqual(paused, worldData(t, restored)) || !reflect.DeepEqual(initialMap, data(restored)["map"]) {
				t.Fatal("restart rerolled world")
			}
			a.request("POST", "/api/admin/games/starfort", map[string]bool{"enabled": false}, 200)
			unpublished := a.room()
			if !reflect.DeepEqual(paused, worldData(t, unpublished)) || !reflect.DeepEqual(initialMap, data(unpublished)["map"]) {
				t.Fatal("unpublish altered existing world")
			}
			if contains(a.state()["availableGames"], "starfort") {
				t.Fatal("unpublish did not hide new rooms")
			}
			tb.record("server seed stable through views, pause, restart and unpublish")
		})
	}
}

func TestStarfortWorldVotesHTTP(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tb := newHost(t, n, scenarioModule())
			worldEvidence(tb)
			tb.start(map[string]any{"scenario": "growth-vote", "mode": "endless", "prepSeconds": 45})
			a := tb.people[0]
			wait(t, "real timer presents expansion after cleared wave 5", func() bool { return data(a.room())["wave"] == float64(6) })
			before := a.room()
			choice := httpWorldChoice(t, before, "choice")
			opts := choice["options"].([]any)
			if len(opts) != 2 || data(before)["controls"].(map[string]any)["canReady"] != false {
				t.Fatal("route preview/readiness")
			}
			if reflect.DeepEqual(opts[0].(map[string]any)["map"], opts[1].(map[string]any)["map"]) {
				t.Fatal("branch previews identical")
			}
			for _, action := range []map[string]any{{"type": "ready"}, {"type": "route_vote", "choice": "invented"}, {"type": "route_vote", "choice": opts[0].(map[string]any)["id"], "epoch": data(before)["epoch"].(float64) - 1}} {
				a.act(action, 400)
			}
			tb.spectator.act(map[string]any{"type": "route_vote", "choice": opts[0].(map[string]any)["id"]}, 400)
			if !reflect.DeepEqual(before, a.room()) {
				t.Fatal("rejected decisions changed state")
			}
			// 1 player selects second; 2 players tie -> first; 4 players 3:1 -> second.
			winner := 1
			if n == 2 {
				winner = 0
			}
			for i, p := range tb.people {
				idx := 1
				if i == 0 && n > 1 {
					idx = 0
				}
				p.act(map[string]any{"type": "route_vote", "choice": opts[idx].(map[string]any)["id"]}, 200)
				if i < n-1 && worldData(t, a.room())["choice"] == nil {
					t.Fatal("resolved before whole team voted")
				}
			}
			after := a.room()
			w := worldData(t, after)
			if w["choice"] != nil || w["stage"] != float64(2) {
				t.Fatal("route vote did not expand")
			}
			if !reflect.DeepEqual(data(after)["map"], opts[winner].(map[string]any)["map"]) {
				t.Fatal("majority/tie selected wrong preview")
			}
			if !reflect.DeepEqual(data(before)["towers"], data(after)["towers"]) || gold(before, 0) != gold(after, 0) || data(before)["baseHP"] != data(after)["baseHP"] {
				t.Fatal("expansion lost tower/resources")
			}
			oldPads := data(before)["map"].(map[string]any)["pads"].([]any)
			newPads := data(after)["map"].(map[string]any)["pads"].([]any)
			if len(newPads) <= len(oldPads) || !reflect.DeepEqual(oldPads, newPads[:len(oldPads)]) {
				t.Fatal("old pad IDs/coordinates not preserved")
			}
			boon := httpWorldChoice(t, after, "boonChoice")
			boonID := boon["options"].([]any)[0].(map[string]any)["id"].(string)
			voteAll(tb, "boonChoice", "boon_vote", 0)
			if worldData(t, a.room())["boons"].(map[string]any)[boonID].(float64) <= 0 {
				t.Fatal("selected boon not active")
			}
			a.act(map[string]any{"type": "boon_vote", "choice": boonID}, 400)
			a.act(map[string]any{"type": "build", "pad": newPads[len(oldPads)].(map[string]any)["id"], "tower": "arrow"}, 200)
			for _, p := range tb.people {
				p.act(map[string]any{"type": "pause"}, 200)
			}
			persisted := worldData(t, a.room())
			tb.restart()
			if !reflect.DeepEqual(persisted, worldData(t, a.room())) {
				t.Fatal("chosen branch or boon rerolled")
			}
			tb.record("majority/tie vote, persistent growing pads, investments and boon")
		})
	}
}

func TestStarfortWorldConcurrentVotesHTTP(t *testing.T) {
	tb := newHost(t, 4, scenarioModule())
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "growth-vote", "mode": "endless", "prepSeconds": 45})
	a := tb.people[0]
	wait(t, "growth vote", func() bool { return data(a.room())["wave"] == float64(6) })
	snap := a.room()
	opts := httpWorldChoice(t, snap, "choice")["options"].([]any)
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for i, c := range tb.people {
		c.serial++
		idx := 1
		if i == 0 {
			idx = 0
		}
		body := map[string]any{"type": "action", "version": snap["version"], "nonce": fmt.Sprintf("world-vote-%012d", c.serial), "action": map[string]any{"type": "route_vote", "choice": opts[idx].(map[string]any)["id"], "epoch": data(snap)["epoch"]}}
		wg.Add(1)
		go func(c *client, body any) {
			defer wg.Done()
			status, _ := c.requestStatus("POST", "/api/rooms/"+snap["id"].(string), body)
			codes <- status
		}(c, body)
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal("concurrent same-epoch vote rejected", code)
		}
	}
	after := a.room()
	if worldData(t, after)["choice"] != nil || !reflect.DeepEqual(data(after)["map"], opts[1].(map[string]any)["map"]) {
		t.Fatal("concurrent votes lost/wrong majority")
	}
	tb.record("four simultaneous same-version HTTP votes resolved one majority")
}

func TestStarfortWorldTimerGrowthHTTP(t *testing.T) {
	for _, wave := range []int{5, 10, 15} {
		t.Run(fmt.Sprint(wave), func(t *testing.T) {
			tb := newHost(t, 2, scenarioModule())
			worldEvidence(tb)
			tb.start(map[string]any{"scenario": fmt.Sprintf("cleared-%d", wave), "mode": "endless", "prepSeconds": 45})
			a := tb.people[0]
			before := a.room()
			wait(t, "real timer moves cleared battle into next wave choice", func() bool { return data(a.room())["wave"] == float64(wave+1) })
			choice := httpWorldChoice(t, a.room(), "choice")
			if choice["stage"] != float64(wave/5+1) {
				t.Fatal("wrong expansion stage")
			}
			voteAll(tb, "choice", "route_vote", 1)
			voteAll(tb, "boonChoice", "boon_vote", 1)
			after := a.room()
			oldMap := data(before)["map"].(map[string]any)
			newMap := data(after)["map"].(map[string]any)
			if newMap["width"].(float64)*newMap["height"].(float64) <= oldMap["width"].(float64)*oldMap["height"].(float64) {
				t.Fatal("world did not grow")
			}
			if len(newMap["routes"].([]any)) <= len(oldMap["routes"].([]any)) {
				t.Fatal("no new entrance")
			}
			if data(after)["nextAt"].(float64)-float64(time.Now().UnixMilli()) < 40000 {
				t.Fatal("new layout lacks preparation time")
			}
			tb.record("real timer boundary and route/boon votes after explicit cleared-wave fixture")
		})
	}
}

func TestStarfortWorldChoiceRestartAndOwnershipHTTP(t *testing.T) {
	tb := newHost(t, 2, scenarioModule())
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "growth-vote", "mode": "endless", "prepSeconds": 45})
	a, b := tb.people[0], tb.people[1]
	wait(t, "growth vote", func() bool { return data(a.room())["wave"] == float64(6) })
	option := httpWorldChoice(t, a.room(), "choice")["options"].([]any)[1].(map[string]any)["id"]
	a.act(map[string]any{"type": "route_vote", "choice": option}, 200)
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	saved := worldData(t, a.room())
	tb.restart()
	if !reflect.DeepEqual(saved, worldData(t, a.room())) {
		t.Fatal("partial votes lost")
	}
	for _, p := range tb.people {
		p.act(map[string]any{"type": "resume"}, 200)
	}
	if httpWorldChoice(t, a.room(), "choice")["votes"].([]any)[0] != option {
		t.Fatal("resume erased existing route vote")
	}
	b.act(map[string]any{"type": "route_vote", "choice": option}, 200)
	voteAll(tb, "boonChoice", "boon_vote", 2)
	tower := data(a.room())["towers"].([]any)[0].(map[string]any)
	b.act(map[string]any{"type": "target", "towerId": tower["id"], "priority": "support"}, 400)
	tb.spectator.act(map[string]any{"type": "target", "towerId": tower["id"], "priority": "strong"}, 400)
	a.act(map[string]any{"type": "target", "towerId": tower["id"], "priority": "invented"}, 400)
	a.act(map[string]any{"type": "target", "towerId": tower["id"], "priority": "support"}, 200)
	if data(a.room())["towers"].([]any)[0].(map[string]any)["priority"] != "support" {
		t.Fatal("owned target change missing")
	}
	tb.record("pending choices survive pause/restart/resume; targeting owner enforced")
}

func TestStarfortWorldLegacyBattleMigrationHTTP(t *testing.T) {
	tb := newHost(t, 2, scenarioModule())
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "legacy-save", "mode": "endless", "prepSeconds": 45})
	a := tb.people[0]
	r := a.room()
	w := worldData(t, r)
	if data(r)["phase"] != "battle" || data(r)["wave"] != float64(4) || w["choice"] != nil || w["stage"] != float64(1) {
		t.Fatal("legacy battle changed phase/wave or expanded midwave")
	}
	path := data(r)["map"].(map[string]any)["path"]
	wantPath := []any{map[string]any{"x": float64(-30), "y": float64(120)}, map[string]any{"x": float64(250), "y": float64(120)}, map[string]any{"x": float64(250), "y": float64(300)}, map[string]any{"x": float64(700), "y": float64(300)}, map[string]any{"x": float64(700), "y": float64(500)}, map[string]any{"x": float64(1000), "y": float64(500)}}
	if !reflect.DeepEqual(path, wantPath) {
		t.Fatal("legacy main road changed during migration")
	}
	tower := data(r)["towers"].([]any)[0].(map[string]any)
	if tower["id"] != float64(1) || tower["pad"] != float64(0) || tower["level"] != float64(2) || tower["spent"] != float64(153) || tower["owner"] != float64(0) || tower["priority"] != "first" {
		t.Fatal("legacy tower lost", tower)
	}
	enemy := data(r)["enemies"].([]any)[0].(map[string]any)
	if enemy["progress"].(float64) < 580 || enemy["route"] != "main" {
		t.Fatal("legacy enemy reset")
	}
	seed := w["seed"]
	before := data(r)["tick"].(float64)
	wait(t, "legacy schema timer advances and persists migrated world", func() bool { return data(a.room())["tick"].(float64) > before })
	r = a.room()
	progress := data(r)["enemies"].([]any)[0].(map[string]any)["progress"].(float64)
	if progress <= enemy["progress"].(float64) {
		t.Fatal("legacy enemy stopped moving")
	}
	tb.restart()
	r = a.room()
	if worldData(t, r)["seed"] != seed || !reflect.DeepEqual(data(r)["map"].(map[string]any)["path"], wantPath) || data(r)["enemies"].([]any)[0].(map[string]any)["progress"].(float64) < progress {
		t.Fatal("restart changed migrated road/seed/progress")
	}
	tb.record("real schema-1 persisted battle migrates without resetting route, tower or moving enemy")
}

func TestStarfortWorldObjectivesHTTP(t *testing.T) {
	tb := newHost(t, 2, scenarioModule())
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "objectives-clear", "mode": "endless", "prepSeconds": 45})
	a, b := tb.people[0], tb.people[1]
	before := a.room()
	objectives := worldData(t, before)["objectives"].([]any)
	if len(objectives) != 3 {
		t.Fatal("stage4 lacks three side-objective types")
	}
	totalCost := 0.0
	for _, v := range objectives {
		o := v.(map[string]any)
		totalCost += o["cost"].(float64)
		tb.spectator.act(map[string]any{"type": "objective", "objectiveId": o["id"]}, 400)
		a.act(map[string]any{"type": "objective", "objectiveId": o["id"]}, 200)
		b.act(map[string]any{"type": "objective", "objectiveId": o["id"]}, 400)
	}
	if gold(a.room(), 0) != gold(before, 0)-totalCost || gold(a.room(), 1) != gold(before, 1) {
		t.Fatal("objective payment duplicated or charged wrong seat")
	}
	tb.record("three objectives accepted over real HTTP and charged exactly once")
	for round := 1; round <= 2; round++ {
		wave := data(a.room())["wave"].(float64)
		for _, p := range tb.people {
			p.act(map[string]any{"type": "ready"}, 200)
		}
		wait(t, "real timer completes explicitly skipped objective combat", func() bool { return data(a.room())["wave"].(float64) > wave })
		for _, v := range worldData(t, a.room())["objectives"].([]any) {
			o := v.(map[string]any)
			expected := "completed"
			if round == 1 && o["kind"] == "convoy" {
				expected = "active"
			}
			if o["status"] != expected {
				t.Fatal("objective progression", round, o)
			}
		}
		tb.record(fmt.Sprintf("objective real-timer progression %d/2; second fixture wave skips safe convoy journey except final 10 units", round))
	}
	if data(a.room())["baseHP"] != float64(24) {
		t.Fatal("convoy did not repair four base health")
	}
	if worldData(t, a.room())["boons"].(map[string]any)["reach"].(float64) < 1 {
		t.Fatal("beacon permanent reach missing")
	}
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	saved := a.room()
	tb.restart()
	if !reflect.DeepEqual(saved, a.room()) {
		t.Fatal("objective rewards changed on restart")
	}
	tb.record("completed cooperative objectives persist without duplicate reward")
}

func TestStarfortWorldObjectivesDamageHTTP(t *testing.T) {
	tb := newHost(t, 2, scenarioModule())
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "objectives-fail", "mode": "endless", "prepSeconds": 45})
	a := tb.people[0]
	for _, v := range worldData(t, a.room())["objectives"].([]any) {
		a.act(map[string]any{"type": "objective", "objectiveId": v.(map[string]any)["id"]}, 200)
	}
	paid := a.room()
	for _, p := range tb.people {
		p.act(map[string]any{"type": "ready"}, 200)
	}
	wait(t, "real enemy impacts destroy all test-only .1 HP objectives", func() bool {
		for _, v := range worldData(t, a.room())["objectives"].([]any) {
			if v.(map[string]any)["status"] != "failed" {
				return false
			}
		}
		return true
	})
	r := a.room()
	if data(r)["baseHP"] != float64(20) || gold(r, 0) != gold(paid, 0) || gold(r, 1) != gold(paid, 1) || !reflect.DeepEqual(worldData(t, paid)["boons"], worldData(t, r)["boons"]) {
		t.Fatal("local objective failure changed base or granted rewards")
	}
	for _, v := range worldData(t, r)["objectives"].([]any) {
		if v.(map[string]any)["hp"] != float64(0) {
			t.Fatal("destroyed objective retained health")
		}
	}
	tb.record("test-only weakened objective sites destroyed by real enemy proximity damage; no reward and base unaffected")
}

func TestStarfortWorldVoteDeadlinesHTTP(t *testing.T) {
	m := scenarioModule()
	timer := m.OnTimer
	m.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		next, err := timer(ctx, match)
		if err != nil {
			return next, err
		}
		var s starfort.State
		if err = json.Unmarshal(next.Data, &s); err != nil {
			return next, err
		}
		// Only the waiting interval is shortened in this explicitly labelled
		// fixture. Majority, fallback selection and transitions are real timers.
		if s.Phase == "prepare" && (s.World.Choice != nil || s.World.BoonChoice != nil) {
			s.NextAt = ctx.Now.Add(1800 * time.Millisecond).UnixMilli()
			if s.World.Choice != nil {
				s.World.Choice.Deadline = s.NextAt
			}
			if s.World.BoonChoice != nil {
				s.World.BoonChoice.Deadline = s.NextAt
			}
			return s.Envelope()
		}
		return next, nil
	}
	tb := newHost(t, 4, m)
	worldEvidence(tb)
	tb.start(map[string]any{"scenario": "growth-vote", "mode": "endless", "prepSeconds": 45})
	a := tb.people[0]
	wait(t, "growth vote", func() bool { return data(a.room())["wave"] == float64(6) })
	initial := a.room()
	choice := httpWorldChoice(t, initial, "choice")
	selected := choice["options"].([]any)[1].(map[string]any)
	a.act(map[string]any{"type": "route_vote", "choice": selected["id"]}, 200)
	wait(t, "actual deadline selects only cast vote", func() bool { return worldData(t, a.room())["choice"] == nil })
	if !reflect.DeepEqual(data(a.room())["map"], selected["map"]) {
		t.Fatal("deadline ignored cast majority")
	}
	wait(t, "actual deadline defaults unanswered boon", func() bool { return worldData(t, a.room())["boonChoice"] == nil })
	r := a.room()
	if worldData(t, r)["boons"].(map[string]any)["power"] != float64(1) || data(r)["phase"] != "prepare" || data(r)["nextAt"].(float64)-float64(time.Now().UnixMilli()) < 40000 {
		t.Fatal("timeout did not provide default boon and fresh preparation")
	}
	a.act(map[string]any{"type": "route_vote", "choice": selected["id"], "epoch": data(initial)["epoch"]}, 400)
	tb.record("test-only 1.8 second decision clocks; real timeout majority/default and fresh preparation")
}
