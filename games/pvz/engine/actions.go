package engine

import (
	"errors"
	"slices"
)

func Apply(s *State, actor int, a Action) error {
	next := Clone(*s)
	if e := applyAction(&next, actor, a); e != nil {
		return e
	}
	next.removeDead()
	states := map[int]string{}
	for _, p := range s.Plants {
		states[p.ID] = p.State
	}
	for i := range next.Plants {
		p := &next.Plants[i]
		if old, ok := states[p.ID]; !ok || old != p.State {
			if p.Flags == nil {
				p.Flags = map[string]int64{}
			}
			p.Flags["phaseAt"] = next.Tick
		}
	}
	*s = next
	return nil
}
func applyAction(s *State, actor int, a Action) error {
	if actor < 0 || actor >= len(s.Players) || s.Phase != "prepare" && s.Phase != "battle" {
		return errors.New("当前不能操作")
	}
	if a.Epoch != s.Epoch {
		return errors.New("回合已更新，请重新操作")
	}
	p := &s.Players[actor]
	if a.Seq <= p.Seq || a.Seq <= 0 || a.Seq > 2147483647 || a.Seq-p.Seq > 1024 {
		return errors.New("操作已处理或序号无效")
	}
	switch a.Type {
	case "deck":
		if (s.Phase != "prepare" && !(s.RulesVersion >= 4 && s.Phase == "battle")) || len(a.Kinds) < 1 || len(a.Kinds) > MaxDeck || a.Kind != "" || a.PlantID != 0 {
			if s.RulesVersion >= 4 {
				return errors.New("快捷栏需要保留 1～10 张本阵营卡牌")
			}
			return errors.New("仅布阵时可选择至多10张卡")
		}
		seen := map[string]bool{}
		for _, kind := range a.Kinds {
			_, ok := PlantSpec(kind)
			if p.Side == "zombies" {
				_, ok = ZombieSpec(kind)
			}
			if !ok || seen[kind] {
				return errors.New("卡组包含未知或重复卡牌")
			}
			seen[kind] = true
		}
		imitate := a.ImitateKind
		if imitate == "" {
			imitate = p.ImitaterKind
		}
		c, ok := PlantSpec(imitate)
		if (seen["imitater"] || s.RulesVersion >= 4) && (!ok || imitate == "imitater" || c.UpgradeFrom != "") {
			return errors.New("模仿者需要选择一种普通植物")
		}
		p.Deck = slices.Clone(a.Kinds)
		p.ImitaterKind = imitate
	case "plant":
		if p.Side != "plants" || !s.canUseCard(actor, a.Kind) {
			return errors.New("请使用本人的植物卡组")
		}
		if a.Row < 0 || a.Row >= s.rowCount() || a.Col < 0 || a.Col >= Columns || a.PlantID != 0 {
			return errors.New("无效植物格子")
		}
		original, ok := PlantSpec(a.Kind)
		if !ok {
			return errors.New("未知植物")
		}
		c := original
		target := ""
		if a.Kind == "imitater" {
			target = p.ImitaterKind
			if a.ImitateKind != "" && a.ImitateKind != target {
				return errors.New("模仿目标与卡组不符")
			}
			c, ok = PlantSpec(target)
			if !ok || c.UpgradeFrom != "" || target == "imitater" {
				return errors.New("无效模仿目标")
			}
		}
		if p.Money < c.Cost {
			return errors.New("阳光不足")
		}
		if p.Cooldowns[a.Kind] > s.Tick || s.TeamCooldowns[p.Team][a.Kind] > s.Tick {
			return errors.New("卡牌尚未冷却")
		}
		if e := s.validatePlantPlace(actor, c, a.Row, a.Col); e != nil {
			return e
		}
		if c.Kind == "coffeebean" && target == "" {
			at := s.bodyAt(a.Row, a.Col)
			s.Plants[at].Flags["wakeAt"] = s.Tick + 20
			s.emit("coffee", a.Row, PlantX(s.Plants[at]), actor, "coffeebean", s.Plants[at].ID, 0)
		} else {
			if c.UpgradeFrom != "" {
				ids := map[int]bool{}
				base := s.bodyAt(a.Row, a.Col)
				if c.Kind == "cattail" {
					base = s.layerAt(a.Row, a.Col, "support")
				}
				ids[s.Plants[base].ID] = true
				if c.Kind == "cobcannon" {
					ids[s.Plants[s.bodyAt(a.Row, a.Col+1)].ID] = true
				}
				live := s.Plants[:0]
				for _, plant := range s.Plants {
					if !ids[plant.ID] {
						live = append(live, plant)
					}
				}
				s.Plants = live
			}
			plant := s.newPlant(c, a.Row, a.Col, actor)
			if target != "" {
				plant.Kind = "imitater"
				plant.TargetKind = target
				plant.State = "imitating"
				plant.ReadyAt = s.Tick + 64
				plant.Imitated = true
				plant.Asleep = false
				plant.HP = original.HP
				plant.MaxHP = original.HP
			}
			s.Plants = append(s.Plants, plant)
			s.emit("plant", a.Row, PlantX(plant), actor, plant.Kind, plant.ID, 0)
		}
		p.Money -= c.Cost
		p.Cooldowns[a.Kind] = s.Tick + int64(c.Cooldown/TickMS)
		if c.Kind == "cherrybomb" {
			s.TeamCooldowns[p.Team][c.Kind] = s.Tick + 30
		}
	case "spawn":
		if p.Side != "zombies" || s.Phase != "battle" || !s.canUseCard(actor, a.Kind) {
			return errors.New("请在开战后使用本人的僵尸卡组")
		}
		c, ok := ZombieSpec(a.Kind)
		if !ok || a.Row < 0 || a.Row >= s.rowCount() || a.PlantID != 0 {
			return errors.New("无效僵尸或进攻行")
		}
		if c.Target != "cell" && a.Col != 0 || c.Target == "cell" && (a.Col < 0 || a.Col >= Columns) {
			return errors.New("无效出兵目标")
		}
		if !specScene(c, s.Map.ID) || c.WaterOnly && !s.isWater(a.Row) || s.isWater(a.Row) && !c.WaterOnly && !c.WaterAllowed {
			return errors.New("该僵尸不能从这条路线出场")
		}
		if s.RulesVersion >= 2 && s.Tick < s.unlockAt(a.Kind) {
			return errors.New("该特殊僵尸尚未解锁，请等待卡牌倒计时")
		}
		if c.RequiresIce {
			ice := false
			for _, v := range s.Terrain {
				if v.Kind == "ice" && v.Row == a.Row && v.Until > s.Tick && v.ToX >= 9000 {
					ice = true
				}
			}
			if !ice {
				return errors.New("雪橇小队需要已有冰道")
			}
		}
		if p.Money < c.Cost {
			return errors.New("脑力不足")
		}
		if p.Cooldowns[a.Kind] > s.Tick || s.LaneCooldowns[a.Row] > s.Tick {
			return errors.New("卡牌或本行入口尚未冷却")
		}
		if len(s.Zombies) >= MaxZombies {
			return errors.New("场上僵尸已满")
		}
		if a.Kind == "zomboss" {
			for _, z := range s.Zombies {
				if z.Kind == "zomboss" && z.HP > 0 {
					return errors.New("场上已有僵王")
				}
			}
		}
		id := s.spawnZombie(a.Kind, a.Row, 9500, actor)
		if id == 0 {
			return errors.New("僵尸未能入场")
		}
		if c.Target == "cell" {
			for i := range s.Zombies {
				z := &s.Zombies[i]
				if z.ID == id {
					z.X = a.Col*1000 + 500
					z.TargetID = s.topPlantID(a.Row, a.Col)
				}
			}
		}
		p.Money -= c.Cost
		p.Cooldowns[a.Kind] = s.Tick + int64(c.Cooldown/TickMS)
		s.LaneCooldowns[a.Row] = s.Tick + 8
		// A single team entrance cadence prevents rapid card cycling and two
		// teammates from creating an opening burst across every lane at once.
		for i := range s.Players {
			q := &s.Players[i]
			if q.Team == p.Team && q.Side == "zombies" {
				kinds := q.Deck
				if s.RulesVersion >= 4 {
					kinds = rosterKinds("zombies")
				}
				for _, kind := range kinds {
					q.Cooldowns[kind] = max(q.Cooldowns[kind], s.Tick+3000/TickMS)
				}
			}
		}
	case "shovel":
		if p.Side != "plants" || a.PlantID <= 0 || a.Kind != "" {
			return errors.New("当前不能铲除")
		}
		at := s.plantIndex(a.PlantID)
		if at < 0 || s.Plants[at].Owner != actor {
			return errors.New("只能铲除自己的植物")
		}
		plant := s.Plants[at]
		s.Plants = append(s.Plants[:at], s.Plants[at+1:]...)
		s.dropUnsupported(plant.Row, plant.Col)
		s.emit("shovel", plant.Row, PlantX(plant), actor, plant.Kind, plant.ID, 0)
	case "ability":
		if s.Phase != "battle" || a.Kind != "" {
			return errors.New("当前不能使用技能")
		}
		if e := s.UsePlantAbility(actor, a.PlantID, a.Row, a.Col); e != nil {
			return e
		}
	case "collect":
		at := -1
		for i, v := range s.Pickups {
			if v.ID == a.PickupID {
				at = i
				break
			}
		}
		if at < 0 || s.Players[s.Pickups[at].Owner].Team != p.Team {
			return errors.New("只能协助收取本队资源")
		}
		s.collectPickup(a.PickupID)
	default:
		return errors.New("未知操作")
	}
	p.Seq = a.Seq
	return nil
}
func specScene(c Spec, scene string) bool {
	return len(c.Scenes) == 0 || slices.Contains(c.Scenes, scene)
}
func (s *State) validatePlantPlace(actor int, c Spec, row, col int) error {
	if !specScene(c, s.Map.ID) || c.WaterOnly && !s.isWater(row) {
		return errors.New("该植物需要水面")
	}
	if (c.Kind == "spikeweed" || c.Kind == "spikerock") && (s.isWater(row) || s.Map.Roof) {
		return errors.New("地刺需要陆地草坪")
	}
	for _, v := range s.Craters {
		if v.Row == row && v.Col == col && v.Until > s.Tick {
			return errors.New("弹坑尚未恢复")
		}
	}
	for _, v := range s.Terrain {
		if v.Kind == "ice" && v.Row == row && PlantX(Plant{Col: col}) >= v.FromX && PlantX(Plant{Col: col}) <= v.ToX && v.Until > s.Tick {
			return errors.New("冰道上暂时不能种植")
		}
	}
	grave := 0
	for _, g := range s.Graves {
		if g.Row == row && g.Col == col {
			grave = g.ID
		}
	}
	if c.Kind == "gravebuster" {
		if grave == 0 {
			return errors.New("这里没有墓碑")
		}
	} else if grave > 0 {
		return errors.New("请先清除墓碑")
	}
	body := s.bodyAt(row, col)
	support := s.layerAt(row, col, "support")
	if c.Kind == "potatomine" && s.isWater(row) {
		return errors.New("土豆雷不能种在水面上")
	}
	if (c.Kind == "spikeweed" || c.Kind == "spikerock") && support >= 0 {
		return errors.New("地刺需要直接种在草地上")
	}
	if (c.Kind == "seashroom" || c.Kind == "tanglekelp") && support >= 0 {
		return errors.New("水生植物需要直接种在水中")
	}
	if support >= 0 && s.Plants[support].Kind == "imitater" && c.Layer != "support" {
		return errors.New("请等待基座模仿完成")
	}
	if c.Kind == "coffeebean" {
		if body < 0 || !s.Plants[body].Asleep || s.Plants[body].Flags["wakeAt"] > s.Tick || s.layerAt(row, col, "instant") >= 0 {
			return errors.New("请选择一株睡眠的蘑菇")
		}
		return nil
	}
	if c.UpgradeFrom != "" {
		if c.Kind == "cattail" && body >= 0 {
			return errors.New("请先移除睡莲上的植物，再升级香蒲")
		}
		base := body
		if c.Kind == "cattail" {
			base = support
		}
		if base < 0 || s.Plants[base].Kind != c.UpgradeFrom || s.Plants[base].Owner != actor {
			return errors.New("升级需要本人的对应基础植物")
		}
		if c.Kind == "cobcannon" {
			if s.layerAt(row, col, "shell") >= 0 || s.layerAt(row, col+1, "shell") >= 0 {
				return errors.New("请先移除南瓜保护壳，再建玉米炮")
			}
			right := s.bodyAt(row, col+1)
			if col >= Columns-1 || right < 0 || s.Plants[right].Kind != "kernelpult" || s.Plants[right].Owner != actor {
				return errors.New("玉米加农炮需要相邻两株本人玉米投手")
			}
			leftPot := support >= 0 && s.Plants[support].Kind == "flowerpot"
			rightBase := s.layerAt(row, col+1, "support")
			rightPot := rightBase >= 0 && s.Plants[rightBase].Kind == "flowerpot"
			if leftPot != rightPot {
				return errors.New("两株玉米投手的花盆状态必须一致")
			}
		}
		return nil
	}
	if c.Layer == "support" {
		if support >= 0 || body >= 0 || s.layerAt(row, col, "shell") >= 0 {
			return errors.New("这里已有基座或植物")
		}
		if c.Kind == "lilypad" && !s.isWater(row) || c.Kind == "flowerpot" && s.isWater(row) {
			return errors.New("基座不适合该地形")
		}
		return nil
	}
	if c.Layer == "shell" {
		if s.layerAt(row, col, "shell") >= 0 {
			return errors.New("这里已有南瓜保护壳")
		}
		if body >= 0 && s.Plants[body].Kind == "cobcannon" {
			return errors.New("玉米加农炮不能套南瓜")
		}
	} else if body >= 0 {
		return errors.New("这个格子已有植物")
	}
	aquatic := c.Kind == "seashroom" || c.Kind == "tanglekelp" || c.Kind == "cattail"
	if s.isWater(row) && !aquatic && support < 0 && !(c.Kind == "pumpkin" && body >= 0 && s.Plants[body].Kind == "cattail") {
		return errors.New("水面种植需要睡莲")
	}
	if s.Map.Roof && support < 0 {
		return errors.New("屋顶种植需要花盆")
	}
	return nil
}
func (s *State) newPlant(c Spec, row, col, owner int) Plant {
	p := Plant{ID: s.id(), Owner: owner, Kind: c.Kind, Row: row, Col: col, HP: c.HP, MaxHP: c.HP, Layer: c.Layer, Asleep: c.Mushroom && !s.Map.Night, BornAt: s.Tick, AttackAt: s.Tick + 30, Flags: map[string]int64{}}
	p.ReadyAt = s.Tick + 20
	switch c.Kind {
	case "sunflower", "twinsunflower":
		p.ReadyMS = s.Tick*TickMS + s.firstSunMS()
		p.ReadyAt = counterTick(p.ReadyMS)
	case "sunshroom":
		p.ReadyMS = s.Tick*TickMS + s.firstSunMS()
		p.ReadyAt = counterTick(p.ReadyMS)
		p.EffectUntil = s.Tick + 2400
	case "potatomine":
		p.ReadyAt = s.Tick + 300
	case "chomper":
		p.ReadyAt = s.Tick
	case "cobcannon":
		p.ReadyAt = s.Tick + 100
		p.ChargeMS = (s.Tick + 100) * TickMS
	case "marigold":
		p.ReadyMS = s.Tick*TickMS + int64(s.random(2501)*10)
		p.ReadyAt = counterTick(p.ReadyMS)
	case "gravebuster":
		p.ReadyAt = s.Tick + 90
	case "blover":
		p.ReadyAt = s.Tick + 10
	}
	if c.LaunchMS > 0 {
		p.AttackMS = s.Tick*TickMS + int64(s.random(c.LaunchMS/10+1)*10)
		p.AttackAt = counterTick(p.AttackMS)
	}
	return p
}
func (s *State) plantIndex(id int) int {
	for i, p := range s.Plants {
		if p.ID == id {
			return i
		}
	}
	return -1
}
func (s *State) topPlantID(row, col int) int {
	for _, layer := range []string{"shell", "body", "support"} {
		at := s.layerAt(row, col, layer)
		if layer == "body" {
			at = s.bodyAt(row, col)
		}
		if at >= 0 {
			return s.Plants[at].ID
		}
	}
	return 0
}
func (s *State) dropUnsupported(row, col int) {
	if s.layerAt(row, col, "support") >= 0 {
		return
	}
	if !s.isWater(row) && !s.Map.Roof {
		return
	}
	for i := range s.Plants {
		p := &s.Plants[i]
		if PlantOccupies(*p, row, col) && p.Layer != "support" && p.Kind != "seashroom" && p.Kind != "tanglekelp" && p.Kind != "cattail" {
			p.HP = 0
		}
	}
}
