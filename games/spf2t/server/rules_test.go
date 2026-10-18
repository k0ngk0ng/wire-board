package spf2t

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func gem(id int, kind, color string, x, y int) Gem {
	return Gem{ID: id, Kind: kind, Color: color, X: x, Y: y, W: 1, H: 1}
}

func powerGem(id int, color string, x, y, w, h int) Gem {
	return Gem{ID: id, Kind: GemPower, Color: color, X: x, Y: y, W: w, H: h}
}

func playerWith(gems ...Gem) Player {
	return Player{Seat: 0, Selected: true, Character: "ryu", Rand: 1, NextID: len(gems) + 1, Gems: gems}
}

func TestPowerGemMergeAndRigidFall(t *testing.T) {
	p := playerWith(
		gem(1, GemNormal, ColorRed, 0, 10),
		gem(2, GemNormal, ColorRed, 1, 10),
		gem(3, GemNormal, ColorRed, 0, 9),
		gem(4, GemNormal, ColorRed, 1, 9),
	)
	created := mergePowerGems(&p)
	if len(created) != 1 || created[0].W != 2 || created[0].H != 2 {
		t.Fatalf("expected one 2x2 power gem, got %+v", created)
	}
	for _, g := range p.Gems {
		if g.Kind == GemNormal {
			t.Fatalf("normal cells survived merge: %+v", p.Gems)
		}
	}
	// With nothing underneath, the rigid 2x2 block falls as one entity.
	settleGems(&p)
	var pg *Gem
	for i := range p.Gems {
		if p.Gems[i].Kind == GemPower {
			pg = &p.Gems[i]
		}
	}
	if pg == nil || pg.Y != 10 {
		t.Fatalf("power gem did not settle rigidly: %+v", p.Gems)
	}
}

func TestCrashGroupClearsConnectedColorAndAdjacentCounter(t *testing.T) {
	p := playerWith(
		gem(1, GemCrash, ColorRed, 2, 5),
		gem(2, GemNormal, ColorRed, 2, 4),
		gem(3, GemNormal, ColorRed, 2, 6),
		gem(4, GemNormal, ColorBlue, 1, 6),
		gem(5, GemCounter, ColorBlue, 3, 5),
	)
	p.Gems[4].Timer = 4
	groups := p.detectCrashGroups()
	if len(groups) != 1 {
		t.Fatalf("expected one crash group, got %+v", groups)
	}
	if groups[0].flatCells != 3 || groups[0].crashCells != 1 {
		t.Fatalf("unexpected group counts: %+v", groups[0])
	}
	cells, flat, crash, counter, powers := p.clearGroups(groups)
	if flat != 3 || crash != 1 || counter != 1 || len(powers) != 0 {
		t.Fatalf("unexpected clear counts flat=%d crash=%d counter=%d powers=%d", flat, crash, counter, len(powers))
	}
	if len(cells) != 4 {
		t.Fatalf("expected 4 cleared cells, got %d: %+v", len(cells), cells)
	}
	if len(p.Gems) != 1 || p.Gems[0].Color != ColorBlue || p.Gems[0].Kind != GemNormal {
		t.Fatalf("unrelated blue gem should survive: %+v", p.Gems)
	}
}

func TestDiamondClearsAllOfTargetColorOnly(t *testing.T) {
	p := playerWith(
		gem(1, GemDiamond, "", 2, 5),
		gem(2, GemNormal, ColorRed, 2, 6),
		gem(3, GemNormal, ColorRed, 0, 1),
		gem(4, GemNormal, ColorBlue, 5, 1),
	)
	if target := diamondTarget(&p, 2, 5); target != ColorRed {
		t.Fatalf("diamond target = %q, want red", target)
	}
	cells, flat, counter, powers := p.diamondClear(ColorRed, 2, 5)
	if flat != 2 || counter != 0 || len(powers) != 0 || len(cells) != 2 {
		t.Fatalf("unexpected diamond clear: cells=%d flat=%d counter=%d powers=%d", len(cells), flat, counter, len(powers))
	}
	if len(p.Gems) != 1 || p.Gems[0].Color != ColorBlue {
		t.Fatalf("non-target color was removed: %+v", p.Gems)
	}
}

func TestCounterTicksAndConverts(t *testing.T) {
	p := playerWith(gem(1, GemCounter, ColorGreen, 0, 0), gem(2, GemCounter, ColorGreen, 1, 0))
	p.Gems[0].Timer = 1
	p.Gems[1].Timer = 3
	decrementCounters(&p)
	if p.Gems[0].Kind != GemNormal || p.Gems[0].Timer != 0 {
		t.Fatalf("timer 1 did not convert: %+v", p.Gems[0])
	}
	if p.Gems[1].Kind != GemCounter || p.Gems[1].Timer != 2 {
		t.Fatalf("timer 3 did not decrement: %+v", p.Gems[1])
	}
}

func TestAttackBenchmarksAndChain(t *testing.T) {
	cases := []struct {
		base, groups int
		diamond      bool
		chain        int
		want         int
	}{
		{9, 1, false, 1, 9},
		{17, 1, false, 1, 17},
		{25, 1, false, 1, 25},
		{39, 1, false, 1, 39},
		{50, 1, false, 1, 50},
		{9, 1, false, 2, 11},
	}
	for _, c := range cases {
		if got := stepAttack(c.chain, c.base, c.groups, c.diamond); got != c.want {
			t.Errorf("stepAttack(%d,%d,%d,%v) = %d, want %d", c.chain, c.base, c.groups, c.diamond, got, c.want)
		}
	}
	if got := powerBase(4, 2, 2); got != 9 {
		t.Errorf("powerBase(4) = %d", got)
	}
	if got := powerBase(16, 4, 4); got != 50 {
		t.Errorf("powerBase(16) = %d", got)
	}
}

func TestPairGeneratorDiamondEvery25(t *testing.T) {
	p := newPlayer(0, 42)
	var sawDiamond bool
	for i := 1; i <= 25; i++ {
		pair := generatePair(&p)
		if i == 25 {
			sawDiamond = pair.B.Kind == GemDiamond || pair.A.Kind == GemDiamond
		}
		if pair.A.Kind == GemCrash && pair.B.Kind == GemCrash {
			t.Fatalf("pair %d has two crash gems", i)
		}
	}
	if !sawDiamond {
		t.Fatal("25th pair did not contain a diamond")
	}
}

func TestStartSelectCountdownPlaying(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(1000, 0)}
	m, err := mod.Start(ctx, raw(Options{Format: FormatBestOf3}))
	if err != nil {
		t.Fatal(err)
	}
	if err = mod.ValidateMatch(m, 2); err != nil {
		t.Fatal(err)
	}
	v, err := mod.View(m, -1)
	if err != nil {
		t.Fatal(err)
	}
	view := v.(View)
	if view.Phase != PhaseSelect || view.SelectUntil == 0 {
		t.Fatalf("unexpected select view: %+v", view)
	}
	for seat := 0; seat < 2; seat++ {
		ctx.Actor = seat
		m, err = mod.Apply(ctx, m, raw(Action{Type: "select", Epoch: view.Epoch, Character: characterDefs[seat].ID}))
		if err != nil {
			t.Fatal(err)
		}
	}
	v, _ = mod.View(m, -1)
	view = v.(View)
	if view.Phase != PhaseCountdown {
		t.Fatalf("phase after both selections = %s", view.Phase)
	}
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(3 * time.Second)
	m, err = mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if err = mod.ValidateMatch(m, 2); err != nil {
		t.Fatal(err)
	}
	v, _ = mod.View(m, 0)
	view = v.(View)
	if view.Phase != PhasePlaying {
		t.Fatalf("phase after countdown = %s", view.Phase)
	}
	if view.Players[0].Active == nil || view.Players[1].Active == nil {
		t.Fatalf("players missing active pairs: %+v", view.Players)
	}
	if view.Controls.CanPlay != true {
		t.Fatalf("own controls should be playable")
	}
	other, _ := mod.View(m, 1)
	if other.(View).Players[0].Seq != 0 {
		t.Fatalf("seat 1 saw seat 0 seq")
	}
}

func TestHardDropLocksAndSpawnsNext(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(2000, 0)}
	m, err := mod.Start(ctx, raw(Options{Format: FormatBestOf3}))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := mod.View(m, -1)
	view := v.(View)
	for seat := 0; seat < 2; seat++ {
		ctx.Actor = seat
		m, err = mod.Apply(ctx, m, raw(Action{Type: "select", Epoch: view.Epoch, Character: characterDefs[seat].ID}))
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx.Actor = -1
	v, _ = mod.View(m, -1)
	view = v.(View)
	ctx.Now = time.UnixMilli(view.StartAt)
	m, err = mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	v, _ = mod.View(m, 0)
	view = v.(View)
	ctx.Actor = 0
	m, err = mod.Apply(ctx, m, raw(Action{Type: "inputs", Epoch: view.Epoch, Seq: 1, Events: []Input{{Kind: "hardDrop"}}}))
	if err != nil {
		t.Fatal(err)
	}
	v, _ = mod.View(m, 0)
	view = v.(View)
	if view.Players[0].Active == nil {
		t.Fatalf("next pair did not spawn: %+v", view.Players[0])
	}
	if len(view.Players[0].Gems) == 0 {
		t.Fatalf("hard drop did not stamp gems: %+v", view.Players[0])
	}
	if view.Players[0].Seq != 1 {
		t.Fatalf("seq = %d", view.Players[0].Seq)
	}
}

func TestDiamondPairClearsTargetColorAndUsesTimer3(t *testing.T) {
	s := &State{Options: Options{Format: FormatSingle}, Phase: PhasePlaying, Round: 1}
	p := &Player{Seat: 0, Selected: true, Character: "ryu", Rand: 1, IdleAt: 1000, NextID: 2}
	p.Gems = []Gem{
		gem(1, GemNormal, ColorRed, 4, 5),
		gem(2, GemNormal, ColorBlue, 5, 5),
	}
	p.Active = &Pair{ID: 1, X: 2, Y: 1, Rot: 0, A: GemSpec{Kind: GemNormal, Color: ColorRed}, B: GemSpec{Kind: GemDiamond}}
	p.Next = Pair{ID: 2, A: GemSpec{Kind: GemNormal, Color: ColorGreen}, B: GemSpec{Kind: GemNormal, Color: ColorGreen}}
	opp := Player{Seat: 1, Selected: true, Character: "ken", Rand: 2}
	s.Players = []Player{*p, opp}
	s.lockPlayer(&s.Players[0], 1000)
	if s.Players[0].Resolve == nil || !s.Players[0].Resolve.Diamond || s.Players[0].Resolve.Target != ColorRed {
		t.Fatalf("diamond resolve not armed: %+v", s.Players[0].Resolve)
	}
	s.resolveStep(&s.Players[0], 1240)
	if s.Players[0].Lost {
		t.Fatal("unexpected top out")
	}
	for _, g := range s.Players[0].Gems {
		if g.Color == ColorRed {
			t.Fatalf("red gem survived diamond: %+v", s.Players[0].Gems)
		}
	}
	if len(s.Players[1].Pending) != 1 || s.Players[1].Pending[0].Timer != 3 {
		t.Fatalf("diamond attack timer = %+v", s.Players[1].Pending)
	}
	if s.Players[0].GemsCleared != 2 || s.Players[0].MaxChain != 1 {
		t.Fatalf("diamond stats = cleared %d chain %d", s.Players[0].GemsCleared, s.Players[0].MaxChain)
	}
}

func TestPendingCancelsBeforeDelivery(t *testing.T) {
	s := &State{Options: Options{Format: FormatSingle}, Phase: PhasePlaying, Round: 1}
	p := Player{Seat: 0, Selected: true, Character: "ryu", Rand: 1, IdleAt: 1000, NextID: 1}
	p.Gems = []Gem{gem(1, GemNormal, ColorBlue, 5, 5)}
	p.Pending = []CounterSpec{
		{Color: ColorGreen, Column: 0, Timer: 5},
		{Color: ColorGreen, Column: 1, Timer: 5},
	}
	p.Next = Pair{ID: 2, A: GemSpec{Kind: GemNormal, Color: ColorYellow}, B: GemSpec{Kind: GemNormal, Color: ColorYellow}}
	opp := Player{Seat: 1, Selected: true, Character: "ken", Rand: 2}
	s.Players = []Player{p, opp}
	s.Players[0].Resolve = &Resolve{Chain: 1, Accum: 1, Cleared: 1}
	s.finishResolution(&s.Players[0], s.Players[0].Resolve, 1000)
	if len(s.Players[0].Pending) != 0 {
		t.Fatalf("pending not consumed: %+v", s.Players[0].Pending)
	}
	if len(s.Players[1].Pending) != 0 {
		t.Fatalf("unexpected attack sent: %+v", s.Players[1].Pending)
	}
	found := false
	for _, g := range s.Players[0].Gems {
		if g.Kind == GemCounter && g.X == 1 && g.Y == 11 && g.Timer == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("remaining pending counter not delivered: %+v", s.Players[0].Gems)
	}
}

func TestFinishDrawAndWinner(t *testing.T) {
	draw := &State{Options: Options{Format: FormatBestOf3}, Phase: PhasePlaying, Round: 3, Players: []Player{{RoundWins: 1}, {RoundWins: 1}}}
	if err := draw.finish(5000, "time_limit"); err != nil {
		t.Fatal(err)
	}
	if !draw.Result.Draw || len(draw.Result.Winners) != 0 || draw.Result.Ranks[0] != 1 || draw.Result.Ranks[1] != 1 {
		t.Fatalf("draw result: %+v", draw.Result)
	}
	win := &State{Options: Options{Format: FormatBestOf3}, Phase: PhasePlaying, Round: 3, Players: []Player{{RoundWins: 2}, {RoundWins: 1}}}
	if err := win.finish(5000, "match_won"); err != nil {
		t.Fatal(err)
	}
	if win.Result.Draw || len(win.Result.Winners) != 1 || win.Result.Winners[0] != 0 || win.Result.Ranks[0] != 1 {
		t.Fatalf("winner result: %+v", win.Result)
	}
}
