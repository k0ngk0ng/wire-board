package game

import (
	"errors"
	"slices"
)

func (s *State) sgHegCanShow(i int) bool {
	if !s.sgHegemony() || s.Sanguosha.Selecting || !s.sgAlive(i) {
		return false
	}
	g := s.Sanguosha
	return i == s.Turn || g.ActivePhase == "inactive" || !s.sgHas(s.Turn, "heg_huoshui")
}

// This only answers whether the player may invoke a private skill. It must not
// be used for public range, gender, faction or compulsory effects. Revealing is
// a separate committed action, never a side effect of a capability query.
func (s *State) sgHegMayInvoke(i int, skill string) bool {
	return s.sgHas(i, skill) || s.sgHegCanShow(i) && slices.Contains(s.sgHegSkills(i, false), skill)
}

func (s *State) sgHegRevealSkill(i int, skill string) error {
	if !s.sgHegemony() || s.sgHas(i, skill) {
		return nil
	}
	if !s.sgHegMayInvoke(i, skill) {
		return errors.New("此时不能明置并发动该技能")
	}
	slot := s.sgHegSkillSlot(i, skill)
	if slot < 0 {
		return errors.New("没有此武将技能")
	}
	s.sgHegShow(i, []int{slot}, true)
	return nil
}

func (s *State) sgHegRevealEvent(e SGEvent) bool {
	if e.Type == "heg_resume_prompt" {
		q := *e.Prompt
		s.sgAsk(q.Player, q.Kind, q.Message, q.Event)
		s.Sanguosha.Pending.Cards, s.Sanguosha.Pending.Targets, s.Sanguosha.Pending.Choices = q.Cards, q.Targets, q.Choices
		return true
	}
	if e.Type != "heg_reveal_turn" {
		return false
	}
	if !s.sgHegCanShow(e.Actor) {
		return true
	}
	h := s.Sanguosha.Players[e.Actor].Hegemony
	choices := []string{}
	if !h.Shown[0] {
		choices = append(choices, "head")
	}
	if !h.Shown[1] {
		choices = append(choices, "deputy")
	}
	if len(choices) == 2 {
		choices = append(choices, "both")
	}
	if len(choices) > 0 {
		s.sgAsk(e.Actor, "heg_reveal_turn", "准备阶段：可明置主将、副将或两将，也可继续暗置", e)
		s.Sanguosha.Pending.Choices = append(choices, "pass")
	}
	return true
}

func (s *State) sgHegRevealRespond(i int, a Action, q SGPrompt) (bool, error) {
	if q.Kind != "heg_reveal_turn" {
		return false, nil
	}
	if !slices.Contains(q.Choices, a.Choice) {
		return true, errors.New("请选择当前可明置的武将牌或放弃")
	}
	if a.Choice == "pass" {
		return true, nil
	}
	if !s.sgHegCanShow(i) {
		return true, errors.New("此时不能明置武将牌")
	}
	slots := map[string][]int{"head": {0}, "deputy": {1}, "both": {0, 1}}
	s.sgHegShow(i, slots[a.Choice], true)
	return true, nil
}

// Real card use/response calls this; sgAs itself remains a read-only validation
// helper. Outer applySanguosha commits the whole action transactionally, so a
// wrong card/target never exposes a general or grants a reveal reward.
func (s *State) sgCommittedAs(i int, ids []int, skill, desired string) (string, error) {
	if s.sgHegemony() && skill != "" && skill != "fan" && skill != "spear" {
		if err := s.sgHegRevealSkill(i, skill); err != nil {
			return "", err
		}
	}
	return s.sgAs(i, ids, skill, desired)
}

func sgHegActiveSkill(skill string) bool {
	return slices.Contains([]string{
		"heg_rende", "heg_zhiheng", "heg_lijian", "heg_fenxun", "heg_xiongyi", "heg_huoshui", "heg_qingcheng",
		"qiangxi", "quhu", "tianyi", "kurou", "fanjian", "jieyin", "dimeng", "zhijian", "qingnang", "luanwu",
	}, skill)
}

// Prompt kinds are private while invoking would expose a hidden skill. Merely
// presenting a response must not tell spectators which general is underneath.
func sgHegResponseSkill(q SGPrompt, a Action) string {
	if a.Choice == "pass" {
		return ""
	}
	switch q.Kind {
	case "invoke", "heg_invoke":
		return q.Event.Kind
	case "draw_phase":
		if slices.Contains([]string{"tuxi", "luoyi", "yingzi", "shuangxiong", "zaiqi", "haoshi"}, a.Choice) {
			return a.Choice
		}
	case "shensu_judge", "shensu_play":
		return "shensu"
	case "heg_luoshen_tiandu":
		return "tiandu"
	case "heg_luoshen", "heg_shenzhi", "heg_shushen", "heg_xiaoguo", "heg_sijian", "heg_kuangfu", "heg_lirang", "heg_shuangren",
		"heg_mingshi", "heg_suishi", "kuanggu",
		"keji", "liuli", "tieji", "liegong", "tianxiang", "leiji", "buqu", "tiandu", "guicai", "guidao", "niepan", "jieming", "fangzhu", "yinghun", "lieren", "qiaobian", "fangquan", "guzheng", "beige", "xingshang", "mengjin":
		return q.Kind
	}
	return ""
}

// A committed invocation pays its cost first. GeneralShown rewards then run
// before the suspended effect or follow-up response; the saved prompt stays
// server-only and receives a fresh ID when restored after the reward chain.
func (s *State) sgHegFlushRewards() {
	if !s.sgHegemony() || len(s.Sanguosha.Hegemony.RevealRewards) == 0 {
		return
	}
	g := s.Sanguosha
	es := g.Hegemony.RevealRewards
	g.Hegemony.RevealRewards = nil
	if g.Pending != nil {
		q := clone(*g.Pending)
		es = append(es, SGEvent{Type: "heg_resume_prompt", Prompt: &q})
		g.Pending = nil
	}
	s.sgPush(es...)
}

func (s *State) sgHegPrivatePrompt(q SGPrompt) bool {
	if !s.sgHegemony() || !s.sgAlive(q.Player) {
		return false
	}
	if q.Kind == "draw_phase" {
		for _, skill := range []string{"tuxi", "luoyi", "yingzi", "shuangxiong", "zaiqi", "haoshi"} {
			if s.sgHegMayInvoke(q.Player, skill) && !s.sgHas(q.Player, skill) {
				return true
			}
		}
	}
	skill := sgHegResponseSkill(q, Action{Choice: "yes"})
	return skill != "" && !s.sgHas(q.Player, skill)
}

// Used only for the skill owner's own decision. A still-hidden potential ally
// never becomes a public friend or a legal target of a public faction card.
func (s *State) sgHegWouldFriend(i, target int) bool {
	if s.sgHegFriend(i, target) {
		return true
	}
	if s.sgHegShown(i) || !s.sgHegShown(target) {
		return false
	}
	g := s.Sanguosha
	kingdom := sgGeneral(g.Players[i].General).Kingdom
	if g.Players[target].Role != kingdom {
		return false
	}
	n := 1
	for j, p := range g.Players {
		if j != i && s.sgHegShown(j) && p.Role == kingdom {
			n++
		}
	}
	return n <= len(g.Players)/2
}
