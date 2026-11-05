package runtimebridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

func networkInputs(t *testing.T, arena uint16) AssetBytes {
	t.Helper()
	old := arenaBridgeInputs(t, arena)
	descriptor, e := os.ReadFile("../data/runtime-resource-descriptor.json")
	if e != nil {
		t.Fatal(e)
	}
	return AssetBytes{Rules: old[0], Sprites: old[1], Arena: old[2], HUD: old[3], ArenaAux: old[4], RenderMetadata: arenaBridgeAsset(t, "renderstate-metadata"), Descriptor: descriptor}
}
func networkAssets(t *testing.T, arena uint16) Assets {
	t.Helper()
	a, e := DecodeNetworkAssets(networkInputs(t, arena), arena)
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func networkOptions(arena uint16, characters []uint8, team, computer bool) engine.ChuSessionOptions {
	o := engine.ChuSessionOptions{Arena: arena, Seed: 0x12345678, Difficulty: 6, TeamBattle: team}
	slots := []uint8{0, 3}
	if len(characters) == 3 {
		slots = []uint8{0, 1, 3}
	} else if len(characters) == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	for seat, id := range characters {
		o.Players = append(o.Players, engine.ChuSessionPlayer{Slot: slots[seat], CharacterID: id, Color: uint8(seat), Computer: computer, HumanChannel: uint8(seat)})
	}
	return o
}
func presentationHUDForTest(h renderstate.HUDState, a Assets) (presentation.HUDState, error) {
	return presentation.MaterializeHUDRefs(h, a.hud)
}
func schemaHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func checkpointTest(t *testing.T, s *Session) Checkpoint {
	t.Helper()
	b, e := s.Checkpoint("0123456789abcdef01234567")
	if e != nil {
		t.Fatal(e)
	}
	var c Checkpoint
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func TestSchema1SaveBytesFrozen(t *testing.T) {
	var fixture struct {
		Rows []struct {
			Arena uint16
			Mark  string
			Tick  uint64
			SHA   string `json:"sha256"`
			Bytes int
		}
	}
	raw, e := os.ReadFile("testdata/schema1-save-baseline.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &fixture); e != nil || len(fixture.Rows) != 10 {
		t.Fatal(e, len(fixture.Rows))
	}
	for _, arena := range []uint16{0, 10} {
		old := arenaBridgeInputs(t, arena)
		a, e := DecodeAssetsForArena(old[0], old[1], old[2], old[3], old[4], arena)
		if e != nil {
			t.Fatal(e)
		}
		o := engine.ChuSessionOptions{Arena: arena, Seed: 0x12345678, Difficulty: 6, Players: []engine.ChuSessionPlayer{{Slot: 0, CharacterID: 7, Computer: true}, {Slot: 3, CharacterID: 9, Color: 1, Computer: true}}}
		s, e := NewSession(o, a)
		if e != nil {
			t.Fatal(e)
		}
		check := func(mark string) {
			t.Helper()
			b, e := s.Save()
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, r := range fixture.Rows {
				if r.Arena == arena && r.Mark == mark {
					found = true
					if len(b) != r.Bytes || schemaHash(b) != r.SHA {
						t.Fatal("historical schema1 bytes changed", arena, mark)
					}
				}
			}
			if !found {
				t.Fatal("missing baseline row")
			}
			restored, e := RestoreSession(b, a)
			if e != nil {
				t.Fatal(e)
			}
			again, _ := restored.Save()
			if !bytes.Equal(b, again) {
				t.Fatal("legacy byte restore changed")
			}
		}
		check("prepared")
		if e = s.Ready(); e != nil {
			t.Fatal(e)
		}
		check("ready")
		for tick := 1; tick <= 600; tick++ {
			if _, e = s.Step([8]engine.Buttons{}); e != nil {
				t.Fatal(e)
			}
			if tick == 1 || tick == 240 || tick == 600 {
				check(fmt.Sprint(tick))
			}
		}
	}
}
func TestSchema2AssetsBindExactBytes(t *testing.T) {
	for _, arena := range []uint16{0, 2, 4, 6, 8, 10, 12, 14} {
		a, e := DecodeNetworkAssets(networkInputs(t, arena), arena)
		if e != nil {
			t.Fatal(arena, e)
		}
		s, e := NewSession(networkOptions(arena, []uint8{7, 9}, false, false), a)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := s.Save()
		var saved map[string]json.RawMessage
		_ = json.Unmarshal(raw, &saved)
		if string(saved["schema"]) != "2" || saved["presentation"] == nil || saved["tile_slots"] != nil || saved["hud"] != nil {
			t.Fatal("schema2 kept legacy mutable owners", arena)
		}
	}
	for _, field := range []string{"rules", "sprites", "hud", "metadata", "arena", "aux", "descriptor"} {
		raw := networkInputs(t, 10)
		var target *[]byte
		switch field {
		case "rules":
			target = &raw.Rules
		case "sprites":
			target = &raw.Sprites
		case "hud":
			target = &raw.HUD
		case "metadata":
			target = &raw.RenderMetadata
		case "arena":
			target = &raw.Arena
		case "aux":
			target = &raw.ArenaAux
		case "descriptor":
			target = &raw.Descriptor
		}
		if field == "descriptor" {
			*target = nil
		} else {
			*target = append(append([]byte(nil), (*target)...), ' ')
		}
		if _, e := DecodeNetworkAssets(raw, 10); e == nil {
			t.Fatal("altered exact bytes accepted", field)
		}
	}
}
func TestSchema2PreviewCheckpointAndOwnership(t *testing.T) {
	a := networkAssets(t, 10)
	o := networkOptions(10, []uint8{7, 9}, false, false)
	s, e := NewSession(o, a)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := s.Save()
	p, e := s.Preview()
	after, _ := s.Save()
	if e != nil || len(p.Pixels) != 320*224*4 || !p.View.WindowComplete || p.View.Frame != "1" || !bytes.Equal(before, after) || s.View().Tick != 0 {
		t.Fatal("preview advanced authority or did not render", e)
	}
	if e = s.Ready(); e != nil {
		t.Fatal(e)
	}
	for tick := 1; tick <= 320; tick++ {
		if _, e = s.StepControlled([4]engine.Buttons{}, 0); e != nil {
			t.Fatal(tick, e)
		}
	}
	c := checkpointTest(t, s)
	if c.Version != 1 || c.Frame != 320 || c.Identity.MetadataSHA256 == "" || c.ResourceDescriptorSHA256 == "" {
		t.Fatal("checkpoint identity/frame absent")
	}
	raw, e := s.Checkpoint(c.Epoch)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(raw, []byte("18446744073709551615")) {
		t.Fatal("actual high64 present fixture did not exercise JS risk")
	}
	restored, e := RestoreCheckpoint(raw, a)
	if e != nil {
		t.Fatal(e)
	}
	round, _ := restored.Checkpoint(c.Epoch)
	if !bytes.Equal(raw, round) {
		t.Fatal("opaque checkpoint changed high bits")
	}
	for i := 0; i < 30; i++ {
		left, e := s.StepControlled([4]engine.Buttons{engine.ButtonRight}, 0)
		if e != nil {
			t.Fatal(e)
		}
		right, e := restored.StepControlled([4]engine.Buttons{engine.ButtonRight}, 0)
		if e != nil || !bytes.Equal(left.Pixels, right.Pixels) {
			t.Fatal("checkpoint replay pixel diverged", i, e)
		}
		l, _ := s.Save()
		r, _ := restored.Save()
		if !bytes.Equal(l, r) {
			t.Fatal("checkpoint replay canonical state diverged")
		}
	}
	var lossy any
	if e = json.Unmarshal(raw, &lossy); e != nil {
		t.Fatal(e)
	}
	bad, _ := json.Marshal(lossy)
	if _, e = RestoreCheckpoint(bad, a); e == nil {
		t.Fatal("JS Number-style high64 corruption accepted")
	}
	if _, exists := reflect.TypeOf(snapshot{}).FieldByName("Slots"); exists {
		t.Fatal("common persistent snapshot still owns legacy sprite slots")
	}
	if _, exists := reflect.TypeOf(snapshot{}).FieldByName("HUD"); exists {
		t.Fatal("common snapshot still owns HUD pixels")
	}
}
func TestSchema2RestoreAndStepErrorsAreAtomic(t *testing.T) {
	a := networkAssets(t, 12)
	s, e := NewSession(networkOptions(12, []uint8{7, 9}, false, false), a)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Ready(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 50; i++ {
		if _, e = s.StepControlled([4]engine.Buttons{}, 0); e != nil {
			t.Fatal(e)
		}
	}
	before, _ := s.Save()
	for _, mask := range []uint8{4, 128, 255} {
		if _, e = s.StepControlled([4]engine.Buttons{}, mask); e == nil {
			t.Fatal("invalid automation accepted")
		}
	}
	if _, e = s.StepControlled([4]engine.Buttons{4096}, 0); e == nil {
		t.Fatal("invalid MD bits accepted")
	}
	after, _ := s.Save()
	if !bytes.Equal(before, after) {
		t.Fatal("invalid controlled tick committed")
	}
	c := checkpointTest(t, s)
	for _, kind := range []string{"descriptor", "identity", "arena", "options", "present", "frame-type", "epoch"} {
		bad := c
		switch kind {
		case "descriptor":
			bad.ResourceDescriptorSHA256 = "wrong"
		case "identity":
			bad.Identity.MetadataSHA256 = "wrong"
		case "arena":
			bad.World.Match.Arena = 14
		case "options":
			bad.Options.Arena = 14
		case "present":
			bad.Presentation.SpriteSlots.Present[3] = 0
		case "epoch":
			bad.Epoch = ""
		}
		raw, _ := json.Marshal(bad)
		if kind == "frame-type" {
			raw = bytes.Replace(raw, []byte(`"frame":"50"`), []byte(`"frame":50`), 1)
		}
		if _, e = RestoreCheckpoint(raw, a); e == nil {
			t.Fatal("invalid checkpoint accepted", kind)
		}
	}
	c.Frame = uint64(1)<<53 + 19
	high, _ := json.Marshal(c)
	large, e := RestoreCheckpoint(high, a)
	if e != nil {
		t.Fatal(e)
	}
	if large.View().Frame != strconv.FormatUint(c.Frame, 10) {
		t.Fatal("network frame narrowed to JS Number/logic16")
	}
	if _, e = large.StepControlled([4]engine.Buttons{}, 0); e != nil {
		t.Fatal(e)
	}
	if large.View().Frame != strconv.FormatUint(c.Frame+1, 10) {
		t.Fatal("network uint64 frame did not advance")
	}
}
func TestSchema2FrozenWholeTrajectories(t *testing.T) {
	var f struct {
		Rows []struct {
			Configuration struct {
				Name        string
				Arena       uint16
				Characters  []uint8
				Team        bool
				HeldFixture string `json:"held_fixture"`
			}
			Ticks      int
			Complete   bool
			PanFrames  int    `json:"pan_frames"`
			WorldTrace string `json:"world_trace_sha256"`
			SlotTrace  string `json:"slot_trace_sha256"`
			HUDTrace   string `json:"hud_trace_sha256"`
			PixelTrace string `json:"pixel_trace_sha256"`
		}
	}
	data, e := os.ReadFile("../renderstate/testdata/bridge-reference.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &f); e != nil || len(f.Rows) != 49 {
		t.Fatal(e)
	}
	shards, shard := 1, 0
	if os.Getenv("CLIENT_BRIDGE_SHARDS") != "" {
		shards, e = strconv.Atoi(os.Getenv("CLIENT_BRIDGE_SHARDS"))
		if e != nil || shards < 1 {
			t.Fatal(e)
		}
		shard, e = strconv.Atoi(os.Getenv("CLIENT_BRIDGE_SHARD"))
		if e != nil || shard < 0 || shard >= shards {
			t.Fatal(e)
		}
	}
	for index, row := range f.Rows {
		if index%shards != shard {
			continue
		}
		t.Run(row.Configuration.Name, func(t *testing.T) {
			c := row.Configuration
			a := networkAssets(t, c.Arena)
			o := networkOptions(c.Arena, c.Characters, c.Team, c.HeldFixture == "")
			s, e := NewSession(o, a)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Ready(); e != nil {
				t.Fatal(e)
			}
			var fixed struct {
				Held [][8]engine.Buttons `json:"held"`
			}
			if c.HeldFixture != "" {
				b, e := os.ReadFile("../../../" + c.HeldFixture)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(b, &fixed); e != nil {
					t.Fatal(e)
				}
			}
			wt, st, ht, pt := sha256.New(), sha256.New(), sha256.New(), sha256.New()
			pan, restores := 0, 0
			for tick := 1; tick <= row.Ticks; tick++ {
				var input [4]engine.Buttons
				if fixed.Held != nil {
					copy(input[:], fixed.Held[tick-1][:4])
				}
				mask := uint8(0)
				if fixed.Held == nil {
					mask = uint8((1 << len(c.Characters)) - 1)
				}
				frame, e := s.StepControlled(input, mask)
				if e != nil {
					t.Fatal(tick, e)
				}
				checkpoint := checkpointTest(t, s)
				b, _ := json.Marshal(checkpoint.World)
				wt.Write(b)
				b, _ = json.Marshal(checkpoint.Presentation.SpriteSlots)
				st.Write(b)
				hud, e := presentationHUDForTest(checkpoint.Presentation.HUD, a)
				if e != nil {
					t.Fatal(e)
				}
				b, _ = json.Marshal(hud)
				ht.Write(b)
				pt.Write(frame.Pixels)
				if frame.View.PanOffset != 0 {
					pan++
				}
				if tick == 500 || tick == 1056 && frame.View.PanOffset != 0 {
					saved, _ := s.Save()
					s, e = RestoreSession(saved, a)
					if e != nil {
						t.Fatal(e)
					}
					restores++
				}
			}
			got := []string{hex.EncodeToString(wt.Sum(nil)), hex.EncodeToString(st.Sum(nil)), hex.EncodeToString(ht.Sum(nil)), hex.EncodeToString(pt.Sum(nil))}
			want := []string{row.WorldTrace, row.SlotTrace, row.HUDTrace, row.PixelTrace}
			if !reflect.DeepEqual(got, want) || s.View().Completed != row.Complete || pan != row.PanFrames || restores == 0 {
				t.Fatalf("schema2 changed frozen full trajectory got=%v want=%v pan=%d/%d", got, want, pan, row.PanFrames)
			}
			rd, pd, e := s.Digests()
			if e != nil || rd == "" || pd == "" {
				t.Fatal("canonical digests absent", e)
			}
		})
	}
}
