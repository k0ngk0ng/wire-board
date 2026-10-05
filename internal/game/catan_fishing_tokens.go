package game

import (
	"encoding/json"
	"errors"
	"slices"
)

const catanFishBoot = 29

// Internal token economy for Fishing on Catan (2025 T&B pp.9–10, extension
// p.5). No room constructor enables it until the map/production/actions are
// integrated. Token IDs are server identities, never a public draw order.
type catanFishingTokens struct {
	DrawPile  []int            `json:"drawPile"`
	Discard   []int            `json:"discard"`
	Hands     [][]int          `json:"hands"`
	BootOwner int              `json:"bootOwner"`
	Pending   []catanFishClaim `json:"pending"`
}

type catanFishClaim struct {
	Player    int `json:"player"`
	Remaining int `json:"remaining"`
}

type catanFishTokenFace struct {
	ID   int `json:"id"`
	Fish int `json:"fish"`
}

type catanFishPlayerView struct {
	Count  int                  `json:"count"`
	Tokens []catanFishTokenFace `json:"tokens,omitempty"`
}

type catanFishTokensView struct {
	Remaining int                   `json:"remaining"`
	Discard   []catanFishTokenFace  `json:"discard"`
	Players   []catanFishPlayerView `json:"players"`
	BootOwner int                   `json:"bootOwner"`
	Responder *int                  `json:"responder,omitempty"`
}

func catanFishValue(id int) int {
	switch {
	case id >= 0 && id < 11:
		return 1
	case id < 21 && id >= 11:
		return 2
	case id < 29 && id >= 21:
		return 3
	case id >= 30 && id < 34:
		return 1
	case id >= 34 && id < 39:
		return 2
	case id >= 39 && id < 44:
		return 3
	}
	return 0 // Boot is not a fish token and cannot be used as payment.
}

func newCatanFishingTokens(players int) (*catanFishingTokens, error) {
	if players < 3 || players > 6 {
		return nil, errors.New("捕鱼筹码需要3至6位玩家")
	}
	f := &catanFishingTokens{Hands: make([][]int, players), BootOwner: -1, Discard: []int{}, Pending: []catanFishClaim{}}
	for i := range f.Hands {
		f.Hands[i] = []int{}
	}
	for id := 0; id < f.size(); id++ {
		f.DrawPile = append(f.DrawPile, id)
	}
	shuffle(f.DrawPile)
	return f, nil
}

func (f catanFishingTokens) size() int {
	if len(f.Hands) > 4 {
		return 44
	}
	return 30
}
func (f catanFishingTokens) validate() error {
	n := len(f.Hands)
	if n < 3 || n > 6 || f.BootOwner < -1 || f.BootOwner >= n {
		return errors.New("invalid fishing player count or boot owner")
	}
	seen := make([]bool, f.size())
	if f.BootOwner >= 0 {
		seen[catanFishBoot] = true
	}
	add := func(id int, hand bool) bool {
		if id < 0 || id >= len(seen) || seen[id] || hand && id == catanFishBoot {
			return false
		}
		seen[id] = true
		return true
	}
	for i, pile := range [][]int{f.DrawPile, f.Discard} {
		for _, id := range pile {
			if !add(id, i == 1) {
				return errors.New("invalid fishing token inventory")
			}
		}
	}
	for _, hand := range f.Hands {
		if len(hand) > 7 {
			return errors.New("fishing hand exceeds seven tokens")
		}
		for _, id := range hand {
			if !add(id, true) {
				return errors.New("invalid fishing hand")
			}
		}
	}
	for _, exists := range seen {
		if !exists {
			return errors.New("missing fishing token")
		}
	}
	actors := map[int]bool{}
	for i, q := range f.Pending {
		if q.Player < 0 || q.Player >= n || q.Remaining < 1 || q.Remaining > f.size() || actors[q.Player] || i == 0 && len(f.Hands[q.Player]) != 7 {
			return errors.New("invalid fishing draw queue")
		}
		actors[q.Player] = true
	}
	return nil
}

func (f *catanFishingTokens) UnmarshalJSON(data []byte) error {
	type saved catanFishingTokens
	var value saved
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	next := catanFishingTokens(value)
	if err := next.validate(); err != nil {
		return err
	}
	*f = next
	return nil
}

func (f catanFishingTokens) copy() catanFishingTokens {
	f.DrawPile = slices.Clone(f.DrawPile)
	f.Discard = slices.Clone(f.Discard)
	f.Pending = slices.Clone(f.Pending)
	f.Hands = slices.Clone(f.Hands)
	for i := range f.Hands {
		f.Hands[i] = slices.Clone(f.Hands[i])
	}
	return f
}

func (f *catanFishingTokens) drawOne(player int) error {
	if len(f.DrawPile) == 0 {
		f.DrawPile = slices.Clone(f.Discard)
		f.Discard = []int{}
		shuffle(f.DrawPile)
	}
	if len(f.DrawPile) == 0 {
		return errors.New("fish supply exhausted")
	}
	last := len(f.DrawPile) - 1
	id := f.DrawPile[last]
	f.DrawPile = f.DrawPile[:last]
	if id == catanFishBoot {
		f.BootOwner = player
	} else {
		f.Hands[player] = append(f.Hands[player], id)
	}
	return nil
}

func (f *catanFishingTokens) continueDraw() error {
	for len(f.Pending) > 0 {
		q := &f.Pending[0]
		if len(f.Hands[q.Player]) == 7 {
			return nil
		}
		if err := f.drawOne(q.Player); err != nil {
			return err
		}
		q.Remaining--
		if q.Remaining == 0 {
			f.Pending = f.Pending[1:]
		}
	}
	return nil
}

// Claims for the whole production must be aggregated before calling this:
// clockwise, one token at a time, including both lake and coast production.
// A boot consumes a draw but not a hand slot. At the cap, pause for that
// player's optional single replacement; all further draws for them are lost.
func (f *catanFishingTokens) beginDraw(active int, claims []int) error {
	if err := f.validate(); err != nil {
		return err
	}
	if active < 0 || active >= len(f.Hands) || len(claims) != len(f.Hands) || len(f.Pending) != 0 {
		return errors.New("invalid fishing production")
	}
	for _, count := range claims {
		if count < 0 || count > f.size() {
			return errors.New("invalid fishing entitlement")
		}
	}
	next := f.copy()
	for offset := range len(claims) {
		p := (active + offset) % len(claims)
		if claims[p] > 0 {
			next.Pending = append(next.Pending, catanFishClaim{p, claims[p]})
		}
	}
	if err := next.continueDraw(); err != nil {
		return err
	}
	if err := next.validate(); err != nil {
		return err
	}
	*f = next
	return nil
}

// token=nil declines; otherwise exchange exactly one owned token. The faceup
// discard cannot be chosen as the replacement, even when it looks better.
func (f *catanFishingTokens) replace(player int, token *int) error {
	if err := f.validate(); err != nil {
		return err
	}
	if len(f.Pending) == 0 || f.Pending[0].Player != player {
		return errors.New("请等待对应玩家选择是否更换鱼筹码")
	}
	next := f.copy()
	if token != nil {
		at := slices.Index(next.Hands[player], *token)
		if at < 0 {
			return errors.New("请选择自己的一枚鱼筹码")
		}
		next.Hands[player] = slices.Delete(next.Hands[player], at, at+1)
		next.Discard = append(next.Discard, *token)
		if err := next.drawOne(player); err != nil {
			return err
		}
	}
	// Always stop this player's draws, including when the replacement was boot.
	next.Pending = next.Pending[1:]
	if err := next.continueDraw(); err != nil {
		return err
	}
	if err := next.validate(); err != nil {
		return err
	}
	*f = next
	return nil
}

// The action layer supplies the price for one legal action. No making change
// and no carrying excess payment to a subsequent action.
func (f *catanFishingTokens) spend(player int, ids []int, cost int) error {
	if err := f.validate(); err != nil {
		return err
	}
	if player < 0 || player >= len(f.Hands) || len(f.Pending) > 0 || cost < 1 || len(ids) == 0 {
		return errors.New("当前不能支付鱼筹码")
	}
	seen := map[int]bool{}
	amount := 0
	for _, id := range ids {
		if seen[id] || !slices.Contains(f.Hands[player], id) || catanFishValue(id) == 0 {
			return errors.New("请选择自己不同的鱼筹码支付")
		}
		seen[id] = true
		amount += catanFishValue(id)
	}
	if amount < cost {
		return errors.New("支付的鱼不足")
	}
	next := f.copy()
	next.Hands[player] = slices.DeleteFunc(next.Hands[player], func(id int) bool { return seen[id] })
	next.Discard = append(next.Discard, ids...)
	*f = next
	return nil
}

// The game layer checks the acting player and supplies public VP only. The
// boot changes the owner's victory threshold, never their actual score.
func (f *catanFishingTokens) passBoot(player, target int, publicVP []int) error {
	if err := f.validate(); err != nil {
		return err
	}
	if len(f.Pending) > 0 || f.BootOwner != player || player < 0 || target < 0 || target >= len(f.Hands) || player == target || len(publicVP) != len(f.Hands) || publicVP[target] < publicVP[player] {
		return errors.New("旧靴子只能交给公开分数不少于你的其他玩家")
	}
	f.BootOwner = target
	return nil
}

func (f catanFishingTokens) view(viewer int) (catanFishTokensView, error) {
	if err := f.validate(); err != nil {
		return catanFishTokensView{}, err
	}
	v := catanFishTokensView{Remaining: len(f.DrawPile), Discard: []catanFishTokenFace{}, BootOwner: f.BootOwner, Players: make([]catanFishPlayerView, len(f.Hands))}
	for _, id := range f.Discard {
		v.Discard = append(v.Discard, catanFishTokenFace{id, catanFishValue(id)})
	}
	for p, hand := range f.Hands {
		v.Players[p].Count = len(hand)
		if p == viewer {
			for _, id := range hand {
				v.Players[p].Tokens = append(v.Players[p].Tokens, catanFishTokenFace{id, catanFishValue(id)})
			}
		}
	}
	if len(f.Pending) > 0 {
		p := f.Pending[0].Player
		v.Responder = &p
	}
	return v, nil
}
