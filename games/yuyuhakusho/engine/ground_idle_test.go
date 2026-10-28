package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// Original fixtures and these tests precede the idle action and the four
// additional fields it reads/writes on FighterBodyState.
func TestGroundIdleOriginalCalls(t *testing.T) {
	for _, name := range []string{"ground-idle-oracle", "ground-idle-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []struct {
				Before, After struct {
					Body       FighterBodyState
					ActorBytes string `json:"actor_bytes"`
					Pool       struct {
						Cursor uint16
						Bytes  string
					}
					Held       Buttons
					Parameters JumpParameters
				}
				Slots [2]int
			}
			readLaneMotion(t, name, &rows)
			if len(rows) == 0 {
				t.Fatal("empty ground-idle trace")
			}
			for i, r := range rows {
				pool, poolBytes := decodeEffectPool(t, r.Before.Pool.Cursor, r.Before.Pool.Bytes)
				wantPool, afterPool := decodeEffectPool(t, r.After.Pool.Cursor, r.After.Pool.Bytes)
				body, next, slots, err := StepGroundIdle(r.Before.Body, pool, r.Before.Parameters, r.Before.Held)
				before, decodeErr := hex.DecodeString(r.Before.ActorBytes)
				if decodeErr != nil {
					t.Fatal(decodeErr)
				}
				after, decodeErr := hex.DecodeString(r.After.ActorBytes)
				if decodeErr != nil {
					t.Fatal(decodeErr)
				}
				actual := encodeJumpBody(body, before)
				binary.BigEndian.PutUint16(actual[0x78:], uint16(body.Health))
				binary.BigEndian.PutUint16(actual[0x7e:], uint16(body.Auxiliary7E))
				binary.BigEndian.PutUint16(actual[0x80:], uint16(body.Auxiliary80))
				actual[0xc8] = body.TransitionDelay
				if err != nil || body != r.After.Body || next != wantPool || slots != r.Slots || !bytes.Equal(actual, after) || !bytes.Equal(encodeEffectPool(next, poolBytes), afterPool) {
					t.Fatalf("idle call %d: got %+v want %+v slots %v/%v err %v", i, body, r.After.Body, slots, r.Slots, err)
				}
			}
		})
	}
}

func TestGroundIdleErrorsAreAtomic(t *testing.T) {
	body := FighterBodyState{GroundActionState: GroundActionState{ActionKind: 0, Pose: 13, Clock: AnimationClock{Completed: 1}}, Y: 23, Ground: 45, Health: 1}
	pool := EffectPool{Cursor: 1}
	got, p, slots, err := StepGroundIdle(body, pool, JumpParameters{}, 0)
	if err == nil || got != body || p != pool || slots != [2]int{-1, -1} {
		t.Fatal("failed jump partially changed idle state")
	}
	body.ActionKind = 1
	pool.Cursor = 0
	got, p, slots, err = StepGroundIdle(body, pool, JumpParameters{}, 0)
	if err == nil || got != body || p != pool || slots != [2]int{-1, -1} {
		t.Fatal("non-idle action accepted")
	}
}
