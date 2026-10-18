package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type postresultTextRow struct {
	Label         string
	Before, After []byte
	QueueHeaders  []uint16
	Command       uint8
	Message       uint8
}

func postresultTextRows(t *testing.T) (text, commands, messages []postresultTextRow) {
	t.Helper()
	f, e := os.Open("testdata/postresult-text.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var r struct{ Text, Commands, Messages []postresultTextRow }
	if e = json.NewDecoder(z).Decode(&r); e != nil {
		t.Fatal(e)
	}
	return r.Text, r.Commands, r.Messages
}
func postresultTextProgressRAM(r []byte) SceneTextProgress {
	return SceneTextProgress{Address: uint16(r[0x6ae])<<8 | uint16(r[0x6af]), Cursor: uint16(r[0x6b0]) | uint16(r[0x6b1])<<8, Repeat: r[0x6b2], Clock: r[0x6b3], Pause: r[0x6b4], Delay: r[0x6b5], LineAddress: uint16(r[0x6b6]) | uint16(r[0x6b7])<<8}
}
func postresultTextUploadsRAM(r []byte, base int) (out [2]EndingUpload) {
	for i := range out {
		a := base + i*35
		out[i].Address = uint16(r[a])<<8 | uint16(r[a+1])
		out[i].Length = r[a+2]
		copy(out[i].Data[:], r[a+3:a+35])
	}
	return
}
func postresultTextStateRAM(r []byte) SceneTextStep {
	return SceneTextStep{Context: resultActorContextRAM(r), Active: r[0x6ac], Progress: postresultTextProgressRAM(r), Uploads: postresultTextUploadsRAM(r, 0x30b)}
}
func postresultTextQueue(headers []uint16, base uint16) uint8 {
	var mask uint8
	for _, h := range headers {
		if h == base {
			mask |= 1
		}
		if h == base+35 {
			mask |= 2
		}
	}
	return mask
}
func TestOriginalSceneTextSourcesAndContinuousRestore(t *testing.T) {
	rows, _, _ := postresultTextRows(t)
	var prior SceneTextStep
	series := ""
	for i, r := range rows {
		input, want := postresultTextStateRAM(r.Before), postresultTextStateRAM(r.After)
		nextSeries := ""
		if strings.HasPrefix(r.Label, "stream-") {
			nextSeries = strings.Split(r.Label, "-tick-")[0]
		}
		if nextSeries != "" && series == nextSeries {
			prior.QueueMask = 0
			prior.ClearPaletteOverrides = false
			if prior != input {
				t.Fatal(r.Label, "continuous text input diverged")
			}
			input = prior
		}
		got, e := StepOriginalSceneText(r.Before[0x588], input.Context, input.Active, input.Progress, input.Uploads)
		if e != nil {
			t.Fatal(r.Label, e)
		}
		want.QueueMask = postresultTextQueue(r.QueueHeaders, 0x30b)
		want.ClearPaletteOverrides = true
		if got != want {
			t.Fatalf("%s got %+v want %+v", r.Label, got, want)
		}
		if i%97 == 0 {
			b, _ := json.Marshal(input)
			var restored SceneTextStep
			if e = json.Unmarshal(b, &restored); e != nil {
				t.Fatal(e)
			}
			repeat, e := StepOriginalSceneText(r.Before[0x588], restored.Context, restored.Active, restored.Progress, restored.Uploads)
			if e != nil || repeat != got {
				t.Fatal(r.Label, "restore", e)
			}
		}
		prior = got
		series = nextSeries
	}
	t.Logf("matched %d complete original FD0A calls, including37 continuous text streams", len(rows))
}
func TestOriginalSceneTextControlHandlers(t *testing.T) {
	_, rows, _ := postresultTextRows(t)
	for _, r := range rows {
		got, want := postresultTextStateRAM(r.Before), postresultTextStateRAM(r.After)
		if e := applyOriginalSceneTextControl(&got, r.Command); e != nil {
			t.Fatal(r.Label, e)
		}
		want.QueueMask = postresultTextQueue(r.QueueHeaders, 0x30b)
		if got != want {
			t.Fatalf("%s got %+v want %+v", r.Label, got, want)
		}
	}
}
func TestOriginalPostResultMessageSources(t *testing.T) {
	_, _, rows := postresultTextRows(t)
	for _, r := range rows {
		input := postresultTextUploadsRAM(r.Before, 0x351)
		got, e := PrepareOriginalPostResultMessage(r.Before[0x588], MatchPhase(r.Before[0x589]), r.Message, input)
		if e != nil {
			t.Fatal(r.Label, e)
		}
		if got.Uploads != postresultTextUploadsRAM(r.After, 0x351) || got.QueueMask != postresultTextQueue(r.QueueHeaders, 0x351) || !got.WaitNextNMI {
			t.Fatalf("%s got %+v", r.Label, got)
		}
	}
	t.Logf("matched %d original C036 message preparations before their NMI wait", len(rows))
}
func TestOriginalSceneTextFailureAtomicity(t *testing.T) {
	rows, _, _ := postresultTextRows(t)
	base := postresultTextStateRAM(rows[0].Before)
	for _, kind := range []string{"scene", "stream", "cursor"} {
		in, scene := base, uint8(7)
		switch kind {
		case "scene":
			scene = 8
		case "stream":
			in.Context.TextID = 37
		case "cursor":
			in.Active = 128
			in.Progress.Clock = 1
			in.Progress.Cursor = 65534
		}
		got, e := StepOriginalSceneText(scene, in.Context, in.Active, in.Progress, in.Uploads)
		if e == nil || !reflect.DeepEqual(got, in) {
			t.Fatal(kind, "failed call changed state", e)
		}
	}
	got, e := PrepareOriginalPostResultMessage(1, 7, 31, base.Uploads)
	if e == nil || got.Uploads != base.Uploads || got.QueueMask != 0 || got.WaitNextNMI {
		t.Fatal("invalid message changed buffers", e)
	}
}
