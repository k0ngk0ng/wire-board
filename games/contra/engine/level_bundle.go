package contra

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

//go:embed leveldata/stage-*.json
var builtinStages embed.FS

type StageResourceIdentity struct {
	Level  uint8  `json:"level"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Digest string `json:"digest"`
}
type LevelBundleManifest struct {
	Version uint8                    `json:"version"`
	ID      string                   `json:"id"`
	Stages  [8]StageResourceIdentity `json:"stages"`
}
type LevelBundle struct {
	manifest  LevelBundleManifest
	payloads  [8][]byte
	resources [8]*LevelResources
}

func resourceSHA(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func canonicalStageDigest(raw []byte) (string, error) {
	var v any
	if e := json.Unmarshal(raw, &v); e != nil {
		return "", e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	return resourceSHA(b), nil
}
func (m LevelBundleManifest) identity() string {
	m.ID = ""
	b, _ := json.Marshal(m)
	return resourceSHA(b)
}
func (m LevelBundleManifest) Validate() error {
	if m.Version != 1 || m.ID != m.identity() {
		return errors.New("level bundle identity differs")
	}
	for i, s := range m.Stages {
		if s.Level != uint8(i) || s.Path != fmt.Sprintf("stage-%d.json", i+1) {
			return errors.New("invalid stage manifest entry")
		}
		for _, value := range []string{s.SHA256, s.Digest} {
			b, e := hex.DecodeString(value)
			if e != nil || len(b) != 32 {
				return errors.New("invalid stage content hash")
			}
		}
	}
	return nil
}
func (m LevelBundleManifest) DecodeStage(level uint8, raw []byte) (*LevelResources, error) {
	if e := m.Validate(); e != nil {
		return nil, e
	}
	if level >= 8 || resourceSHA(raw) != m.Stages[level].SHA256 {
		return nil, errors.New("stage payload identity differs")
	}
	d, e := canonicalStageDigest(raw)
	if e != nil || d != m.Stages[level].Digest {
		return nil, errors.New("stage canonical identity differs")
	}
	r, e := DecodeLevelResources(raw)
	if e != nil {
		return nil, e
	}
	if len(r.stages) != 1 {
		return nil, errors.New("stage payload contains unrelated maps")
	}
	if _, e = r.Stage(int(level)); e != nil {
		return nil, e
	}
	return r, nil
}
func (b *LevelBundle) Manifest() LevelBundleManifest { return b.manifest }
func (b *LevelBundle) Payload(level uint8) ([]byte, error) {
	if b == nil || level >= 8 {
		return nil, errors.New("invalid stage")
	}
	return append([]byte(nil), b.payloads[level]...), nil
}
func (b *LevelBundle) Stage(level uint8) (*LevelResources, error) {
	if b == nil || level >= 8 {
		return nil, errors.New("invalid stage")
	}
	return b.resources[level], nil
}

var builtinBundleOnce sync.Once
var builtinBundle *LevelBundle
var builtinBundleError error

// BuiltinLevelBundle reads only embedded immutable data. A server constructs it
// outside Module callbacks; clients receive the small manifest and fetch only
// their current stage payload. The stable ID survives stage changes.
func BuiltinLevelBundle() (*LevelBundle, error) {
	builtinBundleOnce.Do(func() {
		b := &LevelBundle{manifest: LevelBundleManifest{Version: 1}}
		for i := range 8 {
			path := fmt.Sprintf("stage-%d.json", i+1)
			raw, e := builtinStages.ReadFile("leveldata/" + path)
			if e != nil {
				builtinBundleError = e
				return
			}
			d, e := canonicalStageDigest(raw)
			if e != nil {
				builtinBundleError = e
				return
			}
			b.payloads[i] = raw
			b.manifest.Stages[i] = StageResourceIdentity{Level: uint8(i), Path: path, SHA256: resourceSHA(raw), Digest: d}
		}
		b.manifest.ID = b.manifest.identity()
		for i := range 8 {
			r, e := b.manifest.DecodeStage(uint8(i), b.payloads[i])
			if e != nil {
				builtinBundleError = e
				return
			}
			b.resources[i] = r
		}
		builtinBundle = b
	})
	return builtinBundle, builtinBundleError
}
