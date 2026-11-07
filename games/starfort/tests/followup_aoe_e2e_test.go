package starfort_test

// Before the fix: the main target dies to a cannon's ordinary splash, then its
// bonus blast hits a surviving secondary target already stored in DamageGroups.
// The encoder appends that victim twice, invalidates the next match and the host
// retries the same saved timer forever. Also cover a surviving main target,
// armor/shield victims, skill identity, conserved damage, restored continuation,
// and repeated future ticks. The captured production fixture contains public
// module state only; account IDs, names and room metadata were not retained.

import (
	_ "embed"
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

//go:embed testdata/followup-aoe-stalled-match.json
var followupStalledMatch []byte

func TestStarfortFollowupAOEProductionHTTP(t *testing.T) {
	m := starfort.Module()
	m.Start = func(ctx gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		var match gameapi.Match
		if err := json.Unmarshal(followupStalledMatch, &match); err != nil {
			return match, err
		}
		var s starfort.State
		if err := json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		s.ServerAt = ctx.Now.UnixMilli()
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	waitTick := func(previous float64) {
		deadline := time.Now().Add(8 * time.Second)
		for data(a.room())["tick"].(float64) <= previous {
			if time.Now().After(deadline) {
				t.Fatalf("production room timer stayed at tick %g", previous)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	tb.record("captured production battle restored through authenticated HTTP and real SQLite")
	waitTick(356)
	saved := data(a.room())["tick"].(float64)
	tb.record("first previously failing timer committed")
	tb.http.Close()
	tb.host.Close()
	host, err := server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Starfort captured production recovery")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.host = host
	tb.http = httptest.NewServer(host.Handler())
	for _, c := range append(tb.people, tb.spectator) {
		c.base = tb.http.URL
	}
	if data(a.room())["tick"].(float64) < saved {
		t.Fatal("restart reset captured battle")
	}
	waitTick(saved)
	tb.record("persisted recovered battle continues after service restart")
}

func TestStarfortFollowupAOEProductionContinuation(t *testing.T) {
	b := newRulesTable(t, 2, map[string]any{})
	if err := json.Unmarshal(followupStalledMatch, &b.s); err != nil {
		t.Fatal(err)
	}
	s := cadenceState(b)
	b.now = time.UnixMilli(s.ServerAt)
	b.validate()
	b.record(map[string]any{"op": "production_saved_match_fixture", "scope": "exact public battle state, historical event ring removed, server clock normalized", "match": b.s})
	for second := 0; second < 60 && b.s.Result == nil; second++ {
		b.timer(time.Second)
	}
	if cadenceState(b).Tick <= s.Tick {
		t.Fatal("restored production battle never advanced")
	}
}

func TestStarfortFollowupAOEMortality(t *testing.T) {
	for _, kind := range []string{"slime", "armored", "shield", "elite", "boss"} {
		for _, deadMain := range []bool{false, true} {
			name := kind + "-main-alive"
			if deadMain {
				name = kind + "-main-killed"
			}
			t.Run(name, func(t *testing.T) {
				b := specializationCase(t, "cannon", "power", "slime", kind)
				s := cadenceState(b)
				if deadMain {
					s.Enemies[0].HP = 1
				}
				if kind == "armored" {
					s.Enemies[1].Armor = .55
				}
				if kind == "shield" {
					s.Enemies[1].Shield = 1000
				}
				cadenceSave(b, s)
				before := s.Players[0].Damage
				b.timer(time.Second)
				after := cadenceState(b)
				if specializationEvents(b, "blast") == 0 {
					t.Fatal("blast fixture did not proc")
				}
				// Every actual hit must remain represented once, even if its
				// target has already appeared in another same-time spell.
				var encoded float64
				for _, e := range after.Events {
					if e.Type == "hit" {
						encoded += e.Amount
						for _, d := range e.Damage {
							encoded += d.Amount
						}
						for _, g := range e.DamageGroups {
							encoded += g.Amount * float64(len(g.Targets))
						}
					}
				}
				if math.Abs(encoded-(after.Players[0].Damage-before)) > 1e-6 {
					t.Fatalf("damage delta lost or duplicated: events=%g actual=%g", encoded, after.Players[0].Damage-before)
				}
				for i := 0; i < 10 && b.s.Result == nil; i++ {
					b.timer(time.Second)
				}
			})
		}
	}
}

func TestStarfortFollowupAOESkillIdentity(t *testing.T) {
	b := specializationCase(t, "arcane", "magic", "slime", "armored")
	s := cadenceState(b)
	s.Enemies[0].HP = 1
	cadenceSave(b, s)
	b.timer(time.Second)
	found := false
	for _, e := range cadenceState(b).Events {
		if e.Type == "hit" && e.Skill == "meteor" {
			found = true
		}
	}
	if !found {
		t.Fatal("meteor damage merged into the preceding ordinary hit")
	}
}
