package contra

import (
	"encoding/json"
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type request struct {
	Type           string  `json:"type"`
	Epoch          string  `json:"epoch"`
	Sequence       int64   `json:"sequence"`
	Held           *uint16 `json:"held,omitempty"`
	BundleID       string  `json:"bundleID,omitempty"`
	RulesBuildID   string  `json:"rulesBuildID,omitempty"`
	RuntimeBuildID string  `json:"runtimeBuildID,omitempty"`
	Level          *uint8  `json:"level,omitempty"`
	StageSHA256    string  `json:"stageSHA256,omitempty"`
}

func parse(raw []byte) (request, string, error) {
	var a request
	if e := strict(raw, &a); e != nil {
		return a, "", e
	}
	if a.Epoch == "" || a.Sequence <= 0 || a.Sequence > maxSequence {
		return a, "", errors.New("invalid input epoch or sequence")
	}
	stage := a.BundleID != "" || a.RulesBuildID != "" || a.RuntimeBuildID != "" || a.Level != nil || a.StageSHA256 != ""
	switch a.Type {
	case "input":
		if a.Held == nil || *a.Held > 255 || stage {
			return a, "", errors.New("invalid pad input")
		}
	case "release", "resume":
		if a.Held != nil || stage {
			return a, "", errors.New("invalid control input")
		}
	case "stage-ready":
		if a.Held != nil || a.Level == nil || *a.Level >= 8 || a.BundleID == "" || a.RulesBuildID == "" || a.StageSHA256 == "" {
			return a, "", errors.New("invalid stage readiness")
		}
	default:
		return a, "", errors.New("unknown Contra action")
	}
	b, _ := json.Marshal(a)
	return a, string(b), nil
}
func (h *host) admission(c gameapi.Context, g gameapi.Match, raw []byte) (State, request, string, error) {
	s, e := h.decode(g)
	if e != nil {
		return s, request{}, "", e
	}
	a, key, e := parse(raw)
	if e != nil {
		return s, a, key, e
	}
	if _, e = flags(c, s.Players); e != nil {
		return s, a, key, e
	}
	if c.Actor < 0 || c.Actor >= s.Players || a.Epoch != s.Epoch || g.Result != nil {
		return s, a, key, errors.New("input seat or epoch differs")
	}
	i := s.Inputs[c.Actor]
	if a.Sequence < i.Sequence || a.Sequence-i.Sequence > 4096 || a.Sequence == i.Sequence && key != i.LastAction {
		return s, a, key, errors.New("stale or conflicting input sequence")
	}
	return s, a, key, nil
}

// isInputRelease is host admission metadata only. Apply repeats all checks.
func (h *host) isInputRelease(c gameapi.Context, g gameapi.Match, raw json.RawMessage) bool {
	_, a, _, err := h.admission(c, g, raw)
	return err == nil && a.Type == "release"
}
func (h *host) acceptStale(c gameapi.Context, g gameapi.Match, raw json.RawMessage) bool {
	_, _, _, e := h.admission(c, g, raw)
	return e == nil
}
func (h *host) apply(c gameapi.Context, g gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, a, key, e := h.admission(c, g, raw)
	if e != nil {
		return g, e
	}
	i := &s.Inputs[c.Actor]
	if a.Sequence == i.Sequence {
		return g, nil
	}
	if e = contextError(c, &s); e != nil {
		return g, e
	}
	auto, _ := flags(c, s.Players)
	// A physical release never changes ownership and remains valid while an
	// autoplay cancellation is in flight. Loading pads are acknowledged but
	// discarded below, including a delayed packet from a now-automated seat.
	loadingPad := s.StageLoading != nil && a.Type == "input" && (!s.StageLoading.Ready[c.Actor] || auto[c.Actor])
	if auto[c.Actor] && a.Type != "stage-ready" && a.Type != "release" && !loadingPad {
		return g, errors.New("请先取消托管")
	}
	resume := false
	switch a.Type {
	case "input":
		if loadingPad {
			// Never retain controls from the previous stage or the loading UI.
			// Still commit Sequence/LastAction so retries after gate release are
			// inert; a fresh post-gate heartbeat may establish a new held pad.
			clearInput(i)
			break
		}
		held := engine.Buttons(*a.Held)
		resume = g.Paused && c.Actor == 0 && held&engine.ButtonStart != 0 && i.Held&engine.ButtonStart == 0
		if held != i.Held {
			if len(i.Queue) >= maxQueue {
				return g, errors.New("input queue full; release controls and retry")
			}
			i.Queue = append(i.Queue, held)
		}
		i.Held = held
		i.ReceivedAt = c.Now.UnixMilli()
	case "release":
		clearInput(i)
	case "resume":
		if c.Actor != 0 || !g.Paused {
			return g, errors.New("只有一号角色可以恢复暂停")
		}
		resume = true
		clearInput(i)
	case "stage-ready":
		level := s.Session.World.Active.Combat.World.Level
		if level >= 8 || *a.Level != level || a.BundleID != s.BundleID || a.RulesBuildID != h.build || a.RuntimeBuildID != h.runtimeBuild || a.StageSHA256 != h.bundle.Manifest().Stages[level].SHA256 {
			return g, errors.New("stage resource identity differs")
		}
		if s.StageLoading != nil {
			if !s.StageLoading.Ready[c.Actor] {
				clearInput(i)
			}
			s.StageLoading.Ready[c.Actor] = true
			releaseStage(&s)
			critical(&g, &s, "stage-ready")
		}
	}
	if resume {
		if e = s.Session.RequestResume(); e != nil {
			return g, e
		}
		if e = h.record(&s, "resume", [2]engine.Buttons{}); e != nil {
			return g, e
		}
		g.Paused = false
		s.ServerAt = c.Now.UnixMilli()
		s.Remainder = 0
		g.SystemDeadline = s.ServerAt + frameMS
		for j := range s.Inputs {
			s.Inputs[j].Queue = nil
			s.Inputs[j].Applied = s.Inputs[j].Held
		}
		critical(&g, &s, "resume")
	}
	i.Sequence = a.Sequence
	i.LastAction = key
	return store(g, &s)
}
func releaseStage(s *State) {
	if s.StageLoading == nil {
		return
	}
	for _, r := range s.StageLoading.Ready {
		if !r {
			return
		}
	}
	// This is per-seat resource readiness, never a barrier for the running
	// simulation. A late peer must not clear another seat's controls or clock.
	s.StageLoading = nil
}
