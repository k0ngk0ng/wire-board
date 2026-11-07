package contra

import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"strconv"
	"strings"
)

const streamRedundancy = 6
const ordinaryFrames = 9

type bundleHeader struct {
	Version uint8  `json:"version"`
	ID      string `json:"id"`
}

type snapshotHeader struct {
	Review       *runtime.ReviewInfo    `json:"review,omitempty"`
	RulesBuildID string                 `json:"rulesBuildID"`
	Session      uint64                 `json:"session"`
	Epoch        string                 `json:"epoch"`
	Frame        uint64                 `json:"frame,string"`
	Revision     uint64                 `json:"revision,string"`
	AudioTick    uint64                 `json:"audioTick,string"`
	Digest       string                 `json:"digest"`
	Resources    string                 `json:"resources"`
	Phase        string                 `json:"phase"`
	Level        uint8                  `json:"level"`
	Paused       bool                   `json:"paused"`
	Finalized    bool                   `json:"finalized"`
	Outcome      engine.CampaignOutcome `json:"outcome,omitempty"`
	Players      [2]runtime.PlayerView  `json:"players"`
	Projectiles  int                    `json:"projectiles"`
}

func (v View) MarshalJSON() ([]byte, error) {
	type plain View
	if v.full {
		return json.Marshal(struct {
			plain
			Bundle bundleHeader `json:"bundle"`
		}{plain(v), bundleHeader{v.Bundle.Version, v.Bundle.ID}})
	}
	s := v.Snapshot
	frames := v.Frames
	if !v.streaming && len(frames) > ordinaryFrames {
		frames = frames[len(frames)-ordinaryFrames:]
	}
	if frames == nil {
		frames = []FrameRecord{}
	}
	return json.Marshal(struct {
		plain
		Snapshot snapshotHeader `json:"snapshot"`
		Frames   []FrameRecord  `json:"frames"`
		Bundle   struct {
			Version uint8  `json:"version"`
			ID      string `json:"id"`
		} `json:"bundle"`
	}{plain: plain(v), Snapshot: snapshotHeader{s.Review, s.RulesBuildID, s.Session, s.Epoch, s.Frame, s.Revision, s.AudioTick, s.Digest, s.Resources, s.Phase, s.Level, s.Paused, s.Finalized, s.Outcome, s.Players, s.Projectiles}, Frames: frames, Bundle: struct {
		Version uint8  `json:"version"`
		ID      string `json:"id"`
	}{v.Bundle.Version, v.Bundle.ID}})
}

// The simulation's frame record already contains its authoritative digest.
// Ordinary metadata never constructs a framebuffer or audio history.
func (h *host) snapshotHeader(s *State) (runtime.Snapshot, error) {
	session := s.Session
	w := session.World.Active.Combat.World
	digest := ""
	if len(s.Frames) > 0 {
		digest = s.Frames[len(s.Frames)-1].Digest
	} else {
		var err error
		digest, err = runtime.StateDigest(session, s.BundleID, h.build)
		if err != nil {
			return runtime.Snapshot{}, err
		}
	}
	phase := "transition"
	if w.GameRoutine == 5 {
		switch w.LevelRoutine {
		case 0:
			phase = "initializing"
		case 1:
			phase = "intro"
		case 2, 3:
			phase = "entering"
		case 4:
			phase = "playing"
		}
	}
	if session.World.Campaign.Outcome != engine.CampaignOngoing && !session.Finalized {
		phase = "finishing"
	}
	if session.Finalized {
		phase = "finished"
	}
	v := runtime.Snapshot{RulesBuildID: h.build, Epoch: s.Epoch, Frame: session.Random.Frames, Revision: session.Revision, AudioTick: session.Audio.Tick, Digest: digest, Resources: s.BundleID, Phase: phase, Level: w.Level, Paused: session.World.Campaign.Paused, Finalized: session.Finalized}
	if session.Finalized {
		v.Outcome = session.World.Campaign.Outcome
	}
	for i, p := range session.World.Active.Combat.Players {
		v.Players[i] = runtime.PlayerView{X: p.X, Y: uint8(p.Motion.Position >> 8), State: p.State, Lives: session.PresentedLives[i], GameOver: p.GameOver}
	}
	if session.Finalized && session.Review != nil {
		r := session.Review
		v.Review = &runtime.ReviewInfo{Frame: r.Frame, Revision: r.Revision, Level: r.Level}
		for i := range v.Players {
			v.Players[i].Lives = r.Lives[i]
		}
	}
	for _, p := range session.World.Active.Combat.Weapons.Projectiles {
		if p.Kind != 0 {
			v.Projectiles++
		}
	}
	return v, nil
}

func (h *host) stream(g gameapi.Match, actor int, cursor string) (gameapi.StreamViewResult, error) {
	value, err := h.view(g, actor)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	v := value.(View)
	v.streaming = true
	epoch, number, ok := strings.Cut(cursor, ":")
	after, parseErr := strconv.ParseUint(number, 10, 64)
	valid := ok && parseErr == nil && epoch == v.Epoch && after <= v.Revision
	if valid && after < v.Revision {
		valid = len(v.Frames) > 0 && after+1 >= v.Frames[0].Revision
	}
	if !valid {
		v.Resync = true
		v.Frames = nil
	} else if after == v.Revision {
		v.Frames = nil
	} else {
		first := 0
		for first < len(v.Frames) && v.Frames[first].Revision <= after {
			first++
		}
		first = max(0, first-streamRedundancy)
		v.Frames = append([]FrameRecord(nil), v.Frames[first:]...)
	}
	return gameapi.StreamViewResult{Data: v, Cursor: v.Epoch + ":" + strconv.FormatUint(v.Revision, 10)}, nil
}
