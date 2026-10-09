package game

import "errors"

// This adapter keeps the ordinary three-barbarian transport save format intact.
// Attempts refer to the shared invasion's stable IDs, never a filtered list of
// blockers (capturing or relocating a piece must not reset another attempt).
type catanAttackTransportTravel struct {
	Travel    catanTransportTravel `json:"travel"`
	Attempted []bool               `json:"attempted"`
}

func (p catanAttackTransportPieces) blockingEdges() []int {
	edges := make([]int, len(p.Barbarians))
	for i, piece := range p.Barbarians {
		edges[i] = piece.Edge
	}
	return edges
}

func newCatanAttackTransportTravel(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, player, position, level int) (*catanAttackTransportTravel, error) {
	if b == nil || p == nil {
		return nil, errors.New("蛮族运输棋盘或棋子缺失")
	}
	if err := b.validate(g); err != nil {
		return nil, err
	}
	if err := p.validate(g, b); err != nil {
		return nil, err
	}
	q, err := newCatanTransportTravel(g, &b.Transport, player, position, level)
	if err != nil {
		return nil, err
	}
	return &catanAttackTransportTravel{Travel: *q, Attempted: make([]bool, len(p.Barbarians))}, nil
}
func (q catanAttackTransportTravel) validate(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int) error {
	if b == nil || p == nil {
		return errors.New("蛮族运输棋盘或棋子缺失")
	}
	if err := p.validate(g, b); err != nil {
		return err
	}
	if q.Travel.Attempted != [3]bool{} {
		return errors.New("组合运输不能使用独立剧本的驱赶记录")
	}
	return q.Travel.validateRoutes(g, &b.Transport, p.blockingEdges(), gold, q.Attempted, true)
}
func (q catanAttackTransportTravel) quote(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int, edge int) (catanTransportStep, error) {
	if err := q.validate(g, b, p, gold); err != nil {
		return catanTransportStep{}, err
	}
	return q.Travel.quoteRoutes(g, &b.Transport, p.blockingEdges(), gold, edge, q.Attempted, true)
}
func (q *catanAttackTransportTravel) move(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int, edge int) (catanTransportStep, error) {
	step, err := q.quote(g, b, p, gold, edge)
	if err != nil {
		return step, err
	}
	q.Travel.applyStep(&b.Transport, gold, step)
	return step, nil
}
func (q *catanAttackTransportTravel) driveOff(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int, id, die int) (bool, error) {
	if err := q.validate(g, b, p, gold); err != nil {
		return false, err
	}
	t := &q.Travel
	if t.Ended || t.Pending != -1 || t.Level == 0 || id < 0 || id >= len(p.Barbarians) || q.Attempted[id] || die < 1 || die > 6 {
		return false, errors.New("当前不能尝试驱赶此蛮族")
	}
	piece := p.Barbarians[id]
	if piece.Edge < 0 {
		return false, errors.New("地块中央、供应区或已俘获的蛮族不能被马车驱赶")
	}
	e := g.Edges[piece.Edge]
	if e.A != t.Position && e.B != t.Position {
		return false, errors.New("只能驱赶马车旁边的蛮族")
	}
	q.Attempted[id] = true
	success := die >= 7-t.Level
	if success {
		t.Pending = id
	}
	return success, nil
}
func (q *catanAttackTransportTravel) relocate(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int, tile, edge int) error {
	if err := q.validate(g, b, p, gold); err != nil {
		return err
	}
	if q.Travel.Pending < 0 {
		return errors.New("没有等待安置的蛮族")
	}
	if err := p.relocate(g, b, q.Travel.Pending, tile, edge); err != nil {
		return err
	}
	q.Travel.Pending = -1
	return nil
}
func (q *catanAttackTransportTravel) stop(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int) error {
	if err := q.validate(g, b, p, gold); err != nil {
		return err
	}
	if q.Travel.Ended || q.Travel.Pending != -1 {
		return errors.New("请先完成移动蛮族的选择")
	}
	q.Travel.Ended, q.Travel.Points = true, 0
	return nil
}
func (q *catanAttackTransportTravel) wheat(g *Catan, b *catanAttackTransportBoard, p *catanAttackTransportPieces, gold []int) error {
	if err := q.validate(g, b, p, gold); err != nil {
		return err
	}
	return q.Travel.payWheat(g)
}
