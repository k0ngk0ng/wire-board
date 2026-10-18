package basketball

import (
	"encoding/json"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// CampaignOptions describes an explicit room entry. Password import rebuilds
// the original inventory once; persisted rounds never re-read this entry code.
type CampaignOptions struct {
	Entry              string             `json:"entry,omitempty"`
	Password           string             `json:"password,omitempty"`
	Opponent           uint8              `json:"opponent"`
	Tactic             uint8              `json:"tactic"`
	Preparation        *campaign.Progress `json:"preparation,omitempty"`
	AvailableOpponents []int              `json:"availableOpponents"`
}

func campaignEntryKind(c CampaignOptions) (string, error) {
	kind := c.Entry
	if kind == "" {
		kind = "new"
		if c.Password != "" {
			kind = "password"
		}
	}
	if kind != "new" && kind != "password" {
		return "", fmt.Errorf("请选择新战役或原密码继续")
	}
	if kind == "new" && c.Password != "" {
		return "", fmt.Errorf("新战役不能同时输入原密码")
	}
	return kind, nil
}

// Public JSON lists must remain arrays: encoding/json treats []uint8 as bytes
// and emits a base64 string. The source leaf retains its original byte IDs.
func campaignOpponentIDs(p campaign.Progress) ([]int, error) {
	original, err := campaign.AvailableOpponents(p)
	if err != nil {
		return nil, err
	}
	ids := make([]int, len(original))
	for i, id := range original {
		ids[i] = int(id)
	}
	return ids, nil
}

func campaignMode(mode string) (uint8, error) {
	switch mode {
	case "solo":
		return 0, nil
	case "cooperative":
		return 1, nil
	}
	return 0, fmt.Errorf("战役仅支持单人或双人合作")
}

// Preview has no RNG: empty new-game inventory cannot depend on the future
// reward table. Only actual Start invokes the original one-time reward draw.
func campaignEntryProgress(o Options) (campaign.Progress, error) {
	kind, err := campaignEntryKind(*o.Campaign)
	if err != nil {
		return campaign.Progress{}, err
	}
	mode, err := campaignMode(o.Mode)
	if err != nil {
		return campaign.Progress{}, err
	}
	p, err := campaign.New(mode, 0)
	if kind == "password" {
		p, err = campaign.DecodePassword(o.Campaign.Password)
	}
	if err != nil {
		return p, err
	}
	if p.Mode != mode {
		return p, fmt.Errorf("原密码人数与房间模式不一致")
	}
	if desired := o.Campaign.Preparation; desired != nil {
		if err = desired.Validate(); err != nil {
			return p, err
		}
		if desired.Mode != p.Mode || desired.Period != p.Period || desired.WinBits != p.WinBits || desired.RewardSet != p.RewardSet {
			return p, fmt.Errorf("准备状态与战役入口进度不一致")
		}
		owned, _ := ownedCampaignItems(p)
		next, _ := ownedCampaignItems(*desired)
		if owned != next {
			return p, fmt.Errorf("准备装备必须来自已拥有的库存")
		}
		p = *desired
	}
	return p, nil
}

func campaignCourtName(court uint8) string {
	if court == 1 {
		return "new-york"
	}
	return fmt.Sprintf("court-%d", court)
}

func configureCampaign(o Options) (Options, error) {
	c := *o.Campaign
	o.Campaign = &c
	var err error
	c.Entry, err = campaignEntryKind(c)
	if err != nil {
		return o, err
	}
	if c.Tactic == 0 {
		c.Tactic = 4
	}
	if c.Tactic < 1 || c.Tactic > 5 {
		return o, fmt.Errorf("请选择原版战术 1 至 5")
	}
	if o.Characters[0] > 3 || o.Characters[1] > 3 || o.Characters[0] == o.Characters[1] {
		return o, fmt.Errorf("请选择两名不同的热血队球员")
	}
	p, err := campaignEntryProgress(o)
	if err != nil {
		return o, err
	}
	c.AvailableOpponents, err = campaignOpponentIDs(p)
	if err != nil {
		return o, err
	}
	if c.Opponent == 0 {
		if len(c.AvailableOpponents) == 0 {
			return o, fmt.Errorf("该战役已经结束")
		}
		c.Opponent = uint8(c.AvailableOpponents[0])
	}
	court, err := campaign.CourtForOpponent(p, c.Opponent)
	if err != nil {
		return o, err
	}
	c.Preparation = &p
	o.Teams = [2]uint8{0, c.Opponent}
	o.Court = campaignCourtName(court)
	// Opponents are selected by the original roster draw at confirmation.
	o.Characters[2], o.Characters[3] = 0, 1
	if c.Password != "" {
		c.Password, err = campaign.EncodePassword(p)
	}
	return o, err
}

func newCampaignMatch(o Options, random engine.RandomState) (engine.RuntimeState, error) {
	p, err := campaignEntryProgress(o)
	if err != nil {
		return engine.RuntimeState{}, err
	}
	if o.Campaign.Password == "" {
		initial, e := engine.NewOriginalCampaign(p.Mode, random)
		if e != nil {
			return engine.RuntimeState{}, e
		}
		p.RewardSet, random = initial.Progress.RewardSet, initial.Random
	}
	roster, err := engine.SelectOriginalCampaignRoster(p.Mode, [2]uint8{o.Characters[0], o.Characters[1]}, random)
	if err != nil {
		return engine.RuntimeState{}, err
	}
	return engine.NewCampaignRuntime(p, engine.CampaignMatchSelection{Opponent: o.Campaign.Opponent, Characters: roster.Characters, HumanTactics: [2]uint8{o.Campaign.Tactic, 4}, Seed: roster.Random})
}

// Waiting-room updates use the platform's authenticated host/seat and room
// version. Equipment is shared; per-character ownership survives roster swaps.
func prepareCampaign(ctx gameapi.PreparationContext, o Options, a preparationAction) (json.RawMessage, error) {
	c := *o.Campaign
	o.Campaign = &c
	p := *c.Preparation
	if a.Court != "" {
		return nil, fmt.Errorf("战役场地由对手及主客场进度决定")
	}
	switch a.Type {
	case "character":
		if a.Actor == nil || a.Character == nil || *a.Actor < 0 || *a.Actor > 1 || *a.Character > 3 || a.Opponent != nil || a.Tactic != nil || a.InventoryIndex != nil || a.Slot != nil {
			return nil, fmt.Errorf("无效战役球员选择")
		}
		actor := *a.Actor
		if !ctx.Host && actor != o.seatActors()[ctx.Actor] {
			return nil, fmt.Errorf("只能选择自己的球员")
		}
		other := actor ^ 1
		if o.Characters[actor] != *a.Character && o.Characters[other] == *a.Character {
			if !ctx.Host && other < len(ctx.Bots) && !ctx.Bots[other] {
				return nil, fmt.Errorf("该球员已由队友选择")
			}
			o.Characters[other] = o.Characters[actor]
		}
		o.Characters[actor] = *a.Character
	case "opponent":
		if !ctx.Host || a.Opponent == nil || a.Actor != nil || a.Character != nil || a.Tactic != nil || a.InventoryIndex != nil || a.Slot != nil {
			return nil, fmt.Errorf("只有房主能选择战役对手")
		}
		c.Opponent = *a.Opponent
	case "tactic":
		oneHuman := o.Mode == "solo" || len(ctx.Bots) == 2 && ctx.Bots[0] != ctx.Bots[1]
		if !ctx.Host || !oneHuman || a.Tactic == nil || a.Actor != nil || a.Character != nil || a.Opponent != nil || a.InventoryIndex != nil || a.Slot != nil {
			return nil, fmt.Errorf("队友战术需要房主及一名真人、一名电脑")
		}
		c.Tactic = *a.Tactic
	case "equip":
		if !ctx.Host || a.Character == nil || a.InventoryIndex == nil || a.Actor != nil || a.Opponent != nil || a.Tactic != nil || a.Slot != nil {
			return nil, fmt.Errorf("无效装备选择")
		}
		next, err := campaign.Equip(p, *a.Character, *a.InventoryIndex)
		if err != nil {
			return nil, err
		}
		p = next
	case "unequip":
		if !ctx.Host || a.Character == nil || a.Slot == nil || a.Actor != nil || a.Opponent != nil || a.Tactic != nil || a.InventoryIndex != nil {
			return nil, fmt.Errorf("无效卸装选择")
		}
		next, err := campaign.Unequip(p, *a.Character, *a.Slot)
		if err != nil {
			return nil, err
		}
		p = next.Progress
	default:
		return nil, fmt.Errorf("未知战役准备操作")
	}
	c.Preparation = &p
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	return configure(ctx.Capacity, raw)
}
