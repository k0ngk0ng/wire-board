package cs16

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInputArchiveScannerMatchesStandardJSONValidity(t *testing.T) {
	cases := []string{
		`null`, `true`, `false`, `0`, `-0`, `1234567890`, `-123.456e+789`, `1e999999`, `""`, `"a\\b\"c\u0000\ud800"`,
		`[]`, `{}`, ` [ null, true, false, -42, "x", {"a":[]} ] `,
		`{"world":{"players":[{"seat":0},{"seat":1}],"escaped":"} ] , \\\" world"},"schema":2}`,
		`{"world":{"players":[{},{}]},"world":{"players":[{},{}]}}`,
		``, ` `, `nul`, `True`, `01`, `-01`, `+1`, `.1`, `1.`, `1e`, `1e+`, `--1`, `NaN`, `Infinity`,
		`[1,]`, `[,1]`, `[1 2]`, `{"a":1,}`, `{"a" 1}`, `{"a":}`, `{"a":1 "b":2}`, `{} {}`, `null false`,
		`"\x"`, `"\u123"`, `"\uZZZZ"`, `"unterminated`, "\"line\nfeed\"", "\"\x01\"",
		strings.Repeat("[", 10000) + "0" + strings.Repeat("]", 10000),
		strings.Repeat("[", 10001) + "0" + strings.Repeat("]", 10001),
	}
	for _, text := range cases {
		_, valid := scanInputArchive([]byte(text))
		if valid != json.Valid([]byte(text)) {
			t.Fatal("single scan changed JSON syntax acceptance", text[:min(120, len(text))], valid)
		}
	}
	_, match := cursorFixture(t)
	for end := 0; end <= len(match.Data); end++ {
		raw := match.Data[:end]
		_, valid := scanInputArchive(raw)
		if valid != json.Valid(raw) {
			t.Fatal("truncated snapshot syntax differs", end, valid)
		}
	}
}

func TestInputArchiveScannerFindsOnlyExactRootWorldAndPlayers(t *testing.T) {
	raw := []byte(` {"extension":{"world":{"players":[{"seat":99}]},"quoted":"players [ ] }"},"\u0077orld":{"players":[{"seat":0,"nested":[{"players":[null]}]},{"seat":1}],"frame":17},"observedAt":123} `)
	spans, valid := scanInputArchive(raw)
	if !valid || spans.duplicate || len(spans.players) != 2 || string(raw[spans.root["observedAt"].begin:spans.root["observedAt"].end]) != "123" || string(raw[spans.world["frame"].begin:spans.world["frame"].end]) != "17" {
		t.Fatal("wrong archive field span", valid, spans)
	}
	for index, span := range spans.players {
		var value struct {
			Seat int `json:"seat"`
		}
		if json.Unmarshal(raw[span.begin:span.end], &value) != nil || value.Seat != index {
			t.Fatal("player span includes different entity", index)
		}
	}
	for _, duplicated := range []string{`{"world":{},"\u0077orld":{}}`, `{"world":{"players":[],"players":[]}}`} {
		result, ok := scanInputArchive([]byte(duplicated))
		if !ok || !result.duplicate {
			t.Fatal("duplicate key did not require ordinary decode fallback", duplicated)
		}
	}
	_, match := cursorFixture(t)
	before := bytes.Clone(match.Data)
	if _, ok := scanInputArchive(match.Data); !ok || !bytes.Equal(before, match.Data) {
		t.Fatal("scan mutated input snapshot")
	}
}
