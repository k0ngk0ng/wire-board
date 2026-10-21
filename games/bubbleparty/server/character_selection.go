package bubbleparty

func validPlayerAbilities(version int, p *Player) bool {
	if !validMount(p.Mount) {
		return false
	}
	if version < NativeRulesVersion {
		return p.SpeedMS >= MinMoveMS && p.SpeedMS <= BaseMoveMS && p.MaxBombs >= 1 && p.MaxBombs <= MaxBombCapacity && p.Range >= 1 && p.Range <= MaxBombRange
	}
	profile, ok := characterProfile(p.Character)
	return ok && !profile.Hidden && p.NativeSpeed >= profile.InitialSpeed && p.NativeSpeed <= profile.MaximumSpeed && p.MaxBombs >= profile.InitialBombs && p.MaxBombs <= profile.MaximumBombs && p.Range >= profile.InitialRange && p.Range <= profile.MaximumRange && p.SpeedMS > 0 && p.SpeedMS <= 600
}
func assignCharacter(p *Player, id int) {
	profile, _ := characterProfile(id)
	p.Character = id
	p.NativeSpeed = profile.InitialSpeed
	p.SpeedMS = characterMoveMS(profile.InitialSpeed)
	p.MaxBombs = profile.InitialBombs
	p.Range = profile.InitialRange
	p.Mount = ""
}
func (s *State) lockCharacters(at int64) {
	if s.Phase != PhaseSelect {
		return
	}
	for i := range s.Players {
		s.Players[i].Selected = true
	}
	s.Phase = PhaseCountdown
	s.ServerAt = at
	s.StartAt = at + Countdown
	s.MatchEndsAt = s.StartAt + int64(s.Options.RoundSeconds)*1000
	s.log("角色已确定，3 秒后开始")
}
func (s *State) selectCharacter(seat, id int, confirmed bool, at int64) {
	p := &s.Players[seat]
	assignCharacter(p, id)
	p.Selected = confirmed
	s.ServerAt = at
	all := true
	for _, p := range s.Players {
		all = all && p.Selected
	}
	if all {
		s.lockCharacters(at)
	}
}
