package pal_test

// Actual campaign acceptance, without a scene/wave probe: the new core must
// reach the real blood pool through all preceding original gameplay. Failures:
// script 12184's level-two wave is absent; field prediction stays disabled;
// native phase and field frames diverge; fresh save loses phase/party/resources
// or admits old inputs. Fixture-based pixel tests alone do not satisfy this.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestCandidateBloodPoolActualMapWaveAndSavedField(t *testing.T) {
	if campaignVariant() != "scene-wave" || developmentHealthProfile() != "hp-9999" {
		t.Skip("explicit new-core campaign replay: PAL_CAMPAIGN_VARIANT=scene-wave PAL_DEVELOPMENT_HEALTH=9999")
	}
	r := loadRiverMedicine(t, "campaign-ghost-tomb-state.json")
	initial := r.visible()
	wave := func(v pal.View) engine.DrawOp {
		t.Helper()
		if v.Scene != 60 || v.Map != 76 || v.Phase != "field" || v.Field == nil || len(v.Draws) == 0 || v.Draws[0].Kind != 2 || v.Draws[0].A != 2 || v.Draws[0].C != 0 {
			t.Fatal("actual original blood-pool wave or motor missing", v.Scene, v.Map, v.Phase, v.Field, v.Draws)
		}
		if v.Cash != initial.Cash || !bytes.Equal(raw(v.Party), raw(initial.Party)) {
			t.Fatal("idle original blood pool changed party/resources")
		}
		for _, p := range v.Party {
			if p.Controller != 0 {
				t.Fatal("original wave scene lost manual party ownership")
			}
		}
		return v.Draws[0]
	}
	start := wave(initial)
	phases := map[int]bool{}
	frames := []pal.View{}
	saves := 0
	for tick := 0; tick < 200 && len(phases) < 32; tick++ {
		v := r.visible()
		current := wave(v)
		if int((v.Field.Frame-initial.Field.Frame)%32) != (current.B-start.B+32)%32 {
			t.Fatal("actual blood-pool phase diverged from original field frames", initial.Field.Frame, v.Field.Frame, start.B, current.B)
		}
		if !phases[current.B] {
			phases[current.B] = true
			frames = append(frames, v)
		}
		if len(phases) == 8 && saves == 0 {
			before := r.visible()
			var err error
			r.g, err = r.m.SaveControl(r.c, r.g, true)
			if err != nil {
				t.Fatal(err)
			}
			r.c.Now = r.c.Now.Add(30 * 24 * time.Hour)
			r.m, _ = nativeModule(t)
			r.g, err = r.m.ResumeSaved(r.c, r.g)
			if err != nil {
				t.Fatal(err)
			}
			normalize := func(v pal.View) pal.View {
				v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
				return v
			}
			if !bytes.Equal(raw(normalize(before)), raw(normalize(r.visible()))) {
				t.Fatal("actual blood-pool save changed the wave, drawing or party")
			}
			if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 0, Keys: 4})); err == nil {
				t.Fatal("actual blood-pool resume accepted a stale movement")
			}
			saves++
			if err = os.WriteFile(campaignEvidencePath("campaign-wave-field-state.json", 1), r.g.Data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 1)
		var err error
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(phases) != 32 || saves != 1 {
		t.Fatal("actual blood-pool wave did not complete a saved 32-phase cycle", len(phases), saves)
	}
	source, err := os.ReadFile(campaignEvidencePath("campaign-ghost-tomb-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual blood pool from new-core genuine new-game replay through original tomb; no native state/scene/wave edits; original level-two wave, all 32 phases with public field state, same party/resources, fresh runtime save after 30 offline days and stale input rejection; pixel equivalence and Board interaction are separate checks", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "phases": len(phases), "saves": saves, "frames": frames}, "", "  ")
	if err = os.WriteFile(campaignEvidencePath("campaign-wave-field.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}
