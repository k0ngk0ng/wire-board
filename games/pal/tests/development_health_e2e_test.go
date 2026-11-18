package pal_test

// User-authorized test-only health. Failures: the regular build is overwritten,
// debug HP is absent, original MP/rewards change, a native role selection is
// lost across a fresh runtime, or normal/debug saves cross their build boundary.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Candidate replays keep accepted cores and checkpoints intact. Only an
// explicit workspace-owned, single-component variant can select a candidate.
func campaignVariant() string {
	name := os.Getenv("PAL_CAMPAIGN_VARIANT")
	if name != "" && !regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`).MatchString(name) {
		panic("PAL_CAMPAIGN_VARIANT must be a lowercase workspace candidate name")
	}
	return name
}

func developmentHealthProfile() string {
	value := os.Getenv("PAL_DEVELOPMENT_HEALTH")
	if value == "" {
		return ""
	}
	hp, err := strconv.Atoi(value)
	if err != nil || hp < 1000 || hp > 9999 {
		panic("PAL_DEVELOPMENT_HEALTH must be between 1000 and 9999")
	}
	return "hp-" + strconv.Itoa(hp)
}

func TestDevelopmentHealthHarnessRejectsUnsafeProfiles(t *testing.T) {
	for _, value := range []string{"../native", "0", "10000", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("PAL_DEVELOPMENT_HEALTH", value)
			defer func() {
				if recover() == nil {
					t.Fatal("invalid development profile was accepted")
				}
			}()
			campaignEvidencePath("health.json", 1)
		})
	}
}

// Before enabling a campaign harness: an explicit development run must load
// its own real build identity and keep every checkpoint/report out of the
// original acceptance directory. This is never a production game option.
func TestDevelopmentHealthHarnessSeparatesArtifacts(t *testing.T) {
	t.Setenv("PAL_DEVELOPMENT_HEALTH", "9999")
	_, runtime := nativeModule(t)
	snapshot, err := runtime.StartOpening(context.Background(), 12345)
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.ReadFile(filepath.Join("..", "..", "..", ".local/pal/test-health/hp-9999/campaign/rules.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(program)
	if snapshot.BuildID != hex.EncodeToString(hash[:]) {
		t.Fatal("development campaign loaded the original authority")
	}
	if path := filepath.ToSlash(campaignEvidencePath("campaign-health-proof.json", 1)); !strings.Contains(path, "/verification/development-health/hp-9999/") {
		t.Fatal("development campaign overwrites original acceptance artifacts", path)
	}
}

func TestDevelopmentHealthCampaignThroughHiddenDragon(t *testing.T) {
	if os.Getenv("PAL_DEVELOPMENT_HEALTH") != "9999" {
		t.Skip("explicit development-only campaign; PAL_DEVELOPMENT_HEALTH=9999")
	}
	m, c, g := originalYueruMeeting(t, 1)
	m, c, g = runOriginalTournament(t, m, c, g, tournamentProof{})
	runOriginalHiddenDragon(t, m, c, g, hiddenDragonProof{})
}

func TestDevelopmentHealthBattleSaveIsolation(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	load := func(path string) (gameapi.Module, string) {
		t.Helper()
		program, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal("build the explicit development-health fixture first", err)
		}
		r, err := engine.NewRuntime(context.Background(), program, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close(context.Background()) })
		hash := sha256.Sum256(program)
		return pal.NewModule(r), hex.EncodeToString(hash[:])
	}
	const debugPath = ".local/pal/test-health/hp-9999/battle-team-1-party-3/rules.wasm"
	regular, regularHash := load(".local/pal/battle-fixtures/team-1-party-3/rules.wasm")
	m, debugHash := load(debugPath)
	if regularHash == debugHash {
		t.Fatal("development health reused the regular native identity")
	}
	c := gameapi.Context{Players: 1, Now: time.Unix(1800000000, 0), Automated: []bool{false}}
	base, err := regular.Start(c, raw(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	g, err := m.Start(c, raw(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	baseParty := view(t, regular, base, 0).Party
	initial := view(t, m, g, 0)
	if len(initial.Party) != 3 || len(baseParty) != 3 {
		t.Fatal("development fixture changed the original party")
	}
	for i, p := range initial.Party {
		if p.HP != 9999 || p.MaxHP != 9999 || baseParty[i].MaxHP >= 9999 || p.Role != baseParty[i].Role || p.MP != baseParty[i].MP || p.MaxMP != baseParty[i].MaxMP {
			t.Fatal("test health changed more than HP/maxHP", p, baseParty[i])
		}
	}
	beforeReject := bytes.Clone(g.Data)
	if _, err := regular.ResumeSaved(c, g); err == nil || !bytes.Equal(beforeReject, g.Data) {
		t.Fatal("regular runtime accepted/mutated a development save", err)
	}
	if _, err := m.ResumeSaved(c, base); err == nil {
		t.Fatal("development runtime accepted a regular save")
	}
	saved := false
	for step := 0; step < 1600; step++ {
		v := view(t, m, g, 0)
		if v.Cash == initial.Cash+3 {
			if !saved {
				t.Fatal("development fixture skipped selected-command save")
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "mode": "development-only HP 9999 at battle entry; not original-difficulty acceptance", "regularBuildSHA256": regularHash, "developmentBuildSHA256": debugHash, "normalParty": baseParty, "developmentParty": initial.Party, "originalMPPreserved": true, "normalRejectsDevelopmentSave": true, "developmentRejectsNormalSave": true, "freshRuntimeSelectedCommandSave": true, "oldInputRejected": true, "originalCashReward": 3}, "", "  ")
			if err := os.WriteFile(filepath.Join(root, ".local/pal/verification/development-health-isolation.json"), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if !saved && v.Battle != nil && v.Battle.Phase == 0 && v.Battle.State == 1 && v.Battle.Menu == 0 && v.Battle.Current == 1 {
			before := v
			g, err = m.SaveControl(c, g, true)
			if err != nil {
				t.Fatal(err)
			}
			c.Now = c.Now.Add(24 * time.Hour)
			m, _ = load(debugPath)
			g, err = m.ResumeSaved(c, g)
			if err != nil {
				t.Fatal(err)
			}
			after := view(t, m, g, 0)
			if !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Battle), raw(after.Battle)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
				t.Fatal("development save changed native HP/selection/drawing")
			}
			if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
				t.Fatal("development save accepted stale input")
			}
			saved = true
			c.Automated[0] = true
			v = after
		}
		if saved {
			a, e := m.BotAction(c, g)
			if e != nil {
				t.Fatal(e)
			}
			if len(a) > 0 {
				g, err = m.Apply(c, g, a)
			}
		} else if v.Battle != nil && v.Battle.Phase == 0 && v.Battle.Current == 0 && (v.Battle.State == 1 || v.Battle.State == 2) && v.Seq == v.AppliedSeq {
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2})
		}
		if err != nil {
			t.Fatal(err)
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("development-health battle never completed")
}
