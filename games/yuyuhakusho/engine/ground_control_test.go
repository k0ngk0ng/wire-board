package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// Written after original evidence and before StepGroundControl implementation.
func TestGroundControlAgainstOriginal(t *testing.T) {
	compressed, err := os.ReadFile("testdata/ground-control-oracle.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scope         string
		Slots         [2]int
		Before, After struct {
			Body     FighterBodyState
			Controls FighterControls
			Pool     struct {
				Cursor uint16
				Bytes  string
			}
			Occupied uint8 `json:"occupied_lanes"`
		}
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	mraw, err := os.ReadFile("testdata/ground-control-oracle-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass    bool
		Count   int    `json:"complete_calls"`
		Natural int    `json:"natural_calls"`
		SHA     string `json:"fixture_sha256"`
	}
	if err = json.Unmarshal(mraw, &m); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if !m.Pass || m.Count != len(rows) || m.Natural == 0 || hex.EncodeToString(hash[:]) != m.SHA {
		t.Fatal("unverified ground-control evidence")
	}
	for i, r := range rows {
		before, rawBefore := decodeEffectPool(t, r.Before.Pool.Cursor, r.Before.Pool.Bytes)
		want, rawAfter := decodeEffectPool(t, r.After.Pool.Cursor, r.After.Pool.Bytes)
		body, pool, occupied, slots, err := StepGroundControl(r.Before.Body, before, r.Before.Occupied, r.Before.Controls)
		if err != nil || body != r.After.Body || pool != want || occupied != r.After.Occupied || slots != r.Slots || !bytes.Equal(encodeEffectPool(pool, rawBefore), rawAfter) {
			t.Fatalf("ground-control call %d (%s) differs: body=%+v want=%+v slots=%v/%v err=%v", i, r.Scope, body, r.After.Body, slots, r.Slots, err)
		}
	}
}

func TestGroundControlInvalidPoolIsAtomic(t *testing.T) {
	body := FighterBodyState{X: 123}
	pool := EffectPool{Cursor: 1}
	got, p, occupied, slots, err := StepGroundControl(body, pool, 0x55, FighterControls{Pressed: ButtonY})
	if err == nil || got != body || p != pool || occupied != 0x55 || slots != [2]int{-1, -1} {
		t.Fatal("invalid ground-control input caused partial mutation")
	}
}
