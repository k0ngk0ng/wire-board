package pal

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
)

var stateJSONCases = [][]byte{
	[]byte(`{"native":"AAEC","players":1}`),
	[]byte(`{"players":1,"native":"AAEC","controls":[]}`),
	[]byte(" \n{\"native\":\"AAEC\",\"players\":1}\t"),
	[]byte(`{"native":"\/w==","players":1}`),
	[]byte(`{"\u006eative":"AAEC","players":1}`),
	[]byte(`{"native":"A\nAEC","players":1}`),
	[]byte(`{"native":"AAEC"}`),
	[]byte(`{"native":"AAEC","native":"Ag=="}`),
	[]byte(`{"native":"AAEC","native":null}`),
	[]byte(`{"native":"AAEC","\u006eative":null}`),
	[]byte(`{"native":"AAEC","\u006eative":"Ag=="}`),
	[]byte(`{"native":"AAEC","public":{"dialogue":{"text":"\"native\":\"Ag==\""}}}`),
	[]byte(`{"public":{"dialogue":{"text":"native"}},"native":"AAEC"}`),
	[]byte(`{}`),
	[]byte(`{"native":null}`),
	[]byte(`{"native":""}`),
	[]byte(`{"native":12}`),
	[]byte(`{"native":{}}`),
	[]byte(`{"native":[]}`),
	[]byte(`{"native":"AAEC","unknown":1}`),
	[]byte(`{"native":"AAEC","public":{"unknown":1}}`),
	[]byte(`{"native":"AAEC","native":12}`),
	[]byte(`{"native":"AAEC","native":"?"}`),
	[]byte(`{"native":"AAEC","players":1} {}`),
	[]byte(`{"native":"AAEC","players":1}x`),
	[]byte(`{"native":"AAEC","players":1,}`),
	[]byte(`{"native":"",}`),
	[]byte("{\"native\":\"AAEC\", \r\n }"),
	[]byte(`{"native":"AAEC""players":1}`),
	[]byte(`{"native":"AAEC"`),
	[]byte(`{"native":"AAE}`),
	[]byte(`{"native":"?"}`),
	[]byte(`{"native":"AAE"}`),
	[]byte(`{"native":"AA==="}`),
	[]byte(`{"native":"AA\"EC"}`),
	[]byte("{\"native\":\"AA\nEC\"}"),
	[]byte("{\"native\":\"AA\rEC\"}"),
	[]byte(`null`),
	[]byte(`[]`),
	nil,
}

func compareStateJSON(t *testing.T, raw []byte) {
	t.Helper()
	before := bytes.Clone(raw)
	var want state
	wantError := strict(raw, &want, 48<<20)
	got, gotError := decodeStateJSON(raw)
	if (wantError == nil) != (gotError == nil) || gotError == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("state JSON parser differs from strict decoder: bytes=%d prefix=%q, got %v, want %v", len(raw), raw[:min(len(raw), 128)], gotError, wantError)
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("state JSON parser modified persisted input")
	}
}

func TestStateJSONMatchesStrictRepresentationsAndFailures(t *testing.T) {
	for _, raw := range stateJSONCases {
		compareStateJSON(t, raw)
	}
}

func TestStateJSONSizeBoundariesAndOwnedConcurrentPayloads(t *testing.T) {
	if _, err := decodeStateJSON(bytes.Repeat([]byte{' '}, 48<<20+1)); err == nil {
		t.Fatal("oversized complete state JSON accepted")
	}
	for _, size := range []int{32 << 20, 32<<20 + 1} {
		raw := append([]byte(`{"native":"`), []byte(base64.StdEncoding.EncodeToString(make([]byte, size)))...)
		raw = append(raw, []byte(`","players":1}`)...)
		next, err := decodeStateJSON(raw)
		if size == 32<<20 && (err != nil || len(next.Native) != size) || size > 32<<20 && err == nil {
			t.Fatal("native state JSON size boundary changed", size, err)
		}
		if size > 32<<20 {
			repeated := append(bytes.Clone(raw[:len(raw)-1]), []byte(`,"native":"AAEC"}`)...)
			compareStateJSON(t, repeated)
		}
	}
	raw := []byte(`{"native":"AAEC","public":{"party":[{"role":0,"name":"李逍遥"}]},"players":1}`)
	want, err := decodeStateJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Go(func() {
			for repeat := 0; repeat < 8; repeat++ {
				got, failure := decodeStateJSON(raw)
				if failure != nil || !reflect.DeepEqual(got, want) {
					t.Error("concurrent JSON decoding mixed state", failure)
					return
				}
				got.Native[0] = 99
				got.Public.Party[0].Name = "changed detached output"
			}
		})
	}
	workers.Wait()
}

func TestStateJSONSerializerKeepsNativeFirst(t *testing.T) {
	raw, err := json.Marshal(state{Native: []byte{0, 1, 2}, Players: 1})
	if err != nil || !bytes.HasPrefix(raw, []byte(`{"native":"AAEC",`)) {
		t.Fatal("new state does not use the compatible native-first JSON layout", err)
	}
}

func FuzzStateJSONMatchesStrictDecoder(f *testing.F) {
	for _, raw := range stateJSONCases {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1024*1024 {
			t.Skip()
		}
		compareStateJSON(t, raw)
	})
}
