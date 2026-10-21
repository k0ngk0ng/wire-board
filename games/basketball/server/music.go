package basketball

import (
	"encoding/hex"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/assets"
)

// Music uses the server's media wall clock at playbackRate=1, matching the
// native media channel. It is not a VideoFrames/60 estimate of original PCM.
// Original commands alone select, pause, resume or clear this timeline.
type MusicState struct {
	Epoch           uint64 `json:"epoch"`
	TrackID         int    `json:"trackID"`
	AssetSHA256     string `json:"assetSHA256"`
	ControlSequence uint64 `json:"controlSequence"`
	ElapsedMS       int64  `json:"elapsedMS"`
	StartedAt       int64  `json:"startedAt"`
	Paused          bool   `json:"paused"`
}
type MusicView struct {
	Epoch           uint64 `json:"epoch"`
	TrackID         int    `json:"trackID"`
	AssetSHA256     string `json:"assetSHA256"`
	ControlSequence uint64 `json:"controlSequence"`
	PositionMS      int64  `json:"positionMS"`
	At              int64  `json:"at"`
	Paused          bool   `json:"paused"`
}

func isMusic(id int) bool { return id >= 59 && id <= 66 || id == 75 || id == 76 }
func (s MusicState) PositionMS(at int64) int64 {
	if s.TrackID == 0 {
		return 0
	}
	value := s.ElapsedMS
	if !s.Paused && at > s.StartedAt {
		value += at - s.StartedAt
	}
	return value
}
func (s MusicState) view(at int64) MusicView {
	return MusicView{Epoch: s.Epoch, TrackID: s.TrackID, AssetSHA256: s.AssetSHA256, ControlSequence: s.ControlSequence, PositionMS: s.PositionMS(at), At: at, Paused: s.Paused}
}
func (s MusicState) validate(maxSound uint64) error {
	if s.Epoch > uint64(maxSequence) || s.ControlSequence > maxSound || s.ElapsedMS < 0 || s.StartedAt < 0 {
		return fmt.Errorf("无效音乐时间线")
	}
	if s.TrackID != 0 && !isMusic(s.TrackID) {
		return fmt.Errorf("未知音乐时间线")
	}
	// Legacy unversioned76 checkpoints remain unbound; only a new original
	// music request selects the verified delivery. The frontend rejects empty
	// revisions rather than guessing a migration or replaying an intro.
	if s.TrackID != 0 && !(s.TrackID == 76 && s.AssetSHA256 == "") {
		if len(s.AssetSHA256) != 64 {
			return fmt.Errorf("音乐版本缺失")
		}
		if _, err := hex.DecodeString(s.AssetSHA256); err != nil {
			return err
		}
	} else if s.AssetSHA256 != "" {
		return fmt.Errorf("无效音乐版本")
	}
	return nil
}
func (s *MusicState) Apply(ids []int, sequence uint64, at int64) error {
	if at < 0 || sequence > uint64(maxSequence) {
		return fmt.Errorf("无效音乐事件")
	}
	if sequence <= s.ControlSequence {
		return nil
	}
	next := *s
	for _, id := range ids {
		switch {
		case id == 0:
			next = MusicState{Epoch: next.Epoch + 1, ControlSequence: sequence}
		case id == 253:
			if !next.Paused {
				next.ElapsedMS = next.PositionMS(at)
				next.Paused = true
			}
			next.ControlSequence = sequence
		case id == 255:
			if next.Paused {
				next.StartedAt = at
				next.Paused = false
			}
			next.ControlSequence = sequence
		case isMusic(id):
			revision, err := assets.MusicRevision(id)
			if err != nil {
				return err
			}
			// Original $80E7 clears pause bit $40 in $07E5 for new music.
			next = MusicState{Epoch: next.Epoch + 1, TrackID: id, AssetSHA256: revision, ControlSequence: sequence, StartedAt: at}
		}
	}
	if err := next.validate(sequence); err != nil {
		return err
	}
	*s = next
	return nil
}
