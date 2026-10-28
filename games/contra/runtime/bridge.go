// Package runtime bridges the independent Contra Session to native and Wasm
// callers. It contains no ROM execution, recorded after-states or game rules.
package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"

	contra "github.com/k0ngk0ng/wire-board/games/contra/engine"
)

type PlayerView struct {
	X        uint8 `json:"x"`
	Y        uint8 `json:"y"`
	State    uint8 `json:"state"`
	Lives    uint8 `json:"lives"`
	GameOver uint8 `json:"gameOver"`
}
type Snapshot struct {
	Review       *ReviewInfo            `json:"review,omitempty"`
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
	Outcome      contra.CampaignOutcome `json:"outcome,omitempty"`
	Players      [2]PlayerView          `json:"players"`
	Projectiles  int                    `json:"projectiles"`
	Presentation contra.PresentedFrame  `json:"presentation"`
	Audio        contra.AudioView       `json:"audio"`
}
type ReviewInfo struct {
	Frame    uint64 `json:"frame,string"`
	Revision uint64 `json:"revision,string"`
	Level    uint8  `json:"level"`
}
type Checkpoint struct {
	RulesBuildID string          `json:"rulesBuildID"`
	Version      uint8           `json:"version"`
	Epoch        string          `json:"epoch"`
	Resources    string          `json:"resources"`
	Digest       string          `json:"digest"`
	State        *contra.Session `json:"state"`
}
type Request struct {
	Op         string                      `json:"op"`
	Session    uint64                      `json:"session,omitempty"`
	Epoch      string                      `json:"epoch,omitempty"`
	Frame      uint64                      `json:"frame,string,omitempty"`
	Revision   uint64                      `json:"revision,string,omitempty"`
	Seed       string                      `json:"seed,omitempty"`
	Players    int                         `json:"players,omitempty"`
	Pads       []int                       `json:"pads,omitempty"`
	Resources  json.RawMessage             `json:"resources,omitempty"`
	Bundle     *contra.LevelBundleManifest `json:"bundle,omitempty"`
	Level      uint8                       `json:"level,omitempty"`
	StageData  string                      `json:"stageData,omitempty"`
	Checkpoint *Checkpoint                 `json:"checkpoint,omitempty"`
}
type Bridge struct {
	mu             sync.Mutex
	resources      *contra.LevelResources
	bundle         *contra.LevelBundleManifest
	stages         map[uint8]*contra.LevelResources
	resourceDigest string
	rulesBuildID   string
	next           uint64
	sessions       map[uint64]*contra.Session
}

func New(ids ...string) *Bridge {
	id, _ := selectedRulesBuild(ids)
	return &Bridge{rulesBuildID: id, sessions: map[uint64]*contra.Session{}, stages: map[uint8]*contra.LevelResources{}}
}
func sha(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func strict(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return errors.New("trailing runtime JSON")
	}
	return nil
}
func (b *Bridge) state(id uint64, epoch string) (*contra.Session, error) {
	s := b.sessions[id]
	if s == nil || epoch == "" || s.Audio.Epoch != epoch {
		return nil, errors.New("runtime session/epoch mismatch")
	}
	return s, nil
}
func (b *Bridge) resourcesFor(level uint8) (*contra.LevelResources, error) {
	if b.bundle != nil {
		r := b.stages[level]
		if r == nil {
			return nil, fmt.Errorf("stage %d resources are not installed", level+1)
		}
		return r, nil
	}
	if b.resources == nil {
		return nil, errors.New("runtime resources are not installed")
	}
	return b.resources, nil
}
func StateDigest(s *contra.Session, resourceDigest string, ids ...string) (string, error) {
	code, err := selectedRulesBuild(ids)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", errors.New("missing runtime state")
	}
	// The same Go encoder canonicalizes native and Wasm states, including map
	// order. JavaScript never hashes a differently ordered object as authority.
	raw, err := json.Marshal(struct {
		RulesBuildID string
		Resources    string
		State        *contra.Session
	}{code, resourceDigest, s})
	if err != nil {
		return "", err
	}
	return sha(raw), nil
}
func View(s *contra.Session, resourceDigest string, ids ...string) (Snapshot, error) {
	code, err := selectedRulesBuild(ids)
	if err != nil {
		return Snapshot{}, err
	}
	if s == nil {
		return Snapshot{}, errors.New("missing runtime state")
	}
	digest, err := StateDigest(s, resourceDigest, code)
	if err != nil {
		return Snapshot{}, err
	}
	w := s.World.Active.Combat.World
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
	out := Snapshot{RulesBuildID: code, Epoch: s.Audio.Epoch, Frame: s.Random.Frames, Revision: s.Revision, AudioTick: s.Audio.Tick, Digest: digest, Resources: resourceDigest, Phase: phase, Level: w.Level, Paused: s.World.Campaign.Paused, Finalized: s.Finalized, Presentation: s.World.Active.Presentation.Snapshot(), Audio: s.Audio.View()}
	if s.World.Campaign.Outcome != contra.CampaignOngoing && !s.Finalized {
		out.Phase = "finishing"
	}
	if s.Finalized {
		out.Outcome = s.World.Campaign.Outcome
		out.Phase = "finished"
	}
	for i, p := range s.World.Active.Combat.Players {
		out.Players[i] = PlayerView{X: p.X, Y: uint8(p.Motion.Position >> 8), State: p.State, Lives: s.PresentedLives[i], GameOver: p.GameOver}
	}
	if s.Finalized && s.Review != nil {
		out.Presentation = s.Review.Presentation
		out.Review = &ReviewInfo{Frame: s.Review.Frame, Revision: s.Review.Revision, Level: s.Review.Level}
		for i := range out.Players {
			out.Players[i].Lives = s.Review.Lives[i]
		}
	}
	for _, p := range s.World.Active.Combat.Weapons.Projectiles {
		if p.Kind != 0 {
			out.Projectiles++
		}
	}
	return out, nil
}
func CanonicalResourcesDigest(raw []byte) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return sha(canonical), nil
}
func CheckpointOf(s *contra.Session, resources string, ids ...string) (Checkpoint, error) {
	code, err := selectedRulesBuild(ids)
	if err != nil {
		return Checkpoint{}, err
	}
	if s == nil {
		return Checkpoint{}, errors.New("missing runtime state")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return Checkpoint{}, err
	}
	var owned contra.Session
	if err = json.Unmarshal(raw, &owned); err != nil {
		return Checkpoint{}, err
	}
	digest, err := StateDigest(s, resources, code)
	if err != nil {
		return Checkpoint{}, err
	}
	return Checkpoint{RulesBuildID: code, Version: 1, Epoch: s.Audio.Epoch, Resources: resources, Digest: digest, State: &owned}, nil
}
func (b *Bridge) snapshot(id uint64, s *contra.Session) (Snapshot, error) {
	view, err := View(s, b.resourceDigest, b.rulesBuildID)
	view.Session = id
	return view, err
}
func (b *Bridge) Call(request Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !validRulesBuildID(b.rulesBuildID) {
		return nil, errors.New("missing compiled rules build identity")
	}
	switch request.Op {
	case "bundle":
		if request.Bundle == nil {
			return nil, errors.New("missing level bundle manifest")
		}
		if err := request.Bundle.Validate(); err != nil {
			return nil, err
		}
		if len(b.sessions) > 0 && (b.bundle == nil || b.bundle.ID != request.Bundle.ID) {
			return nil, errors.New("bundle cannot change during a session")
		}
		if b.bundle == nil || b.bundle.ID != request.Bundle.ID {
			copy := *request.Bundle
			b.bundle = &copy
			b.resources = nil
			b.stages = map[uint8]*contra.LevelResources{}
			b.resourceDigest = copy.ID
		}
		return map[string]string{"digest": b.resourceDigest, "rulesBuildID": b.rulesBuildID}, nil
	case "stage":
		if b.bundle == nil {
			return nil, errors.New("stage requires a verified bundle")
		}
		r, err := b.bundle.DecodeStage(request.Level, []byte(request.StageData))
		if err != nil {
			return nil, err
		}
		b.stages[request.Level] = r
		return b.bundle.Stages[request.Level], nil
	case "resources":
		if len(b.sessions) != 0 {
			return nil, errors.New("resources cannot change during a session")
		}
		r, err := contra.DecodeLevelResources(request.Resources)
		if err != nil {
			return nil, err
		}
		digest, err := CanonicalResourcesDigest(request.Resources)
		if err != nil {
			return nil, err
		}
		b.resources = r
		b.bundle = nil
		b.stages = map[uint8]*contra.LevelResources{}
		b.resourceDigest = digest
		return map[string]string{"digest": b.resourceDigest, "rulesBuildID": b.rulesBuildID}, nil
	case "new":
		if (b.resources == nil && b.bundle == nil) || len(b.sessions) >= 8 {
			return nil, errors.New("runtime resource/session limit")
		}
		if _, err := b.resourcesFor(0); err != nil {
			return nil, err
		}
		seed, err := strconv.ParseUint(request.Seed, 10, 64)
		if err != nil {
			return nil, errors.New("invalid host seed")
		}
		s, err := contra.NewSession(seed, request.Players, request.Epoch)
		if err != nil {
			return nil, err
		}
		b.next++
		b.sessions[b.next] = s
		return b.snapshot(b.next, s)
	case "restore":
		c := request.Checkpoint
		if (b.resources == nil && b.bundle == nil) || c == nil || c.Version != 1 || c.RulesBuildID != b.rulesBuildID || c.Epoch != request.Epoch || c.Resources != b.resourceDigest || c.State == nil || c.State.Audio.Epoch != request.Epoch || len(b.sessions) >= 8 {
			return nil, errors.New("checkpoint identity differs")
		}
		digest, err := StateDigest(c.State, b.resourceDigest, b.rulesBuildID)
		if err != nil || digest != c.Digest {
			return nil, errors.New("checkpoint checksum differs")
		}
		raw, _ := json.Marshal(c.State)
		var owned contra.Session
		if err = json.Unmarshal(raw, &owned); err != nil {
			return nil, err
		}
		if err = owned.Validate(); err != nil {
			return nil, err
		}
		level := owned.World.Active.Combat.World.Level
		if level == 8 && owned.World.Campaign.Outcome == contra.CampaignCleared {
			level = 7
		}
		if _, err = b.resourcesFor(level); err != nil {
			return nil, err
		}
		b.next++
		b.sessions[b.next] = &owned
		return b.snapshot(b.next, &owned)
	}
	s, err := b.state(request.Session, request.Epoch)
	if err != nil {
		return nil, err
	}
	switch request.Op {
	case "snapshot":
		return b.snapshot(request.Session, s)
	case "checkpoint":
		return CheckpointOf(s, b.resourceDigest, b.rulesBuildID)
	case "dispose":
		delete(b.sessions, request.Session)
		return map[string]bool{"disposed": true}, nil
	case "paused", "resume", "terminal":
		if request.Revision == 0 {
			return nil, errors.New("paused operation requires an exact revision")
		}
		if request.Revision == s.Revision {
			return b.snapshot(request.Session, s)
		}
		if s.Revision == ^uint64(0) || request.Revision != s.Revision+1 {
			return nil, errors.New("runtime requires next revision")
		}
		candidate := *s
		if request.Op == "paused" {
			resources, e := b.resourcesFor(candidate.World.Active.Combat.World.Level)
			if e != nil {
				return nil, e
			}
			_, err = candidate.PresentPaused(resources)
		} else if request.Op == "resume" {
			err = candidate.RequestResume()
		} else {
			_, err = candidate.FlushTerminal()
		}
		if err != nil {
			return nil, err
		}
		if candidate.Revision != request.Revision {
			return nil, errors.New("runtime operation did not produce requested revision")
		}
		*s = candidate
		return b.snapshot(request.Session, s)
	case "pads":
		if request.Revision != 0 && request.Revision <= s.Revision {
			if request.Revision == s.Revision && request.Frame == s.Random.Frames {
				return b.snapshot(request.Session, s)
			}
			return nil, errors.New("stale runtime revision")
		}
		if len(request.Pads) != 2 {
			return nil, errors.New("two normalized pads required")
		}
		var pads [2]contra.Buttons
		for i, v := range request.Pads {
			if v < 0 || v > 255 {
				return nil, errors.New("invalid pad mask")
			}
			pads[i] = contra.Buttons(v)
		}
		candidate := *s
		resources, e := b.resourcesFor(candidate.World.Active.Combat.World.Level)
		if e != nil {
			return nil, e
		}
		if _, err = candidate.StepPads(request.Frame, pads, resources); err != nil {
			return nil, err
		}
		if request.Revision != 0 && candidate.Revision != request.Revision {
			return nil, errors.New("runtime revision differs from accepted operation")
		}
		*s = candidate
		return b.snapshot(request.Session, s)
	default:
		return nil, errors.New("unknown runtime operation")
	}
}
func (b *Bridge) JSON(raw string) (result string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			encoded, _ := json.Marshal(map[string]any{"ok": false, "error": fmt.Sprint(recovered)})
			result = string(encoded)
		}
	}()
	var request Request
	var value any
	err := strict([]byte(raw), &request)
	if err == nil {
		value, err = b.Call(request)
	}
	envelope := map[string]any{"ok": err == nil}
	if err != nil {
		envelope["error"] = err.Error()
	} else {
		envelope["data"] = value
	}
	encoded, _ := json.Marshal(envelope)
	return string(encoded)
}
