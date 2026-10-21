package basketball

import "github.com/k0ngk0ng/wire-board/internal/gameapi"

// Controlled boundaries for external-package HTTP acceptance. These helpers are
// compiled only by go test, keeping the production module free of test hooks.
var ExportCampaignHostHalftime = campaignHostHalftime
var ExportCampaignFinishedHost = campaignFinishedHost
var ExportDecode = decode

func ExportStateMatch(s State) (gameapi.Match, error) { return s.match() }
