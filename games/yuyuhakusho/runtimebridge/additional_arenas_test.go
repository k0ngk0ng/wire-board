package runtimebridge

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func arenaBridgeAsset(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../data/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func arenaBridgeInputs(t *testing.T, arena uint16) [5][]byte {
	t.Helper()
	name := "arena-assets-default"
	if arena != 0 {
		name = fmt.Sprintf("arena-assets-%x", arena)
	}
	raw := [5][]byte{arenaBridgeAsset(t, "chu-world-rules"), arenaBridgeAsset(t, "world-sprites"), arenaBridgeAsset(t, name), arenaBridgeAsset(t, "hud-assets")}
	if arena == 10 {
		raw[4] = arenaBridgeAsset(t, "arena-a-aux-assets")
	}
	return raw
}

func arenaBridgeOptions(arena uint16, first uint8) engine.ChuSessionOptions {
	return engine.ChuSessionOptions{Arena: arena, Seed: 0x12345678, Difficulty: 6, Players: []engine.ChuSessionPlayer{
		{Slot: 0, CharacterID: first, Color: 0, HumanChannel: 0},
		{Slot: 3, CharacterID: (first + 4) % 11, Color: 1, Computer: true},
	}}
}

func arenaBridgeSave(t *testing.T, session *Session) []byte {
	t.Helper()
	raw, err := session.Save()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAdditionalArenaPrepareRenderAndRestore(t *testing.T) {
	for i, arena := range []uint16{2, 4, 6, 8, 10, 12, 14} {
		t.Run(fmt.Sprintf("%x", arena), func(t *testing.T) {
			raw := arenaBridgeInputs(t, arena)
			assets, err := DecodeAssetsForArena(raw[0], raw[1], raw[2], raw[3], raw[4], arena)
			if err != nil {
				t.Fatal(err)
			}
			session, err := NewSession(arenaBridgeOptions(arena, uint8(i)), assets)
			if err != nil {
				t.Fatal(err)
			}
			before := arenaBridgeSave(t, session)
			if _, err := session.Step([8]engine.Buttons{}); err == nil || !bytes.Equal(before, arenaBridgeSave(t, session)) {
				t.Fatal("unready selected arena advanced or changed render state", err)
			}
			if err := session.Ready(); err != nil {
				t.Fatal(err)
			}
			before = arenaBridgeSave(t, session)
			if err := session.Ready(); err == nil || !bytes.Equal(before, arenaBridgeSave(t, session)) {
				t.Fatal("duplicate readiness changed selected arena", err)
			}
			if _, err := session.Step([8]engine.Buttons{0x1000}); err == nil || !bytes.Equal(before, arenaBridgeSave(t, session)) {
				t.Fatal("invalid input partially changed selected arena", err)
			}
			var restored *Session
			pixels := make(map[[32]byte]bool)
			spriteFrames := 0
			for tick := 1; tick <= 700; tick++ {
				var held [8]engine.Buttons
				if tick >= 320 && tick < 345 {
					held[0] = engine.Buttons(8)
				}
				frame, err := session.Step(held)
				if err != nil || len(frame.Pixels) != 320*224*4 || !frame.View.WindowComplete {
					t.Fatalf("selected arena %x tick %d: %v", arena, tick, err)
				}
				if frame.SpritePixels > 0 {
					spriteFrames++
				}
				if tick%100 == 0 {
					pixels[sha256.Sum256(frame.Pixels)] = true
				}
				if restored != nil && tick <= 380 {
					other, err := restored.Step(held)
					if err != nil || !bytes.Equal(frame.Pixels, other.Pixels) || !reflect.DeepEqual(frame.View, other.View) || !bytes.Equal(arenaBridgeSave(t, session), arenaBridgeSave(t, restored)) {
						t.Fatal("restored arena state, pixels or view diverged", tick, err)
					}
				}
				if tick == 350 {
					restored, err = RestoreSession(arenaBridgeSave(t, session), assets)
					if err != nil {
						t.Fatal(err)
					}
				}
				if frame.View.Players[0].Computer || frame.View.Players[0].InputChannel != 0 {
					t.Fatal("human was reassigned while idle or restoring")
				}
			}
			if len(pixels) < 3 || spriteFrames < 100 || session.saved.World.Match.Arena != arena {
				t.Fatal("arena has no continuing complete presentation", len(pixels), spriteFrames)
			}
		})
	}
}

func TestAdditionalArenaAssetsAndSaveBinding(t *testing.T) {
	raw := arenaBridgeInputs(t, 10)
	assets, err := DecodeAssetsForArena(raw[0], raw[1], raw[2], raw[3], raw[4], 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, arena := range []uint16{0, 1, 2, 4, 6, 8, 12, 14, 16, 65534} {
		if _, err := DecodeAssetsForArena(raw[0], raw[1], raw[2], raw[3], raw[4], arena); err == nil {
			t.Fatal("accepted mismatched selected catalog", arena)
		}
		if _, err := NewSession(arenaBridgeOptions(arena, 9), assets); err == nil {
			t.Fatal("prepared against another arena's loaded assets", arena)
		}
	}
	if _, err := DecodeAssetsForArena(raw[0], raw[1], raw[2], raw[3], nil, 10); err == nil {
		t.Fatal("arena a opened without its auxiliary material")
	}
	defaultRaw := arenaBridgeInputs(t, 0)
	if _, err := DecodeAssetsForArena(defaultRaw[0], defaultRaw[1], defaultRaw[2], defaultRaw[3], raw[4], 0); err == nil {
		t.Fatal("default arena accepted unrelated auxiliary material")
	}
	session, err := NewSession(arenaBridgeOptions(10, 9), assets)
	if err != nil {
		t.Fatal(err)
	}
	saved := arenaBridgeSave(t, session)
	for _, mutation := range []string{"options", "match", "aux-digest", "background-digest"} {
		var changed snapshot
		if err := json.Unmarshal(saved, &changed); err != nil {
			t.Fatal(err)
		}
		switch mutation {
		case "options":
			changed.Options.Arena = 2
		case "match":
			changed.World.Match.Arena = 2
		case "aux-digest":
			changed.ArenaAuxDigest = "different"
		case "background-digest":
			changed.AssetDigests[2] = "different"
		}
		data, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RestoreSession(data, assets); err == nil {
			t.Fatal("restored inconsistent selected arena", mutation)
		}
	}
	if !bytes.Equal(saved, arenaBridgeSave(t, session)) {
		t.Fatal("failed restore mutated running independent session")
	}
}
