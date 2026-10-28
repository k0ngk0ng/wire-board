package contra

// Failure cases before Boss door implementation: siren/sound-region mismatch;
// missing audio reset or retaining pre-reset sounds; clearing delay-high too
// early; wrong destroy-all exemptions; advancing tunnel after queue refusal;
// premature collision changes; full effect pool; early platform victory.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalBossDoorStates(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-bossdoor.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	type globals struct{ DelayLow, DelayHigh, Boss uint8 }
	type audioEvent struct {
		Kind string
		Code uint8
	}
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct{ Routine, Scroll, Used uint8 }
		Records  []struct {
			Case                                               int
			Before, After                                      [][]uint8
			GlobalBefore, GlobalAfter                          globals
			Audio                                              []audioEvent
			Requests                                           []struct{ X, Y, Code, Used uint8 }
			CollisionBefore, CollisionAfter, Supertiles, Queue string
			Used                                               uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Records) != 102 {
		t.Fatal("Boss door coverage changed")
	}
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range oracle.Records {
		input := oracle.Cases[r.Case-1]
		rawMap, _ := hex.DecodeString(r.Supertiles)
		mapped := ([128]uint8)(rawMap)
		collision, _ := hex.DecodeString(r.CollisionBefore)
		terrain := CollisionWindow(collision)
		c := EnemyStepContext{Level: 0, FrameScroll: input.Scroll, Terrain: &terrain, Scroll: CollisionScroll{Y: 224, NameTable: 0xb0}}
		q := GraphicsQueue{Used: input.Used}
		requests := 0
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			if requests >= len(r.Requests) {
				t.Fatalf("door%d unexpected request%+v", r.Case, req)
			}
			want := r.Requests[requests]
			requests++
			if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
				t.Fatalf("door%d graphic request differs", r.Case)
			}
			v, err := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, err
		}
		s := EnemySystem{Slots: enemyPoolFromOracle(r.Before)}
		signal, err := s.StepBossDoor(15, c)
		if err != nil {
			t.Fatalf("door%d: %v", r.Case, err)
		}
		world := r.GlobalBefore
		actualAudio := []audioEvent{}
		switch signal {
		case BossDoorNoSignal:
		case BossDoorDefeated:
			world.Boss = 1
			world.DelayLow = 255
			actualAudio = append(actualAudio, audioEvent{Kind: "reset"})
		case BossDoorTunnelFinished:
			world.DelayLow = 48
			world.DelayHigh = 0
		default:
			t.Fatal("unknown signal")
		}
		for _, code := range s.PendingSounds {
			actualAudio = append(actualAudio, audioEvent{Kind: "play", Code: code})
		}
		want := enemyPoolFromOracle(r.After)
		if s.Slots != want {
			for i, v := range s.Slots {
				if v != want[i] {
					t.Fatalf("door%d phase%d slot%d got%+v want%+v before%+v", r.Case, input.Routine, i, v, want[i], enemyFromOracle(r.Before[i]))
				}
			}
		}
		if world != r.GlobalAfter || !reflect.DeepEqual(actualAudio, r.Audio) || requests != len(r.Requests) {
			t.Fatalf("door%d global/audio/request got%+v/%+v want%+v/%+v", r.Case, world, actualAudio, r.GlobalAfter, r.Audio)
		}
		wantCollision, _ := hex.DecodeString(r.CollisionAfter)
		if !bytes.Equal(terrain[:], wantCollision) {
			t.Fatalf("door%d collision differs", r.Case)
		}
		queue := make([]uint8, 128)
		at := int(input.Used)
		for _, packet := range q.Packets {
			data, err := packet.Encode()
			if err != nil {
				t.Fatal(err)
			}
			copy(queue[at:], data)
			at += len(data)
		}
		wantQueue, _ := hex.DecodeString(r.Queue)
		if q.Used != r.Used || !bytes.Equal(queue, wantQueue) {
			t.Fatalf("door%d graphics queue differs", r.Case)
		}
	}
}
