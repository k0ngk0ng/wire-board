package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Synthetic cards exercise the published rules, not the unverified card catalog.
func gemOrientTest(t *testing.T) *State {
	t.Helper()
	s := gemExpandedTest(t, SplendorOptions{})
	s.Splendor.Options.Orient = true
	s.Splendor.Market = make([][]Card, 6)
	s.Splendor.Decks = make([][]Card, 6)
	s.Splendor.Nobles = nil
	return s
}
func orientFixture(id, tier, color int, power string) Card {
	return Card{ID: id, Tier: tier, Color: color, Orient: power, Cost: make([]int, 5)}
}
func gemOwnFixture(s *State, player int, cards ...Card) {
	p := &s.Splendor.Players[player]
	for _, c := range cards {
		p.Cards = append(p.Cards, c)
		p.Score += c.Points
		if c.gemBonus() > 0 {
			p.Bonus[c.Color] += c.gemBonus()
		}
	}
}

func TestSplendorOrientCascadeRestoreAndPurchaseTriggers(t *testing.T) {
	s := gemOrientTest(t)
	g := s.Splendor
	g.Options.TradingPosts, g.Options.Strongholds = true, true
	g.Strongholds = map[int]GemStronghold{}
	g.Players[0].TradingPosts = []int{GemPostPurchaseToken}
	gemOwnFixture(s, 0, orientFixture(101, 2, 0, GemOrientDouble))
	top := orientFixture(102, 3, 1, GemOrientCascade)
	middle := orientFixture(103, 2, -1, GemOrientCopyCascade)
	bottom := orientFixture(104, 1, -1, GemOrientCopy)
	blocked := orientFixture(105, 1, 2, "")
	g.Market[0] = []Card{blocked}
	g.Market[3] = []Card{bottom}
	g.Market[4] = []Card{middle}
	g.Market[5] = []Card{top}
	g.Strongholds[blocked.ID] = GemStronghold{Player: 1, Count: 1}
	for row := 3; row < 6; row++ {
		g.Decks[row] = []Card{orientFixture(110+row, row-2, -1, GemOrientGold)}
	}
	gemExpansionApply(t, s, Action{Type: "buy", Card: 102})
	if s.Phase != "gem_free_card" {
		t.Fatal(s.Phase)
	}
	gemExpansionReject(t, s, Action{Type: "gem_free_card", Card: 114})
	gemExpansionApply(t, s, Action{Type: "gem_free_card", Card: 103})
	if s.Phase != "gem_copy" {
		t.Fatal(s.Phase)
	}
	gemExpansionReject(t, s, Action{Type: "gem_copy", Card: 103})
	restored := clone(*s)
	s = &restored
	gemExpansionApply(t, s, Action{Type: "gem_copy", Card: 101})
	if s.Phase != "gem_free_card" {
		t.Fatal(s.Phase)
	}
	gemExpansionReject(t, s, Action{Type: "gem_free_card", Card: 105})
	gemExpansionApply(t, s, Action{Type: "gem_free_card", Card: 104})
	gemExpansionApply(t, s, Action{Type: "gem_copy", Card: 103})
	g = s.Splendor
	if s.Phase != "gem_token" || g.Players[0].Bonus[0] != 6 || g.Players[0].gemCardCounts()[0] != 3 {
		t.Fatal("copy double bonus or physical counts incorrect", s.Phase, g.Players[0])
	}
	if eligible(&g.Players[0], Noble{Cost: []int{4, 0, 0, 0, 0}}) {
		t.Fatal("double bonuses earned noble as physical cards")
	}
	for row := 3; row < 6; row++ {
		if g.Market[row][0].ID != 0 || len(g.Decks[row]) != 1 {
			t.Fatal("refilled before effects finished")
		}
	}
	gemExpansionApply(t, s, Action{Type: "gem_token", Color: 2})
	if s.Phase != "gem_stronghold" {
		t.Fatal(s.Phase)
	}
	gemExpansionApply(t, s, Action{Type: "gem_stronghold", Choice: "remove", Card: 105})
	g = s.Splendor
	if s.Turn != 1 || len(g.Effects) != 0 || g.Players[0].Tokens[2] != 1 {
		t.Fatal("free cards repeated purchase triggers", s.Phase)
	}
	for row := 3; row < 6; row++ {
		if g.Market[row][0].ID != 110+row || len(g.Decks[row]) != 0 {
			t.Fatal("wrong refill deck")
		}
	}
	if len(g.CardEvents) != 3 || g.CardEvents[0].Action != "buy" || g.CardEvents[1].Action != "free" || g.CardEvents[2].Action != "free" {
		t.Fatal("wrong cascade animations", g.CardEvents)
	}
}

func TestSplendorOrientVirtualGoldPayments(t *testing.T) {
	p := GemPlayer{Tokens: make([]int, 6), Bonus: make([]int, 5), Cards: []Card{orientFixture(101, 1, -1, GemOrientGold), orientFixture(102, 1, -1, GemOrientGold)}}
	c := orientFixture(103, 1, 2, "")
	c.Cost = []int{1, 1, 1, 0, 0}
	if _, _, err := gemOrientPayment(p, c, Action{Cards: []int{101}}); err == nil {
		t.Fatal("one gold card paid three colors")
	}
	if pay, ids, err := gemOrientPayment(p, c, Action{Cards: []int{101, 102}}); err != nil || sum(pay) != 0 || len(ids) != 2 {
		t.Fatal(pay, ids, err)
	}
	p.TradingPosts = []int{GemPostDoubleGold}
	if _, _, err := gemOrientPayment(p, c, Action{Cards: []int{101}}); err == nil {
		t.Fatal("doubled virtual gold split across colors")
	}
	c.Cost = []int{3, 0, 0, 0, 0}
	if _, _, err := gemOrientPayment(p, c, Action{Cards: []int{101}}); err != nil {
		t.Fatal(err)
	}
	c.Cost = []int{1, 0, 0, 0, 0}
	for _, a := range []Action{{Cards: []int{101, 102}}, {Cards: []int{101, 101}}, {Cards: []int{999}}, {Cards: []int{101}, Tokens: []int{-1, 0, 0, 0, 0, 0}}, {Cards: []int{101}, Tokens: []int{0, 0, 0, 0, 0, -1}}} {
		if _, _, err := gemOrientPayment(p, c, a); err == nil {
			t.Fatal("invalid virtual payment", a)
		}
	}
	s := gemOrientTest(t)
	s.Splendor.Players[0] = p
	s.Splendor.Market[0] = []Card{c}
	gemExpansionApply(t, s, Action{Type: "buy", Card: 103, Cards: []int{101}})
	g := s.Splendor
	if len(g.Exiled) != 1 || g.Exiled[0].ID != 101 || len(g.Players[0].Cards) != 2 || sum(g.Players[0].Tokens) != 0 || len(g.TokenEvents) != 0 {
		t.Fatal("virtual gold became physical or was not removed")
	}
}

func TestSplendorOrientSacrificePriorityAndPermanentAwards(t *testing.T) {
	s := gemOrientTest(t)
	copied := orientFixture(101, 1, 0, GemOrientCopy)
	copied.BonusCount = 2
	copied.Points = 1
	doubled := orientFixture(102, 2, 0, GemOrientDouble)
	doubled.Points = 2
	normal := orientFixture(103, 1, 0, "")
	gemOwnFixture(s, 0, copied, doubled, normal)
	p := &s.Splendor.Players[0]
	p.Nobles = []Noble{{ID: 1}}
	p.Score += 3
	p.TradingPosts = []int{GemPostDoubleGold}
	c := orientFixture(104, 3, 2, GemOrientSacrifice)
	c.SacrificeColor = 0
	c.Points = 3
	s.Splendor.Market[5] = []Card{c}
	gemExpansionReject(t, s, Action{Type: "buy", Card: 104, Cards: []int{102, 103}})
	gemExpansionReject(t, s, Action{Type: "buy", Card: 104, Cards: []int{101, 101}})
	gemExpansionReject(t, s, Action{Type: "buy", Card: 104, Cards: []int{101, 102}, Tokens: []int{-1, 1, 0, 0, 0, 0}})
	gemExpansionApply(t, s, Action{Type: "buy", Card: 104, Cards: []int{101, 102}})
	g := s.Splendor
	p = &g.Players[0]
	if p.Score != 6 || p.Bonus[0] != 1 || p.Bonus[2] != 1 || len(p.Nobles) != 1 || !p.hasPost(GemPostDoubleGold) || len(g.Exiled) != 2 {
		t.Fatal("sacrifice changed permanent awards or counted wrong bonuses", p)
	}
	if len(g.Market[5]) != 1 || g.Market[5][0].ID != 0 {
		t.Fatal("depleted market position moved")
	}
}

func TestSplendorOrientReservationPrivacyDeckAndElimination(t *testing.T) {
	for _, choose := range []bool{false, true} {
		s := gemOrientTest(t)
		s.Splendor.Options.TradingPosts = true
		s.Splendor.Players[0].TradingPosts = []int{GemPostBlindReserve}
		s.Splendor.Decks[3] = []Card{orientFixture(501, 1, -1, GemOrientGold), orientFixture(502, 1, -1, GemOrientCopy), orientFixture(503, 1, -1, GemOrientGold)}
		gemExpansionApply(t, s, Action{Type: "reserve", Tier: 1, Choice: "orient"})
		for _, viewer := range []int{-1, 1, 2} {
			raw, _ := json.Marshal(s.View(viewer))
			if strings.Contains(string(raw), "501") || strings.Contains(string(raw), "502") || strings.Contains(string(raw), "reserveChoice") {
				t.Fatal("private orient draw leaked")
			}
		}
		if len(s.View(0)["splendor"].(map[string]any)["remaining"].([]int)) != 6 {
			t.Fatal("orient deck counts missing")
		}
		restored := clone(*s)
		s = &restored
		if choose {
			gemExpansionApply(t, s, Action{Type: "gem_reserve", Card: 502})
			g := s.Splendor
			if len(g.Decks[0]) != 0 || g.Decks[3][0].ID != 503 || g.Decks[3][1].ID != 501 || !g.CardEvents[0].Orient || g.CardEvents[0].Card != nil {
				t.Fatal("wrong deck or reservation animation leak")
			}
			s.Turn = 0
		}
		if err := s.EliminateSplendor(0); err != nil {
			t.Fatal(err)
		}
		if len(s.Splendor.Decks[3]) != 3 || len(s.Splendor.Decks[0]) != 0 || len(s.Splendor.ReserveChoice) != 0 {
			t.Fatal("elimination lost or mixed cards")
		}
	}
}

func TestSplendorOrientBotsResolveEveryChoice(t *testing.T) {
	s := gemOrientTest(t)
	gemOwnFixture(s, 0, orientFixture(101, 1, 0, ""), orientFixture(102, 1, -1, GemOrientGold))
	c := orientFixture(103, 3, 1, GemOrientCascade)
	c.Cost = []int{1, 1, 1, 0, 0}
	s.Splendor.Market[5] = []Card{c}
	s.Splendor.Market[4] = []Card{orientFixture(104, 2, -1, GemOrientCopyCascade)}
	s.Splendor.Market[3] = []Card{orientFixture(105, 1, -1, GemOrientCopy)}
	for step := 0; s.Turn == 0 && step < 10; step++ {
		a, err := s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		gemExpansionApply(t, s, a)
		restored := clone(*s)
		s = &restored
	}
	if s.Turn != 1 || len(s.Splendor.Players[0].Cards) != 4 || len(s.Splendor.Exiled) != 1 {
		t.Fatal("bot failed to pay virtual gold and resolve chain", s.Phase)
	}
}

func TestSplendorOrientRequiresCopySourceAndSkipsEmptyCascade(t *testing.T) {
	s := gemOrientTest(t)
	s.Splendor.Market[3] = []Card{orientFixture(101, 1, -1, GemOrientCopy)}
	gemExpansionReject(t, s, Action{Type: "buy", Card: 101})
	s.Splendor.Market[5] = []Card{orientFixture(102, 3, 0, GemOrientCascade)}
	gemExpansionApply(t, s, Action{Type: "buy", Card: 102})
	if s.Turn != 1 || s.Phase != "turn" {
		t.Fatal("empty cascade stuck")
	}
	// Reserving a copy before owning another card remains legal.
	gemExpansionApply(t, s, Action{Type: "reserve", Card: 101})
	if len(s.Splendor.Players[1].Reserved) != 1 {
		t.Fatal("copy reservation unavailable")
	}
}

func TestSplendorOrientCombinationSimulationsConserveCardsAndTokens(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		for n := 2; n <= 4; n++ {
			t.Run(fmt.Sprintf("combination-%d-players-%d", mask, n), func(t *testing.T) {
				s, err := NewSplendor(n, SplendorOptions{TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0})
				if err != nil {
					t.Fatal(err)
				}
				g := s.Splendor
				g.Options.Orient = true
				g.Options.Cities = mask&4 != 0
				g.Market = append(g.Market, make([][]Card, 3)...)
				g.Decks = append(g.Decks, make([][]Card, 3)...)
				if g.Options.Cities {
					g.Nobles = nil
					g.Cities = []GemCity{{Name: "测试城市", Points: 17}}
				}
				for tier := 1; tier <= 3; tier++ {
					for color := 0; color < 5; color++ {
						for variant := 0; variant < 2; variant++ {
							id := 1000 + tier*100 + color*2 + variant
							power := [][]string{{GemOrientGold, GemOrientCopy}, {GemOrientDouble, GemOrientCopyCascade}, {GemOrientCascade, GemOrientSacrifice}}[tier-1][variant]
							c := orientFixture(id, tier, color, power)
							c.Points = tier - 1
							c.Cost[(color+1)%5] = tier
							if power == GemOrientGold || c.gemCopy() {
								c.Color = -1
							}
							if power == GemOrientSacrifice {
								c.Points = 3
								c.SacrificeColor = (color + 1) % 5
								c.Cost = make([]int, 5)
							}
							g.Decks[tier+2] = append(g.Decks[tier+2], c)
						}
					}
					shuffle(g.Decks[tier+2])
					g.Market[tier+2] = append([]Card{}, g.Decks[tier+2][:2]...)
					g.Decks[tier+2] = g.Decks[tier+2][2:]
				}
				initial := append([]int{}, g.Bank...)
				check := func() {
					t.Helper()
					g = s.Splendor
					counts := map[int]int{}
					add := func(cards []Card) {
						for _, c := range cards {
							if c.ID > 0 {
								counts[c.ID]++
							}
						}
					}
					for _, row := range g.Market {
						add(row)
					}
					for _, deck := range g.Decks {
						add(deck)
					}
					add(g.Exiled)
					add(g.ReserveChoice)
					tokens := append([]int{}, g.Bank...)
					for _, p := range g.Players {
						add(p.Cards)
						add(p.Reserved)
						bonus := make([]int, 5)
						score := len(p.Nobles) * 3
						for _, c := range p.Cards {
							if c.gemBonus() > 0 {
								bonus[c.Color] += c.gemBonus()
							}
							score += c.Points
						}
						if p.hasPost(GemPostPrestige) {
							score += len(p.TradingPosts)
						}
						if !reflect.DeepEqual(bonus, p.Bonus) || score != p.Score {
							t.Fatal("owned cards disagree with bonuses/score")
						}
						for i, v := range p.Tokens {
							if v < 0 {
								t.Fatal("negative tokens")
							}
							tokens[i] += v
						}
					}
					if !reflect.DeepEqual(tokens, initial) || len(counts) != 120 {
						t.Fatal("lost cards or tokens", len(counts), tokens)
					}
					for id, count := range counts {
						if count != 1 {
							t.Fatal("duplicate card", id)
						}
					}
					for _, v := range g.Bank {
						if v < 0 {
							t.Fatal("negative bank")
						}
					}
				}
				check()
				for steps := 0; !s.Finished && steps < 2500; steps++ {
					a, err := s.BotAction(s.Turn)
					if err != nil {
						t.Fatal(steps, s.Phase, err)
					}
					gemExpansionApply(t, s, a)
					check()
					if steps%19 == 0 {
						restored := clone(*s)
						s = &restored
						check()
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatal("combination bots failed to finish")
				}
				orient := 0
				for _, p := range s.Splendor.Players {
					for _, c := range p.Cards {
						if c.Orient != "" {
							orient++
						}
					}
				}
				if orient == 0 {
					t.Fatal("simulation never acquired expansion cards")
				}
			})
		}
	}
}
