package contra

// Failures enumerated before implementation: original guards skipped; standard
// B held treated as automatic; M burst carries or idle low-nibble timing wrong;
// slot partition/occupancy ignored; partial spread or laser replacement wrong;
// R speed guessed; jump/down muzzle order changed; stale fields zeroed or kept
// incorrectly; camera movement/frame ordering and offscreen recycling differ.
// Natural playback and explicitly staged boundary calls are separate oracles.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type weaponOracle struct {
	Scope string
	Fire  []struct {
		Frame, Case             int
		Request                 FireRequest
		Before, After           string
		MachineGunTimer, Recoil uint8
		Sounds                  []uint8
	}
	Steps []struct {
		Frame                                                                                    int
		StageType, Vertical, Scroll, SolidCollision, ScreenCleared, ScrollX, ScrollY, PPUControl uint8
		Before, After, Collision                                                                 string
	}
}

func readWeaponOracle(t *testing.T, name string) weaponOracle {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/weapons-" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle weaponOracle
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Scope != name {
		t.Fatal("weapon oracle scope mismatch")
	}
	return oracle
}

func originalWeaponPool(t *testing.T, encoded string) WeaponSystem {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != 336 {
		t.Fatal("invalid original projectile bytes", err)
	}
	var out WeaponSystem
	for i := range out.Projectiles {
		v := func(field int) uint8 { return raw[field*16+i] }
		out.Projectiles[i] = Projectile{Sprite: v(0), Attributes: v(1), Kind: v(2), YFraction: v(3), XFraction: v(4), Y: v(5), X: v(6), VY: int16(uint16(v(7)) | uint16(v(9))<<8), VX: int16(uint16(v(8)) | uint16(v(10))<<8), Timer: v(11), Aim: v(12), Routine: v(13), Owner: v(14), RapidOrSpread: v(15), DistanceOrSpread: v(16), CenterX: v(17), CenterY: v(18), CenterXFraction: v(19), CenterYFraction: v(20)}
	}
	return out
}

func TestOriginalWeaponFire(t *testing.T) {
	for _, name := range []string{"natural", "isolated", "equipment", "machine"} {
		t.Run(name, func(t *testing.T) {
			oracle := readWeaponOracle(t, name)
			wantCount := 480
			if name == "isolated" {
				wantCount = 6440
			}
			if name == "equipment" {
				wantCount = 9488
			}
			if name == "machine" {
				wantCount = 1367
			}
			if len(oracle.Fire) != wantCount {
				t.Fatalf("missing weapon coverage: %d", len(oracle.Fire))
			}
			for i, row := range oracle.Fire {
				pool := originalWeaponPool(t, row.Before)
				want := originalWeaponPool(t, row.After)
				got := pool.Fire(row.Request)
				if got.MachineGunTimer != row.MachineGunTimer || got.Recoil != row.Recoil || !reflect.DeepEqual(append([]uint8{}, got.Sounds...), append([]uint8{}, row.Sounds...)) || pool != want {
					t.Fatalf("%s record %d case %d frame %d input %+v\nresult %+v expected timer=%d recoil=%d sounds=%v\npool=%+v\nwant=%+v", name, i, row.Case, row.Frame, row.Request, got, row.MachineGunTimer, row.Recoil, row.Sounds, pool, want)
				}
			}
			t.Logf("%d original %s weapon calls match", wantCount, name)
		})
	}
}

func TestOriginalNaturalProjectileSteps(t *testing.T) {
	for _, name := range []string{"natural", "equipment", "machine"} {
		oracle := readWeaponOracle(t, name)
		wantCount := 240
		if name == "equipment" {
			wantCount = 5391
		}
		if name == "machine" {
			wantCount = 700
		}
		if len(oracle.Steps) != wantCount {
			t.Fatal("missing natural projectile steps")
		}
		for i, row := range oracle.Steps {
			pool := originalWeaponPool(t, row.Before)
			want := originalWeaponPool(t, row.After)
			raw, err := hex.DecodeString(row.Collision)
			if err != nil || len(raw) != 128 {
				t.Fatal("invalid terrain oracle")
			}
			window := CollisionWindow(raw)
			scroll := CollisionScroll{X: row.ScrollX, Y: row.ScrollY, NameTable: row.PPUControl}
			pool.Step(BulletStep{StageType: row.StageType, ScrollType: row.Vertical, Scroll: row.Scroll, SolidCollision: row.SolidCollision&128 != 0, ScreenCleared: row.ScreenCleared != 0, Terrain: func(x, y uint8) TerrainKind { return window.At(x, y, scroll).Kind }})
			if pool != want {
				t.Fatalf("natural projectile step %d frame %d\ngot=%+v\nwant=%+v", i, row.Frame, pool, want)
			}
		}
		t.Logf("%d %s full-pool steps match", wantCount, name)
	}
}
