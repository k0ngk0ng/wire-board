package yuyuhakusho

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/sessiontick"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
)

func (s *State) advance(now int64) error {
	if now < s.ServerAt {
		return errors.New("系统时钟倒退")
	}
	if s.Loading != nil || s.World.MatchComplete {
		return nil
	}
	elapsed := now - s.ServerAt
	if elapsed > maxAdvanceMS {
		elapsed = maxAdvanceMS
	}
	total := s.Remainder + elapsed*clockNumerator
	steps := total / clockDenominator
	remainder := total % clockDenominator
	config, err := s.liveOptions()
	if err != nil {
		return err
	}
	mask := s.currentMask()
	for step := int64(0); step < steps && !s.World.MatchComplete; step++ {
		if s.Frame == ^uint64(0) {
			return errors.New("网络帧计数已满")
		}
		next := s.Frame + 1
		var held [4]engine.Buttons
		for seat := range s.Inputs {
			input := &s.Inputs[seat]
			if input.Automated {
				input.Buttons = 0
				input.Changes = nil
				continue
			}
			held[seat] = s.World.Controllers[seat].Current
			// With no pending transition, the transport's desired state owns
			// the next sample. Restore/reclaim clear it without rewriting rule
			// history; sampling zero here produces the original Released bits.
			if len(input.Changes) == 0 {
				held[seat] = input.Buttons
			}
			if now-input.LastPacketAt >= lostInputMS {
				input.Buttons = 0
				input.Changes = nil
				input.AppliedSequence = input.Sequence
				held[seat] = 0
				continue
			}
			if len(input.Changes) > 0 && input.Changes[0].Frame <= next {
				change := input.Changes[0]
				held[seat] = change.Buttons
				input.AppliedSequence = change.Sequence
				input.Changes = append([]InputChange(nil), input.Changes[1:]...)
			}
		}
		world, visual, _, _, err := sessiontick.Advance(s.World, s.Presentation, config, held, mask, resources.rules, resources.graphics)
		if err != nil {
			return err
		}
		history, err := syncwire.Push(s.History, syncwire.InputFrame{Frame: next, Held: held, Automated: mask})
		if err != nil {
			return err
		}
		s.World, s.Presentation, s.History, s.Frame = world, visual, history, next
	}
	s.ServerAt = now
	s.Remainder = remainder
	return nil
}
