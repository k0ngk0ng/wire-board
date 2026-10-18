package basketball

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Saved campaign rosters are constrained by earned progress and the engine's
// source rules. The stricter single-match waiting-room content gate remains
// unchanged; accepting an earned save does not unlock new room options.
func validateSavedRoster(s State) error {
	if s.Runtime.Config.Campaign == nil {
		return validateRoster(s.Options)
	}
	p := s.Runtime.Config.Campaign
	if p.Mode == 0 && s.Options.Mode == "solo" || p.Mode == 1 && s.Options.Mode == "cooperative" {
		// decode subsequently validates the full runtime, including progress,
		// original mode, ownership, roster, equipment and preparation revisions.
		return nil
	}
	return fmt.Errorf("战役存档与席位模式不一致")
}

func applyCampaignPreparation(ctx gameapi.Context, original State, a action) (State, error) {
	fail := func(err error) (State, error) { return original, err }
	flags, err := automated(ctx, original.Options)
	if err != nil {
		return fail(err)
	}
	// The original first controller owns the shared team menu. Context does
	// not carry host identity during play: do not infer it from client fields.
	if ctx.Actor != 0 || original.actorForSeat(ctx.Actor) != 0 || flags[0] {
		return fail(fmt.Errorf("只有未托管的第一席玩家可以调整中场准备"))
	}
	if a.Epoch != original.Epoch || a.Preparation == nil || a.Sequence != 0 || a.Buttons != 0 || a.Seat != nil || a.BasedOn != nil || a.Events != nil {
		return fail(fmt.Errorf("中场准备已过期或包含无效字段"))
	}
	s := original
	// Current authenticated automation controls tactic availability, including
	// takeover/reclaim transactions that have not yet reached a video tick.
	if err = s.syncAutomation(flags); err != nil {
		return fail(err)
	}
	step, err := engine.ApplyCampaignPreparation(s.Runtime, *a.Preparation)
	if err != nil {
		return fail(err)
	}
	s.Runtime = step.State
	// Editing a menu counts as activity, but never consumes controller input
	// sequence numbers or extends another seat's timeout.
	s.Inputs[0].IdleAt = ctx.Now.UnixMilli() + idleMS
	return s, nil
}
