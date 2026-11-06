package engine

import (
	"errors"
	"slices"
)

func Validate(s State) error {
	bad := errors.New("植物大战僵尸存档无效")
	o, e := NormalizeOptions(s.Options, len(s.Players))
	if e != nil || !slices.Equal(o.Teams, s.Options.Teams) || o.Mode != s.Options.Mode || o.RoundSeconds != s.Options.RoundSeconds || o.PrepSeconds != s.Options.PrepSeconds {
		return bad
	}
	if s.InitialRandom == 0 || s.RandomState == 0 || s.BaseEpoch == "" || len(s.BaseEpoch) > 128 || s.Epoch == "" || s.Round < 1 || s.Round > 2 || s.Tick < 0 || s.Tick > 100000000 || !slices.Contains([]string{"prepare", "battle", "intermission", "finished"}, s.Phase) || s.WinnerTeam < -2 || s.WinnerTeam > 1 || len(s.Plants) > Rows*Columns || len(s.Zombies) > MaxZombies || len(s.Projectiles) > MaxProjectiles || len(s.Mowers) != Rows || len(s.Events) > MaxEvents || len(s.Rounds) > 2 || s.NextID < 0 || s.NextID > 10000000 || s.EventSerial < 0 {
		return bad
	}
	if s.Phase == "finished" && (len(s.Rounds) != 2 || s.WinnerTeam == -2) || s.Phase != "finished" && s.PhaseEndTick <= s.Tick {
		return bad
	}
	for p, player := range s.Players {
		side := "zombies"
		if s.Options.Teams[p] == (s.Round-1)%2 {
			side = "plants"
		}
		if player.Team != s.Options.Teams[p] || player.Side != side || player.Money < 0 || player.Money > MaxMoney || player.Seq < 0 || player.Seq > 2147483647 || player.Cooldowns == nil {
			return bad
		}
		for k, at := range player.Cooldowns {
			_, plant := PlantSpec(k)
			_, zombie := ZombieSpec(k)
			if !plant && !zombie || at < 0 || at > s.Tick+120000/TickMS {
				return bad
			}
		}
	}
	for _, m := range s.TeamCooldowns {
		if m == nil {
			return bad
		}
	}
	ids := map[int]bool{}
	validID := func(id int) bool {
		if id <= 0 || id > s.NextID || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	owner := func(p int, side string) bool { return p >= 0 && p < len(s.Players) && s.Players[p].Side == side }
	tiles := map[int]bool{}
	for _, p := range s.Plants {
		c, ok := PlantSpec(p.Kind)
		tile := p.Row*Columns + p.Col
		if !ok || !validID(p.ID) || !owner(p.Owner, "plants") || p.Row < 0 || p.Row >= Rows || p.Col < 0 || p.Col >= Columns || p.HP <= 0 || p.HP > c.HP || p.MaxHP != c.HP || tiles[tile] {
			return bad
		}
		tiles[tile] = true
	}
	for _, z := range s.Zombies {
		c, ok := ZombieSpec(z.Kind)
		if !ok || !validID(z.ID) || !owner(z.Owner, "zombies") || z.Row < 0 || z.Row >= Rows || z.X < -1000 || z.X > 10000 || z.HP <= 0 || z.HP > c.HP || z.MaxHP != c.HP || z.Armor < 0 || z.Armor > c.Armor || z.MaxArmor != c.Armor || z.SpeedMilli < 6000 || z.SpeedMilli > 30000 || z.SpecialSpeedMilli < 0 || z.SpecialSpeedMilli > 30000 || z.MoveRemainder < 0 || z.MoveRemainder >= 1000 || z.BiteCarryMS < 0 || z.BiteCarryMS >= 80 || !slices.Contains([]string{"walking", "eating", "angry", "vaulting"}, z.State) {
			return bad
		}
	}
	for _, p := range s.Projectiles {
		if !validID(p.ID) || !owner(p.Owner, "plants") || p.Row < 0 || p.Row >= Rows || p.X < 0 || p.X > 10000 || p.Damage != 20 || p.Kind != "pea" && p.Kind != "icepea" {
			return bad
		}
	}
	for row, m := range s.Mowers {
		if m.Row != row || !slices.Contains([]string{"ready", "moving", "spent"}, m.State) || m.X < -500 || m.X > 11000 {
			return bad
		}
	}
	return nil
}
