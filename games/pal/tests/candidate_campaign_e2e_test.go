package pal_test

// Before candidate campaign selection: a path escape writes outside the
// workspace; a candidate replaces accepted story evidence or uses the old
// core silently; development and original candidate saves share a directory.
import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateCampaignHarnessRejectsUnsafeVariants(t *testing.T) {
	for _, name := range []string{"../native", "/tmp/core", "wave/core", "", "wave_1"} {
		if name == "" {
			continue // Empty means the accepted default campaign.
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("PAL_CAMPAIGN_VARIANT", name)
			defer func() {
				if recover() == nil {
					t.Fatal("unsafe candidate campaign name accepted", name)
				}
			}()
			campaignEvidencePath("candidate.json", 1)
		})
	}
}

func TestCandidateCampaignHarnessKeepsAcceptedEvidence(t *testing.T) {
	t.Setenv("PAL_CAMPAIGN_VARIANT", "scene-wave")
	t.Setenv("PAL_DEVELOPMENT_HEALTH", "9999")
	path := filepath.ToSlash(campaignEvidencePath("candidate.json", 1))
	if !strings.Contains(path, "/verification/runtime-candidates/scene-wave/development-health/hp-9999/") {
		t.Fatal("candidate campaign shares accepted evidence", path)
	}
	if os.Getenv("PAL_CANDIDATE_CORE_CHECK") != "1" {
		return
	}
	_, r := nativeModule(t)
	s, err := r.StartCampaignForPlayers(t.Context(), 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../../.local/pal/runtime-candidates/scene-wave/hp-9999/rules.wasm")
	digest := sha256.Sum256(data)
	if err != nil || hex.EncodeToString(digest[:]) != s.BuildID {
		t.Fatal("candidate campaign did not load its real candidate core", s.BuildID, err)
	}
}
