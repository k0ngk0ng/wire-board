package contra

// Campaign owns progression, continue resources and transition timing. Level,
// LevelRoutine, GameRoutine and BossDefeated remain authoritative in OutdoorWorld.
// A caller runs the returned work on one World copy and commits only when all
// work for the tick has completed. Savepoints must not contain Pending work.
type Campaign struct {
	Continues, DelayLow, DelayHigh, GameOverDelay uint8
	Choice, EndSequence, Loops, AliveAtExit       uint8
	Paused                                        bool
	Outcome                                       CampaignOutcome
	CommandSerial, LastTick                       uint64
	HasTick                                       bool
	Pending                                       CampaignWork
	OriginRoutine                                 uint8
}

type CampaignOutcome string

const (
	CampaignOngoing CampaignOutcome = ""
	CampaignCleared CampaignOutcome = "cleared"
	CampaignFailed  CampaignOutcome = "failed"
)

type CampaignWork uint8

const (
	CampaignNoWork CampaignWork = iota
	CampaignCombat
	CampaignEnemies
	CampaignExitEnemies
	CampaignInitializeLevel
	CampaignLevelIntro
	CampaignPrepareBattle
	CampaignLevelEntrance
	CampaignRunEndSequence
	CampaignRunInterlude
)

type CampaignPlayer struct{ State, GameOver, Lives, Weapon uint8 }

type CampaignInput struct {
	Pressed      Buttons // Original player-one edges, after platform authorization.
	PauseBlocked bool    // Original demo/secondary pause/PPU-ready guard aggregate.
	CheatLives   bool    // Only an explicitly enabled, unrated original variant.
	Players      [2]CampaignPlayer
}

type CampaignResetKind uint8

const (
	CampaignResetAdvance CampaignResetKind = iota + 1
	CampaignResetGameOver
	CampaignResetContinue
)

// CampaignReset is a command payload, not a direct write to another subsystem.
// Advance/GameOver clear scene/player/enemy/projectile transient state; reserve
// lives and game-over flags survive. Only Advance retains weapons. Continue
// resets active lives, scores and life thresholds; its earlier GameOver reset
// already removed weapons. HighScore and the current level always survive.
type CampaignReset struct {
	Kind              CampaignResetKind
	Players           [2]CampaignPlayer
	ClearStageRuntime bool
	ClearArcadeScores bool
	NextLife          [2]uint16
	PlayerMode        uint8
}

type CampaignCommand struct {
	ID        uint64
	Kind      string
	FromLevel uint8
	ToLevel   uint8
	Reset     *CampaignReset
}

type CampaignPlan struct {
	Work      CampaignWork
	Duplicate bool
	Commands  []CampaignCommand
	Sounds    []uint8
}
