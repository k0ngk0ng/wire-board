package contra

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type victoryBoundary struct {
	Schema                       int               `json:"schema"`
	Scope                        string            `json:"scope"`
	Players                      int               `json:"players"`
	Assisted                     bool              `json:"assisted"`
	Source                       string            `json:"source"`
	SourceSHA256                 map[string]string `json:"sourceSHA256"`
	PrefixFrame                  uint64            `json:"prefixFrame"`
	FinalFrame                   uint64            `json:"finalFrame"`
	TailFrames                   uint64            `json:"tailFrames"`
	TailPadsZero                 bool              `json:"tailPadsZero"`
	RecordedStateMatches         bool              `json:"recordedStateMatches"`
	RecordedRulesAndEffectsMatch bool              `json:"recordedRulesAndEffectsMatch"`
	ReplayDerivedReviewAdded     bool              `json:"replayDerivedReviewAdded"`
	PrefixSHA256                 string            `json:"prefixSHA256"`
	FinalSHA256                  string            `json:"finalSHA256"`
	Prefix                       []byte            `json:"-"`
	Final                        []byte            `json:"-"`
	Directory                    string            `json:"-"`
}

func fixtureSHA(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func fixtureWorkspacePath(root, value string) (string, error) {
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	path, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("fixture path escaped workspace")
	}
	return path, nil
}
func loadVictoryBoundary(directory string) (*victoryBoundary, error) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		return nil, err
	}
	directory, err = fixtureWorkspacePath(root, directory)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, "victory-boundary.json"))
	if err != nil {
		return nil, err
	}
	var b victoryBoundary
	if err = json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	b.Directory = directory
	if b.Schema != 1 || !b.Assisted || !(b.RecordedStateMatches || b.RecordedRulesAndEffectsMatch && b.ReplayDerivedReviewAdded) || !b.TailPadsZero || b.Players != 1 || b.TailFrames == 0 || b.TailFrames > 600 || b.PrefixFrame+b.TailFrames != b.FinalFrame {
		return nil, errors.New("unsupported or unverified assisted victory boundary")
	}
	source, err := fixtureWorkspacePath(root, b.Source)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"result.json", "final-session.json", "pads.bin.gz", "test-life-refills.json"} {
		data, e := os.ReadFile(filepath.Join(source, name))
		if e != nil {
			return nil, e
		}
		if fixtureSHA(data) != b.SourceSHA256[name] {
			return nil, errors.New("recorded victory source changed: " + name)
		}
	}
	b.Prefix, err = os.ReadFile(filepath.Join(directory, "before-terminal.json"))
	if err != nil {
		return nil, err
	}
	b.Final, err = os.ReadFile(filepath.Join(directory, "expected-final.json"))
	if err != nil {
		return nil, err
	}
	if fixtureSHA(b.Prefix) != b.PrefixSHA256 || fixtureSHA(b.Final) != b.FinalSHA256 {
		return nil, errors.New("victory boundary state identity differs")
	}
	var initial, final engine.Session
	if err = strict(b.Prefix, &initial); err != nil {
		return nil, err
	}
	if err = strict(b.Final, &final); err != nil {
		return nil, err
	}
	if err = initial.Validate(); err != nil {
		return nil, err
	}
	if err = final.Validate(); err != nil {
		return nil, err
	}
	if initial.Finalized || initial.World.Campaign.Outcome != engine.CampaignOngoing || initial.Random.Frames != b.PrefixFrame || int(initial.World.Active.Combat.World.PlayerMode)+1 != b.Players || !final.Finalized || final.World.Campaign.Outcome != engine.CampaignCleared || final.Random.Frames != b.FinalFrame || initial.Audio.Epoch != final.Audio.Epoch {
		return nil, errors.New("victory fixture is not a real preterminal-to-cleared transition")
	}
	return &b, nil
}

// Test-only: a source-verified diagnostic prefix replaces Start. The unchanged
// production OnTimer publishes the result after an atomic test timing gate.
// Rating eligibility is projected onto private users solely to test the ledger;
// this does not claim that assisted runs qualify for production scores.
func withVictoryBoundary(module gameapi.Module, b *victoryBoundary, gate *atomic.Bool) gameapi.Module {
	start, timer := module.Start, module.OnTimer
	module.Start = func(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		if c.Players != b.Players {
			return gameapi.Match{}, errors.New("victory fixture requires the recorded player count")
		}
		match, err := start(c, raw)
		if err != nil {
			return match, err
		}
		var state State
		if err = json.Unmarshal(match.Data, &state); err != nil {
			return match, err
		}
		var session engine.Session
		if err = json.Unmarshal(b.Prefix, &session); err != nil {
			return match, err
		}
		state.Session = &session
		state.Epoch = session.Audio.Epoch
		state.Frames = []FrameRecord{}
		state.ServerAt = c.Now.UnixMilli()
		state.Remainder = 0
		match.CheckpointKey = state.Epoch + ":assisted-victory-boundary"
		match.Log = append(match.Log, "测试夹具：已声明补命诊断的胜利终局；积分仅验证私有测试账本，不代表标准命数验收。")
		return store(match, &state)
	}
	module.OnTimer = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		if gate.Load() {
			return timer(c, match)
		}
		var state State
		if err := json.Unmarshal(match.Data, &state); err != nil {
			return match, err
		}
		state.ServerAt = c.Now.UnixMilli()
		state.Remainder = 0
		match.SystemDeadline = state.ServerAt + frameMS
		return store(match, &state)
	}
	return module
}
