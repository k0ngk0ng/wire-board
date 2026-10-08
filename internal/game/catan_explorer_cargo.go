package game

import (
	"errors"
	"slices"
)

// Each player owns two settlers followed by nine crew: stable ID = player*11
// + component slot. Locations partition the physical inventory, including the
// supply. Fish are shared size-two pieces tracked separately from units.
type catanExplorerCargoLocation struct {
	Kind  string `json:"kind"` // supply, ship, harbor; lair/farm (crew), shoal (fish), farm (spice).
	Index int    `json:"index"`
}
type catanExplorerCargoTurn struct {
	Player       int    `json:"player"`
	Sequence     uint64 `json:"sequence"`
	Phase        string `json:"phase"`                  // action, movement, ended.
	BuildStopped []int  `json:"buildStopped,omitempty"` // Newly built ships that discovered fog before movement.
}
type catanExplorerCargo struct {
	Scenario string                       `json:"scenario"`
	Units    []catanExplorerCargoLocation `json:"units"`
	Fish     []catanExplorerCargoLocation `json:"fish,omitempty"`
	Spice    []catanExplorerSpiceSack     `json:"spice,omitempty"`
	Turn     *catanExplorerCargoTurn      `json:"turn,omitempty"`
}

func catanExplorerUnitSize(id int) int {
	if id%11 < 2 {
		return 2
	}
	return 1
}
func catanExplorerUnitCost(id int) []int {
	if id%11 < 2 {
		return []int{1, 1, 1, 1, 0}
	}
	return []int{0, 0, 1, 0, 1}
}
func newCatanExplorerCargo(g *Catan, fleet *catanExplorerSailing, scenario string) (*catanExplorerCargo, error) {
	if g == nil {
		return nil, errors.New("探险地图缺失")
	}
	c := &catanExplorerCargo{Scenario: scenario, Units: make([]catanExplorerCargoLocation, len(g.Players)*11)}
	for i := range c.Units {
		c.Units[i] = catanExplorerCargoLocation{"supply", -1}
	}
	if catanExplorerFishScenario(scenario) {
		c.Fish = make([]catanExplorerCargoLocation, catanExplorerStock(len(g.Players)).fish)
		for i := range c.Fish {
			c.Fish[i] = catanExplorerCargoLocation{"supply", -1}
		}
	}
	if catanExplorerSpiceScenario(scenario) {
		c.Spice = make([]catanExplorerSpiceSack, catanExplorerStock(len(g.Players)).spice)
		for i := range c.Spice {
			c.Spice[i] = catanExplorerSpiceSack{Origin: -1, Owner: -1, At: catanExplorerCargoLocation{"supply", -1}}
		}
	}
	return c, c.validate(g, fleet)
}
func catanExplorerLandVertex(g *Catan, vertex int) bool {
	if vertex < 0 || vertex >= len(g.Vertices) {
		return false
	}
	land := false
	for _, tile := range g.Tiles {
		if !slices.Contains(tile.Vertices, vertex) {
			continue
		}
		if tile.Resource == CatanFog || tile.Resource == CatanGold && tile.Number == 0 || tile.Resource < 0 || tile.Resource > CatanGold && tile.Resource != catanLake {
			return false
		}
		land = land || tile.Resource < CatanSea || tile.Resource == CatanGold || tile.Resource == catanLake
	}
	return land
}
func catanExplorerCoast(g *Catan, vertex int) bool {
	for _, edge := range g.Edges {
		if (edge.A == vertex || edge.B == vertex) && catanExplorerSeaEdge(g, edge.ID) {
			return true
		}
	}
	return false
}
func (c catanExplorerCargo) contents(location catanExplorerCargoLocation) []int {
	ids := []int{}
	for id, loc := range c.Units {
		if loc == location {
			ids = append(ids, id)
		}
	}
	return ids
}
func (c catanExplorerCargo) used(location catanExplorerCargoLocation) int {
	used := 0
	for _, id := range c.contents(location) {
		used += catanExplorerUnitSize(id)
	}
	for _, loc := range c.Fish {
		if loc == location {
			used += 2
		}
	}
	for _, sack := range c.Spice {
		if sack.At == location {
			used++
		}
	}
	return used
}
func (c catanExplorerCargo) holder(g *Catan, fleet *catanExplorerSailing, owner int, location catanExplorerCargoLocation) bool {
	switch location.Kind {
	case "ship":
		return location.Index >= 0 && location.Index < len(fleet.Positions) && location.Index/3 == owner && fleet.Positions[location.Index] >= 0
	case "harbor":
		return location.Index >= 0 && location.Index < len(g.Vertices) && g.Vertices[location.Index].Owner == owner && catanExplorerHarborAt(g, location.Index)
	}
	return false
}
func (c catanExplorerCargo) validate(g *Catan, fleet *catanExplorerSailing) error {
	if g == nil || fleet == nil || len(c.Units) != len(g.Players)*11 || c.Scenario != "land-ho" && c.Scenario != "pirate-lairs" && !catanExplorerFishScenario(c.Scenario) || g.Seafarers != nil || g.Two != nil || g.Transport != nil {
		return errors.New("探险货物库存、剧本或组合无效")
	}
	if k := g.CitiesKnights; k != nil && (len(g.Players) < 3 || k.Rules != catanCitiesKnightsRules(len(g.Players)) || len(k.Players) != len(g.Players)) {
		return errors.New("探险城市骑士人数或组件无效")
	}
	if g.Attack != nil || g.Caravans != nil || g.Rivers != nil || g.Fishing != nil && g.Fishing.Explorer != catanExplorerFishingRule(len(g.Players)) || g.RevealedEvent != nil && (g.EventDeck == nil || g.EventDeck.Explorer != CatanEventExplorerRules) || g.CardEvent != nil || g.FriendlyRobber != nil || g.Harbors != nil || g.BaseSetup != nil || g.GoldPending != nil {
		return errors.New("探险货物尚未接入其他扩展组合")
	}
	if err := g.validateExplorerHelperInventory(); err != nil {
		return err
	}
	if pair := g.Paired; pair != nil && (len(g.Players) < 5 || len(g.Players) > 6 || pair.Primary < 0 || pair.Primary >= len(g.Players) || pair.Secondary < 0 || pair.Secondary >= len(g.Players)) {
		return errors.New("探险配对玩家标记无效")
	}
	if err := fleet.validate(g); err != nil {
		return err
	}
	fallen := map[int]bool{}
	if k := g.CitiesKnights; k != nil {
		for _, v := range k.FallenCities {
			if v < 0 || v >= len(g.Vertices) || fallen[v] || g.Vertices[v].Level != 1 || g.Vertices[v].Harbor || g.Vertices[v].Owner < 0 {
				return errors.New("横置城市记录无效")
			}
			fallen[v] = true
		}
	}
	settlements, harbors, cities, roads := map[int]int{}, map[int]int{}, map[int]int{}, map[int]int{}
	for _, v := range g.Vertices {
		if v.Level == 0 && v.Owner == -1 && !v.Harbor {
			continue
		}
		if v.Harbor && v.Level != 2 || v.Level < 1 || v.Level > 2 || v.Owner == -1 || v.Owner >= len(g.Players) || v.Owner < 0 && (len(g.Players) != 2 || v.Owner < -3) || !c.landVertex(g, v.Owner, v.ID) || catanExplorerHarborAt(g, v.ID) && !catanExplorerCoast(g, v.ID) {
			return errors.New("探险村庄或港口位置无效")
		}
		if v.Level == 1 && !fallen[v.ID] {
			settlements[v.Owner]++
		} else if catanExplorerHarborAt(g, v.ID) {
			harbors[v.Owner]++
		} else {
			cities[v.Owner]++
		}
		if settlements[v.Owner] > 5 || harbors[v.Owner] > 4 || cities[v.Owner] > 4 || v.Owner < 0 && (settlements[v.Owner] > 1 || harbors[v.Owner] > 1) {
			return errors.New("探险村庄或港口组件超额")
		}
	}
	for _, e := range g.Edges {
		if e.A < 0 || e.A >= len(g.Vertices) || e.B < 0 || e.B >= len(g.Vertices) || g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
			return errors.New("探险建筑违反距离规则")
		}
		if e.Ship || e.Bridge || e.Warship || e.Damaged || e.Owner < -3 || e.Owner >= len(g.Players) || e.Owner < -1 && len(g.Players) != 2 {
			return errors.New("探险道路组件或所有者无效")
		}
		if e.Owner != -1 {
			if !c.landEdge(g, e.Owner, e.ID) {
				return errors.New("探险道路不能位于纯海、迷雾或未解放金矿边")
			}
			roads[e.Owner]++
			if roads[e.Owner] > 15 || e.Owner < -1 && roads[e.Owner] > 1 {
				return errors.New("探险道路库存超额")
			}
		}
	}
	for id, loc := range c.Units {
		if loc.Kind == "supply" && loc.Index == -1 {
			continue
		}
		if loc.Kind == "lair" {
			if c.Scenario == "land-ho" || id%11 < 2 || loc.Index < 0 || loc.Index >= len(g.Tiles) || g.Tiles[loc.Index].Resource != CatanGold || c.used(loc) > 3 {
				return errors.New("巢穴船员位置、种类或数量无效")
			}
			continue
		}
		if loc.Kind == "farm" {
			if !catanExplorerSpiceScenario(c.Scenario) || id%11 < 2 || g.Players[id/11].Eliminated || loc.Index < 0 || loc.Index >= len(g.Tiles) || g.Tiles[loc.Index].Resource != CatanDesert || !c.farmFriend(id/11, loc.Index) {
				return errors.New("香料农场只能派驻已建立联系的己方船员")
			}
			for other := id / 11 * 11; other < id; other++ {
				if c.Units[other] == loc {
					return errors.New("每位玩家在每座农场只能派驻一名船员")
				}
			}
			continue
		}
		if c.Scenario == "land-ho" && id%11 >= 2 || !c.holder(g, fleet, id/11, loc) || c.used(loc) > 2 {
			return errors.New("探险单位位置、所属玩家或舱位容量无效")
		}
	}
	if catanExplorerFishScenario(c.Scenario) && len(c.Fish) != catanExplorerStock(len(g.Players)).fish || !catanExplorerFishScenario(c.Scenario) && len(c.Fish) != 0 {
		return errors.New("鱼群实体库存与剧本不符")
	}
	for _, loc := range c.Fish {
		if loc == (catanExplorerCargoLocation{"supply", -1}) {
			continue
		}
		if loc.Kind == "shoal" && loc.Index >= 0 && loc.Index < len(g.Tiles) && g.Tiles[loc.Index].Resource == CatanSea && c.used(loc) == 2 {
			continue
		}
		owner := -1
		if loc.Kind == "ship" && loc.Index >= 0 && loc.Index < len(fleet.Positions) {
			owner = loc.Index / 3
		}
		if loc.Kind == "harbor" && loc.Index >= 0 && loc.Index < len(g.Vertices) {
			owner = g.Vertices[loc.Index].Owner
		}
		if owner < 0 || owner >= len(g.Players) || g.Players[owner].Eliminated || !c.holder(g, fleet, owner, loc) || c.used(loc) != 2 {
			return errors.New("鱼群位置、归属或舱位容量无效")
		}
	}
	if err := c.validateSpiceCargo(g, fleet); err != nil {
		return err
	}
	t := c.Turn
	if t == nil {
		if fleet.Turn != nil {
			return errors.New("货物回合缺失")
		}
		return nil
	}
	if t.Player < 0 || t.Player >= len(g.Players) || t.Sequence == 0 || t.Phase != "action" && t.Phase != "movement" && t.Phase != "ended" {
		return errors.New("探险货物行动阶段无效")
	}
	q := fleet.Turn
	if t.Phase == "action" {
		if q != nil && (q.Open || q.Sequence >= t.Sequence) {
			return errors.New("建设阶段不能重开同次航行")
		}
	} else if q == nil || q.Player != t.Player || q.Sequence != t.Sequence || q.Open != (t.Phase == "movement") {
		return errors.New("货物与航行阶段不一致")
	}
	for i, ship := range t.BuildStopped {
		if ship < 0 || ship >= len(fleet.Positions) || ship/3 != t.Player || slices.Contains(t.BuildStopped[:i], ship) || t.Phase == "action" && fleet.Positions[ship] < 0 || t.Phase != "action" && !q.Ships[ship].Closed && q.Ships[ship].Second == nil {
			return errors.New("造船探索后的停船记录无效")
		}
	}
	return nil
}
func (c *catanExplorerCargo) beginAction(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64) error {
	if err := c.validate(g, fleet); err != nil {
		return err
	}
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || sequence == 0 || c.Turn != nil && (c.Turn.Phase != "ended" || sequence <= c.Turn.Sequence) {
		return errors.New("不能跳过或重开探险行动阶段")
	}
	c.Turn = &catanExplorerCargoTurn{Player: player, Sequence: sequence, Phase: "action"}
	return nil
}
func (c catanExplorerCargo) allowed(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, phase string) error {
	if err := c.validate(g, fleet); err != nil {
		return err
	}
	if c.Turn == nil || c.Turn.Player != player || c.Turn.Sequence != sequence || c.Turn.Phase != phase || g.Players[player].Eliminated {
		return errors.New("探险货物操作玩家、阶段或回应序号无效")
	}
	if phase == "movement" {
		return fleet.allowed(g, player, sequence)
	}
	return nil
}
func (c *catanExplorerCargo) beginMovement(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, speed int) error {
	if err := c.allowed(g, fleet, player, sequence, "action"); err != nil {
		return err
	}
	if err := fleet.begin(g, player, sequence, speed); err != nil {
		return err
	}
	for _, ship := range c.Turn.BuildStopped {
		fleet.Turn.Ships[ship].Remaining, fleet.Turn.Ships[ship].Closed = 0, true
	}
	c.Turn.Phase = "movement"
	return nil
}
func (c *catanExplorerCargo) endMovement(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64) error {
	if err := c.allowed(g, fleet, player, sequence, "movement"); err != nil {
		return err
	}
	if err := fleet.end(g, player, sequence); err != nil {
		return err
	}
	c.Turn.Phase = "ended"
	return nil
}

func catanExplorerCanPay(g *Catan, player int, cost []int) bool {
	if g == nil || player < 0 || player >= len(g.Players) || !catanBundle(cost) || len(g.Bank) != 5 && g.CitiesKnights == nil || len(g.Bank) != 8 && g.CitiesKnights != nil || !g.cardBundle(g.Bank) {
		return false
	}
	stock := catanExplorerStock(len(g.Players)).resources
	for _, p := range g.Players {
		if !g.cardBundle(p.Resources) {
			return false
		}
	}
	for resource := range g.Bank {
		total := g.Bank[resource]
		for _, p := range g.Players {
			total += p.Resources[resource]
		}
		expected := stock
		if resource >= 5 {
			expected = 12
			if len(g.Players) > 4 {
				expected = 18
			}
		}
		if total != expected || resource < 5 && g.Players[player].Resources[resource] < cost[resource] {
			return false
		}
	}
	return true
}
func catanExplorerPay(g *Catan, player int, cost []int) {
	for resource, amount := range cost {
		g.Players[player].Resources[resource] -= amount
		g.Bank[resource] += amount
	}
}
func (c catanExplorerCargo) docked(g *Catan, fleet *catanExplorerSailing, player, ship, harbor int) bool {
	if !c.holder(g, fleet, player, catanExplorerCargoLocation{"ship", ship}) || !c.holder(g, fleet, player, catanExplorerCargoLocation{"harbor", harbor}) {
		return false
	}
	e := g.Edges[fleet.Positions[ship]]
	return e.A == harbor || e.B == harbor
}
func (c catanExplorerCargo) buildLocation(g *Catan, fleet *catanExplorerSailing, player int, loc catanExplorerCargoLocation) bool {
	if !c.holder(g, fleet, player, loc) {
		return false
	}
	if loc.Kind == "harbor" {
		return true
	}
	for _, v := range g.Vertices {
		if c.docked(g, fleet, player, loc.Index, v.ID) {
			return true
		}
	}
	return false
}

func (c *catanExplorerCargo) buildHarbor(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, vertex int) error {
	return c.upgradeSettlement(g, fleet, player, sequence, vertex, "harbor", false)
}
func (c *catanExplorerCargo) buildShip(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, edge int) error {
	return c.buildShipCost(g, fleet, player, sequence, ship, edge, false)
}

func (c *catanExplorerCargo) buildShipCost(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, edge int, free bool) error {
	if err := c.allowed(g, fleet, player, sequence, "action"); err != nil {
		return err
	}
	if ship < 0 || ship >= len(fleet.Positions) || ship/3 != player || !catanExplorerSeaEdge(g, edge) {
		return errors.New("造船位置或船只组件无效")
	}
	e := g.Edges[edge]
	if !c.holder(g, fleet, player, catanExplorerCargoLocation{"harbor", e.A}) && !c.holder(g, fleet, player, catanExplorerCargoLocation{"harbor", e.B}) {
		return errors.New("只能在己方港口旁造船")
	}
	for _, tile := range e.Tiles {
		if g.Tiles[tile].Resource == CatanFog {
			return errors.New("不能在未探索地块的边上造船")
		}
	}
	for id, at := range fleet.Positions {
		if id != ship && at == edge {
			return errors.New("新船必须放在空海边；移动时才允许两艘停泊")
		}
		if fleet.Positions[ship] >= 0 && id/3 == player && at < 0 {
			return errors.New("有库存船时不能拆除已部署的船重新建造")
		}
	}
	cost := []int{1, 0, 1, 0, 0}
	if free {
		cost = []int{0, 0, 0, 0, 0}
	}
	if !catanExplorerCanPay(g, player, cost) {
		return errors.New("无法支付造船的木材与羊毛")
	}
	catanExplorerPay(g, player, cost)
	for _, unit := range c.contents(catanExplorerCargoLocation{"ship", ship}) {
		c.Units[unit] = catanExplorerCargoLocation{"supply", -1}
	}
	for id, loc := range c.Fish {
		if loc == (catanExplorerCargoLocation{"ship", ship}) {
			c.Fish[id] = catanExplorerCargoLocation{"supply", -1}
		}
	}
	for id, sack := range c.Spice {
		if sack.At == (catanExplorerCargoLocation{"ship", ship}) {
			c.Spice[id].At = catanExplorerCargoLocation{"supply", -1}
		}
	}
	fleet.Positions[ship] = edge
	// Returning a ship and paying to build a new one creates a new vessel;
	// the discovery controller will mark it again if its new berth reveals fog.
	c.Turn.BuildStopped = slices.DeleteFunc(c.Turn.BuildStopped, func(id int) bool { return id == ship })
	// An ended prior movement record must not retain the recycled ship's MPs.
	if fleet.Turn != nil {
		fleet.Turn.Ships[ship] = catanExplorerShipMove{Closed: true}
		if fleet.Turn.Current == ship {
			fleet.Turn.Current = -1
		}
	}
	return nil
}
func (c *catanExplorerCargo) buildUnit(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, unit int, target catanExplorerCargoLocation, discard []int) error {
	return c.buildUnitFreight(g, fleet, player, sequence, unit, target, discard, nil, nil)
}

// The rulebook's singular "a piece" does not explain a settler replacing two
// small pieces. Site supplement: clear the minimum pieces from one full berth,
// only when every legal building berth is full. Never a free discard action.
// Fish and spice have independent ID spaces, as in cargo transfer actions.
func (c catanExplorerCargo) buildBerthsFull(g *Catan, fleet *catanExplorerSailing, player int) bool {
	for _, v := range g.Vertices {
		loc := catanExplorerCargoLocation{"harbor", v.ID}
		if c.buildLocation(g, fleet, player, loc) && c.used(loc) < 2 {
			return false
		}
	}
	for ship := player * 3; ship < (player+1)*3; ship++ {
		loc := catanExplorerCargoLocation{"ship", ship}
		if c.buildLocation(g, fleet, player, loc) && c.used(loc) < 2 {
			return false
		}
	}
	return true
}

// Enumerate cargo choices without touching private discoveries or mutating state.
// Legality, payment and minimum removal are checked by buildUnitFreight.
func (c catanExplorerCargo) recruitReturns(loc catanExplorerCargoLocation) []Action {
	options := []Action{{}}
	all := Action{}
	for _, id := range c.contents(loc) {
		options = append(options, Action{Cards: []int{id}})
		all.Cards = append(all.Cards, id)
	}
	for _, id := range c.fishContents(loc) {
		options = append(options, Action{Targets: []int{id}})
		all.Targets = append(all.Targets, id)
	}
	for _, id := range c.spiceContents(loc) {
		options = append(options, Action{SpiceUnload: []int{id}})
		all.SpiceUnload = append(all.SpiceUnload, id)
	}
	if len(options) == 3 {
		options = append(options, all)
	}
	return options
}

func (c *catanExplorerCargo) buildUnitFreight(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, unit int, target catanExplorerCargoLocation, discard, discardFish, discardSpice []int) error {
	if err := c.allowed(g, fleet, player, sequence, "action"); err != nil {
		return err
	}
	if unit < 0 || unit >= len(c.Units) || unit/11 != player || c.Scenario == "land-ho" && unit%11 >= 2 || !c.buildLocation(g, fleet, player, target) {
		return errors.New("请选择可建造的单位及己方港口或停靠船舱")
	}
	size, remaining := catanExplorerUnitSize(unit), c.used(target)
	returns := len(discard) + len(discardFish) + len(discardSpice)
	if returns > 2 {
		return errors.New("只能归还同一舱内建造所需的货物，最多两件")
	}
	removedSizes := []int{}
	if returns > 0 {
		if !c.buildBerthsFull(g, fleet, player) {
			return errors.New("仍有未满的港口或停靠船舱，不能归还货物")
		}
		for i, id := range discard {
			if id < 0 || id >= len(c.Units) || id/11 != player || c.Units[id] != target || slices.Contains(discard[:i], id) {
				return errors.New("只能归还目标舱位中的己方单位")
			}
			remaining -= catanExplorerUnitSize(id)
			removedSizes = append(removedSizes, catanExplorerUnitSize(id))
		}
		for i, id := range discardFish {
			if id < 0 || id >= len(c.Fish) || c.Fish[id] != target || slices.Contains(discardFish[:i], id) {
				return errors.New("只能归还目标舱位中的鱼群")
			}
			remaining -= 2
			removedSizes = append(removedSizes, 2)
		}
		for i, id := range discardSpice {
			if id < 0 || id >= len(c.Spice) || c.Spice[id].Owner != player || c.Spice[id].At != target || slices.Contains(discardSpice[:i], id) {
				return errors.New("只能归还目标舱位中的己方香料")
			}
			remaining--
			removedSizes = append(removedSizes, 1)
		}
		for _, returnedSize := range removedSizes {
			if remaining+size+returnedSize <= 2 {
				return errors.New("不能为建造额外弃置不需要归还的货物")
			}
		}
	}
	if remaining+size > 2 || c.Units[unit].Kind != "supply" && !slices.Contains(discard, unit) || !catanExplorerCanPay(g, player, catanExplorerUnitCost(unit)) {
		return errors.New("舱位、单位库存或建造资源不足")
	}
	catanExplorerPay(g, player, catanExplorerUnitCost(unit))
	for _, id := range discard {
		c.Units[id] = catanExplorerCargoLocation{"supply", -1}
	}
	for _, id := range discardFish {
		c.Fish[id] = catanExplorerCargoLocation{"supply", -1}
	}
	for _, id := range discardSpice {
		// Preserve the farm claim and its permanent crew/ability. Returning a
		// sack is not delivery and must not allow claiming another from the farm.
		c.Spice[id].At = catanExplorerCargoLocation{"supply", -1}
	}
	c.Units[unit] = target
	return nil
}

// Simultaneous load/unload/swap at one actual harbor. Every piece must start
// in the selected source; final capacity is checked before either side changes.
// Direct ship-to-ship transfer is deliberately not an action.
func (c *catanExplorerCargo) transfer(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, harbor int, load, unload []int) error {
	return c.transferFreight(g, fleet, player, sequence, ship, harbor, load, unload, nil, nil)
}
func (c *catanExplorerCargo) transferFreight(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, harbor int, load, unload, loadFish, unloadFish []int) error {
	return c.transferAllFreight(g, fleet, player, sequence, ship, harbor, load, unload, loadFish, unloadFish, nil, nil)
}
func (c *catanExplorerCargo) transferAllFreight(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, harbor int, load, unload, loadFish, unloadFish, loadSpice, unloadSpice []int) error {
	if err := c.allowed(g, fleet, player, sequence, "movement"); err != nil {
		return err
	}
	if !c.docked(g, fleet, player, ship, harbor) || len(load)+len(unload)+len(loadFish)+len(unloadFish)+len(loadSpice)+len(unloadSpice) == 0 {
		return errors.New("只能在实际停靠的己方港口装卸或交换")
	}
	shipLoc, harborLoc := catanExplorerCargoLocation{"ship", ship}, catanExplorerCargoLocation{"harbor", harbor}
	shipUsed, harborUsed := c.used(shipLoc), c.used(harborLoc)
	seen := map[int]bool{}
	for group, ids := range [][]int{load, unload} {
		source := harborLoc
		if group == 1 {
			source = shipLoc
		}
		for _, id := range ids {
			if id < 0 || id >= len(c.Units) || id/11 != player || seen[id] || c.Units[id] != source {
				return errors.New("装卸单位不在来源舱位、重复或属于其他玩家")
			}
			seen[id] = true
			delta := catanExplorerUnitSize(id)
			if group == 1 {
				delta = -delta
			}
			shipUsed, harborUsed = shipUsed+delta, harborUsed-delta
		}
	}
	seenFish := map[int]bool{}
	for group, ids := range [][]int{loadFish, unloadFish} {
		source, delta := harborLoc, 2
		if group == 1 {
			source, delta = shipLoc, -2
		}
		for _, id := range ids {
			if id < 0 || id >= len(c.Fish) || seenFish[id] || c.Fish[id] != source {
				return errors.New("鱼群不在来源舱位或重复选择")
			}
			seenFish[id] = true
			shipUsed, harborUsed = shipUsed+delta, harborUsed-delta
		}
	}
	seenSpice := map[int]bool{}
	for group, ids := range [][]int{loadSpice, unloadSpice} {
		source, delta := harborLoc, 1
		if group == 1 {
			source, delta = shipLoc, -1
		}
		for _, id := range ids {
			if id < 0 || id >= len(c.Spice) || seenSpice[id] || c.Spice[id].At != source || c.Spice[id].Owner != player {
				return errors.New("香料不在来源舱位、重复或属于其他玩家")
			}
			seenSpice[id] = true
			shipUsed, harborUsed = shipUsed+delta, harborUsed-delta
		}
	}
	if shipUsed > 2 || harborUsed > 2 {
		return errors.New("交换后的船舱或港口超出容量")
	}
	for _, id := range load {
		c.Units[id] = shipLoc
	}
	for _, id := range unload {
		c.Units[id] = harborLoc
	}
	for _, id := range loadFish {
		c.Fish[id] = shipLoc
	}
	for _, id := range unloadFish {
		c.Fish[id] = harborLoc
	}
	for _, id := range loadSpice {
		c.Spice[id].At = shipLoc
	}
	for _, id := range unloadSpice {
		c.Spice[id].At = harborLoc
	}
	return nil
}
func (c *catanExplorerCargo) settle(g *Catan, fleet *catanExplorerSailing, player int, sequence uint64, ship, vertex int) error {
	if err := c.allowed(g, fleet, player, sequence, "movement"); err != nil {
		return err
	}
	loc := catanExplorerCargoLocation{"ship", ship}
	if !c.holder(g, fleet, player, loc) || !c.landVertex(g, player, vertex) || g.Vertices[vertex].Level != 0 || g.knightAt(vertex) != nil {
		return errors.New("移民只能在己方船端相邻的已探索陆地定居")
	}
	e := g.Edges[fleet.Positions[ship]]
	if e.A != vertex && e.B != vertex {
		return errors.New("定居点必须是移民船的一个端点")
	}
	units := c.contents(loc)
	if len(units) != 1 || units[0]%11 >= 2 {
		return errors.New("定居需要船上有一枚移民")
	}
	for _, edge := range g.Edges {
		if edge.A == vertex && g.Vertices[edge.B].Level != 0 || edge.B == vertex && g.Vertices[edge.A].Level != 0 {
			return errors.New("移民定居必须遵守建筑距离规则")
		}
	}
	if g.settlementPiecesLeft(player) <= 0 {
		return errors.New("村庄组件已全部使用")
	}
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = player, 1
	g.Players[player].Score++
	c.Units[units[0]] = catanExplorerCargoLocation{"supply", -1}
	fleet.Positions[ship] = -1
	fleet.Turn.Ships[ship] = catanExplorerShipMove{Closed: true}
	if fleet.Turn.Current == ship {
		fleet.Turn.Current = -1
	}
	return nil
}

// Exact printed Land Ho! setup, still a private component constructor. Main
// production/Apply, rewards, turn clocks and victory are not installed here.
func newCatanExplorerLandHo(players int) (*Catan, *catanExplorerBoard, *catanExplorerSailing, *catanExplorerCargo, error) {
	g, board, err := newCatanExplorerBoard(players, "land-ho", "fixed")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	fleet, _ := newCatanExplorerSailing(players)
	cargo, err := newCatanExplorerCargo(g, fleet, "land-ho")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	g.Bank = []int{19, 19, 19, 19, 19}
	for p := range g.Players {
		g.Players[p].Resources = slices.Clone(board.Opening[p].Resources)
		g.Players[p].Dev, g.Players[p].NewDev = make([]int, 5), make([]int, 5)
		g.Players[p].Score = 3
		for r, n := range g.Players[p].Resources {
			g.Bank[r] -= n
		}
	}
	for color, setup := range board.Opening {
		owner := color
		if color >= players {
			if players != 2 {
				continue
			}
			owner = -color // White=-2, orange=-3; static obstacles, not traders.
		}
		g.Vertices[setup.Settlement].Owner, g.Vertices[setup.Settlement].Level = owner, 1
		g.Vertices[setup.Harbor].Owner, g.Vertices[setup.Harbor].Level, g.Vertices[setup.Harbor].Harbor = owner, 2, true
		g.Edges[setup.Road].Owner = owner
		if owner >= 0 {
			fleet.Positions[owner*3] = setup.Ship
			cargo.Units[owner*11] = catanExplorerCargoLocation{"ship", owner * 3}
		}
	}
	return g, board, fleet, cargo, cargo.validate(g, fleet)
}
