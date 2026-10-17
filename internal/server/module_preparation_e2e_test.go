package server

// Failure inventory before implementation: forged actor/host/capacity/bots;
// spectator/anonymous/bot or playing access; stale/future/replayed commands;
// legacy Action decoding of a module preparation; invalid action/return JSON;
// missing Prepare; Configure bypass; callback input/output aliasing; failed
// callback/save changing options/ready/version; one actor changing another's
// selections; restart losing preparation. All room lifecycle actions use HTTP.
import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

import "github.com/k0ngk0ng/wire-board/internal/gameapi"

type preparationOptions struct {
	Values          map[string]int `json:"values"`
	Shared          string         `json:"shared"`
	CapacityChecked int            `json:"capacityChecked"`
}
type preparationObservation struct {
	Actor, Capacity int
	Host            bool
	Bots            []bool
}

func TestModulePreparationHTTP(t *testing.T) {
	var mu sync.Mutex
	var observed []preparationObservation
	var botRefs [][]bool
	var optionsRefs, actionRefs, configuredRefs []json.RawMessage
	configureCalls := 0
	m := gameapi.Module{ID: "preparation-fixture", Name: "准备契约验收", MinPlayers: 2, MaxPlayers: 4}
	m.Configure = func(capacity int, raw json.RawMessage) (json.RawMessage, error) {
		mu.Lock()
		defer mu.Unlock()
		configureCalls++
		if capacity < 2 || capacity > 4 {
			return nil, errors.New("bad capacity")
		}
		o := preparationOptions{Values: map[string]int{}}
		if len(raw) > 0 {
			d := json.NewDecoder(bytes.NewReader(raw))
			d.DisallowUnknownFields()
			if e := d.Decode(&o); e != nil {
				return nil, e
			}
		}
		if o.Values == nil {
			o.Values = map[string]int{}
		}
		for seat, value := range o.Values {
			i, e := strconv.Atoi(seat)
			if e != nil || i < 0 || i >= capacity || value < 0 || value > 9 {
				return nil, errors.New("bad selection")
			}
		}
		if o.Shared == "reject" {
			return nil, errors.New("configuration rejected")
		}
		if o.Shared == "invalid_config" {
			return json.RawMessage(" \nnull "), nil
		}
		o.CapacityChecked = capacity
		out, _ := json.Marshal(o)
		configuredRefs = append(configuredRefs, out)
		return out, nil
	}
	m.Prepare = func(c gameapi.PreparationContext, raw, action json.RawMessage) (json.RawMessage, error) {
		mu.Lock()
		defer mu.Unlock()
		observed = append(observed, preparationObservation{c.Actor, c.Capacity, c.Host, append([]bool(nil), c.Bots...)})
		botRefs = append(botRefs, c.Bots)
		optionsRefs = append(optionsRefs, raw)
		actionRefs = append(actionRefs, action)
		var a struct {
			Op    string `json:"op"`
			Value int    `json:"value"`
			Type  any    `json:"type"`
		}
		if e := json.Unmarshal(action, &a); e != nil {
			return nil, e
		}
		var o preparationOptions
		if e := json.Unmarshal(raw, &o); e != nil {
			return nil, e
		}
		// Inputs must be detached, including the journal's RawAction. Deliberate
		// mutations simulate a buggy plugin and must not affect host-owned data.
		for i := range c.Bots {
			c.Bots[i] = !c.Bots[i]
		}
		if len(raw) > 0 {
			raw[0] = '!'
		}
		if len(action) > 0 {
			action[0] = '!'
		}
		switch a.Op {
		case "error":
			return nil, errors.New("preparation rejected after mutation")
		case "malformed":
			return json.RawMessage("[broken"), nil
		case "null":
			return json.RawMessage(" \nnull "), nil
		case "injection":
			return json.RawMessage(`{"host":"attacker","capacity":99,"seats":[],"values":{}}`), nil
		case "invalid_config":
			o.Shared = "invalid_config"
		case "config_reject":
			o.Shared = "reject"
		case "shared":
			if !c.Host {
				return nil, errors.New("host required")
			}
			o.Shared = "host choice"
		case "select":
			if o.Values == nil {
				o.Values = map[string]int{}
			}
			o.Values[strconv.Itoa(c.Actor)] = a.Value
		default:
			return nil, errors.New("unknown preparation")
		}
		return json.Marshal(o)
	}
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Data: append(json.RawMessage(nil), o...), Actors: []int{}, TimeoutActors: []int{}, Log: []string{"started"}}, nil
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		return v, errors.New("unused fixture action")
	}
	m.View = func(v gameapi.Match, actor int) (any, error) {
		var out any
		e := json.Unmarshal(v.Data, &out)
		return out, e
	}
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	m.Reclaim = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) { return v, nil }
	plain := m
	plain.ID = "no-preparation-fixture"
	plain.Prepare = nil
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m, plain}}
	files := fstest.MapFS{"index.html": {Data: []byte("preparation fixture")}}
	s, e := New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, m.ID)
	publishModuleForTest(t, ts.URL, plain.ID)
	a, b, viewer, other := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("准备房主")
	b.register("准备玩家")
	viewer.register("准备观众")
	other.register("另一房间")
	trace := []any{}
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		out := filepath.Join("..", "..", ".local", "module-preparation")
		if e := os.MkdirAll(out, 0755); e != nil {
			t.Error(e)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "authenticated HTTP fixture only; no concrete game rules", "callbacks": observed, "configureCalls": configureCalls, "trace": trace}, "", "  ")
		if e := os.WriteFile(filepath.Join(out, "http.json"), raw, 0600); e != nil {
			t.Error(e)
		}
	})
	r := a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "准备房间", "capacity": 4}, 201)
	id := r["id"].(string)
	b.command(r, "join", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	viewer.post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
	other.post("/api/rooms", map[string]any{"kind": m.ID, "name": "隔离房间", "capacity": 2}, 201)
	otherRoom := current(other)
	getCounts := func() (int, int) { mu.Lock(); defer mu.Unlock(); return len(observed), configureCalls }
	unchanged := func(before map[string]any) {
		t.Helper()
		if !reflect.DeepEqual(before, current(a)) {
			t.Fatal("rejected preparation changed room")
		}
	}
	before := current(a)
	n, _ := getCounts()
	viewer.command(before, "prepare", map[string]any{"op": "select", "value": 2, "actor": 0, "host": true}, 400)
	anonymous := newClient(t, ts.URL)
	anonymous.command(before, "prepare", map[string]any{"op": "select", "value": 2}, 401)
	if calls, _ := getCounts(); calls != n {
		t.Fatal("unauthorized identity reached callback")
	}
	unchanged(before)
	// A test-only credential for the HTTP-created bot seat verifies that even
	// an authenticated identity cannot use a non-human seat to Prepare.
	botSeat := current(a)["seats"].([]any)[2].(map[string]any)
	token := randomID(24)
	if _, e = s.db.Exec("INSERT INTO users(id,name,password) VALUES(?,?,?)", botSeat["id"], "准备测试BotIdentity", "disabled-test-credential"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("INSERT INTO sessions(token,user_id,expires) VALUES(?,?,?)", digest(token), botSeat["id"], time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	botBody, _ := json.Marshal(map[string]any{"type": "prepare", "version": before["version"], "nonce": randomID(12), "action": map[string]any{"op": "select", "value": 1}})
	botRequest, _ := http.NewRequest("POST", ts.URL+"/api/rooms/"+id, bytes.NewReader(botBody))
	botRequest.AddCookie(&http.Cookie{Name: "wb_session", Value: token})
	botRequest.Header.Set("Content-Type", "application/json")
	botResponse, e := http.DefaultClient.Do(botRequest)
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, botResponse.Body)
	botResponse.Body.Close()
	if botResponse.StatusCode != 400 {
		t.Fatal("bot seat prepared", botResponse.StatusCode)
	}
	if calls, _ := getCounts(); calls != n {
		t.Fatal("authenticated bot reached preparation")
	}
	unchanged(before)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	before = current(a)
	calls, configs := getCounts()
	req := map[string]any{"type": "prepare", "version": before["version"], "nonce": randomID(12), "actor": 0, "host": true, "capacity": 99, "bots": []bool{true, true, true}, "action": map[string]any{"op": "select", "value": 4, "type": 17, "actor": 0, "host": true, "capacity": 99, "bots": []bool{true, true, true}}}
	b.post("/api/rooms/"+id, req, 400)
	unchanged(before)
	for _, key := range []string{"actor", "host", "capacity", "bots"} {
		delete(req, key)
	}
	b.post("/api/rooms/"+id, req, 200)
	after := current(a)
	trace = append(trace, after)
	if next, _ := getCounts(); next != calls+1 {
		t.Fatal("prepare did not run once")
	}
	if _, next := getCounts(); next != configs+1 {
		t.Fatal("prepared options bypassed Configure")
	}
	options := after["options"].(map[string]any)
	values := options["values"].(map[string]any)
	if values["1"] != float64(4) || values["0"] != nil || options["capacityChecked"] != float64(4) {
		t.Fatal("actor/capacity forged or configuration skipped", options)
	}
	seats := after["seats"].([]any)
	for i, v := range seats {
		seat := v.(map[string]any)
		if (seat["ready"] == true) != (i == 2) || (seat["bot"] == true) != (i == 2) {
			t.Fatal("preparation changed bot metadata or failed to clear human readiness", seats)
		}
	}
	mu.Lock()
	last := observed[len(observed)-1]
	mu.Unlock()
	if last.Actor != 1 || last.Host || last.Capacity != 4 || !reflect.DeepEqual(last.Bots, []bool{false, false, true}) {
		t.Fatal("HTTP metadata forged", last)
	}
	if !reflect.DeepEqual(otherRoom["options"], current(other)["options"]) {
		t.Fatal("preparation polluted other room")
	}
	// Callback-owned buffers cannot alter the successfully committed options.
	stable := current(a)
	mu.Lock()
	if raw := configuredRefs[len(configuredRefs)-1]; len(raw) > 0 {
		raw[0] = '?'
	}
	for _, refs := range botRefs {
		for i := range refs {
			refs[i] = true
		}
	}
	mu.Unlock()
	unchanged(stable)
	// Journal remains the authenticated command, not the callback-mutated bytes.
	var journal []byte
	if e = s.db.QueryRow("SELECT action FROM actions WHERE room_id=? AND nonce=?", id, req["nonce"]).Scan(&journal); e != nil {
		t.Fatal(e)
	}
	if !json.Valid(journal) {
		t.Fatal("callback corrupted action journal", string(journal))
	}
	a.command(current(a), "ready", nil, 200)
	replayBefore := current(a)
	n, _ = getCounts()
	b.post("/api/rooms/"+id, req, 200)
	unchanged(replayBefore)
	if calls, _ := getCounts(); calls != n {
		t.Fatal("nonce replay reran preparation")
	}
	for _, version := range []any{before["version"], int(current(a)["version"].(float64)) + 10} {
		b.post("/api/rooms/"+id, map[string]any{"type": "prepare", "version": version, "nonce": randomID(12), "action": map[string]any{"op": "select", "value": 1}}, 409)
	}
	for _, op := range []string{"error", "malformed", "null", "injection", "invalid_config", "config_reject", "shared"} {
		before = current(a)
		b.command(current(b), "prepare", map[string]any{"op": op, "value": 8}, 400)
		unchanged(before)
	}
	for _, action := range []any{nil, "not a structured selection", []int{1, 2}} {
		before = current(a)
		b.command(current(b), "prepare", action, 400)
		unchanged(before)
	}
	// Invalid outer JSON is rejected by the actual HTTP decoder.
	before = current(a)
	request, _ := http.NewRequest("POST", ts.URL+"/api/rooms/"+id, bytes.NewBufferString(`{"type":"prepare","action":`))
	request.Header.Set("Content-Type", "application/json")
	response, e := b.client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal(response.StatusCode)
	}
	unchanged(before)
	// Database rollback includes ready/version/options and nonce journal.
	before = current(a)
	if _, e = s.db.Exec("CREATE TRIGGER reject_preparation_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(FAIL,'fixture rollback'); END"); e != nil {
		t.Fatal(e)
	}
	failedReq := map[string]any{"type": "prepare", "version": before["version"], "nonce": randomID(12), "action": map[string]any{"op": "select", "value": 6}}
	b.post("/api/rooms/"+id, failedReq, 500)
	if _, e = s.db.Exec("DROP TRIGGER reject_preparation_save"); e != nil {
		t.Fatal(e)
	}
	unchanged(before)
	b.post("/api/rooms/"+id, failedReq, 200)
	a.command(current(a), "prepare", map[string]any{"op": "shared"}, 200)
	after = current(a)
	trace = append(trace, after)
	mu.Lock()
	last = observed[len(observed)-1]
	mu.Unlock()
	if last.Actor != 0 || !last.Host || !reflect.DeepEqual(last.Bots, []bool{false, false, true}) {
		t.Fatal("host/metadata changed across callbacks", last)
	}
	// Preparation survives normal server restart before players mark ready.
	saved := current(a)
	ts.Close()
	s.Close()
	s, e = New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	ts = httptest.NewServer(s.Handler())
	for _, client := range []*testClient{a, b, viewer, other} {
		client.base = ts.URL
	}
	if !reflect.DeepEqual(saved, current(a)) {
		t.Fatal("restart changed preparation")
	}
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	before = current(a)
	n, _ = getCounts()
	b.command(current(b), "prepare", map[string]any{"op": "select", "value": 3}, 400)
	unchanged(before)
	viewer.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	before = current(a)
	viewer.command(current(viewer), "prepare", map[string]any{"op": "select", "value": 3}, 400)
	unchanged(before)
	if calls, _ := getCounts(); calls != n {
		t.Fatal("playing preparation reached callback")
	}
	// A module without the extension remains a normal waiting room.
	other.command(current(other), "leave", nil, 200)
	plainRoom := other.post("/api/rooms", map[string]any{"kind": plain.ID, "name": "无准备扩展", "capacity": 2}, 201)
	other.command(plainRoom, "prepare", map[string]any{"op": "select", "value": 3}, 400)
	mu.Lock()
	defer mu.Unlock()
	pointers := map[*bool]bool{}
	for _, v := range botRefs {
		if len(v) != 3 {
			t.Fatal("Bots padded to capacity or lost seats")
		}
		if pointers[&v[0]] {
			t.Fatal("callbacks share Bots metadata")
		}
		pointers[&v[0]] = true
	}
	t.Logf("verified %d detached preparation callbacks through authenticated HTTP, options validation, transactional rollback and restart", len(observed))
}
