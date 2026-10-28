package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The complete original prelude fixtures and this test precede its Go code.
func TestActionPreludeOriginalCalls(t *testing.T) {
	for _, name := range []string{"action-prelude-oracle", "action-prelude-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []struct {
				Before, After ActionPreludeState
				ActorBefore   string `json:"actor_before"`
				ActorAfter    string `json:"actor_after"`
				Dispatch      bool
			}
			readLaneMotion(t, name, &rows)
			if len(rows) == 0 {
				t.Fatal("empty action prelude fixture")
			}
			for i, r := range rows {
				got, dispatch := PrepareActionStep(r.Before)
				before, err := hex.DecodeString(r.ActorBefore)
				if err != nil {
					t.Fatal(err)
				}
				after, err := hex.DecodeString(r.ActorAfter)
				if err != nil {
					t.Fatal(err)
				}
				actual := encodeJumpBody(got.Body, before)
				binary.BigEndian.PutUint16(actual[0x78:], uint16(got.Body.Health))
				binary.BigEndian.PutUint16(actual[0x7e:], uint16(got.Body.Auxiliary7E))
				binary.BigEndian.PutUint16(actual[0x80:], uint16(got.Body.Auxiliary80))
				actual[0xc8] = got.Body.TransitionDelay
				actual[0x69], actual[0x6a], actual[0x86] = got.HitStopTicks, got.Shake, got.StatusDelay
				binary.BigEndian.PutUint16(actual[0x54:], uint16(got.ScreenOffsetX))
				binary.BigEndian.PutUint32(actual[0x98:], uint32(got.Residual))
				if got != r.After || dispatch != r.Dispatch || !bytes.Equal(actual, after) {
					t.Fatalf("prelude %d: got %+v/%v want %+v/%v", i, got, dispatch, r.After, r.Dispatch)
				}
			}
		})
	}
}
