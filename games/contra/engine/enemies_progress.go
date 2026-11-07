package contra

// BossDoorSignal is a source-level progression command, not a match result.
// The world applies it immediately to its existing World/Campaign state.
type BossDoorSignal uint8

const (
	BossDoorNoSignal BossDoorSignal = iota
	BossDoorDefeated
	BossDoorTunnelFinished
	BossBaseFinished
)

// BossControl is applied synchronously at the original instruction boundary.
// Audio reset is independent: some bosses silence audio before final defeat.
type BossControl struct {
	Kind  BossControlKind
	Delay uint8
}

type BossControlKind uint8

const (
	BossBeginDefeat BossControlKind = iota + 1
	BossExitReady
	BossAudioReset
)

// EnemySceneContext points to the world's unique scene/palette state. Enemy
// rules must not retain a second copy of these mutable values between frames.
type EnemySceneContext struct {
	BossScrollComplete       *uint8
	Palette                  *PaletteOutput
	Header                   *PaletteState
	RefreshBackgroundPalette func(uint8) (PaletteResult, error)
}

// EnemyAudioPrefix contains only the effective requests before a source sound
// call, after the most recent reset in this enemy phase. World projects these
// onto a safe copy of its phase-entry audio state to recover the call's carry;
// the normal enemy result remains the sole source of actual sound requests.
type EnemyAudioPrefix struct {
	Reset  bool
	Sounds []uint8
}
