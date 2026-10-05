package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func fishingTokens(t *testing.T, n int) *catanFishingTokens {
	t.Helper()
	f, err := newCatanFishingTokens(n)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func fishTop(f *catanFishingTokens, ids ...int) {
	for i := len(ids) - 1; i >= 0; i-- {
		at := slices.Index(f.DrawPile, ids[i])
		f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
		f.DrawPile = append(f.DrawPile, ids[i])
	}
}
func fishOwn(f *catanFishingTokens, p int, ids ...int) {
	for _, id := range ids {
		at := slices.Index(f.DrawPile, id)
		f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
		f.Hands[p] = append(f.Hands[p], id)
	}
}
func fishReject(t *testing.T, f *catanFishingTokens, action func() error) {
	t.Helper()
	before, _ := json.Marshal(f)
	if action() == nil {
		t.Fatal("accepted invalid fishing action")
	}
	after, _ := json.Marshal(f)
	if string(before) != string(after) {
		t.Fatal("invalid fishing action partially mutated state")
	}
}

func TestCatanFishingTokensOfficialInventoryAndRestore(t *testing.T) {
	for n := 3; n <= 6; n++ {
		f := fishingTokens(t, n)
		counts := make([]int, 4)
		for _, id := range f.DrawPile {
			counts[catanFishValue(id)]++
		}
		want := []int{1, 11, 10, 8}
		if n > 4 {
			want = []int{1, 15, 15, 13}
		}
		if !slices.Equal(counts, want) {
			t.Fatal("wrong printed token inventory", n, counts)
		}
		for step := 0; step < 18; step++ {
			claims := make([]int, n)
			for p := range claims {
				claims[p] = 2
			}
			if err := f.beginDraw(step%n, claims); err != nil {
				t.Fatal(err)
			}
			for len(f.Pending) > 0 {
				p := f.Pending[0].Player
				token := f.Hands[p][0]
				if err := f.replace(p, &token); err != nil {
					t.Fatal(err)
				}
			}
			for p, hand := range f.Hands {
				if len(hand) > 0 {
					if err := f.spend(p, []int{hand[0]}, 1); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := f.validate(); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(f)
			var restored catanFishingTokens
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(*f, restored) {
				t.Fatal("round-trip changed fishing inventory")
			}
			f = &restored
		}
	}
	for _, n := range []int{0, 2, 7} {
		if _, err := newCatanFishingTokens(n); err == nil {
			t.Fatal("unsupported player count")
		}
	}
}

func TestCatanFishingTokensClockwiseBootAndSevenTokenCap(t *testing.T) {
	f := fishingTokens(t, 4)
	fishOwn(f, 2, 0, 1, 2, 3, 4, 5)
	fishOwn(f, 3, 6, 7, 8, 9, 10, 11, 12)
	fishTop(f, catanFishBoot, 21, 22, 23, 24)
	if err := f.beginDraw(2, []int{1, 1, 4, 2}); err != nil {
		t.Fatal(err)
	}
	// Boot consumes the first of four draws; 21 is the second. At seven fish
	// tokens, two remaining draws become one optional replacement, not two.
	if f.BootOwner != 2 || len(f.Hands[2]) != 7 || len(f.Pending) != 4 || f.Pending[0] != (catanFishClaim{2, 2}) || f.Pending[1].Player != 3 {
		t.Fatal("wrong clockwise cap/boot resolution", f)
	}
	before, _ := json.Marshal(f)
	var saved catanFishingTokens
	if err := json.Unmarshal(before, &saved); err != nil {
		t.Fatal(err)
	}
	f = &saved
	fishReject(t, f, func() error { return f.replace(3, nil) })
	bad := 29
	fishReject(t, f, func() error { return f.replace(2, &bad) })
	token := 0
	if err := f.replace(2, &token); err != nil {
		t.Fatal(err)
	}
	if len(f.Hands[2]) != 7 || !slices.Contains(f.Hands[2], 22) || slices.Contains(f.Hands[2], 23) || f.Pending[0].Player != 3 {
		t.Fatal("replacement drew too many tokens")
	}
	if err := f.replace(3, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.Pending) != 0 || !slices.Equal(f.Hands[0], []int{23}) || !slices.Equal(f.Hands[1], []int{24}) || len(f.Hands[3]) != 7 {
		t.Fatal("declining did not continue clockwise", f)
	}
}

func TestCatanFishingTokensReplacementBootStopsAndDiscardReshuffles(t *testing.T) {
	f := fishingTokens(t, 3)
	fishOwn(f, 0, 0, 1, 2, 3, 4, 5, 6)
	fishTop(f, catanFishBoot, 21)
	if err := f.beginDraw(0, []int{3, 1, 0}); err != nil {
		t.Fatal(err)
	}
	token := 0
	if err := f.replace(0, &token); err != nil {
		t.Fatal(err)
	}
	if f.BootOwner != 0 || len(f.Hands[0]) != 6 || !slices.Equal(f.Hands[1], []int{21}) || len(f.Pending) != 0 {
		t.Fatal("replacement boot illegally triggered extra draws")
	}
	// Put all remaining fish into the faceup discard; only the boot remains public.
	f.Discard = append(f.Discard, f.DrawPile...)
	f.DrawPile = []int{}
	oldDiscard := slices.Clone(f.Discard)
	if err := f.beginDraw(2, []int{0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if len(f.Discard) != 0 || len(f.DrawPile) != len(oldDiscard)-1 || !slices.Contains(oldDiscard, f.Hands[2][0]) {
		t.Fatal("empty bag did not recycle the discard")
	}
}

func TestCatanFishingTokensPaymentsAndBootPublicRanking(t *testing.T) {
	f := fishingTokens(t, 3)
	fishOwn(f, 0, 0, 11, 21, 22)
	fishTop(f, catanFishBoot)
	if err := f.beginDraw(0, []int{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]int{nil, {21, 21}, {29}, {1}, {0}} {
		fishReject(t, f, func() error { return f.spend(0, ids, 2) })
	}
	if err := f.spend(0, []int{21}, 2); err != nil {
		t.Fatal(err)
	} // Three fish pays two, with no change.
	if !slices.Equal(f.Hands[0], []int{0, 11, 22}) || !slices.Equal(f.Discard, []int{21}) {
		t.Fatal("overpayment made change")
	}
	fishReject(t, f, func() error { return f.spend(0, []int{22}, 4) }) // No excess credit from prior action.
	if err := f.spend(0, []int{11, 22}, 5); err != nil {
		t.Fatal(err)
	}
	fishReject(t, f, func() error { return f.passBoot(1, 2, []int{3, 3, 4}) })
	fishReject(t, f, func() error { return f.passBoot(0, 1, []int{3, 2, 4}) })
	fishReject(t, f, func() error { return f.passBoot(0, 0, []int{3, 3, 4}) })
	scores := []int{3, 3, 4}
	if err := f.passBoot(0, 1, scores); err != nil {
		t.Fatal(err)
	}
	if f.BootOwner != 1 || !slices.Equal(scores, []int{3, 3, 4}) {
		t.Fatal("boot changed scores rather than ownership")
	}
}

func TestCatanFishingTokensViewPrivacyAndAliasing(t *testing.T) {
	f := fishingTokens(t, 3)
	fishOwn(f, 0, 0, 11)
	fishOwn(f, 1, 21)
	fishTop(f, catanFishBoot)
	if err := f.beginDraw(2, []int{0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := f.spend(0, []int{11}, 2); err != nil {
		t.Fatal(err)
	}
	for viewer := -1; viewer < 3; viewer++ {
		v, err := f.view(viewer)
		if err != nil {
			t.Fatal(err)
		}
		if v.BootOwner != 2 || v.Discard[0].Fish != 2 {
			t.Fatal("public boot/discards missing")
		}
		for p, hand := range v.Players {
			if hand.Count != len(f.Hands[p]) || len(hand.Tokens) > 0 && p != viewer {
				t.Fatal("other player's secret tokens exposed")
			}

			if p == viewer {
				if len(hand.Tokens) != len(f.Hands[p]) {
					t.Fatal("own fish faces absent")
				}
				for i, token := range hand.Tokens {
					if token.ID != f.Hands[p][i] || token.Fish != catanFishValue(token.ID) {
						t.Fatal("wrong own token face")
					}
				}
			}
		}
		g := f.copy()
		slices.Reverse(g.DrawPile)
		other, _ := g.view(viewer)
		if !reflect.DeepEqual(v, other) {
			t.Fatal("view depends on hidden draw order")
		}
		v.Discard[0].Fish = 99
		if len(v.Players[0].Tokens) > 0 {
			v.Players[0].Tokens[0].Fish = 99
		}
		again, _ := f.view(viewer)
		if again.Discard[0].Fish != 2 {
			t.Fatal("view aliases state")
		}
	}
}

func TestCatanFishingTokensCorruptRestoreAndInvalidClaimsAreAtomic(t *testing.T) {
	f := fishingTokens(t, 3)
	for _, claims := range [][]int{nil, {1, 0}, {1, -1, 0}, {100, 0, 0}} {
		fishReject(t, f, func() error { return f.beginDraw(0, claims) })
	}
	fishReject(t, f, func() error { return f.beginDraw(-1, []int{1, 1, 1}) })
	for _, corrupt := range []func(*catanFishingTokens){
		func(g *catanFishingTokens) { g.DrawPile = g.DrawPile[1:] },
		func(g *catanFishingTokens) { g.DrawPile[0] = g.DrawPile[1] },
		func(g *catanFishingTokens) { g.BootOwner = 0 },
		func(g *catanFishingTokens) { fishOwn(g, 0, catanFishBoot) },
		func(g *catanFishingTokens) {
			at := slices.Index(g.DrawPile, catanFishBoot)
			g.DrawPile = slices.Delete(g.DrawPile, at, at+1)
			g.Discard = append(g.Discard, catanFishBoot)
		},
		func(g *catanFishingTokens) { fishOwn(g, 0, 0, 1, 2, 3, 4, 5, 6, 7) },
		func(g *catanFishingTokens) { g.Pending = []catanFishClaim{{0, 1}} },
		func(g *catanFishingTokens) { g.BootOwner = -2 },
	} {
		bad := f.copy()
		corrupt(&bad)
		data, _ := json.Marshal(bad)
		fishReject(t, f, func() error { return json.Unmarshal(data, f) })
	}
	fishOwn(f, 0, 0, 1, 2, 3, 4, 5, 6)
	if err := f.beginDraw(0, []int{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	fishReject(t, f, func() error { return f.beginDraw(0, []int{1, 0, 0}) })
	fishReject(t, f, func() error { return f.spend(0, []int{0}, 1) })
}
