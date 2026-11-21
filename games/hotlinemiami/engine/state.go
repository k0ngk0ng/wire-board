package engine

import "errors"

func New(options Options, players int, seed uint64) (*State, error) {
	return NewWithMap(options, players, seed, MapByID(options.Map))
}

func NewWithMap(options Options, players int, seed uint64, world Map) (*State, error) {
	if players < 2 || players > 4 || world.ID == "" || world.Width < 64*Unit || world.Height < 64*Unit || world.Width > 8192*Unit || world.Height > 8192*Unit || len(world.Spawns) < players {
		return nil, errors.New("无效的混战人数或地图")
	}
	if options.RoundSeconds == 0 {
		options.RoundSeconds = 180
	}
	if options.RoundSeconds < 1 || options.RoundSeconds > 600 {
		return nil, errors.New("无效比赛时长")
	}
	if options.Map == "" {
		options.Map = world.ID
	}
	if options.Map != world.ID {
		return nil, errors.New("地图不匹配")
	}
	characters := make([]string, players)
	weapons := make([]string, players)
	for seat := 0; seat < players; seat++ {
		characters[seat] = Characters()[seat%len(Characters())]
		weapons[seat] = "pistol"
		if seat < len(options.Characters) {
			characters[seat] = options.Characters[seat]
		}
		if seat < len(options.Weapons) {
			weapons[seat] = options.Weapons[seat]
		}
		if !validCharacter(characters[seat]) {
			return nil, errors.New("无效人物")
		}
		if _, ok := WeaponByID(weapons[seat]); !ok {
			return nil, errors.New("无效武器")
		}
	}
	options.Characters, options.Weapons = characters, weapons
	s := &State{Options: options, MapID: world.ID, EndFrame: int64(options.RoundSeconds * FPS), Seed: seed, World: world, Players: make([]Player, players), Bullets: []Bullet{}, Drops: []Drop{}, Doors: []Door{}, BrokenGlass: []int{}, Corpses: []Corpse{}, Events: []Event{}, WeaponTimers: make([]int64, len(world.WeaponSpawns))}
	if s.Seed == 0 {
		s.Seed = 1
	}
	for _, door := range world.Doors {
		s.Doors = append(s.Doors, Door{ID: door.ID})
	}
	for seat := range s.Players {
		s.Players[seat] = Player{Seat: seat, Character: characters[seat], Life: 1, SpawnWeapon: weapons[seat]}
		if !s.spawn(seat, false) {
			return nil, errors.New("地图缺少有效出生位置")
		}
	}
	s.refreshWeapons()
	s.emit("start", -1, -1, 0, 0, "")
	s.recordHistory()
	return s, nil
}

func (s *State) world() Map {
	if s.World.ID != "" {
		return s.World
	}
	return MapByID(s.MapID)
}

func (s *State) random() uint64 {
	x := s.Seed
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	s.Seed = x
	return x
}

func (s *State) nextID() int {
	s.NextID++
	return s.NextID
}

func (s *State) emit(kind string, seat, target, x, y int, weapon string) {
	s.EventID++
	e := Event{ID: s.EventID, Frame: s.Frame, Kind: kind, Seat: seat, Target: target, X: x, Y: y, Weapon: weapon}
	if seat >= 0 && seat < len(s.Players) {
		e.Life = s.Players[seat].Life
	}
	s.Events = append(s.Events, e)
	if len(s.Events) > MaxEvents {
		copy(s.Events, s.Events[len(s.Events)-MaxEvents:])
		s.Events = s.Events[:MaxEvents]
	}
}

func QueueInput(s *State, seat int, in Input) error {
	if s.Finished || seat < 0 || seat >= len(s.Players) {
		return errors.New("当前不能操作")
	}
	p := &s.Players[seat]
	if in.Seq <= p.ReceivedSeq || in.Seq <= 0 || in.Seq > MaxInputSeq || in.Seq-p.ReceivedSeq > 4096 {
		return errors.New("输入序号已过期或超出范围")
	}
	if in.Life != p.Life {
		return errors.New("角色生命已更新")
	}
	if in.ViewFrame < 0 || in.ViewFrame > s.Frame {
		return errors.New("无效的画面帧")
	}
	if in.ViewFrame > 0 && in.ViewFrame < s.Frame-6 {
		in.ViewFrame = 0
	}
	if in.ViewFrame > 0 {
		past := s.historyAt(in.ViewFrame)
		if past == nil || seat >= len(past.Players) || past.Players[seat].Life != p.Life {
			in.ViewFrame = 0
		}
	}
	if in.MoveX < -1000 || in.MoveX > 1000 || in.MoveY < -1000 || in.MoveY > 1000 || in.Aim < 0 || in.Aim >= 1024 {
		return errors.New("无效方向")
	}
	if len(p.Pending) >= MaxPending {
		return errors.New("输入窗口已满")
	}
	p.ReceivedSeq = in.Seq
	p.LastInputFrame = s.Frame
	p.Pending = append(p.Pending, in)
	return nil
}

func ClearInput(s *State, seat int) {
	if seat < 0 || seat >= len(s.Players) {
		return
	}
	p := &s.Players[seat]
	p.Pending = nil
	p.Input = Input{Seq: p.ReceivedSeq, Life: p.Life, Aim: p.Aim}
	p.AppliedSeq = p.ReceivedSeq
	p.LastInputFrame = s.Frame
}

func (s *State) spawn(seat int, increment bool) bool {
	world := s.world()
	best, bestSafety := -1, int64(-1)
	for i, point := range world.Spawns {
		if blocked(world, s.Doors, s.BrokenGlass, point.X, point.Y) {
			continue
		}
		safety := int64(1 << 60)
		for _, other := range s.Players {
			if other.Seat == seat || !other.Alive {
				continue
			}
			d := distance2(point.X, point.Y, other.X, other.Y)
			if Visible(world, s.Doors, s.BrokenGlass, point.X, point.Y, other.X, other.Y) {
				d /= 4
			}
			safety = min(safety, d)
		}
		if safety > bestSafety {
			best, bestSafety = i, safety
		}
	}
	if best < 0 {
		return false
	}
	p := &s.Players[seat]
	if increment {
		p.Life++
	}
	p.X, p.Y = world.Spawns[best].X, world.Spawns[best].Y
	p.Alive = true
	p.DownUntil = 0
	p.RespawnFrame = 0
	p.ProtectedUntil = s.Frame + 48
	p.NextAttackFrame = s.Frame
	p.BusyUntil = 0
	p.Weapon = p.SpawnWeapon
	w, _ := WeaponByID(p.Weapon)
	p.Ammo = w.Ammo
	ClearInput(s, seat)
	s.emit("spawn", seat, -1, p.X, p.Y, p.Weapon)
	return true
}

func (s *State) addDrop(drop Drop) {
	if drop.Weapon == "" {
		return
	}
	if drop.ID == 0 {
		drop.ID = s.nextID()
	}
	drop.Frame = s.Frame
	s.Drops = append(s.Drops, drop)
	if len(s.Drops) > MaxDrops {
		remove := 0
		for i, d := range s.Drops {
			if d.SpawnID == 0 {
				remove = i
				break
			}
		}
		s.Drops = append(s.Drops[:remove], s.Drops[remove+1:]...)
	}
}

func (s *State) refreshWeapons() {
	world := s.world()
	for i, point := range world.WeaponSpawns {
		if i >= len(s.WeaponTimers) || s.WeaponTimers[i] > s.Frame {
			continue
		}
		found := false
		for _, drop := range s.Drops {
			if drop.SpawnID == i+1 {
				found = true
				break
			}
		}
		if found {
			continue
		}
		weapon, ok := WeaponByID(point.Weapon)
		if ok {
			s.addDrop(Drop{X: point.X, Y: point.Y, Weapon: point.Weapon, Ammo: weapon.Ammo, SpawnID: i + 1})
		}
	}
}

func Advance(s *State, targetFrame int64) {
	for s.Frame < targetFrame && !s.Finished {
		Step(s)
	}
}

func Step(s *State) {
	if s.Finished {
		return
	}
	s.Frame++
	if s.Frame >= s.EndFrame {
		s.Finished = true
		for seat := range s.Players {
			ClearInput(s, seat)
		}
		s.emit("end", -1, -1, 0, 0, "")
		return
	}
	world := s.world()
	if world.ID == "" {
		return
	}
	hits := []damage{}
	for seat := range s.Players {
		p := &s.Players[seat]
		if !p.Alive {
			ClearInput(s, seat)
			if s.Frame >= p.RespawnFrame {
				s.spawn(seat, true)
			}
			continue
		}
		fire := len(p.Pending) == 0 && p.Input.Fire
		fireFrame := p.Input.ViewFrame
		commands := p.Pending
		p.Pending = nil
		for _, in := range commands {
			p.AppliedSeq = in.Seq
			if in.Life != p.Life {
				continue
			}
			p.Input = in
			p.Aim = in.Aim
			fire = fire || in.Fire
			if in.Fire {
				fireFrame = in.ViewFrame
			}
			if p.DownUntil > s.Frame || p.BusyUntil > s.Frame {
				continue
			}
			if in.Interact {
				s.interact(seat)
			}
			if in.Pickup {
				s.pickup(seat)
			}
			if in.Throw {
				s.throw(seat)
			}
			if in.Finish {
				hits = append(hits, s.finish(seat)...)
			}
		}
		if s.Frame-p.LastInputFrame > 15 {
			p.Input.MoveX = 0
			p.Input.MoveY = 0
			p.Input.Fire = false
			fire = false
		}
		if p.DownUntil > s.Frame || p.BusyUntil > s.Frame {
			continue
		}
		p.DownUntil = 0
		p.X, p.Y, p.Aim = PredictMotion(world, p.X, p.Y, p.Input.Aim, p.Input.MoveX, p.Input.MoveY, 1, s.Doors, s.BrokenGlass)
		if fire {
			hits = append(hits, s.attack(seat, fireFrame)...)
		}
	}
	hits = append(hits, s.advanceBullets()...)
	s.settle(hits)
	s.refreshWeapons()
	corpses := s.Corpses[:0]
	for _, corpse := range s.Corpses {
		if s.Frame-corpse.Frame < 1800 {
			corpses = append(corpses, corpse)
		}
	}
	s.Corpses = corpses
	s.recordHistory()
}
