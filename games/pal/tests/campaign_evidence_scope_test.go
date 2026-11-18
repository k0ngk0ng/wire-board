package pal_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCampaignEvidenceScopesPreserveUnscopedParents(t *testing.T) {
	t.Setenv("PAL_CAMPAIGN_VARIANT", "audio")
	t.Setenv("PAL_DEVELOPMENT_HEALTH", "9999")
	t.Setenv("PAL_EVIDENCE_SCOPE", "")
	base := filepath.Join("..", "..", "..", ".local/pal/verification/runtime-candidates/audio/development-health/hp-9999")
	if actual := campaignEvidencePath("checkpoint.json", 1); actual != filepath.Join(base, "checkpoint-1.json") {
		t.Fatal("accepted evidence path changed", actual)
	}
	t.Setenv("PAL_EVIDENCE_SCOPE", "hidden-island-entry")
	if actual := campaignEvidencePath("checkpoint.json", 1); actual != filepath.Join(base, "scopes/hidden-island-entry/checkpoint-1.json") {
		t.Fatal("fresh fixture evidence can overwrite accepted parent reports", actual)
	}
	for _, invalid := range []string{"../escape", "/tmp/escape", "hidden/entry", ".", "a\\b", "hidden entry", "hidden..entry", strings.Repeat("x", 65)} {
		t.Run(invalid, func(t *testing.T) {
			t.Setenv("PAL_EVIDENCE_SCOPE", invalid)
			defer func() {
				if recover() == nil {
					t.Fatal("invalid evidence scope was accepted", invalid)
				}
			}()
			campaignEvidencePath("checkpoint.json", 1)
		})
	}
}
