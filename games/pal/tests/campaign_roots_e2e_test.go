package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type campaignRootRecord struct {
	SHA256       string `json:"sha256"`
	Report       string `json:"report"`
	ReportSHA256 string `json:"reportSHA256"`
}

type campaignRootRegistration struct {
	Profile    string                        `json:"profile"`
	CoreSHA256 string                        `json:"coreSHA256"`
	Roots      map[string]campaignRootRecord `json:"roots"`
}

type campaignRootCatalog struct {
	Version    int                                 `json:"version"`
	Candidates map[string]campaignRootRegistration `json:"candidates"`
}

func validateCandidateCampaignRoot(variant, profile, name string, source []byte, legacy string, core, report []byte, catalog campaignRootCatalog) error {
	digest := func(data []byte) string {
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}
	if variant == "" || variant == "scene-wave" {
		if digest(source) != legacy {
			return errors.New("retained campaign root changed")
		}
		return nil
	}
	registration, ok := catalog.Candidates[variant]
	if catalog.Version != 1 || !ok || profile != registration.Profile || digest(core) != registration.CoreSHA256 {
		return errors.New("candidate campaign core or profile has no accepted root registration")
	}
	validName := regexp.MustCompile(`^[a-z][a-z0-9-]*\.json$`)
	root, ok := registration.Roots[name]
	if !ok || !validName.MatchString(name) || !validName.MatchString(root.Report) || digest(source) != root.SHA256 || digest(report) != root.ReportSHA256 {
		return errors.New("candidate campaign root or its accepted report changed")
	}
	var result struct {
		Passed           bool `json:"passed"`
		ResumedTestRoute bool `json:"resumedTestRoute"`
	}
	if err := json.Unmarshal(report, &result); err != nil || !result.Passed || result.ResumedTestRoute {
		return errors.New("candidate root report did not accept a complete non-debug stage")
	}
	return nil
}

func acceptedCampaignRoot(name string, source []byte, legacy string) error {
	variant, profile := campaignVariant(), developmentHealthProfile()
	if variant == "" || variant == "scene-wave" {
		return validateCandidateCampaignRoot(variant, profile, name, source, legacy, nil, nil, campaignRootCatalog{})
	}
	data, err := os.ReadFile("campaign-roots.json")
	if err != nil {
		return err
	}
	var catalog campaignRootCatalog
	if err = json.Unmarshal(data, &catalog); err != nil {
		return err
	}
	registration, ok := catalog.Candidates[variant]
	if !ok || registration.Profile != profile {
		return fmt.Errorf("candidate %s/%s is not registered", variant, profile)
	}
	root, ok := registration.Roots[name]
	if !ok || !regexp.MustCompile(`^[a-z][a-z0-9-]*\.json$`).MatchString(root.Report) {
		return fmt.Errorf("candidate %s has no accepted source for %s", variant, name)
	}
	core, err := os.ReadFile(filepath.Join("..", "..", "..", ".local/pal/runtime-candidates", variant, profile, "rules.wasm"))
	if err != nil {
		return err
	}
	report, err := os.ReadFile(campaignEvidencePath(root.Report, 1))
	if err != nil {
		return err
	}
	return validateCandidateCampaignRoot(variant, profile, name, source, legacy, core, report, catalog)
}
