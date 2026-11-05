package yuyuhakusho

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func seatSlots(n int) []uint8 {
	switch n {
	case 2:
		return []uint8{0, 3}
	case 3:
		return []uint8{0, 1, 3}
	case 4:
		return []uint8{0, 1, 2, 3}
	}
	return nil
}
func hostAutomation(ctx gameapi.Context, n int) ([]bool, error) {
	if ctx.Players != n || n < 2 || n > 4 || ctx.Now.UnixMilli() < 0 {
		return nil, errors.New("无效席位或时钟")
	}
	if ctx.Automated == nil {
		return make([]bool, n), nil
	}
	if len(ctx.Automated) != n {
		return nil, errors.New("席位控制信息不完整")
	}
	return append([]bool(nil), ctx.Automated...), nil
}
func (s *State) syncAutomation(flags []bool) {
	for seat, enabled := range flags {
		input := &s.Inputs[seat]
		if input.Automated != enabled {
			input.Buttons = 0
			input.Changes = nil
			if s.Loading != nil && !enabled {
				s.Loading.Ready[seat] = false
			}
		}
		input.Automated = enabled
		if enabled {
			s.HadComputer = true
			if s.Loading != nil {
				s.Loading.Ready[seat] = true
			}
		}
	}
}
func (s State) currentMask() uint8 {
	var mask uint8
	for seat, input := range s.Inputs {
		if input.Automated {
			mask |= 1 << seat
		}
	}
	return mask
}
func (s State) liveOptions() (engine.ChuSessionOptions, error) {
	flags := make([]bool, len(s.Inputs))
	for seat, slot := range seatSlots(len(flags)) {
		flags[seat] = s.World.Match.Controllers[slot] == 0
	}
	return sessionOptions(s.Options, flags, 0)
}
func strictSaved(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("存档包含多余内容")
	}
	return nil
}
func decode(m gameapi.Match) (State, error) {
	var s State
	if resources.err != nil {
		return s, fmt.Errorf("游戏计算资源不可用：%w", resources.err)
	}
	if m.Schema != 1 || len(m.Data) == 0 || len(m.Data) > 2<<20 {
		return s, errors.New("不支持的对局存档")
	}
	if err := strictSaved(m.Data, &s); err != nil {
		return State{}, err
	}
	if s.ResourceDescriptorSHA256 != resources.descriptorSHA {
		return State{}, errors.New("对局存档与当前资源版本不一致")
	}
	n := len(s.Options.Characters)
	if err := validateOptions(n, s.Options); err != nil {
		return State{}, err
	}
	if len(s.Epoch) != 24 || s.ServerAt < 0 || s.ServerAt > int64(^uint64(0)>>1)-maxAdvanceMS || s.Remainder < 0 || s.Remainder >= clockDenominator || len(s.Inputs) != n || len(s.World.Stream) != 1024 || len(s.World.Objects) > 128 || s.World.Match.Arena != s.Options.Arena || s.World.Match.Grouping == 4 != s.Options.TeamBattle || s.World.Clock.Frame != uint16(s.Frame) {
		return State{}, errors.New("无效对局状态")
	}
	if _, err := hex.DecodeString(s.Epoch); err != nil {
		return State{}, errors.New("无效对局标识")
	}
	if s.Loading != nil {
		if len(s.Loading.Ready) != n || s.Frame != 0 || s.World.ResourceBusy == 0 {
			return State{}, errors.New("无效加载状态")
		}
	} else if s.World.ResourceBusy != 0 {
		return State{}, errors.New("资源就绪状态不一致")
	}
	for seat, slot := range seatSlots(n) {
		if s.World.Match.Characters[slot] != s.Options.Characters[seat]+1 {
			return State{}, errors.New("角色与存档配置不一致")
		}
	}
	for _, input := range s.Inputs {
		if input.Sequence < 0 || input.Sequence > maxSequence || input.AppliedSequence < 0 || input.AppliedSequence > input.Sequence || input.LastPacketAt < 0 || len(input.Changes) > maxInputQueue {
			return State{}, errors.New("无效输入状态")
		}
		if err := engine.ValidateControllerButtons(input.Buttons); err != nil {
			return State{}, err
		}
		lastFrame, lastSeq := s.Frame, input.AppliedSequence
		for _, change := range input.Changes {
			if change.Frame <= lastFrame || change.Sequence <= lastSeq || change.Sequence > input.Sequence {
				return State{}, errors.New("输入队列顺序无效")
			}
			if err := engine.ValidateControllerButtons(change.Buttons); err != nil {
				return State{}, err
			}
			lastFrame, lastSeq = change.Frame, change.Sequence
		}
	}
	if len(s.History) > syncwire.MaxFrames {
		return State{}, errors.New("同步历史过长")
	}
	if _, err := syncwire.Encode(s.History); err != nil {
		return State{}, err
	}
	if len(s.History) > 0 && s.History[len(s.History)-1].Frame != s.Frame {
		return State{}, errors.New("同步历史与世界帧不一致")
	}
	if err := renderstate.Validate(s.World, s.Presentation, resources.graphics); err != nil {
		return State{}, err
	}
	return s, nil
}
func (s State) phase() string {
	if s.Loading != nil {
		return "loading"
	}
	if s.World.MatchComplete {
		return "result"
	}
	if s.World.Clock.PauseFlags&128 != 0 {
		return "paused"
	}
	if manager, ok := s.World.Objects[0x8040]; ok && manager.Header.Behavior == 0xf2a4 && manager.Header.Stage <= 12 {
		return "opening"
	}
	return "playing"
}
func (s State) match() (gameapi.Match, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	// All original active actors correspond to platform seats; there are no
	// hidden non-seat AI fighters. Host bot/autoplay metadata remains separate.
	m := gameapi.Match{Schema: 1, Data: raw, Actors: []int{}, TimeoutActors: []int{}, Attention: []int{}, CheckpointKey: s.checkpointKey()}
	n := len(s.Inputs)
	if s.World.MatchComplete {
		result := &gameapi.Result{Ranks: make([]int, n), Winners: []int{}, Unrated: s.HadComputer, Team: s.Options.TeamBattle}
		if s.World.Match.WinnerCode == engine.MatchDrawCode {
			result.Draw = true
			result.Team = false
			for seat := range result.Ranks {
				result.Ranks[seat] = 1
			}
		} else {
			for seat, slot := range seatSlots(n) {
				team := int16(slot + 1)
				if s.Options.TeamBattle {
					team = 1
					if slot >= 2 {
						team = 4
					}
				}
				if team == s.World.Match.WinnerCode {
					result.Winners = append(result.Winners, seat)
				}
			}
			if len(result.Winners) == 0 {
				return gameapi.Match{}, errors.New("胜方无法映射到房间席位")
			}
			for seat := range result.Ranks {
				result.Ranks[seat] = len(result.Winners) + 1
			}
			for _, seat := range result.Winners {
				result.Ranks[seat] = 1
			}
		}
		result.Details, _ = json.Marshal(struct {
			Frame      uint64 `json:"frame,string"`
			Arena      uint16 `json:"arena"`
			WinnerCode int16  `json:"winnerCode"`
		}{s.Frame, s.Options.Arena, s.World.Match.WinnerCode})
		m.Result = result
		return m, nil
	}
	for seat := 0; seat < n; seat++ {
		m.Actors = append(m.Actors, seat)
	}
	if s.Loading == nil {
		m.SystemDeadline = s.ServerAt + frameMS
	}
	return m, nil
}
func (s State) checkpointKey() string {
	stage := uint16(0)
	if manager, ok := s.World.Objects[0x8040]; ok {
		stage = manager.Header.Stage
	}
	return fmt.Sprintf("%s/%t/%d/%t", s.Epoch, s.Loading != nil, stage, s.World.MatchComplete)
}
