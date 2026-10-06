package game

import (
	"errors"
	"slices"
)

// Private movement kernel owned by the end-turn/commodity controller.
// It is not an alternative public action path.
type catanTransportTravel struct {
	NeutralTolls int     `json:"neutralTolls,omitempty"` // Aggregate for this Move Wagon action; bank gets ceil(total/2).
	Player       int     `json:"player"`
	Level        int     `json:"level"` // 0–4, four paid upgrades.
	Position     int     `json:"position"`
	Points       int     `json:"points"`
	WheatUsed    bool    `json:"wheatUsed"`
	Attempted    [3]bool `json:"attempted"` // Stable piece IDs, not edge IDs.
	Pending      int     `json:"pending"`   // Successful drive-off awaiting destination; -1 otherwise.
	Arrived      int     `json:"arrived"`   // Site index reached this movement; -1 otherwise.
	Ended        bool    `json:"ended"`
}

type catanTransportStep struct {
	Edge    int  `json:"edge"`
	To      int  `json:"to"`
	MP      int  `json:"mp"`
	Toll    int  `json:"toll"`
	Pay     int  `json:"pay"`            // Player recipient, or -1 if no player is paid.
	Bank    int  `json:"bank,omitempty"` // Neutral-road share paid to the supply.
	Neutral bool `json:"neutral,omitempty"`
}

func catanTransportMovement(level int) int {
	if level < 0 || level > 4 {
		return 0
	}
	return []int{4, 5, 6, 7, 7}[level]
}
func catanTransportUpgradeCost(level int) []int {
	// Official wagon board p22: wood/wool/ore, then two wood for upgrades 3/4.
	if level < 0 || level >= 4 {
		return nil
	}
	wood := 1
	if level >= 2 {
		wood = 2
	}
	return []int{wood, 0, 1, 0, 1}
}
func catanTransportPlayersValid(g *Catan) bool {
	return g != nil && ((len(g.Players) == 2 && g.Two != nil) || (len(g.Players) >= 3 && len(g.Players) <= 6 && g.Two == nil))
}

func newCatanTransportTravel(g *Catan, m *catanTransportMap, player, position, level int) (*catanTransportTravel, error) {
	if !catanTransportPlayersValid(g) || m == nil || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || position < 0 || position >= len(g.Vertices) || catanTransportMovement(level) == 0 {
		return nil, errors.New("运输移动玩家、位置、等级或双人控制器无效")
	}
	return &catanTransportTravel{Player: player, Level: level, Position: position, Points: catanTransportMovement(level), Pending: -1, Arrived: -1}, nil
}
func (q catanTransportTravel) validate(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int) error {
	if !catanTransportPlayersValid(g) || m == nil || q.Player < 0 || q.Player >= len(g.Players) || g.Players[q.Player].Eliminated || q.Position < 0 || q.Position >= len(g.Vertices) || catanTransportMovement(q.Level) == 0 || len(gold) != len(g.Players) {
		return errors.New("马车移动记录无效")
	}
	maxMP := catanTransportMovement(q.Level)
	if q.WheatUsed {
		maxMP += 2
	}
	if q.NeutralTolls < 0 || q.NeutralTolls+q.Points > maxMP || g.Two == nil && q.NeutralTolls != 0 || q.Points < 0 || q.Points > maxMP || q.Pending < -1 || q.Pending >= len(barbarians) || q.Arrived < -1 || q.Arrived >= len(m.Sites) || q.Ended && q.Points != 0 || q.Arrived >= 0 && (!q.Ended || m.Sites[q.Arrived].Center != q.Position) {
		return errors.New("马车步数、到达或结束状态无效")
	}
	for id, edge := range barbarians {
		if edge < 0 || edge >= len(g.Edges) || slices.Contains(barbarians[:id], edge) {
			return errors.New("蛮族必须位于三个不同的可通行边")
		}
	}
	if q.Pending >= 0 {
		e := g.Edges[barbarians[q.Pending]]
		if q.Ended || q.Level == 0 || !q.Attempted[q.Pending] || e.A != q.Position && e.B != q.Position {
			return errors.New("待移走蛮族的回应无效")
		}
	}
	for _, amount := range gold {
		if amount < 0 || amount > m.Gold {
			return errors.New("玩家金币无效")
		}
	}
	if sum(gold) > m.Gold {
		return errors.New("玩家金币超出供应")
	}
	return nil
}

// Cost quotation does not mutate anything. Buildings and other wagons never
// block traversal. A damaged road counts as roadless for MPs (event variant),
// but remains another player's road for the one-gold toll.
func (q catanTransportTravel) quote(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int, edge int) (catanTransportStep, error) {
	step := catanTransportStep{Edge: edge, To: -1, Pay: -1}
	if err := q.validate(g, m, barbarians, gold); err != nil {
		return step, err
	}
	if q.Ended || q.Pending != -1 || edge < 0 || edge >= len(g.Edges) {
		return step, errors.New("当前不能沿此边移动马车")
	}
	e := g.Edges[edge]
	if e.A == q.Position {
		step.To = e.B
	} else if e.B == q.Position {
		step.To = e.A
	} else {
		return step, errors.New("马车只能移动到相邻交点")
	}
	if e.Owner < -1 && (g.Two == nil || e.Owner < -3) || e.Owner >= len(g.Players) || e.Ship || e.Bridge || e.Warship {
		return step, errors.New("运输道路所有者或种类无效")
	}
	step.MP = 2
	if e.Owner != -1 {
		if !e.Damaged {
			step.MP = 1
		}
		if e.Owner != q.Player {
			step.Toll, step.Pay = 1, e.Owner
			if e.Owner < -1 {
				step.Neutral = true
				step.Pay = -1
				if q.NeutralTolls%2 == 0 {
					step.Bank = 1
				} else {
					step.Pay = 1 - q.Player
				}
			}
		}
	}
	if slices.Contains(barbarians[:], edge) {
		step.MP += 2
	}
	if q.Points < step.MP || gold[q.Player] < step.Toll {
		return step, errors.New("马车移动点数或过路费不足")
	}
	return step, nil
}
func (q *catanTransportTravel) move(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int, edge int) (catanTransportStep, error) {
	step, err := q.quote(g, m, barbarians, gold, edge)
	if err != nil {
		return step, err
	}
	q.Points -= step.MP
	q.Position = step.To
	gold[q.Player] -= step.Toll
	if step.Pay >= 0 {
		gold[step.Pay] += step.Toll - step.Bank
	}
	if step.Neutral {
		q.NeutralTolls++
	}
	if site := m.siteAt(q.Position); site >= 0 {
		q.Arrived, q.Ended, q.Points = site, true, 0
	}
	return step, nil
}
func (q *catanTransportTravel) wheat(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int) error {
	if err := q.validate(g, m, barbarians, gold); err != nil {
		return err
	}
	if q.Ended || q.Pending != -1 || q.WheatUsed || !catanBundle(g.Bank) || !catanBundle(g.Players[q.Player].Resources) || g.Players[q.Player].Resources[3] < 1 {
		return errors.New("本回合只能支付一次粮食增加2移动点")
	}
	stock := 19
	if len(g.Players) > 4 {
		stock = 24
	}
	if g.Bank[3] >= stock {
		return errors.New("粮食银行库存无效")
	}
	g.Players[q.Player].Resources[3]--
	g.Bank[3]++
	q.Points += 2
	q.WheatUsed = true
	return nil
}
func (q *catanTransportTravel) stop(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int) error {
	if err := q.validate(g, m, barbarians, gold); err != nil {
		return err
	}
	if q.Ended || q.Pending != -1 {
		return errors.New("请先完成移动蛮族的选择")
	}
	q.Ended, q.Points = true, 0
	return nil
}

// The supplied die is server-generated. Callers must not expose it as a
// client-controlled Action field. Both a success and a failure consume this
// particular piece's attempt for the turn, regardless of subsequent relocation.
func (q *catanTransportTravel) driveOff(g *Catan, m *catanTransportMap, barbarians [3]int, gold []int, piece, die int) (bool, error) {
	if err := q.validate(g, m, barbarians, gold); err != nil {
		return false, err
	}
	if q.Ended || q.Pending != -1 || q.Level == 0 || piece < 0 || piece >= len(barbarians) || q.Attempted[piece] || die < 1 || die > 6 {
		return false, errors.New("当前不能尝试驱赶此蛮族")
	}
	e := g.Edges[barbarians[piece]]
	if e.A != q.Position && e.B != q.Position {
		return false, errors.New("只能驱赶马车旁边的蛮族")
	}
	q.Attempted[piece] = true
	success := die >= 7-q.Level
	if success {
		q.Pending = piece
	}
	return success, nil
}
func (q *catanTransportTravel) relocate(g *Catan, m *catanTransportMap, barbarians *[3]int, gold []int, edge int) error {
	if barbarians == nil {
		return errors.New("蛮族位置缺失")
	}
	if err := q.validate(g, m, *barbarians, gold); err != nil {
		return err
	}
	if q.Pending < 0 || edge < 0 || edge >= len(g.Edges) || slices.Contains(barbarians[:], edge) {
		return errors.New("请选择没有蛮族的可通行边")
	}
	barbarians[q.Pending] = edge
	q.Pending = -1
	return nil // Drive-off never steals a resource from the road owner.
}
