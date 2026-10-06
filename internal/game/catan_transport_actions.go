package game

import "errors"

func (t *catanTransport) placeWagon(g *Catan, player, city int) error {
	if err := t.validate(g); err != nil {
		return err
	}
	if t.TurnSerial != 0 || player < 0 || player >= len(t.Wagons) || g.Players[player].Eliminated || t.Wagons[player].Position != -1 || city < 0 || city >= len(g.Vertices) || g.Vertices[city].Owner != player || g.Vertices[city].Level != 2 || t.Map.siteAt(city) >= 0 {
		return errors.New("马车必须与自己的起始城市一起放置且只能放置一次")
	}
	t.Wagons[player].Position = city
	return nil
}

// Called by the eventual production/paired-turn controller after production
// and mandatory responses. Public Actions must not let a client choose a turn.
func (t *catanTransport) beginTurn(g *Catan, player int, serial uint64) error {
	if err := t.validate(g); err != nil {
		return err
	}
	if player < 0 || player >= len(t.Wagons) || g.Players[player].Eliminated || serial == 0 || serial != t.TurnSerial+1 {
		return errors.New("运输行动回合序号或玩家无效")
	}
	for _, w := range t.Wagons {
		if w.Position == -1 {
			return errors.New("请先放置全部起始马车")
		}
	}
	if t.TurnSerial > 0 && (t.Travel == nil || !t.Travel.Ended || t.Travel.Arrived >= 0 && !t.ArrivalResolved) {
		return errors.New("上一回合移动或装卸尚未结束")
	}
	t.Active, t.TurnSerial, t.Bought = player, serial, 0
	t.Travel, t.ArrivalResolved = nil, false
	return nil
}
func (t catanTransport) actionAllowed(g *Catan, player int) error {
	if err := t.validate(g); err != nil {
		return err
	}
	if player < 0 || player != t.Active || g.Players[player].Eliminated || t.Travel != nil {
		return errors.New("只有当前玩家能在行动阶段操作")
	}
	return nil
}
func (t *catanTransport) upgrade(g *Catan, player int) error {
	if err := t.actionAllowed(g, player); err != nil {
		return err
	}
	cost := catanTransportUpgradeCost(t.Wagons[player].Level)
	if cost == nil || !catanHas(g.Players[player].Resources, cost) {
		return errors.New("马车已满级或升级资源不足")
	}
	catanMove(g.Players[player].Resources, g.Bank, cost)
	t.Wagons[player].Level++
	return nil
}
func (t *catanTransport) buyResource(g *Catan, player, resource int) error {
	if err := t.actionAllowed(g, player); err != nil {
		return err
	}
	if resource < 0 || resource >= 5 || t.Bought >= 2 || t.Gold[player] < 2 || g.Bank[resource] == 0 {
		return errors.New("每回合至多两次用2金币购买银行现有资源")
	}
	t.Gold[player] -= 2
	t.GoldBank += 2
	g.Bank[resource]--
	g.Players[player].Resources[resource]++
	t.Bought++
	return nil
}
func (t *catanTransport) sellResource(g *Catan, player, resource int) error {
	if err := t.actionAllowed(g, player); err != nil {
		return err
	}
	if resource < 0 || resource >= 5 {
		return errors.New("请选择一种普通资源")
	}
	rate := g.rates(player)[resource]
	if g.Players[player].Resources[resource] < rate || t.GoldBank < 1 {
		return errors.New("资源不足或金币银行没有库存")
	}
	g.Players[player].Resources[resource] -= rate
	g.Bank[resource] += rate
	t.Gold[player]++
	t.GoldBank--
	return nil
}
func (t *catanTransport) beginTravel(g *Catan, player int) error {
	if err := t.actionAllowed(g, player); err != nil {
		return err
	}
	w := t.Wagons[player]
	q, err := newCatanTransportTravel(g, t.Map, player, w.Position, w.Level)
	if err != nil {
		return err
	}
	t.Travel, t.ArrivalResolved = q, false
	t.Sequence++
	return nil
}
func (t catanTransport) travelAllowed(g *Catan, player int, sequence uint64) error {
	if err := t.validate(g); err != nil {
		return err
	}
	if player < 0 || player != t.Active || g.Players[player].Eliminated || t.Travel == nil || sequence == 0 || sequence != t.Sequence {
		return errors.New("运输移动已过期或不是当前玩家")
	}
	return nil
}
func (t *catanTransport) move(g *Catan, player int, sequence uint64, edge int) (catanTransportStep, error) {
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return catanTransportStep{}, err
	}
	step, err := t.Travel.move(g, t.Map, t.Barbarians, t.Gold, edge)
	if err == nil {
		t.Wagons[player].Position = t.Travel.Position
	}
	return step, err
}
func (t *catanTransport) wheat(g *Catan, player int, sequence uint64) error {
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return err
	}
	return t.Travel.wheat(g, t.Map, t.Barbarians, t.Gold)
}
func (t *catanTransport) stop(g *Catan, player int, sequence uint64) error {
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return err
	}
	return t.Travel.stop(g, t.Map, t.Barbarians, t.Gold)
}
func (t *catanTransport) driveOff(g *Catan, player int, sequence uint64, piece, die int) (bool, error) {
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return false, err
	}
	return t.Travel.driveOff(g, t.Map, t.Barbarians, t.Gold, piece, die)
}
func (t *catanTransport) relocate(g *Catan, player int, sequence uint64, edge int) error {
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return err
	}
	return t.Travel.relocate(g, t.Map, &t.Barbarians, t.Gold, edge)
}
func (t catanTransport) canDeliver() bool {
	if t.Travel == nil || !t.Travel.Ended || t.Travel.Arrived < 0 || t.ArrivalResolved {
		return false
	}
	token, ok := t.token(t.Wagons[t.Active].Cargo)
	return ok && t.Map.accepts(t.Travel.Arrived, token.Cargo)
}

// The player may decline delivery, keeping the current token. An empty wagon
// takes the top token of the corresponding shared source stack; a full wagon
// never swaps/discards its cargo. Resolve exactly once per genuine arrival.
func (t *catanTransport) resolveArrival(g *Catan, player int, sequence uint64, deliver bool) (catanTransportArrivalResult, error) {
	return t.resolveArrivalWithStop(g, player, sequence, deliver, false)
}

func (t *catanTransport) resolveArrivalWithStop(g *Catan, player int, sequence uint64, deliver, stopAfterDelivery bool) (catanTransportArrivalResult, error) {
	result := catanTransportArrivalResult{}
	if err := t.travelAllowed(g, player, sequence); err != nil {
		return result, err
	}
	q := t.Travel
	if !q.Ended || q.Arrived < 0 || t.ArrivalResolved {
		return result, errors.New("没有待处理的货物到达")
	}
	if deliver && !t.canDeliver() {
		return result, errors.New("此地点不接受马车当前货物")
	}
	w := &t.Wagons[player]
	payment := 0
	if deliver {
		payment = w.Level + 1
		if t.GoldBank < payment {
			// Explicit acceptance gate, NOT an invented official shortage rule.
			// Keep the full arrival pending and do not remove cargo or add VP.
			return result, errors.New("运输交付金币供应不足的处理尚待核对")
		}
	}
	result = catanTransportArrivalResult{Sequence: sequence, Player: player, Site: q.Arrived, Gold: payment}
	if deliver {
		result.Delivered = w.Cargo
		w.Delivered = append(w.Delivered, w.Cargo)
		w.Cargo = 0
		t.Gold[player] += payment
		t.GoldBank -= payment
	}
	stack := catanTransportOriginIndex(t.Map.Sites[q.Arrived].Kind)
	if !(deliver && stopAfterDelivery) && w.Cargo == 0 && len(t.Stacks[stack]) > 0 {
		w.Cargo = t.Stacks[stack][0]
		t.Stacks[stack] = t.Stacks[stack][1:]
		result.Loaded = w.Cargo
	}
	t.ArrivalResolved = true
	t.LastArrival = &result
	return result, nil
}
