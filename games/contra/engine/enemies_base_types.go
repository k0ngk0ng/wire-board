package contra

// BaseEnemyState owns the base-room enemy counters at original $86..$8A.
// ScreenCleared remains the single CombatFrame value supplied through context;
// it is intentionally not duplicated here. The room loader resets these values
// according to the original indoor script before any room enemy phase runs.
type BaseEnemyState struct {
	CoresRemaining    uint8
	PlatingDestroyed  uint8
	AttackRounds      uint8
	RedSoldierCreated uint8
	GrenadeLauncher   uint8
}
