package game

import (
	"errors"
	"slices"
	"strings"
)

// Bluffs are private resumable continuations. The card stays in hand during
// questioning, then moves to the public table before rewards/HP loss resolve.
// A stack supports a rescue bluff inside the consequences of another bluff.
type SGBluff struct {
	ID         int       `json:"id"`
	Player     int       `json:"player"`
	Card       int       `json:"card"`
	Declared   string    `json:"declared"`
	Action     Action    `json:"action"`
	Prompt     *SGPrompt `json:"prompt,omitempty"`
	Questioned []int     `json:"questioned,omitempty"`
	Success    bool      `json:"success"`
	Resolved   bool      `json:"resolved"`
}
type SGVirtual struct {
	Player int
	Card   int
	Kind   string
	Paid   bool
}

func (s *State) sgGuhuoKinds() []string {
	out := []string{}
	for _, c := range s.Sanguosha.cardCatalog() {
		if SGCardTypes[c.Kind].Slot == "" && !sgIsDelayed(c.Kind) && !slices.Contains(out, c.Kind) {
			out = append(out, c.Kind)
		}
	}
	slices.Sort(out)
	return out
}

func (s *State) sgStartGuhuo(i int, a Action) error {
	g := s.Sanguosha
	if !s.sgHas(i, "guhuo") || s.sgValidateCards(i, a.Cards, 1, true) != nil {
		return errors.New("蛊惑需要一张自己的手牌")
	}
	if !slices.Contains(s.sgGuhuoKinds(), a.Choice) {
		return errors.New("蛊惑只能声明本局的基本牌或非延时锦囊")
	}
	if a.Choice == "iron_chain" && len(a.Targets) == 0 {
		return errors.New("蛊惑的铁索不能重铸，请选择目标")
	}
	if q := g.Pending; q != nil && !slices.Contains([]string{"card", "support", "peach", "nullification", "weapon_after_jink", "luanwu", "tiaoxin"}, q.Kind) {
		return errors.New("此响应不能发动蛊惑")
	}
	trial := clone(*s)
	trial.Sanguosha.Virtual = &SGVirtual{Player: i, Card: a.Cards[0], Kind: a.Choice}
	if err := trial.sgApply(i, a); err != nil {
		return err
	}
	g.Sequence++
	b := SGBluff{ID: g.Sequence, Player: i, Card: a.Cards[0], Declared: a.Choice, Action: clone(a), Prompt: clone(g.Pending)}
	g.Bluffs = append(g.Bluffs, b)
	g.Pending = nil
	s.sgLog("%s 发动蛊惑，声明「%s」：%s", s.sgName(i), SGCardTypes[a.Choice].Name, s.sgBluffContext(b))
	s.sgPush(SGEvent{Type: "guhuo_question", Actor: i, Aux: b.ID})
	return nil
}

// Only public participants and declared effects belong in this description.
// Never serialize the saved action/prompt: they include the concealed card.
func (s *State) sgBluffContext(b SGBluff) string {
	if q := b.Prompt; q != nil {
		e := q.Event
		switch q.Kind {
		case "nullification":
			prefix := "抵消"
			if e.Flag {
				prefix = "恢复"
			}
			return prefix + "「" + SGCardTypes[e.Kind].Name + "」对" + s.sgName(e.Target) + "的效果"
		case "peach":
			return "救助" + s.sgName(e.Target)
		case "tiaoxin":
			return "挑衅：对" + s.sgName(e.Actor) + "使用杀"
		case "luanwu":
			names := []string{}
			for _, t := range b.Action.Targets {
				names = append(names, s.sgName(t))
			}
			return "乱武：对" + strings.Join(names, "、") + "使用杀"
		case "support":
			return "响应" + s.sgName(e.Actor) + "的「" + SGSkills[e.Kind].Name + "」"
		default:
			return "响应" + s.sgName(e.Actor) + "的「" + SGCardTypes[e.Kind].Name + "」"
		}
	}
	targets := b.Action.Targets
	switch b.Declared {
	case "peach", "analeptic", "ex_nihilo":
		targets = []int{b.Player}
	case "amazing_grace", "god_salvation":
		targets = s.sgOrder(b.Player)
	case "savage_assault", "archery_attack":
		targets = sgRemove(s.sgOrder(b.Player), b.Player)
	}
	names := []string{}
	for _, target := range targets {
		names = append(names, s.sgName(target))
	}
	if b.Declared == "collateral" && len(names) == 2 {
		return "令" + names[0] + "对" + names[1] + "使用杀"
	}
	return "目标：" + strings.Join(names, "、")
}

func (s *State) sgBluff(id int) (int, *SGBluff) {
	for i := range s.Sanguosha.Bluffs {
		if s.Sanguosha.Bluffs[i].ID == id {
			return i, &s.Sanguosha.Bluffs[i]
		}
	}
	return -1, nil
}

func (s *State) sgGuhuoEvent(e SGEvent) bool {
	if e.Type != "guhuo_question" && e.Type != "guhuo_finish" {
		return false
	}
	g := s.Sanguosha
	index, b := s.sgBluff(e.Aux)
	if b == nil {
		return true
	}
	if e.Type == "guhuo_finish" {
		s.sgFinishGuhuo(index, *b)
		return true
	}
	for e.Step < len(g.Players) {
		who := (s.Turn + e.Step) % len(g.Players)
		e.Step++
		if who == b.Player || !s.sgAlive(who) || g.Players[who].HP <= 0 {
			continue
		}
		e.Kind = b.Declared
		s.sgAsk(who, "guhuo_question", s.sgName(b.Player)+" 蛊惑声明「"+SGCardTypes[b.Declared].Name+"」（"+s.sgBluffContext(*b)+"）：是否质疑？", e)
		g.Pending.Choices = []string{"challenge", "pass"}
		return true
	}
	c := s.sgCardFor(b.Player, b.Card)
	real := c.Kind == b.Declared || b.Declared == "slash" && sgIsSlash(c.Kind)
	b.Success = len(b.Questioned) == 0 || real && c.Suit == 1
	b.Resolved = true
	s.sgLog("蛊惑揭示：%s %d「%s」；声明%s", []string{"♠", "♥", "♣", "♦"}[c.Suit], c.Rank, SGCardTypes[c.Kind].Name, map[bool]string{true: "真实", false: "虚假"}[real])
	es := []SGEvent{}
	for _, who := range b.Questioned {
		if real {
			es = append(es, SGEvent{Type: "lose_hp", Target: who, Amount: 1})
		} else {
			es = append(es, SGEvent{Type: "draw", Actor: who, Amount: 1})
		}
	}
	es = append(es, SGEvent{Type: "guhuo_finish", Aux: b.ID})
	s.sgPush(es...)
	s.sgPay(b.Player, []int{b.Card})
	return true
}

func (s *State) sgGuhuoRespond(i int, a Action, q SGPrompt) (bool, error) {
	if q.Kind != "guhuo_question" {
		return false, nil
	}
	_, b := s.sgBluff(q.Event.Aux)
	if b == nil {
		return true, errors.New("蛊惑已结束")
	}
	if a.Choice != "pass" && a.Choice != "challenge" {
		return true, errors.New("请选择质疑或不质疑")
	}
	if a.Choice == "challenge" {
		if s.Sanguosha.Players[i].HP <= 0 {
			return true, errors.New("体力不大于0时不能质疑")
		}
		b.Questioned = append(b.Questioned, i)
	}
	s.sgLog("%s 对蛊惑选择%s", s.sgName(i), map[bool]string{true: "质疑", false: "不质疑"}[a.Choice == "challenge"])
	s.sgPush(q.Event)
	return true, nil
}

func (s *State) sgFinishGuhuo(index int, b SGBluff) {
	g := s.Sanguosha
	g.Bluffs = append(g.Bluffs[:index], g.Bluffs[index+1:]...)
	if b.Success && s.sgAlive(b.Player) {
		trial := clone(*s)
		tg := trial.Sanguosha
		tg.Pending = clone(b.Prompt)
		if b.Prompt == nil {
			trial.Phase = "sg_play"
		}
		kind := b.Declared
		if kind == "slash" && sgIsSlash(sgCard(b.Card).Kind) {
			kind = sgCard(b.Card).Kind
		}
		tg.Virtual = &SGVirtual{Player: b.Player, Card: b.Card, Kind: kind, Paid: true}
		if err := trial.sgApply(b.Player, b.Action); err == nil {
			tg.Virtual = nil
			// Keep the pointer sgRun is draining; replacing it would strand the queue.
			*g = *tg
			trial.Sanguosha = g
			*s = trial
			return
		}
		s.sgLog("蛊惑声明的牌已无合法结算条件")
	} else {
		s.sgLog("%s 的蛊惑未生效", s.sgName(b.Player))
	}
	s.sgFinishCards([]int{b.Card})
	if b.Prompt != nil {
		q := clone(*b.Prompt)
		// Failed bluff responses can be attempted again with another card, or
		// passed. Use a new token so the pre-questioning response cannot replay.
		if !s.sgAlive(b.Player) {
			g.Pending = &q
			_ = s.sgRespond(b.Player, Action{Choice: "pass"})
		} else {
			s.sgAsk(q.Player, q.Kind, q.Message, q.Event)
			g.Pending.Cards = q.Cards
			g.Pending.Choices = q.Choices
			g.Pending.Targets = q.Targets
		}
	}
}
