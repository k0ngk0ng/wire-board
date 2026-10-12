package monopoly

import (
	"errors"
	"slices"
	"time"
)

func (s *State) roll(p int, now time.Time) error {
	if s.Phase != "roll" || p != s.Turn {
		return errors.New("当前不能掷骰")
	}
	s.RollSerial++
	s.Dice = [2]int{s.Random.pick(6) + 1, s.Random.pick(6) + 1}
	s.UtilityDice = [2]int{}
	s.note("玩家 %d 掷出 %d、%d", p+1, s.Dice[0], s.Dice[1])
	double := s.Dice[0] == s.Dice[1]
	steps := s.Dice[0] + s.Dice[1]
	if s.Players[p].Jailed {
		s.NextPhase = "end"
		s.Doubles = 0
		if double {
			s.Players[p].Jailed = false
			s.Players[p].JailTurns = 0
			return s.move(p, steps, now)
		}
		s.Players[p].JailTurns++
		if s.Players[p].JailTurns < 3 {
			s.enter("end", now)
			return nil
		}
		s.Effects = append([]Effect{
			{Kind: "payment", Actor: p, Target: -1, Amount: 50, Reason: "第三次出狱尝试失败"},
			{Kind: "release_move", Actor: p, Amount: steps},
		}, s.Effects...)
		return s.drain(now)
	}
	if double {
		s.Doubles++
	} else {
		s.Doubles = 0
	}
	if s.Doubles == 3 {
		s.jail(p)
		return s.drain(now)
	}
	s.NextPhase = "end"
	if double {
		s.NextPhase = "roll"
	}
	return s.move(p, steps, now)
}

func (s *State) jail(p int) {
	s.Players[p].Position = 10
	s.Players[p].Jailed = true
	s.Players[p].JailTurns = 0
	s.Doubles = 0
	s.NextPhase = "end"
	s.note("玩家 %d 进入监狱", p+1)
}

func (s *State) leaveJail(p int, card string, useCard bool, now time.Time) error {
	if s.Phase != "roll" || p != s.Turn || !s.Players[p].Jailed {
		return errors.New("当前不能选择出狱方式")
	}
	if useCard {
		index := slices.Index(s.Players[p].JailCards, card)
		if index < 0 || (card != "chance" && card != "chest") {
			return errors.New("未持有该免狱卡")
		}
		s.Players[p].JailCards = append(s.Players[p].JailCards[:index], s.Players[p].JailCards[index+1:]...)
		deck := 0
		if card == "chest" {
			deck = 1
		}
		s.Decks[deck] = append(s.Decks[deck], 0)
		s.note("玩家 %d 使用免狱卡", p+1)
	} else {
		if s.available(p) < 50 {
			return errors.New("出狱资金不足，可以先筹资或尝试掷双骰")
		}
		s.pay(p, -1, 50)
		s.note("玩家 %d 支付 50 出狱", p+1)
	}
	s.Players[p].Jailed = false
	s.Players[p].JailTurns = 0
	s.Doubles = 0
	s.enter("roll", now)
	return nil
}

func (s *State) move(p, steps int, now time.Time) error {
	return s.moveWithRent(p, steps, 1, 0, now)
}

func (s *State) moveWithRent(p, steps, stationFactor, utilityFactor int, now time.Time) error {
	position := s.Players[p].Position + steps
	if position >= 40 {
		s.pay(-1, p, 200)
		s.note("玩家 %d 经过起点，领取 200", p+1)
	}
	s.Players[p].Position = (position%40 + 40) % 40
	return s.landWithRent(p, stationFactor, utilityFactor, now)
}

func (s *State) rent(id, utilityFactor int) int {
	prop := s.Properties[id]
	space := board[id]
	if prop.Owner < 0 || prop.Mortgaged {
		return 0
	}
	count, total := 0, 0
	for i, p := range s.Properties {
		if board[i].Group == space.Group && board[i].Price > 0 {
			total++
			if p.Owner == prop.Owner {
				count++
			}
		}
	}
	switch space.Kind {
	case "station":
		return 25 << (count - 1)
	case "utility":
		s.UtilityDice = [2]int{s.Random.pick(6) + 1, s.Random.pick(6) + 1}
		multiplier := 4
		if count == 2 {
			multiplier = 10
		}
		if utilityFactor > 0 {
			multiplier = utilityFactor
		}
		s.note("公用事业租金骰为 %d、%d，倍率 %d", s.UtilityDice[0], s.UtilityDice[1], multiplier)
		return (s.UtilityDice[0] + s.UtilityDice[1]) * multiplier
	default:
		rent := space.Rents[prop.Buildings]
		if prop.Buildings == 0 && count == total {
			rent *= 2
		}
		return rent
	}
}

func (s *State) landWithRent(p, stationFactor, utilityFactor int, now time.Time) error {
	id := s.Players[p].Position
	space := board[id]
	s.note("玩家 %d 到达%s", p+1, space.Name)
	if space.Price > 0 {
		owner := s.Properties[id].Owner
		if owner < 0 {
			s.enter("purchase", now)
			return nil
		}
		if owner != p {
			if rent := s.rent(id, utilityFactor); rent > 0 {
				if space.Kind == "station" {
					rent *= stationFactor
				}
				s.Effects = append([]Effect{{Kind: "payment", Actor: p, Target: owner, Amount: rent, Reason: space.Name + "租金"}}, s.Effects...)
			}
		}
	} else {
		switch space.Kind {
		case "tax":
			s.Effects = append([]Effect{{Kind: "payment", Actor: p, Target: -1, Amount: space.Tax, Reason: space.Name}}, s.Effects...)
		case "jail":
			s.jail(p)
		case "chance", "chest":
			return s.drawCard(p, space.Kind, now)
		}
	}
	return s.drain(now)
}
