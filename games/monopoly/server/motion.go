package monopoly

// Motion is a bounded public presentation log, independent of game decisions.
// Signed steps distinguish reverse cards and passing GO; jail is a direct jump.
type Motion struct {
	ID     int    `json:"id"`
	Kind   string `json:"kind"`
	Player int    `json:"player"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Steps  int    `json:"steps"`
	Dice   [2]int `json:"dice"`
}

func (s *State) animate(kind string, player, from, to, steps int) {
	s.MotionSerial++
	s.Motion = append(s.Motion, Motion{ID: s.MotionSerial, Kind: kind, Player: player, From: from, To: to, Steps: steps, Dice: s.Dice})
	if len(s.Motion) > 24 {
		s.Motion = append([]Motion(nil), s.Motion[len(s.Motion)-24:]...)
	}
}
