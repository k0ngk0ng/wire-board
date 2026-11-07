package contra

// BotObservation is the complete strategy boundary. It contains no pointer to
// authoritative rules, enemy HP/AI timers, random state or future spawn data.
// Velocity is inferred by Bot from successive public positions, never copied
// from a hostile entity's hidden movement fields.
type BotPlayer struct {
	X, Y                                int
	Alive, Visible                      bool
	Lives, Weapon, Sprite               uint8
	Appearance                          uint8
	LivesKnown, WeaponKnown, AliveKnown bool
	GameOverKnown, GameOver             bool
}
type BotEnemy struct {
	ID                       int
	X, Y                     int
	Kind, Sprite, Appearance uint8
	Family                   string
}
type BotPlayerCandidate struct {
	Mask   uint8
	Exact  bool
	Player BotPlayer
}
type BotObservation struct {
	Tick                                             uint64
	Actor                                            int
	Seats                                            int
	Stage                                            uint8
	Phase                                            string
	Scene                                            uint8
	Vertical, Paused                                 bool
	Choice                                           uint8
	StageKnown, SceneKnown, ChoiceKnown, PausedKnown bool
	ViewX, ViewY                                     uint8
	GridKnown                                        bool
	Players                                          [2]BotPlayer
	Enemies                                          []BotEnemy
	PlayerCandidates                                 []BotPlayerCandidate
	// Exact visible cell geometry, including partial cells at viewport edges.
	// No source byte/index, hidden map half or next-screen data is exposed.
	Terrain      [15][17]TerrainKind
	TerrainKnown [15][17]bool
}

func (o *BotObservation) terrainAt(x, y int) TerrainKind {
	if x < 0 || x >= 256 || y < 0 || y >= 224 {
		return TerrainEmpty
	}
	return o.Terrain[(y+int(o.ViewY&15))/16][(x+int(o.ViewX&15))/16]
}

func (o *BotObservation) supportAt(x, y int) TerrainKind {
	if kind := o.terrainAt(x, y); kind != TerrainEmpty {
		return kind
	}
	if o.Stage == 2 {
		for _, e := range o.Enemies {
			if e.Kind == 16 && e.Sprite == 72 && absBot(x-e.X) < 16 && y >= e.Y-16 && y < e.Y {
				return TerrainPlatform
			}
		}
	}
	return TerrainEmpty
}

func (o *BotObservation) terrainKnownAt(x, y int) bool {
	if x < 0 || x >= 256 || y < 0 || y >= 224 {
		return false
	}
	return o.TerrainKnown[(y+int(o.ViewY&15))/16][(x+int(o.ViewX&15))/16]
}
