package engine

// InteractionState is the state written by the original response phase.
// Ownership stays here until the later action handlers explicitly release it.
type InteractionState struct {
	Players       [4]CombatActor    `json:"players"`
	Auxiliary     [2]AuxiliaryState `json:"auxiliary"`
	SpecialHolder int8              `json:"specialHolder"`
	Ball          Ball              `json:"ball"`
	Holder        int8              `json:"holder"`
	LastTouch     int8              `json:"lastTouch"`
	SelectedHoop  int8              `json:"selectedHoop"`
}

type InteractionContext struct {
	CourtID                         uint8
	Candidates                      [7]int8
	Controls                        [4]uint8
	Attachments                     [4]uint8
	ExcludedCatcher, PreviousHolder int8
	AuxiliaryOwners                 [2]int8
	AuxiliaryKinds                  [2]uint8
	HoopOwners                      [6]int8
}

type InteractionStep struct {
	ChangeCaptureCounter bool
	CaptureCounter       uint8
	State                InteractionState
	Sounds               []CombatSound
	Catch                CatchResolution
}

// Only bit 16 is read by the original ordinary-player catch branch. These
// ranges are the exact $9321 entries below action 96, independently extracted
// in testdata/player-catch-flags.json. Search-time geometry flags are not used.
func originalPlayerCatchFlags(action Action) uint8 {
	switch {
	case action <= 5, action >= 0x20 && action <= 0x23, action >= 0x3d && action <= 0x43, action >= 0x50 && action <= 0x53:
		return 16
	default:
		return 0
	}
}

// StepOriginalInteractions restores $84B9 through the return at $801D:
// complete attack responses first, then determine catch eligibility from the
// updated actions. Ordinary catches add no sound; unresolved prop branches fail
// explicitly and roll back the complete phase rather than leave partial hits.
func StepOriginalInteractions(original InteractionState, ctx InteractionContext) (InteractionStep, error) {
	initial := InteractionStep{State: original, Catch: CatchResolution{Holder: original.Holder, LastTouch: original.LastTouch, Actor: -1}}
	result := initial
	combatContext := CombatContext{CourtID: ctx.CourtID, AuxiliaryKinds: ctx.AuxiliaryKinds, PlayerAttachments: &ctx.Attachments}
	copy(combatContext.Candidates[:], ctx.Candidates[:6])
	combat, err := StepOriginalCombat(CombatState{Players: original.Players, Auxiliary: original.Auxiliary, SpecialHolder: original.SpecialHolder}, combatContext)
	if err != nil {
		return initial, err
	}
	result.State.Players, result.Sounds = combat.State.Players, combat.Sounds
	result.State.Auxiliary, result.State.SpecialHolder = combat.State.Auxiliary, combat.State.SpecialHolder
	result.ChangeCaptureCounter, result.CaptureCounter = combat.ChangeCaptureCounter, combat.CaptureCounter
	tail, err := StepOriginalBallContact(result.State, ctx)
	if err != nil {
		return initial, err
	}
	result.State, result.Catch = tail.State, tail.Catch
	result.Sounds = append(result.Sounds, tail.Sounds...)
	return result, nil
}
