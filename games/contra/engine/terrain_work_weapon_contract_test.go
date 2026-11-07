package contra

// Failure inventory before the caller correction: weapon collision receives a
// raw platform kind instead of DFB8's solid-support interpretation, so bullets
// pass a platform backed by a solid cell. Inputs and complete pool outcomes
// below come from real stage-six player/projectile phases, not a mock probe.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalWeaponTerrainSupportContract(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/terrain-work-stage6-v2.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Original bool `json:"source_is_original_execution"`
		Frames   []struct {
			Frame      int
			Operations []struct {
				Name, AfterRAM string
				Input          struct{ RAM string }
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		t.Fatal(err)
	}
	if !data.Original || len(data.Frames) != 800 {
		t.Fatal("stage-six original frame coverage")
	}
	meta := loadFrameOracle(t)
	for _, r := range data.Frames {
		before, after := "", ""
		for _, o := range r.Operations {
			if o.Name == "players" {
				before = o.Input.RAM
			}
			if o.Name == "weapons" {
				after = o.AfterRAM
			}
		}
		if before == "" || after == "" {
			t.Fatalf("frame%d missing native combat boundaries", r.Frame)
		}
		b, err := hex.DecodeString(before)
		if err != nil {
			t.Fatal(err)
		}
		got := activeWorldFromOriginal(t, meta, before).Combat
		want := activeWorldFromOriginal(t, meta, after).Combat
		// At D2D9 the command dispatcher has already reused zero-page 00..05;
		// the per-player sampled controllers remain in F1/F2 and F5/F6.
		inputs := [2]PlayerInput{{Held: Buttons(b[0xf1]), Pressed: Buttons(b[0xf5])}, {Held: Buttons(b[0xf2]), Pressed: Buttons(b[0xf6])}}
		if _, err = got.AdvancePlayersAndProjectilesWithEffects(inputs); err != nil {
			t.Fatalf("frame%d: %v", r.Frame, err)
		}
		if got.Weapons != want.Weapons {
			for i, p := range got.Weapons.Projectiles {
				if p != want.Weapons.Projectiles[i] {
					t.Fatalf("frame%d projectile%d got%+v want%+v", r.Frame, i, p, want.Weapons.Projectiles[i])
				}
			}
		}
	}
}
