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

// Written after original evidence and before BeginLaneSwitch implementation.
func TestLaneSwitchAgainstOriginal(t *testing.T) {
	compressed, err := os.ReadFile("testdata/lane-switch-oracle.json.gz")
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
			Body FighterBodyState
			Pool struct {
				Cursor uint16
				Bytes  string
			}
			Occupied uint8 `json:"occupied_lanes"`
		}
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	mraw, err := os.ReadFile("testdata/lane-switch-oracle-manifest.json")
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
		t.Fatal("unverified lane switch evidence")
	}
	for i, r := range rows {
		before, rawBefore := decodeEffectPool(t, r.Before.Pool.Cursor, r.Before.Pool.Bytes)
		want, rawAfter := decodeEffectPool(t, r.After.Pool.Cursor, r.After.Pool.Bytes)
		body, pool, occupied, slots, err := BeginLaneSwitch(r.Before.Body, before, r.Before.Occupied)
		if err != nil || body != r.After.Body || pool != want || occupied != r.After.Occupied || slots != r.Slots || !bytes.Equal(encodeEffectPool(pool, rawBefore), rawAfter) {
			t.Fatalf("lane call %d (%s) differs: body=%+v want=%+v slots=%v/%v err=%v", i, r.Scope, body, r.After.Body, slots, r.Slots, err)
		}
	}
}

func TestLaneSwitchInvalidPoolIsAtomic(t *testing.T) {
	body := FighterBodyState{X: 123}
	pool := EffectPool{Cursor: 1}
	got, p, occupied, slots, err := BeginLaneSwitch(body, pool, 0x55)
	if err == nil || got != body || p != pool || occupied != 0x55 || slots != [2]int{-1, -1} {
		t.Fatal("invalid lane input caused partial mutation")
	}
}
