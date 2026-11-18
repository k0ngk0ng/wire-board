package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestCandidateCampaignRootRequiresItsOwnCoreCheckpointAndFullReport(t *testing.T) {
	digest := func(data []byte) string {
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}
	core, source, report := []byte("actual new candidate core"), []byte("actual accepted checkpoint"), []byte(`{"passed":true,"resumedTestRoute":false}`)
	entry := campaignRootRecord{SHA256: digest(source), Report: "campaign-parent.json", ReportSHA256: digest(report)}
	registration := campaignRootRegistration{Profile: "hp-9999", CoreSHA256: digest(core), Roots: map[string]campaignRootRecord{"campaign-parent-state.json": entry}}
	catalog := campaignRootCatalog{Version: 1, Candidates: map[string]campaignRootRegistration{"audio": registration}}
	check := func(candidate, profile, name string, currentSource, currentCore, currentReport []byte, currentCatalog campaignRootCatalog) error {
		return validateCandidateCampaignRoot(candidate, profile, name, currentSource, digest(source), currentCore, currentReport, currentCatalog)
	}
	if err := check("audio", "hp-9999", "campaign-parent-state.json", source, core, report, catalog); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"", "scene-wave"} {
		if err := check(variant, "hp-9999", "campaign-parent-state.json", source, nil, nil, campaignRootCatalog{}); err != nil {
			t.Fatal("legacy pinned source was changed", err)
		}
	}
	for _, test := range []struct {
		name, candidate, profile, state string
		source, core, report            []byte
	}{
		{"unknown-candidate", "unregistered", "hp-9999", "campaign-parent-state.json", source, core, report},
		{"wrong-profile", "audio", "", "campaign-parent-state.json", source, core, report},
		{"missing-root", "audio", "hp-9999", "other-state.json", source, core, report},
		{"changed-source", "audio", "hp-9999", "campaign-parent-state.json", []byte("different save"), core, report},
		{"changed-core", "audio", "hp-9999", "campaign-parent-state.json", source, []byte("different core"), report},
		{"changed-report", "audio", "hp-9999", "campaign-parent-state.json", source, core, []byte(`{"passed":true}`)},
		{"escaping-state", "audio", "hp-9999", "../campaign-parent-state.json", source, core, report},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := check(test.candidate, test.profile, test.state, test.source, test.core, test.report, catalog); err == nil {
				t.Fatal("invalid candidate source accepted")
			}
		})
	}
	for _, rejected := range []string{`{"passed":false}`, `{"passed":true,"resumedTestRoute":true}`, `{"passed":true,"resumedTestRoute":false} trailing`} {
		proof := []byte(rejected)
		record := entry
		record.ReportSHA256 = digest(proof)
		current := registration
		current.Roots = map[string]campaignRootRecord{"campaign-parent-state.json": record}
		bad := campaignRootCatalog{Version: 1, Candidates: map[string]campaignRootRegistration{"audio": current}}
		if err := check("audio", "hp-9999", "campaign-parent-state.json", source, core, proof, bad); err == nil {
			t.Fatal("failed or debug-only report was accepted", rejected)
		}
	}
}
