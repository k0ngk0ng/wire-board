package rootcontrol_test

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
)

//go:embed testdata/pause-native-levels.json.gz
var originalPauseLevels []byte

func TestOriginalMixedPauseMuteAndRestoreLevels(t *testing.T) {
	z, err := gzip.NewReader(bytes.NewReader(originalPauseLevels))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Version                                                      int
		Levels                                                       [6][4]uint8
		Mute, Restore                                                [24]rootcontrol.LevelWrite
		OriginalCacheVisibleEqualAtCapture, OriginalRestoreEqualsPre bool
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if source.Version != 1 || !source.OriginalCacheVisibleEqualAtCapture || !source.OriginalRestoreEqualsPre {
		t.Fatal("native pause provenance incomplete")
	}
	ym := chips.ResetYMState()
	for ch := range ym.Channels {
		for op := range ym.Channels[ch].Operators {
			ym.Channels[ch].Operators[op].Level = source.Levels[ch][op]
		}
	}
	chip, err := chips.NewInteractive(chips.Config{Profile: chips.ReferenceMixedHQ44100}, chips.State{YM: ym, PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := rootcontrol.CapturePauseLevels(chip.Snapshot().YM)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Levels != source.Levels {
		t.Fatal("captured another operator order", saved.Levels, source.Levels)
	}
	mute := rootcontrol.PauseMuteWrites()
	if mute != source.Mute {
		t.Fatal("native mute request order changed", mute, source.Mute)
	}
	for _, w := range mute {
		if err = chip.WritePort(0, w.Bank*2, w.Register); err != nil {
			t.Fatal(err)
		}
		if err = chip.WritePort(0, w.Bank*2+1, w.Value); err != nil {
			t.Fatal(err)
		}
	}
	for _, ch := range chip.Snapshot().YM.Channels {
		for _, op := range ch.Operators {
			if op.Level != 127 {
				t.Fatal("physical mute missing")
			}
		}
	}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var restored rootcontrol.PauseLevels
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	writes, err := rootcontrol.PauseRestoreWrites(restored)
	if err != nil {
		t.Fatal(err)
	}
	if writes != source.Restore {
		t.Fatal("native restore request order changed", writes, source.Restore)
	}
	for _, w := range writes {
		if err = chip.WritePort(0, w.Bank*2, w.Register); err != nil {
			t.Fatal(err)
		}
		if err = chip.WritePort(0, w.Bank*2+1, w.Value); err != nil {
			t.Fatal(err)
		}
	}
	for ch, c := range chip.Snapshot().YM.Channels {
		for op, o := range c.Operators {
			if o.Level != source.Levels[ch][op] {
				t.Fatal("did not restore original visible TL", ch, op, o.Level)
			}
		}
	}
	if saved.Levels != source.Levels {
		t.Fatal("mute/restore aliased the pending payload")
	}
}

func TestPauseLevelsHaveIndependentInformationAfterMute(t *testing.T) {
	a, b := chips.ResetYMState(), chips.ResetYMState()
	a.Channels[2].Operators[1].Level = 13
	b.Channels[2].Operators[1].Level = 29
	pa, err := rootcontrol.CapturePauseLevels(a)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := rootcontrol.CapturePauseLevels(b)
	if err != nil {
		t.Fatal(err)
	}
	for ch := range a.Channels {
		for op := range a.Channels[ch].Operators {
			a.Channels[ch].Operators[op].Level = 127
			b.Channels[ch].Operators[op].Level = 127
		}
	}
	if !reflect.DeepEqual(a, b) || pa == pb {
		t.Fatal("restore information was not preserved independently")
	}
}

func TestPauseLevelPayloadRejectsOutOfRange(t *testing.T) {
	y := chips.ResetYMState()
	y.Channels[0].Operators[0].Level = 128
	if got, err := rootcontrol.CapturePauseLevels(y); err == nil || got != (rootcontrol.PauseLevels{}) {
		t.Fatal("invalid live level accepted")
	}
	p := rootcontrol.PauseLevels{}
	p.Levels[5][3] = 255
	if got, err := rootcontrol.PauseRestoreWrites(p); err == nil || got != ([24]rootcontrol.LevelWrite{}) {
		t.Fatal("invalid pending level accepted")
	}
}

func TestPausePayloadIsOnlyTwentyFourLevels(t *testing.T) {
	typeOf := reflect.TypeFor[rootcontrol.PauseLevels]()
	if typeOf.NumField() != 1 || typeOf.Field(0).Name != "Levels" || typeOf.Size() != 24 {
		t.Fatal("pending restore payload became a chip/voice mirror", typeOf)
	}
}
