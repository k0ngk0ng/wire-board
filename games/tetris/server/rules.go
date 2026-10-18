package tetris

import (
	"math"
	"strings"
)

var baseShapes = map[Kind][]string{
	I: {"....", "####", "....", "...."},
	J: {"#..", "###", "..."},
	L: {"..#", "###", "..."},
	O: {"##", "##"},
	S: {".##", "##.", "..."},
	T: {".#.", "###", "..."},
	Z: {"##.", ".##", "..."},
}

var shapeCache [9][4][]Point

func init() {
	for k := I; k <= Z; k++ {
		rows := baseShapes[k]
		for r := 0; r < 4; r++ {
			var cells []Point
			for y, row := range rows {
				for x := 0; x < len(row); x++ {
					if row[x] == '#' {
						cells = append(cells, Point{X: x, Y: y})
					}
				}
			}
			shapeCache[k][r] = cells
			rows = rotateCW(rows)
		}
	}
}

func rotateCW(rows []string) []string {
	n := len(rows)
	out := make([][]byte, n)
	for i := range out {
		out[i] = make([]byte, n)
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			out[x][n-1-y] = rows[y][x]
		}
	}
	res := make([]string, n)
	for i := range out {
		res[i] = string(out[i])
	}
	return res
}

func shapeCells(kind Kind, rot int) []Point {
	if kind < I || kind > Z {
		return nil
	}
	return shapeCache[kind][((rot%4)+4)%4]
}

func emptyRow() string { return strings.Repeat("0", BoardWidth) }

func emptyBoard() (b [BoardRows]string) {
	for i := range b {
		b[i] = emptyRow()
	}
	return b
}

func cellAt(b [BoardRows]string, x, y int) Kind {
	if x < 0 || x >= BoardWidth || y < 0 || y >= BoardRows {
		return Empty
	}
	return Kind(b[y][x] - '0')
}

func setCell(b *[BoardRows]string, x, y int, k Kind) {
	if x < 0 || x >= BoardWidth || y < 0 || y >= BoardRows {
		return
	}
	row := []byte(b[y])
	row[x] = byte('0' + int(k))
	b[y] = string(row)
}

func fits(board [BoardRows]string, kind Kind, rot, x, y int) bool {
	for _, pt := range shapeCells(kind, rot) {
		bx, by := x+pt.X, y+pt.Y
		if bx < 0 || bx >= BoardWidth || by < 0 || by >= BoardRows {
			return false
		}
		if board[by][bx] != '0' {
			return false
		}
	}
	return true
}

func dropY(board [BoardRows]string, kind Kind, rot, x, y int) int {
	if !fits(board, kind, rot, x, y) {
		return y
	}
	for fits(board, kind, rot, x, y+1) {
		y++
	}
	return y
}

func fullRow(row string) bool { return !strings.ContainsRune(row, '0') }

func clearFullRows(board *[BoardRows]string) []int {
	full := []int{}
	for y := 0; y < BoardRows; y++ {
		if fullRow(board[y]) {
			full = append(full, y)
		}
	}
	if len(full) == 0 {
		return nil
	}
	var out [BoardRows]string
	write := BoardRows - 1
	for y := BoardRows - 1; y >= 0; y-- {
		if fullRow(board[y]) {
			continue
		}
		out[write] = board[y]
		write--
	}
	for ; write >= 0; write-- {
		out[write] = emptyRow()
	}
	*board = out
	return full
}

func garbageRow(hole int) string {
	if hole < 0 || hole >= BoardWidth {
		hole = 0
	}
	row := []byte(strings.Repeat("8", BoardWidth))
	row[hole] = '0'
	return string(row)
}

// insertGarbage adds one row per hole from the bottom, pushing the existing
// stack upward. A non-empty top hidden row means the insertion would push a
// block out of the board, so the player tops out.
func insertGarbage(board *[BoardRows]string, holes []int) bool {
	for _, hole := range holes {
		if board[0] != emptyRow() {
			return true
		}
		for y := 0; y < BoardRows-1; y++ {
			board[y] = board[y+1]
		}
		board[BoardRows-1] = garbageRow(hole)
	}
	return false
}

func scoreForLines(lines, level int) int {
	if lines < 0 {
		return 0
	}
	base := []int{0, 100, 300, 500, 800}
	if lines >= len(base) {
		lines = len(base) - 1
	}
	if level < 1 {
		level = 1
	}
	return base[lines] * level
}

func levelForLines(lines int) int {
	if lines < 0 {
		return 1
	}
	return 1 + lines/10
}

func gravityFor(level int) int64 {
	if level < 1 {
		level = 1
	}
	ms := 800 - (level-1)*60
	if ms < 100 {
		ms = 100
	}
	return int64(ms)
}

func attackFor(lines int) int {
	switch {
	case lines >= 4:
		return 4
	case lines == 3:
		return 2
	case lines == 2:
		return 1
	default:
		return 0
	}
}

func nextRandom(state *uint64) uint64 {
	*state += 0x9e3779b97f4a7c15
	z := *state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func mixSeed(seed uint64, seat int) uint64 {
	return seed ^ (uint64(seat+1) * 0x9e3779b97f4a7c15)
}

func nextBag(p *Player) []Kind {
	bag := []Kind{I, J, L, O, S, T, Z}
	for i := len(bag) - 1; i > 0; i-- {
		j := int(nextRandom(&p.Rand) % uint64(i+1))
		bag[i], bag[j] = bag[j], bag[i]
	}
	return bag
}

func refillQueue(p *Player) {
	for len(p.Queue) < 5 {
		if len(p.Bag) == 0 {
			p.Bag = nextBag(p)
		}
		p.Queue = append(p.Queue, p.Bag[0])
		p.Bag = p.Bag[1:]
	}
}

func newPlayer(seat int, seed uint64) Player {
	p := Player{
		Seat: seat, Alive: true, Level: 1, Board: emptyBoard(),
		Rand: mixSeed(seed, seat), TargetSeat: -1,
	}
	refillQueue(&p)
	return p
}

func spawnX(kind Kind) int {
	if kind == O {
		return 4
	}
	return 3
}

func spawnKind(p *Player, kind Kind, now int64) bool {
	p.PieceID++
	p.Active = &Piece{ID: p.PieceID, Kind: kind, Rot: 0, X: spawnX(kind), Y: 1, SpawnedAt: now}
	p.FallAt = now + gravityFor(p.Level)
	p.LockAt = 0
	p.LockResets = 0
	return fits(p.Board, kind, 0, p.Active.X, p.Active.Y)
}

func spawn(p *Player, now int64) bool {
	if len(p.Queue) == 0 {
		refillQueue(p)
	}
	kind := p.Queue[0]
	p.Queue = p.Queue[1:]
	refillQueue(p)
	return spawnKind(p, kind, now)
}

// resetLock clears or extends the lock delay after a successful player move.
// Once the reset budget is exhausted the piece locks on the existing deadline.
func resetLock(p *Player, now int64) {
	if p.Active == nil {
		return
	}
	if fits(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y+1) {
		p.LockAt = 0
		p.LockResets = 0
		return
	}
	if p.LockAt > 0 && p.LockResets < MaxLockResets {
		p.LockAt = now + LockDelayMS
		p.LockResets++
	}
}

func tryMove(p *Player, dx int) bool {
	if p.Active == nil {
		return false
	}
	if !fits(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X+dx, p.Active.Y) {
		return false
	}
	p.Active.X += dx
	return true
}

type kick struct{ X, Y int }

var kicksJLSTZ = map[int][]kick{
	0*4 + 1: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	1*4 + 0: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	1*4 + 2: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	2*4 + 1: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	2*4 + 3: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
	3*4 + 2: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	3*4 + 0: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	0*4 + 3: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
}

var kicksI = map[int][]kick{
	0*4 + 1: {{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
	1*4 + 0: {{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
	1*4 + 2: {{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
	2*4 + 1: {{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
	2*4 + 3: {{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
	3*4 + 2: {{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
	3*4 + 0: {{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
	0*4 + 3: {{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
}

func tryRotate(p *Player, dir int) bool {
	if p.Active == nil || p.Active.Kind == O {
		return false
	}
	target := (p.Active.Rot + dir + 4) % 4
	table := kicksJLSTZ
	if p.Active.Kind == I {
		table = kicksI
	}
	for _, k := range table[p.Active.Rot*4+target] {
		x, y := p.Active.X+k.X, p.Active.Y+k.Y
		if fits(p.Board, p.Active.Kind, target, x, y) {
			p.Active.Rot = target
			p.Active.X = x
			p.Active.Y = y
			return true
		}
	}
	return false
}

func visibleBoard(b [BoardRows]string) []string {
	out := make([]string, VisibleRows)
	copy(out, b[HiddenRows:])
	return out
}

func nextAliveTarget(s *State, seat, after int) int {
	n := len(s.Players)
	for step := 1; step <= n; step++ {
		j := (after + step) % n
		if j == seat {
			continue
		}
		if s.Players[j].Alive {
			return j
		}
	}
	return -1
}

func normalizeTargets(s *State) {
	for i := range s.Players {
		if !s.Players[i].Alive {
			s.Players[i].TargetSeat = -1
			continue
		}
		s.Players[i].TargetSeat = nextAliveTarget(s, i, i)
	}
}

func advanceTarget(s *State, p *Player) {
	if p.TargetSeat < 0 {
		p.TargetSeat = nextAliveTarget(s, p.Seat, p.Seat)
		return
	}
	p.TargetSeat = nextAliveTarget(s, p.Seat, p.TargetSeat)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(a int) int {
	return int(math.Abs(float64(a)))
}
