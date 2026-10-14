package shengji

// Failure cases for phases: duplicate/tied/self counter-declaration; no caller
// fallback leaks kitty; reclaim affects other actors; invalid bury ownership;
// failed throw can be cancelled; no-score redeal bypasses proof; wrong dealer
// rotates after scoring; A can be skipped without defending a hand at A.
import (
	"bytes"
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func fixture(t *testing.T, decks int, handSpecs []string) (*State, gameapi.Context) {
	t.Helper()
	ctx := gameapi.Context{Players: 4, Now: time.Unix(1700000000, 0)}
	m, e := Module().Start(ctx, json.RawMessage(`{"decks":`+string(rune('0'+decks))+`,"rounds":3}`))
	if e != nil {
		t.Fatal(e)
	}
	var s State
	_ = json.Unmarshal(m.Data, &s)
	s.Hands = make([][]int, 4)
	used := map[int]bool{}
	for p, spec := range handSpecs {
		s.Hands[p] = cards(spec)
		for _, id := range s.Hands[p] {
			if used[id] {
				t.Fatal("fixture duplicated physical card")
			}
			used[id] = true
		}
	}
	count := 25
	if decks == 1 {
		count = 12
	}
	deck := []int{}
	for id := 0; id < decks*54; id++ {
		if !used[id] {
			deck = append(deck, id)
		}
	}
	for p := 0; p < 4; p++ {
		n := count - len(s.Hands[p])
		s.Hands[p] = append(s.Hands[p], deck[:n]...)
		deck = deck[n:]
	}
	s.Bottom = deck
	s.Dealer = 0
	s.Level = 2
	s.Trump = -1
	s.Declaration = nil
	s.Revealed = []Reveal{}
	s.Passed = make([]bool, 4)
	s.nextWindow(ctx.Now)
	return &s, ctx
}
func act(t *testing.T, s *State, ctx gameapi.Context, p int, a Action) *State {
	t.Helper()
	m, e := s.envelope()
	if e != nil {
		t.Fatal(e)
	}
	ctx.Actor = p
	a.Window = s.Window
	raw, _ := json.Marshal(a)
	next, e := Module().Apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	var out State
	_ = json.Unmarshal(next.Data, &out)
	return &out
}
func TestDeclarationCounterAndOwnDeadlines(t *testing.T) {
	s, ctx := fixture(t, 2, []string{"C2", "D2 D2", "BJ BJ", "RJ RJ"})
	s = act(t, s, ctx, 0, Action{Type: "declare", Cards: cards("C2")})
	if s.Declaration.Strength != 1 || s.Trump != 0 {
		t.Fatal("single declaration")
	}
	s = act(t, s, ctx, 1, Action{Type: "declare", Cards: cards("D2 D2")})
	if s.Declaration.Strength != 2 || s.Trump != 1 {
		t.Fatal("pair counter")
	}
	s = act(t, s, ctx, 2, Action{Type: "declare", Cards: cards("BJ BJ")})
	if s.Declaration.Strength != 3 || s.Trump != -1 {
		t.Fatal("joker no trump")
	}
	m, _ := s.envelope()
	a, _ := json.Marshal(Action{Type: "declare", Window: s.Window, Cards: cards("RJ RJ")})
	ctx.Actor = 3
	if _, e := Module().Apply(ctx, m, a); e == nil {
		t.Fatal("equal strength overcalled")
	}
	old := s.Due[1]
	ctx.Actor = 0
	ctx.Now = ctx.Now.Add(5 * time.Second)
	next, e := Module().Reclaim(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	var after State
	_ = json.Unmarshal(next.Data, &after)
	if after.Due[0] != old+5000 || after.Due[1] != old || next.Deadline != old {
		t.Fatal("unrelated deadline moved")
	}
	for _, p := range []int{0, 1, 3} {
		s = act(t, s, ctx, p, Action{Type: "pass"})
	}
	if s.Phase != "bury" || len(s.Hands[0]) != 33 || len(s.Bottom) != 0 {
		t.Fatal("taking bottom")
	}
	m, _ = s.envelope()
	a, _ = json.Marshal(Action{Type: "bury", Window: s.Window, Cards: s.Hands[0][:7]})
	ctx.Actor = 0
	if _, e = Module().Apply(ctx, m, a); e == nil {
		t.Fatal("buried seven instead of eight")
	}
	s = act(t, s, ctx, 0, Action{Type: "bury", Cards: s.Hands[0][:8]})
	if s.Phase != "play" || len(s.Hands[0]) != 25 || s.Turn != 0 {
		t.Fatal("bury phase")
	}
	for _, p := range []int{1, 2, 3, -1} {
		m, _ = s.envelope()
		v, e := Module().View(m, p)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(v)
		var d map[string]any
		_ = json.Unmarshal(b, &d)
		if len(d["bottom"].([]any)) != 0 {
			t.Fatal("kitty leak")
		}
	}
}

// Invalid throws are refused atomically, including in restored legacy rooms.
func TestInvalidThrowIsRejectedWithoutSideEffects(t *testing.T) {
	s, ctx := fixture(t, 2, []string{"CA CQ CQ CJ", "CK", "D3", "H3"})
	s.Phase = "play"
	s.Trump = 3
	s.Turn = 0
	s.nextWindow(ctx.Now)
	m, e := s.envelope()
	if e != nil {
		t.Fatal(e)
	}
	before := append([]byte{}, m.Data...)
	ctx.Actor = 0
	raw, _ := json.Marshal(Action{Type: "play", Window: s.Window, Cards: cards("CA CQ CQ CJ")})
	next, e := Module().Apply(ctx, m, raw)
	if e == nil {
		t.Fatal("invalid throw was accepted and penalized")
	}
	if !bytes.Equal(next.Data, before) || !bytes.Equal(m.Data, before) || next.Deadline != m.Deadline {
		t.Fatal("rejection mutated cards, scores, clock or turn")
	}
}

func TestRebelRequiresNoScoreCards(t *testing.T) {
	s, ctx := fixture(t, 1, []string{"C5", "D3", "H3", "S3"})
	m, _ := s.envelope()
	ctx.Actor = 0
	a, _ := json.Marshal(Action{Type: "rebel", Window: s.Window})
	if _, e := Module().Apply(ctx, m, a); e == nil {
		t.Fatal("scoring hand rebelled")
	}
}
func TestDealerRotationAndMustPlayAce(t *testing.T) {
	for _, c := range []struct {
		points, level int
		next, team    int
		finish        bool
	}{{0, 2, 2, 0, false}, {80, 2, 1, 1, false}, {0, 13, 2, 0, false}, {0, 14, 2, 0, true}} {
		s := State{Options: Options{Decks: 2, Rounds: 0, TurnSeconds: 30}, Round: 1, Dealer: 0, Level: c.level, Levels: []int{c.level, 2}, Captured: []int{0, c.points}, Due: make([]int64, 4), WinnerTeam: -1}
		s.finishRound(0, Shape{Count: 1, Parts: []Part{{Kind: "single", Cards: []int{0}}}}, time.Unix(1000, 0))
		if s.NextDealer != c.next || s.Results[0].Team != c.team || (s.Phase == "finished") != c.finish {
			t.Fatal(c, s.NextDealer, s.Results, s.Phase)
		}
	}
}
