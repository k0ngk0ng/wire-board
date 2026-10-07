package game

// Paired records the two markers, independently of the player currently acting.
// Both action phases belong to one production turn (including Helpers locks).
// Explorer uses a distinct action serial for each portion; its own controller
// tracks production counts and does not call catanNextPaired.
type CatanPairedTurn struct {
	Primary   int  `json:"primary"`
	Secondary int  `json:"secondary"`
	Second    bool `json:"second"`
}

func (g *Catan) pairedPartner(primary int) int {
	alive := []int{}
	for step := 1; step < len(g.Players); step++ {
		player := (primary + step) % len(g.Players)
		if !g.Players[player].Eliminated {
			alive = append(alive, player)
		}
	}
	if len(alive) == 0 {
		return primary
	}
	return alive[min(2, len(alive)-1)]
}

func (s *State) catanNextPaired() {
	g := s.Catan
	pair := g.Paired
	if !pair.Second && pair.Secondary != pair.Primary && !g.Players[pair.Secondary].Eliminated {
		pair.Second = true
		s.Turn = pair.Secondary
		s.Phase = "catan_turn"
		s.catanLog(s.Turn, "开始配对行动：无需掷骰，可以建造、使用发展卡及银行/港口交易")
	} else {
		primary := pair.Primary
		for {
			primary = (primary + 1) % len(g.Players)
			if primary == g.StartPlayer {
				s.Round++
			}
			if !g.Players[primary].Eliminated {
				break
			}
		}
		pair.Primary = primary
		pair.Secondary = g.pairedPartner(primary)
		pair.Second = false
		g.TurnSerial++
		s.Turn = primary
		s.Phase = "catan_roll"
	}
	// Cards bought in the player's previous action phase are now playable,
	// even when that previous phase used the other paired-player marker.
	g.Players[s.Turn].NewDev = make([]int, 5)
}
