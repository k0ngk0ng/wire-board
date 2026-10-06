package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestSplendorExtraNoblesPublisherFieldsAndSetup(t *testing.T) {
	// The Silk Road publisher 11-1 photograph and Sun Never Sets Asmodee
	// product image3. Read independently, in green/white/blue/black/red order.
	want := []Noble{{101, []int{3, 0, 3, 3, 0}}, {102, []int{0, 4, 0, 0, 4}}}
	if !reflect.DeepEqual(SplendorExtraNobles(), want) {
		t.Fatal("publisher noble requirements changed")
	}
	all := append(Nobles(), want...)
	seen := map[int]bool{}
	for n := 2; n <= 4; n++ {
		for i := 0; i < 80; i++ {
			s, err := NewSplendor(n, SplendorOptions{ExtraNobles: true})
			if err != nil {
				t.Fatal(err)
			}
			g := s.Splendor
			if len(g.Nobles) != n+1 || !g.Options.ExtraNobles || g.Options.Rules != SplendorExpansionRules {
				t.Fatal("wrong expanded setup")
			}
			ids := map[int]bool{}
			for _, noble := range g.Nobles {
				index := slices.IndexFunc(all, func(x Noble) bool { return x.ID == noble.ID })
				if index < 0 || !reflect.DeepEqual(noble, all[index]) || ids[noble.ID] {
					t.Fatal("invalid or duplicated noble", noble)
				}
				ids[noble.ID], seen[noble.ID] = true, true
			}
			if len(g.Players[0].Nobles) != 0 || g.Players[0].Score != 0 {
				t.Fatal("nobles granted rather than shuffled")
			}
			restored := clone(*s)
			if !reflect.DeepEqual(restored.Splendor.Options, g.Options) || !reflect.DeepEqual(restored.Splendor.Nobles, g.Nobles) {
				t.Fatal("save changed noble selection")
			}
		}
		base, _ := NewSplendor(n, SplendorOptions{})
		for _, noble := range base.Splendor.Nobles {
			if noble.ID > 10 {
				t.Fatal("base game gained a bonus noble")
			}
		}
		cities, err := newSplendorModernCatalogState(n, SplendorOptions{Cities: true, ExtraNobles: true})
		if err != nil || len(cities.Splendor.Nobles) != 0 || cities.Splendor.Options.ExtraNobles {
			t.Fatal("cities did not remove optional nobles", err)
		}
	}
	if !seen[101] || !seen[102] {
		t.Fatal("bonus nobles absent from random pool")
	}
}

func TestSplendorExtraNoblesSelectionScoreAndPhysicalCards(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{ExtraNobles: true})
	s.Splendor.Nobles = SplendorExtraNobles()
	p := &s.Splendor.Players[0]
	p.Bonus = []int{3, 4, 3, 3, 4}
	p.Score = 12
	s.gemAfter()
	if s.Phase != "noble" || p.Score != 12 {
		t.Fatal("must choose only one qualified noble")
	}
	gemExpansionReject(t, s, Action{Type: "noble", Noble: 1})
	gemExpansionApply(t, s, Action{Type: "noble", Noble: 102})
	p = &s.Splendor.Players[0]
	if p.Score != 15 || len(p.Nobles) != 1 || p.Nobles[0].ID != 102 || len(s.Splendor.Nobles) != 1 || !s.Splendor.LastRound {
		t.Fatal("noble award or last-round trigger")
	}
	e := s.Splendor.CardEvents[len(s.Splendor.CardEvents)-1]
	if e.Action != "noble" || e.Noble == nil || e.Noble.ID != 102 || e.Player != 0 {
		t.Fatal("missing public noble animation")
	}
	q := GemPlayer{Bonus: []int{0, 4, 0, 0, 4}, Cards: []Card{{Color: 1, Orient: GemOrientDouble, BonusCount: 2}, {Color: 1}, {Color: 1}, {Color: 4}, {Color: 4}, {Color: 4}, {Color: 4}}}
	if eligible(&q, SplendorExtraNobles()[1]) {
		t.Fatal("double discount substituted for four physical white cards")
	}
	q.Cards = append(q.Cards, Card{Color: 1, Orient: GemOrientCopy, BonusCount: 1})
	if !eligible(&q, SplendorExtraNobles()[1]) {
		t.Fatal("paired copy did not count as a white card")
	}
}

func TestSplendorExtraNoblesCombinationMatches(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		for n := 2; n <= 4; n++ {
			t.Run(fmt.Sprintf("%d/%d", mask, n), func(t *testing.T) {
				s, err := newSplendorModernCatalogState(n, SplendorOptions{ExtraNobles: true, TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0, Cities: mask&4 != 0, Orient: mask&8 != 0})
				if err != nil {
					t.Fatal(err)
				}
				initial := slices.Clone(s.Splendor.Bank)
				nobles := map[int]Noble{}
				for _, x := range s.Splendor.Nobles {
					nobles[x.ID] = x
				}
				want := 90
				if s.Splendor.Options.Orient {
					want = 120
				}
				for step := 0; !s.Finished && step < 2500; step++ {
					a, err := s.BotAction(s.Turn)
					if err == nil {
						err = s.Apply(s.Turn, a)
					}
					if err != nil {
						t.Fatal(step, err)
					}
					assertSplendorCatalogInventory(t, s, initial, want)
					ids := map[int]bool{}
					check := func(list []Noble) {
						for _, x := range list {
							if ids[x.ID] || !reflect.DeepEqual(x, nobles[x.ID]) {
								t.Fatal("noble changed or duplicated", x)
							}
							ids[x.ID] = true
						}
					}
					check(s.Splendor.Nobles)
					for _, p := range s.Splendor.Players {
						check(p.Nobles)
					}
					if len(ids) != len(nobles) {
						t.Fatal("noble disappeared")
					}
					if step%29 == 0 {
						restored := clone(*s)
						s = &restored
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatal("match did not finish")
				}
			})
		}
	}
}
