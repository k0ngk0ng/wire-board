package contra

// Continuous acceptance: retain the Go door and collision state between calls;
// only independently owned external entities/graphics work come from the source
// frame. This catches drift that isolated entry/exit samples could conceal.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestContinuousOriginalBossDoor(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-bossdoor-sequence.json.gz")
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
		Original  bool `json:"source_is_original_execution"`
		Sequences []struct {
			Mode    int
			Records []struct {
				Frame, Slot                                                          int
				Scroll, ScrollX, ScrollY, Control                                    uint8
				Before, After                                                        [][]uint8
				GlobalBefore, GlobalAfter                                            globals
				Audio                                                                []audioEvent
				Requests                                                             []struct{ X, Y, Code, Used uint8 }
				CollisionBefore, CollisionAfter, Supertiles, QueueBefore, QueueAfter string
				UsedBefore, UsedAfter                                                uint8
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original || len(oracle.Sequences) != 2 {
		t.Fatal("sequence coverage changed")
	}
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	count, rejected := 0, 0
	for _, seq := range oracle.Sequences {
		var live Enemy
		var terrain CollisionWindow
		started := false
		defeated, finished := 0, 0
		for _, r := range seq.Records {
			pool := enemyPoolFromOracle(r.Before)
			beforeCollision, _ := hex.DecodeString(r.CollisionBefore)
			if started {
				if live != pool[r.Slot] || !bytes.Equal(terrain[:], beforeCollision) {
					t.Fatalf("sequence%d frame%d closed state drifted", seq.Mode, r.Frame)
				}
			} else {
				live = pool[r.Slot]
				terrain = CollisionWindow(beforeCollision)
				started = true
			}
			pool[r.Slot] = live
			mapBytes, _ := hex.DecodeString(r.Supertiles)
			mapped := ([128]uint8)(mapBytes)
			c := EnemyStepContext{Level: 0, FrameScroll: r.Scroll, Terrain: &terrain, Scroll: CollisionScroll{X: r.ScrollX, Y: r.ScrollY, NameTable: r.Control}}
			q := GraphicsQueue{Used: r.UsedBefore}
			requests := 0
			c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
				if requests >= len(r.Requests) {
					t.Fatalf("sequence%d frame%d extra request", seq.Mode, r.Frame)
				}
				want := r.Requests[requests]
				requests++
				if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
					t.Fatal("continuous graphic input differs")
				}
				v, err := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
				if !v.Accepted {
					rejected++
				}
				return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, err
			}
			s := EnemySystem{Slots: pool}
			signal, err := s.StepBossDoor(r.Slot, c)
			if err != nil {
				t.Fatal(err)
			}
			g := r.GlobalBefore
			audio := []audioEvent{}
			switch signal {
			case BossDoorDefeated:
				defeated++
				g.Boss = 1
				g.DelayLow = 255
				audio = append(audio, audioEvent{Kind: "reset"})
			case BossDoorTunnelFinished:
				finished++
				g.DelayLow = 48
				g.DelayHigh = 0
			}
			for _, code := range s.PendingSounds {
				audio = append(audio, audioEvent{Kind: "play", Code: code})
			}
			want := enemyPoolFromOracle(r.After)
			if s.Slots != want || g != r.GlobalAfter || !reflect.DeepEqual(audio, r.Audio) || requests != len(r.Requests) {
				t.Fatalf("sequence%d frame%d state/global/audio/request differs", seq.Mode, r.Frame)
			}
			wantCollision, _ := hex.DecodeString(r.CollisionAfter)
			if !bytes.Equal(terrain[:], wantCollision) {
				t.Fatalf("sequence%d frame%d collision differs", seq.Mode, r.Frame)
			}
			queue, _ := hex.DecodeString(r.QueueBefore)
			at := int(r.UsedBefore)
			for _, packet := range q.Packets {
				raw, err := packet.Encode()
				if err != nil {
					t.Fatal(err)
				}
				copy(queue[at:], raw)
				at += len(raw)
			}
			wantQueue, _ := hex.DecodeString(r.QueueAfter)
			if q.Used != r.UsedAfter || !bytes.Equal(queue, wantQueue) {
				t.Fatalf("sequence%d frame%d queue differs", seq.Mode, r.Frame)
			}
			live = s.Slots[r.Slot]
			count++
		}
		if live.Routine != 0 || finished != 1 || defeated != 1-seq.Mode {
			t.Fatalf("sequence%d terminal signal differs", seq.Mode)
		}
	}
	if count != 180 || rejected != 1 {
		t.Fatalf("continuous coverage calls%d refused%d", count, rejected)
	}
}
