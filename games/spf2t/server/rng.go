package spf2t

// splitmix64 is the same deterministic generator used by the tetris module:
// no math/rand global state, no time, and a persisted per-seat word.
func splitmix64(state *uint64) uint64 {
	*state += 0x9e3779b97f4a7c15
	z := *state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func newPlayer(seat int, seed uint64) Player {
	rand := seed ^ (uint64(seat)+1)*0x9e3779b97f4a7c15
	if rand == 0 {
		rand = 0x9e3779b97f4a7c15
	}
	return Player{Seat: seat, RoundWins: 0, Rand: rand}
}

var gemColors = []string{ColorRed, ColorYellow, ColorGreen, ColorBlue}

func randomColor(p *Player) string {
	return gemColors[int(splitmix64(&p.Rand)%uint64(len(gemColors)))]
}

// generatePair creates the next falling pair. Roughly one pair in ten contains
// a crash gem (the arcade pairs are not two independent 10% rolls). Every 25th
// pair replaces the second gem with a diamond.
func generatePair(p *Player) Pair {
	p.PairCount++
	p.PairID++
	pair := Pair{ID: p.PairID}
	pair.A = GemSpec{Kind: GemNormal, Color: randomColor(p)}
	pair.B = GemSpec{Kind: GemNormal, Color: randomColor(p)}
	if p.PairCount%25 == 0 {
		pair.B = GemSpec{Kind: GemDiamond}
		return pair
	}
	if splitmix64(&p.Rand)%10 == 0 {
		if splitmix64(&p.Rand)%2 == 0 {
			pair.A.Kind = GemCrash
		} else {
			pair.B.Kind = GemCrash
		}
	}
	return pair
}

func randomCharacter(p *Player) string {
	return characterDefs[int(splitmix64(&p.Rand)%uint64(len(characterDefs)))].ID
}

// botCharacter is a pure choice: bots must not consume the match RNG, and the
// same epoch/seat must choose the same character after a restart.
func botCharacter(epoch string, seat int) string {
	var h uint64 = 1469598103934665603
	write := func(s string) {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
	}
	write(epoch)
	write(string(rune('0' + seat)))
	write("spf2t")
	return characterDefs[int(h%uint64(len(characterDefs)))].ID
}
