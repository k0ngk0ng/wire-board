package guandan

// Regression failure cases: first-hand A cannot finish; a restored last-play
// key carries forged strength; reclaim extends unrelated simultaneous clocks;
// bot decisions depend on opponents' hidden hands.
import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestStartingAtAceCanPassAce(t *testing.T) {
	for _, order := range [][]int{{0, 2}, {0, 1, 2}, {0, 1, 3}} {
		s := State{Options: Options{StartLevel: 14, TurnSeconds: 30}, Round: 1, Level: 14, Levels: []int{14, 14}, DealerTeam: -1, FinishOrder: order, Due: make([]int64, 4)}
		s.finishRound(time.Unix(1000, 0))
		qualifies := order[len(order)-1] == 2
		if qualifies != (s.Phase == "finished" && s.FinishReason == "passed_ace") {
			t.Fatalf("order %v: %s %s", order, s.Phase, s.FinishReason)
		}
	}
}

func TestRestoredCombinationMustMatchDeclaration(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 4, Now: time.Unix(1000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = m.Actors[0]
	a, e := mod.BotAction(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	m, e = mod.Apply(ctx, m, a)
	if e != nil {
		t.Fatal(e)
	}
	var s State
	_ = json.Unmarshal(m.Data, &s)
	s.LastPlay.High += 100
	m.Data, _ = json.Marshal(s)
	if _, e = mod.View(m, 0); e == nil {
		t.Fatal("accepted forged strength with valid key")
	}
}

func TestReclaimOnlyOwnDeadline(t *testing.T) {
	s := State{Phase: "round_end", Options: Options{TurnSeconds: 30}, Ready: make([]bool, 4), Due: make([]int64, 4)}
	now := time.Unix(1000, 0)
	s.nextWindow(now)
	original := s.Due[1]
	// Exercise the actual callback with a naturally completed limited match's
	// intermediate round; no hidden snapshot modifications are needed.
	mod := Module()
	ctx := gameapi.Context{Players: 4, Now: now}
	m, e := mod.Start(ctx, json.RawMessage(`{"rounds":3}`))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 1000; i++ {
		_ = json.Unmarshal(m.Data, &s)
		if s.Phase == "round_end" {
			break
		}
		ctx.Actor = m.Actors[0]
		a, err := mod.BotAction(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		m, e = mod.Apply(ctx, m, a)
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.Phase != "round_end" {
		t.Fatal("no intermediate round")
	}
	original = s.Due[1]
	ctx.Actor = 0
	ctx.Now = now.Add(5 * time.Second)
	next, e := mod.Reclaim(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	_ = json.Unmarshal(next.Data, &s)
	if s.Due[1] != original || s.Due[0] != original+5000 || next.Deadline != original {
		t.Fatal("reclaim changed other clocks")
	}
	if len(next.TimeoutActors) != 3 {
		t.Fatal("wrong timeout actors", next.TimeoutActors)
	}
	a, _ := json.Marshal(Action{Type: "ready", Window: s.Window})
	ctx.Actor = 1
	if !mod.AcceptStale(ctx, next, a) {
		t.Fatal("simultaneous ready rejected")
	}
	next, e = mod.Apply(ctx, next, a)
	if e != nil {
		t.Fatal(e)
	}
	if mod.AcceptStale(ctx, next, a) {
		t.Fatal("repeated ready accepted")
	}
}

func TestBotIgnoresOtherHands(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 4, Now: time.Unix(1000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = m.Actors[0]
	a, e := mod.BotAction(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	var s State
	_ = json.Unmarshal(m.Data, &s)
	x, y := (ctx.Actor+1)%4, (ctx.Actor+2)%4
	s.Hands[x], s.Hands[y] = s.Hands[y], s.Hands[x]
	m.Data, _ = json.Marshal(s)
	b, e := mod.BotAction(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("hidden cards changed bot decision: %s / %s", a, b)
	}
}

// Manual selections are not restricted to physical-card IDs chosen by the
// suggestion generator. Preview must expose every legal wildcard declaration
// privately, without playing cards or refreshing a decision deadline.
func TestManualWildPreviewIsPrivateAndAuthoritative(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 4, Actor: 0, Now: time.Unix(1000, 0)}
	m, e := mod.Start(ctx, json.RawMessage(`{"startLevel":4}`))
	if e != nil {
		t.Fatal(e)
	}
	var s State
	_ = json.Unmarshal(m.Data, &s)
	chosen := cards("H4 H4 C8 D8 C9 D9")
	used := map[int]bool{}
	for _, id := range chosen {
		used[id] = true
	}
	rest := []int{}
	for id := 0; id < 108; id++ {
		if !used[id] {
			rest = append(rest, id)
		}
	}
	s.Hands[0] = append(append([]int{}, chosen...), rest[:21]...)
	rest = rest[21:]
	for p := 1; p < 4; p++ {
		s.Hands[p] = append([]int{}, rest[:27]...)
		rest = rest[27:]
	}
	s.Turn = 0
	s.OpeningSeat = 0
	s.Revealed = []Reveal{}
	s.nextWindow(ctx.Now)
	m, e = s.envelope()
	if e != nil {
		t.Fatal(e)
	}
	before := m
	outsider, _ := mod.View(m, 1)
	spectator, _ := mod.View(m, -1)
	raw := json.RawMessage(`{"type":"preview","window":` + strconv.Itoa(s.Window) + `,"cards":` + string(mustCardsJSON(chosen)) + `}`)
	next, e := mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	if next.Deadline != before.Deadline {
		t.Fatal("preview extended deadline")
	}
	other, _ := mod.View(next, 1)
	spec, _ := mod.View(next, -1)
	if !reflect.DeepEqual(outsider, other) || !reflect.DeepEqual(spectator, spec) {
		t.Fatal("private selection leaked")
	}
	own, _ := mod.View(next, 0)
	b, _ := json.Marshal(own)
	var v map[string]any
	_ = json.Unmarshal(b, &v)
	ctrl := v["controls"].(map[string]any)
	interpretations := ctrl["interpretations"].([]any)
	if len(interpretations) < 3 {
		t.Fatal("manual interpretations incomplete", ctrl)
	}
	if len(v["hand"].([]any)) != 27 {
		t.Fatal("preview played cards")
	}
	key := interpretations[0].(map[string]any)["key"].(string)
	a, _ := json.Marshal(Action{Type: "play", Window: s.Window, Cards: chosen, Interpretation: key})
	next, e = mod.Apply(ctx, next, a)
	if e != nil {
		t.Fatal(e)
	}
	var played State
	_ = json.Unmarshal(next.Data, &played)
	announced := false
	for _, event := range played.Events {
		if event.Type == "play" {
			want := played.LastPlay.High
			if want == 15 {
				want = played.Level
			}
			if event.Value == want {
				announced = true
			}
		}
	}
	if !announced {
		t.Fatal("declared rank missing from voice event")
	}
	if bytes.Equal(next.Data, before.Data) {
		t.Fatal("confirmed preview did not play")
	}
}
func mustCardsJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
