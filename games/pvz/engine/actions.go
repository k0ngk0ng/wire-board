package engine

import "errors"

func Apply(s *State, actor int, a Action) error {
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
	case "plant":
		c, ok := PlantSpec(a.Kind)
		if !ok || a.Row < 0 || a.Row >= Rows || a.Col < 0 || a.Col >= Columns || a.PlantID != 0 {
			return errors.New("无效植物或格子")
		}
		if p.Side != "plants" {
			return errors.New("僵尸方不能种植")
		}
		if p.Money < c.Cost {
			return errors.New("阳光不足")
		}
		if p.Cooldowns[a.Kind] > s.Tick || s.TeamCooldowns[p.Team][a.Kind] > s.Tick {
			return errors.New("卡牌尚未冷却")
		}
		for _, plant := range s.Plants {
			if plant.Row == a.Row && plant.Col == a.Col {
				return errors.New("这个格子已有植物")
			}
		}
		plant := Plant{ID: s.id(), Owner: actor, Kind: a.Kind, Row: a.Row, Col: a.Col, HP: c.HP, MaxHP: c.HP, BornAt: s.Tick, AttackAt: s.Tick + 30}
		switch a.Kind {
		case "sunflower":
			plant.ReadyMS = s.Tick*TickMS + int64((300+s.random(951))*10)
			plant.ReadyAt = counterTick(plant.ReadyMS)
		case "potatomine":
			plant.ReadyAt = s.Tick + 300
		case "cherrybomb":
			plant.ReadyAt = s.Tick + 20
		}
		if a.Kind == "peashooter" || a.Kind == "snowpea" || a.Kind == "repeater" {
			plant.AttackMS = s.Tick*TickMS + int64((136+s.random(15))*10)
			plant.AttackAt = counterTick(plant.AttackMS)
		}
		s.Plants = append(s.Plants, plant)
		p.Money -= c.Cost
		p.Cooldowns[a.Kind] = s.Tick + int64(c.Cooldown/TickMS)
		if a.Kind == "cherrybomb" {
			s.TeamCooldowns[p.Team][a.Kind] = s.Tick + 30
		}
		s.emit("plant", a.Row, PlantX(plant), actor, a.Kind, plant.ID, 0)
	case "spawn":
		c, ok := ZombieSpec(a.Kind)
		if !ok || a.Row < 0 || a.Row >= Rows || a.Col != 0 || a.PlantID != 0 {
			return errors.New("无效僵尸或进攻行")
		}
		if p.Side != "zombies" {
			return errors.New("植物方不能派出僵尸")
		}
		if s.Phase != "battle" {
			return errors.New("布阵结束后才能派出僵尸")
		}
		if p.Money < c.Cost {
			return errors.New("脑力不足")
		}
		if p.Cooldowns[a.Kind] > s.Tick {
			return errors.New("卡牌尚未冷却")
		}
		if len(s.Zombies) >= MaxZombies {
			return errors.New("场上僵尸已满")
		}
		if s.LaneCooldowns[a.Row] > s.Tick {
			return errors.New("本行入口稍后开放")
		}
		zombie := Zombie{ID: s.id(), Owner: actor, Kind: a.Kind, Row: a.Row, X: 9500, HP: c.HP, MaxHP: c.HP, Armor: c.Armor, MaxArmor: c.Armor, State: "walking", BornAt: s.Tick}
		// Original movement follows the animation _ground track. The independent
		// versus engine preserves its verified cycle-average speed with sub-unit
		// accumulation, rather than claiming to replay that per-frame track.
		zombie.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
		switch a.Kind {
		case "football":
			zombie.SpeedMilli = (660 + s.random(21)) * 29375 / 1000
		case "polevaulter":
			zombie.SpecialSpeedMilli = (660 + s.random(21)) * 29375 / 1000
		case "newspaper":
			zombie.SpecialSpeedMilli = (890 + s.random(21)) * 29375 / 1000
		}
		s.Zombies = append(s.Zombies, zombie)
		p.Money -= c.Cost
		p.Cooldowns[a.Kind] = s.Tick + int64(c.Cooldown/TickMS)
		s.LaneCooldowns[a.Row] = s.Tick + 8
		s.emit("spawn", a.Row, 9500, actor, a.Kind, s.NextID, 0)
	case "shovel":
		if p.Side != "plants" || a.PlantID <= 0 || a.Kind != "" {
			return errors.New("当前不能铲除")
		}
		at := -1
		for i, plant := range s.Plants {
			if plant.ID == a.PlantID {
				at = i
				break
			}
		}
		if at < 0 || s.Plants[at].Owner != actor {
			return errors.New("只能铲除自己的植物")
		}
		plant := s.Plants[at]
		s.Plants = append(s.Plants[:at], s.Plants[at+1:]...)
		s.emit("shovel", plant.Row, PlantX(plant), actor, plant.Kind, plant.ID, 0)
	default:
		return errors.New("未知操作")
	}
	p.Seq = a.Seq
	return nil
}
