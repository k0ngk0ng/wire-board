package contra

// Failure inventory before implementation: plate gates; stopped timers; byte
// carry and negation; damage-layer transitions; empty/full child pool; collision
// flicker; shared random writes; dead-player aiming; projectile threshold;
// homing wrap/tie; Boss sound reset and delayed source campaign transition.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type baseBossOracleSnapshot struct {
	Pool           [][]uint8
	World          []uint8
	Players        [2]EnemyTarget
	GenerationBias *uint8
}
type baseBossOracleAudio struct {
	Kind string
	Code uint8
}
type baseBossOracleRecord struct {
	Case, Slot, Type int
	Name             string
	Before, After    baseBossOracleSnapshot
	Audio            []baseBossOracleAudio
	BiasWrites       []struct {
		PC    uint16
		Value uint8
	}
}

func checkBaseBossOracle(t *testing.T, r baseBossOracleRecord, slot int) {
	t.Helper()
	w := append([]uint8(nil), r.Before.World...)
	rng := RandomState{Value: w[4]}
	s := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Pool), Base: BaseEnemyState{CoresRemaining: w[12], PlatingDestroyed: w[13], AttackRounds: w[14], RedSoldierCreated: w[15], GrenadeLauncher: w[16]}}
	c := EnemyStepContext{Frame: w[0], WeaponStrength: w[1], Level: w[2], GameLoop: w[3], Random: w[4], SceneType: w[6], Vertical: w[7] != 0, Screen: w[8], ScreenCleared: &w[9], FrameScroll: w[10], AttackEnabled: w[17] != 0, Players: r.Before.Players, SharedRandom: &rng}
	signal, err := s.StepBaseBoss(slot, c)
	if err != nil {
		t.Fatalf("case%d type%d state%d: %v", r.Case, s.Slots[slot].Kind, r.Before.Pool[slot][4], err)
	}
	actualAudio := []baseBossOracleAudio{}
	switch signal {
	case BossDoorNoSignal:
	case BossDoorDefeated:
		w[5], w[19] = 1, 255
		actualAudio = append(actualAudio, baseBossOracleAudio{Kind: "reset"})
	case BossBaseFinished:
		w[19], w[20] = 96, 0
	default:
		t.Fatalf("unknown base Boss signal %d", signal)
	}
	w[4] = rng.Value
	w[12], w[13], w[14], w[15], w[16] = s.Base.CoresRemaining, s.Base.PlatingDestroyed, s.Base.AttackRounds, s.Base.RedSoldierCreated, s.Base.GrenadeLauncher
	for _, code := range s.PendingSounds {
		actualAudio = append(actualAudio, baseBossOracleAudio{Kind: "play", Code: code})
	}
	want := enemyPoolFromOracle(r.After.Pool)
	if s.Slots != want {
		for i, v := range s.Slots {
			if v != want[i] {
				t.Fatalf("case%d type%d state%d slot%d got%+v want%+v before%+v", r.Case, r.Before.Pool[slot][11], r.Before.Pool[slot][4], i, v, want[i], enemyFromOracle(r.Before.Pool[i]))
			}
		}
	}
	if !reflect.DeepEqual(w, r.After.World) {
		t.Fatalf("case%d world got%v want%v", r.Case, w, r.After.World)
	}
	if !reflect.DeepEqual(actualAudio, r.Audio) {
		t.Fatalf("case%d audio got%+v want%+v", r.Case, actualAudio, r.Audio)
	}
	if r.Before.GenerationBias != nil {
		if r.After.GenerationBias == nil {
			t.Fatal("missing original generation bias output")
		}
		got := *r.Before.GenerationBias
		if s.GenerationBiasChanged {
			got = s.GenerationBiasValue
		}
		if got != *r.After.GenerationBias || s.GenerationBiasChanged != (len(r.BiasWrites) > 0) {
			t.Fatalf("case%d type%x phase%d generation bias got%d changed%v want%d writes%+v", r.Case, r.Before.Pool[slot][11], r.Before.Pool[slot][4], got, s.GenerationBiasChanged, *r.After.GenerationBias, r.BiasWrites)
		}
	}
}

// Full Tick exposed the missing $06 producer in Eye projectile phase 1.
// Failures: lost speed=6 write; bubble's temporary speed retained instead of
// its final 24-direction write; homing alignment/limit paths write wrongly.
func TestOriginalBaseBossGenerationBias(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/enemies-base-boss-bias.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Original bool `json:"source_is_original_execution"`
		Records  []baseBossOracleRecord
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if !fixture.Original || len(fixture.Records) != 810 {
		t.Fatal("base Boss bias coverage changed")
	}
	for _, r := range fixture.Records {
		if r.Before.GenerationBias == nil || r.After.GenerationBias == nil {
			t.Fatal("missing original bias observation")
		}
		checkBaseBossOracle(t, r, 15)
	}
}

func TestOriginalBaseBossControlled(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/enemies-base-boss-controlled.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Original bool `json:"source_is_original_execution"`
		Records  []baseBossOracleRecord
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if !fixture.Original || len(fixture.Records) != 810 {
		t.Fatal("base Boss original coverage changed")
	}
	for _, r := range fixture.Records {
		checkBaseBossOracle(t, r, 15)
	}
}

func TestOriginalBaseBossControllerContinuation(t *testing.T) {
	count := 0
	for _, path := range []string{"enemies-base-2-boss.json.gz", "enemies-base-4-boss.json.gz", "enemies-base-boss-sequence-2.json.gz", "enemies-base-boss-sequence-4.json.gz"} {
		f, err := os.Open("../../../docs/research/nes-contra/evidence/" + path)
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct {
			Original bool `json:"source_is_original_execution"`
			Records  []baseBossOracleRecord
		}
		err = json.NewDecoder(z).Decode(&fixture)
		z.Close()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !fixture.Original {
			t.Fatal("not original execution")
		}
		for _, r := range fixture.Records {
			if r.Name == "step" && (r.Type == 16 || r.Type == 27 || r.Type == 28 || r.Type == 29) {
				count++
				r.Case = count
				checkBaseBossOracle(t, r, r.Slot)
			}
		}
	}
	if count != 7959 {
		t.Fatalf("original Boss call coverage changed: %d", count)
	}
	t.Logf("original controller continuation: %d Boss calls", count)
}
