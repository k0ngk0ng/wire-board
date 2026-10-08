package game

import (
	"encoding/json"
	"errors"
	"slices"
)

// Saved Explorer aggregate; each public room recipe has its own acceptance gate.
type catanExplorer struct {
	Spice        *catanExplorerSpice   `json:"spice,omitempty"`
	Fish         *catanExplorerFish    `json:"fish,omitempty"`
	Setup        *catanExplorerSetup   `json:"setup,omitempty"`
	Pirate       *catanExplorerPirate  `json:"pirate,omitempty"`
	Lairs        *catanExplorerLairs   `json:"lairs,omitempty"`
	ActionID     uint64                `json:"actionId,omitempty"`
	Motion       *catanExplorerMotion  `json:"motion,omitempty"`
	SkippedRolls int                   `json:"skippedRolls,omitempty"` // Platform removal before production, never a fabricated dice roll.
	Board        *catanExplorerBoard   `json:"board"`
	Fleet        *catanExplorerSailing `json:"fleet"`
	Cargo        *catanExplorerCargo   `json:"cargo"`
	Economy      *catanExplorerEconomy `json:"economy"`
}

type catanExplorerDiscovery struct {
	Tile      int   `json:"tile"`
	Resource  int   `json:"resource"`
	Number    int   `json:"number"`
	Resources []int `json:"resources"`
	Gold      int   `json:"gold"`
}

type catanExplorerVoyage struct {
	Sail        catanExplorerSailQuote   `json:"sail"`
	Discoveries []catanExplorerDiscovery `json:"discoveries"`
}

func newCatanExplorerLandHoWorld(players int) (*Catan, *catanExplorer, error) {
	g, board, fleet, cargo, err := newCatanExplorerLandHo(players)
	if err != nil {
		return nil, nil, err
	}
	economy, err := newCatanExplorerEconomy(g, fleet, cargo)
	if err != nil {
		return nil, nil, err
	}
	x := &catanExplorer{Board: board, Fleet: fleet, Cargo: cargo, Economy: economy}
	return g, x, x.validate(g)
}

func (x catanExplorer) validate(g *Catan) error {
	if x.Board != nil && x.Board.CitiesKnights || g != nil && g.CitiesKnights != nil {
		return errors.New("探险家与城市骑士组合尚未完成整局控制器验收")
	}
	return x.validateComponents(g)
}

// Component transactions may be exercised by the private city controller.
// The standalone world validator above rejects combinations; combined State
// validation additionally checks cities, cards, events and turn metadata.
func (x catanExplorer) validateComponents(g *Catan) error {
	if g == nil || x.Board == nil || x.Fleet == nil || x.Cargo == nil || x.Economy == nil || x.Board.Scenario != "land-ho" && !catanExplorerPirateScenario(x.Board.Scenario) || x.Cargo.Scenario != x.Board.Scenario {
		return errors.New("探险地图、航行、货物或经济组件不匹配；其他任务尚未完整接入")
	}
	if err := x.Board.validate(g); err != nil {
		return err
	}
	if x.Board.Fishing != "" {
		return errors.New("探险渔夫尚未接通完整行动控制器")
	}
	if err := x.Economy.validate(g, x.Fleet, x.Cargo); err != nil {
		return err
	}
	if x.Board.Scenario == "land-ho" {
		if x.Setup != nil || x.Pirate != nil || x.Lairs != nil {
			return errors.New("初航不能包含海盗任务")
		}
	} else {
		if x.Pirate == nil || (catanExplorerMissionScenario(x.Board.Scenario) != (x.Lairs != nil)) {
			return errors.New("海盗巢穴任务组件缺失")
		}
		if err := x.Pirate.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
			return err
		}
		if x.Lairs != nil {
			if err := x.Lairs.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
				return err
			}
			for _, site := range x.Lairs.Sites {
				if site.Ready > 0 && site.Resolved == 0 && (x.Economy.Turn == nil || site.Ready != x.Economy.Turn.Sequence || site.Captor != x.Economy.Turn.Player) {
					return errors.New("尚未结算的巢穴不能跨回合")
				}
			}
			for _, h := range x.Board.Hidden {
				if h.Revealed && h.Resource == CatanGold && x.Lairs.site(h.Tile) < 0 {
					return errors.New("已发现的金矿缺少巢穴标记")
				}
			}
		}
		if x.Setup == nil && x.Economy.Turn != nil && x.Economy.Turn.Phase == "pirate" && (x.Pirate.Pending == nil || x.Pirate.Pending.Resume != "action") {
			return errors.New("七点海盗回应记录缺失")
		}
		if x.Setup != nil {
			if err := x.Setup.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
				return err
			}
		}
	}
	if catanExplorerFishScenario(x.Board.Scenario) != (x.Fish != nil) {
		return errors.New("鱼群任务与地图剧本不符")
	}
	if x.Fish != nil {
		if err := x.Fish.validate(g, x.Board, x.Fleet, x.Cargo); err != nil {
			return err
		}
		for _, loc := range x.Cargo.Fish {
			if loc.Kind == "shoal" && x.Pirate.Owner >= 0 && loc.Index == x.Pirate.Tile {
				return errors.New("海盗所在渔场不能残留鱼群")
			}
		}
	}
	if (catanExplorerSpiceScenario(x.Board.Scenario)) != (x.Spice != nil) {
		return errors.New("香料任务与地图剧本不符")
	}
	if x.Spice != nil {
		if err := x.Spice.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
			return err
		}
	}
	if g.Robber != -1 || g.LongestOwner != -1 || g.ArmyOwner != -1 || len(g.DevDeck)+len(g.DevDiscard) != 0 {
		return errors.New("初航不使用强盗、发展卡或最长道路/军队奖")
	}
	for p, player := range g.Players {
		if player.Eliminated {
			if sum(player.Resources) != 0 || x.Economy.Gold[p] != 0 {
				return errors.New("离场探险玩家不能保留资源或金币")
			}
			for ship := p * 3; ship < (p+1)*3; ship++ {
				if x.Fleet.Positions[ship] != -1 {
					return errors.New("离场探险玩家的船须归还供应区")
				}
			}
			for unit := p * 11; unit < (p+1)*11; unit++ {
				if x.Cargo.Units[unit] != (catanExplorerCargoLocation{"supply", -1}) {
					return errors.New("离场探险玩家的移民和船员须归还供应区")
				}
			}
		}
		score := 0
		for _, v := range g.Vertices {
			if v.Owner == p {
				score += v.Level
			}
		}
		if x.Lairs != nil {
			score += x.Lairs.scores()[p]
		}
		if x.Fish != nil {
			score += x.Fish.publicView(len(g.Players)).Scores[p]
		}
		if x.Spice != nil {
			score += x.Spice.publicView(g).Scores[p]
		}
		if k := g.CitiesKnights; k != nil {
			score += k.Players[p].DefenderPoints + k.Players[p].ProgressPoints
			if k.Merchant != nil && k.Merchant.Owner == p {
				score++
			}
			for track := range 3 {
				if g.cityMetropolisOwner(track) == p {
					score += 2
				}
			}
		}
		if player.Score != score || !catanBundle(player.Dev) || !catanBundle(player.NewDev) || sum(player.Dev)+sum(player.NewDev) != 0 {
			return errors.New("初航建筑分数或发展卡库存不符")
		}
	}
	if x.Fleet.Turn != nil && len(x.Fleet.Turn.Exploring) != 0 {
		return errors.New("公开完成操作前必须同时完成探索揭示与奖励")
	}
	for _, edge := range x.Fleet.Positions {
		if edge < 0 {
			continue
		}
		for _, tile := range g.Tiles {
			if tile.Resource == CatanFog && catanExplorerTouches(g, edge, tile.ID) {
				return errors.New("船端触及的迷雾尚未完成探索")
			}
		}
	}
	return nil
}

// Mutating integration actions run on a complete snapshot. A later failed
// reward must not leave an earlier reveal, number draw, payment or move behind.
func (x catanExplorer) copy(g *Catan) (*Catan, *catanExplorer, error) {
	if err := x.validateComponents(g); err != nil {
		return nil, nil, err
	}
	type snapshot struct {
		Game     *Catan         `json:"game"`
		Explorer *catanExplorer `json:"explorer"`
	}
	base := *g
	base.Explorer = nil // Do not serialize the aggregate twice through Catan.
	b, err := json.Marshal(snapshot{&base, &x})
	if err != nil {
		return nil, nil, err
	}
	var out snapshot
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, nil, err
	}
	if g.Explorer != nil {
		out.Game.Explorer = out.Explorer
	}
	return out.Game, out.Explorer, nil
}

// Only called inside a snapshot transaction, with tiles derived from actual
// ship contact. There is deliberately no arbitrary reveal/claim-reward action.
func (x *catanExplorer) discover(g *Catan, player int, tiles []int) ([]catanExplorerDiscovery, error) {
	result := []catanExplorerDiscovery{}
	for _, tile := range tiles {
		hidden, err := x.Board.reveal(g, tile)
		if err != nil {
			return nil, err
		}
		award := catanExplorerDiscovery{Tile: tile, Resource: hidden.Resource, Number: hidden.Number, Resources: make([]int, 5)}
		if hidden.Resource >= 0 && hidden.Resource < 5 {
			// Inherit finite resource supply: an empty bank cannot pay this
			// reward, but mandatory discovery still reveals and stops the ship.
			// The two-gold alternative is for terrain without a resource type,
			// not a substitute for an exhausted ordinary-resource pile.
			if g.Bank[hidden.Resource] > 0 {
				g.Bank[hidden.Resource]--
				g.Players[player].Resources[hidden.Resource]++
				award.Resources[hidden.Resource] = 1
			}
		} else if hidden.Resource == CatanSea || hidden.Resource == CatanGold && x.Lairs != nil || hidden.Farm != "" && x.Spice != nil {
			if hidden.Resource == CatanGold {
				if err := x.Lairs.discover(g, x.Board, tile); err != nil {
					return nil, err
				}
			}
			if hidden.Farm != "" {
				if err := x.Cargo.discoverSpice(g, x.Board, x.Fleet, tile); err != nil {
					return nil, err
				}
			}
			if err := x.Economy.ensureGold(2); err != nil {
				return nil, err
			}
			x.Economy.GoldBank -= 2
			x.Economy.Gold[player] += 2
			award.Gold = 2
		} else {
			return nil, errors.New("此任务地块的探索标记及奖励尚未接入")
		}
		result = append(result, award)
	}
	return result, nil
}

func (x *catanExplorer) sail(g *Catan, player int, sequence uint64, ship int, path []int) (catanExplorerVoyage, error) {
	q, next, err := x.copy(g)
	if err != nil {
		return catanExplorerVoyage{}, err
	}
	if err = next.Economy.productionAllowed(q, next.Fleet, next.Cargo, player, sequence, "ready"); err != nil {
		return catanExplorerVoyage{}, err
	}
	if err = next.Cargo.allowed(q, next.Fleet, player, sequence, "movement"); err != nil {
		return catanExplorerVoyage{}, err
	}
	pirateOwner, pirateTile := -1, -1
	if next.Pirate != nil {
		pirateOwner, pirateTile = next.Pirate.Owner, next.Pirate.Tile
	}
	quote, err := next.Fleet.sail(q, player, sequence, ship, path, pirateOwner, pirateTile, next.Economy.Gold, &next.Economy.GoldBank)
	if err != nil {
		return catanExplorerVoyage{}, err
	}
	result := catanExplorerVoyage{Sail: quote, Discoveries: []catanExplorerDiscovery{}}
	if len(quote.Exploring) > 0 {
		result.Discoveries, err = next.discover(q, player, quote.Exploring)
		if err != nil {
			return catanExplorerVoyage{}, err
		}
		if err = next.Fleet.discovered(q, player, sequence); err != nil {
			return catanExplorerVoyage{}, err
		}
	}
	if err = next.validateComponents(q); err != nil {
		return catanExplorerVoyage{}, err
	}
	*g, *x = *q, *next
	if g.Explorer != nil {
		g.Explorer = x
	}
	return result, nil
}

func (x *catanExplorer) buildShip(g *Catan, player int, sequence uint64, ship, edge int) ([]catanExplorerDiscovery, error) {
	q, next, err := x.copy(g)
	if err != nil {
		return nil, err
	}
	if err = next.Economy.actionAllowed(q, next.Fleet, next.Cargo, player, sequence); err != nil {
		return nil, err
	}
	if err = next.Cargo.buildShip(q, next.Fleet, player, sequence, ship, edge); err != nil {
		return nil, err
	}
	tiles := []int{}
	for _, tile := range q.Tiles {
		if tile.Resource == CatanFog && catanExplorerTouches(q, edge, tile.ID) {
			tiles = append(tiles, tile.ID)
		}
	}
	result, err := next.discover(q, player, tiles)
	if err != nil {
		return nil, err
	}
	if len(tiles) > 0 && !slices.Contains(next.Cargo.Turn.BuildStopped, ship) {
		next.Cargo.Turn.BuildStopped = append(next.Cargo.Turn.BuildStopped, ship)
	}
	if err = next.validateComponents(q); err != nil {
		return nil, err
	}
	*g, *x = *q, *next
	if g.Explorer != nil {
		g.Explorer = x
	}
	return result, nil
}
