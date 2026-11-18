package pokepath_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type nativeSaveClient struct {
	fixture *nativeSaveFixture
	client  *http.Client
	name    string
	serial  atomic.Uint64
}

type nativeSaveFixture struct {
	application *server.Server
	host        *httptest.Server
	database    *sql.DB
	config      server.Config
	files       fstest.MapFS
	directory   string
	output      string
	clients     []*nativeSaveClient
	roomID      string
	matchID     string
	mutex       sync.Mutex
	trace       []map[string]any
	resumeCalls atomic.Int32
}

func nativeSaveObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func nativeSaveDecode(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("unexpected trailing JSON")
	}
	return value, nil
}

func nativeSaveInteger(value any) int64 {
	switch value := value.(type) {
	case json.Number:
		integer, _ := value.Int64()
		return integer
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	}
	return 0
}

func nativeSaveCanonical(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func nativeSaveHash(value any) string {
	sum := sha256.Sum256(nativeSaveCanonical(value))
	return hex.EncodeToString(sum[:])
}

func nativeSavePhysical(value map[string]any) map[string]any {
	copy, _ := nativeSaveDecode(nativeSaveCanonical(value))
	delete(copy, "epoch")
	return copy
}

func (client *nativeSaveClient) exchange(method, path string, body any) (map[string]any, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	request, err := http.NewRequest(method, client.fixture.host.URL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, err
	}
	value, err := nativeSaveDecode(encoded)
	client.fixture.mutex.Lock()
	client.fixture.trace = append(client.fixture.trace, map[string]any{"client": client.name, "method": method, "path": path, "status": response.StatusCode})
	client.fixture.mutex.Unlock()
	return value, response.StatusCode, err
}

func (client *nativeSaveClient) request(t *testing.T, method, path string, body any, expected int) map[string]any {
	t.Helper()
	value, status, err := client.exchange(method, path, body)
	if err != nil || status != expected {
		raw := nativeSaveCanonical(value)
		if len(raw) > 1600 {
			raw = raw[:1600]
		}
		t.Fatalf("%s %s returned %d, expected %d: %v %s", method, path, status, expected, err, raw)
	}
	return value
}

func (client *nativeSaveClient) room(t *testing.T) map[string]any {
	t.Helper()
	return nativeSaveObject(client.request(t, http.MethodGet, "/api/state", nil, 200)["room"])
}

func nativeSaveData(room map[string]any) map[string]any {
	return nativeSaveObject(nativeSaveObject(room["game"])["data"])
}

func (client *nativeSaveClient) command(t *testing.T, kind string, extra map[string]any, expected int) map[string]any {
	t.Helper()
	room := client.room(t)
	if room == nil {
		t.Fatal("client has no active room")
	}
	body := map[string]any{"type": kind, "version": room["version"], "nonce": fmt.Sprintf("native-save-%s-%012d", client.name, client.serial.Add(1))}
	for key, value := range extra {
		body[key] = value
	}
	return client.request(t, http.MethodPost, "/api/rooms/"+room["id"].(string), body, expected)
}

func (client *nativeSaveClient) action(t *testing.T, action map[string]any, expected int) map[string]any {
	t.Helper()
	room := client.room(t)
	copy := map[string]any{}
	for key, value := range action {
		copy[key] = value
	}
	copy["epoch"] = nativeSaveData(room)["epoch"]
	return client.command(t, "action", map[string]any{"action": copy}, expected)
}

func (client *nativeSaveClient) assertRejectedEpoch(t *testing.T, epoch int64) {
	t.Helper()
	nonce := fmt.Sprintf("native-save-%s-%012d", client.name, client.serial.Add(1))
	for attempt := 0; attempt < 24; attempt++ {
		room := client.room(t)
		body := map[string]any{"type": "action", "version": room["version"], "nonce": nonce, "action": map[string]any{"type": "speed", "epoch": epoch}}
		value, status, err := client.exchange(http.MethodPost, "/api/rooms/"+room["id"].(string), body)
		if err != nil {
			t.Fatal(err)
		}
		if status == http.StatusConflict {
			continue
		}
		if status != http.StatusBadRequest || value["error"] != "巡径守卫操作已经过期，请同步后重试" {
			t.Fatalf("old epoch returned %d: %v", status, value)
		}
		return
	}
	t.Fatal("the room version kept changing before the old epoch could be tested")
}

func newNativeSaveFixture(t *testing.T, humans, bots int) *nativeSaveFixture {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, ".local", "pokepath", "native-save")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(output, "database-")
	if err != nil {
		t.Fatal(err)
	}
	fixture := &nativeSaveFixture{directory: directory, output: output, files: fstest.MapFS{"index.html": {Data: []byte("native-save-frontend-build-one")}}}
	module := pokepath.Module()
	resume := module.ResumeSaved
	if resume != nil {
		module.ResumeSaved = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
			fixture.resumeCalls.Add(1)
			return resume(ctx, match)
		}
	}
	fixture.config = server.Config{DataDir: directory, InviteCode: "native-save-test", ClientLoading: true, Modules: []gameapi.Module{module}}
	fixture.application, err = server.New(fixture.config, fixture.files)
	if err != nil {
		os.RemoveAll(directory)
		t.Fatal(err)
	}
	fixture.host = httptest.NewServer(fixture.application.Handler())
	fixture.database, err = sql.Open("sqlite", filepath.Join(directory, "wire-board.db"))
	if err != nil {
		fixture.host.Close()
		fixture.application.Close()
		os.RemoveAll(directory)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if fixture.host != nil {
			fixture.host.Close()
		}
		if fixture.application != nil {
			fixture.application.Close()
		}
		fixture.database.Close()
		os.RemoveAll(directory)
	})
	for index := 0; index < humans+1; index++ {
		jar, _ := cookiejar.New(nil)
		client := &nativeSaveClient{fixture: fixture, name: fmt.Sprintf("p%d", index), client: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
		fixture.clients = append(fixture.clients, client)
		client.request(t, http.MethodPost, "/api/register", map[string]any{"name": fmt.Sprintf("原生存档队友%d", index), "password": "native-save-password-test", "invite": "native-save-test"}, 200)
	}
	owner := fixture.clients[0]
	created := owner.request(t, http.MethodPost, "/api/rooms", map[string]any{"kind": "pokepath", "name": "原生合作保存验收", "capacity": humans + bots, "options": map[string]any{"starter": "charmander"}}, 201)
	fixture.roomID = created["id"].(string)
	for _, client := range fixture.clients[1:humans] {
		client.request(t, http.MethodPost, "/api/rooms/"+fixture.roomID, map[string]any{"type": "join", "version": owner.room(t)["version"], "nonce": "native-save-join-" + client.name}, 200)
	}
	for index := 0; index < bots; index++ {
		owner.command(t, "add_bot", nil, 200)
	}
	for _, client := range fixture.clients[:humans] {
		client.command(t, "ready", nil, 200)
	}
	owner.command(t, "start", nil, 200)
	fixture.matchID = owner.room(t)["matchId"].(string)
	return fixture
}

func (fixture *nativeSaveFixture) stored(t *testing.T) map[string]any {
	t.Helper()
	var raw []byte
	if err := fixture.database.QueryRow("SELECT snapshot FROM rooms WHERE id=?", fixture.roomID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	room, err := nativeSaveDecode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return room
}

func nativeSaveStoredData(room map[string]any) map[string]any {
	return nativeSaveObject(nativeSaveObject(room["module"])["data"])
}

func nativeSaveSnapshot(t *testing.T, data map[string]any) map[string]any {
	t.Helper()
	snapshot := nativeSaveObject(data["snapshot"])
	if snapshot == nil {
		t.Fatal("native game must expose its complete serializable state")
	}
	if _, oldTape := snapshot["operations"]; oldTape {
		t.Fatal("native saved state cannot use the discarded JavaScript replay architecture")
	}
	if _, oldRunner := data["native"]; oldRunner {
		t.Fatal("native room view must not expose an embedded-game runner projection")
	}
	return snapshot
}

func (fixture *nativeSaveFixture) load(t *testing.T, humans int) {
	t.Helper()
	for _, client := range fixture.clients[:humans] {
		room := client.room(t)
		loading := nativeSaveObject(room["loading"])
		if loading == nil {
			t.Fatal("expected the shared client resource loading barrier")
		}
		body := map[string]any{"loadToken": loading["token"], "loadBuild": loading["build"], "loadClient": "native-save-page-" + client.name}
		client.command(t, "load_begin", body, 200)
		client.command(t, "load_ready", body, 200)
	}
	if fixture.clients[0].room(t)["loading"] != nil {
		t.Fatal("original clients did not release the loading barrier")
	}
}

func (fixture *nativeSaveFixture) restart(t *testing.T, build string) {
	t.Helper()
	if fixture.host != nil {
		fixture.host.Close()
	}
	if fixture.application != nil {
		if err := fixture.application.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fixture.host, fixture.application = nil, nil
	if build != "" {
		fixture.files["index.html"] = &fstest.MapFile{Data: []byte(build)}
	}
	application, err := server.New(fixture.config, fixture.files)
	if err != nil {
		t.Fatal(err)
	}
	fixture.application = application
	fixture.host = httptest.NewServer(application.Handler())
}

func (fixture *nativeSaveFixture) report(t *testing.T, name string, details map[string]any) {
	t.Helper()
	fixture.mutex.Lock()
	trace := append([]map[string]any(nil), fixture.trace...)
	fixture.mutex.Unlock()
	value := map[string]any{"test": t.Name(), "command": ". scripts/env.sh && go test -race ./games/pokepath/tests -run '^TestPokepathNativeSave' -count=1", "passed": !t.Failed(), "scope": "real authenticated HTTP, SQLite, original roster and independent native Go rules", "trace": trace, "details": details}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.output, name+".json"), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPokepathNativeSaveSnapshotContractHTTP(t *testing.T) {
	fixture := newNativeSaveFixture(t, 1, 0)
	data := nativeSaveData(fixture.clients[0].room(t))
	snapshot := nativeSaveSnapshot(t, data)
	stored := fixture.stored(t)
	if nativeSaveInteger(nativeSaveObject(stored["module"])["schema"]) < 2 {
		t.Fatal("native save format must reject the discarded embedded runtime schema")
	}
	identity, _ := nativeSaveStoredData(stored)["hash"].(string)
	if len(identity) < 16 {
		t.Fatal("native rules and resources must have a persisted identity")
	}
	fixture.report(t, "contract", map[string]any{"rulesHash": identity, "snapshotHash": nativeSaveHash(snapshot), "snapshot": snapshot})
}

func nativeSaveArray(value any) []any {
	array, _ := value.([]any)
	return array
}

func (fixture *nativeSaveFixture) battle(t *testing.T, humans int) map[string]any {
	t.Helper()
	owner := fixture.clients[0]
	fixture.load(t, humans)
	view := nativeSaveData(owner.room(t))
	team := nativeSaveArray(view["team"])
	if len(team) == 0 {
		t.Fatal("native game has no initial partner")
	}
	pokemonID := nativeSaveObject(team[0])["id"]
	paths := nativeSaveArray(view["paths"])
	if len(paths) == 0 || len(nativeSaveArray(paths[0])) == 0 {
		t.Fatal("native game has no active route")
	}
	entry := nativeSaveObject(nativeSaveArray(paths[0])[0])
	point := func(value any) float64 {
		number, _ := value.(json.Number)
		floating, _ := number.Float64()
		return floating
	}
	best, score := map[string]any(nil), 1e20
	for _, candidate := range nativeSaveArray(view["tiles"]) {
		tile := nativeSaveObject(candidate)
		if nativeSaveInteger(tile["terrain"]) != 1 {
			continue
		}
		dx, dy := point(tile["x"])-point(entry["x"]), point(tile["y"])-point(entry["y"])
		distance := dx*dx + dy*dy
		candidateScore := distance - 85*85
		if candidateScore < 0 {
			candidateScore = -candidateScore
		}
		if candidateScore < score {
			best, score = tile, candidateScore
		}
	}
	if best == nil {
		t.Fatal("native route has no legal starter terrain")
	}
	owner.action(t, map[string]any{"type": "deploy", "pokemonID": pokemonID, "tileID": best["id"]}, 200)
	owner.action(t, map[string]any{"type": "upgrade", "pokemonID": pokemonID}, 200)
	owner.action(t, map[string]any{"type": "wave"}, 200)
	for begin := time.Now(); time.Since(begin) < 5*time.Second; {
		view = nativeSaveData(owner.room(t))
		snapshot := nativeSaveSnapshot(t, view)
		if len(nativeSaveArray(snapshot["enemies"])) > 0 && len(nativeSaveArray(snapshot["towers"])) > 0 && len(nativeSaveArray(snapshot["projectiles"])) > 0 {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("real native battle did not produce an in-flight projectile")
	return nil
}

func (fixture *nativeSaveFixture) finishSave(t *testing.T, humans int) map[string]any {
	t.Helper()
	for _, client := range fixture.clients[:humans] {
		client.command(t, "save", nil, 200)
	}
	for _, client := range fixture.clients[:humans] {
		if client.room(t) != nil {
			t.Fatal("saved native world retained an original active seat")
		}
	}
	stored := fixture.stored(t)
	if stored["status"] != "saved" || stored["matchId"] != fixture.matchID {
		t.Fatal("save replaced original native room or match identity")
	}
	return stored
}

func (fixture *nativeSaveFixture) settledCounts(t *testing.T) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, table := range []string{"match_history", "rating_ledger"} {
		var count int
		if err := fixture.database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func (fixture *nativeSaveFixture) assertNoSettlement(t *testing.T) {
	t.Helper()
	for table, count := range fixture.settledCounts(t) {
		if count != 0 {
			t.Fatalf("save/resume created %d rows in %s", count, table)
		}
	}
}

func TestPokepathNativeSaveCompleteBattleStateAndOriginalRosterHTTP(t *testing.T) {
	for _, humans := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprintf("%d-humans", humans), func(t *testing.T) {
			fixture := newNativeSaveFixture(t, humans, 0)
			owner, stranger := fixture.clients[0], fixture.clients[humans]
			stranger.request(t, http.MethodPost, "/api/rooms/"+fixture.roomID+"/watch", map[string]any{}, 200)
			active := fixture.battle(t, humans)
			room := owner.room(t)
			stranger.request(t, http.MethodPost, "/api/rooms/"+fixture.roomID, map[string]any{"type": "save", "version": room["version"], "nonce": "native-save-spectator-denied"}, 400)
			oldEpoch := active["epoch"]
			owner.command(t, "save", nil, 200)
			frozenRoom := fixture.stored(t)
			frozen := nativeSaveSnapshot(t, nativeSaveStoredData(frozenRoom))
			if len(nativeSaveArray(frozen["projectiles"])) == 0 {
				t.Fatal("save did not capture the real in-flight projectile")
			}
			if humans > 1 {
				if owner.room(t)["status"] != "playing" || nativeSaveObject(owner.room(t)["game"])["paused"] != true {
					t.Fatal("first original human did not freeze the native battle")
				}
				time.Sleep(180 * time.Millisecond)
				if !reflect.DeepEqual(frozen, nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))) {
					t.Fatal("native world advanced while awaiting save consent")
				}
				for _, client := range fixture.clients[1:humans] {
					client.command(t, "save", nil, 200)
				}
			}
			for _, client := range fixture.clients[:humans] {
				if client.room(t) != nil {
					t.Fatal("save did not release an original human seat")
				}
			}
			if stranger.room(t) != nil {
				t.Fatal("save retained an old spectator relationship")
			}
			listed := owner.request(t, http.MethodGet, "/api/saves", nil, 200)
			ownSaves := nativeSaveArray(listed["saves"])
			if len(ownSaves) != 1 || nativeSaveObject(ownSaves[0])["id"] != fixture.roomID {
				t.Fatal("the original native save was not visible to its authenticated owner")
			}
			for _, key := range []string{"snapshot", "module", "data", "random", "state"} {
				if _, disclosed := nativeSaveObject(ownSaves[0])[key]; disclosed {
					t.Fatal("save summary disclosed the native rule payload")
				}
			}
			if len(nativeSaveArray(stranger.request(t, http.MethodGet, "/api/saves", nil, 200)["saves"])) != 0 {
				t.Fatal("a spectator could list another account's native save")
			}
			anonymous := &nativeSaveClient{fixture: fixture, client: &http.Client{Timeout: 10 * time.Second}, name: "anonymous"}
			anonymous.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 401)
			savedRoom := fixture.stored(t)
			savedData := nativeSaveStoredData(savedRoom)
			saved := nativeSaveSnapshot(t, savedData)
			if savedRoom["status"] != "saved" || savedRoom["matchId"] != fixture.matchID || !reflect.DeepEqual(saved, frozen) {
				t.Fatal("final save changed the approved native battle")
			}
			identity := savedData["hash"]
			if identity == nil || identity == "" {
				t.Fatal("save lost its native rule/resource identity")
			}
			fixture.restart(t, "native-save-frontend-build-two")
			time.Sleep(120 * time.Millisecond)
			if !reflect.DeepEqual(saved, nativeSaveSnapshot(t, nativeSaveStoredData(fixture.stored(t)))) {
				t.Fatal("new server advanced a saved native battle")
			}
			stranger.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 404)
			owner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
			if humans > 1 {
				if owner.room(t)["status"] != "restoring" {
					t.Fatal("one human bypassed the original cooperative roster")
				}
				for _, client := range fixture.clients[1:humans] {
					if client.room(t) != nil {
						t.Fatal("continuation occupied a missing original human")
					}
				}
				time.Sleep(120 * time.Millisecond)
				if !reflect.DeepEqual(saved, nativeSaveSnapshot(t, nativeSaveStoredData(fixture.stored(t)))) {
					t.Fatal("native world advanced during original-roster continuation")
				}
				for _, client := range fixture.clients[1:humans] {
					client.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
				}
			}
			resumedRoom := owner.room(t)
			resumed := nativeSaveSnapshot(t, nativeSaveData(resumedRoom))
			if resumedRoom["matchId"] != fixture.matchID || resumedRoom["id"] != fixture.roomID || nativeSaveInteger(resumed["epoch"]) <= nativeSaveInteger(frozen["epoch"]) || !reflect.DeepEqual(nativeSavePhysical(resumed), nativeSavePhysical(saved)) {
				t.Fatal("continuation changed physical/random/resource state or kept an old input epoch")
			}
			loading := nativeSaveObject(resumedRoom["loading"])
			build := sha256.Sum256([]byte("native-save-frontend-build-two"))
			if loading == nil || loading["build"] != hex.EncodeToString(build[:]) {
				t.Fatal("continuation did not use the current same-origin frontend loading identity")
			}
			if nativeSaveStoredData(fixture.stored(t))["hash"] != identity {
				t.Fatal("frontend build change rewrote the saved native resource identity")
			}
			time.Sleep(120 * time.Millisecond)
			if !reflect.DeepEqual(resumed, nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))) {
				t.Fatal("native battle moved before restored clients loaded")
			}
			fixture.load(t, humans)
			owner.assertRejectedEpoch(t, nativeSaveInteger(oldEpoch))
			for begin := time.Now(); ; {
				current := nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))
				if nativeSaveInteger(current["clock"]) > nativeSaveInteger(saved["clock"]) {
					break
				}
				if time.Since(begin) > 2*time.Second {
					t.Fatal("native battle remained frozen after all clients loaded")
				}
				time.Sleep(20 * time.Millisecond)
			}
			fixture.assertNoSettlement(t)
			fixture.report(t, fmt.Sprintf("lifecycle-%d", humans), map[string]any{"humans": humans, "roomID": fixture.roomID, "matchID": fixture.matchID, "rulesHash": identity, "activeProjectileCount": len(nativeSaveArray(active["projectiles"])), "savedStateHash": nativeSaveHash(saved), "resumedPhysicalHash": nativeSaveHash(nativeSavePhysical(resumed)), "savedPhysicalHash": nativeSaveHash(nativeSavePhysical(saved)), "saved": saved, "resumedBeforeLoading": resumed, "frontendBuild": loading["build"], "noSettlement": fixture.settledCounts(t)})
		})
	}
}

func TestPokepathNativeSaveSQLiteRollbackAndManualPauseHTTP(t *testing.T) {
	fixture := newNativeSaveFixture(t, 2, 0)
	owner, partner := fixture.clients[0], fixture.clients[1]
	fixture.load(t, 2)
	owner.action(t, map[string]any{"type": "pause"}, 200)
	paused := fixture.stored(t)
	if nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))["paused"] != true {
		t.Fatal("native manual pause was not accepted")
	}
	trigger := func(enabled bool) {
		statement := "DROP TRIGGER native_save_block"
		if enabled {
			statement = "CREATE TRIGGER native_save_block BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'native-save-rollback'); END"
		}
		if _, err := fixture.database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { fixture.database.Exec("DROP TRIGGER IF EXISTS native_save_block") })
	trigger(true)
	owner.command(t, "save", nil, 500)
	trigger(false)
	if !reflect.DeepEqual(paused, fixture.stored(t)) {
		t.Fatal("failed first save consent mutated persisted state")
	}
	owner.command(t, "save", nil, 200)
	frozen := fixture.stored(t)
	trigger(true)
	partner.command(t, "save_cancel", nil, 500)
	trigger(false)
	if !reflect.DeepEqual(frozen, fixture.stored(t)) {
		t.Fatal("failed refusal discarded freeze, consent or physical state")
	}
	partner.command(t, "save_cancel", nil, 200)
	if nativeSaveData(owner.room(t))["saving"] == true || nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))["paused"] != true {
		t.Fatal("refusing a save resumed an independently paused native game")
	}
	owner.command(t, "save", nil, 200)
	frozen = fixture.stored(t)
	trigger(true)
	partner.command(t, "save", nil, 500)
	trigger(false)
	if !reflect.DeepEqual(frozen, fixture.stored(t)) || owner.room(t) == nil || partner.room(t) == nil {
		t.Fatal("failed final consent released original seats or changed the native world")
	}
	partner.command(t, "save", nil, 200)
	fixture.restart(t, "")
	owner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
	waiting := fixture.stored(t)
	trigger(true)
	partner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 500)
	trigger(false)
	if !reflect.DeepEqual(waiting, fixture.stored(t)) || partner.room(t) != nil {
		t.Fatal("failed final continuation changed original confirmation or native state")
	}
	partner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
	fixture.load(t, 2)
	if nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))["paused"] != true || nativeSaveData(owner.room(t))["saving"] == true {
		t.Fatal("continuation confused manual pause with save approval pause")
	}
	fixture.assertNoSettlement(t)
	fixture.report(t, "sqlite-rollback", map[string]any{"firstConsentRollback": true, "finalConsentRollback": true, "refusalRollback": true, "finalContinuationRollback": true, "manualPausePreserved": true, "snapshot": nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))})
}

func TestPokepathNativeSaveRejectsMismatchedIdentityWithoutRewritingSQLite(t *testing.T) {
	for _, corruption := range []string{"hash", "module-schema", "state-schema", "rules-version"} {
		t.Run(corruption, func(t *testing.T) {
			fixture := newNativeSaveFixture(t, 1, 0)
			fixture.load(t, 1)
			fixture.finishSave(t, 1)
			fixture.host.Close()
			if err := fixture.application.Close(); err != nil {
				t.Fatal(err)
			}
			fixture.host, fixture.application = nil, nil
			var original []byte
			if err := fixture.database.QueryRow("SELECT snapshot FROM rooms WHERE id=?", fixture.roomID).Scan(&original); err != nil {
				t.Fatal(err)
			}
			stored, err := nativeSaveDecode(original)
			if err != nil {
				t.Fatal(err)
			}
			module := nativeSaveObject(stored["module"])
			data := nativeSaveObject(module["data"])
			snapshot := nativeSaveSnapshot(t, data)
			switch corruption {
			case "hash":
				data["hash"] = strings.Repeat("f", 64)
			case "module-schema":
				module["schema"] = 1
			case "state-schema":
				snapshot["schema"] = 999
			case "rules-version":
				snapshot["rulesVersion"] = "unrecognized-native-resource-version"
			}
			invalid := nativeSaveCanonical(stored)
			if _, err := fixture.database.Exec("UPDATE rooms SET snapshot=? WHERE id=?", invalid, fixture.roomID); err != nil {
				t.Fatal(err)
			}
			application, err := server.New(fixture.config, fixture.files)
			rejection := "server-start"
			if err == nil {
				fixture.application = application
				fixture.host = httptest.NewServer(application.Handler())
				fixture.clients[0].request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 400)
				fixture.host.Close()
				if err := fixture.application.Close(); err != nil {
					t.Fatal(err)
				}
				fixture.host, fixture.application = nil, nil
				rejection = "authenticated-resume"
			}
			var retained []byte
			if err := fixture.database.QueryRow("SELECT snapshot FROM rooms WHERE id=?", fixture.roomID).Scan(&retained); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(invalid, retained) {
				t.Fatal("rejected native format/resource identity rewrote the original saved bytes")
			}
			if _, err := fixture.database.Exec("UPDATE rooms SET snapshot=? WHERE id=?", original, fixture.roomID); err != nil {
				t.Fatal(err)
			}
			fixture.report(t, "identity-"+corruption, map[string]any{"corruption": corruption, "rejected": true, "rejectionBoundary": rejection, "preservedSQLiteBytes": true, "invalidHash": nativeSaveHash(json.RawMessage(invalid)), "originalHash": nativeSaveHash(json.RawMessage(original))})
		})
	}
}

func TestPokepathNativeSaveMixedBotAndConcurrentResumeHTTP(t *testing.T) {
	fixture := newNativeSaveFixture(t, 2, 1)
	owner, partner, stranger := fixture.clients[0], fixture.clients[1], fixture.clients[2]
	fixture.load(t, 2)
	var active map[string]any
	for begin := time.Now(); ; {
		active = nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))
		if len(nativeSaveArray(active["towers"])) > 0 || len(nativeSaveArray(active["team"])) > 1 || active["waveActive"] == true {
			break
		}
		if time.Since(begin) > 3*time.Second {
			t.Fatal("the real native bot did not submit any legal gameplay action")
		}
		time.Sleep(20 * time.Millisecond)
	}
	savedRoom := fixture.finishSave(t, 2)
	saved := nativeSaveSnapshot(t, nativeSaveStoredData(savedRoom))
	if nativeSaveInteger(saved["players"]) != 3 {
		t.Fatal("the bot did not retain its original platform seat")
	}
	fixture.restart(t, "")
	stranger.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 404)
	errors := make(chan error, 16)
	begin := make(chan struct{})
	var group sync.WaitGroup
	for _, client := range []*nativeSaveClient{owner, partner} {
		for request := 0; request < 8; request++ {
			group.Add(1)
			go func(client *nativeSaveClient) {
				defer group.Done()
				<-begin
				body, status, err := client.exchange(http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{})
				if err != nil || status != 200 {
					errors <- fmt.Errorf("concurrent native continuation status=%d err=%v body=%s", status, err, nativeSaveCanonical(body))
					return
				}
				errors <- nil
			}(client)
		}
	}
	close(begin)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	room := owner.room(t)
	resumed := nativeSaveSnapshot(t, nativeSaveData(room))
	if room["status"] != "playing" || room["matchId"] != fixture.matchID || !reflect.DeepEqual(nativeSavePhysical(saved), nativeSavePhysical(resumed)) || nativeSaveInteger(resumed["epoch"]) <= nativeSaveInteger(saved["epoch"]) {
		t.Fatal("concurrent continuation changed original native state or match identity")
	}
	var rooms int
	if err := fixture.database.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&rooms); err != nil {
		t.Fatal(err)
	}
	if rooms != 1 || len(nativeSaveArray(room["resumeJoined"])) != 2 || fixture.resumeCalls.Load() != 1 {
		t.Fatal("concurrent continuation created another world or duplicate original confirmations")
	}
	seats := nativeSaveArray(room["seats"])
	if len(seats) != 3 || nativeSaveObject(seats[2])["bot"] != true {
		t.Fatal("continuation reset the original computer seat")
	}
	readiness := nativeSaveArray(nativeSaveObject(room["loading"])["ready"])
	if len(readiness) != 3 || readiness[2] != true {
		t.Fatal("the computer incorrectly required a human loading confirmation")
	}
	owner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
	partner.request(t, http.MethodPost, "/api/saves/"+fixture.roomID+"/resume", map[string]any{}, 200)
	if !reflect.DeepEqual(resumed, nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))) || fixture.resumeCalls.Load() != 1 {
		t.Fatal("a successful continuation retry advanced or reset the native state")
	}
	fixture.assertNoSettlement(t)
	fixture.report(t, "mixed-bot-concurrent", map[string]any{"humans": 2, "computerSeats": 1, "concurrentRequests": 16, "committedResumeCallbacks": fixture.resumeCalls.Load(), "storedRooms": rooms, "originalConfirmations": len(nativeSaveArray(room["resumeJoined"])), "active": active, "saved": saved, "resumed": resumed, "physicalHash": nativeSaveHash(nativeSavePhysical(resumed)), "noSettlement": fixture.settledCounts(t)})
}

func TestPokepathNativeSavePendingConsentSurvivesRestartHTTP(t *testing.T) {
	fixture := newNativeSaveFixture(t, 2, 0)
	owner, partner := fixture.clients[0], fixture.clients[1]
	fixture.battle(t, 2)
	owner.command(t, "save", nil, 200)
	before := fixture.stored(t)
	frozen := nativeSaveSnapshot(t, nativeSaveStoredData(before))
	votes := nativeSaveArray(before["saveVotes"])
	if len(votes) != 1 || before["status"] != "playing" {
		t.Fatal("first consent was not an original-roster pending save")
	}
	fixture.restart(t, "")
	after := fixture.stored(t)
	restored := nativeSaveSnapshot(t, nativeSaveStoredData(after))
	if after["status"] != "playing" || nativeSaveStoredData(after)["saving"] != true || !reflect.DeepEqual(votes, nativeSaveArray(after["saveVotes"])) || !reflect.DeepEqual(nativeSavePhysical(frozen), nativeSavePhysical(restored)) {
		t.Fatal("restart lost consent, save-only freeze or native battle progress")
	}
	time.Sleep(180 * time.Millisecond)
	if !reflect.DeepEqual(restored, nativeSaveSnapshot(t, nativeSaveData(owner.room(t)))) {
		t.Fatal("native rules advanced while a restarted save was waiting for the original partner")
	}
	partner.command(t, "save_cancel", nil, 200)
	room := owner.room(t)
	if nativeSaveData(room)["saving"] == true || nativeSaveObject(room["game"])["paused"] == true || len(nativeSaveArray(room["saveVotes"])) != 0 {
		t.Fatal("the original partner could not refuse the pending save after restart")
	}
	fixture.assertNoSettlement(t)
	fixture.report(t, "pending-consent-restart", map[string]any{"originalVotes": votes, "saved": frozen, "restored": restored, "physicalHash": nativeSaveHash(nativeSavePhysical(restored)), "originalPartnerRefusal": true})
}
