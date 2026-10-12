package monopoly

import (
	_ "embed"
	"encoding/json"
	"errors"
	"time"
)

//go:embed cards.json
var cardsJSON []byte

type Card struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	Effect      string `json:"effect"`
	Amount      int    `json:"amount,omitempty"`
	Destination int    `json:"destination,omitempty"`
	HouseFee    int    `json:"houseFee,omitempty"`
	HotelFee    int    `json:"hotelFee,omitempty"`
}
type CardEvent struct {
	Card
	Deck     string `json:"deck"`
	Player   int    `json:"player"`
	Sequence int    `json:"sequence"`
}

var cards = func() map[string][]Card {
	var values map[string][]Card
	if err := json.Unmarshal(cardsJSON, &values); err != nil {
		panic(err)
	}
	for _, deck := range []string{"chance", "chest"} {
		if len(values[deck]) != 16 {
			panic("invalid card catalogue")
		}
		for i, c := range values[deck] {
			if c.ID != i {
				panic("invalid card ID")
			}
		}
	}
	return values
}()

func (s *State) drawCard(p int, deck string, now time.Time) error {
	d := 0
	if deck == "chest" {
		d = 1
	}
	if len(s.Decks[d]) == 0 {
		return errors.New("事件牌堆为空")
	}
	id := s.Decks[d][0]
	s.Decks[d] = s.Decks[d][1:]
	card := cards[deck][id]
	// Return non-held cards to the bottom immediately in the private snapshot.
	// No further turn can draw from that deck until pending obligations finish.
	if id != 0 {
		s.Decks[d] = append(s.Decks[d], id)
	}
	s.CardSerial++
	s.LastCard = &CardEvent{Card: card, Deck: deck, Player: p, Sequence: s.CardSerial}
	s.note("玩家 %d 抽到「%s」：%s", p+1, card.Title, card.Text)
	switch card.Effect {
	case "jail_card":
		s.Players[p].JailCards = append(s.Players[p].JailCards, deck)
	case "credit":
		s.pay(-1, p, card.Amount)
	case "pay":
		s.Effects = append([]Effect{{Kind: "payment", Actor: p, Target: -1, Amount: card.Amount, Reason: card.Title}}, s.Effects...)
	case "repairs":
		amount := 0
		for _, prop := range s.Properties {
			if prop.Owner != p {
				continue
			}
			if prop.Buildings == 5 {
				amount += card.HotelFee
			} else {
				amount += prop.Buildings * card.HouseFee
			}
		}
		if amount > 0 {
			s.Effects = append([]Effect{{Kind: "payment", Actor: p, Target: -1, Amount: amount, Reason: card.Title}}, s.Effects...)
		}
	case "pay_each", "collect_each":
		queue := []Effect{}
		for step := 1; step < len(s.Players); step++ {
			other := (p + step) % len(s.Players)
			if s.Players[other].Bankrupt {
				continue
			}
			from, to := p, other
			if card.Effect == "collect_each" {
				from, to = other, p
			}
			queue = append(queue, Effect{Kind: "payment", Actor: from, Target: to, Amount: card.Amount, Reason: card.Title})
		}
		s.Effects = append(queue, s.Effects...)
	case "advance":
		return s.move(p, (card.Destination-s.Players[p].Position+39)%40+1, now)
	case "back":
		return s.move(p, -card.Amount, now)
	case "nearest_station", "nearest_utility":
		targets := []int{5, 15, 25, 35}
		stationFactor, utilityFactor := 2, 0
		if card.Effect == "nearest_utility" {
			targets = []int{12, 28}
			stationFactor, utilityFactor = 1, 10
		}
		steps := 40
		for _, target := range targets {
			steps = min(steps, (target-s.Players[p].Position+39)%40+1)
		}
		return s.moveWithRent(p, steps, stationFactor, utilityFactor, now)
	case "jail":
		s.jail(p)
	default:
		return errors.New("未知事件卡效果")
	}
	return s.drain(now)
}

func (s *State) validateDecks() error {
	var seen [2][16]int
	for d, deck := range s.Decks {
		for _, id := range deck {
			if id < 0 || id >= 16 {
				return errors.New("无效事件卡编号")
			}
			seen[d][id]++
		}
	}
	for _, p := range s.Players {
		for _, card := range p.JailCards {
			d := 0
			if card == "chest" {
				d = 1
			} else if card != "chance" {
				return errors.New("无效免狱卡")
			}
			seen[d][0]++
		}
	}
	for _, deck := range seen {
		for _, count := range deck {
			if count != 1 {
				return errors.New("事件卡库存不平衡")
			}
		}
	}
	return nil
}
