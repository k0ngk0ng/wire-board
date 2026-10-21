package basketball

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Runtime is the only running match. Once settled it remains immutable behind
// these menu stages, until explicit next-match confirmation replaces it.
type CampaignState struct {
	Stage       string                      `json:"stage"`
	Round       uint64                      `json:"round"`
	Revision    uint64                      `json:"revision"`
	DecisionAt  int64                       `json:"decisionAt"`
	Settlement  *engine.CampaignMatchResult `json:"settlement,omitempty"`
	Preparation *CampaignRoundPreparation   `json:"preparation,omitempty"`
}
type CampaignRoundPreparation struct {
	Progress     campaign.Progress  `json:"progress"`
	Random       engine.RandomState `json:"random"`
	Opponent     uint8              `json:"opponent"`
	Teammates    [2]uint8           `json:"teammates"`
	HumanTactics [2]uint8           `json:"humanTactics"`
}
type CampaignView struct {
	CampaignState
	Attributes      *CampaignAttributes `json:"attributes,omitempty"`
	CanAct          bool                `json:"canAct"`
	CanChooseTactic bool                `json:"canChooseTactic"`
	Password        string              `json:"password,omitempty"`
	Opponents       []int               `json:"opponents"`
	AutomaticAt     int64               `json:"automaticAt"`
}
type CampaignAction struct {
	Type           string        `json:"type"`
	Revision       uint64        `json:"revision"`
	Opponent       uint8         `json:"opponent,omitempty"`
	Teammates      [2]uint8      `json:"teammates,omitempty"`
	Character      uint8         `json:"character,omitempty"`
	InventoryIndex uint8         `json:"inventoryIndex,omitempty"`
	Slot           campaign.Slot `json:"slot,omitempty"`
	Tactic         uint8         `json:"tactic,omitempty"`
}

func (s State) campaignMenu() bool { return s.Campaign != nil && s.Campaign.Stage != "playing" }
func (s State) campaignTerminal() bool {
	return s.Campaign != nil && (s.Campaign.Stage == "complete" || s.Campaign.Stage == "quit")
}

func (s *State) settleCampaign() error {
	if s.Campaign == nil || s.Campaign.Stage != "playing" || !engine.RuntimeFinished(s.Runtime) {
		return nil
	}
	result, err := engine.SettleCampaignRuntime(s.Runtime)
	if err != nil {
		return err
	}
	c := *s.Campaign
	if c.Revision >= uint64(maxSequence) {
		return fmt.Errorf("战役操作版本已耗尽")
	}
	c.Settlement = &result
	c.Preparation = nil
	c.Revision++
	c.DecisionAt = s.ServerAt + 3000
	switch result.NextStage {
	case engine.CampaignPrepare:
		c.Stage = "reward"
		// The original reward movie already supplies the confirmation flow.
		// Legacy saves without a movie retain their existing Continue screen.
		if s.Runtime.Presentation != nil && s.Runtime.Presentation.Complete {
			choices, e := campaign.AvailableOpponents(result.Progress)
			if e != nil || len(choices) == 0 {
				return fmt.Errorf("没有可选择的下一场比赛")
			}
			c.Stage = "prepare"
			c.Preparation = &CampaignRoundPreparation{Progress: result.Progress, Random: result.Random, Opponent: choices[0], Teammates: [2]uint8{s.Runtime.Config.Match.Characters[0], s.Runtime.Config.Match.Characters[1]}, HumanTactics: s.Runtime.Config.HumanTactics}
		}
	case engine.CampaignDefeat:
		c.Stage = "defeat"
	case engine.CampaignComplete:
		c.Stage = "complete"
		c.DecisionAt = 0
	}
	s.Campaign = &c
	for actor := range s.Inputs {
		clearTransport(&s.Inputs[actor])
		s.Inputs[actor].IdleAt = s.ServerAt + idleMS
	}
	return nil
}

func ownedCampaignItems(p campaign.Progress) (counts [32]int, total int) {
	add := func(item campaign.Item) {
		if item != campaign.Empty {
			counts[item]++
			total++
		}
	}
	for _, item := range p.Inventory {
		add(item)
	}
	for _, slots := range p.Equipment {
		for _, item := range slots {
			add(item)
		}
	}
	return
}

func validateRoundPreparation(p CampaignRoundPreparation, base engine.CampaignMatchResult, tactics [2]uint8) error {
	if err := p.Progress.Validate(); err != nil {
		return err
	}
	b := base.Progress
	if p.Progress.Mode != b.Mode || p.Progress.Period != b.Period || p.Progress.WinBits != b.WinBits || p.Progress.RewardSet != b.RewardSet || p.Random != base.Random || p.HumanTactics[1] != tactics[1] {
		return fmt.Errorf("准备阶段不能改写已获得的战役进度")
	}
	if p.Teammates[0] > 3 || p.Teammates[1] > 3 || p.Teammates[0] == p.Teammates[1] || p.HumanTactics[0] < 1 || p.HumanTactics[0] > 5 {
		return fmt.Errorf("无效战役阵容或战术")
	}
	if _, err := campaign.CourtForOpponent(p.Progress, p.Opponent); err != nil {
		return err
	}
	old, oldTotal := ownedCampaignItems(b)
	next, nextTotal := ownedCampaignItems(p.Progress)
	for item, count := range next {
		if count > old[item] {
			return fmt.Errorf("准备阶段出现未拥有的装备")
		}
	}
	if nextTotal < oldTotal && nextTotal < 16 {
		return fmt.Errorf("非满库存状态丢失装备")
	}
	return nil
}

func (s State) validateCampaignState() error {
	c := s.Campaign
	if c == nil {
		if s.Options.Campaign != nil {
			return fmt.Errorf("缺少战役会话")
		}
		return nil
	}
	if s.Options.Campaign == nil || s.Runtime.Config.Campaign == nil || c.Round == 0 || c.Round > uint64(maxSequence) || c.Revision > uint64(maxSequence) || c.DecisionAt < 0 {
		return fmt.Errorf("无效战役会话")
	}
	if c.Stage == "playing" {
		if c.Settlement != nil || c.Preparation != nil || c.DecisionAt != 0 || engine.RuntimeFinished(s.Runtime) {
			return fmt.Errorf("活动比赛与战役阶段不一致")
		}
		return nil
	}
	result, err := engine.SettleCampaignRuntime(s.Runtime)
	if err != nil {
		return err
	}
	if c.Settlement == nil || !reflect.DeepEqual(*c.Settlement, result) {
		return fmt.Errorf("战役结算与已确认比赛不一致")
	}
	switch c.Stage {
	case "reward":
		if result.NextStage != engine.CampaignPrepare || c.Preparation != nil {
			return fmt.Errorf("无效奖励阶段")
		}
	case "defeat":
		if result.NextStage != engine.CampaignDefeat || c.Preparation != nil {
			return fmt.Errorf("无效失败阶段")
		}
	case "complete":
		if result.NextStage != engine.CampaignComplete || c.Preparation != nil || c.DecisionAt != 0 {
			return fmt.Errorf("无效通关阶段")
		}
	case "prepare":
		if result.NextStage == engine.CampaignComplete || c.Preparation == nil {
			return fmt.Errorf("缺少下一场准备")
		}
	case "quit":
		if result.NextStage == engine.CampaignComplete || c.DecisionAt != 0 {
			return fmt.Errorf("无效退出阶段")
		}
	default:
		return fmt.Errorf("未知战役阶段")
	}
	if c.Preparation != nil {
		return validateRoundPreparation(*c.Preparation, result, s.Runtime.Config.HumanTactics)
	}
	return nil
}

func (s State) campaignView(actor int) (*CampaignView, error) {
	if s.Campaign == nil {
		return nil, nil
	}
	c := s.Campaign
	v := &CampaignView{CampaignState: *c, CanAct: actor == 0 && !s.Inputs[0].Automated && s.campaignMenu() && !s.campaignTerminal(), Opponents: []int{}}
	if s.campaignMenu() && !s.campaignTerminal() && s.Inputs[0].Automated {
		v.AutomaticAt = c.DecisionAt
	}
	p := *s.Runtime.Config.Campaign
	if c.Settlement != nil {
		p = c.Settlement.Progress
	}
	if c.Preparation != nil {
		p = c.Preparation.Progress
		attributes, err := campaignAttributes(p)
		if err != nil {
			return nil, err
		}
		v.Attributes = &attributes
	}
	if p.Period < 28 {
		var err error
		v.Password, err = campaign.EncodePassword(p)
		if err != nil {
			return nil, err
		}
		v.Opponents, err = campaignOpponentIDs(p)
		if err != nil {
			return nil, err
		}
	}
	v.CanChooseTactic = !s.Inputs[0].Automated && s.Inputs[1].Automated || s.Inputs[0].Automated && !s.Inputs[1].Automated
	return v, nil
}

func applyCampaignAction(ctx gameapi.Context, original State, a action) (State, error) {
	flags, err := automated(ctx, original.Options)
	if err != nil {
		return original, err
	}
	if ctx.Actor != 0 || original.actorForSeat(ctx.Actor) != 0 || flags[0] {
		return original, fmt.Errorf("只有未托管的第一席玩家可以管理战役")
	}
	if a.Epoch != original.Epoch || a.Campaign == nil || a.Preparation != nil || a.Sequence != 0 || a.Buttons != 0 || a.Seat != nil || a.BasedOn != nil || a.Events != nil {
		return original, fmt.Errorf("战役请求已过期或包含无效字段")
	}
	return advanceCampaignMenu(ctx, original, *a.Campaign, flags)
}

// This is platform menu scheduling, not an alternative basketball AI. All
// gameplay decisions continue through the recovered original engine.
func advanceCampaignMenu(ctx gameapi.Context, original State, a CampaignAction, flags [4]bool) (State, error) {
	fail := func(err error) (State, error) { return original, err }
	if !original.campaignMenu() || original.campaignTerminal() || a.Revision != original.Campaign.Revision {
		return fail(fmt.Errorf("战役准备已过期或当前不能操作"))
	}
	s := original
	c := *original.Campaign
	s.Campaign = &c
	if c.Preparation != nil {
		p := *c.Preparation
		c.Preparation = &p
	}
	if c.Revision >= uint64(maxSequence) {
		return fail(fmt.Errorf("战役操作版本已耗尽"))
	}
	switch a.Type {
	case "continue":
		if c.Stage != "reward" && c.Stage != "defeat" {
			return fail(fmt.Errorf("当前不是赛后确认阶段"))
		}
		choices, err := campaign.AvailableOpponents(c.Settlement.Progress)
		if err != nil || len(choices) == 0 {
			return fail(fmt.Errorf("没有可选择的下一场比赛"))
		}
		c.Preparation = &CampaignRoundPreparation{Progress: c.Settlement.Progress, Random: c.Settlement.Random, Opponent: choices[0], Teammates: [2]uint8{s.Runtime.Config.Match.Characters[0], s.Runtime.Config.Match.Characters[1]}, HumanTactics: s.Runtime.Config.HumanTactics}
		c.Stage = "prepare"
	case "quit":
		c.Stage = "quit"
	case "start":
		if c.Stage != "prepare" || c.Preparation == nil {
			return fail(fmt.Errorf("请先完成下一场准备"))
		}
		return startNextCampaign(ctx, s, flags)
	default:
		if c.Stage != "prepare" || c.Preparation == nil {
			return fail(fmt.Errorf("当前不能修改下一场准备"))
		}
		p := c.Preparation
		var err error
		switch a.Type {
		case "opponent":
			p.Opponent = a.Opponent
		case "teammates":
			p.Teammates = a.Teammates
		case "tactic":
			if flags[0] == flags[1] {
				return fail(fmt.Errorf("队友战术需要一名真人与一名电脑"))
			}
			p.HumanTactics[0] = a.Tactic
		case "equip":
			p.Progress, err = campaign.Equip(p.Progress, a.Character, a.InventoryIndex)
		case "unequip":
			var step campaign.UnequipStep
			step, err = campaign.Unequip(p.Progress, a.Character, a.Slot)
			p.Progress = step.Progress
		default:
			return fail(fmt.Errorf("未知战役操作"))
		}
		if err != nil {
			return fail(err)
		}
		if err = validateRoundPreparation(*p, *c.Settlement, s.Runtime.Config.HumanTactics); err != nil {
			return fail(err)
		}
		if *p == *original.Campaign.Preparation {
			return original, nil
		}
	}
	c.Revision++
	c.DecisionAt = ctx.Now.UnixMilli() + 3000
	if c.Stage == "quit" {
		c.DecisionAt = 0
	}
	s.Inputs[0].IdleAt = ctx.Now.UnixMilli() + idleMS
	if err := s.syncAutomation(flags); err != nil {
		return fail(err)
	}
	if err := s.validateCampaignState(); err != nil {
		return fail(err)
	}
	return s, nil
}

func startNextCampaign(ctx gameapi.Context, original State, flags [4]bool) (State, error) {
	c := original.Campaign
	p := c.Preparation
	if c.Round >= uint64(maxSequence) {
		return original, fmt.Errorf("战役场次已耗尽")
	}
	roster, err := engine.SelectOriginalCampaignRoster(p.Progress.Mode, p.Teammates, p.Random)
	if err != nil {
		return original, err
	}
	r, err := engine.NewCampaignRuntime(p.Progress, engine.CampaignMatchSelection{Opponent: p.Opponent, Characters: roster.Characters, HumanTactics: p.HumanTactics, Seed: roster.Random})
	if err != nil {
		return original, err
	}
	// Continue preserves the saved campaign's policy. A new campaign or
	// password entry still uses the current platform default.
	r.Config.RecoverUnsupportedBall = original.Runtime.Config.RecoverUnsupportedBall
	r, err = engine.SetRuntimeAutomation(r, flags)
	if err != nil {
		return original, err
	}
	o := original.Options
	entry := *o.Campaign
	o.Campaign = &entry
	o.Teams = r.Config.Match.Teams
	o.Characters = r.Config.Match.Characters
	o.Court = campaignCourtName(r.Config.Match.Court)
	entry.Opponent = p.Opponent
	entry.Tactic = p.HumanTactics[0]
	// Epoch identifies transport history, not RNG; deterministic derivation is
	// safe for retries because only the accepted transaction increments Round.
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s/campaign/%d", original.Epoch, c.Round+1)))
	s := State{Options: o, Epoch: hex.EncodeToString(digest[:12]), ServerAt: ctx.Now.UnixMilli(), Runtime: r, Events: []SoundEvent{}, Campaign: &CampaignState{Stage: "playing", Round: c.Round + 1}}
	for actor, automatic := range flags {
		s.Inputs[actor] = InputState{Automated: automatic, IdleAt: s.ServerAt + idleMS, LastPacketAt: s.ServerAt}
	}
	return s, nil
}

func (s State) campaignMatch(m gameapi.Match) (gameapi.Match, error) {
	if s.campaignTerminal() {
		r := &gameapi.Result{Cooperative: true, Ranks: make([]int, len(s.seatActors())), Scores: make([]int, len(s.seatActors())), Winners: []int{}}
		for seat := range r.Ranks {
			r.Ranks[seat] = 1
			r.Scores[seat] = int(s.scores()[0])
			if s.Campaign.Stage == "complete" {
				r.Winners = append(r.Winners, seat)
			}
		}
		r.Details, _ = json.Marshal(s.Campaign)
		m.Result = r
		return m, nil
	}
	m.Actors = []int{0}
	m.Attention = []int{0}
	m.SystemDeadline = s.ServerAt + frameMS
	return m, nil
}
