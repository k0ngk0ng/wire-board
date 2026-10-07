package game

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

// All 30 Orient entries are cross-checked against BGA component facts; 24 also
// have independent physical-image evidence. Cities remain partially verified,
// with two physical faces and reverse pairings still unverified. The physical
// Madrid conditions support the candidate, not BGA's additional blue variant.
// The public constructor uses only the corroborated Orient table. Cities remain
// restricted to the private integration constructor.
// See docs/research/splendor-modern-catalog.md for the exact evidence limits.
//
//go:embed splendor_modern_catalog.json
var splendorModernCatalogJSON []byte

type splendorModernCatalog struct {
	Rules  string    `json:"rules"`
	Status string    `json:"status"`
	Orient []Card    `json:"orient"`
	Cities []GemCity `json:"cities"`
}

func readSplendorModernCatalog() (splendorModernCatalog, error) {
	var catalog splendorModernCatalog
	if err := json.Unmarshal(splendorModernCatalogJSON, &catalog); err != nil {
		return catalog, err
	}
	return catalog, catalog.validate()
}

func (c splendorModernCatalog) validate() error {
	if c.Rules != SplendorExpansionRules || c.Status != "secondary-source-partially-crosschecked" || len(c.Orient) != 30 || len(c.Cities) != 14 {
		return errors.New("新版宝石候选牌表的规则版本、来源状态或数量不符")
	}
	effects := map[string]int{}
	for i, card := range c.Orient {
		if card.ID != 1001+i || card.Tier != i/10+1 || len(card.Cost) != 5 || card.Color < -1 || card.Color >= 5 {
			return fmt.Errorf("东方候选卡%d的编号、等级或颜色无效", i+1)
		}
		for _, n := range card.Cost {
			if n < 0 || n > 6 {
				return errors.New("东方候选卡费用无效")
			}
		}
		effects[card.Orient]++
		switch card.Orient {
		case GemOrientGold, GemOrientCopy:
			if card.Tier != 1 || card.Points != 0 || card.Color != -1 || card.BonusCount != 0 || sum(card.Cost) != map[string]int{GemOrientGold: 3, GemOrientCopy: 5}[card.Orient] {
				return errors.New("一级东方候选卡与组件类型不符")
			}
		case GemOrientDouble:
			if card.Tier != 2 || card.Points != 1 || card.Color < 0 || card.BonusCount != 2 || sum(card.Cost) != 7 {
				return errors.New("双奖励候选卡无效")
			}
		case GemOrientCopyCascade:
			if card.Tier != 2 || card.Points != 1 || card.Color != -1 || card.BonusCount != 0 || sum(card.Cost) != 8 {
				return errors.New("复制连锁候选卡无效")
			}
		case GemOrientCascade:
			if card.Tier != 3 || card.Points != 1 || card.Color < 0 || card.BonusCount != 0 || sum(card.Cost) != 10 {
				return errors.New("三级连锁候选卡无效")
			}
		case GemOrientSacrifice:
			if card.Tier != 3 || card.Points != 3 || card.Color < 0 || card.BonusCount != 0 || card.SacrificeColor < 0 || card.SacrificeColor >= 5 || sum(card.Cost) != 0 {
				return errors.New("弃牌购买候选卡无效")
			}
		default:
			return errors.New("未知东方候选卡效果")
		}
	}
	for _, kind := range []string{GemOrientGold, GemOrientCopy, GemOrientDouble, GemOrientCopyCascade, GemOrientCascade, GemOrientSacrifice} {
		if effects[kind] != 5 {
			return errors.New("东方候选牌表各效果应有5张")
		}
	}
	for i, city := range c.Cities {
		if city.Tile != i/2+1 || city.Side != i%2 || city.Name == "" || city.Points < 12 || city.Points > 17 || city.Any < 0 || city.Any > 6 {
			return errors.New("城市候选面编号、目标或名称无效")
		}
		if i%2 == 1 && city.Name != c.Cities[i-1].Name {
			return errors.New("同一实体城市的两面名称不一致")
		}
		for _, n := range city.Cost {
			if n < 0 || n > 4 {
				return errors.New("城市候选颜色条件无效")
			}
		}
	}
	return nil
}

// Internal integration path, not a public room recipe. Keeps the catalog's
// unverified provenance distinct from generated synthetic-rule fixtures.
func newSplendorModernCatalogState(n int, options SplendorOptions) (*State, error) {
	if options.Rules != "" && options.Rules != SplendorExpansionRules {
		return nil, errors.New("不支持的璀璨宝石扩展规则版本")
	}
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		return nil, err
	}
	// Cities remove all nobles, including the optional bonus pair.
	if options.Cities {
		options.ExtraNobles = false
	}
	s, err := NewSplendor(n, SplendorOptions{TradingPosts: options.TradingPosts, Strongholds: options.Strongholds, ExtraNobles: options.ExtraNobles})
	if err != nil {
		return nil, err
	}
	if options.expanded() {
		options.Rules = SplendorExpansionRules
	}
	g := s.Splendor
	g.Options = options
	if options.Orient || options.Cities {
		g.Catalog = "2025-secondary-v1"
	}
	if options.Orient {
		g.installOrient(catalog.Orient)
	}
	if options.Cities {
		g.Nobles = []Noble{}
		tiles := []int{0, 1, 2, 3, 4, 5, 6}
		shuffle(tiles)
		for _, tile := range tiles[:3] {
			sides := []int{0, 1}
			shuffle(sides)
			g.Cities = append(g.Cities, catalog.Cities[tile*2+sides[0]])
		}
	}
	return s, nil
}

// installOrient adds three independent ten-card tiers to a fresh base setup.
func (g *Splendor) installOrient(cards []Card) {
	g.Decks = append(g.Decks, make([][]Card, 3)...)
	g.Market = append(g.Market, make([][]Card, 3)...)
	for _, card := range cards {
		g.Decks[card.gemDeck()] = append(g.Decks[card.gemDeck()], card)
	}
	for row := 3; row < 6; row++ {
		shuffle(g.Decks[row])
		g.Market[row] = append([]Card{}, g.Decks[row][:2]...)
		g.Decks[row] = g.Decks[row][2:]
	}
}
