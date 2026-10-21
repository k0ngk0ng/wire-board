package bubbleparty_test

// Explicit version-2 opening fixture for the historical appearance-only API.
// New matches use the separate character selection contract tested elsewhere.
import (
	"encoding/json"
	bp "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func legacyAppearanceModule() gameapi.Module {
	mod := bp.Module()
	original := mod.Start
	mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, err := original(ctx, raw)
		if err != nil {
			return m, err
		}
		var s bp.State
		if err = json.Unmarshal(m.Data, &s); err != nil {
			return m, err
		}
		s.RulesVersion = 2
		s.Phase = bp.PhaseCountdown
		s.SelectUntil = 0
		s.StartAt = ctx.Now.UnixMilli() + bp.Countdown
		s.MatchEndsAt = s.StartAt + int64(s.Options.RoundSeconds)*1000
		s.NativeObjects = nil
		s.BoxMotions = nil
		for i := range s.Cells {
			if s.Cells[i] == bp.CellPushCrate {
				s.Cells[i] = bp.CellCrate
			}
		}
		for i := range s.Players {
			p := &s.Players[i]
			p.Character = 0
			p.NativeSpeed = 0
			p.Selected = false
			p.Mount = ""
			p.SpeedMS = bp.BaseMoveMS
			p.MaxBombs = 1
			p.Range = bp.BaseBombRange
		}
		m.Data, err = json.Marshal(s)
		if err != nil {
			return m, err
		}
		return mod.Restore(ctx, m)
	}
	return mod
}
