package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"

	"github.com/tetratelabs/wazero/api"
)

type AudioCommand struct {
	Seq    uint32 `json:"seq"`
	Clock  uint32 `json:"clock"`
	Kind   int    `json:"kind"`
	ID     int    `json:"id"`
	Loop   bool   `json:"loop"`
	FadeMS int    `json:"fadeMS"`
}

type AudioJournal struct {
	Serial   uint32         `json:"serial"`
	Commands []AudioCommand `json:"commands"`
}

type TimedAudioCommand struct {
	OffsetMS uint32
	Command  AudioCommand
}

func validateAudio(journal *AudioJournal) error {
	if journal == nil {
		return nil
	}
	if len(journal.Commands) != int(min(journal.Serial, 128)) {
		return errors.New("invalid native audio journal length")
	}
	for i, command := range journal.Commands {
		if command.Seq != journal.Serial-uint32(len(journal.Commands))+uint32(i)+1 || command.Kind < 1 || command.Kind > 3 || command.ID < -2 || command.ID > 65535 || command.FadeMS < 0 || command.FadeMS > 60000 || command.Kind == 2 && (command.ID < 0 || command.Loop || command.FadeMS != 0) || command.Kind == 3 && (command.FadeMS != 0 || command.Loop != (command.ID > 0)) {
			return errors.New("invalid native audio command")
		}
		if i > 0 && command.Clock < journal.Commands[i-1].Clock {
			return errors.New("native audio clock went backwards")
		}
	}
	return nil
}

func audioSince(journal *AudioJournal, serial uint32) ([]AudioCommand, error) {
	if journal == nil {
		return nil, nil
	}
	if journal.Serial < serial || journal.Serial-serial > uint32(len(journal.Commands)) {
		return nil, errors.New("native audio journal lost ordered commands")
	}
	count := int(journal.Serial - serial)
	return slices.Clone(journal.Commands[len(journal.Commands)-count:]), nil
}

func observeAudio(ctx context.Context, module api.Module) (*AudioJournal, error) {
	if module.ExportedFunction("wb_audio_info") == nil {
		return nil, nil
	}
	address, err := call(ctx, module, "wb_audio_info")
	if err != nil {
		return nil, err
	}
	header, ok := module.Memory().Read(uint32(address), 12)
	if !ok || binary.LittleEndian.Uint32(header) != 1 {
		return nil, errors.New("invalid native audio header")
	}
	serial, count := binary.LittleEndian.Uint32(header[4:]), binary.LittleEndian.Uint32(header[8:])
	if count > 128 {
		return nil, errors.New("native audio journal exceeds limit")
	}
	data, ok := module.Memory().Read(uint32(address)+12, count*24)
	if !ok {
		return nil, errors.New("native audio journal exceeds memory")
	}
	journal := &AudioJournal{Serial: serial, Commands: make([]AudioCommand, count)}
	for i := range journal.Commands {
		row := data[i*24:]
		get := func(n int) int { return int(int32(binary.LittleEndian.Uint32(row[n*4:]))) }
		if get(4) < 0 || get(4) > 1 {
			return nil, errors.New("invalid native audio loop value")
		}
		journal.Commands[i] = AudioCommand{Seq: uint32(get(0)), Clock: uint32(get(1)), Kind: get(2), ID: get(3), Loop: get(4) == 1, FadeMS: get(5)}
	}
	return journal, validateAudio(journal)
}
