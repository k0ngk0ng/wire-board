package resourcebridge_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func workResource(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("../../data/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestInstrumentResourceSetIdentityAndRealDriver(t *testing.T) {
	s := workResource(t, "audio-score.json.gz")
	p := workResource(t, "audio-pcm-payloads.json.gz")
	m := workResource(t, "audio-score-parser-work.json.gz")
	w := workResource(t, "audio-score-instrument-work.json.gz")
	legacy, err := resourcebridge.Decode(s, p, m)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := resourcebridge.DecodeWithInstrumentWork(s, p, m, w)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Identity() == bound.Identity() {
		t.Fatal("fourth resource omitted from identity")
	}
	d, err := score.New(bound.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: 0x95}); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, voice := range d.Snapshot().Voices {
		if voice.Owned && len(voice.Channel) >= 2 && voice.Channel[:2] == "fm" {
			if _, ok := bound.Catalog().Instrument(voice.Tone); !ok {
				t.Fatal("resource set returned legacy score")
			}
			seen++
		}
	}
	if seen == 0 {
		t.Fatal("real source dispatch did not create FM voices")
	}
	if _, err = d.AdvanceObserved(score.ObservedInput{}, score.ObservedHooks{Parser: bound.Parser(), Operation: func(score.Operation) error { return nil }, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }}); err != nil {
		t.Fatal("parser not bound to same score variant", err)
	}
	config, err := bound.PCMConfig(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	device, err := pcmcommands.New(config)
	if err != nil {
		t.Fatal(err)
	}
	old, err := legacy.PCMConfig(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pcmcommands.Restore(old, device.Snapshot()); err == nil {
		t.Fatal("checkpoint ignored instrument resource identity")
	}
	if len(config.Descriptors) != 129 {
		t.Fatal("encoded payload closure changed")
	}
	if _, err = resourcebridge.DecodeWithInstrumentWork(s, p, m, bytes.Repeat([]byte("x"), 16)); err == nil {
		t.Fatal("broken fourth resource accepted")
	}
}
