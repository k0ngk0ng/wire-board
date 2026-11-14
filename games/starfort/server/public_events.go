package starfort

import (
	"errors"
	"slices"
)

// PublicEvent is the per-wave income ledger, separate from tasks and balances.
// Persisting it prevents spending, reconnects or refunds from changing tax.
type PublicEvent struct {
	Wave      int    `json:"wave"`
	Kind      string `json:"kind"`
	StartedMS int64  `json:"startedMS"`
	Earned    []int  `json:"earned"`
	Settled   bool   `json:"settled"`
}

func (s *State) publicRules() bool { return s.RevisionRules >= 3 }
func (s *State) startPublicEvent() {
	if !s.publicRules() {
		return
	}
	// A wave-local stream does not consume any existing gameplay RNG state.
	x := uint64(s.World.Seed) ^ uint64(s.Wave)*0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	kind := "none"
	if x%100 < 30 {
		kind = "tax"
		if (x/100)%2 == 1 {
			kind = "grant"
		}
	}
	s.PublicEvent = &PublicEvent{Wave: s.Wave, Kind: kind, StartedMS: s.ElapsedMS, Earned: make([]int, len(s.Players))}
}
func (s *State) combatGold(owner, amount int) {
	before := s.Players[owner].Gold
	s.Players[owner].Gold = min(maxValue, before+amount)
	if e := s.PublicEvent; s.publicRules() && e != nil && !e.Settled && e.Wave == s.Wave {
		e.Earned[owner] = min(maxValue, e.Earned[owner]+s.Players[owner].Gold-before)
	}
}
func (s *State) deathGold(e Enemy) int {
	reward := s.rewardGold(e.Reward)
	if s.publicRules() && e.Kind == "boss" {
		bonus := min(maxValue, s.Wave*10)
		reward = min(maxValue, reward+bonus)
		s.note("击败星陨巨像：每位玩家额外获得 %d 金币", bonus)
	}
	for i := range s.Players {
		s.combatGold(i, reward)
	}
	return reward
}
func (s *State) settlePublicEvent() {
	e := s.PublicEvent
	if !s.publicRules() || e == nil || e.Settled || e.Wave != s.Wave {
		return
	}
	e.Settled = true
	for i, income := range e.Earned {
		switch e.Kind {
		case "tax":
			s.Players[i].Gold = max(-maxValue, s.Players[i].Gold-income*20/100)
		case "grant":
			s.Players[i].Gold = min(maxValue, s.Players[i].Gold+income*30/100)
		}
	}
	if e.Kind == "tax" {
		s.note("征税结算：各玩家本波非任务收入扣除 20%%")
	}
	if e.Kind == "grant" {
		s.note("发钱结算：各玩家本波非任务收入增加 30%%")
	}
}
func (s *State) validPublicEvent() error {
	if s.RewardRemainingMS < 0 || s.RewardRemainingMS > int64(s.Options.PrepSeconds)*1000 || len(s.RewardChoosing) > len(s.Players) || len(s.RewardChoosing) == 0 && s.RewardRemainingMS != 0 || len(s.RewardChoosing) > 0 && (!s.publicRules() || s.Phase != "prepare" && s.Phase != "paused") {
		return errors.New("无效奖励选择计时")
	}
	seen := map[int]bool{}
	for _, owner := range s.RewardChoosing {
		if owner < 0 || owner >= len(s.Players) || seen[owner] || s.rewardFor(owner, "") == nil {
			return errors.New("无效奖励选择玩家")
		}
		seen[owner] = true
	}

	e := s.PublicEvent
	if e == nil {
		return nil
	}
	if !s.publicRules() || e.Wave < 1 || e.Wave > s.Wave || !slices.Contains([]string{"none", "tax", "grant"}, e.Kind) || e.StartedMS < 0 || e.StartedMS > s.ElapsedMS || len(e.Earned) != len(s.Players) || !e.Settled && e.Wave != s.Wave {
		return errors.New("无效公共事件")
	}
	for _, v := range e.Earned {
		if v < 0 || v > maxValue {
			return errors.New("无效公共事件收入")
		}
	}
	return nil
}
func (s *State) canPayTask(player, cost int) bool {
	if s.publicRules() {
		return s.Players[player].Gold >= 0
	}
	return s.Players[player].Gold >= cost
}

func (s *State) iceSlowStep() float64 {
	if s.publicRules() {
		return .05
	}
	return .02
}
func (s *State) volleyVisualMS() int {
	if s.publicRules() {
		return 3000
	}
	return 2000
}

func (s *State) selectReward(owner int, open bool, now int64) error {
	if !s.publicRules() {
		return errors.New("当前规则不支持奖励计时")
	}
	if !open {
		s.RewardChoosing = slices.DeleteFunc(s.RewardChoosing, func(p int) bool { return p == owner })
		if len(s.RewardChoosing) == 0 {
			if s.RewardRemainingMS > 0 && s.Phase == "prepare" {
				s.NextAt = now + s.RewardRemainingMS
			}
			s.RewardRemainingMS = 0
		}
		return nil
	}
	if s.Phase != "prepare" || s.World.Choice != nil || s.World.BoonChoice != nil || s.rewardFor(owner, "") == nil {
		return errors.New("当前没有可选择的波间奖励")
	}
	if slices.Contains(s.RewardChoosing, owner) {
		return nil
	}
	if len(s.RewardChoosing) == 0 {
		s.RewardRemainingMS = max(int64(0), s.NextAt-now)
		s.ServerAt = now
	}
	s.RewardChoosing = append(s.RewardChoosing, owner)
	return nil
}
