package pvz_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type backendRun struct {
	t       *testing.T
	m       gameapi.Module
	match   gameapi.Match
	now     time.Time
	players int
	trace   []any
}

func backendNew(t *testing.T, n int) *backendRun {
	t.Helper()
	b := &backendRun{t: t, m: pvz.Module(), now: time.Unix(1800000000, 0), players: n}
	mode := "duel"
	if n == 4 {
		mode = "team"
	}
	raw, _ := json.Marshal(map[string]any{"mode": mode, "roundSeconds": 180, "prepSeconds": 10})
	var err error
	b.match, err = b.m.Start(b.ctx(-1), raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		out := filepath.Join("..", "..", "..", ".local", "pvz", "backend")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "public module/engine action and clock integration; focused combat fixture explicitly labelled", "trace": b.trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	return b
}
func (b *backendRun) ctx(actor int) gameapi.Context {
	return gameapi.Context{Actor: actor, Players: b.players, Now: b.now, Version: 1, RequestedVersion: 1}
}
func (b *backendRun) view() pvz.View {
	b.t.Helper()
	x, e := b.m.View(b.match, 0)
	if e != nil {
		b.t.Fatal(e)
	}
	return x.(pvz.View)
}
func (b *backendRun) act(actor int, a engine.Action, wantErr bool) {
	b.t.Helper()
	v := b.view()
	if a.Epoch == "" {
		a.Epoch = v.Epoch
	}
	if a.Seq == 0 && actor >= 0 && actor < len(v.Players) {
		a.Seq = v.Players[actor].Seq + 1
	}
	raw, _ := json.Marshal(a)
	before := append([]byte(nil), b.match.Data...)
	next, e := b.m.Apply(b.ctx(actor), b.match, raw)
	if (e != nil) != wantErr {
		b.t.Fatalf("actor=%d action=%+v err=%v wantErr=%v", actor, a, e, wantErr)
	}
	if e != nil {
		if !bytes.Equal(before, b.match.Data) {
			b.t.Fatal("failed action mutated persisted input")
		}
		return
	}
	b.match = next
	b.trace = append(b.trace, map[string]any{"action": a, "actor": actor, "tick": b.view().Tick})
}
func (b *backendRun) advance(ms int) {
	b.t.Helper()
	for ms > 0 {
		step := min(ms, 50)
		b.now = b.now.Add(time.Duration(step) * time.Millisecond)
		var e error
		b.match, e = b.m.OnTimer(b.ctx(-1), b.match)
		if e != nil {
			b.t.Fatal(e)
		}
		ms -= step
	}
}

func TestPVZBackendConfigurationAndWallets(t *testing.T) {
	m := pvz.Module()
	for _, bad := range []struct {
		n   int
		raw string
	}{{3, `{}`}, {2, `{"mode":"team"}`}, {4, `{"mode":"duel"}`}, {4, `{"mode":"team","teams":[0,0,0,1]}`}, {2, `{"roundSeconds":1}`}, {2, `{"prepSeconds":0}`}, {2, `{"scenario":"free-money"}`}} {
		if _, e := m.Configure(bad.n, json.RawMessage(bad.raw)); e == nil {
			t.Fatalf("accepted invalid %d %s", bad.n, bad.raw)
		}
	}
	b := backendNew(t, 2)
	team := backendNew(t, 4)
	v, tv := b.view(), team.view()
	for side := range 2 {
		solo, total := 0, 0
		for _, p := range v.Players {
			if p.Team == side {
				solo += p.Money
			}
		}
		for _, p := range tv.Players {
			if p.Team == side {
				total += p.Money
			}
		}
		if solo != total {
			t.Fatalf("team%d initial economy %d != %d", side, solo, total)
		}
	}
	b.act(0, engine.Action{Type: "plant", Kind: "sunflower", Row: 0, Col: 0}, false)
	v = b.view()
	if v.Players[0].Money >= 300 || v.Players[1].Money != 250 || len(v.Plants) != 1 {
		t.Fatal("wrong independent wallet debit")
	}
	before := v.Players[0].Money
	b.act(0, engine.Action{Type: "plant", Kind: "peashooter", Row: 0, Col: 0}, true)
	if b.view().Players[0].Money != before {
		t.Fatal("occupied tile charged money")
	}
	b.act(1, engine.Action{Type: "plant", Kind: "sunflower", Row: 1, Col: 0}, true)
	b.act(0, engine.Action{Type: "spawn", Kind: "zombie", Row: 1}, true)
	b.act(1, engine.Action{Type: "spawn", Kind: "zombie", Row: 1}, true)
	b.act(-1, engine.Action{Type: "plant", Kind: "sunflower", Row: 1, Col: 0, Seq: 1}, true)
	team.act(0, engine.Action{Type: "plant", Kind: "sunflower", Row: 0, Col: 0}, false)
	team.act(2, engine.Action{Type: "shovel", PlantID: team.view().Plants[0].ID}, true)
	team.act(0, engine.Action{Type: "shovel", PlantID: team.view().Plants[0].ID}, false)
	b.advance(10000)
	team.advance(10000)
	if b.view().Phase != "battle" {
		t.Fatal("clock did not start battle")
	}
	b.act(1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0}, false)
	after := b.view()
	seq := after.Players[1].Seq
	b.act(1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0, Seq: seq}, true)
	if b.view().Players[1].Money != after.Players[1].Money {
		t.Fatal("repeat action charged twice")
	}
	b.act(0, engine.Action{Type: "plant", Kind: "sunflower", Row: -1, Col: 0}, true)
	b.act(0, engine.Action{Type: "plant", Kind: "unknown", Row: 1, Col: 0}, true)
	b.trace = append(b.trace, b.view())
}

func TestPVZBackendWholeMatchBotsAndRestore(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := backendNew(t, n)
			actions := 0
			roundsSeen := map[int]bool{}
			for frames := 0; frames < 8500 && b.match.Result == nil; frames++ {
				v := b.view()
				roundsSeen[v.Round] = true
				for p := range n {
					raw, e := b.m.BotAction(b.ctx(p), b.match)
					if e != nil {
						t.Fatal(e)
					}
					if len(raw) > 0 {
						next, e := b.m.Apply(b.ctx(p), b.match, raw)
						if e != nil {
							t.Fatalf("illegal bot action %s: %v", raw, e)
						}
						b.match = next
						actions++
					}
				}
				if frames == 350 {
					before := b.view().Snapshot
					b.now = b.now.Add(5 * time.Hour)
					restored, e := b.m.Restore(b.ctx(-1), b.match)
					if e != nil {
						t.Fatal(e)
					}
					b.match = restored
					if !reflect.DeepEqual(before, b.view().Snapshot) {
						t.Fatal("restart changed simulation or caught up offline")
					}
					restored, e = b.m.Reclaim(b.ctx(0), b.match)
					if e != nil {
						t.Fatal(e)
					}
					b.match = restored
					if !reflect.DeepEqual(before, b.view().Snapshot) {
						t.Fatal("manual reclaim changed simulation")
					}
				}
				b.advance(50)
			}
			if b.match.Result == nil || actions < 20 || len(roundsSeen) != 2 {
				t.Fatalf("bots did not complete both rounds: actions=%d phase=%s rounds=%v", actions, b.view().Phase, roundsSeen)
			}
			if e := b.match.Validate(n); e != nil {
				t.Fatal(e)
			}
			result := b.match.Result
			if n == 4 && !result.Team {
				t.Fatal("2v2 result not team")
			}
			if result.Draw && len(result.Winners) != 0 {
				t.Fatal("draw has winners")
			}
			b.trace = append(b.trace, map[string]any{"actions": actions, "final": b.view(), "result": result})
		})
	}
}

func TestPVZBackendReplayCloneAndStream(t *testing.T) {
	b := backendNew(t, 2)
	s := b.view().Snapshot
	copy := engine.Clone(s)
	copy.Players[0].Cooldowns["sunflower"] = 999
	if s.Players[0].Cooldowns["sunflower"] == 999 {
		t.Fatal("clone shares cooldown map")
	}
	copy = engine.Clone(s)
	a := engine.Action{Type: "plant", Kind: "peashooter", Row: 2, Col: 0, Epoch: s.Epoch, Seq: 1}
	if err := engine.Apply(&s, 0, a); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(&copy, 0, a); err != nil {
		t.Fatal(err)
	}
	if err := engine.Step(&s, 500); err != nil {
		t.Fatal(err)
	}
	for range 500 {
		if err := engine.Step(&copy, 1); err != nil {
			t.Fatal(err)
		}
	}
	if engine.Digest(s) != engine.Digest(copy) {
		t.Fatal("chunked deterministic replay diverged")
	}
	first, e := b.m.StreamView(b.match, 0, "")
	if e != nil {
		t.Fatal(e)
	}
	b.act(0, a, false)
	delta, e := b.m.StreamView(b.match, 0, first.Cursor)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(delta.Data)
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if decoded["snapshot"] != nil || decoded["plants"] != nil || decoded["catalog"] != nil {
		t.Fatalf("routine stream repeated static/full simulation data: %s", raw)
	}
	if decoded["applied"] == nil {
		t.Fatal("delta omitted accepted operation")
	}
	x, e := b.m.View(b.match, -1)
	if e != nil || x.(pvz.View).CanAct {
		t.Fatal("spectator can act")
	}
	b.trace = append(b.trace, map[string]any{"fullCursor": first.Cursor, "delta": decoded, "digest": engine.Digest(s)})
}

func backendCombatState(t *testing.T) engine.State {
	t.Helper()
	s, e := engine.New(engine.Options{Mode: "duel", RoundSeconds: 180, PrepSeconds: 10}, 2, "combat-fixture")
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.Step(&s, 200); e != nil {
		t.Fatal(e)
	}
	s.Players[0].Money = 10000
	s.Players[1].Money = 10000
	return s
}
func backendEngineAct(t *testing.T, s *engine.State, p int, a engine.Action) {
	t.Helper()
	a.Epoch = s.Epoch
	a.Seq = s.Players[p].Seq + 1
	if e := engine.Apply(s, p, a); e != nil {
		t.Fatal(e)
	}
}
func TestPVZBackendCombatMechanics(t *testing.T) {
	t.Run("armor-and-slow", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "snowpea", Row: 2, Col: 0})
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "conehead", Row: 2})
		armor := s.Zombies[0].Armor
		_ = engine.Step(&s, 100)
		if len(s.Zombies) != 1 || s.Zombies[0].Armor >= armor || s.Zombies[0].HP != s.Zombies[0].MaxHP || s.Zombies[0].SlowUntil <= s.Tick {
			t.Fatal("ice projectile must remove armor and slow before damaging body")
		}
	})
	t.Run("pole-tallnut-and-single-vault", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "tallnut", Row: 1, Col: 7})
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "polevaulter", Row: 1})
		_ = engine.Step(&s, 250)
		if len(s.Zombies) != 1 || s.Zombies[0].X < 7500 {
			t.Fatal("pole vaulter crossed tallnut")
		}
		s = backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "wallnut", Row: 1, Col: 7})
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "polevaulter", Row: 1})
		_ = engine.Step(&s, 250)
		if len(s.Zombies) != 1 || !s.Zombies[0].Vaulted || s.Zombies[0].X >= 7500 {
			t.Fatal("pole vaulter did not vault ordinary plant")
		}
	})
	t.Run("mine-and-cherry", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "potatomine", Row: 3, Col: 7})
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 3})
		_ = engine.Step(&s, 20)
		if len(s.Zombies) != 1 || s.Zombies[0].HP != s.Zombies[0].MaxHP {
			t.Fatal("unarmed mine detonated")
		}
		s = backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "potatomine", Row: 3, Col: 7})
		_ = engine.Step(&s, 300)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 3})
		s.Zombies[0].X = 7500
		_ = engine.Step(&s, 1)
		if len(s.Zombies) != 0 {
			t.Fatal("armed mine failed to kill close zombie")
		}
		s = backendCombatState(t)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "buckethead", Row: 2})
		s.Zombies[0].X = 7500
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "cherrybomb", Row: 2, Col: 7})
		_ = engine.Step(&s, 30)
		if len(s.Zombies) != 0 {
			t.Fatal("cherry blast failed")
		}
	})
	t.Run("mower-and-round-epoch", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0})
		s.Zombies[0].X = 50
		_ = engine.Step(&s, 10)
		if s.Mowers[0].State == "ready" || len(s.Zombies) > 0 {
			t.Fatal("mower did not clear intruder")
		}
		old := s.Epoch
		s.Mowers[0].State = "spent"
		s.Mowers[0].X = 10000
		s.Players[1].Cooldowns["zombie"] = 0
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0})
		s.Zombies[0].X = -1
		_ = engine.Step(&s, 1)
		if s.Phase != "intermission" || len(s.Rounds) != 1 || !s.Rounds[0].Breached {
			t.Fatal("unprotected house did not end round")
		}
		_ = engine.Step(&s, 200)
		if s.Round != 2 || s.Epoch == old || s.Players[0].Side != "zombies" || len(s.Plants) != 0 {
			t.Fatal("round switch failed")
		}
		a := engine.Action{Type: "spawn", Kind: "zombie", Row: 1, Epoch: old, Seq: 1}
		if engine.Apply(&s, 0, a) == nil {
			t.Fatal("old epoch accepted")
		}
	})
}

func TestPVZBackendOriginalCounterRangesReplay(t *testing.T) {
	firstTimes := map[int64]bool{}
	for i := 0; i < 6; i++ {
		s, e := engine.New(engine.Options{Mode: "duel", RoundSeconds: 180, PrepSeconds: 10}, 2, fmt.Sprintf("verified-counter-seed-%d", i))
		if e != nil {
			t.Fatal(e)
		}
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "sunflower", Row: 0, Col: 0})
		first := s.Plants[0].ReadyAt - s.Tick
		firstTimes[first] = true
		if first < 60 || first > 250 {
			t.Fatalf("sunflower first production outside REA range: %d ticks", first)
		}
		copy := engine.Clone(s)
		_ = engine.Step(&s, int(first))
		for range first {
			_ = engine.Step(&copy, 1)
		}
		if engine.Digest(s) != engine.Digest(copy) {
			t.Fatal("production random state not deterministic")
		}
		if len(s.Plants) != 1 {
			t.Fatal("producer disappeared")
		}
		next := s.Plants[0].ReadyAt - s.Tick
		if next < 470 || next > 500 {
			t.Fatalf("sunflower repeat production outside REA range: %d ticks", next)
		}
	}
	if len(firstTimes) < 2 {
		t.Fatal("sunflower ignores original randomized first-production counter")
	}
}

func TestPVZBackendOriginalMovementAndBiteCadence(t *testing.T) {
	t.Run("football-period-average", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "football", Row: 4})
		x := s.Zombies[0].X
		_ = engine.Step(&s, 20)
		distance := x - s.Zombies[0].X
		if distance < 385 || distance > 402 {
			t.Fatalf("football one-second motion %d units, want verified period-average 385..402", distance)
		}
	})
	t.Run("newspaper-rage-period-average", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "newspaper", Row: 4})
		s.Zombies[0].Armor = 0
		x := s.Zombies[0].X
		_ = engine.Step(&s, 20)
		distance := x - s.Zombies[0].X
		if distance < 520 || distance > 540 {
			t.Fatalf("angry newspaper one-second motion %d units, want verified period-average 520..540", distance)
		}
	})
	t.Run("small-bites-then-100hp-per-second", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 0, engine.Action{Type: "plant", Kind: "wallnut", Row: 1, Col: 7})
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 1})
		s.Zombies[0].X = 7900
		hp := s.Plants[0].HP
		_ = engine.Step(&s, 1)
		damage := hp - s.Plants[0].HP
		if damage < 4 || damage > 8 {
			t.Fatalf("contact bite lumpy: got %d want 4..8", damage)
		}
		_ = engine.Step(&s, 19)
		if hp-s.Plants[0].HP != 100 {
			t.Fatalf("wrong continuous bite rate: %d", hp-s.Plants[0].HP)
		}
	})
}
