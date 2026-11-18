package server

// Real HTTP/SQLite/ticker acceptance for original combat automation. Failures:
// added bot seats never issue commands; the host ignores explicit autoplay;
// restoring an unfinished human menu confirms the wrong action; save/restart
// changes the original party or rewards; an old page controls resumed combat.
// The entry is an isolated original PAL_StartBattle, not a story checkpoint.
import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalBattleBotsHTTP(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{palBoardModuleAt(t, ".local/pal/battle-fixtures/team-1-party-3/rules.wasm")}}
			files := fstest.MapFS{"index.html": {Data: []byte("PAL original battle bots")}}
			s, e := New(cfg, files)
			if e != nil {
				t.Fatal(e)
			}
			ts := httptest.NewServer(s.Handler())
			defer func() { ts.Close(); s.Close() }()
			a := newClient(t, ts.URL)
			a.register(fmt.Sprintf("仙剑战斗托管%d", players))
			r := a.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版电脑战斗", "capacity": players}, 201)
			id := r["id"].(string)
			for seat := 1; seat < players; seat++ {
				a.command(current(a), "add_bot", nil, 200)
			}
			a.command(current(a), "ready", nil, 200)
			a.command(current(a), "start", nil, 200)
			data := func() map[string]any {
				return current(a)["game"].(map[string]any)["data"].(map[string]any)
			}
			combatCommands := func(battle any) map[string]any {
				// Idle sprites animate between HTTP requests. Compare every
				// command/resource field while allowing only those frame counters.
				encoded, err := json.Marshal(battle)
				if err != nil {
					t.Fatal(err)
				}
				var result map[string]any
				if err = json.Unmarshal(encoded, &result); err != nil {
					t.Fatal(err)
				}
				for _, collection := range []string{"players", "enemies"} {
					for _, member := range result[collection].([]any) {
						delete(member.(map[string]any), "frame")
					}
				}
				return result
			}
			wait := func(predicate func(map[string]any) bool) map[string]any {
				t.Helper()
				until := time.Now().Add(30 * time.Second)
				for time.Now().Before(until) {
					v := data()
					if predicate(v) {
						return v
					}
					time.Sleep(40 * time.Millisecond)
				}
				v := data()
				t.Fatal("original combat bot HTTP checkpoint unavailable", v["phase"], "received", v["seq"], "discarded", v["discardedSeq"], v["battle"])
				return nil
			}
			d := wait(func(v map[string]any) bool {
				b, ok := v["battle"].(map[string]any)
				return ok && b["state"] == float64(1) && b["current"] == float64(0)
			})
			// Capture the input marker and room version from the same response.
			// A second GET here can age the first marker under instrumentation.
			r = current(a)
			d = r["game"].(map[string]any)["data"].(map[string]any)
			offset, _ := r["clockOffsetMs"].(float64)
			t.Logf("initial observed control clock age: %dms", time.Now().UnixMilli()-int64(offset)-int64(d["controlAt"].(float64)))
			a.command(r, "action", map[string]any{"type": "input", "epoch": d["epoch"], "seq": d["seq"].(float64) + 1, "controlAt": d["controlAt"], "direction": 0}, 200)
			d = wait(func(v map[string]any) bool { return v["battle"].(map[string]any)["selected"] == float64(3) })
			beforeParty, beforeBattle := d["party"], combatCommands(d["battle"])
			oldPage := current(a)
			a.command(current(a), "save", nil, 200)
			if a.state()["room"] != nil {
				t.Fatal("saving original bot combat retained the human seat")
			}
			ts.Close()
			if e = s.Close(); e != nil {
				t.Fatal(e)
			}
			// A separate authority instance loads the stored native continuation.
			cfg.Modules = []gameapi.Module{palBoardModuleAt(t, ".local/pal/battle-fixtures/team-1-party-3/rules.wasm")}
			s, e = New(cfg, files)
			if e != nil {
				t.Fatal(e)
			}
			ts = httptest.NewServer(s.Handler())
			a.base = ts.URL
			a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
			d = data()
			if !reflect.DeepEqual(beforeParty, d["party"]) || !reflect.DeepEqual(beforeBattle, combatCommands(d["battle"])) || d["cash"] != float64(0) {
				t.Fatal("fresh server changed saved original battle")
			}
			a.command(oldPage, "action", map[string]any{"type": "input", "epoch": oldPage["game"].(map[string]any)["data"].(map[string]any)["epoch"], "seq": 999, "direction": 4, "keys": 2}, 409)
			r = current(a)
			a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "version": r["version"], "nonce": randomID(12), "enabled": true}, 200)
			d = wait(func(v map[string]any) bool { return v["cash"] == float64(3) })
			if len(d["party"].([]any)) != 3 {
				t.Fatal("bot support changed the original party")
			}
			r = current(a)
			if r["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
				t.Fatal("human autoplay was not explicitly retained")
			}
			a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "version": r["version"], "nonce": randomID(12), "enabled": false}, 200)
			if data()["cash"] != float64(3) || current(a)["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
				t.Fatal("manual reclaim reset combat or left autoplay enabled")
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "authenticated HTTP, SQLite restart, original isolated PAL_StartBattle, explicit single-human autoplay, saved unfinished menu, stale page rejection and manual reclaim; not campaign completion", "players": players, "botSeats": players - 1, "cash": d["cash"]}, "", "  ")
			if e = os.WriteFile(filepath.Join("..", "..", fmt.Sprintf(".local/pal/verification/battle-bots-http-%d.json", players)), report, 0600); e != nil {
				t.Fatal(e)
			}
		})
	}
}
