package server

// Failure scenarios specified before implementation: a returning player's own
// deadline expired after another still-pending deadline, so the host never asks
// Reclaim and manual actions remain rejected; broadening Reclaim resets another
// player's budget or an unexpired/submitted/paused window; replay, spectator,
// failed storage and restart change deadlines/control inconsistently; legacy
// modules with unconditional Reclaim receive new calls and reset live budgets.
// Real authenticated HTTP commands create all fixture states. Only the explicit
// fixture's expiry command changes its game-owned deadlines; host state is never
// overwritten to manufacture the non-earliest timeout case.
import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type reclaimCancelData struct {
	Due       []int64 `json:"due"`
	Submitted []bool  `json:"submitted"`
	Moves     []int   `json:"moves"`
	Reclaims  int     `json:"reclaims"`
}

func reclaimCancelEnvelope(v gameapi.Match, d reclaimCancelData) gameapi.Match {
	v.Data, _ = json.Marshal(d)
	v.Actors = []int{0, 1}
	v.Attention = []int{}
	v.TimeoutActors = nil
	v.Deadline = 0
	if !v.Paused {
		for i, due := range d.Due {
			if d.Submitted[i] {
				continue
			}
			v.Attention = append(v.Attention, i)
			if v.Deadline == 0 || due < v.Deadline {
				v.Deadline = due
				v.TimeoutActors = []int{i}
			} else if due == v.Deadline {
				v.TimeoutActors = append(v.TimeoutActors, i)
			}
		}
	}
	return v
}

func reclaimCancelFixture(independent bool) gameapi.Module {
	m := acceptanceModule()
	m.ReclaimOnCancel = independent
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		d := reclaimCancelData{Due: []int64{c.Now.Add(180 * time.Second).UnixMilli(), c.Now.Add(120 * time.Second).UnixMilli()}, Submitted: []bool{false, false}, Moves: []int{0, 0}}
		return reclaimCancelEnvelope(gameapi.Match{}, d), nil
	}
	m.View = func(v gameapi.Match, _ int) (any, error) {
		var d reclaimCancelData
		err := json.Unmarshal(v.Data, &d)
		return d, err
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var a struct {
			Type string `json:"type"`
		}
		var d reclaimCancelData
		if json.Unmarshal(raw, &a) != nil || json.Unmarshal(v.Data, &d) != nil {
			return v, errors.New("invalid fixture state/action")
		}
		switch a.Type {
		case "expire":
			d.Due = []int64{c.Now.Add(-time.Second).UnixMilli(), c.Now.Add(-2 * time.Second).UnixMilli()}
		case "submit":
			d.Submitted[c.Actor] = true
		case "pause":
			v.Paused = true
		case "advance":
			if v.Paused || d.Submitted[c.Actor] || d.Due[c.Actor] <= c.Now.UnixMilli() {
				return v, errors.New("own action deadline has expired")
			}
			d.Moves[c.Actor]++
		default:
			return v, errors.New("unknown fixture action")
		}
		return reclaimCancelEnvelope(v, d), nil
	}
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	m.Reclaim = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		var d reclaimCancelData
		if err := json.Unmarshal(v.Data, &d); err != nil {
			return v, err
		}
		d.Reclaims++
		if !independent || (!v.Paused && !d.Submitted[c.Actor] && d.Due[c.Actor] <= c.Now.UnixMilli()) {
			d.Due[c.Actor] = c.Now.Add(90 * time.Second).UnixMilli()
		}
		return reclaimCancelEnvelope(v, d), nil
	}
	return m
}

func reclaimCancelTable(t *testing.T, m gameapi.Module) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("independent action budgets")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "acceptance")
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("独立时限甲")
	b.register("独立时限乙")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "独立时限", "capacity": 2}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	return s, ts, []*testClient{a, b}, r["id"].(string)
}

func reclaimCancelState(t *testing.T, c *testClient) reclaimCancelData {
	t.Helper()
	raw, _ := json.Marshal(current(c)["game"].(map[string]any)["data"])
	var d reclaimCancelData
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func reclaimCancelProof(t *testing.T, name string, v any) {
	t.Helper()
	out := filepath.Join("..", "..", ".local", "module-reclaim-cancel")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(filepath.Join(out, name+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestModuleIndependentDeadlineReclaimHTTP(t *testing.T) {
	s, ts, clients, id := reclaimCancelTable(t, reclaimCancelFixture(true))
	a, b := clients[0], clients[1]
	a.command(current(a), "action", map[string]any{"type": "expire"}, 200)
	before := reclaimCancelState(t, a)
	if before.Due[0] <= before.Due[1] || before.Due[0] >= time.Now().UnixMilli() || !reflect.DeepEqual(s.rooms[id].Module.TimeoutActors, []int{1}) {
		t.Fatal("fixture lacks two expired windows with later returning seat", before, s.rooms[id].Module.TimeoutActors)
	}
	setAutoPlay(a, current(a), true, 200)
	controlled := current(a)
	if _, err := s.db.Exec("CREATE TRIGGER fail_reclaim_cancel BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'reclaim rollback'); END"); err != nil {
		t.Fatal(err)
	}
	request := map[string]any{"type": "autoplay", "enabled": false, "version": controlled["version"], "nonce": "independent-reclaim-once"}
	a.post("/api/rooms/"+id, request, 500)
	if _, err := s.db.Exec("DROP TRIGGER fail_reclaim_cancel"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(controlled, current(a)) {
		t.Fatal("failed reclaim changed control or deadlines")
	}
	a.post("/api/rooms/"+id, request, 200)
	after := reclaimCancelState(t, a)
	if after.Reclaims != 1 || after.Due[0] <= time.Now().UnixMilli() {
		t.Fatal("later expired seat cancelled autoplay without restoring its own window", before, after)
	}
	if after.Due[1] != before.Due[1] || s.rooms[id].Module.Deadline != before.Due[1] || !reflect.DeepEqual(s.rooms[id].Module.TimeoutActors, []int{1}) {
		t.Fatal("reclaim changed another seat's earlier pending window", after)
	}
	if s.rooms[id].Seats[0].AutoPlay {
		t.Fatal("returning seat still automated")
	}
	reclaimed := current(a)
	a.post("/api/rooms/"+id, request, 200)
	if !reflect.DeepEqual(reclaimed, current(a)) {
		t.Fatal("duplicate cancellation reapplied reclaim")
	}
	a.command(current(a), "action", map[string]any{"type": "advance"}, 200)
	if reclaimCancelState(t, a).Moves[0] != 1 {
		t.Fatal("returning player still cannot act")
	}
	// B is still the globally due actor; taking over A cannot make B legal.
	b.command(current(b), "action", map[string]any{"type": "advance"}, 400)
	var raw []byte
	if err := s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored Room
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	var saved reclaimCancelData
	if err := json.Unmarshal(stored.Module.Data, &saved); err != nil {
		t.Fatal(err)
	}
	if stored.Seats[0].AutoPlay || saved.Due[0] != after.Due[0] || saved.Due[1] != before.Due[1] {
		t.Fatal("reclaimed control/budgets were not persisted")
	}
	// Reopen the actual SQLite database and continue using the same authenticated
	// session. Restore cannot reset the independently recovered deadline.
	ts.Close()
	s.Close()
	restarted, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(restarted)
	ts2 := httptest.NewServer(restarted.Handler())
	defer ts2.Close()
	defer restarted.Close()
	for _, client := range clients {
		client.base = ts2.URL
	}
	restored := reclaimCancelState(t, a)
	if !reflect.DeepEqual(saved, restored) || restarted.rooms[id].Seats[0].AutoPlay {
		t.Fatal("restart changed reclaimed control or independent deadlines", saved, restored)
	}
	a.command(current(a), "action", map[string]any{"type": "advance"}, 200)
	if reclaimCancelState(t, a).Moves[0] != 2 {
		t.Fatal("restored manual control cannot continue")
	}
	reclaimCancelProof(t, "independent-expired", map[string]any{"test": t.Name(), "passed": !t.Failed(), "before": before, "after": after, "stored": stored, "restored": restored, "continued": current(a)})
}

func TestModuleReclaimCancelPreservesOtherStatesHTTP(t *testing.T) {
	for _, mode := range []string{"live", "submitted", "paused", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := reclaimCancelTable(t, reclaimCancelFixture(mode != "legacy"))
			a := clients[0]
			if mode == "submitted" {
				a.command(current(a), "action", map[string]any{"type": "submit"}, 200)
			}
			if mode == "paused" {
				a.command(current(a), "action", map[string]any{"type": "pause"}, 200)
			}
			before := reclaimCancelState(t, a)
			setAutoPlay(a, current(a), true, 200)
			spectator := newClient(t, ts.URL)
			spectator.register("时限旁观者")
			spectator.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			setAutoPlay(spectator, current(spectator), false, 400)
			setAutoPlay(a, current(a), false, 200)
			after := reclaimCancelState(t, a)
			if !slices.Equal(before.Due, after.Due) || !slices.Equal(before.Submitted, after.Submitted) || !slices.Equal(before.Moves, after.Moves) {
				t.Fatal("cancel changed live/submitted/paused state", before, after)
			}
			want := 1
			if mode == "legacy" {
				want = 0
			}
			if after.Reclaims != want {
				t.Fatal("incorrect optional reclaim dispatch", after.Reclaims, want)
			}
			setAutoPlay(a, current(a), false, 200)
			if reclaimCancelState(t, a).Reclaims != want {
				t.Fatal("cancel while already manual invoked reclaim")
			}
			if mode == "paused" && (!s.rooms[id].Module.Paused || s.rooms[id].TurnDeadline != 0) {
				t.Fatal("cancel resumed paused match")
			}
			reclaimCancelProof(t, mode, map[string]any{"test": t.Name(), "passed": !t.Failed(), "before": before, "after": after, "room": current(a)})
		})
	}
}
