package engine

import (
	"errors"
	"slices"
	"strconv"
)

func Validate(s State) error {
	bad := errors.New("植物大战僵尸存档无效")
	o, e := NormalizeOptions(s.Options, len(s.Players))
	if e != nil || !slices.Equal(o.Teams, s.Options.Teams) || o.Mode != s.Options.Mode || o.Scene != s.Options.Scene || o.RoundSeconds != s.Options.RoundSeconds || o.PrepSeconds != s.Options.PrepSeconds {
		return bad
	}
	world, ok := SceneInfo(o.Scene)
	if !ok || s.Map.ID != world.ID || s.Map.Rows != world.Rows || s.Map.Cols != world.Cols || s.Map.Night != world.Night || s.Map.Fog != world.Fog || s.Map.Roof != world.Roof || !slices.Equal(s.Map.WaterRows, world.WaterRows) {
		return bad
	}
	if s.InitialRandom == 0 || s.RandomState == 0 || s.BaseEpoch == "" || len(s.BaseEpoch) > 128 || s.Round < 1 || s.Round > 2 || s.Epoch != s.BaseEpoch+":"+strconv.Itoa(s.Round) || s.Tick < 0 || s.Tick > 100000000 || !slices.Contains([]string{"prepare", "battle", "intermission", "finished"}, s.Phase) || s.WinnerTeam < -2 || s.WinnerTeam > 1 || len(s.Plants) > MaxRows*Columns*3 || len(s.Zombies) > MaxZombies || len(s.Projectiles) > MaxProjectiles || len(s.Mowers) != world.Rows || len(s.Events) > MaxEvents || len(s.Rounds) > 2 || s.NextID < 0 || s.NextID > 10000000 || s.EventSerial < 0 || len(s.Pickups) > 128 || len(s.Craters) > MaxRows*Columns || len(s.Graves) > MaxRows*Columns || len(s.Terrain) > 256 || len(s.Ladders) > MaxRows*Columns {
		return bad
	}
	if s.Phase == "finished" && (len(s.Rounds) != 2 || s.WinnerTeam == -2) || s.Phase != "finished" && s.PhaseEndTick <= s.Tick {
		return bad
	}
	for i, p := range s.Players {
		side := "zombies"
		if s.Options.Teams[i] == (s.Round-1)%2 {
			side = "plants"
		}
		if p.Team != s.Options.Teams[i] || p.Side != side || p.Money < 0 || p.Money > MaxMoney || p.Seq < 0 || p.Seq > 2147483647 || p.Cooldowns == nil || len(p.Cooldowns) > 100 || len(p.Deck) < 1 || len(p.Deck) > MaxDeck {
			return bad
		}
		seen := map[string]bool{}
		for _, k := range p.Deck {
			_, yes := PlantSpec(k)
			if side == "zombies" {
				_, yes = ZombieSpec(k)
			}
			if !yes || seen[k] {
				return bad
			}
			seen[k] = true
		}
		for k, at := range p.Cooldowns {
			_, a := PlantSpec(k)
			_, b := ZombieSpec(k)
			if !a && !b || at < 0 || at > s.Tick+180000/TickMS {
				return bad
			}
		}
	}
	for _, m := range s.TeamCooldowns {
		if m == nil || len(m) > 100 {
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
	owner := func(p int) bool { return p >= 0 && p < len(s.Players) }
	flags := func(m map[string]int64) bool {
		if len(m) > 64 {
			return false
		}
		for k, v := range m {
			if len(k) > 64 || v < -1000000000 || v > 1000000000 {
				return false
			}
		}
		return true
	}
	tiles := map[string]bool{}
	for _, p := range s.Plants {
		c, yes := PlantSpec(p.Kind)
		if p.Kind == "imitater" {
			_, targetOK := PlantSpec(p.TargetKind)
			yes = yes && targetOK
		}
		tile := strconv.Itoa(p.Row) + ":" + strconv.Itoa(p.Col) + ":" + p.Layer
		if !yes || !validID(p.ID) || !owner(p.Owner) || s.Players[p.Owner].Side != "plants" || p.Row < 0 || p.Row >= world.Rows || p.Col < 0 || p.Col >= Columns || p.HP <= 0 || p.HP > p.MaxHP || p.MaxHP != c.HP || tiles[tile] || !slices.Contains([]string{"support", "body", "shell", "instant"}, p.Layer) || !flags(p.Flags) {
			return bad
		}
		tiles[tile] = true
		if p.Kind == "cobcannon" {
			other := strconv.Itoa(p.Row) + ":" + strconv.Itoa(p.Col+1) + ":body"
			if p.Col >= Columns-1 || tiles[other] {
				return bad
			}
			tiles[other] = true
		}
	}
	for _, z := range s.Zombies {
		c, yes := ZombieSpec(z.Kind)
		if !yes || !validID(z.ID) || !owner(z.Owner) || z.Row < 0 || z.Row >= world.Rows || z.X < -3000 || z.X > 14000 || z.HP <= 0 || z.HP > z.MaxHP || z.MaxHP > c.HP || z.MaxHP <= 0 || z.Armor < 0 || z.Armor > z.MaxArmor || z.MaxArmor > c.Armor || z.SpeedMilli < 0 || z.SpeedMilli > 500000 || z.SpecialSpeedMilli < 0 || z.SpecialSpeedMilli > 500000 || z.MoveRemainder < 0 || z.MoveRemainder >= 1000 || z.BiteCarryMS < 0 || z.BiteCarryMS >= 80 || len(z.State) > 64 || !slices.Contains([]string{"plants", "zombies"}, z.Allegiance) || z.Altitude < 0 || z.Altitude > 5000 || !flags(z.Flags) {
			return bad
		}
	}
	for _, p := range s.Projectiles {
		if !validID(p.ID) || !owner(p.Owner) || p.Row < 0 || p.Row >= world.Rows || p.X < -3000 || p.X > 14000 || p.Y < -3000 || p.Y > 10000 || p.Damage < 0 || p.Damage > 100000 || len(p.Kind) > 64 || len(p.Hits) > MaxZombies {
			return bad
		}
	}
	for row, m := range s.Mowers {
		if m.Row != row || !slices.Contains([]string{"ready", "moving", "spent"}, m.State) || m.X < -500 || m.X > 11000 {
			return bad
		}
	}
	for _, p := range s.Pickups {
		if !validID(p.ID) || !owner(p.Owner) || p.Row < 0 || p.Row >= world.Rows || p.Amount <= 0 || p.Amount > MaxMoney || p.AutoAt < 0 || p.ExpiresAt < 0 {
			return bad
		}
	}
	for _, g := range s.Graves {
		if !validID(g.ID) || g.Row < 0 || g.Row >= world.Rows || g.Col < 0 || g.Col >= Columns || g.HP <= 0 {
			return bad
		}
	}
	for _, c := range s.Craters {
		if !validID(c.ID) || c.Row < 0 || c.Row >= world.Rows || c.Col < 0 || c.Col >= Columns || c.Until < 0 {
			return bad
		}
	}
	for _, v := range s.Terrain {
		if !validID(v.ID) || v.Row < 0 || v.Row >= world.Rows || v.FromX < -3000 || v.ToX > 14000 || v.FromX > v.ToX || v.Until < 0 {
			return bad
		}
	}
	for _, v := range s.Ladders {
		if v.Row < 0 || v.Row >= world.Rows || v.Col < 0 || v.Col >= Columns || !owner(v.Owner) {
			return bad
		}
	}
	return nil
}
