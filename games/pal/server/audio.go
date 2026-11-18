package pal

import (
	"errors"
	"slices"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

type AudioTrack struct {
	Seq       uint32 `json:"seq"`
	Kind      int    `json:"kind"`
	ID        int    `json:"id"`
	Loop      bool   `json:"loop"`
	StartedAt int64  `json:"startedAt"`
}

type AudioSound struct {
	Seq       uint32 `json:"seq"`
	ID        int    `json:"id"`
	StartedAt int64  `json:"startedAt"`
}

type AudioView struct {
	AssetID string       `json:"assetID"`
	Serial  uint32       `json:"serial"`
	Music   *AudioTrack  `json:"music,omitempty"`
	Sounds  []AudioSound `json:"sounds"`
}

type audioMemory struct {
	LastSound int `json:"lastSound"`
}

type audioSchedule struct {
	StartedAt int64
	Commands  []engine.TimedAudioCommand
}

type audioCatalog struct {
	Sounds map[int][2]int64
	Music  map[int][2]int64
	CD     map[int][2]int64
}

func audioClipMS(assetID string, kind, id int) int64 {
	catalog, ok := audioCatalogs[assetID]
	if !ok {
		return 0
	}
	var clip [2]int64
	switch kind {
	case 1:
		clip = catalog.Music[id]
	case 2:
		clip = catalog.Sounds[id]
	case 3:
		clip = catalog.CD[id]
	}
	if clip[1] == 0 {
		return 0
	}
	return (clip[0]*1000 + clip[1] - 1) / clip[1]
}

func validAudio(s state) bool {
	view := s.Public.Audio
	if view == nil {
		return s.Audio == nil
	}
	if _, ok := audioCatalogs[view.AssetID]; !ok || s.Audio == nil || s.Audio.LastSound < 0 || s.Audio.LastSound > 65535 || len(view.Sounds) > 128 {
		return false
	}
	if music := view.Music; music != nil {
		if music.Seq == 0 || music.Seq > view.Serial || music.Kind != 1 && music.Kind != 3 || audioClipMS(view.AssetID, music.Kind, music.ID) == 0 || music.StartedAt <= 0 || music.StartedAt > s.ServerAt+tickMS || music.Kind == 3 && !music.Loop {
			return false
		}
	}
	var previous uint32
	for _, sound := range view.Sounds {
		if sound.Seq <= previous || sound.Seq > view.Serial || audioClipMS(view.AssetID, 2, sound.ID) == 0 || sound.StartedAt <= 0 || sound.StartedAt > s.ServerAt+tickMS {
			return false
		}
		previous = sound.Seq
	}
	return true
}

func cloneAudio(view *AudioView) *AudioView {
	if view == nil {
		return nil
	}
	copy := *view
	copy.Sounds = slices.Clone(view.Sounds)
	if view.Music != nil {
		music := *view.Music
		copy.Music = &music
	}
	return &copy
}

func pruneAudio(s *state, now int64) {
	view := s.Public.Audio
	if view == nil {
		return
	}
	if music := view.Music; music != nil && !music.Loop && now-music.StartedAt >= audioClipMS(view.AssetID, music.Kind, music.ID) {
		view.Music = nil
	}
	before := len(view.Sounds)
	view.Sounds = slices.DeleteFunc(view.Sounds, func(sound AudioSound) bool { return now-sound.StartedAt >= audioClipMS(view.AssetID, 2, sound.ID) })
	if before != len(view.Sounds) {
		s.Audio.LastSound = 0
	}
}

func syncNativeAudio(s *state, journal *engine.AudioJournal, now int64, timed []engine.TimedAudioCommand) error {
	if journal == nil {
		if s.Public.Audio != nil || s.Audio != nil {
			return errors.New("仙剑音频核心与存档不匹配")
		}
		return nil
	}
	if s.Public.Audio == nil {
		s.Public.Audio = &AudioView{AssetID: audioAssetID, Sounds: []AudioSound{}}
		s.Audio = &audioMemory{}
	}
	view := s.Public.Audio
	if _, ok := audioCatalogs[view.AssetID]; !ok {
		return errors.New("仙剑音频版本不可用，原存档已保留")
	}
	if journal.Serial < view.Serial {
		return errors.New("仙剑音频事件代次倒退")
	}
	commands := timed
	if len(commands) == 0 {
		for _, command := range journal.Commands {
			if command.Seq > view.Serial {
				commands = append(commands, engine.TimedAudioCommand{Command: command})
			}
		}
	}
	if uint32(len(commands)) != journal.Serial-view.Serial {
		return errors.New("仙剑音频事件不完整")
	}
	previousAt := int64(0)
	for _, timing := range commands {
		command := timing.Command
		at := now + int64(timing.OffsetMS)
		if command.Seq != view.Serial+1 || at < previousAt {
			return errors.New("仙剑音频事件顺序无效")
		}
		previousAt = at
		pruneAudio(s, at)
		switch command.Kind {
		case 1, 3:
			if audioClipMS(view.AssetID, command.Kind, command.ID) == 0 {
				if view.Music != nil && view.Music.Kind == command.Kind {
					view.Music = nil
				}
			} else if view.Music == nil || view.Music.Kind != command.Kind || view.Music.ID != command.ID {
				view.Music = &AudioTrack{Seq: command.Seq, Kind: command.Kind, ID: command.ID, Loop: command.Loop, StartedAt: at}
			}
		case 2:
			if command.ID != s.Audio.LastSound {
				s.Audio.LastSound = command.ID
				if audioClipMS(view.AssetID, 2, command.ID) > 0 {
					view.Sounds = append(view.Sounds, AudioSound{Seq: command.Seq, ID: command.ID, StartedAt: at})
				}
			}
		default:
			return errors.New("仙剑音频事件类型无效")
		}
		view.Serial = command.Seq
	}
	if len(view.Sounds) > 128 {
		return errors.New("仙剑同时音效超过已验证上限")
	}
	return nil
}
