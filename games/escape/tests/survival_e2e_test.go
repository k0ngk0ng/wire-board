package escape_test

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestEscapeGenerationCorpusE2E(t *testing.T) {
	// Inspect generated rule state for structural guarantees, never to choose a
	// player action. Player and bot behavior is covered by complete campaigns.
	m := module(t)
	fingerprints := map[string]bool{}
	cases := 0
	for _, scene := range []string{"station", "archive"} {
		for seed := 0; seed < 1000; seed++ {
			s := start(t, m, 2, scene, "normal", fmt.Sprint("corpus-", seed))
			var state map[string]any
			if e := json.Unmarshal(s.Data, &state); e != nil {
				t.Fatal(e)
			}
			rooms := array(state, "rooms")
			seen := map[string]bool{"r4": true}
			queue := []string{"r4"}
			for len(queue) > 0 {
				id := queue[0]
				queue = queue[1:]
				for _, raw := range rooms {
					r := obj(raw)
					if text(r, "id") != id {
						continue
					}
					for _, raw := range array(r, "doors") {
						next := text(obj(raw), "room")
						if !seen[next] {
							seen[next] = true
							queue = append(queue, next)
						}
					}
				}
			}
			if len(seen) != 9 {
				t.Fatal("disconnected generated facility", scene, seed)
			}
			required := map[string]bool{}
			for _, raw := range rooms {
				r := obj(raw)
				required[text(r, "kind")] = true
			}
			for _, k := range []string{"entry", "exit", "power", "pump", "signal", "medical", "storage", "relay", "records"} {
				if !required[k] {
					t.Fatal("missing prerequisite room", k)
				}
			}
			raw, _ := json.Marshal(rooms)
			fingerprints[string(raw)] = true
			cases++
		}
	}
	if len(fingerprints) < 1800 {
		t.Fatal("generation lacks variation", len(fingerprints))
	}
	evidence(t, "generation-corpus", map[string]any{"passed": true, "cases": cases, "distinctLayouts": len(fingerprints), "seeds": "corpus-0 through corpus-999 in each scenario", "checks": []string{"connected graph", "all prerequisite rooms", "two-person compatibility", "layout variation"}})
}

func TestEscapeDangerWarningRescueAndTimeoutE2E(t *testing.T) {
	m := module(t)
	x := expedition{t, m, start(t, m, 2, "station", "hard", "rescue-31"), epochTime}
	// Discover through normal doors, so rescue planning has no hidden map.
	for x.s.Result == nil {
		frontier := ""
		for _, r := range x.rooms() {
			if !boolv(r, "known") && len(x.path(text(r, "id"))) > 0 {
				frontier = text(r, "id")
				break
			}
		}
		if frontier == "" {
			break
		}
		x.goRoom(frontier)
	}
	warned := false
	for round := 0; round < 8 && x.s.Result == nil && number(obj(array(x.view(), "players")[0]), "hp") > 0; round++ {
		for x.view()["incident"] == nil && x.s.Result == nil {
			x.advance(1000)
		}
		incident := obj(x.view()["incident"])
		if len(incident) == 0 {
			break
		}
		if number(incident, "begins")-number(x.view(), "tick") > 0 {
			warned = true
		}
		// Move the rescue partner away before the announced incident starts.
		partner := obj(array(x.view(), "players")[1])
		if text(partner, "room") == text(incident, "room") {
			for _, r := range x.rooms() {
				if text(r, "id") == text(partner, "room") {
					d := obj(array(r, "doors")[0])
					var e error
					x.s, e = action(t, m, x.s, 1, map[string]any{"type": "travel", "target": d["room"]}, x.now)
					if e != nil {
						t.Fatal(e)
					}
					break
				}
			}
		}
		x.goRoom(text(incident, "room"))
		until := number(incident, "ends") + 100
		for number(x.view(), "tick") < until && x.s.Result == nil {
			x.advance(100)
		}
	}
	if !warned || number(obj(array(x.view(), "players")[0]), "hp") > 0 || x.s.Result != nil {
		t.Fatal("did not reach recoverable downed state through announced incidents")
	}
	before := x.view()
	for step := 0; step < 500 && number(obj(array(x.view(), "players")[0]), "hp") <= 0; step++ {
		ctx := gameapi.Context{Actor: 1, Players: 2, Now: x.now}
		raw, e := m.BotAction(ctx, x.s)
		if e != nil {
			t.Fatal(e)
		}
		if len(raw) > 0 {
			x.s, e = m.Apply(ctx, x.s, raw)
			if e != nil {
				t.Fatal(e)
			}
		}
		x.advance(100)
	}
	if number(obj(array(x.view(), "players")[0]), "hp") <= 0 || number(obj(x.view()["stats"]), "rescues") < 1 {
		t.Fatal("ordinary rescue did not restore teammate")
	}
	evidence(t, "danger-rescue", map[string]any{"passed": true, "warningObserved": warned, "before": before, "rescued": x.view()})
	// A completely inactive team eventually reaches a shared terminal result.
	s := start(t, m, 2, "archive", "hard", "timeout-11")
	now := epochTime
	for i := 0; i < 9500 && s.Result == nil; i++ {
		now = now.Add(100 * time.Millisecond)
		var e error
		s, e = m.OnTimer(gameapi.Context{Players: 2, Now: now}, s)
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.Result == nil || !s.Result.Cooperative || len(s.Result.Winners) != 0 {
		t.Fatal("inactive survival campaign did not end in shared defeat")
	}
	evidence(t, "survival-timeout", map[string]any{"passed": true, "view": readView(t, m, s, 0), "result": s.Result})
}
